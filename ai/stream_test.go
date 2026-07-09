package ai

// Ports the EventStream contract from packages/ai/src/utils/event-stream.ts
// and the protocol invariants exercised by packages/ai/test/stream.test.ts.

import (
	"context"
	"runtime"
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
	for ev := range s.Events(context.Background()) {
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
	for range s.Events(context.Background()) {
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
		for range s.Events(context.Background()) {
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
	for range s.Events(context.Background()) {
	}
	got, err := s.Result(context.Background())
	if err != nil || got != final {
		t.Fatalf("got %v, %v", got, err)
	}
}

// --- Events(ctx) cancellation ------------------------------------------------
//
// Events' pump goroutine sends on an unbuffered channel. Without a cancel path
// a consumer that stops ranging early strands that goroutine forever, holding
// the stream and every event it references live. These tests pin the contract
// that makes the leak impossible: cancelling ctx unblocks the pump wherever it
// is parked -- mid-send on the channel, or asleep in the condition variable
// waiting for an event that never comes.

// pumpExited reports whether the pump closed ch (its `defer close(ch)`) within
// the deadline. Channel closure is the pump's own done-signal: it can only
// happen from inside the goroutine, as its last act.
func pumpExited(t *testing.T, ch <-chan Event, within time.Duration) bool {
	t.Helper()
	deadline := time.After(within)
	for {
		select {
		case _, open := <-ch:
			if !open {
				return true
			}
			// A buffered-up event we don't care about; keep draining.
		case <-deadline:
			return false
		}
	}
}

// TestStreamEventsPumpExitsWhenConsumerStopsEarly is the leak itself: the
// consumer takes one event and walks away from a stream that never terminates.
// The pump is parked on `ch <- ev` with the next event.
func TestStreamEventsPumpExitsWhenConsumerStopsEarly(t *testing.T) {
	s := NewStream()
	msg := &AssistantMessage{StopReason: StopReasonStop}
	ctx, cancel := context.WithCancel(context.Background())

	ch := s.Events(ctx)
	s.Push(StartEvent{Partial: msg})
	s.Push(TextDeltaEvent{ContentIndex: 0, Delta: "one", Partial: msg})
	s.Push(TextDeltaEvent{ContentIndex: 0, Delta: "two", Partial: msg})

	<-ch // consume exactly one event, then break out of the loop
	cancel()

	if !pumpExited(t, ch, time.Second) {
		t.Fatal("pump goroutine still running after cancel: Events leaked it")
	}
}

// TestStreamEventsPumpExitsWhileWaitingForEvents parks the pump in
// s.cond.Wait() instead: the queue is empty and the stream never terminates,
// so only a Broadcast can wake it. sync.Cond cannot select on ctx.Done().
func TestStreamEventsPumpExitsWhileWaitingForEvents(t *testing.T) {
	s := NewStream()
	ctx, cancel := context.WithCancel(context.Background())

	ch := s.Events(ctx)
	time.Sleep(20 * time.Millisecond) // let the pump reach cond.Wait()
	cancel()

	if !pumpExited(t, ch, time.Second) {
		t.Fatal("pump goroutine still parked in cond.Wait() after cancel")
	}
}

// TestStreamEventsCancelledBeforeFirstEvent covers the already-cancelled ctx.
func TestStreamEventsCancelledBeforeFirstEvent(t *testing.T) {
	s := NewStream()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if !pumpExited(t, s.Events(ctx), time.Second) {
		t.Fatal("Events(cancelled ctx) did not close its channel")
	}
}

// TestStreamEventsBackgroundContextDrainsFully is the regression guard for
// every existing full-drain consumer: nothing about their behavior changes.
func TestStreamEventsBackgroundContextDrainsFully(t *testing.T) {
	s := NewStream()
	msg := &AssistantMessage{StopReason: StopReasonStop}
	go func() {
		s.Push(StartEvent{Partial: msg})
		s.Push(TextDeltaEvent{ContentIndex: 0, Delta: "hi", Partial: msg})
		s.Push(DoneEvent{Reason: StopReasonStop, Message: msg})
	}()

	count := 0
	for range s.Events(context.Background()) {
		count++
	}
	if count != 3 {
		t.Fatalf("drained %d events, want 3", count)
	}
}

// TestStreamEventsLeavesNoGoroutinesBehind guards the cancellation plumbing
// itself. Waking a pump parked in sync.Cond needs a watcher goroutine on
// ctx.Done(); with context.Background() that channel is nil and never fires,
// so the watcher must instead exit when the pump finishes. If it doesn't,
// every completed stream leaks one goroutine -- trading the bug for a quieter
// one.
func TestStreamEventsLeavesNoGoroutinesBehind(t *testing.T) {
	settle := func() int {
		var n int
		for i := 0; i < 50; i++ {
			n = runtime.NumGoroutine()
			time.Sleep(10 * time.Millisecond)
			if runtime.NumGoroutine() == n {
				return n
			}
		}
		return n
	}

	before := settle()

	for i := 0; i < 50; i++ {
		s := NewStream()
		msg := &AssistantMessage{StopReason: StopReasonStop}
		s.Push(DoneEvent{Reason: StopReasonStop, Message: msg})
		for range s.Events(context.Background()) {
		}
	}

	if after := settle(); after > before+2 {
		t.Fatalf("goroutines: %d before, %d after 50 fully-drained streams", before, after)
	}
}
