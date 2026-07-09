package faux

// Ports: packages/ai/test/faux-provider.test.ts (core cases) plus the faux
// abort behavior exercised upstream via stream.test.ts/abort.test.ts.

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
)

func userContext(text string) ai.Context {
	return ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText(text), Timestamp: 1}}}
}

func complete(t *testing.T, models ai.Models, model *ai.Model, chat ai.Context, opts *ai.StreamOptions) *ai.AssistantMessage {
	t.Helper()
	result, err := models.Complete(context.Background(), model, chat, opts)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	return result
}

func newHarness(options *Options) (*Handle, ai.MutableModels) {
	handle := New(options)
	models := ai.CreateModels(nil)
	models.SetProvider(handle.Provider)
	return handle, models
}

func TestRegistersProviderAndEstimatesUsage(t *testing.T) {
	handle, models := newHarness(nil)
	handle.SetResponses(Step(TextMessage("hello world", nil)))

	response := complete(t, models, handle.GetModel(""), userContext("hi there"), nil)

	if len(response.Content) != 1 {
		t.Fatalf("content = %#v", response.Content)
	}
	if text, ok := response.Content[0].(ai.TextContent); !ok || text.Text != "hello world" {
		t.Errorf("content[0] = %#v", response.Content[0])
	}
	if response.Usage.Input <= 0 || response.Usage.Output <= 0 {
		t.Errorf("usage = %+v", response.Usage)
	}
	if response.Usage.TotalTokens != response.Usage.Input+response.Usage.Output {
		t.Errorf("totalTokens = %d, want %d", response.Usage.TotalTokens, response.Usage.Input+response.Usage.Output)
	}
	if handle.State().CallCount() != 1 {
		t.Errorf("callCount = %d", handle.State().CallCount())
	}
}

func TestHelperBlocks(t *testing.T) {
	handle, models := newHarness(nil)
	handle.SetResponses(Step(AssistantMessage([]ai.AssistantContentPart{
		Thinking("pondering"),
		Text("answer"),
		ToolCall("search", map[string]any{"q": "go"}, &ToolCallOptions{ID: "tc_1"}),
	}, &AssistantMessageOptions{StopReason: ai.StopReasonToolUse})))

	response := complete(t, models, handle.GetModel(""), userContext("q"), nil)

	if response.StopReason != ai.StopReasonToolUse {
		t.Errorf("stopReason = %v", response.StopReason)
	}
	if len(response.Content) != 3 {
		t.Fatalf("content = %#v", response.Content)
	}
	if th, ok := response.Content[0].(ai.ThinkingContent); !ok || th.Thinking != "pondering" {
		t.Errorf("content[0] = %#v", response.Content[0])
	}
	if tc, ok := response.Content[2].(ai.ToolCall); !ok || tc.ID != "tc_1" || tc.Arguments["q"] != "go" {
		t.Errorf("content[2] = %#v", response.Content[2])
	}
}

func TestMultipleModelsAndModelAwareFactories(t *testing.T) {
	handle, models := newHarness(&Options{Models: []ModelDefinition{
		{ID: "faux-fast"},
		{ID: "faux-thinker", Reasoning: true},
	}})

	if handle.GetModel("") != handle.Models[0] {
		t.Error("GetModel(\"\") must return the first model")
	}
	if handle.GetModel("faux-fast").Reasoning {
		t.Error("faux-fast must not be reasoning")
	}
	if !handle.GetModel("faux-thinker").Reasoning {
		t.Error("faux-thinker must be reasoning")
	}

	factory := func(_ context.Context, _ ai.Context, _ *ai.StreamOptions, _ *State, model *ai.Model) (*ai.AssistantMessage, error) {
		return TextMessage(fmt.Sprintf("%s:%v", model.ID, model.Reasoning), nil), nil
	}
	handle.SetResponses(StepFunc(factory), StepFunc(factory))

	fast := complete(t, models, handle.GetModel("faux-fast"), userContext("x"), nil)
	thinker := complete(t, models, handle.GetModel("faux-thinker"), userContext("x"), nil)

	if fast.Content[0].(ai.TextContent).Text != "faux-fast:false" {
		t.Errorf("fast = %#v", fast.Content)
	}
	if thinker.Content[0].(ai.TextContent).Text != "faux-thinker:true" {
		t.Errorf("thinker = %#v", thinker.Content)
	}
}

func TestRewritesProvenanceOnReturnedMessages(t *testing.T) {
	handle, models := newHarness(&Options{
		Api:      "faux:test",
		Provider: "faux-provider",
		Models:   []ModelDefinition{{ID: "faux-model"}},
	})
	handle.SetResponses(Step(TextMessage("hi", nil)))

	response := complete(t, models, handle.GetModel("faux-model"), userContext("x"), nil)

	if response.Api != "faux:test" || response.Provider != "faux-provider" || response.Model != "faux-model" {
		t.Errorf("provenance = %s/%s/%s", response.Api, response.Provider, response.Model)
	}
}

func TestConsumesQueuedResponsesInOrderAndErrorsWhenExhausted(t *testing.T) {
	handle, models := newHarness(nil)
	handle.SetResponses(Step(TextMessage("first", nil)), Step(TextMessage("second", nil)))
	model := handle.GetModel("")

	first := complete(t, models, model, userContext("x"), nil)
	second := complete(t, models, model, userContext("x"), nil)
	exhausted := complete(t, models, model, userContext("x"), nil)

	if first.Content[0].(ai.TextContent).Text != "first" || second.Content[0].(ai.TextContent).Text != "second" {
		t.Errorf("order broken: %#v / %#v", first.Content, second.Content)
	}
	if exhausted.StopReason != ai.StopReasonError || exhausted.ErrorMessage != "No more faux responses queued" {
		t.Errorf("exhausted = %v %q", exhausted.StopReason, exhausted.ErrorMessage)
	}
	if handle.PendingResponseCount() != 0 || handle.State().CallCount() != 3 {
		t.Errorf("pending=%d calls=%d", handle.PendingResponseCount(), handle.State().CallCount())
	}
}

func TestReplaceAndAppendQueuedResponses(t *testing.T) {
	handle, models := newHarness(nil)
	model := handle.GetModel("")

	handle.SetResponses(Step(TextMessage("first", nil)))
	if got := complete(t, models, model, userContext("x"), nil); got.Content[0].(ai.TextContent).Text != "first" {
		t.Errorf("got %#v", got.Content)
	}
	if handle.PendingResponseCount() != 0 {
		t.Errorf("pending = %d", handle.PendingResponseCount())
	}

	handle.AppendResponses(Step(TextMessage("second", nil)))
	if handle.PendingResponseCount() != 1 {
		t.Errorf("pending = %d", handle.PendingResponseCount())
	}
	if got := complete(t, models, model, userContext("x"), nil); got.Content[0].(ai.TextContent).Text != "second" {
		t.Errorf("got %#v", got.Content)
	}

	handle.SetResponses(Step(TextMessage("third", nil)), Step(TextMessage("fourth", nil)))
	if handle.PendingResponseCount() != 2 {
		t.Errorf("pending = %d", handle.PendingResponseCount())
	}
	if got := complete(t, models, model, userContext("x"), nil); got.Content[0].(ai.TextContent).Text != "third" {
		t.Errorf("got %#v", got.Content)
	}
	if got := complete(t, models, model, userContext("x"), nil); got.Content[0].(ai.TextContent).Text != "fourth" {
		t.Errorf("got %#v", got.Content)
	}
}

func TestFactoryErrorBecomesInBandErrorEvent(t *testing.T) {
	handle, models := newHarness(nil)
	handle.SetResponses(StepFunc(func(context.Context, ai.Context, *ai.StreamOptions, *State, *ai.Model) (*ai.AssistantMessage, error) {
		return nil, fmt.Errorf("boom")
	}))

	stream := models.Stream(context.Background(), handle.GetModel(""), userContext("x"), nil)
	var events []ai.Event
	for ev := range stream.Events() {
		events = append(events, ev)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1 (just the error)", len(events))
	}
	errEvent, ok := events[0].(ai.ErrorEvent)
	if !ok {
		t.Fatalf("event = %#v", events[0])
	}
	if errEvent.Error.StopReason != ai.StopReasonError || errEvent.Error.ErrorMessage != "boom" {
		t.Errorf("error = %v %q", errEvent.Error.StopReason, errEvent.Error.ErrorMessage)
	}
}

func TestEventProtocolSequence(t *testing.T) {
	handle, models := newHarness(nil)
	handle.SetResponses(Step(AssistantMessage([]ai.AssistantContentPart{
		Thinking("think"),
		Text("some longer text that spans several chunks for sure"),
		ToolCall("run", map[string]any{"cmd": "ls -la"}, nil),
	}, &AssistantMessageOptions{StopReason: ai.StopReasonToolUse})))

	stream := models.Stream(context.Background(), handle.GetModel(""), userContext("x"), nil)
	var kinds []ai.EventType
	for ev := range stream.Events() {
		kinds = append(kinds, ev.EventKind())
		// Every non-terminal event carries a partial snapshot.
		if !ai.IsTerminalEvent(ev) {
			switch e := ev.(type) {
			case ai.StartEvent:
				if e.Partial == nil {
					t.Error("start without partial")
				}
			case ai.TextDeltaEvent:
				if e.Partial == nil {
					t.Error("delta without partial")
				}
			}
		}
	}

	joined := ""
	for _, k := range kinds {
		joined += string(k) + " "
	}
	if kinds[0] != ai.EventStart {
		t.Errorf("first event = %v", kinds[0])
	}
	if kinds[len(kinds)-1] != ai.EventDone {
		t.Errorf("last event = %v", kinds[len(kinds)-1])
	}
	for _, required := range []string{
		"thinking_start", "thinking_delta", "thinking_end",
		"text_start", "text_delta", "text_end",
		"toolcall_start", "toolcall_delta", "toolcall_end",
	} {
		if !strings.Contains(joined, required) {
			t.Errorf("missing %s in %s", required, joined)
		}
	}
}

func TestPromptCachingPerSessionID(t *testing.T) {
	handle, models := newHarness(nil)
	model := handle.GetModel("")
	opts := &ai.StreamOptions{SessionID: "session-1"}

	handle.SetResponses(Step(TextMessage("one", nil)), Step(TextMessage("two", nil)))

	first := complete(t, models, model, userContext("shared prefix message"), opts)
	if first.Usage.CacheRead != 0 || first.Usage.CacheWrite <= 0 {
		t.Errorf("first usage = %+v", first.Usage)
	}

	longer := ai.Context{Messages: []ai.Message{
		ai.UserMessage{Content: ai.UserText("shared prefix message"), Timestamp: 1},
		ai.UserMessage{Content: ai.UserText("and a follow-up"), Timestamp: 2},
	}}
	second := complete(t, models, model, longer, opts)
	if second.Usage.CacheRead <= 0 {
		t.Errorf("second usage = %+v (expected cache read on shared prefix)", second.Usage)
	}
}

func TestNoCacheSharingAcrossSessionsOrWithoutSessionID(t *testing.T) {
	handle, models := newHarness(nil)
	model := handle.GetModel("")
	chat := userContext("the same prompt")

	handle.SetResponses(Step(TextMessage("a", nil)), Step(TextMessage("b", nil)), Step(TextMessage("c", nil)))

	first := complete(t, models, model, chat, &ai.StreamOptions{SessionID: "s1"})
	if first.Usage.CacheWrite <= 0 {
		t.Errorf("first = %+v", first.Usage)
	}
	// Different session: no shared cache.
	second := complete(t, models, model, chat, &ai.StreamOptions{SessionID: "s2"})
	if second.Usage.CacheRead != 0 || second.Usage.CacheWrite <= 0 {
		t.Errorf("second = %+v", second.Usage)
	}
	// No sessionId: no cache at all.
	third := complete(t, models, model, chat, nil)
	if third.Usage.CacheRead != 0 || third.Usage.CacheWrite != 0 {
		t.Errorf("third = %+v", third.Usage)
	}
}

func TestAbortMidStream(t *testing.T) {
	handle, models := newHarness(&Options{TokensPerSecond: 50})
	handle.SetResponses(Step(TextMessage(strings.Repeat("long text ", 200), nil)))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := models.Stream(ctx, handle.GetModel(""), userContext("x"), nil)

	sawDelta := false
	var terminal ai.Event
	for ev := range stream.Events() {
		if ev.EventKind() == ai.EventTextDelta && !sawDelta {
			sawDelta = true
			cancel()
		}
		if ai.IsTerminalEvent(ev) {
			terminal = ev
		}
	}
	if !sawDelta {
		t.Fatal("no delta observed")
	}
	errEvent, ok := terminal.(ai.ErrorEvent)
	if !ok {
		t.Fatalf("terminal = %#v", terminal)
	}
	if errEvent.Reason != ai.StopReasonAborted || errEvent.Error.StopReason != ai.StopReasonAborted {
		t.Errorf("reason = %v", errEvent.Reason)
	}
	if errEvent.Error.ErrorMessage != "Request was aborted" {
		t.Errorf("errorMessage = %q", errEvent.Error.ErrorMessage)
	}
}

func TestImmediateAbort(t *testing.T) {
	handle, models := newHarness(nil)
	handle.SetResponses(Step(TextMessage("never streamed", nil)))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stream := models.Stream(ctx, handle.GetModel(""), userContext("x"), nil)
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonAborted {
		t.Errorf("stopReason = %v", result.StopReason)
	}
}

func TestStreamSimpleDelegates(t *testing.T) {
	handle, models := newHarness(nil)
	handle.SetResponses(Step(TextMessage("simple", nil)))
	result, err := models.CompleteSimple(context.Background(), handle.GetModel(""), userContext("x"),
		&ai.SimpleStreamOptions{Reasoning: ai.ThinkingHigh})
	if err != nil {
		t.Fatalf("CompleteSimple: %v", err)
	}
	if result.Content[0].(ai.TextContent).Text != "simple" {
		t.Errorf("content = %#v", result.Content)
	}
}

func TestPacedStreamingRespectsTokensPerSecond(t *testing.T) {
	handle, models := newHarness(&Options{TokensPerSecond: 1000})
	handle.SetResponses(Step(TextMessage(strings.Repeat("x", 400), nil)))
	start := time.Now()
	complete(t, models, handle.GetModel(""), userContext("x"), nil)
	// 100 tokens at 1000 tokens/s ≈ 100ms minimum.
	if elapsed := time.Since(start); elapsed < 50*time.Millisecond {
		t.Errorf("stream finished too fast for pacing: %v", elapsed)
	}
}
