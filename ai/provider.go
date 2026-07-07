package ai

// Ports: packages/ai/src/models.ts (Provider, Models, createModels,
// createProvider)

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Provider is the concrete runtime unit: it owns id/name/base metadata, auth
// methods, model listing, and stream behavior.
type Provider interface {
	ID() string
	Name() string
	BaseURL() string
	Headers() ProviderHeaders

	// Auth is required: at least one of APIKey/OAuth. Even providers with
	// only ambient credentials provide APIKey auth whose Resolve reports
	// whether the provider is configured.
	Auth() ProviderAuth

	// GetModels returns current known models. Static providers return their
	// catalog; dynamic providers return the list as of the last
	// RefreshModels (empty before the first). Must not fail.
	GetModels() []*Model

	// RefreshModels lets dynamic providers fetch and update the model list.
	// Static providers return nil immediately. Concurrent calls share one
	// in-flight fetch. On failure the model list stays at its last-known
	// state and a later call retries.
	RefreshModels(ctx context.Context) error

	// CanRefreshModels reports whether this is a dynamic provider.
	CanRefreshModels() bool

	Stream(ctx context.Context, model *Model, chat Context, opts *StreamOptions) *Stream
	StreamSimple(ctx context.Context, model *Model, chat Context, opts *SimpleStreamOptions) *Stream
}

// Models is a runtime collection of providers plus auth application and
// stream convenience. Providers own stream behavior; Models resolves auth and
// delegates each request to the provider that owns the model.
type Models interface {
	GetProviders() []Provider
	GetProvider(id string) Provider

	// GetModels reads last-known models from one provider (by id) or all
	// providers (empty id). Best-effort.
	GetModels(provider string) []*Model

	// GetModel is a sync runtime lookup against last-known lists.
	GetModel(provider, id string) *Model

	// Refresh asks dynamic providers to re-fetch their model lists. With a
	// provider id it fails with a ModelsError ("model_source") on that
	// provider's fetch failure; with "" it refreshes all providers
	// concurrently best-effort. Static providers are no-ops.
	Refresh(ctx context.Context, provider string) error

	// GetAuth resolves request auth for a model, including a source label for
	// status UI. Returns (nil, nil) when the provider is unknown or
	// unconfigured. Fails with a ModelsError: code "oauth" when a token
	// refresh fails (the stored credential is preserved for retry; re-login
	// fixes it), code "auth" when api-key resolution or the credential store
	// fails.
	GetAuth(ctx context.Context, model *Model) (*AuthResult, error)

	Stream(ctx context.Context, model *Model, chat Context, opts *StreamOptions) *Stream
	Complete(ctx context.Context, model *Model, chat Context, opts *StreamOptions) (*AssistantMessage, error)
	StreamSimple(ctx context.Context, model *Model, chat Context, opts *SimpleStreamOptions) *Stream
	CompleteSimple(ctx context.Context, model *Model, chat Context, opts *SimpleStreamOptions) (*AssistantMessage, error)
}

// MutableModels extends Models with provider registration.
type MutableModels interface {
	Models
	// SetProvider upserts/replaces by Provider.ID.
	SetProvider(provider Provider)
	DeleteProvider(id string)
	ClearProviders()
}

// CreateModelsOptions configures a Models collection.
type CreateModelsOptions struct {
	Credentials CredentialStore
	AuthContext AuthContext
}

type modelsImpl struct {
	mu          sync.RWMutex
	providers   map[string]Provider
	order       []string
	credentials CredentialStore
	authContext AuthContext
}

// CreateModels builds an empty mutable collection with an in-memory
// credential store and the default process-env auth context unless overridden.
func CreateModels(options *CreateModelsOptions) MutableModels {
	m := &modelsImpl{providers: make(map[string]Provider)}
	if options != nil && options.Credentials != nil {
		m.credentials = options.Credentials
	} else {
		m.credentials = NewInMemoryCredentialStore()
	}
	if options != nil && options.AuthContext != nil {
		m.authContext = options.AuthContext
	} else {
		m.authContext = DefaultAuthContext()
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
			var me *ModelsError
			if errors.As(err, &me) {
				return err
			}
			return NewModelsError(ModelsErrorModelSource, fmt.Sprintf("Model refresh failed for %s", provider), err)
		}
		return nil
	}

	// Refresh all providers concurrently, best-effort.
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

func (m *modelsImpl) GetAuth(ctx context.Context, model *Model) (*AuthResult, error) {
	provider := m.GetProvider(model.Provider)
	if provider == nil {
		return nil, nil
	}
	return ResolveProviderAuth(ctx, provider.ID(), provider.Auth(), model, m.credentials, m.authContext, nil)
}

func (m *modelsImpl) requireProvider(model *Model) (Provider, error) {
	provider := m.GetProvider(model.Provider)
	if provider == nil {
		return nil, NewModelsError(ModelsErrorProvider, fmt.Sprintf("Unknown provider: %s", model.Provider), nil)
	}
	return provider, nil
}

// applyAuth resolves auth and merges it into the request model/options.
// Explicit request options win per field; headers/env merge per key.
func (m *modelsImpl) applyAuth(ctx context.Context, model *Model, opts *StreamOptions) (*Model, *StreamOptions, error) {
	provider, err := m.requireProvider(model)
	if err != nil {
		return nil, nil, err
	}
	var overrides *AuthResolutionOverrides
	if opts != nil && (opts.APIKey != "" || opts.Env != nil) {
		overrides = &AuthResolutionOverrides{Env: opts.Env}
		if opts.APIKey != "" {
			key := opts.APIKey
			overrides.APIKey = &key
		}
	}
	resolution, err := ResolveProviderAuth(ctx, provider.ID(), provider.Auth(), model, m.credentials, m.authContext, overrides)
	if err != nil {
		return nil, nil, err
	}
	if resolution == nil {
		return model, opts, nil
	}
	auth := resolution.Auth

	requestModel := model
	if auth.BaseURL != "" {
		clone := *model
		clone.BaseURL = auth.BaseURL
		requestModel = &clone
	}

	var requestOptions StreamOptions
	if opts != nil {
		requestOptions = *opts
	}
	if requestOptions.APIKey == "" {
		requestOptions.APIKey = auth.APIKey
	}
	if auth.Headers != nil || requestOptions.Headers != nil {
		merged := make(ProviderHeaders, len(auth.Headers)+len(requestOptions.Headers))
		for k, v := range auth.Headers {
			merged[k] = v
		}
		for k, v := range requestOptions.Headers {
			merged[k] = v
		}
		requestOptions.Headers = merged
	}
	if resolution.Env != nil || requestOptions.Env != nil {
		merged := make(ProviderEnv, len(resolution.Env)+len(requestOptions.Env))
		for k, v := range resolution.Env {
			merged[k] = v
		}
		for k, v := range requestOptions.Env {
			merged[k] = v
		}
		requestOptions.Env = merged
	}

	return requestModel, &requestOptions, nil
}

func (m *modelsImpl) Stream(ctx context.Context, model *Model, chat Context, opts *StreamOptions) *Stream {
	return LazyStream(ctx, model, func(ctx context.Context) (*Stream, error) {
		provider, err := m.requireProvider(model)
		if err != nil {
			return nil, err
		}
		requestModel, requestOptions, err := m.applyAuth(ctx, model, opts)
		if err != nil {
			return nil, err
		}
		return provider.Stream(ctx, requestModel, chat, requestOptions), nil
	})
}

func (m *modelsImpl) Complete(ctx context.Context, model *Model, chat Context, opts *StreamOptions) (*AssistantMessage, error) {
	return m.Stream(ctx, model, chat, opts).Result(ctx)
}

func (m *modelsImpl) StreamSimple(ctx context.Context, model *Model, chat Context, opts *SimpleStreamOptions) *Stream {
	return LazyStream(ctx, model, func(ctx context.Context) (*Stream, error) {
		provider, err := m.requireProvider(model)
		if err != nil {
			return nil, err
		}
		var base *StreamOptions
		if opts != nil {
			base = &opts.StreamOptions
		}
		requestModel, requestOptions, err := m.applyAuth(ctx, model, base)
		if err != nil {
			return nil, err
		}
		requestSimple := &SimpleStreamOptions{}
		if opts != nil {
			*requestSimple = *opts
		}
		if requestOptions != nil {
			requestSimple.StreamOptions = *requestOptions
		}
		return provider.StreamSimple(ctx, requestModel, chat, requestSimple), nil
	})
}

func (m *modelsImpl) CompleteSimple(ctx context.Context, model *Model, chat Context, opts *SimpleStreamOptions) (*AssistantMessage, error) {
	return m.StreamSimple(ctx, model, chat, opts).Result(ctx)
}

// CreateProviderOptions configures CreateProvider.
type CreateProviderOptions struct {
	ID string
	// Name defaults to ID.
	Name    string
	BaseURL string
	Headers ProviderHeaders
	// Auth is required — every provider has auth semantics, even
	// ambient/keyless ones.
	Auth ProviderAuth
	// Models is the initial model list (empty for purely dynamic providers).
	Models []*Model
	// RefreshModels makes the provider dynamic: it fetches the current list.
	// Concurrent calls share one in-flight fetch; on failure the stored list
	// stays at its last-known state.
	RefreshModels func(ctx context.Context) ([]*Model, error)
	// Api is the single implementation streaming all models; ApiByProtocol
	// dispatches on model.Api for mixed-API providers. Exactly one must be
	// set. A model whose api has no entry produces a stream error.
	Api           ProviderStreams
	ApiByProtocol map[Api]ProviderStreams
}

type providerImpl struct {
	opts CreateProviderOptions

	modelsMu sync.RWMutex
	models   []*Model

	refreshMu sync.Mutex
	inflight  chan struct{}
	lastErr   error
}

// CreateProvider builds a provider from parts. Built-in provider factories
// and custom providers both go through this.
func CreateProvider(opts CreateProviderOptions) Provider {
	p := &providerImpl{opts: opts, models: opts.Models}
	return p
}

func (p *providerImpl) ID() string { return p.opts.ID }

func (p *providerImpl) Name() string {
	if p.opts.Name != "" {
		return p.opts.Name
	}
	return p.opts.ID
}

func (p *providerImpl) BaseURL() string          { return p.opts.BaseURL }
func (p *providerImpl) Headers() ProviderHeaders { return p.opts.Headers }
func (p *providerImpl) Auth() ProviderAuth       { return p.opts.Auth }

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

func (p *providerImpl) apiFor(model *Model) ProviderStreams {
	if p.opts.Api != nil {
		return p.opts.Api
	}
	return p.opts.ApiByProtocol[model.Api]
}

func (p *providerImpl) dispatch(
	ctx context.Context,
	model *Model,
	run func(streams ProviderStreams) *Stream,
) *Stream {
	streams := p.apiFor(model)
	if streams == nil {
		return LazyStream(ctx, model, func(context.Context) (*Stream, error) {
			return nil, NewModelsError(ModelsErrorStream,
				fmt.Sprintf("Provider %s has no API implementation for %q", p.opts.ID, model.Api), nil)
		})
	}
	return run(streams)
}

func (p *providerImpl) Stream(ctx context.Context, model *Model, chat Context, opts *StreamOptions) *Stream {
	return p.dispatch(ctx, model, func(streams ProviderStreams) *Stream {
		return streams.Stream(ctx, model, chat, opts)
	})
}

func (p *providerImpl) StreamSimple(ctx context.Context, model *Model, chat Context, opts *SimpleStreamOptions) *Stream {
	return p.dispatch(ctx, model, func(streams ProviderStreams) *Stream {
		return streams.StreamSimple(ctx, model, chat, opts)
	})
}

// StreamFuncs adapts a pair of stream functions to the ProviderStreams
// interface (the Go stand-in for a TS api module's exports).
type StreamFuncs struct {
	StreamFunc       StreamFunc
	StreamSimpleFunc SimpleStreamFunc
}

func (f StreamFuncs) Stream(ctx context.Context, model *Model, chat Context, opts *StreamOptions) *Stream {
	return f.StreamFunc(ctx, model, chat, opts)
}

func (f StreamFuncs) StreamSimple(ctx context.Context, model *Model, chat Context, opts *SimpleStreamOptions) *Stream {
	return f.StreamSimpleFunc(ctx, model, chat, opts)
}
