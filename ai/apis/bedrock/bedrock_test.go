package bedrock

// End-to-end tests driving Stream/StreamSimple through run(), with
// newConverseStreamClient overridden to a fake client (mirroring
// ai/apis/google/vertex's adcTokenFunc stubbing pattern), matching upstream's
// mocked-BedrockRuntimeClient test setup: OnPayload captures the golden
// request shape before the (fake) client call, and the fake client's
// captured *bedrockruntime.ConverseStreamInput is available for assertions
// that need the AWS-typed shape (e.g. tool schemas, cache points).

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"github.com/julienlegoux/kern-link/ai"
)

// fakeReader implements bedrockruntime.ConverseStreamOutputReader.
type fakeReader struct {
	events chan types.ConverseStreamOutput
	err    error
}

func (f *fakeReader) Events() <-chan types.ConverseStreamOutput { return f.events }
func (f *fakeReader) Close() error                              { return nil }
func (f *fakeReader) Err() error                                { return f.err }

// fakeClient implements converseStreamAPI, capturing the input it was
// called with.
type fakeClient struct {
	captured *bedrockruntime.ConverseStreamInput
	stream   *bedrockruntime.ConverseStreamEventStream
	err      error
}

func (f *fakeClient) ConverseStream(_ context.Context, params *bedrockruntime.ConverseStreamInput, _ ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseStreamEventStream, error) {
	f.captured = params
	if f.err != nil {
		return nil, f.err
	}
	return f.stream, nil
}

// alwaysErrorsClient mirrors the TS mock's `send(): Promise<never>` --
// used by tests that only care about the payload captured via OnPayload
// before the (fake) send.
func alwaysErrorsClient(err error) func(ctx context.Context, model *ai.Model, opts *ai.StreamOptions) (converseStreamAPI, error) {
	return func(ctx context.Context, model *ai.Model, opts *ai.StreamOptions) (converseStreamAPI, error) {
		return &fakeClient{err: err}, nil
	}
}

func withFakeClient(t *testing.T, factory func(ctx context.Context, model *ai.Model, opts *ai.StreamOptions) (converseStreamAPI, error)) {
	t.Helper()
	prev := newConverseStreamClient
	newConverseStreamClient = factory
	t.Cleanup(func() { newConverseStreamClient = prev })
}

func capturePayload(t *testing.T, model *ai.Model, chat ai.Context, opts *ai.StreamOptions) (*wireRequest, *ai.AssistantMessage) {
	t.Helper()
	withFakeClient(t, alwaysErrorsClient(errors.New("mock send")))

	var captured *wireRequest
	base := ai.StreamOptions{}
	if opts != nil {
		base = *opts
	}
	base.OnPayload = func(_ context.Context, payload any, _ *ai.Model) (any, error) {
		captured = payload.(*wireRequest)
		return payload, nil
	}

	stream := Stream(context.Background(), model, chat, &base)
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("stream.Result: %v", err)
	}
	if captured == nil {
		t.Fatal("expected payload to be captured before the fake client call failed")
	}
	return captured, result
}

func TestStream_CapturesPayloadAndSurfacesClientError(t *testing.T) {
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("Hello"), Timestamp: time.Now().UnixMilli()}}}
	payload, result := capturePayload(t, testModel(), chat, nil)

	if payload.ModelID != testModel().ID {
		t.Errorf("modelId = %q", payload.ModelID)
	}
	if len(payload.Messages) != 1 || payload.Messages[0].Content[0].Text != "Hello" {
		t.Errorf("messages = %+v", payload.Messages)
	}
	if result.StopReason != ai.StopReasonError {
		t.Errorf("stopReason = %q, want error", result.StopReason)
	}
	if result.ErrorMessage != "mock send" {
		t.Errorf("errorMessage = %q, want %q", result.ErrorMessage, "mock send")
	}
}

func TestStream_ThrottlingExceptionGetsHumanReadablePrefix(t *testing.T) {
	withFakeClient(t, alwaysErrorsClient(&types.ThrottlingException{Message: strPtr("too many requests")}))
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("Hello"), Timestamp: time.Now().UnixMilli()}}}
	stream := Stream(context.Background(), testModel(), chat, &ai.StreamOptions{})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("stream.Result: %v", err)
	}
	if result.ErrorMessage != "Throttling error: too many requests" {
		t.Errorf("errorMessage = %q", result.ErrorMessage)
	}
}

func TestStream_DecodesFakeEventStreamToDone(t *testing.T) {
	events := make(chan types.ConverseStreamOutput, 4)
	events <- &types.ConverseStreamOutputMemberMessageStart{Value: types.MessageStartEvent{Role: types.ConversationRoleAssistant}}
	events <- &types.ConverseStreamOutputMemberContentBlockDelta{Value: types.ContentBlockDeltaEvent{
		ContentBlockIndex: i32(0),
		Delta:             &types.ContentBlockDeltaMemberText{Value: "Hi there"},
	}}
	events <- &types.ConverseStreamOutputMemberContentBlockStop{Value: types.ContentBlockStopEvent{ContentBlockIndex: i32(0)}}
	events <- &types.ConverseStreamOutputMemberMessageStop{Value: types.MessageStopEvent{StopReason: types.StopReasonEndTurn}}
	close(events)

	fakeStream := bedrockruntime.NewConverseStreamEventStream(func(s *bedrockruntime.ConverseStreamEventStream) {
		s.Reader = &fakeReader{events: events}
	})
	withFakeClient(t, func(ctx context.Context, model *ai.Model, opts *ai.StreamOptions) (converseStreamAPI, error) {
		return &fakeClient{stream: fakeStream}, nil
	})

	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("Hello"), Timestamp: time.Now().UnixMilli()}}}
	stream := Stream(context.Background(), testModel(), chat, &ai.StreamOptions{})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("stream.Result: %v", err)
	}
	if result.StopReason != ai.StopReasonStop {
		t.Errorf("stopReason = %q, want stop", result.StopReason)
	}
	if len(result.Content) != 1 {
		t.Fatalf("len(content) = %d, want 1", len(result.Content))
	}
	text, ok := result.Content[0].(ai.TextContent)
	if !ok || text.Text != "Hi there" {
		t.Errorf("content[0] = %+v", result.Content[0])
	}
}

func TestStreamSimple_SetsAdaptiveThinkingReasoning(t *testing.T) {
	withFakeClient(t, alwaysErrorsClient(errors.New("mock send")))

	var captured *wireRequest
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("Hello"), Timestamp: time.Now().UnixMilli()}}}
	model := opus48()
	model.MaxTokens = 8192
	model.ContextWindow = 200000
	model.Input = []ai.Modality{ai.ModalityText}

	opts := &ai.SimpleStreamOptions{
		Reasoning: ai.ThinkingHigh,
		StreamOptions: ai.StreamOptions{
			OnPayload: func(_ context.Context, payload any, _ *ai.Model) (any, error) {
				captured = payload.(*wireRequest)
				return payload, nil
			},
		},
	}
	stream := StreamSimple(context.Background(), model, chat, opts)
	_, _ = stream.Result(context.Background())

	if captured == nil {
		t.Fatal("expected payload to be captured")
	}
	thinking, _ := captured.AdditionalModelRequestFields["thinking"].(map[string]any)
	if thinking["type"] != "adaptive" {
		t.Errorf("thinking = %+v, want adaptive", thinking)
	}
}
