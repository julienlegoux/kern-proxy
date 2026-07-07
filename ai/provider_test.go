package ai

// Ports the Models/createProvider behavior from packages/ai/src/models.ts.

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func staticAuth() ProviderAuth {
	return ProviderAuth{APIKey: &APIKeyAuth{
		Name: "test",
		Resolve: func(context.Context, APIKeyResolveInput) (*AuthResult, error) {
			return &AuthResult{Auth: ModelAuth{APIKey: "resolved-key"}, Source: "test"}, nil
		},
	}}
}

func echoStreams(record *[]*StreamOptions) ProviderStreams {
	return StreamFuncs{
		StreamFunc: func(ctx context.Context, model *Model, chat Context, opts *StreamOptions) *Stream {
			if record != nil {
				*record = append(*record, opts)
			}
			s := NewStream()
			msg := &AssistantMessage{Api: model.Api, Provider: model.Provider, Model: model.ID, StopReason: StopReasonStop}
			s.Push(StartEvent{Partial: msg})
			s.Push(DoneEvent{Reason: StopReasonStop, Message: msg})
			return s
		},
		StreamSimpleFunc: func(ctx context.Context, model *Model, chat Context, opts *SimpleStreamOptions) *Stream {
			s := NewStream()
			msg := &AssistantMessage{Api: model.Api, Provider: model.Provider, Model: model.ID, StopReason: StopReasonStop}
			s.Push(DoneEvent{Reason: StopReasonStop, Message: msg})
			return s
		},
	}
}

func testModel(provider, id, api string) *Model {
	return &Model{ID: id, Provider: provider, Api: api, BaseURL: "http://localhost:0"}
}

func TestModelsRegistryLookup(t *testing.T) {
	models := CreateModels(nil)
	m1 := testModel("p1", "m1", "api-a")
	m2 := testModel("p1", "m2", "api-a")
	provider := CreateProvider(CreateProviderOptions{
		ID: "p1", Auth: staticAuth(), Models: []*Model{m1, m2}, Api: echoStreams(nil),
	})
	models.SetProvider(provider)

	if got := models.GetModel("p1", "m2"); got != m2 {
		t.Errorf("GetModel = %#v", got)
	}
	if got := models.GetModel("p1", "nope"); got != nil {
		t.Errorf("expected nil for unknown model, got %#v", got)
	}
	if got := models.GetModel("unknown", "m1"); got != nil {
		t.Errorf("expected nil for unknown provider, got %#v", got)
	}
	if got := len(models.GetModels("")); got != 2 {
		t.Errorf("all models = %d", got)
	}

	models.DeleteProvider("p1")
	if got := len(models.GetModels("")); got != 0 {
		t.Errorf("models after delete = %d", got)
	}
}

func TestUnknownProviderYieldsInBandStreamError(t *testing.T) {
	models := CreateModels(nil)
	model := testModel("ghost", "m", "api")
	result, err := models.Complete(context.Background(), model, Context{}, nil)
	if err != nil {
		t.Fatalf("Complete must not fail: %v", err)
	}
	if result.StopReason != StopReasonError {
		t.Errorf("stopReason = %v", result.StopReason)
	}
	if result.ErrorMessage == "" || result.Model != "m" {
		t.Errorf("message = %+v", result)
	}
}

func TestProviderWithoutApiForModelYieldsStreamError(t *testing.T) {
	models := CreateModels(nil)
	model := testModel("p1", "m1", "api-b")
	provider := CreateProvider(CreateProviderOptions{
		ID: "p1", Auth: staticAuth(), Models: []*Model{model},
		ApiByProtocol: map[Api]ProviderStreams{"api-a": echoStreams(nil)},
	})
	models.SetProvider(provider)

	result, err := models.Complete(context.Background(), model, Context{}, nil)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if result.StopReason != StopReasonError {
		t.Errorf("stopReason = %v", result.StopReason)
	}
}

func TestApplyAuthInjectsAPIKeyAndMergesHeaders(t *testing.T) {
	var recorded []*StreamOptions
	models := CreateModels(nil)
	model := testModel("p1", "m1", "api-a")
	provider := CreateProvider(CreateProviderOptions{
		ID: "p1",
		Auth: ProviderAuth{APIKey: &APIKeyAuth{
			Name: "test",
			Resolve: func(context.Context, APIKeyResolveInput) (*AuthResult, error) {
				return &AuthResult{Auth: ModelAuth{
					APIKey:  "resolved-key",
					Headers: ProviderHeaders{"x-auth-default": HeaderValue("from-auth")},
				}}, nil
			},
		}},
		Models: []*Model{model},
		Api:    echoStreams(&recorded),
	})
	models.SetProvider(provider)

	_, err := models.Complete(context.Background(), model, Context{}, &StreamOptions{
		Headers: ProviderHeaders{"x-user": HeaderValue("mine")},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if len(recorded) != 1 {
		t.Fatalf("stream calls = %d", len(recorded))
	}
	opts := recorded[0]
	if opts.APIKey != "resolved-key" {
		t.Errorf("apiKey = %q", opts.APIKey)
	}
	if v := opts.Headers["x-auth-default"]; v == nil || *v != "from-auth" {
		t.Errorf("auth header lost: %#v", opts.Headers)
	}
	if v := opts.Headers["x-user"]; v == nil || *v != "mine" {
		t.Errorf("user header lost: %#v", opts.Headers)
	}

	// Explicit options win per field.
	recorded = nil
	_, err = models.Complete(context.Background(), model, Context{}, &StreamOptions{APIKey: "explicit"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if recorded[0].APIKey != "explicit" {
		t.Errorf("apiKey = %q, want explicit override", recorded[0].APIKey)
	}
}

func TestAuthBaseURLOverridesModel(t *testing.T) {
	models := CreateModels(nil)
	model := testModel("p1", "m1", "api-a")
	var seenBaseURL string
	provider := CreateProvider(CreateProviderOptions{
		ID: "p1",
		Auth: ProviderAuth{APIKey: &APIKeyAuth{
			Name: "test",
			Resolve: func(context.Context, APIKeyResolveInput) (*AuthResult, error) {
				return &AuthResult{Auth: ModelAuth{APIKey: "k", BaseURL: "https://proxy.example"}}, nil
			},
		}},
		Models: []*Model{model},
		Api: StreamFuncs{
			StreamFunc: func(ctx context.Context, m *Model, chat Context, opts *StreamOptions) *Stream {
				seenBaseURL = m.BaseURL
				s := NewStream()
				s.Push(DoneEvent{Reason: StopReasonStop, Message: &AssistantMessage{StopReason: StopReasonStop}})
				return s
			},
			StreamSimpleFunc: func(ctx context.Context, m *Model, chat Context, opts *SimpleStreamOptions) *Stream {
				s := NewStream()
				s.Push(DoneEvent{Reason: StopReasonStop, Message: &AssistantMessage{StopReason: StopReasonStop}})
				return s
			},
		},
	})
	models.SetProvider(provider)

	if _, err := models.Complete(context.Background(), model, Context{}, nil); err != nil {
		t.Fatal(err)
	}
	if seenBaseURL != "https://proxy.example" {
		t.Errorf("baseUrl = %q", seenBaseURL)
	}
	if model.BaseURL != "http://localhost:0" {
		t.Errorf("original model mutated: %q", model.BaseURL)
	}
}

func TestRefreshDedupesInflightAndUpdatesModels(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	provider := CreateProvider(CreateProviderOptions{
		ID: "dyn", Auth: staticAuth(), Models: nil,
		RefreshModels: func(ctx context.Context) ([]*Model, error) {
			calls.Add(1)
			<-release
			return []*Model{testModel("dyn", "fetched", "api-a")}, nil
		},
		Api: echoStreams(nil),
	})

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = provider.RefreshModels(context.Background())
		}()
	}
	// Let every goroutine reach RefreshModels while the first fetch is still
	// blocked, so all five calls are concurrent (only concurrent calls share
	// the in-flight fetch).
	time.Sleep(100 * time.Millisecond)
	close(release)
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Errorf("refresh calls = %d, want 1 (deduped)", got)
	}
	if len(provider.GetModels()) != 1 || provider.GetModels()[0].ID != "fetched" {
		t.Errorf("models = %#v", provider.GetModels())
	}
}

func TestRefreshFailureKeepsLastKnownModels(t *testing.T) {
	initial := testModel("dyn", "initial", "api-a")
	provider := CreateProvider(CreateProviderOptions{
		ID: "dyn", Auth: staticAuth(), Models: []*Model{initial},
		RefreshModels: func(ctx context.Context) ([]*Model, error) {
			return nil, fmt.Errorf("network down")
		},
		Api: echoStreams(nil),
	})
	models := CreateModels(nil)
	models.SetProvider(provider)

	err := models.Refresh(context.Background(), "dyn")
	var me *ModelsError
	if !errors.As(err, &me) || me.ErrCode != ModelsErrorModelSource {
		t.Errorf("err = %v", err)
	}
	if len(provider.GetModels()) != 1 || provider.GetModels()[0] != initial {
		t.Errorf("models changed on failure: %#v", provider.GetModels())
	}

	// Refresh-all is best-effort and never fails.
	if err := models.Refresh(context.Background(), ""); err != nil {
		t.Errorf("refresh all = %v", err)
	}
	// Static providers are no-ops.
	if err := models.Refresh(context.Background(), "unknown"); err != nil {
		t.Errorf("unknown provider refresh = %v", err)
	}
}

func TestGetAuthUnknownProviderReturnsNil(t *testing.T) {
	models := CreateModels(nil)
	result, err := models.GetAuth(context.Background(), testModel("ghost", "m", "api"))
	if result != nil || err != nil {
		t.Errorf("got %v, %v", result, err)
	}
}
