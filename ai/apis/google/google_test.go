package google

// New Go tests covering request building (buildParams/buildHeaders) and the
// StreamSimple thinking-level/budget selection, matching the fidelity bar of
// the sibling anthropic/openairesponses packages (upstream has no dedicated
// request-building test file for google-generative-ai.ts either -- its
// fidelity is exercised indirectly through higher-level agent/session tests).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kern-ia/kern-link/ai"
)

func TestBuildParams_BasicRequestShape(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{
		SystemPrompt: "be helpful",
		Messages:     []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}},
	}
	temp := 0.5
	maxTokens := 100
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "k", Temperature: &temp, MaxTokens: &maxTokens})

	if params.Model != "gemini-2.5-pro" {
		t.Errorf("model = %q, want gemini-2.5-pro", params.Model)
	}
	if len(params.Contents) != 1 {
		t.Fatalf("contents = %#v, want 1", params.Contents)
	}
	if params.SystemInstruction == nil {
		t.Fatalf("systemInstruction = nil, want set")
	}
	if params.GenerationConfig.Temperature == nil || *params.GenerationConfig.Temperature != 0.5 {
		t.Errorf("temperature = %v, want 0.5", params.GenerationConfig.Temperature)
	}
	if params.GenerationConfig.MaxOutputTokens == nil || *params.GenerationConfig.MaxOutputTokens != 100 {
		t.Errorf("maxOutputTokens = %v, want 100", params.GenerationConfig.MaxOutputTokens)
	}
	if params.Tools != nil {
		t.Errorf("tools = %#v, want nil when no tools offered", params.Tools)
	}
}

func TestBuildParams_ToolChoiceSetsFunctionCallingConfig(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{
		Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}},
		Tools:    []ai.Tool{{Name: "noop", Parameters: json.RawMessage(`{"type":"object"}`)}},
	}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "k", GoogleToolChoice: "none"})
	if params.Tools == nil {
		t.Fatalf("tools = nil, want set")
	}
	if params.ToolConfig == nil || params.ToolConfig.FunctionCallingConfig.Mode != "NONE" {
		t.Errorf("toolConfig = %#v, want mode NONE", params.ToolConfig)
	}
}

// TestBuildParams_ThinkingBudgetForNonGemini3Model ports getGoogleBudget's
// 2.5-pro budget table.
func TestBuildParams_ThinkingBudgetForNonGemini3Model(t *testing.T) {
	model := testModel("https://example.invalid")
	model.Reasoning = true
	on := true
	budget := 2048
	params := buildParams(model, ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}, &ai.StreamOptions{
		APIKey:               "k",
		ThinkingEnabled:      &on,
		ThinkingBudgetTokens: &budget,
	})
	if params.GenerationConfig.ThinkingConfig == nil {
		t.Fatalf("thinkingConfig = nil, want set")
	}
	if params.GenerationConfig.ThinkingConfig.ThinkingBudget == nil || *params.GenerationConfig.ThinkingConfig.ThinkingBudget != 2048 {
		t.Errorf("thinkingBudget = %v, want 2048", params.GenerationConfig.ThinkingConfig.ThinkingBudget)
	}
	if !params.GenerationConfig.ThinkingConfig.IncludeThoughts {
		t.Errorf("includeThoughts = false, want true")
	}
}

func TestBuildParams_ThinkingLevelForGemini3ProModel(t *testing.T) {
	model := testModel("https://example.invalid")
	model.ID = "gemini-3-pro"
	model.Reasoning = true
	on := true
	params := buildParams(model, ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}, &ai.StreamOptions{
		APIKey:              "k",
		ThinkingEnabled:     &on,
		GoogleThinkingLevel: ai.GoogleThinkingLevelHigh,
	})
	if params.GenerationConfig.ThinkingConfig == nil || params.GenerationConfig.ThinkingConfig.ThinkingLevel != "HIGH" {
		t.Errorf("thinkingConfig = %#v, want thinkingLevel HIGH", params.GenerationConfig.ThinkingConfig)
	}
}

// TestBuildParams_ThinkingDisabledUsesBudgetZeroForBudgetModel ports
// getDisabledThinkingConfig's Gemini 2.x branch (thinkingBudget: 0).
func TestBuildParams_ThinkingDisabledUsesBudgetZeroForBudgetModel(t *testing.T) {
	model := testModel("https://example.invalid")
	model.Reasoning = true
	off := false
	params := buildParams(model, ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}, &ai.StreamOptions{
		APIKey:          "k",
		ThinkingEnabled: &off,
	})
	if params.GenerationConfig.ThinkingConfig == nil || params.GenerationConfig.ThinkingConfig.ThinkingBudget == nil || *params.GenerationConfig.ThinkingConfig.ThinkingBudget != 0 {
		t.Errorf("thinkingConfig = %#v, want thinkingBudget=0", params.GenerationConfig.ThinkingConfig)
	}
}

// TestBuildParams_ThinkingDisabledUsesLowLevelForGemini3Pro ports
// getDisabledThinkingConfig's Gemini 3 Pro branch (thinkingLevel: LOW, since
// full thinking-off isn't supported).
func TestBuildParams_ThinkingDisabledUsesLowLevelForGemini3Pro(t *testing.T) {
	model := testModel("https://example.invalid")
	model.ID = "gemini-3-pro"
	model.Reasoning = true
	off := false
	params := buildParams(model, ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}, &ai.StreamOptions{
		APIKey:          "k",
		ThinkingEnabled: &off,
	})
	if params.GenerationConfig.ThinkingConfig == nil || params.GenerationConfig.ThinkingConfig.ThinkingLevel != "LOW" {
		t.Errorf("thinkingConfig = %#v, want thinkingLevel LOW", params.GenerationConfig.ThinkingConfig)
	}
}

func TestBuildHeaders_SetsAPIKeyHeader(t *testing.T) {
	headers := buildHeaders(testModel("https://example.invalid"), nil, "my-key")
	if headers["x-goog-api-key"] != "my-key" {
		t.Errorf("headers = %#v, want x-goog-api-key=my-key", headers)
	}
	if headers["content-type"] != "application/json" {
		t.Errorf("headers = %#v, want content-type=application/json", headers)
	}
}

func TestRequestURL_UsesStreamGenerateContentSSEEndpoint(t *testing.T) {
	model := testModel("https://example.invalid")
	url := requestURL(model)
	want := "https://example.invalid/v1beta/models/gemini-2.5-pro:streamGenerateContent?alt=sse"
	if url != want {
		t.Errorf("url = %q, want %q", url, want)
	}
}

// TestRequestURL_DoesNotDoubleV1BetaFromCatalogBaseURL is the fix for the
// doubled-path bug: the embedded catalog / provider base URL already carries a
// trailing /v1beta, which requestURL appends again. requestURL must strip the
// trailing segment so the path is not ".../v1beta/v1beta/models/..." (404).
func TestRequestURL_DoesNotDoubleV1BetaFromCatalogBaseURL(t *testing.T) {
	model := testModel("https://generativelanguage.googleapis.com/v1beta")
	url := requestURL(model)
	want := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-pro:streamGenerateContent?alt=sse"
	if url != want {
		t.Errorf("url = %q, want %q", url, want)
	}
	if strings.Contains(url, "/v1beta/v1beta") {
		t.Errorf("url = %q doubles the /v1beta segment", url)
	}
}

func TestStreamSimple_UsesBudgetBasedThinkingByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make(map[string]any)
		_ = json.NewDecoder(r.Body).Decode(&body)
		config, _ := body["generationConfig"].(map[string]any)
		thinkingConfig, ok := config["thinkingConfig"].(map[string]any)
		if !ok {
			t.Errorf("body = %#v, want generationConfig.thinkingConfig", body)
		} else if _, hasBudget := thinkingConfig["thinkingBudget"]; !hasBudget {
			t.Errorf("thinkingConfig = %#v, want thinkingBudget for a 2.5-pro model", thinkingConfig)
		}
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}]}` + "\n\n"))
	}))
	defer srv.Close()

	model := testModel(srv.URL)
	model.Reasoning = true
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	stream := StreamSimple(context.Background(), model, chat, &ai.SimpleStreamOptions{StreamOptions: ai.StreamOptions{APIKey: "k"}, Reasoning: ai.ThinkingMedium})
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}
}
