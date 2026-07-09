package ai

// Wire-format round-trip tests: the JSON produced by this package must match
// the TS package byte-for-byte in structure (field names, discriminators,
// string-vs-array user content) so persisted sessions interoperate.

import (
	"encoding/json"
	"reflect"
	"testing"
)

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestContentPartWireFormat(t *testing.T) {
	cases := []struct {
		name string
		part ContentPart
		want string
	}{
		{"text", TextContent{Text: "hi"}, `{"type":"text","text":"hi"}`},
		{"text with signature", TextContent{Text: "hi", TextSignature: "sig"}, `{"type":"text","text":"hi","textSignature":"sig"}`},
		{"thinking", ThinkingContent{Thinking: "hmm"}, `{"type":"thinking","thinking":"hmm"}`},
		{"redacted thinking", ThinkingContent{Thinking: "", ThinkingSignature: "enc", Redacted: true}, `{"type":"thinking","thinking":"","thinkingSignature":"enc","redacted":true}`},
		{"image", ImageContent{Data: "AAAA", MimeType: "image/png"}, `{"type":"image","data":"AAAA","mimeType":"image/png"}`},
		{"toolCall", ToolCall{ID: "call_1", Name: "get_weather", Arguments: map[string]any{"city": "Paris"}}, `{"type":"toolCall","id":"call_1","name":"get_weather","arguments":{"city":"Paris"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mustJSON(t, tc.part)
			if got != tc.want {
				t.Errorf("marshal = %s, want %s", got, tc.want)
			}
			back, err := UnmarshalContentPart([]byte(got))
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if !reflect.DeepEqual(back, tc.part) {
				t.Errorf("round trip = %#v, want %#v", back, tc.part)
			}
		})
	}
}

func TestUserMessageStringContentRoundTrip(t *testing.T) {
	msg := &UserMessage{Content: UserText("hello"), Timestamp: 123}
	got := mustJSON(t, msg)
	want := `{"role":"user","content":"hello","timestamp":123}`
	if got != want {
		t.Fatalf("marshal = %s, want %s", got, want)
	}
	back, err := UnmarshalMessage([]byte(got))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	user, ok := back.(*UserMessage)
	if !ok {
		t.Fatalf("wrong type %T", back)
	}
	if user.Content.Plain == nil || *user.Content.Plain != "hello" {
		t.Errorf("plain content lost: %#v", user.Content)
	}
}

func TestUserMessageBlockContentRoundTrip(t *testing.T) {
	msg := &UserMessage{
		Content: UserBlocks(
			TextContent{Text: "look at this"},
			ImageContent{Data: "AAAA", MimeType: "image/png"},
		),
		Timestamp: 5,
	}
	got := mustJSON(t, msg)
	want := `{"role":"user","content":[{"type":"text","text":"look at this"},{"type":"image","data":"AAAA","mimeType":"image/png"}],"timestamp":5}`
	if got != want {
		t.Fatalf("marshal = %s, want %s", got, want)
	}
	back, err := UnmarshalMessage([]byte(got))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if mustJSON(t, back) != want {
		t.Errorf("round trip drifted: %s", mustJSON(t, back))
	}
}

func TestAssistantMessageRoundTrip(t *testing.T) {
	reasoning := 3
	msg := &AssistantMessage{
		Content: []AssistantContentPart{
			ThinkingContent{Thinking: "let me think", ThinkingSignature: "sig1"},
			TextContent{Text: "answer"},
			ToolCall{ID: "tc_1", Name: "search", Arguments: map[string]any{"q": "go"}},
		},
		Api:        ApiAnthropicMessages,
		Provider:   "anthropic",
		Model:      "claude-opus-4-8",
		ResponseID: "msg_123",
		Usage: Usage{
			Input: 10, Output: 20, CacheRead: 1, CacheWrite: 2,
			Reasoning: &reasoning, TotalTokens: 33,
			Cost: UsageCost{Input: 0.1, Output: 0.2, CacheRead: 0.01, CacheWrite: 0.02, Total: 0.33},
		},
		StopReason: StopReasonToolUse,
		Timestamp:  1700000000000,
	}
	got := mustJSON(t, msg)
	back, err := UnmarshalMessage([]byte(got))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	assistant, ok := back.(*AssistantMessage)
	if !ok {
		t.Fatalf("wrong type %T", back)
	}
	if mustJSON(t, assistant) != got {
		t.Errorf("round trip drifted:\n first=%s\nsecond=%s", got, mustJSON(t, assistant))
	}
	if len(assistant.Content) != 3 {
		t.Fatalf("content length = %d", len(assistant.Content))
	}
	if _, ok := assistant.Content[0].(ThinkingContent); !ok {
		t.Errorf("content[0] type = %T", assistant.Content[0])
	}
	if tc, ok := assistant.Content[2].(ToolCall); !ok || tc.Arguments["q"] != "go" {
		t.Errorf("content[2] = %#v", assistant.Content[2])
	}
	if assistant.Usage.Reasoning == nil || *assistant.Usage.Reasoning != 3 {
		t.Errorf("reasoning lost: %#v", assistant.Usage)
	}
}

func TestToolResultMessageRoundTrip(t *testing.T) {
	msg := &ToolResultMessage{
		ToolCallID: "tc_1",
		ToolName:   "search",
		Content:    []UserContentPart{TextContent{Text: "result"}},
		IsError:    false,
		Timestamp:  7,
	}
	got := mustJSON(t, msg)
	want := `{"role":"toolResult","toolCallId":"tc_1","toolName":"search","content":[{"type":"text","text":"result"}],"isError":false,"timestamp":7}`
	if got != want {
		t.Fatalf("marshal = %s, want %s", got, want)
	}
	back, err := UnmarshalMessage([]byte(got))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if mustJSON(t, back) != want {
		t.Errorf("round trip drifted: %s", mustJSON(t, back))
	}
}

func TestContextRoundTrip(t *testing.T) {
	raw := `{"systemPrompt":"be nice","messages":[{"role":"user","content":"hi","timestamp":1},{"role":"assistant","content":[{"type":"text","text":"hello"}],"api":"anthropic-messages","provider":"anthropic","model":"m","usage":{"input":1,"output":2,"cacheRead":0,"cacheWrite":0,"totalTokens":3,"cost":{"input":0,"output":0,"cacheRead":0,"cacheWrite":0,"total":0}},"stopReason":"stop","timestamp":2}],"tools":[{"name":"t","description":"d","parameters":{"type":"object"}}]}`
	var chat Context
	if err := json.Unmarshal([]byte(raw), &chat); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if chat.SystemPrompt != "be nice" || len(chat.Messages) != 2 || len(chat.Tools) != 1 {
		t.Fatalf("bad decode: %+v", chat)
	}
	if _, ok := chat.Messages[1].(*AssistantMessage); !ok {
		t.Errorf("messages[1] type = %T", chat.Messages[1])
	}
}

func TestEventWireFormat(t *testing.T) {
	partial := &AssistantMessage{Api: "a", Provider: "p", Model: "m", StopReason: StopReasonStop}
	ev := TextDeltaEvent{ContentIndex: 1, Delta: "x", Partial: partial}
	got := mustJSON(t, ev)
	var probe map[string]any
	if err := json.Unmarshal([]byte(got), &probe); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if probe["type"] != "text_delta" || probe["contentIndex"] != float64(1) || probe["delta"] != "x" {
		t.Errorf("wire form = %s", got)
	}
	back, err := UnmarshalEvent([]byte(got))
	if err != nil {
		t.Fatalf("UnmarshalEvent: %v", err)
	}
	if back.EventKind() != EventTextDelta {
		t.Errorf("kind = %v", back.EventKind())
	}

	done := DoneEvent{Reason: StopReasonToolUse, Message: partial}
	backDone, err := UnmarshalEvent([]byte(mustJSON(t, done)))
	if err != nil {
		t.Fatalf("UnmarshalEvent(done): %v", err)
	}
	if !IsTerminalEvent(backDone) || EventResult(backDone) == nil {
		t.Error("done event must be terminal with a result")
	}
}
