package ai

import (
	"context"
	"sync"
)

// Ports: packages/ai/src/utils/event-stream.ts (EventStream,
// AssistantMessageEventStream)

// Stream is the unified streaming handle returned by every Stream call.
//
// It mirrors the TS AssistantMessageEventStream contract:
//   - producers Push events and never block (the queue is unbounded, so a
//     consumer that only waits on Result never deadlocks the producer);
//   - the stream completes on the first terminal event (done/error), whose
//     AssistantMessage becomes the final result;
//   - events pushed after completion are dropped;
//   - Result never fails because of a provider error — failures are encoded
//     in the returned message's StopReason/ErrorMessage.
type Stream struct {
	mu     sync.Mutex
	cond   *sync.Cond
	queue  []Event
	done   bool
	result *AssistantMessage
	// resultCh closes when the final result is set (or the stream is ended).
	resultCh chan struct{}
}

// NewStream creates an open Stream.
func NewStream() *Stream {
	s := &Stream{resultCh: make(chan struct{})}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// Push delivers an event to consumers. A terminal event (done/error) completes
// the stream and resolves Result. Events pushed after completion are dropped.
// Push never blocks.
func (s *Stream) Push(ev Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done {
		return
	}
	if IsTerminalEvent(ev) {
		s.done = true
		s.result = EventResult(ev)
		close(s.resultCh)
	}
	s.queue = append(s.queue, ev)
	s.cond.Broadcast()
}

// End closes the stream without a terminal event, waking all consumers.
// If result is non-nil it becomes the final result. This mirrors TS
// EventStream.end and exists for test doubles and adapters that complete
// out-of-band; normal adapters terminate by pushing done/error.
func (s *Stream) End(result *AssistantMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done {
		return
	}
	s.done = true
	s.result = result
	close(s.resultCh)
	s.cond.Broadcast()
}

// next pops the next queued event, blocking until one is available, the stream
// is drained, or ctx is cancelled. ok=false means there will be no more events.
//
// The ctx check sits inside the cond.Wait loop rather than around it: a waiter
// only ever wakes on a Broadcast, so cancellation is delivered by broadcasting
// (see wakeOnCancel) and then observed here on the next loop iteration.
func (s *Stream) next(ctx context.Context) (Event, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for {
		if ctx.Err() != nil {
			return nil, false
		}
		if len(s.queue) > 0 {
			ev := s.queue[0]
			s.queue = s.queue[1:]
			return ev, true
		}
		if s.done {
			return nil, false
		}
		s.cond.Wait()
	}
}

// wakeOnCancel broadcasts once ctx is cancelled, so a pump asleep in
// s.cond.Wait() re-checks ctx.Err() and gives up. It returns as soon as stop
// closes, which is what keeps it from outliving its pump: for a
// context.Background() consumer ctx.Done() is nil and never fires, and a
// watcher parked on it forever would be a goroutine leak of its own.
//
// The Broadcast is issued under s.mu so it cannot land in the window between
// next's ctx.Err() check and its cond.Wait() — a lost wakeup there would park
// the pump for good, which is the very bug this exists to prevent.
func (s *Stream) wakeOnCancel(ctx context.Context, stop <-chan struct{}) {
	select {
	case <-ctx.Done():
		s.mu.Lock()
		s.cond.Broadcast()
		s.mu.Unlock()
	case <-stop:
	}
}

// Events returns a channel of all events, closed after the terminal event or
// as soon as ctx is cancelled. Each call returns a fresh channel, but events
// are consumed from a shared queue — use a single consumer per stream.
//
// A consumer that stops ranging early MUST cancel ctx; the channel is
// unbuffered, so the goroutine feeding it would otherwise block forever on the
// next event. Pass context.Background() only when the loop is guaranteed to
// run to completion.
//
// This takes a context where upstream's EventStream async-iterator does not;
// see docs/PORTING.md.
func (s *Stream) Events(ctx context.Context) <-chan Event {
	ch := make(chan Event)
	stop := make(chan struct{})
	go s.wakeOnCancel(ctx, stop)
	go func() {
		defer close(ch)
		defer close(stop)
		for {
			ev, ok := s.next(ctx)
			if !ok {
				return
			}
			select {
			case ch <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

// Result blocks until the stream terminates and returns the final
// AssistantMessage. Provider failures do NOT return a Go error: the message
// carries StopReason "error"/"aborted" and ErrorMessage. The returned error is
// only non-nil when ctx is cancelled before the stream terminates, or when the
// stream was Ended without a result.
func (s *Stream) Result(ctx context.Context) (*AssistantMessage, error) {
	select {
	case <-s.resultCh:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.result == nil {
		return nil, ErrStreamEndedWithoutResult
	}
	return s.result, nil
}

// errStreamEndedWithoutResult is returned by Result when End(nil) closed the
// stream with no terminal message.
type streamError string

func (e streamError) Error() string { return string(e) }

// ErrStreamEndedWithoutResult reports a stream closed via End(nil).
const ErrStreamEndedWithoutResult = streamError("ai: stream ended without a result")
