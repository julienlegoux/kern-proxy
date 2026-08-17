package auth

// Ports: packages/coding-agent/test/auth-storage.test.ts ("persistence
// semantics" suite; the store surface of the other suites). Config-value
// resolution ($ENV / !command), runtime overrides, and OAuth refresh are
// separate layers in this port (ai/resolve.go), so those suites do not port
// here. The cross-goroutine and cross-process locking tests are new: Go can
// exercise the mutual exclusion the upstream store gets from proper-lockfile.

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"

	"github.com/kern-ia/kern-link/ai"
)

func newTestStore(t *testing.T) (*FileCredentialStore, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "auth.json")
	return NewFileCredentialStore(path), path
}

func writeAuthJSON(t *testing.T, path string, data map[string]any) {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readAuthJSON(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	return data
}

func TestReadMissingFileReturnsNil(t *testing.T) {
	store, _ := newTestStore(t)
	cred, err := store.Read(context.Background(), "anthropic")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if cred != nil {
		t.Fatalf("expected nil credential, got %#v", cred)
	}
}

func TestReadMissingEntryReturnsNil(t *testing.T) {
	store, path := newTestStore(t)
	writeAuthJSON(t, path, map[string]any{
		"openai": map[string]any{"type": "api_key", "key": "openai-key"},
	})
	cred, err := store.Read(context.Background(), "anthropic")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if cred != nil {
		t.Fatalf("expected nil credential, got %#v", cred)
	}
}

// Upstream: "literal API key is returned directly" — narrowed to storage:
// this store persists and returns credentials; value resolution is a
// different layer.
func TestReadReturnsStoredCredential(t *testing.T) {
	store, path := newTestStore(t)
	writeAuthJSON(t, path, map[string]any{
		"anthropic": map[string]any{"type": "api_key", "key": "sk-ant-literal-key"},
	})
	cred, err := store.Read(context.Background(), "anthropic")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	apiKey, ok := cred.(*ai.APIKeyCredential)
	if !ok {
		t.Fatalf("expected *ai.APIKeyCredential, got %#v", cred)
	}
	if apiKey.Key != "sk-ant-literal-key" {
		t.Fatalf("Key = %q", apiKey.Key)
	}
}

func TestModifyCreatesFileWithModes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "agent")
	path := filepath.Join(dir, "auth.json")
	store := NewFileCredentialStore(path)

	written, err := store.Modify(context.Background(), "anthropic",
		func(current ai.Credential) (ai.Credential, error) {
			if current != nil {
				t.Fatalf("expected nil current, got %#v", current)
			}
			return &ai.APIKeyCredential{Key: "fresh-key"}, nil
		})
	if err != nil {
		t.Fatalf("Modify: %v", err)
	}
	if written.(*ai.APIKeyCredential).Key != "fresh-key" {
		t.Fatalf("returned credential = %#v", written)
	}

	if runtime.GOOS != "windows" {
		fileInfo, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := fileInfo.Mode().Perm(); perm != 0o600 {
			t.Errorf("auth.json mode = %o, want 600", perm)
		}
		dirInfo, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if perm := dirInfo.Mode().Perm(); perm != 0o700 {
			t.Errorf("dir mode = %o, want 700", perm)
		}
	}

	data := readAuthJSON(t, path)
	if _, ok := data["anthropic"]; !ok {
		t.Fatalf("anthropic entry missing from %v", data)
	}
}

// Upstream: "set preserves unrelated external edits". The google entry uses
// an unknown credential type to prove foreign entries round-trip untouched.
func TestModifyPreservesUnrelatedExternalEdits(t *testing.T) {
	store, path := newTestStore(t)
	writeAuthJSON(t, path, map[string]any{
		"anthropic": map[string]any{"type": "api_key", "key": "old-anthropic"},
		"openai":    map[string]any{"type": "api_key", "key": "openai-key"},
	})

	// Simulate an external edit between store operations.
	writeAuthJSON(t, path, map[string]any{
		"anthropic": map[string]any{"type": "api_key", "key": "old-anthropic"},
		"openai":    map[string]any{"type": "api_key", "key": "openai-key"},
		"google":    map[string]any{"type": "weird-future-type", "blob": "opaque"},
	})

	_, err := store.Modify(context.Background(), "anthropic",
		func(ai.Credential) (ai.Credential, error) {
			return &ai.APIKeyCredential{Key: "new-anthropic"}, nil
		})
	if err != nil {
		t.Fatalf("Modify: %v", err)
	}

	data := readAuthJSON(t, path)
	var anthropic struct{ Key string }
	if err := json.Unmarshal(data["anthropic"], &anthropic); err != nil {
		t.Fatal(err)
	}
	if anthropic.Key != "new-anthropic" {
		t.Errorf("anthropic key = %q", anthropic.Key)
	}
	var openai struct{ Key string }
	if err := json.Unmarshal(data["openai"], &openai); err != nil {
		t.Fatal(err)
	}
	if openai.Key != "openai-key" {
		t.Errorf("openai key = %q", openai.Key)
	}
	var google struct {
		Type string `json:"type"`
		Blob string `json:"blob"`
	}
	if err := json.Unmarshal(data["google"], &google); err != nil {
		t.Fatal(err)
	}
	if google.Type != "weird-future-type" || google.Blob != "opaque" {
		t.Errorf("google entry not preserved: %s", data["google"])
	}
}

// Upstream: "remove preserves unrelated external edits".
func TestDeletePreservesUnrelatedExternalEdits(t *testing.T) {
	store, path := newTestStore(t)
	writeAuthJSON(t, path, map[string]any{
		"anthropic": map[string]any{"type": "api_key", "key": "anthropic-key"},
	})

	writeAuthJSON(t, path, map[string]any{
		"anthropic": map[string]any{"type": "api_key", "key": "anthropic-key"},
		"openai":    map[string]any{"type": "api_key", "key": "openai-key"},
		"google":    map[string]any{"type": "api_key", "key": "google-key"},
	})

	if err := store.Delete(context.Background(), "anthropic"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	data := readAuthJSON(t, path)
	if _, ok := data["anthropic"]; ok {
		t.Errorf("anthropic entry not deleted")
	}
	for _, provider := range []string{"openai", "google"} {
		if _, ok := data[provider]; !ok {
			t.Errorf("%s entry lost", provider)
		}
	}
}

// Upstream: "throws and does not overwrite malformed auth file after load
// error".
func TestModifyMalformedFileFailsWithoutOverwrite(t *testing.T) {
	store, path := newTestStore(t)
	if err := os.WriteFile(path, []byte("{invalid-json"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := store.Modify(context.Background(), "openai",
		func(ai.Credential) (ai.Credential, error) {
			return &ai.APIKeyCredential{Key: "openai-key"}, nil
		})
	if err == nil {
		t.Fatal("expected error on malformed auth.json")
	}

	raw, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(raw) != "{invalid-json" {
		t.Fatalf("malformed file was overwritten: %q", raw)
	}
}

func TestReadMalformedFileReturnsError(t *testing.T) {
	store, path := newTestStore(t)
	if err := os.WriteFile(path, []byte("{invalid-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Read(context.Background(), "anthropic"); err == nil {
		t.Fatal("expected error on malformed auth.json")
	}
}

func TestModifyNilResultLeavesEntryUnchanged(t *testing.T) {
	store, path := newTestStore(t)
	writeAuthJSON(t, path, map[string]any{
		"anthropic": map[string]any{"type": "api_key", "key": "keep-me"},
	})

	result, err := store.Modify(context.Background(), "anthropic",
		func(current ai.Credential) (ai.Credential, error) {
			return nil, nil
		})
	if err != nil {
		t.Fatalf("Modify: %v", err)
	}
	if result.(*ai.APIKeyCredential).Key != "keep-me" {
		t.Fatalf("expected current credential back, got %#v", result)
	}

	cred, err := store.Read(context.Background(), "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	if cred.(*ai.APIKeyCredential).Key != "keep-me" {
		t.Fatalf("entry changed: %#v", cred)
	}
}

func TestModifyFnErrorPropagatesWithoutWrite(t *testing.T) {
	store, path := newTestStore(t)
	writeAuthJSON(t, path, map[string]any{
		"anthropic": map[string]any{"type": "api_key", "key": "keep-me"},
	})

	wantErr := os.ErrPermission
	_, err := store.Modify(context.Background(), "anthropic",
		func(ai.Credential) (ai.Credential, error) {
			return nil, wantErr
		})
	if err == nil {
		t.Fatal("expected fn error to propagate")
	}

	cred, err := store.Read(context.Background(), "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	if cred.(*ai.APIKeyCredential).Key != "keep-me" {
		t.Fatalf("entry changed after fn error: %#v", cred)
	}
}

func TestDeleteMissingEntryIsNoop(t *testing.T) {
	store, _ := newTestStore(t)
	if err := store.Delete(context.Background(), "anthropic"); err != nil {
		t.Fatalf("Delete on missing file: %v", err)
	}
}

func TestOAuthCredentialRoundTripPreservesExtra(t *testing.T) {
	store, _ := newTestStore(t)
	_, err := store.Modify(context.Background(), "github-copilot",
		func(ai.Credential) (ai.Credential, error) {
			return &ai.OAuthCredential{
				Refresh: "gh-refresh",
				Access:  "gh-access",
				Expires: 1234567890,
				Extra:   map[string]any{"enterpriseUrl": "https://company.ghe.com"},
			}, nil
		})
	if err != nil {
		t.Fatalf("Modify: %v", err)
	}

	cred, err := store.Read(context.Background(), "github-copilot")
	if err != nil {
		t.Fatal(err)
	}
	oauth, ok := cred.(*ai.OAuthCredential)
	if !ok {
		t.Fatalf("expected *ai.OAuthCredential, got %#v", cred)
	}
	if oauth.Refresh != "gh-refresh" || oauth.Access != "gh-access" || oauth.Expires != 1234567890 {
		t.Fatalf("core fields lost: %#v", oauth)
	}
	if oauth.Extra["enterpriseUrl"] != "https://company.ghe.com" {
		t.Fatalf("extra fields lost: %#v", oauth.Extra)
	}
}

func TestDefaultPath(t *testing.T) {
	path, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(".pi", "agent", "auth.json")
	if !filepath.IsAbs(path) {
		t.Errorf("DefaultPath not absolute: %q", path)
	}
	if got := path[len(path)-len(want):]; got != want {
		t.Errorf("DefaultPath = %q, want suffix %q", path, want)
	}
}

// incrementKey bumps the integer stored in an APIKeyCredential.Key. A lost
// or double-applied update shows up as a wrong final count.
func incrementKey(current ai.Credential) (ai.Credential, error) {
	count := 0
	if current != nil {
		n, err := strconv.Atoi(current.(*ai.APIKeyCredential).Key)
		if err != nil {
			return nil, err
		}
		count = n
	}
	return &ai.APIKeyCredential{Key: strconv.Itoa(count + 1)}, nil
}

func TestConcurrentModifyAcrossGoroutines(t *testing.T) {
	store, _ := newTestStore(t)
	const goroutines = 16
	const perGoroutine = 5

	var wg sync.WaitGroup
	errs := make(chan error, goroutines*perGoroutine)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				if _, err := store.Modify(context.Background(), "counter", incrementKey); err != nil {
					errs <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("Modify: %v", err)
	}

	cred, err := store.Read(context.Background(), "counter")
	if err != nil {
		t.Fatal(err)
	}
	if got := cred.(*ai.APIKeyCredential).Key; got != strconv.Itoa(goroutines*perGoroutine) {
		t.Fatalf("count = %s, want %d (lost updates)", got, goroutines*perGoroutine)
	}
}

// TestFileStoreChildProcess is the re-exec helper for the cross-process
// test below; it only runs when the parent sets FILESTORE_CHILD_PATH.
func TestFileStoreChildProcess(t *testing.T) {
	path := os.Getenv("FILESTORE_CHILD_PATH")
	if path == "" {
		t.Skip("helper process for TestConcurrentModifyAcrossProcesses")
	}
	n, err := strconv.Atoi(os.Getenv("FILESTORE_CHILD_N"))
	if err != nil {
		t.Fatal(err)
	}
	store := NewFileCredentialStore(path)
	for i := 0; i < n; i++ {
		if _, err := store.Modify(context.Background(), "counter", incrementKey); err != nil {
			t.Fatalf("Modify: %v", err)
		}
	}
}

func TestConcurrentModifyAcrossProcesses(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns child processes")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "auth.json")
	const children = 4
	const perChild = 8

	var wg sync.WaitGroup
	outputs := make([][]byte, children)
	errs := make([]error, children)
	for i := 0; i < children; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cmd := exec.Command(exe, "-test.run=^TestFileStoreChildProcess$", "-test.count=1")
			cmd.Env = append(os.Environ(),
				"FILESTORE_CHILD_PATH="+path,
				"FILESTORE_CHILD_N="+strconv.Itoa(perChild),
			)
			outputs[i], errs[i] = cmd.CombinedOutput()
		}(i)
	}
	wg.Wait()
	for i := 0; i < children; i++ {
		if errs[i] != nil {
			t.Fatalf("child %d: %v\n%s", i, errs[i], outputs[i])
		}
	}

	store := NewFileCredentialStore(path)
	cred, err := store.Read(context.Background(), "counter")
	if err != nil {
		t.Fatal(err)
	}
	if got := cred.(*ai.APIKeyCredential).Key; got != strconv.Itoa(children*perChild) {
		t.Fatalf("count = %s, want %d (lost updates across processes)", got, children*perChild)
	}
}
