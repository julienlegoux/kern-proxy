package mistral

// Message-conversion tests, plus a Go-appropriate port of
// test/mistral-tool-schema.test.ts: upstream's test guards against TypeBox's
// non-enumerable Symbol keys leaking into the request the SDK validates
// against; Go's json.RawMessage has no such concept, so this instead asserts
// the fidelity property that test was really protecting -- an arbitrarily
// nested tool parameters schema round-trips into the wire payload unchanged.

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
)

func TestToChatMessages_PlainUserText(t *testing.T) {
	model := testModel("https://example.invalid")
	messages := []ai.Message{ai.UserMessage{Content: ai.UserText("hello"), Timestamp: time.Now().UnixMilli()}}
	out := toChatMessages(messages, model, newToolCallIDNormalizer())
	if len(out) != 1 || out[0].Role != "user" || out[0].Content != "hello" {
		t.Fatalf("out = %#v, want single user message %q", out, "hello")
	}
}

// TestToChatMessages_AssistantTextAndToolCall covers text+tool_calls
// conversion on an assistant message. The tool call is followed by a
// matching ToolResultMessage so apis.TransformMessages's orphan-result
// synthesis (tested in its own package) doesn't also fire here.
func TestToChatMessages_AssistantTextAndToolCall(t *testing.T) {
	model := testModel("https://example.invalid")
	messages := []ai.Message{
		&ai.AssistantMessage{
			Content: []ai.AssistantContentPart{
				ai.TextContent{Text: "thinking out loud"},
				ai.ToolCall{ID: "call_1", Name: "get_weather", Arguments: map[string]any{"city": "nyc"}},
			},
			Api: ai.ApiMistralConversations, Provider: "mistral", Model: "mistral-large-latest",
			StopReason: ai.StopReasonToolUse, Timestamp: time.Now().UnixMilli(),
		},
		ai.ToolResultMessage{
			ToolCallID: "call_1", ToolName: "get_weather",
			Content:   []ai.UserContentPart{ai.TextContent{Text: "sunny"}},
			Timestamp: time.Now().UnixMilli(),
		},
	}
	out := toChatMessages(messages, model, newToolCallIDNormalizer())
	if len(out) != 2 {
		t.Fatalf("out = %#v, want [assistant, tool]", out)
	}
	if out[0].Role != "assistant" {
		t.Fatalf("role = %q, want assistant", out[0].Role)
	}
	if len(out[0].ToolCalls) != 1 || out[0].ToolCalls[0].Function.Name != "get_weather" {
		t.Fatalf("toolCalls = %#v, want get_weather", out[0].ToolCalls)
	}
}

func TestToChatMessages_ToolResult(t *testing.T) {
	model := testModel("https://example.invalid")
	messages := []ai.Message{
		ai.ToolResultMessage{
			ToolCallID: "call_1", ToolName: "get_weather",
			Content:   []ai.UserContentPart{ai.TextContent{Text: "sunny"}},
			Timestamp: time.Now().UnixMilli(),
		},
	}
	out := toChatMessages(messages, model, newToolCallIDNormalizer())
	if len(out) != 1 || out[0].Role != "tool" || out[0].ToolCallID != "call_1" || out[0].Name != "get_weather" {
		t.Fatalf("out = %#v, want tool message for call_1/get_weather", out)
	}
}

func TestBuildToolResultText_ErrorPrefixAndTrim(t *testing.T) {
	if got := buildToolResultText("  boom  ", false, false, true); got != "[tool error] boom" {
		t.Errorf("buildToolResultText = %q, want %q", got, "[tool error] boom")
	}
}

func TestBuildToolResultText_EmptyNoImages(t *testing.T) {
	if got := buildToolResultText("", false, false, false); got != "(no tool output)" {
		t.Errorf("buildToolResultText = %q, want %q", got, "(no tool output)")
	}
}

func TestBuildToolResultText_ImageOmittedWhenUnsupported(t *testing.T) {
	if got := buildToolResultText("", true, false, false); got != "(image omitted: model does not support images)" {
		t.Errorf("buildToolResultText = %q, want image-omitted placeholder", got)
	}
}

// TestToFunctionTools_NestedSchemaRoundTrips ports the fidelity property
// behind test/mistral-tool-schema.test.ts: a nested JSON Schema document
// passed as a tool's parameters must appear unchanged in the wire payload.
func TestToFunctionTools_NestedSchemaRoundTrips(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"nested":{"type":"object","properties":{"value":{"type":"string"}}}}}`)
	tools := []ai.Tool{{Name: "inspect_schema", Description: "Inspect the schema", Parameters: schema}}

	out := toFunctionTools(tools)
	if len(out) != 1 {
		t.Fatalf("out = %#v, want 1 tool", out)
	}
	if out[0].Type != "function" {
		t.Errorf("type = %q, want function", out[0].Type)
	}
	if out[0].Function.Strict != false {
		t.Errorf("strict = %v, want false", out[0].Function.Strict)
	}

	var want, got any
	if err := json.Unmarshal(schema, &want); err != nil {
		t.Fatalf("unmarshal want: %v", err)
	}
	if err := json.Unmarshal(out[0].Function.Parameters, &got); err != nil {
		t.Fatalf("unmarshal got: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("parameters = %#v, want %#v (schema must round-trip unchanged)", got, want)
	}
}

func TestMapToolChoice_StringEnum(t *testing.T) {
	if got := mapToolChoice("required", ""); got != "required" {
		t.Errorf("mapToolChoice = %#v, want \"required\"", got)
	}
}

func TestMapToolChoice_NamedFunction(t *testing.T) {
	got := mapToolChoice("auto", "get_weather")
	m, ok := got.(map[string]any)
	if !ok || m["type"] != "function" {
		t.Fatalf("mapToolChoice = %#v, want named-function map", got)
	}
	fn, ok := m["function"].(map[string]any)
	if !ok || fn["name"] != "get_weather" {
		t.Fatalf("mapToolChoice.function = %#v, want name=get_weather", m["function"])
	}
}

func TestMapToolChoice_EmptyWhenUnset(t *testing.T) {
	if got := mapToolChoice("", ""); got != nil {
		t.Errorf("mapToolChoice = %#v, want nil", got)
	}
}

func TestMapChatStopReason(t *testing.T) {
	cases := map[string]ai.StopReason{
		"stop":         ai.StopReasonStop,
		"length":       ai.StopReasonLength,
		"model_length": ai.StopReasonLength,
		"tool_calls":   ai.StopReasonToolUse,
		"error":        ai.StopReasonError,
		"":             ai.StopReasonStop,
		"unknown":      ai.StopReasonStop,
	}
	for reason, want := range cases {
		if got := mapChatStopReason(reason); got != want {
			t.Errorf("mapChatStopReason(%q) = %q, want %q", reason, got, want)
		}
	}
}
