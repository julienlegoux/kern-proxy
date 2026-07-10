package images

// Ports: packages/ai/src/images-api-registry.ts, packages/ai/src/images.ts.

import (
	"context"
	"fmt"
	"sync"

	"github.com/julienlegoux/kern-link/ai"
)

var (
	registryMu sync.RWMutex
	registry   = map[ai.Api]Function{}
)

// RegisterAPIProvider registers the adapter for api, replacing any existing
// registration for the same api (last write wins, matching upstream's
// Map.set semantics in images-api-registry.ts). Adapter packages call this
// from an init() (see openrouter.go), the direct-registration equivalent of
// upstream's providers/images/register-builtins.ts side-effect import — Go
// has no code-splitting/lazy-import concern to preserve, so no lazy wrapper
// is ported (matching the ai/api/lazy.ts deviation already noted in
// docs/PORTING.md).
func RegisterAPIProvider(api ai.Api, fn Function) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[api] = fn
}

// GetAPIProvider looks up the registered adapter for api, or nil.
func GetAPIProvider(api ai.Api) Function {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry[api]
}

// GenerateImages dispatches model.Api to its registered adapter: the direct,
// auth-agnostic entrypoint (TS images.ts's generateImages). Callers that need
// auth resolved through a CredentialStore should go through a Models
// collection's GenerateImages instead (provider.go).
//
// Deviation: upstream throws synchronously when no adapter is registered for
// model.Api (a caller/config bug, not a request failure). This port folds
// that case into the same in-band AssistantImages{StopReason: "error"}
// result every adapter already promises never to (Go-)error on, keeping one
// consistent "never fails" contract across this package; no ported test
// exercises the alternate branch.
func GenerateImages(ctx context.Context, model *Model, imgCtx Context, opts *Options) *AssistantImages {
	fn := GetAPIProvider(model.Api)
	if fn == nil {
		return errorResult(model, fmt.Sprintf("No API provider registered for api: %s", model.Api))
	}
	return fn(ctx, model, imgCtx, opts)
}
