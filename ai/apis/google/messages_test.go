package google

// Ports the pure-function upstream tests for google-shared.ts:
// test/google-thinking-signature.test.ts (IsThinkingPart, RetainThoughtSignature)
// plus new Go tests for ConvertMessages/ConvertTools/MapToolChoice/MapStopReason,
// matching the fidelity bar of the sibling openairesponses/anthropic packages
// (upstream has no dedicated request-building test file for these either).

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/julienlegoux/kern-link/ai"
)

func testModel(baseURL string) *ai.Model {
	return &ai.Model{
		ID:            "gemini-2.5-pro",
		Name:          "Gemini 2.5 Pro",
		Api:           ai.ApiGoogleGenerativeAI,
		Provider:      "google",
		BaseURL:       baseURL,
		Input:         []ai.Modality{ai.ModalityText, ai.ModalityImage},
		ContextWindow: 1000000,
		MaxTokens:     8192,
	}
}

// --- thinking-signature helpers (ports test/google-thinking-signature.test.ts) ---

func TestIsThinkingPart_TreatsThoughtTrueAsThinking(t *testing.T) {
	if !IsThinkingPart(true, "") {
		t.Errorf("thought=true, sig=\"\" => want thinking")
	}
	if !IsThinkingPart(true, "opaque-signature") {
		t.Errorf("thought=true, sig set => want thinking")
	}
}

func TestIsThinkingPart_DoesNotTreatSignatureAloneAsThinking(t *testing.T) {
	if IsThinkingPart(false, "opaque-signature") {
		t.Errorf("thought=false, sig set => want not thinking")
	}
}

func TestIsThinkingPart_EmptyIsNotThinking(t *testing.T) {
	if IsThinkingPart(false, "") {
		t.Errorf("thought=false, sig=\"\" => want not thinking")
	}
}

func TestRetainThoughtSignature_PreservesExistingWhenIncomingEmpty(t *testing.T) {
	first := RetainThoughtSignature("", "sig-1")
	if first != "sig-1" {
		t.Fatalf("first = %q, want sig-1", first)
	}
	second := RetainThoughtSignature(first, "")
	if second != "sig-1" {
		t.Fatalf("second = %q, want sig-1 (preserved)", second)
	}
}

func TestRetainThoughtSignature_UpdatesOnNewNonEmptySignature(t *testing.T) {
	updated := RetainThoughtSignature("sig-1", "sig-2")
	if updated != "sig-2" {
		t.Fatalf("updated = %q, want sig-2", updated)
	}
}

// --- MapToolChoice / MapStopReason ---

func TestMapToolChoice(t *testing.T) {
	cases := map[string]string{
		"auto": "AUTO",
		"none": "NONE",
		"any":  "ANY",
		"":     "AUTO",
		"nope": "AUTO",
	}
	for in, want := range cases {
		if got := MapToolChoice(in); got != want {
			t.Errorf("MapToolChoice(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMapStopReasonString(t *testing.T) {
	cases := map[string]ai.StopReason{
		"STOP":       ai.StopReasonStop,
		"MAX_TOKENS": ai.StopReasonLength,
		"SAFETY":     ai.StopReasonError,
		"OTHER":      ai.StopReasonError,
	}
	for in, want := range cases {
		if got := MapStopReasonString(in); got != want {
			t.Errorf("MapStopReasonString(%q) = %q, want %q", in, got, want)
		}
	}
}

// --- ConvertTools ---

func TestConvertTools_EmptyReturnsNil(t *testing.T) {
	if got := ConvertTools(nil, false); got != nil {
		t.Errorf("ConvertTools(nil) = %#v, want nil", got)
	}
}

func TestConvertTools_BuildsFunctionDeclarationsWithParametersJsonSchema(t *testing.T) {
	tools := []ai.Tool{{
		Name:        "get_weather",
		Description: "Gets the weather",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`),
	}}
	got := ConvertTools(tools, false)
	if len(got) != 1 || len(got[0].FunctionDeclarations) != 1 {
		t.Fatalf("got = %#v, want 1 tool with 1 declaration", got)
	}
	decl := got[0].FunctionDeclarations[0]
	if decl["name"] != "get_weather" || decl["description"] != "Gets the weather" {
		t.Errorf("decl = %#v, want name/description set", decl)
	}
	if _, ok := decl["parametersJsonSchema"]; !ok {
		t.Errorf("decl = %#v, want parametersJsonSchema", decl)
	}
	if _, ok := decl["parameters"]; ok {
		t.Errorf("decl = %#v, want no legacy parameters field", decl)
	}
}

func TestConvertTools_UseParametersStripsMetaKeywords(t *testing.T) {
	tools := []ai.Tool{{
		Name:       "noop",
		Parameters: json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","properties":{}}`),
	}}
	got := ConvertTools(tools, true)
	decl := got[0].FunctionDeclarations[0]
	params, ok := decl["parameters"].(map[string]any)
	if !ok {
		t.Fatalf("decl = %#v, want legacy parameters map", decl)
	}
	if _, has := params["$schema"]; has {
		t.Errorf("params = %#v, want $schema stripped", params)
	}
	if _, ok := decl["parametersJsonSchema"]; ok {
		t.Errorf("decl = %#v, want no parametersJsonSchema when useParameters", decl)
	}
}

// --- ConvertMessages ---

func TestConvertMessages_PlainUserText(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	contents := ConvertMessages(model, chat)
	if len(contents) != 1 || contents[0].Role != "user" {
		t.Fatalf("contents = %#v, want 1 user content", contents)
	}
	if len(contents[0].Parts) != 1 || contents[0].Parts[0]["text"] != "hi" {
		t.Errorf("parts = %#v, want [{text: hi}]", contents[0].Parts)
	}
}

// TestConvertMessages_ThinkingBlockKeptOnlyForSameProviderAndModel ports the
// isSameProviderAndModel gate in convertMessages: thinking blocks replay as
// `thought: true` parts only when the source assistant message came from the
// exact same provider+model; otherwise they downgrade to plain text.
func TestConvertMessages_ThinkingBlockKeptOnlyForSameProviderAndModel(t *testing.T) {
	model := testModel("https://example.invalid")

	sameModelChat := ai.Context{Messages: []ai.Message{
		&ai.AssistantMessage{
			Content:    []ai.AssistantContentPart{ai.ThinkingContent{Thinking: "pondering", ThinkingSignature: "c2ln"}},
			Api:        ai.ApiGoogleGenerativeAI,
			Provider:   "google",
			Model:      "gemini-2.5-pro",
			StopReason: ai.StopReasonStop,
		},
	}}
	contents := ConvertMessages(model, sameModelChat)
	if len(contents) != 1 {
		t.Fatalf("contents = %#v, want 1 model content", contents)
	}
	part := contents[0].Parts[0]
	if part["thought"] != true {
		t.Errorf("part = %#v, want thought=true (same model)", part)
	}
	if part["thoughtSignature"] != "c2ln" {
		t.Errorf("part = %#v, want thoughtSignature=c2ln", part)
	}

	crossModelChat := ai.Context{Messages: []ai.Message{
		&ai.AssistantMessage{
			Content:    []ai.AssistantContentPart{ai.ThinkingContent{Thinking: "pondering", ThinkingSignature: "c2ln"}},
			Api:        ai.ApiAnthropicMessages,
			Provider:   "anthropic",
			Model:      "claude-x",
			StopReason: ai.StopReasonStop,
		},
	}}
	contents = ConvertMessages(model, crossModelChat)
	part = contents[0].Parts[0]
	if _, hasThought := part["thought"]; hasThought {
		t.Errorf("part = %#v, want no thought field (cross-model downgrade)", part)
	}
	if part["text"] != "pondering" {
		t.Errorf("part = %#v, want text=pondering", part)
	}
}

// TestConvertMessages_ToolResultMergesIntoSingleUserTurn ports the Cloud Code
// Assist requirement: consecutive function-response messages merge into one
// user-role content, rather than one content per result.
func TestConvertMessages_ToolResultMergesIntoSingleUserTurn(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{Messages: []ai.Message{
		&ai.ToolResultMessage{ToolCallID: "1", ToolName: "a", Content: []ai.UserContentPart{ai.TextContent{Text: "ok-a"}}},
		&ai.ToolResultMessage{ToolCallID: "2", ToolName: "b", Content: []ai.UserContentPart{ai.TextContent{Text: "ok-b"}}},
	}}
	contents := ConvertMessages(model, chat)
	if len(contents) != 1 {
		t.Fatalf("contents = %#v, want 1 merged user turn", contents)
	}
	if len(contents[0].Parts) != 2 {
		t.Fatalf("parts = %#v, want 2 functionResponse parts", contents[0].Parts)
	}
}

func TestRequiresToolCallID(t *testing.T) {
	cases := map[string]bool{
		"claude-3.5-sonnet": true,
		"gpt-oss-120b":      true,
		"gemini-2.5-pro":    false,
	}
	for id, want := range cases {
		if got := RequiresToolCallID(id); got != want {
			t.Errorf("RequiresToolCallID(%q) = %v, want %v", id, got, want)
		}
	}
}
