// Package toolcallexample is the small end-to-end example this issue calls
// for: it streams one user turn through any ai.Models, follows up with a
// tool result if the model calls the get_weather tool, and returns the
// final assistant message. The same Run function backs both the
// deterministic faux-provider test and the env-gated real-provider smoke
// test (live_smoke_test.go), and cmd/pi-ai/example's runnable main.
package toolcallexample

// Ports: none — original: end-to-end tool-call round-trip harness backing the
// faux/live tests and cmd/pi-ai/example.

import (
	"context"
	"fmt"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
)

// WeatherTool is the single tool offered in the round trip.
var WeatherTool = ai.Tool{
	Name:        "get_weather",
	Description: "Get the current weather for a location.",
	Parameters:  ai.JSONSchema(`{"type":"object","properties":{"location":{"type":"string","description":"City name"}},"required":["location"]}`),
}

// ResolveTool answers one tool call. Swap DefaultResolveTool for a real
// implementation in a production agent.
type ResolveTool func(call ai.ToolCall) ai.ToolResultMessage

// DefaultResolveTool answers a get_weather call with a canned forecast; it
// never calls out to a real weather service.
func DefaultResolveTool(call ai.ToolCall) ai.ToolResultMessage {
	location, _ := call.Arguments["location"].(string)
	if location == "" {
		location = "that location"
	}
	return ai.ToolResultMessage{
		ToolCallID: call.ID,
		ToolName:   call.Name,
		Content:    []ai.UserContentPart{ai.TextContent{Text: fmt.Sprintf("Sunny and 22C in %s.", location)}},
		Timestamp:  time.Now().UnixMilli(),
	}
}

// Run streams prompt through model (offering WeatherTool), and — only if
// the model responds with a tool call — resolves it (via resolve, or
// DefaultResolveTool when nil) and streams a second turn with the tool
// result appended, returning that final message. A model that answers
// directly without calling a tool has its first message returned unchanged.
func Run(
	ctx context.Context,
	models ai.Models,
	model *ai.Model,
	prompt string,
	resolve ResolveTool,
	apiKey string,
) (*ai.AssistantMessage, error) {
	if resolve == nil {
		resolve = DefaultResolveTool
	}

	chat := ai.Context{
		Messages: []ai.Message{ai.UserMessage{Content: ai.UserText(prompt), Timestamp: time.Now().UnixMilli()}},
		Tools:    []ai.Tool{WeatherTool},
	}

	first, err := models.Complete(ctx, model, chat, &ai.StreamOptions{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("toolcallexample: first turn: %w", err)
	}
	if first.StopReason == ai.StopReasonError || first.StopReason == ai.StopReasonAborted {
		return nil, fmt.Errorf("toolcallexample: first turn failed (%s): %s", first.StopReason, first.ErrorMessage)
	}

	call, ok := firstToolCall(first)
	if !ok {
		return first, nil
	}

	result := resolve(call)
	chat.Messages = append(chat.Messages, first, result)

	final, err := models.Complete(ctx, model, chat, &ai.StreamOptions{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("toolcallexample: follow-up turn: %w", err)
	}
	return final, nil
}

// firstToolCall returns the first ToolCall block in message, if any.
func firstToolCall(message *ai.AssistantMessage) (ai.ToolCall, bool) {
	for _, block := range message.Content {
		if call, ok := block.(ai.ToolCall); ok {
			return call, true
		}
	}
	return ai.ToolCall{}, false
}
