package mistral

// New Go tests covering request building (buildParams/buildHeaders) and
// auth assertion, matching the fidelity bar of the sibling anthropic/google
// packages (upstream has no dedicated request-building test file for
// mistral-conversations.ts either -- its fidelity is exercised indirectly
// through higher-level agent/session tests and the two Mistral-specific test
// files ported into reasoning_test.go and messages_test.go).

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kern-ia/kern-link/ai"
)

func testModel(baseURL string) *ai.Model {
	return &ai.Model{
		ID:            "mistral-large-latest",
		Name:          "Mistral Large",
		Api:           ai.ApiMistralConversations,
		Provider:      "mistral",
		BaseURL:       baseURL,
		Input:         []ai.Modality{ai.ModalityText, ai.ModalityImage},
		ContextWindow: 128000,
		MaxTokens:     4096,
	}
}

func testChat() ai.Context {
	return ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
}

func TestBuildParams_BasicRequestShape(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{
		SystemPrompt: "be helpful",
		Messages:     []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}},
	}
	temp := 0.5
	maxTokens := 100
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "k", Temperature: &temp, MaxTokens: &maxTokens})

	if params.Model != "mistral-large-latest" {
		t.Errorf("model = %q, want mistral-large-latest", params.Model)
	}
	if !params.Stream {
		t.Errorf("stream = false, want true")
	}
	if len(params.Messages) != 2 {
		t.Fatalf("messages = %#v, want 2 (system + user)", params.Messages)
	}
	if params.Messages[0].Role != "system" {
		t.Errorf("messages[0].role = %q, want system", params.Messages[0].Role)
	}
	if params.Temperature == nil || *params.Temperature != 0.5 {
		t.Errorf("temperature = %v, want 0.5", params.Temperature)
	}
	if params.MaxTokens == nil || *params.MaxTokens != 100 {
		t.Errorf("maxTokens = %v, want 100", params.MaxTokens)
	}
	if params.Tools != nil {
		t.Errorf("tools = %#v, want nil when no tools offered", params.Tools)
	}
}

func TestBuildParams_ToolsIncludedWhenOffered(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := testChat()
	chat.Tools = []ai.Tool{{Name: "noop", Description: "does nothing", Parameters: json.RawMessage(`{"type":"object"}`)}}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "k"})
	if len(params.Tools) != 1 {
		t.Fatalf("tools = %#v, want 1", params.Tools)
	}
	if params.Tools[0].Function.Name != "noop" {
		t.Errorf("tools[0].function.name = %q, want noop", params.Tools[0].Function.Name)
	}
}

func TestBuildParams_ToolChoicePassthrough(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := testChat()
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "k", MistralToolChoice: "none"})
	if params.ToolChoice == nil {
		t.Fatalf("toolChoice = nil, want set")
	}
	s, ok := params.ToolChoice.(string)
	if !ok || s != "none" {
		t.Errorf("toolChoice = %#v, want \"none\"", params.ToolChoice)
	}
}

func TestBuildParams_ToolChoiceFunctionOverridesString(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := testChat()
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "k", MistralToolChoice: "auto", MistralToolChoiceFunction: "get_weather"})
	choice, ok := params.ToolChoice.(map[string]any)
	if !ok {
		t.Fatalf("toolChoice = %#v, want named-function map", params.ToolChoice)
	}
	if choice["type"] != "function" {
		t.Errorf("toolChoice.type = %v, want function", choice["type"])
	}
}

func TestBuildParams_PromptCacheKeyFromSessionID(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := testChat()
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "k", SessionID: "session-123"})
	if params.PromptCacheKey != "session-123" {
		t.Errorf("promptCacheKey = %q, want session-123", params.PromptCacheKey)
	}
}

func TestBuildParams_PromptCacheKeyOmittedWhenCacheRetentionNone(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := testChat()
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "k", SessionID: "session-123", CacheRetention: ai.CacheRetentionNone})
	if params.PromptCacheKey != "" {
		t.Errorf("promptCacheKey = %q, want empty", params.PromptCacheKey)
	}
}

func TestBuildParams_PromptModeAndReasoningEffortPassthrough(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := testChat()
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "k", MistralPromptMode: "reasoning", MistralReasoningEffort: "high"})
	if params.PromptMode != "reasoning" {
		t.Errorf("promptMode = %q, want reasoning", params.PromptMode)
	}
	if params.ReasoningEffort != "high" {
		t.Errorf("reasoningEffort = %q, want high", params.ReasoningEffort)
	}
}

func TestBuildHeaders_SetsBearerAuth(t *testing.T) {
	headers := buildHeaders(testModel("https://example.invalid"), nil, "my-key")
	if headers["authorization"] != "Bearer my-key" {
		t.Errorf("headers = %#v, want authorization=Bearer my-key", headers)
	}
	if headers["content-type"] != "application/json" {
		t.Errorf("headers = %#v, want content-type=application/json", headers)
	}
}

func TestBuildHeaders_SetsAffinityHeaderForSessionCaching(t *testing.T) {
	headers := buildHeaders(testModel("https://example.invalid"), &ai.StreamOptions{SessionID: "session-123"}, "my-key")
	if headers["x-affinity"] != "session-123" {
		t.Errorf("headers = %#v, want x-affinity=session-123", headers)
	}
}

func TestBuildHeaders_OmitsAffinityHeaderWhenCacheRetentionNone(t *testing.T) {
	headers := buildHeaders(testModel("https://example.invalid"), &ai.StreamOptions{SessionID: "session-123", CacheRetention: ai.CacheRetentionNone}, "my-key")
	if _, ok := headers["x-affinity"]; ok {
		t.Errorf("headers = %#v, want no x-affinity", headers)
	}
}

func TestAssertRequestAuth_RequiresAPIKey(t *testing.T) {
	if err := assertRequestAuth("mistral", ""); err == nil {
		t.Fatal("assertRequestAuth(\"\") = nil, want error")
	}
	if err := assertRequestAuth("mistral", "k"); err != nil {
		t.Fatalf("assertRequestAuth(\"k\") = %v, want nil", err)
	}
}
