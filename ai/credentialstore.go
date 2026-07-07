package ai

// Ports: packages/ai/src/auth/credential-store.ts

import (
	"context"
	"sync"
)

// InMemoryCredentialStore is the default credential store. Apps inject
// persistent stores (see ai/auth for the file-backed implementation). Keyed
// by Provider.ID, one credential per provider; writes are serialized per
// provider.
type InMemoryCredentialStore struct {
	mu          sync.Mutex
	credentials map[string]Credential
	locks       map[string]*sync.Mutex
}

// NewInMemoryCredentialStore creates an empty store.
func NewInMemoryCredentialStore() *InMemoryCredentialStore {
	return &InMemoryCredentialStore{
		credentials: make(map[string]Credential),
		locks:       make(map[string]*sync.Mutex),
	}
}

func (s *InMemoryCredentialStore) providerLock(providerID string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, ok := s.locks[providerID]
	if !ok {
		lock = &sync.Mutex{}
		s.locks[providerID] = lock
	}
	return lock
}

func (s *InMemoryCredentialStore) Read(_ context.Context, providerID string) (Credential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.credentials[providerID], nil
}

func (s *InMemoryCredentialStore) Modify(
	_ context.Context,
	providerID string,
	fn func(current Credential) (Credential, error),
) (Credential, error) {
	lock := s.providerLock(providerID)
	lock.Lock()
	defer lock.Unlock()

	s.mu.Lock()
	current := s.credentials[providerID]
	s.mu.Unlock()

	next, err := fn(current)
	if err != nil {
		return nil, err
	}
	if next != nil {
		s.mu.Lock()
		s.credentials[providerID] = next
		s.mu.Unlock()
		return next, nil
	}
	return current, nil
}

func (s *InMemoryCredentialStore) Delete(_ context.Context, providerID string) error {
	lock := s.providerLock(providerID)
	lock.Lock()
	defer lock.Unlock()
	s.mu.Lock()
	delete(s.credentials, providerID)
	s.mu.Unlock()
	return nil
}
