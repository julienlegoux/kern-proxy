package codex

// New Go tests (not literal upstream ports): upstream has no dedicated
// request-building test file for openai-codex-responses.ts (its fidelity is
// exercised indirectly through the SSE-level tests in
// openai-codex-stream.test.ts, ported at the Stream level in stream_test.go).
// These cover buildRequestBody/applyReasoning/resolveCodexServiceTier at the
// same fidelity bar as the sibling openairesponses package's params tests.

import (
	"encoding/json"
	"testing"

	"github.com/julienlegoux/kern-proxy/ai"
)

func testModel(baseURL string) *ai.Model {
	return &ai.Model{
		ID:            "gpt-5.1-codex",
		Name:          "GPT-5.1 Codex",
		Api:           ai.ApiOpenAICodexResponses,
		Provider:      "openai-codex",
		BaseURL:       baseURL,
		Reasoning:     true,
		Input:         []ai.Modality{ai.ModalityText},
		ContextWindow: 400000,
		MaxTokens:     128000,
	}
}

func marshalRoundtrip(t *testing.T, v any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return decoded
}

func TestBuildRequestBody_BasicRequestShape(t *testing.T) {
	model := testModel("https://chatgpt.com/backend-api")
	chat := ai.Context{
		SystemPrompt: "be helpful",
		Messages:     []ai.Message{&ai.UserMessage{Content: ai.UserText("hi")}},
	}

	body := buildRequestBody(model, chat, &ai.StreamOptions{APIKey: "token"})
	decoded := marshalRoundtrip(t, body)

	if decoded["model"] != "gpt-5.1-codex" {
		t.Errorf("model = %v, want gpt-5.1-codex", decoded["model"])
	}
	if decoded["store"] != false {
		t.Errorf("store = %v, want false", decoded["store"])
	}
	if decoded["stream"] != true {
		t.Errorf("stream = %v, want true", decoded["stream"])
	}
	if decoded["instructions"] != "be helpful" {
		t.Errorf("instructions = %v, want be helpful", decoded["instructions"])
	}
	if decoded["tool_choice"] != "auto" {
		t.Errorf("tool_choice = %v, want auto", decoded["tool_choice"])
	}
	if decoded["parallel_tool_calls"] != true {
		t.Errorf("parallel_tool_calls = %v, want true", decoded["parallel_tool_calls"])
	}
	include, ok := decoded["include"].([]any)
	if !ok || len(include) != 1 || include[0] != "reasoning.encrypted_content" {
		t.Errorf("include = %#v, want [reasoning.encrypted_content]", decoded["include"])
	}
	text, ok := decoded["text"].(map[string]any)
	if !ok || text["verbosity"] != "low" {
		t.Errorf("text = %#v, want verbosity low", decoded["text"])
	}
	if _, hasReasoning := decoded["reasoning"]; hasReasoning {
		t.Errorf("reasoning present = %#v, want absent when unrequested", decoded["reasoning"])
	}
	if _, hasCacheKey := decoded["prompt_cache_key"]; hasCacheKey {
		t.Errorf("prompt_cache_key present = %#v, want omitted when no sessionId", decoded["prompt_cache_key"])
	}
	// instructions comes from context.systemPrompt directly, not through the
	// shared ConvertMessages system-prompt item -- confirm it isn't
	// duplicated as an input item too.
	input, ok := decoded["input"].([]any)
	if !ok || len(input) != 1 {
		t.Fatalf("input = %#v, want 1 entry (user only, no system item)", decoded["input"])
	}
}

func TestBuildRequestBody_DefaultInstructionsWhenNoSystemPrompt(t *testing.T) {
	model := testModel("https://chatgpt.com/backend-api")
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi")}}}
	body := buildRequestBody(model, chat, &ai.StreamOptions{APIKey: "token"})
	decoded := marshalRoundtrip(t, body)
	if decoded["instructions"] != "You are a helpful assistant." {
		t.Errorf("instructions = %v, want default assistant prompt", decoded["instructions"])
	}
}

func TestBuildRequestBody_TextVerbosityOverride(t *testing.T) {
	model := testModel("https://chatgpt.com/backend-api")
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi")}}}
	body := buildRequestBody(model, chat, &ai.StreamOptions{APIKey: "token", TextVerbosity: "high"})
	decoded := marshalRoundtrip(t, body)
	text := decoded["text"].(map[string]any)
	if text["verbosity"] != "high" {
		t.Errorf("verbosity = %v, want high", text["verbosity"])
	}
}

func TestBuildRequestBody_PromptCacheKeySetAndClampedWhenSessionIDPresent(t *testing.T) {
	model := testModel("https://chatgpt.com/backend-api")
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi")}}}
	longSession := ""
	for i := 0; i < 67; i++ {
		longSession += "x"
	}
	body := buildRequestBody(model, chat, &ai.StreamOptions{APIKey: "token", SessionID: longSession})
	decoded := marshalRoundtrip(t, body)
	key, _ := decoded["prompt_cache_key"].(string)
	if len(key) != 64 {
		t.Errorf("prompt_cache_key length = %d, want 64", len(key))
	}
}

func TestBuildRequestBody_ToolsIncludedWhenPresent(t *testing.T) {
	model := testModel("https://chatgpt.com/backend-api")
	chat := ai.Context{
		Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi")}},
		Tools:    []ai.Tool{{Name: "get_weather", Description: "gets weather", Parameters: ai.JSONSchema(`{"type":"object"}`)}},
	}
	body := buildRequestBody(model, chat, &ai.StreamOptions{APIKey: "token"})
	decoded := marshalRoundtrip(t, body)
	tools, ok := decoded["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("tools = %#v, want 1 entry", decoded["tools"])
	}
}

func TestBuildRequestBody_ReasoningEffortMinimalClampedViaThinkingLevelMap(t *testing.T) {
	model := testModel("https://chatgpt.com/backend-api")
	model.ThinkingLevelMap = ai.ThinkingLevelMap{ai.ThinkingMinimal: strPtr("low")}
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi")}}}
	body := buildRequestBody(model, chat, &ai.StreamOptions{APIKey: "token", ReasoningEffort: ai.ThinkingMinimal})
	decoded := marshalRoundtrip(t, body)
	reasoning, ok := decoded["reasoning"].(map[string]any)
	if !ok {
		t.Fatalf("reasoning = %#v, want present", decoded["reasoning"])
	}
	if reasoning["effort"] != "low" {
		t.Errorf("effort = %v, want low", reasoning["effort"])
	}
	if reasoning["summary"] != "auto" {
		t.Errorf("summary = %v, want auto", reasoning["summary"])
	}
}

func TestBuildRequestBody_ReasoningOffMapsToNoneOrSuppresses(t *testing.T) {
	model := testModel("https://chatgpt.com/backend-api")
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi")}}}
	body := buildRequestBody(model, chat, &ai.StreamOptions{APIKey: "token", ReasoningEffort: ai.ThinkingOff})
	decoded := marshalRoundtrip(t, body)
	reasoning, ok := decoded["reasoning"].(map[string]any)
	if !ok {
		t.Fatalf("reasoning = %#v, want present with effort none", decoded["reasoning"])
	}
	if reasoning["effort"] != "none" {
		t.Errorf("effort = %v, want none", reasoning["effort"])
	}

	// An explicit off:nil mapping suppresses the field entirely.
	model.ThinkingLevelMap = ai.ThinkingLevelMap{ai.ThinkingOff: nil}
	body = buildRequestBody(model, chat, &ai.StreamOptions{APIKey: "token", ReasoningEffort: ai.ThinkingOff})
	decoded = marshalRoundtrip(t, body)
	if _, present := decoded["reasoning"]; present {
		t.Errorf("reasoning present = %#v, want suppressed by explicit off:nil mapping", decoded["reasoning"])
	}
}

func TestResolveCodexServiceTier(t *testing.T) {
	cases := []struct {
		name     string
		response string
		request  string
		want     string
	}{
		{"echoes default, request flex -> flex", "default", "flex", "flex"},
		{"echoes default, request priority -> priority", "default", "priority", "priority"},
		{"response reports real tier -> response wins", "priority", "flex", "priority"},
		{"no response tier -> request wins", "", "flex", "flex"},
		{"neither set -> empty", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveCodexServiceTier(tc.response, tc.request)
			if got != tc.want {
				t.Errorf("resolveCodexServiceTier(%q, %q) = %q, want %q", tc.response, tc.request, got, tc.want)
			}
		})
	}
}

func TestResolveCodexURL(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		want    string
	}{
		{"empty uses default", "", "https://chatgpt.com/backend-api/codex/responses"},
		{"base without codex suffix", "https://example.invalid", "https://example.invalid/codex/responses"},
		{"base with trailing slash", "https://example.invalid/", "https://example.invalid/codex/responses"},
		{"base ending in /codex", "https://example.invalid/codex", "https://example.invalid/codex/responses"},
		{"base already full path", "https://example.invalid/codex/responses", "https://example.invalid/codex/responses"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveCodexURL(tc.baseURL)
			if got != tc.want {
				t.Errorf("resolveCodexURL(%q) = %q, want %q", tc.baseURL, got, tc.want)
			}
		})
	}
}

func strPtr(s string) *string { return &s }
