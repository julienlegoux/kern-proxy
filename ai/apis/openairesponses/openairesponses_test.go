package openairesponses

// New Go tests (not literal upstream ports): upstream has no dedicated
// request-building test file for openai-responses.ts /
// openai-responses-shared.ts (its fidelity is exercised indirectly through
// higher-level agent/session tests), so these are new Go tests covering the
// same fidelity bar as the sibling anthropic/openaicompletions packages:
// request building (input items, tools, reasoning config) via buildParams,
// and streaming decode via Stream (see stream_test.go).

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kern-ia/kern-link/ai"
)

func testModel(baseURL string) *ai.Model {
	return &ai.Model{
		ID:            "gpt-5.1",
		Name:          "GPT-5.1",
		Api:           ai.ApiOpenAIResponses,
		Provider:      "openai",
		BaseURL:       baseURL,
		Input:         []ai.Modality{ai.ModalityText, ai.ModalityImage},
		ContextWindow: 400000,
		MaxTokens:     32000,
	}
}

func TestBuildParams_BasicRequestShape(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{
		SystemPrompt: "be helpful",
		Messages:     []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}},
	}

	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	if params.Model != "gpt-5.1" {
		t.Errorf("model = %q, want gpt-5.1", params.Model)
	}
	if !params.Stream {
		t.Errorf("stream = %v, want true", params.Stream)
	}
	if params.Store == nil || *params.Store != false {
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
	input, ok := decoded["input"].([]any)
	if !ok || len(input) != 2 {
		t.Fatalf("input = %#v, want 2 entries (system, user)", decoded["input"])
	}
	sysMsg := input[0].(map[string]any)
	if sysMsg["role"] != "system" || sysMsg["content"] != "be helpful" {
		t.Errorf("input[0] = %#v, want system/be helpful", sysMsg)
	}
	userMsg := input[1].(map[string]any)
	if userMsg["role"] != "user" {
		t.Errorf("input[1].role = %v, want user", userMsg["role"])
	}
	userContent, ok := userMsg["content"].([]any)
	if !ok || len(userContent) != 1 {
		t.Fatalf("input[1].content = %#v, want 1 input_text part", userMsg["content"])
	}
	part := userContent[0].(map[string]any)
	if part["type"] != "input_text" || part["text"] != "hi" {
		t.Errorf("input[1].content[0] = %#v, want input_text/hi", part)
	}
	if _, hasTools := decoded["tools"]; hasTools {
		t.Errorf("tools present = %#v, want omitted when no tools", decoded["tools"])
	}
}

// TestBuildParams_UsesDeveloperRoleForReasoningModels ports the
// supportsDeveloperRole compat gate: reasoning models get the system prompt
// under the "developer" role unless compat.supportsDeveloperRole is false.
func TestBuildParams_UsesDeveloperRoleForReasoningModels(t *testing.T) {
	model := testModel("https://example.invalid")
	model.Reasoning = true
	chat := ai.Context{
		SystemPrompt: "be helpful",
		Messages:     []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}},
	}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})
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
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	raw, _ := json.Marshal(params)
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	input := decoded["input"].([]any)
	sysMsg := input[0].(map[string]any)
	if sysMsg["role"] != "system" {
		t.Errorf("role = %v, want system", sysMsg["role"])
	}
}

// TestBuildParams_UserImageBlocksConvertToInputImageParts covers the
// input_image conversion branch of convertResponsesMessages.
func TestBuildParams_UserImageBlocksConvertToInputImageParts(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{
		Messages: []ai.Message{&ai.UserMessage{
			Content:   ai.UserBlocks(ai.TextContent{Text: "look"}, ai.ImageContent{Data: "Zm9v", MimeType: "image/png"}),
			Timestamp: time.Now().UnixMilli(),
		}},
	}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	raw, _ := json.Marshal(params)
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	input := decoded["input"].([]any)
	userMsg := input[0].(map[string]any)
	content := userMsg["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("content = %#v, want 2 parts", content)
	}
	imgPart := content[1].(map[string]any)
	if imgPart["type"] != "input_image" {
		t.Errorf("content[1].type = %v, want input_image", imgPart["type"])
	}
	if imgPart["image_url"] != "data:image/png;base64,Zm9v" {
		t.Errorf("content[1].image_url = %v, want data URL", imgPart["image_url"])
	}
	if imgPart["detail"] != "auto" {
		t.Errorf("content[1].detail = %v, want auto", imgPart["detail"])
	}
}

// TestBuildParams_RoundTripsAssistantToolCallAndToolResult verifies an
// assistant tool-call turn followed by its tool result round-trips into the
// Responses wire shape: a function_call item followed by a
// function_call_output item, matching upstream's item-array replay.
func TestBuildParams_RoundTripsAssistantToolCallAndToolResult(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{
		Messages: []ai.Message{
			&ai.UserMessage{Content: ai.UserText("use the tool"), Timestamp: time.Now().UnixMilli()},
			&ai.AssistantMessage{
				Content:    []ai.AssistantContentPart{ai.ToolCall{ID: "call_1|fc_item1", Name: "noop", Arguments: map[string]any{"x": float64(1)}}},
				StopReason: ai.StopReasonToolUse,
				Api:        ai.ApiOpenAIResponses,
				Provider:   "openai",
				Model:      "gpt-5.1",
			},
			&ai.ToolResultMessage{ToolCallID: "call_1|fc_item1", ToolName: "noop", Content: []ai.UserContentPart{ai.TextContent{Text: "done"}}},
		},
	}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	raw, _ := json.Marshal(params)
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	input := decoded["input"].([]any)
	if len(input) != 3 {
		t.Fatalf("input = %#v, want 3 (user, function_call, function_call_output)", input)
	}
	fc := input[1].(map[string]any)
	if fc["type"] != "function_call" || fc["call_id"] != "call_1" || fc["name"] != "noop" {
		t.Errorf("input[1] = %#v, want function_call/call_1/noop", fc)
	}
	if fc["arguments"] != `{"x":1}` {
		t.Errorf("input[1].arguments = %v, want {\"x\":1}", fc["arguments"])
	}
	fco := input[2].(map[string]any)
	if fco["type"] != "function_call_output" || fco["call_id"] != "call_1" || fco["output"] != "done" {
		t.Errorf("input[2] = %#v, want function_call_output/call_1/done", fco)
	}
}

// TestBuildParams_ReplaysThinkingSignatureVerbatim verifies a same-model
// thinking block with a signature is replayed as the raw reasoning item
// (upstream: JSON.parse(block.thinkingSignature) pushed as-is).
func TestBuildParams_ReplaysThinkingSignatureVerbatim(t *testing.T) {
	model := testModel("https://example.invalid")
	sig := `{"type":"reasoning","id":"rs_1","summary":[],"encrypted_content":"abc"}`
	chat := ai.Context{
		Messages: []ai.Message{
			&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()},
			&ai.AssistantMessage{
				Content: []ai.AssistantContentPart{
					ai.ThinkingContent{Thinking: "reasoning summary", ThinkingSignature: sig},
					ai.TextContent{Text: "answer"},
				},
				StopReason: ai.StopReasonStop,
				Api:        ai.ApiOpenAIResponses,
				Provider:   "openai",
				Model:      "gpt-5.1",
			},
		},
	}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	raw, _ := json.Marshal(params)
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	input := decoded["input"].([]any)
	if len(input) != 3 {
		t.Fatalf("input = %#v, want 3 (user, reasoning, message)", input)
	}
	reasoning := input[1].(map[string]any)
	if reasoning["type"] != "reasoning" || reasoning["id"] != "rs_1" || reasoning["encrypted_content"] != "abc" {
		t.Errorf("input[1] = %#v, want verbatim replay of the stored signature", reasoning)
	}
}

// TestBuildParams_OmitsToolsFieldWhenContextToolsEmpty and its sibling below
// port the "empty tools handling" behavior shared with openaicompletions.
func TestBuildParams_OmitsToolsFieldWhenContextToolsEmpty(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{
		Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}},
		Tools:    []ai.Tool{},
	}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	if params.Tools != nil {
		t.Errorf("tools = %#v, want nil (omitted)", params.Tools)
	}
}

func TestBuildParams_IncludesToolsWithStrictDefaultFalse(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{
		Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}},
		Tools: []ai.Tool{
			{Name: "get_weather", Description: "gets weather", Parameters: json.RawMessage(`{"type":"object"}`)},
		},
	}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	if params.Tools == nil || len(*params.Tools) != 1 {
		t.Fatalf("tools = %#v, want 1 tool", params.Tools)
	}
	tool := (*params.Tools)[0]
	if tool.Type != "function" || tool.Name != "get_weather" || tool.Strict != false {
		t.Errorf("tool = %#v, want function/get_weather/strict=false", tool)
	}
}

// TestBuildParams_ClampsMaxOutputTokensToMinimum ports the OpenAI Responses
// 16-token minimum quirk documented in the upstream comment
// (OPENAI_RESPONSES_MIN_OUTPUT_TOKENS).
func TestBuildParams_ClampsMaxOutputTokensToMinimum(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	small := 4
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test", MaxTokens: &small})
	if params.MaxOutputTokens == nil || *params.MaxOutputTokens != 16 {
		t.Errorf("maxOutputTokens = %v, want 16 (clamped)", params.MaxOutputTokens)
	}
}

// TestBuildParams_ReasoningEffortSetsEffortAndSummary verifies reasoning
// models with a requested effort get reasoning.effort/summary and the
// encrypted_content include directive.
func TestBuildParams_ReasoningEffortSetsEffortAndSummary(t *testing.T) {
	model := testModel("https://example.invalid")
	model.Reasoning = true
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test", ReasoningEffort: ai.ThinkingHigh})
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

// TestBuildParams_NonReasoningModelNeverSetsReasoningField verifies the
// model.Reasoning gate: non-reasoning models never get a reasoning field even
// when a ReasoningEffort is requested.
func TestBuildParams_NonReasoningModelNeverSetsReasoningField(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test", ReasoningEffort: ai.ThinkingHigh})
	if params.Reasoning != nil {
		t.Errorf("reasoning = %#v, want nil for non-reasoning model", params.Reasoning)
	}
}

// TestBuildParams_NoRequestedEffortSendsMappedOffEffort ports the "else"
// branch of upstream buildParams' reasoning gate: when reasoning is
// supported but no effort/summary was requested, the model's own "off"
// mapping (or "none" absent a mapping) is still sent as a bare effort, with
// no summary and no include directive.
func TestBuildParams_NoRequestedEffortSendsMappedOffEffort(t *testing.T) {
	model := testModel("https://example.invalid")
	model.Reasoning = true
	off := "minimal"
	model.ThinkingLevelMap = ai.ThinkingLevelMap{ai.ThinkingOff: &off}
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	if params.Reasoning == nil {
		t.Fatalf("reasoning = nil, want set")
	}
	if params.Reasoning.Effort != "minimal" {
		t.Errorf("reasoning.effort = %q, want minimal", params.Reasoning.Effort)
	}
	if params.Reasoning.Summary != "" {
		t.Errorf("reasoning.summary = %q, want empty", params.Reasoning.Summary)
	}
	if params.Include != nil {
		t.Errorf("include = %#v, want nil (no encrypted-content directive)", params.Include)
	}
}

func TestBuildParams_NoRequestedEffortDefaultsToNoneWithoutMapping(t *testing.T) {
	model := testModel("https://example.invalid")
	model.Reasoning = true
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	if params.Reasoning == nil || params.Reasoning.Effort != "none" {
		t.Errorf("reasoning = %#v, want effort=none", params.Reasoning)
	}
}

// TestBuildParams_ExplicitNullOffSuppressesReasoningField ports the explicit
// "off" -> null marker: reasoning is omitted entirely rather than defaulting
// to "none".
func TestBuildParams_ExplicitNullOffSuppressesReasoningField(t *testing.T) {
	model := testModel("https://example.invalid")
	model.Reasoning = true
	model.ThinkingLevelMap = ai.ThinkingLevelMap{ai.ThinkingOff: nil}
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	if params.Reasoning != nil {
		t.Errorf("reasoning = %#v, want nil (explicitly suppressed)", params.Reasoning)
	}
}
