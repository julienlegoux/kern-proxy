package bedrock

// DecodeStream has no upstream unit-test coverage (bedrock-converse-stream.ts's
// decode path is only exercised by an env-gated live E2E test in
// bedrock-thinking-payload.test.ts). These tests are new, written to satisfy
// this issue's "thinking payloads round-trip" and "ConverseStream event
// decode to unified events" acceptance criteria, driving DecodeStream with
// synthetic AWS SDK stream events.

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"github.com/julienlegoux/kern-proxy/ai"
)

func i32(v int32) *int32 { return &v }

func newOutput(model *ai.Model) *ai.AssistantMessage {
	return &ai.AssistantMessage{
		Content:    []ai.AssistantContentPart{},
		Api:        model.Api,
		Provider:   model.Provider,
		Model:      model.ID,
		StopReason: ai.StopReasonStop,
		Timestamp:  time.Now().UnixMilli(),
	}
}

func drainEvents(out *ai.Stream) []ai.Event {
	var events []ai.Event
	for ev := range out.Events() {
		events = append(events, ev)
	}
	return events
}

func TestDecodeStream_TextDelta(t *testing.T) {
	model := testModel()
	output := newOutput(model)
	out := ai.NewStream()

	events := make(chan types.ConverseStreamOutput, 8)
	events <- &types.ConverseStreamOutputMemberMessageStart{Value: types.MessageStartEvent{Role: types.ConversationRoleAssistant}}
	events <- &types.ConverseStreamOutputMemberContentBlockDelta{Value: types.ContentBlockDeltaEvent{
		ContentBlockIndex: i32(0),
		Delta:             &types.ContentBlockDeltaMemberText{Value: "Hello"},
	}}
	events <- &types.ConverseStreamOutputMemberContentBlockDelta{Value: types.ContentBlockDeltaEvent{
		ContentBlockIndex: i32(0),
		Delta:             &types.ContentBlockDeltaMemberText{Value: ", world"},
	}}
	events <- &types.ConverseStreamOutputMemberContentBlockStop{Value: types.ContentBlockStopEvent{ContentBlockIndex: i32(0)}}
	events <- &types.ConverseStreamOutputMemberMessageStop{Value: types.MessageStopEvent{StopReason: types.StopReasonEndTurn}}
	close(events)

	DecodeStream(out, output, model, events)
	out.End(output)

	if len(output.Content) != 1 {
		t.Fatalf("len(content) = %d, want 1", len(output.Content))
	}
	text, ok := output.Content[0].(ai.TextContent)
	if !ok || text.Text != "Hello, world" {
		t.Errorf("content[0] = %+v, want text %q", output.Content[0], "Hello, world")
	}
	if output.StopReason != ai.StopReasonStop {
		t.Errorf("stopReason = %q, want stop", output.StopReason)
	}

	var sawStart, sawEnd bool
	for _, ev := range drainEvents(out) {
		switch ev.(type) {
		case ai.StartEvent:
			sawStart = true
		case ai.TextEndEvent:
			sawEnd = true
		}
	}
	if !sawStart || !sawEnd {
		t.Errorf("sawStart=%v sawEnd=%v, want both true", sawStart, sawEnd)
	}
}

func TestDecodeStream_ToolCallDelta(t *testing.T) {
	model := testModel()
	output := newOutput(model)
	out := ai.NewStream()

	events := make(chan types.ConverseStreamOutput, 8)
	events <- &types.ConverseStreamOutputMemberContentBlockStart{Value: types.ContentBlockStartEvent{
		ContentBlockIndex: i32(0),
		Start: &types.ContentBlockStartMemberToolUse{Value: types.ToolUseBlockStart{
			ToolUseId: strPtr("call-1"),
			Name:      strPtr("get_weather"),
		}},
	}}
	events <- &types.ConverseStreamOutputMemberContentBlockDelta{Value: types.ContentBlockDeltaEvent{
		ContentBlockIndex: i32(0),
		Delta:             &types.ContentBlockDeltaMemberToolUse{Value: types.ToolUseBlockDelta{Input: strPtr(`{"city":`)}},
	}}
	events <- &types.ConverseStreamOutputMemberContentBlockDelta{Value: types.ContentBlockDeltaEvent{
		ContentBlockIndex: i32(0),
		Delta:             &types.ContentBlockDeltaMemberToolUse{Value: types.ToolUseBlockDelta{Input: strPtr(`"paris"}`)}},
	}}
	events <- &types.ConverseStreamOutputMemberContentBlockStop{Value: types.ContentBlockStopEvent{ContentBlockIndex: i32(0)}}
	close(events)

	DecodeStream(out, output, model, events)

	if len(output.Content) != 1 {
		t.Fatalf("len(content) = %d, want 1", len(output.Content))
	}
	tc, ok := output.Content[0].(ai.ToolCall)
	if !ok {
		t.Fatalf("content[0] = %+v, want ToolCall", output.Content[0])
	}
	if tc.ID != "call-1" || tc.Name != "get_weather" {
		t.Errorf("toolCall = %+v", tc)
	}
	if tc.Arguments["city"] != "paris" {
		t.Errorf("arguments = %+v, want city=paris", tc.Arguments)
	}
}

func TestDecodeStream_ReasoningContentRoundTrip(t *testing.T) {
	model := testModel()
	output := newOutput(model)
	out := ai.NewStream()

	events := make(chan types.ConverseStreamOutput, 8)
	events <- &types.ConverseStreamOutputMemberContentBlockDelta{Value: types.ContentBlockDeltaEvent{
		ContentBlockIndex: i32(0),
		Delta:             &types.ContentBlockDeltaMemberReasoningContent{Value: &types.ReasoningContentBlockDeltaMemberText{Value: "Let me think"}},
	}}
	events <- &types.ConverseStreamOutputMemberContentBlockDelta{Value: types.ContentBlockDeltaEvent{
		ContentBlockIndex: i32(0),
		Delta:             &types.ContentBlockDeltaMemberReasoningContent{Value: &types.ReasoningContentBlockDeltaMemberSignature{Value: "sig-abc"}},
	}}
	events <- &types.ConverseStreamOutputMemberContentBlockStop{Value: types.ContentBlockStopEvent{ContentBlockIndex: i32(0)}}
	close(events)

	DecodeStream(out, output, model, events)

	if len(output.Content) != 1 {
		t.Fatalf("len(content) = %d, want 1", len(output.Content))
	}
	thinking, ok := output.Content[0].(ai.ThinkingContent)
	if !ok {
		t.Fatalf("content[0] = %+v, want ThinkingContent", output.Content[0])
	}
	if thinking.Thinking != "Let me think" || thinking.ThinkingSignature != "sig-abc" {
		t.Errorf("thinking = %+v, want text %q sig %q", thinking, "Let me think", "sig-abc")
	}
}

func TestDecodeStream_MapsStopReasons(t *testing.T) {
	cases := []struct {
		aws  types.StopReason
		want ai.StopReason
	}{
		{types.StopReasonEndTurn, ai.StopReasonStop},
		{types.StopReasonStopSequence, ai.StopReasonStop},
		{types.StopReasonMaxTokens, ai.StopReasonLength},
		{types.StopReasonToolUse, ai.StopReasonToolUse},
	}
	for _, tc := range cases {
		model := testModel()
		output := newOutput(model)
		out := ai.NewStream()
		events := make(chan types.ConverseStreamOutput, 1)
		events <- &types.ConverseStreamOutputMemberMessageStop{Value: types.MessageStopEvent{StopReason: tc.aws}}
		close(events)
		DecodeStream(out, output, model, events)
		if output.StopReason != tc.want {
			t.Errorf("aws=%v: stopReason = %q, want %q", tc.aws, output.StopReason, tc.want)
		}
	}
}

func TestDecodeStream_MetadataUpdatesUsageAndCost(t *testing.T) {
	model := testModel()
	model.Cost = ai.ModelCost{Input: 3, Output: 15, CacheRead: 0.3, CacheWrite: 3.75}
	output := newOutput(model)
	out := ai.NewStream()

	events := make(chan types.ConverseStreamOutput, 1)
	events <- &types.ConverseStreamOutputMemberMetadata{Value: types.ConverseStreamMetadataEvent{
		Usage: &types.TokenUsage{
			InputTokens:           i32(100),
			OutputTokens:          i32(50),
			TotalTokens:           i32(150),
			CacheReadInputTokens:  i32(10),
			CacheWriteInputTokens: i32(5),
		},
	}}
	close(events)

	DecodeStream(out, output, model, events)

	if output.Usage.Input != 100 || output.Usage.Output != 50 || output.Usage.TotalTokens != 150 {
		t.Errorf("usage = %+v", output.Usage)
	}
	if output.Usage.CacheRead != 10 || output.Usage.CacheWrite != 5 {
		t.Errorf("cache usage = %+v", output.Usage)
	}
	if output.Usage.Cost.Total <= 0 {
		t.Errorf("cost = %+v, want > 0", output.Usage.Cost)
	}
}

func strPtr(s string) *string { return &s }
