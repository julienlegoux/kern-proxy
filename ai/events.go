package ai

import (
	"encoding/json"
	"fmt"
)

// Ports: packages/ai/src/types.ts (AssistantMessageEvent)

// EventType discriminates streaming events on the wire ("type" field).
type EventType string

const (
	EventStart         EventType = "start"
	EventTextStart     EventType = "text_start"
	EventTextDelta     EventType = "text_delta"
	EventTextEnd       EventType = "text_end"
	EventThinkingStart EventType = "thinking_start"
	EventThinkingDelta EventType = "thinking_delta"
	EventThinkingEnd   EventType = "thinking_end"
	EventToolCallStart EventType = "toolcall_start"
	EventToolCallDelta EventType = "toolcall_delta"
	EventToolCallEnd   EventType = "toolcall_end"
	EventDone          EventType = "done"
	EventError         EventType = "error"
)

// Event is the closed union of streaming events.
//
// Protocol: a stream emits `start` first, then partial updates, and terminates
// with exactly one of `done` (final successful AssistantMessage) or `error`
// (final AssistantMessage with StopReason "error"/"aborted" and ErrorMessage).
// Every non-terminal event carries Partial — a snapshot of the accumulating
// AssistantMessage.
type Event interface {
	EventKind() EventType
}

// StartEvent opens the stream.
type StartEvent struct {
	Partial *AssistantMessage `json:"partial"`
}

func (StartEvent) EventKind() EventType { return EventStart }

// TextStartEvent begins a text block at ContentIndex.
type TextStartEvent struct {
	ContentIndex int               `json:"contentIndex"`
	Partial      *AssistantMessage `json:"partial"`
}

func (TextStartEvent) EventKind() EventType { return EventTextStart }

// TextDeltaEvent appends Delta to the text block at ContentIndex.
type TextDeltaEvent struct {
	ContentIndex int               `json:"contentIndex"`
	Delta        string            `json:"delta"`
	Partial      *AssistantMessage `json:"partial"`
}

func (TextDeltaEvent) EventKind() EventType { return EventTextDelta }

// TextEndEvent closes the text block; Content is its final text.
type TextEndEvent struct {
	ContentIndex int               `json:"contentIndex"`
	Content      string            `json:"content"`
	Partial      *AssistantMessage `json:"partial"`
}

func (TextEndEvent) EventKind() EventType { return EventTextEnd }

// ThinkingStartEvent begins a thinking block at ContentIndex.
type ThinkingStartEvent struct {
	ContentIndex int               `json:"contentIndex"`
	Partial      *AssistantMessage `json:"partial"`
}

func (ThinkingStartEvent) EventKind() EventType { return EventThinkingStart }

// ThinkingDeltaEvent appends Delta to the thinking block at ContentIndex.
type ThinkingDeltaEvent struct {
	ContentIndex int               `json:"contentIndex"`
	Delta        string            `json:"delta"`
	Partial      *AssistantMessage `json:"partial"`
}

func (ThinkingDeltaEvent) EventKind() EventType { return EventThinkingDelta }

// ThinkingEndEvent closes the thinking block; Content is its final text.
type ThinkingEndEvent struct {
	ContentIndex int               `json:"contentIndex"`
	Content      string            `json:"content"`
	Partial      *AssistantMessage `json:"partial"`
}

func (ThinkingEndEvent) EventKind() EventType { return EventThinkingEnd }

// ToolCallStartEvent begins a tool-call block at ContentIndex.
type ToolCallStartEvent struct {
	ContentIndex int               `json:"contentIndex"`
	Partial      *AssistantMessage `json:"partial"`
}

func (ToolCallStartEvent) EventKind() EventType { return EventToolCallStart }

// ToolCallDeltaEvent appends raw argument-JSON Delta to the tool-call block.
type ToolCallDeltaEvent struct {
	ContentIndex int               `json:"contentIndex"`
	Delta        string            `json:"delta"`
	Partial      *AssistantMessage `json:"partial"`
}

func (ToolCallDeltaEvent) EventKind() EventType { return EventToolCallDelta }

// ToolCallEndEvent closes the tool-call block with its final parsed ToolCall.
type ToolCallEndEvent struct {
	ContentIndex int               `json:"contentIndex"`
	ToolCall     ToolCall          `json:"toolCall"`
	Partial      *AssistantMessage `json:"partial"`
}

func (ToolCallEndEvent) EventKind() EventType { return EventToolCallEnd }

// DoneEvent terminates a successful stream. Reason is "stop", "length", or
// "toolUse".
type DoneEvent struct {
	Reason  StopReason        `json:"reason"`
	Message *AssistantMessage `json:"message"`
}

func (DoneEvent) EventKind() EventType { return EventDone }

// ErrorEvent terminates a failed or aborted stream. Reason is "error" or
// "aborted"; Error is the final AssistantMessage carrying ErrorMessage.
type ErrorEvent struct {
	Reason StopReason        `json:"reason"`
	Error  *AssistantMessage `json:"error"`
}

func (ErrorEvent) EventKind() EventType { return EventError }

// IsTerminalEvent reports whether ev completes the stream.
func IsTerminalEvent(ev Event) bool {
	k := ev.EventKind()
	return k == EventDone || k == EventError
}

// EventResult extracts the final AssistantMessage from a terminal event, or
// nil for non-terminal events.
func EventResult(ev Event) *AssistantMessage {
	switch e := ev.(type) {
	case DoneEvent:
		return e.Message
	case ErrorEvent:
		return e.Error
	default:
		return nil
	}
}

func marshalEvent(t EventType, v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	tag, err := json.Marshal(struct {
		Type EventType `json:"type"`
	}{t})
	if err != nil {
		return nil, err
	}
	if string(raw) == "{}" {
		return tag, nil
	}
	out := append(tag[:len(tag)-1], ',')
	out = append(out, raw[1:]...)
	return out, nil
}

func (e StartEvent) MarshalJSON() ([]byte, error) {
	type alias StartEvent
	return marshalEvent(EventStart, alias(e))
}
func (e TextStartEvent) MarshalJSON() ([]byte, error) {
	type alias TextStartEvent
	return marshalEvent(EventTextStart, alias(e))
}
func (e TextDeltaEvent) MarshalJSON() ([]byte, error) {
	type alias TextDeltaEvent
	return marshalEvent(EventTextDelta, alias(e))
}
func (e TextEndEvent) MarshalJSON() ([]byte, error) {
	type alias TextEndEvent
	return marshalEvent(EventTextEnd, alias(e))
}
func (e ThinkingStartEvent) MarshalJSON() ([]byte, error) {
	type alias ThinkingStartEvent
	return marshalEvent(EventThinkingStart, alias(e))
}
func (e ThinkingDeltaEvent) MarshalJSON() ([]byte, error) {
	type alias ThinkingDeltaEvent
	return marshalEvent(EventThinkingDelta, alias(e))
}
func (e ThinkingEndEvent) MarshalJSON() ([]byte, error) {
	type alias ThinkingEndEvent
	return marshalEvent(EventThinkingEnd, alias(e))
}
func (e ToolCallStartEvent) MarshalJSON() ([]byte, error) {
	type alias ToolCallStartEvent
	return marshalEvent(EventToolCallStart, alias(e))
}
func (e ToolCallDeltaEvent) MarshalJSON() ([]byte, error) {
	type alias ToolCallDeltaEvent
	return marshalEvent(EventToolCallDelta, alias(e))
}
func (e ToolCallEndEvent) MarshalJSON() ([]byte, error) {
	type alias ToolCallEndEvent
	return marshalEvent(EventToolCallEnd, alias(e))
}
func (e DoneEvent) MarshalJSON() ([]byte, error) {
	type alias DoneEvent
	return marshalEvent(EventDone, alias(e))
}
func (e ErrorEvent) MarshalJSON() ([]byte, error) {
	type alias ErrorEvent
	return marshalEvent(EventError, alias(e))
}

// UnmarshalEvent decodes one streaming event by its "type" discriminator.
func UnmarshalEvent(data []byte) (Event, error) {
	var probe struct {
		Type EventType `json:"type"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, err
	}
	switch probe.Type {
	case EventStart:
		return decodeEvent[StartEvent](data)
	case EventTextStart:
		return decodeEvent[TextStartEvent](data)
	case EventTextDelta:
		return decodeEvent[TextDeltaEvent](data)
	case EventTextEnd:
		return decodeEvent[TextEndEvent](data)
	case EventThinkingStart:
		return decodeEvent[ThinkingStartEvent](data)
	case EventThinkingDelta:
		return decodeEvent[ThinkingDeltaEvent](data)
	case EventThinkingEnd:
		return decodeEvent[ThinkingEndEvent](data)
	case EventToolCallStart:
		return decodeEvent[ToolCallStartEvent](data)
	case EventToolCallDelta:
		return decodeEvent[ToolCallDeltaEvent](data)
	case EventToolCallEnd:
		return decodeEvent[ToolCallEndEvent](data)
	case EventDone:
		return decodeEvent[DoneEvent](data)
	case EventError:
		return decodeEvent[ErrorEvent](data)
	default:
		return nil, fmt.Errorf("ai: unknown event type %q", probe.Type)
	}
}

func decodeEvent[T Event](data []byte) (Event, error) {
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return v, nil
}
