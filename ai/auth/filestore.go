package auth

// Ports: packages/coding-agent/src/core/auth-storage.ts (FileAuthStorageBackend
// + the AuthStorage persistence semantics). The upstream class also resolves
// config values ($ENV / !command), runtime overrides, and OAuth refresh; in
// this port those live in ai/resolve.go and the login flows (phase 12) — this
// package only persists credentials. proper-lockfile's sidecar lock becomes a
// gofrs/flock lock on auth.json.lock: flock on Unix, LockFileEx on Windows.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"

	"github.com/julienlegoux/kern-proxy/ai"
)

// lockRetryInterval is how often a blocked lock acquisition retries; the
// upstream sync path polls every 20ms.
const lockRetryInterval = 20 * time.Millisecond

// FileCredentialStore is a persistent ai.CredentialStore backed by a JSON
// file (by default ~/.pi/agent/auth.json): one object keyed by provider ID,
// values in the ai.UnmarshalCredential wire shape. Every method runs under
// an OS-level lock on a sidecar auth.json.lock file, so read-modify-write
// cycles are atomic across goroutines and across processes. Entries for
// other providers — including credential types this build does not know —
// round-trip untouched, so external edits survive concurrent use.
type FileCredentialStore struct {
	path string
}

var _ ai.CredentialStore = (*FileCredentialStore)(nil)

// NewFileCredentialStore creates a store backed by the JSON file at path.
// The file and its directory are created on first write (0600 inside 0700).
func NewFileCredentialStore(path string) *FileCredentialStore {
	return &FileCredentialStore{path: path}
}

// DefaultPath returns the conventional credential file location,
// ~/.pi/agent/auth.json.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("auth: resolve home directory: %w", err)
	}
	return filepath.Join(home, ".pi", "agent", "auth.json"), nil
}

// withLock runs fn holding the exclusive cross-process lock. The lock lives
// beside the store file so locking never touches auth.json itself.
func (s *FileCredentialStore) withLock(ctx context.Context, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("auth: create credential directory: %w", err)
	}
	lock := flock.New(s.path + ".lock")
	locked, err := lock.TryLockContext(ctx, lockRetryInterval)
	if err != nil {
		return fmt.Errorf("auth: lock %s: %w", lock.Path(), err)
	}
	if !locked {
		return fmt.Errorf("auth: lock %s: not acquired", lock.Path())
	}
	defer func() { _ = lock.Unlock() }()
	return fn()
}

// load reads the raw provider map. A missing file is an empty store.
func (s *FileCredentialStore) load() (map[string]json.RawMessage, error) {
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("auth: read %s: %w", s.path, err)
	}
	entries := map[string]json.RawMessage{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &entries); err != nil {
			return nil, fmt.Errorf("auth: parse %s: %w", s.path, err)
		}
	}
	return entries, nil
}

// save writes the provider map back, pretty-printed like the upstream file,
// with owner-only permissions.
func (s *FileCredentialStore) save(entries map[string]json.RawMessage) error {
	raw, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("auth: encode %s: %w", s.path, err)
	}
	if err := os.WriteFile(s.path, raw, 0o600); err != nil {
		return fmt.Errorf("auth: write %s: %w", s.path, err)
	}
	// The 0600 in WriteFile only applies on create; clamp pre-existing files
	// too, like the upstream chmod after every write.
	if err := os.Chmod(s.path, 0o600); err != nil {
		return fmt.Errorf("auth: chmod %s: %w", s.path, err)
	}
	return nil
}

func decodeEntry(entries map[string]json.RawMessage, providerID string) (ai.Credential, error) {
	raw, ok := entries[providerID]
	if !ok {
		return nil, nil
	}
	cred, err := ai.UnmarshalCredential(raw)
	if err != nil {
		return nil, fmt.Errorf("auth: credential %q: %w", providerID, err)
	}
	return cred, nil
}

func (s *FileCredentialStore) Read(ctx context.Context, providerID string) (ai.Credential, error) {
	if _, err := os.Stat(s.path); errors.Is(err, os.ErrNotExist) {
		// Nothing stored; skip locking so reads never create the directory.
		return nil, nil
	}
	var cred ai.Credential
	err := s.withLock(ctx, func() error {
		entries, err := s.load()
		if err != nil {
			return err
		}
		cred, err = decodeEntry(entries, providerID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return cred, nil
}

func (s *FileCredentialStore) Modify(
	ctx context.Context,
	providerID string,
	fn func(current ai.Credential) (ai.Credential, error),
) (ai.Credential, error) {
	var result ai.Credential
	err := s.withLock(ctx, func() error {
		entries, err := s.load()
		if err != nil {
			return err
		}
		current, err := decodeEntry(entries, providerID)
		if err != nil {
			return err
		}
		next, err := fn(current)
		if err != nil {
			return err
		}
		if next == nil {
			result = current
			return nil
		}
		raw, err := json.Marshal(next)
		if err != nil {
			return fmt.Errorf("auth: encode credential %q: %w", providerID, err)
		}
		entries[providerID] = raw
		if err := s.save(entries); err != nil {
			return err
		}
		result = next
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *FileCredentialStore) Delete(ctx context.Context, providerID string) error {
	return s.withLock(ctx, func() error {
		entries, err := s.load()
		if err != nil {
			return err
		}
		if _, ok := entries[providerID]; !ok {
			return nil
		}
		delete(entries, providerID)
		return s.save(entries)
	})
}
