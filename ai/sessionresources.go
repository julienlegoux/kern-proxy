package ai

// Ports: packages/ai/src/session-resources.ts

import (
	"errors"
	"sync"
)

var (
	sessionResourcesMu sync.Mutex
	sessionCleanups    = map[int]func(sessionID string) error{}
	sessionCleanupSeq  int
)

// RegisterSessionResourceCleanup registers a cleanup callback for per-session
// resources (connections, servers). The returned function unregisters it.
func RegisterSessionResourceCleanup(fn func(sessionID string) error) (unregister func()) {
	sessionResourcesMu.Lock()
	defer sessionResourcesMu.Unlock()
	sessionCleanupSeq++
	id := sessionCleanupSeq
	sessionCleanups[id] = fn
	return func() {
		sessionResourcesMu.Lock()
		defer sessionResourcesMu.Unlock()
		delete(sessionCleanups, id)
	}
}

// CleanupSessionResources runs all registered cleanups for a session,
// collecting failures into a single joined error.
func CleanupSessionResources(sessionID string) error {
	sessionResourcesMu.Lock()
	fns := make([]func(string) error, 0, len(sessionCleanups))
	for _, fn := range sessionCleanups {
		fns = append(fns, fn)
	}
	sessionResourcesMu.Unlock()

	var errs []error
	for _, fn := range fns {
		if err := fn(sessionID); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
