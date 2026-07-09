package ai

// Ports the EventStream contract from packages/ai/src/utils/event-stream.ts
// and the protocol invariants exercised by packages/ai/test/stream.test.ts.

import (
	"context"
	"testing"
	"time"
)

func TestStreamDeliversEventsInOrder(t *testing.T) {
	s := NewStream()
	msg := &AssistantMessage{StopReason: StopReasonStop}
	go func() {
		s.Push(StartEvent{Partial: msg})
		s.Push(TextStartEvent{ContentIndex: 0, Partial: msg})
		s.Push(TextDeltaEvent{ContentIndex: 0, Delta: "he", Partial: msg})
		s.Push(TextDeltaEvent{ContentIndex: 0, Delta: "llo", Partial: msg})
		s.Push(TextEndEvent{ContentIndex: 0, Content: "hello", Partial: msg})
		s.Push(DoneEvent{Reason: StopReasonStop, Message: msg})
	}()

	var kinds []EventType
	for ev := range s.Events() {
		kinds = append(kinds, ev.EventKind())
	}
	want := []EventType{EventStart, EventTextStart, EventTextDelta, EventTextDelta, EventTextEnd, EventDone}
	if len(kinds) != len(want) {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("kinds = %v, want %v", kinds, want)
		}
	}
}

func TestStreamResultWithoutDraining(t *testing.T) {
	// The producer must never block even when nobody consumes events
	// (complete() only awaits the result).
	s := NewStream()
	final := &AssistantMessage{StopReason: StopReasonStop}
	go func() {
		for i := 0; i < 10000; i++ {
			s.Push(TextDeltaEvent{ContentIndex: 0, Delta: "x", Partial: final})
		}
		s.Push(DoneEvent{Reason: StopReasonStop, Message: final})
	}()
	got, err := s.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if got != final {
		t.Errorf("wrong result: %#v", got)
	}
}

func TestStreamErrorEventResolvesResult(t *testing.T) {
	s := NewStream()
	errMsg := &AssistantMessage{StopReason: StopReasonError, ErrorMessage: "boom"}
	s.Push(ErrorEvent{Reason: StopReasonError, Error: errMsg})
	got, err := s.Result(context.Background())
	if err != nil {
		t.Fatalf("Result must not fail on in-band errors: %v", err)
	}
	if got.StopReason != StopReasonError || got.ErrorMessage != "boom" {
		t.Errorf("got %#v", got)
	}
}

func TestStreamDropsEventsAfterTerminal(t *testing.T) {
	s := NewStream()
	final := &AssistantMessage{StopReason: StopReasonStop}
	s.Push(DoneEvent{Reason: StopReasonStop, Message: final})
	s.Push(TextDeltaEvent{Delta: "late", Partial: final})
	s.Push(DoneEvent{Reason: StopReasonStop, Message: &AssistantMessage{}})

	count := 0
	for range s.Events() {
		count++
	}
	if count != 1 {
		t.Errorf("events after terminal must be dropped, got %d events", count)
	}
	got, _ := s.Result(context.Background())
	if got != final {
		t.Error("result must be the first terminal message")
	}
}

func TestStreamResultContextCancellation(t *testing.T) {
	s := NewStream()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := s.Result(ctx)
	if err == nil {
		t.Fatal("expected context error for never-terminating stream")
	}
}

func TestStreamEndWakesConsumers(t *testing.T) {
	s := NewStream()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range s.Events() {
		}
	}()
	s.End(nil)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("consumer not woken by End")
	}
	if _, err := s.Result(context.Background()); err == nil {
		t.Error("End(nil) must surface ErrStreamEndedWithoutResult")
	}
}

func TestStreamConsumeThenResult(t *testing.T) {
	s := NewStream()
	final := &AssistantMessage{StopReason: StopReasonToolUse}
	go func() {
		s.Push(StartEvent{Partial: final})
		s.Push(DoneEvent{Reason: StopReasonToolUse, Message: final})
	}()
	for range s.Events() {
	}
	got, err := s.Result(context.Background())
	if err != nil || got != final {
		t.Fatalf("got %v, %v", got, err)
	}
}
