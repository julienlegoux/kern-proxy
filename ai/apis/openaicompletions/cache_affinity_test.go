package openaicompletions

// Ports the remaining upstream openai-completions-* suites this issue closes
// out: test/openai-completions-cache-control-format.test.ts and
// test/openai-completions-prompt-cache.test.ts. Upstream builds its fixture
// models from the generated catalog (getModel), which this Go port does not
// yet embed (Epic 11); these tests construct equivalent model literals
// directly, matching the same style as compat_wiring_test.go and
// thinking_test.go in this package.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
)

// cacheAffinityModel builds an openrouter-style model whose compat can be
// overridden per test, mirroring the upstream cache-control-format test's
// "custom-qwen" fixture.
func cacheAffinityModel(baseURL string, compat *ai.Compat) *ai.Model {
	return &ai.Model{
		ID:            "custom-qwen",
		Name:          "Custom Qwen",
		Api:           ai.ApiOpenAICompletions,
		Provider:      "openrouter",
		BaseURL:       baseURL,
		Reasoning:     true,
		Input:         []ai.Modality{ai.ModalityText},
		ContextWindow: 128000,
		MaxTokens:     32000,
		Compat:        compat,
	}
}

func cacheAffinityChat() ai.Context {
	return ai.Context{
		SystemPrompt: "System prompt",
		Messages:     []ai.Message{ai.UserMessage{Content: ai.UserText("Hello"), Timestamp: time.Now().UnixMilli()}},
		Tools: []ai.Tool{{
			Name:        "read",
			Description: "Read a file",
			Parameters:  []byte(`{"type":"object","properties":{"path":{"type":"string"}}}`),
		}},
	}
}

func findInstructionMessage(messages []wireMessage) *wireMessage {
	for i := range messages {
		if messages[i].Role == "system" || messages[i].Role == "developer" {
			return &messages[i]
		}
	}
	return nil
}

func expectAnthropicCacheMarkers(t *testing.T, params *wireRequest) {
	t.Helper()

	instruction := findInstructionMessage(params.Messages)
	if instruction == nil {
		t.Fatal("no system/developer instruction message found")
	}
	parts, ok := instruction.Content.([]wireContentPart)
	if !ok || len(parts) != 1 {
		t.Fatalf("instruction content = %#v, want a 1-part array", instruction.Content)
	}
	if parts[0].CacheControl == nil || parts[0].CacheControl.Type != "ephemeral" {
		t.Errorf("instruction cache_control = %#v, want {type: ephemeral}", parts[0].CacheControl)
	}

	if params.Tools == nil || len(*params.Tools) != 1 {
		t.Fatalf("tools = %#v, want 1 entry", params.Tools)
	}
	tool := (*params.Tools)[0]
	if tool.CacheControl == nil || tool.CacheControl.Type != "ephemeral" {
		t.Errorf("tool cache_control = %#v, want {type: ephemeral}", tool.CacheControl)
	}

	last := params.Messages[len(params.Messages)-1]
	if last.Role != "user" {
		t.Fatalf("last message role = %q, want user", last.Role)
	}
	lastParts, ok := last.Content.([]wireContentPart)
	if !ok || len(lastParts) != 1 {
		t.Fatalf("last message content = %#v, want a 1-part array", last.Content)
	}
	if lastParts[0].CacheControl == nil || lastParts[0].CacheControl.Type != "ephemeral" {
		t.Errorf("last message cache_control = %#v, want {type: ephemeral}", lastParts[0].CacheControl)
	}
}

// TestBuildParams_AppliesAnthropicCacheMarkersWhenCompatEnabled ports "applies
// Anthropic-style cache markers when model compat enables them".
func TestBuildParams_AppliesAnthropicCacheMarkersWhenCompatEnabled(t *testing.T) {
	model := cacheAffinityModel("https://example.com/v1", &ai.Compat{CacheControlFormat: "anthropic"})
	params := buildParams(model, cacheAffinityChat(), &ai.StreamOptions{APIKey: "sk-test"})
	expectAnthropicCacheMarkers(t, params)
}

// TestBuildParams_OmitsAnthropicCacheMarkersWhenCacheRetentionNone ports
// "omits Anthropic-style cache markers when cacheRetention is none".
func TestBuildParams_OmitsAnthropicCacheMarkersWhenCacheRetentionNone(t *testing.T) {
	model := cacheAffinityModel("https://example.com/v1", &ai.Compat{CacheControlFormat: "anthropic"})
	params := buildParams(model, cacheAffinityChat(), &ai.StreamOptions{APIKey: "sk-test", CacheRetention: ai.CacheRetentionNone})

	instruction := findInstructionMessage(params.Messages)
	if instruction == nil {
		t.Fatal("no system/developer instruction message found")
	}
	if _, isArray := instruction.Content.([]wireContentPart); isArray {
		t.Errorf("instruction content = %#v, want plain string (untouched)", instruction.Content)
	}

	if params.Tools == nil || len(*params.Tools) != 1 {
		t.Fatalf("tools = %#v, want 1 entry", params.Tools)
	}
	if (*params.Tools)[0].CacheControl != nil {
		t.Errorf("tool cache_control = %#v, want nil", (*params.Tools)[0].CacheControl)
	}

	last := params.Messages[len(params.Messages)-1]
	if _, isString := last.Content.(string); !isString {
		t.Errorf("last message content = %#v, want plain string (untouched)", last.Content)
	}
}

// --- prompt_cache_key / prompt_cache_retention ------------------------------

func directOpenAIModel(overrides func(*ai.Model)) *ai.Model {
	m := &ai.Model{
		ID:            "gpt-4o-mini",
		Name:          "GPT-4o mini",
		Api:           ai.ApiOpenAICompletions,
		Provider:      "openai",
		BaseURL:       "https://api.openai.com/v1",
		Input:         []ai.Modality{ai.ModalityText},
		ContextWindow: 128000,
		MaxTokens:     16384,
	}
	if overrides != nil {
		overrides(m)
	}
	return m
}

// TestBuildParams_SetsPromptCacheKeyForDirectOpenAIRequests ports "sets
// prompt_cache_key for direct OpenAI requests when caching is enabled".
func TestBuildParams_SetsPromptCacheKeyForDirectOpenAIRequests(t *testing.T) {
	params := buildParams(directOpenAIModel(nil), simpleChat(), &ai.StreamOptions{APIKey: "sk-test", SessionID: "session-123"})
	if params.PromptCacheKey != "session-123" {
		t.Errorf("prompt_cache_key = %q, want session-123", params.PromptCacheKey)
	}
	if params.PromptCacheRetention != "" {
		t.Errorf("prompt_cache_retention = %q, want empty", params.PromptCacheRetention)
	}
}

// TestBuildParams_SetsPromptCacheRetention24hForLongRetention ports "sets
// prompt_cache_retention to 24h for direct OpenAI requests when
// cacheRetention is long".
func TestBuildParams_SetsPromptCacheRetention24hForLongRetention(t *testing.T) {
	params := buildParams(directOpenAIModel(nil), simpleChat(), &ai.StreamOptions{
		APIKey:         "sk-test",
		CacheRetention: ai.CacheRetentionLong,
		SessionID:      "session-456",
	})
	if params.PromptCacheKey != "session-456" {
		t.Errorf("prompt_cache_key = %q, want session-456", params.PromptCacheKey)
	}
	if params.PromptCacheRetention != "24h" {
		t.Errorf("prompt_cache_retention = %q, want 24h", params.PromptCacheRetention)
	}
}

// TestBuildParams_ClampsPromptCacheKeyTo64Chars ports "clamps prompt_cache_key
// to OpenAI's 64-character limit".
func TestBuildParams_ClampsPromptCacheKeyTo64Chars(t *testing.T) {
	sessionID := ""
	for i := 0; i < 67; i++ {
		sessionID += "x"
	}
	params := buildParams(directOpenAIModel(nil), simpleChat(), &ai.StreamOptions{APIKey: "sk-test", SessionID: sessionID})
	if len(params.PromptCacheKey) != 64 {
		t.Errorf("prompt_cache_key length = %d, want 64", len(params.PromptCacheKey))
	}
}

// TestBuildParams_OmitsPromptCacheFieldsWhenCacheRetentionNone ports "omits
// prompt cache fields when cacheRetention is none".
func TestBuildParams_OmitsPromptCacheFieldsWhenCacheRetentionNone(t *testing.T) {
	params := buildParams(directOpenAIModel(nil), simpleChat(), &ai.StreamOptions{
		APIKey:         "sk-test",
		CacheRetention: ai.CacheRetentionNone,
		SessionID:      "session-789",
	})
	if params.PromptCacheKey != "" {
		t.Errorf("prompt_cache_key = %q, want empty", params.PromptCacheKey)
	}
	if params.PromptCacheRetention != "" {
		t.Errorf("prompt_cache_retention = %q, want empty", params.PromptCacheRetention)
	}
}

// TestBuildParams_OmitsPromptCacheFieldsForNonOpenAIBaseURLWithoutLongRetention
// ports "omits prompt cache fields for non-OpenAI base URLs without
// compatible long retention".
func TestBuildParams_OmitsPromptCacheFieldsForNonOpenAIBaseURLWithoutLongRetention(t *testing.T) {
	no := false
	model := directOpenAIModel(func(m *ai.Model) {
		m.BaseURL = "https://proxy.example.com/v1"
		m.Compat = &ai.Compat{SupportsLongCacheRetention: &no}
	})
	params := buildParams(model, simpleChat(), &ai.StreamOptions{
		APIKey:         "sk-test",
		CacheRetention: ai.CacheRetentionLong,
		SessionID:      "session-proxy",
	})
	if params.PromptCacheKey != "" {
		t.Errorf("prompt_cache_key = %q, want empty", params.PromptCacheKey)
	}
	if params.PromptCacheRetention != "" {
		t.Errorf("prompt_cache_retention = %q, want empty", params.PromptCacheRetention)
	}
}

// TestBuildParams_UsesPICacheRetentionEnvVarForDirectOpenAIRequests ports
// "uses PI_CACHE_RETENTION for direct OpenAI requests".
func TestBuildParams_UsesPICacheRetentionEnvVarForDirectOpenAIRequests(t *testing.T) {
	params := buildParams(directOpenAIModel(nil), simpleChat(), &ai.StreamOptions{
		APIKey:    "sk-test",
		SessionID: "session-env",
		Env:       ai.ProviderEnv{"PI_CACHE_RETENTION": "long"},
	})
	if params.PromptCacheKey != "session-env" {
		t.Errorf("prompt_cache_key = %q, want session-env", params.PromptCacheKey)
	}
	if params.PromptCacheRetention != "24h" {
		t.Errorf("prompt_cache_retention = %q, want 24h", params.PromptCacheRetention)
	}
}

// --- session-affinity headers ----------------------------------------------

func captureHeaders(t *testing.T, model *ai.Model, opts *ai.StreamOptions) http.Header {
	t.Helper()
	var captured http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Clone()
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(writeChunkedSSE(minimalTextChunks())))
	}))
	t.Cleanup(srv.Close)
	model.BaseURL = srv.URL

	if opts == nil {
		opts = &ai.StreamOptions{}
	}
	opts.APIKey = "sk-test"

	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	stream := Stream(context.Background(), model, chat, opts)
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}
	return captured
}

// TestStream_SendsSessionAffinityHeadersWhenCompatEnabled ports "sends known
// session-affinity headers when compat.sendSessionAffinityHeaders is
// enabled".
func TestStream_SendsSessionAffinityHeadersWhenCompatEnabled(t *testing.T) {
	yes := true
	model := directOpenAIModel(func(m *ai.Model) {
		m.Compat = &ai.Compat{SendSessionAffinityHeaders: &yes}
	})
	headers := captureHeaders(t, model, &ai.StreamOptions{SessionID: "session-affinity"})

	for name, want := range map[string]string{
		"session_id":          "session-affinity",
		"x-client-request-id": "session-affinity",
		"x-session-affinity":  "session-affinity",
	} {
		if got := headers.Get(name); got != want {
			t.Errorf("header %q = %q, want %q", name, got, want)
		}
	}
}

// TestStream_OmitsSessionAffinityHeadersWhenCacheRetentionNone ports "omits
// session-affinity headers when cacheRetention is none".
func TestStream_OmitsSessionAffinityHeadersWhenCacheRetentionNone(t *testing.T) {
	yes := true
	model := directOpenAIModel(func(m *ai.Model) {
		m.Compat = &ai.Compat{SendSessionAffinityHeaders: &yes}
	})
	headers := captureHeaders(t, model, &ai.StreamOptions{SessionID: "session-affinity", CacheRetention: ai.CacheRetentionNone})

	for _, name := range []string{"session_id", "x-client-request-id", "x-session-affinity"} {
		if got := headers.Get(name); got != "" {
			t.Errorf("header %q = %q, want empty", name, got)
		}
	}
}

// TestStream_ExplicitHeadersOverrideSessionAffinityHeaders ports "lets
// explicit headers override generated session-affinity headers".
func TestStream_ExplicitHeadersOverrideSessionAffinityHeaders(t *testing.T) {
	yes := true
	model := directOpenAIModel(func(m *ai.Model) {
		m.Compat = &ai.Compat{SendSessionAffinityHeaders: &yes}
	})
	headers := captureHeaders(t, model, &ai.StreamOptions{
		SessionID: "session-affinity",
		Headers: ai.ProviderHeaders{
			"session_id":          ai.HeaderValue("override-session"),
			"x-client-request-id": ai.HeaderValue("override-request"),
			"x-session-affinity":  ai.HeaderValue("override-affinity"),
		},
	})

	for name, want := range map[string]string{
		"session_id":          "override-session",
		"x-client-request-id": "override-request",
		"x-session-affinity":  "override-affinity",
	} {
		if got := headers.Get(name); got != want {
			t.Errorf("header %q = %q, want %q", name, got, want)
		}
	}
}
