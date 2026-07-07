package ai

// Ports: packages/ai/src/api/lazy.ts

import (
	"context"
	"time"
)

// NewSetupErrorMessage builds the zero-usage error AssistantMessage emitted
// when stream setup (auth resolution, dispatch) fails before the adapter runs.
func NewSetupErrorMessage(model *Model, err error) *AssistantMessage {
	return &AssistantMessage{
		Content:      []AssistantContentPart{},
		Api:          model.Api,
		Provider:     model.Provider,
		Model:        model.ID,
		StopReason:   StopReasonError,
		ErrorMessage: err.Error(),
		Timestamp:    time.Now().UnixMilli(),
	}
}

// LazyStream returns a Stream synchronously while running setup (auth
// resolution, provider dispatch) in a goroutine behind it. Setup failures
// terminate the stream with an in-band error event — this is why invoked
// streams never fail with a Go error.
func LazyStream(ctx context.Context, model *Model, setup func(ctx context.Context) (*Stream, error)) *Stream {
	outer := NewStream()
	go func() {
		inner, err := setup(ctx)
		if err != nil {
			message := NewSetupErrorMessage(model, err)
			outer.Push(ErrorEvent{Reason: StopReasonError, Error: message})
			return
		}
		for ev := range inner.Events() {
			outer.Push(ev)
		}
		// A well-behaved inner stream terminates via done/error, which also
		// completes outer. End covers inner streams closed without a terminal.
		outer.End(nil)
	}()
	return outer
}
