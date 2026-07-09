package ai

// The Message interface is implemented on pointer receivers by all three
// message types. AssistantMessage always was (it is mutated during streaming
// and its MarshalJSON is on the pointer); UserMessage and ToolResultMessage
// used to implement it by value, which meant a Context could hold a mix of
// value and pointer forms and every type switch had to spell both.

import (
	"encoding/json"
	"reflect"
	"testing"
)

var messageType = reflect.TypeOf((*Message)(nil)).Elem()

// TestMessageIsImplementedOnPointerReceivers is the whole point of the change:
// the pointer type satisfies Message and the value type does not, so the
// compiler rejects a value-form store into a []Message.
func TestMessageIsImplementedOnPointerReceivers(t *testing.T) {
	for _, tc := range []struct {
		name    string
		ptr     any
		value   any
		wantRol Role
	}{
		{"UserMessage", &UserMessage{}, UserMessage{}, RoleUser},
		{"AssistantMessage", &AssistantMessage{}, AssistantMessage{}, RoleAssistant},
		{"ToolResultMessage", &ToolResultMessage{}, ToolResultMessage{}, RoleToolResult},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !reflect.TypeOf(tc.ptr).Implements(messageType) {
				t.Errorf("*%s does not implement Message", tc.name)
			}
			if reflect.TypeOf(tc.value).Implements(messageType) {
				t.Errorf("%s (value) implements Message; it must not", tc.name)
			}
			if got := tc.ptr.(Message).MessageRole(); got != tc.wantRol {
				t.Errorf("MessageRole() = %q, want %q", got, tc.wantRol)
			}
		})
	}
}

// TestUnmarshalMessageProducesPointerForms pins the decode side: a serialized
// session restores into the same pointer forms a caller would construct.
func TestUnmarshalMessageProducesPointerForms(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want any
	}{
		{"user", `{"role":"user","content":"hi","timestamp":1}`, &UserMessage{}},
		{"assistant", `{"role":"assistant","content":[],"api":"anthropic-messages","provider":"anthropic","model":"m","usage":{},"stopReason":"stop","timestamp":1}`, &AssistantMessage{}},
		{"toolResult", `{"role":"toolResult","toolCallId":"c","toolName":"t","content":[],"isError":false,"timestamp":1}`, &ToolResultMessage{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := UnmarshalMessage(json.RawMessage(tc.raw))
			if err != nil {
				t.Fatalf("UnmarshalMessage: %v", err)
			}
			if reflect.TypeOf(got) != reflect.TypeOf(tc.want) {
				t.Fatalf("decoded %T, want %T", got, tc.want)
			}
		})
	}
}

// TestSessionRoundTripPreservesPointerForms drives the full Context codec.
func TestSessionRoundTripPreservesPointerForms(t *testing.T) {
	original := Context{
		SystemPrompt: "be brief",
		Messages: []Message{
			&UserMessage{Content: UserText("hi"), Timestamp: 1},
			&AssistantMessage{
				Content: []AssistantContentPart{TextContent{Text: "hello"}},
				Api:     ApiAnthropicMessages, Provider: "anthropic", Model: "m",
				StopReason: StopReasonToolUse, Timestamp: 2,
			},
			&ToolResultMessage{ToolCallID: "c", ToolName: "t", Content: []UserContentPart{}, Timestamp: 3},
		},
	}

	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var restored Context
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if len(restored.Messages) != 3 {
		t.Fatalf("restored %d messages, want 3", len(restored.Messages))
	}
	for i, want := range []any{&UserMessage{}, &AssistantMessage{}, &ToolResultMessage{}} {
		if reflect.TypeOf(restored.Messages[i]) != reflect.TypeOf(want) {
			t.Errorf("message %d decoded %T, want %T", i, restored.Messages[i], want)
		}
	}
	if got := restored.Messages[0].(*UserMessage).Content.Plain; got == nil || *got != "hi" {
		t.Errorf("user content did not round-trip: %v", got)
	}
	if got := restored.Messages[2].(*ToolResultMessage).ToolCallID; got != "c" {
		t.Errorf("toolCallId = %q, want c", got)
	}
}
