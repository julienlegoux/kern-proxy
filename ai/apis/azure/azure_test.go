package azure

// New Go tests covering buildParams/buildHeaders fidelity (upstream has no
// dedicated request-building test file for azure-openai-responses.ts beyond
// the base-URL/prompt-cache-key/store assertions already ported in
// baseurl_test.go/azure-openai-base-url.test.ts), matching the same fidelity
// bar as the sibling openairesponses package's own buildParams tests.

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/julienlegoux/kern-link/ai"
)

func TestBuildParams_BasicRequestShape(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{
		SystemPrompt: "be helpful",
		Messages:     []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}},
	}

	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "azure-key"}, "my-deployment")
	if params.Model != "my-deployment" {
		t.Errorf("model = %q, want my-deployment (the resolved deployment name)", params.Model)
	}
	if !params.Stream {
		t.Errorf("stream = %v, want true", params.Stream)
	}
	if params.Store != false {
		t.Errorf("store = %v, want false", params.Store)
	}
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := decoded["store"]; !ok {
		t.Errorf("store field omitted, want always present (false)")
	}
	if _, ok := decoded["prompt_cache_key"]; !ok {
		t.Errorf("prompt_cache_key field omitted, want always present (even empty)")
	}
	input, ok := decoded["input"].([]any)
	if !ok || len(input) != 2 {
		t.Fatalf("input = %#v, want 2 entries (system, user)", decoded["input"])
	}
}

// TestBuildParams_PromptCacheKeyClampedTo64Chars ports the upstream
// "clamps prompt_cache_key to OpenAI's 64-character limit" assertion.
func TestBuildParams_PromptCacheKeyClampedTo64Chars(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	long := ""
	for i := 0; i < 67; i++ {
		long += "x"
	}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "azure-key", SessionID: long}, "dep")
	want := ""
	for i := 0; i < 64; i++ {
		want += "x"
	}
	if params.PromptCacheKey != want {
		t.Errorf("promptCacheKey len = %d, want 64", len(params.PromptCacheKey))
	}
}

// TestBuildParams_UsesDeveloperRoleForReasoningModels ports the
// supportsDeveloperRole compat gate shared with the openairesponses core.
func TestBuildParams_UsesDeveloperRoleForReasoningModels(t *testing.T) {
	model := testModel("https://example.invalid")
	model.Reasoning = true
	chat := ai.Context{
		SystemPrompt: "be helpful",
		Messages:     []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}},
	}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "azure-key"}, "dep")
	raw, _ := json.Marshal(params)
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	input := decoded["input"].([]any)
	sysMsg := input[0].(map[string]any)
	if sysMsg["role"] != "developer" {
		t.Errorf("role = %v, want developer", sysMsg["role"])
	}
}

func TestBuildParams_FallsBackToSystemRoleWhenCompatDisablesDeveloperRole(t *testing.T) {
	model := testModel("https://example.invalid")
	model.Reasoning = true
	no := false
	model.Compat = &ai.Compat{SupportsDeveloperRole: &no}
	chat := ai.Context{
		SystemPrompt: "be helpful",
		Messages:     []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}},
	}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "azure-key"}, "dep")
	raw, _ := json.Marshal(params)
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	input := decoded["input"].([]any)
	sysMsg := input[0].(map[string]any)
	if sysMsg["role"] != "system" {
		t.Errorf("role = %v, want system", sysMsg["role"])
	}
}

func TestBuildParams_OmitsToolsFieldWhenContextToolsEmpty(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{
		Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}},
		Tools:    []ai.Tool{},
	}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "azure-key"}, "dep")
	if params.Tools != nil {
		t.Errorf("tools = %#v, want nil (omitted)", params.Tools)
	}
}

func TestBuildParams_IncludesToolsWhenPresent(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{
		Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}},
		Tools: []ai.Tool{
			{Name: "get_weather", Description: "gets weather", Parameters: json.RawMessage(`{"type":"object"}`)},
		},
	}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "azure-key"}, "dep")
	raw, _ := json.Marshal(params)
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	tools, ok := decoded["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("tools = %#v, want 1 tool", decoded["tools"])
	}
	tool := tools[0].(map[string]any)
	if tool["type"] != "function" || tool["name"] != "get_weather" {
		t.Errorf("tool = %#v, want function/get_weather", tool)
	}
}

func TestBuildParams_ClampsMaxOutputTokensToMinimum(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	small := 4
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "azure-key", MaxTokens: &small}, "dep")
	if params.MaxOutputTokens == nil || *params.MaxOutputTokens != 16 {
		t.Errorf("maxOutputTokens = %v, want 16 (clamped)", params.MaxOutputTokens)
	}
}

func TestBuildParams_ReasoningEffortSetsEffortAndSummary(t *testing.T) {
	model := testModel("https://example.invalid")
	model.Reasoning = true
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "azure-key", ReasoningEffort: ai.ThinkingHigh}, "dep")
	if params.Reasoning == nil {
		t.Fatalf("reasoning = nil, want set")
	}
	if params.Reasoning.Effort != "high" {
		t.Errorf("reasoning.effort = %q, want high", params.Reasoning.Effort)
	}
	if params.Reasoning.Summary != "auto" {
		t.Errorf("reasoning.summary = %q, want auto", params.Reasoning.Summary)
	}
	if len(params.Include) != 1 || params.Include[0] != "reasoning.encrypted_content" {
		t.Errorf("include = %#v, want [reasoning.encrypted_content]", params.Include)
	}
}

func TestBuildParams_NonReasoningModelNeverSetsReasoningField(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "azure-key", ReasoningEffort: ai.ThinkingHigh}, "dep")
	if params.Reasoning != nil {
		t.Errorf("reasoning = %#v, want nil for non-reasoning model", params.Reasoning)
	}
}

func TestBuildParams_NoRequestedEffortSendsMappedOffEffort(t *testing.T) {
	model := testModel("https://example.invalid")
	model.Reasoning = true
	off := "minimal"
	model.ThinkingLevelMap = ai.ThinkingLevelMap{ai.ThinkingOff: &off}
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "azure-key"}, "dep")
	if params.Reasoning == nil || params.Reasoning.Effort != "minimal" {
		t.Errorf("reasoning = %#v, want effort=minimal", params.Reasoning)
	}
	if params.Reasoning.Summary != "" {
		t.Errorf("reasoning.summary = %q, want empty", params.Reasoning.Summary)
	}
}

func TestBuildParams_ExplicitNullOffSuppressesReasoningField(t *testing.T) {
	model := testModel("https://example.invalid")
	model.Reasoning = true
	model.ThinkingLevelMap = ai.ThinkingLevelMap{ai.ThinkingOff: nil}
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "azure-key"}, "dep")
	if params.Reasoning != nil {
		t.Errorf("reasoning = %#v, want nil (explicitly suppressed)", params.Reasoning)
	}
}

// --- headers / auth ----------------------------------------------------------

func TestBuildHeaders_SetsAPIKeyHeader(t *testing.T) {
	model := testModel("https://example.invalid")
	headers := buildHeaders(model, &ai.StreamOptions{}, "azure-key")
	if headers["api-key"] != "azure-key" {
		t.Errorf("api-key = %q, want azure-key", headers["api-key"])
	}
	if headers["content-type"] != "application/json" {
		t.Errorf("content-type = %q", headers["content-type"])
	}
}

func TestBuildHeaders_OptHeadersOverrideDefaults(t *testing.T) {
	model := testModel("https://example.invalid")
	headers := buildHeaders(model, &ai.StreamOptions{Headers: ai.ProviderHeaders{"api-key": ai.HeaderValue("override-key")}}, "azure-key")
	if headers["api-key"] != "override-key" {
		t.Errorf("api-key = %q, want override-key", headers["api-key"])
	}
}

func TestAssertRequestAuth_MissingAPIKeyErrors(t *testing.T) {
	err := assertRequestAuth("azure-openai-responses", "")
	if err == nil {
		t.Fatal("err = nil, want a missing-key error")
	}
}

func TestAssertRequestAuth_APIKeyPresentPasses(t *testing.T) {
	if err := assertRequestAuth("azure-openai-responses", "azure-key"); err != nil {
		t.Errorf("err = %v, want nil", err)
	}
}
