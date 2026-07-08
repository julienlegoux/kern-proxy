package bedrock

// Ports the streaming half of bedrock-converse-stream.ts: handleContentBlockStart,
// handleContentBlockDelta, handleContentBlockStop, handleMetadata, and
// mapStopReason, decoding the AWS SDK's ConverseStream event union into the
// unified event protocol.

import (
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/internal/partialjson"
)

// toolCallScratch tracks the in-flight partial-JSON argument buffer for one
// streaming tool call, keyed by the AWS contentBlockIndex.
type toolCallScratch struct {
	args strings.Builder
}

// DecodeStream reads a ConverseStream event channel and pushes ai.Events
// reflecting its content, mutating output in place. Ports the per-event
// switch inside bedrock-converse-stream.ts's stream function body.
func DecodeStream(out *ai.Stream, output *ai.AssistantMessage, model *ai.Model, events <-chan types.ConverseStreamOutput) {
	// indexOf maps the AWS SDK's contentBlockIndex to output.Content's index.
	indexOf := map[int32]int{}
	toolScratch := map[int32]*toolCallScratch{}

	for ev := range events {
		switch e := ev.(type) {
		case *types.ConverseStreamOutputMemberMessageStart:
			out.Push(ai.StartEvent{Partial: output.Clone()})

		case *types.ConverseStreamOutputMemberContentBlockStart:
			handleContentBlockStart(e.Value, indexOf, output, out)

		case *types.ConverseStreamOutputMemberContentBlockDelta:
			handleContentBlockDelta(e.Value, indexOf, toolScratch, output, out)

		case *types.ConverseStreamOutputMemberContentBlockStop:
			handleContentBlockStop(e.Value, indexOf, toolScratch, output, out)

		case *types.ConverseStreamOutputMemberMessageStop:
			output.StopReason = mapStopReason(e.Value.StopReason)

		case *types.ConverseStreamOutputMemberMetadata:
			handleMetadata(e.Value, model, output)
		}
	}
}

func handleContentBlockStart(event types.ContentBlockStartEvent, indexOf map[int32]int, output *ai.AssistantMessage, out *ai.Stream) {
	if event.ContentBlockIndex == nil {
		return
	}
	awsIndex := *event.ContentBlockIndex

	toolUse, ok := event.Start.(*types.ContentBlockStartMemberToolUse)
	if !ok {
		return
	}
	id := ""
	if toolUse.Value.ToolUseId != nil {
		id = *toolUse.Value.ToolUseId
	}
	name := ""
	if toolUse.Value.Name != nil {
		name = *toolUse.Value.Name
	}
	output.Content = append(output.Content, ai.ToolCall{ID: id, Name: name, Arguments: map[string]any{}})
	indexOf[awsIndex] = len(output.Content) - 1
	out.Push(ai.ToolCallStartEvent{ContentIndex: indexOf[awsIndex], Partial: output.Clone()})
}

func handleContentBlockDelta(
	event types.ContentBlockDeltaEvent,
	indexOf map[int32]int,
	toolScratch map[int32]*toolCallScratch,
	output *ai.AssistantMessage,
	out *ai.Stream,
) {
	if event.ContentBlockIndex == nil {
		return
	}
	awsIndex := *event.ContentBlockIndex

	switch delta := event.Delta.(type) {
	case *types.ContentBlockDeltaMemberText:
		index, ok := indexOf[awsIndex]
		if !ok {
			output.Content = append(output.Content, ai.TextContent{})
			index = len(output.Content) - 1
			indexOf[awsIndex] = index
			out.Push(ai.TextStartEvent{ContentIndex: index, Partial: output.Clone()})
		}
		cur := output.Content[index].(ai.TextContent)
		cur.Text += delta.Value
		output.Content[index] = cur
		out.Push(ai.TextDeltaEvent{ContentIndex: index, Delta: delta.Value, Partial: output.Clone()})

	case *types.ContentBlockDeltaMemberToolUse:
		index, ok := indexOf[awsIndex]
		if !ok {
			output.Content = append(output.Content, ai.ToolCall{Arguments: map[string]any{}})
			index = len(output.Content) - 1
			indexOf[awsIndex] = index
			out.Push(ai.ToolCallStartEvent{ContentIndex: index, Partial: output.Clone()})
		}
		scratch, ok := toolScratch[awsIndex]
		if !ok {
			scratch = &toolCallScratch{}
			toolScratch[awsIndex] = scratch
		}
		argsDelta := ""
		if delta.Value.Input != nil {
			argsDelta = *delta.Value.Input
			scratch.args.WriteString(argsDelta)
			cur := output.Content[index].(ai.ToolCall)
			cur.Arguments = partialjson.ParseStreamingObject(scratch.args.String())
			output.Content[index] = cur
		}
		out.Push(ai.ToolCallDeltaEvent{ContentIndex: index, Delta: argsDelta, Partial: output.Clone()})

	case *types.ContentBlockDeltaMemberReasoningContent:
		index, ok := indexOf[awsIndex]
		if !ok {
			output.Content = append(output.Content, ai.ThinkingContent{})
			index = len(output.Content) - 1
			indexOf[awsIndex] = index
			out.Push(ai.ThinkingStartEvent{ContentIndex: index, Partial: output.Clone()})
		}
		switch reasoning := delta.Value.(type) {
		case *types.ReasoningContentBlockDeltaMemberText:
			cur := output.Content[index].(ai.ThinkingContent)
			cur.Thinking += reasoning.Value
			output.Content[index] = cur
			out.Push(ai.ThinkingDeltaEvent{ContentIndex: index, Delta: reasoning.Value, Partial: output.Clone()})
		case *types.ReasoningContentBlockDeltaMemberSignature:
			cur := output.Content[index].(ai.ThinkingContent)
			cur.ThinkingSignature += reasoning.Value
			output.Content[index] = cur
		}
	}
}

func handleContentBlockStop(
	event types.ContentBlockStopEvent,
	indexOf map[int32]int,
	toolScratch map[int32]*toolCallScratch,
	output *ai.AssistantMessage,
	out *ai.Stream,
) {
	if event.ContentBlockIndex == nil {
		return
	}
	awsIndex := *event.ContentBlockIndex
	index, ok := indexOf[awsIndex]
	if !ok {
		return
	}

	switch b := output.Content[index].(type) {
	case ai.TextContent:
		out.Push(ai.TextEndEvent{ContentIndex: index, Content: b.Text, Partial: output.Clone()})
	case ai.ThinkingContent:
		out.Push(ai.ThinkingEndEvent{ContentIndex: index, Content: b.Thinking, Partial: output.Clone()})
	case ai.ToolCall:
		if scratch, ok := toolScratch[awsIndex]; ok {
			b.Arguments = partialjson.ParseStreamingObject(scratch.args.String())
			output.Content[index] = b
			delete(toolScratch, awsIndex)
		}
		out.Push(ai.ToolCallEndEvent{ContentIndex: index, ToolCall: b, Partial: output.Clone()})
	}
}

func handleMetadata(event types.ConverseStreamMetadataEvent, model *ai.Model, output *ai.AssistantMessage) {
	if event.Usage == nil {
		return
	}
	u := event.Usage
	if u.InputTokens != nil {
		output.Usage.Input = int(*u.InputTokens)
	}
	if u.OutputTokens != nil {
		output.Usage.Output = int(*u.OutputTokens)
	}
	if u.CacheReadInputTokens != nil {
		output.Usage.CacheRead = int(*u.CacheReadInputTokens)
	}
	if u.CacheWriteInputTokens != nil {
		output.Usage.CacheWrite = int(*u.CacheWriteInputTokens)
	}
	if u.TotalTokens != nil {
		output.Usage.TotalTokens = int(*u.TotalTokens)
	} else {
		output.Usage.TotalTokens = output.Usage.Input + output.Usage.Output
	}
	ai.CalculateCost(model, &output.Usage)
}

// mapStopReason maps Bedrock's StopReason to the unified StopReason. Ports
// mapStopReason.
func mapStopReason(reason types.StopReason) ai.StopReason {
	switch reason {
	case types.StopReasonEndTurn, types.StopReasonStopSequence:
		return ai.StopReasonStop
	case types.StopReasonMaxTokens, types.StopReasonModelContextWindowExceeded:
		return ai.StopReasonLength
	case types.StopReasonToolUse:
		return ai.StopReasonToolUse
	default:
		return ai.StopReasonError
	}
}
