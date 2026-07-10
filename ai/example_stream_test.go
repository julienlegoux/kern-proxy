package ai_test

// Runnable versions of the streaming, tool-calling, and cost-tracking flows
// from the README and docs/usage.md. They run against the in-process faux
// provider, so `go test ./...` compiles and asserts them offline — the
// documented flows cannot drift away from the API.

import (
	"context"
	"fmt"
	"strings"

	"github.com/julienlegoux/kern-link/ai"
	"github.com/julienlegoux/kern-link/ai/providers/faux"
)

// ExampleStream_Events consumes the typed event protocol: text arrives as
// deltas, and the stream ends with exactly one terminal done/error event.
// Deltas are accumulated and printed after the loop because chunk boundaries
// are provider-dependent; the assembled text is not.
func ExampleStream_Events() {
	handle := faux.New(nil)
	handle.SetResponses(faux.Step(faux.TextMessage("Hello from the event stream!", nil)))

	models := ai.CreateModels(nil)
	models.SetProvider(handle.Provider)

	ctx := context.Background()
	chat := ai.Context{Messages: []ai.Message{
		&ai.UserMessage{Content: ai.UserText("Say hello."), Timestamp: 1},
	}}

	// This loop drains the stream to completion, so context.Background() is
	// safe; a consumer that may break out early must cancel ctx instead.
	stream := models.Stream(ctx, handle.GetModel(""), chat, nil)

	var text strings.Builder
	for ev := range stream.Events(ctx) {
		switch e := ev.(type) {
		case ai.TextDeltaEvent:
			text.WriteString(e.Delta)
		case ai.DoneEvent:
			fmt.Println("stop reason:", e.Reason)
		}
	}
	fmt.Println(text.String())
	// Output:
	// stop reason: stop
	// Hello from the event stream!
}

// Example_toolCalling shows the tool-call round trip: declare a tool on the
// Context, receive an ai.ToolCall from the assistant, run the tool, append a
// ToolResultMessage, and complete the turn.
func Example_toolCalling() {
	handle := faux.New(nil)
	models := ai.CreateModels(nil)
	models.SetProvider(handle.Provider)

	chat := ai.Context{
		Messages: []ai.Message{
			&ai.UserMessage{Content: ai.UserText("What's the weather in Paris?"), Timestamp: 1},
		},
		Tools: []ai.Tool{{
			Name:        "get_weather",
			Description: "Current weather for a city.",
			Parameters:  ai.JSONSchema(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`),
		}},
	}

	// Script the two assistant turns: first a tool call, then the answer.
	handle.SetResponses(
		faux.Step(faux.AssistantMessage(
			[]ai.AssistantContentPart{
				faux.ToolCall("get_weather", map[string]any{"city": "Paris"}, &faux.ToolCallOptions{ID: "call_1"}),
			},
			&faux.AssistantMessageOptions{StopReason: ai.StopReasonToolUse},
		)),
		faux.Step(faux.TextMessage("It is 22°C and sunny in Paris.", nil)),
	)

	ctx := context.Background()
	msg, err := models.Complete(ctx, handle.GetModel(""), chat, nil)
	if err != nil {
		panic(err)
	}

	call := msg.Content[0].(ai.ToolCall)
	fmt.Printf("tool call: %s(city=%v)\n", call.Name, call.Arguments["city"])

	// The caller executes the tool and feeds the result back as a message.
	chat.Messages = append(chat.Messages, msg, &ai.ToolResultMessage{
		ToolCallID: call.ID,
		ToolName:   call.Name,
		Content:    []ai.UserContentPart{ai.TextContent{Text: "22°C, sunny"}},
		Timestamp:  2,
	})

	final, err := models.Complete(ctx, handle.GetModel(""), chat, nil)
	if err != nil {
		panic(err)
	}
	fmt.Println(final.Content[0].(ai.TextContent).Text)
	// Output:
	// tool call: get_weather(city=Paris)
	// It is 22°C and sunny in Paris.
}

// ExampleCalculateCost prices a Usage block against a model's per-million-token
// price sheet. Models from the embedded catalog (ai/catalog) or from
// providers.Models carry their price sheet already filled in.
func ExampleCalculateCost() {
	model := &ai.Model{
		ID: "example-model",
		// Dollars per million tokens.
		Cost: ai.ModelCost{Input: 3, Output: 15, CacheRead: 0.3, CacheWrite: 3.75},
	}
	usage := ai.Usage{Input: 12000, Output: 800, CacheRead: 40000, CacheWrite: 2000}

	cost := ai.CalculateCost(model, &usage)
	fmt.Printf("input $%.4f + output $%.4f + cache $%.4f = $%.4f\n",
		cost.Input, cost.Output, cost.CacheRead+cost.CacheWrite, cost.Total)
	// Output:
	// input $0.0360 + output $0.0120 + cache $0.0195 = $0.0675
}
