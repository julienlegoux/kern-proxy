package images

// Ports: packages/ai/src/images-models.ts

import (
	"context"
	"fmt"
	"sync"

	"github.com/julienlegoux/kern-proxy/ai"
)

// Provider is an image-generation provider: the image-side counterpart of
// ai.Provider. It owns id/name metadata, auth, model listing, and generation
// behavior.
type Provider interface {
	ID() string
	Name() string

	// Auth is required: at least one of APIKey/OAuth. Same semantics as chat
	// providers; Models.GetAuth returns (nil, nil) when the provider is
	// unconfigured.
	Auth() ai.ProviderAuth

	// GetModels returns current known models. Static providers return their
	// catalog; dynamic providers return the list as of the last
	// RefreshModels (empty before the first).
	GetModels() []*Model

	// RefreshModels lets dynamic providers fetch and update the model list.
	// Static providers return nil immediately. Concurrent calls share one
	// in-flight fetch; on failure the model list stays at its last-known
	// state and a later call retries.
	RefreshModels(ctx context.Context) error

	// CanRefreshModels reports whether this is a dynamic provider.
	CanRefreshModels() bool

	GenerateImages(ctx context.Context, model *Model, imgCtx Context, opts *Options) *AssistantImages
}

// Models is a runtime collection of image-generation providers plus auth
// application and generation convenience: the image-side counterpart of
// ai.Models.
type Models interface {
	GetProviders() []Provider
	GetProvider(id string) Provider

	// GetModels reads last-known models from one provider (by id) or all
	// providers (""). Best-effort: a provider whose GetModels panics is not
	// guarded against (matching ai.Models — Go has no try/catch to make that
	// promise cheaply, and no built-in provider throws).
	GetModels(provider string) []*Model

	// GetModel is a sync runtime lookup against last-known lists.
	GetModel(provider, id string) *Model

	// Refresh asks dynamic providers to re-fetch their model lists. With a
	// provider id it fails with a ModelsError ("model_source") on that
	// provider's fetch failure; with "" it refreshes all providers
	// concurrently best-effort. Static providers are no-ops.
	Refresh(ctx context.Context, provider string) error

	// GetAuth resolves request auth for an image model. Same contract as
	// ai.Models.GetAuth: (nil, nil) when unknown/unconfigured, error
	// (ModelsError) on real failures.
	GetAuth(ctx context.Context, model *Model) (*ai.AuthResult, error)

	// GenerateImages generates through the owning provider with auth
	// resolved and merged (explicit options win per field). Never returns a
	// Go error; failures are returned as an AssistantImages with StopReason
	// "error".
	GenerateImages(ctx context.Context, model *Model, imgCtx Context, opts *Options) *AssistantImages
}

// MutableModels extends Models with provider registration.
type MutableModels interface {
	Models
	// SetProvider upserts/replaces by Provider.ID().
	SetProvider(provider Provider)
	DeleteProvider(id string)
	ClearProviders()
}

type modelsImpl struct {
	mu          sync.RWMutex
	providers   map[string]Provider
	order       []string
	credentials ai.CredentialStore
	authContext ai.AuthContext
}

// CreateModels builds an empty mutable collection, reusing ai.CreateModelsOptions
// (Credentials/AuthContext) so callers share the same options shape as the
// chat-side ai.CreateModels.
func CreateModels(options *ai.CreateModelsOptions) MutableModels {
	m := &modelsImpl{providers: make(map[string]Provider)}
	if options != nil && options.Credentials != nil {
		m.credentials = options.Credentials
	} else {
		m.credentials = ai.NewInMemoryCredentialStore()
	}
	if options != nil && options.AuthContext != nil {
		m.authContext = options.AuthContext
	} else {
		m.authContext = ai.DefaultAuthContext()
	}
	return m
}

func (m *modelsImpl) SetProvider(provider Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.providers[provider.ID()]; !exists {
		m.order = append(m.order, provider.ID())
	}
	m.providers[provider.ID()] = provider
}

func (m *modelsImpl) DeleteProvider(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.providers[id]; exists {
		delete(m.providers, id)
		for i, existing := range m.order {
			if existing == id {
				m.order = append(m.order[:i], m.order[i+1:]...)
				break
			}
		}
	}
}

func (m *modelsImpl) ClearProviders() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers = make(map[string]Provider)
	m.order = nil
}

func (m *modelsImpl) GetProviders() []Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Provider, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, m.providers[id])
	}
	return out
}

func (m *modelsImpl) GetProvider(id string) Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.providers[id]
}

func (m *modelsImpl) GetModels(provider string) []*Model {
	if provider != "" {
		entry := m.GetProvider(provider)
		if entry == nil {
			return nil
		}
		return entry.GetModels()
	}
	var models []*Model
	for _, entry := range m.GetProviders() {
		models = append(models, entry.GetModels()...)
	}
	return models
}

func (m *modelsImpl) GetModel(provider, id string) *Model {
	for _, model := range m.GetModels(provider) {
		if model.ID == id {
			return model
		}
	}
	return nil
}

func (m *modelsImpl) Refresh(ctx context.Context, provider string) error {
	if provider != "" {
		entry := m.GetProvider(provider)
		if entry == nil || !entry.CanRefreshModels() {
			return nil
		}
		if err := entry.RefreshModels(ctx); err != nil {
			var me *ai.ModelsError
			if isModelsError(err, &me) {
				return err
			}
			return ai.NewModelsError(ai.ModelsErrorModelSource, fmt.Sprintf("Model refresh failed for %s", provider), err)
		}
		return nil
	}

	providers := m.GetProviders()
	var wg sync.WaitGroup
	for _, entry := range providers {
		if !entry.CanRefreshModels() {
			continue
		}
		wg.Add(1)
		go func(p Provider) {
			defer wg.Done()
			_ = p.RefreshModels(ctx)
		}(entry)
	}
	wg.Wait()
	return nil
}

func (m *modelsImpl) GetAuth(ctx context.Context, model *Model) (*ai.AuthResult, error) {
	provider := m.GetProvider(model.Provider)
	if provider == nil {
		return nil, nil
	}
	return ai.ResolveProviderAuth(ctx, provider.ID(), provider.Auth(), toShimModel(model), m.credentials, m.authContext, nil)
}

func (m *modelsImpl) GenerateImages(ctx context.Context, model *Model, imgCtx Context, opts *Options) *AssistantImages {
	provider := m.GetProvider(model.Provider)
	if provider == nil {
		return errorResult(model, fmt.Sprintf("Unknown provider: %s", model.Provider))
	}

	var overrides *ai.AuthResolutionOverrides
	var apiKey string
	var optEnv ai.ProviderEnv
	if opts != nil {
		apiKey = opts.APIKey
		optEnv = opts.Env
	}
	if apiKey != "" || optEnv != nil {
		overrides = &ai.AuthResolutionOverrides{Env: optEnv}
		if apiKey != "" {
			k := apiKey
			overrides.APIKey = &k
		}
	}

	resolution, err := ai.ResolveProviderAuth(ctx, provider.ID(), provider.Auth(), toShimModel(model), m.credentials, m.authContext, overrides)
	if err != nil {
		return errorResult(model, err.Error())
	}
	if resolution == nil {
		return provider.GenerateImages(ctx, model, imgCtx, opts)
	}

	requestModel := model
	if resolution.Auth.BaseURL != "" {
		clone := *model
		clone.BaseURL = resolution.Auth.BaseURL
		requestModel = &clone
	}

	var requestOptions Options
	if opts != nil {
		requestOptions = *opts
	}
	if requestOptions.APIKey == "" {
		requestOptions.APIKey = resolution.Auth.APIKey
	}
	if resolution.Auth.Headers != nil || requestOptions.Headers != nil {
		merged := make(ai.ProviderHeaders, len(resolution.Auth.Headers)+len(requestOptions.Headers))
		for k, v := range resolution.Auth.Headers {
			merged[k] = v
		}
		for k, v := range requestOptions.Headers {
			merged[k] = v
		}
		requestOptions.Headers = merged
	}
	if resolution.Env != nil || requestOptions.Env != nil {
		merged := make(ai.ProviderEnv, len(resolution.Env)+len(requestOptions.Env))
		for k, v := range resolution.Env {
			merged[k] = v
		}
		for k, v := range requestOptions.Env {
			merged[k] = v
		}
		requestOptions.Env = merged
	}

	return provider.GenerateImages(ctx, requestModel, imgCtx, &requestOptions)
}

// toShimModel adapts an images.Model to an *ai.Model carrying just the
// overlapping fields, for ai.ResolveProviderAuth's model parameter (auth.go/
// resolve.go are read-only context for this package: they take a concrete
// *ai.Model, and the ported resolvers we exercise — env-key lookups — only
// ever read Model.Provider, so this shim never needs to be exhaustive).
func toShimModel(m *Model) *ai.Model {
	if m == nil {
		return nil
	}
	return &ai.Model{
		ID:       m.ID,
		Name:     m.Name,
		Api:      m.Api,
		Provider: m.Provider,
		BaseURL:  m.BaseURL,
		Input:    m.Input,
		Cost:     m.Cost,
		Headers:  m.Headers,
	}
}

func isModelsError(err error, target **ai.ModelsError) bool {
	me, ok := err.(*ai.ModelsError)
	if ok {
		*target = me
	}
	return ok
}

// CreateProviderOptions configures CreateProvider.
type CreateProviderOptions struct {
	ID string
	// Name defaults to ID.
	Name string
	// Auth is required — every provider has auth semantics, even
	// ambient/keyless ones.
	Auth ai.ProviderAuth
	// Models is the initial model list (empty for purely dynamic providers).
	Models []*Model
	// RefreshModels makes the provider dynamic: it fetches the current list.
	// Concurrent calls share one in-flight fetch; on failure the stored list
	// stays at its last-known state.
	RefreshModels func(ctx context.Context) ([]*Model, error)
	// API is the adapter function backing GenerateImages.
	API Function
}

type providerImpl struct {
	opts CreateProviderOptions

	modelsMu sync.RWMutex
	models   []*Model

	refreshMu sync.Mutex
	inflight  chan struct{}
	lastErr   error
}

// CreateProvider builds an image-generation provider from parts.
func CreateProvider(opts CreateProviderOptions) Provider {
	return &providerImpl{opts: opts, models: opts.Models}
}

func (p *providerImpl) ID() string { return p.opts.ID }

func (p *providerImpl) Name() string {
	if p.opts.Name != "" {
		return p.opts.Name
	}
	return p.opts.ID
}

func (p *providerImpl) Auth() ai.ProviderAuth { return p.opts.Auth }

func (p *providerImpl) GetModels() []*Model {
	p.modelsMu.RLock()
	defer p.modelsMu.RUnlock()
	return p.models
}

func (p *providerImpl) CanRefreshModels() bool { return p.opts.RefreshModels != nil }

func (p *providerImpl) RefreshModels(ctx context.Context) error {
	if p.opts.RefreshModels == nil {
		return nil
	}

	p.refreshMu.Lock()
	if p.inflight != nil {
		wait := p.inflight
		p.refreshMu.Unlock()
		<-wait
		p.refreshMu.Lock()
		err := p.lastErr
		p.refreshMu.Unlock()
		return err
	}
	done := make(chan struct{})
	p.inflight = done
	p.refreshMu.Unlock()

	models, err := p.opts.RefreshModels(ctx)
	if err == nil {
		p.modelsMu.Lock()
		p.models = models
		p.modelsMu.Unlock()
	}

	p.refreshMu.Lock()
	p.lastErr = err
	p.inflight = nil
	p.refreshMu.Unlock()
	close(done)
	return err
}

func (p *providerImpl) GenerateImages(ctx context.Context, model *Model, imgCtx Context, opts *Options) *AssistantImages {
	return p.opts.API(ctx, model, imgCtx, opts)
}
