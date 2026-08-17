package toolcallexample

import (
	"context"
	"testing"

	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/providers/faux"
)

func TestRun_FauxToolCallRoundTrip(t *testing.T) {
	handle := faux.New(nil)
	handle.SetResponses(
		faux.Step(faux.AssistantMessage(
			[]ai.AssistantContentPart{faux.ToolCall("get_weather", map[string]any{"location": "Paris"}, nil)},
			&faux.AssistantMessageOptions{StopReason: ai.StopReasonToolUse},
		)),
		faux.Step(faux.TextMessage("It's sunny and 22C in Paris.", nil)),
	)

	models := ai.CreateModels(nil)
	models.SetProvider(handle.Provider)
	model := handle.GetModel("")

	var resolvedCall ai.ToolCall
	resolve := func(call ai.ToolCall) *ai.ToolResultMessage {
		resolvedCall = call
		return DefaultResolveTool(call)
	}

	final, err := Run(context.Background(), models, model, "What's the weather in Paris?", resolve, "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resolvedCall.Name != "get_weather" {
		t.Errorf("expected the tool call to reach resolve, got %+v", resolvedCall)
	}
	if final.StopReason != ai.StopReasonStop {
		t.Fatalf("final.StopReason = %q, want stop (errorMessage=%q)", final.StopReason, final.ErrorMessage)
	}
	if len(final.Content) == 0 {
		t.Fatal("final message has no content")
	}
	if got := handle.State().CallCount(); got != 2 {
		t.Errorf("expected exactly 2 stream calls (tool-call turn + follow-up), got %d", got)
	}
}

func TestRun_DirectAnswerSkipsToolResult(t *testing.T) {
	handle := faux.New(nil)
	handle.SetResponses(faux.Step(faux.TextMessage("No tools needed.", nil)))

	models := ai.CreateModels(nil)
	models.SetProvider(handle.Provider)
	model := handle.GetModel("")

	called := false
	resolve := func(call ai.ToolCall) *ai.ToolResultMessage {
		called = true
		return DefaultResolveTool(call)
	}

	final, err := Run(context.Background(), models, model, "Just say hi.", resolve, "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if called {
		t.Error("resolve should not be called when the model answers directly")
	}
	if got := handle.State().CallCount(); got != 1 {
		t.Errorf("expected exactly 1 stream call, got %d", got)
	}
	if final.Content[0].(ai.TextContent).Text != "No tools needed." {
		t.Errorf("unexpected final content: %+v", final.Content)
	}
}

func TestRun_FirstTurnErrorPropagates(t *testing.T) {
	handle := faux.New(nil)
	// No responses queued: faux returns a StopReasonError message.
	models := ai.CreateModels(nil)
	models.SetProvider(handle.Provider)
	model := handle.GetModel("")

	_, err := Run(context.Background(), models, model, "hello", nil, "")
	if err == nil {
		t.Fatal("expected an error when the first turn fails")
	}
}

func TestDefaultResolveTool_EchoesLocation(t *testing.T) {
	call := ai.ToolCall{ID: "tc_1", Name: "get_weather", Arguments: map[string]any{"location": "Tokyo"}}
	result := DefaultResolveTool(call)
	if result.ToolCallID != "tc_1" || result.ToolName != "get_weather" {
		t.Errorf("unexpected result metadata: %+v", result)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty tool result content")
	}
	text, ok := result.Content[0].(ai.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	if text.Text == "" {
		t.Error("expected non-empty tool result text")
	}
}
