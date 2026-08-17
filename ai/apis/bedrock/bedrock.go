// Package bedrock implements the bedrock-converse-stream wire adapter: Converse
// request building (messages, tools, inference config, thinking payloads) and
// ConverseStream event decoding into the unified event protocol, over
// aws-sdk-go-v2's bedrockruntime client -- the one transport upstream (and
// this port) uses an SDK for deliberately, since SigV4 request signing and
// event-stream framing would be impractical to hand-roll faithfully.
//
// The full AWS auth matrix -- explicit keys, profiles, region resolution
// (including inference-profile ARN extraction and built-in endpoint
// derivation), and bearer-token auth -- is resolved by clientauth.go's
// resolveClientConfig and applied by client.go's newBedrockRuntimeClient.
//
// Ports: packages/ai/src/api/bedrock-converse-stream.ts
package bedrock

import (
	"context"
	"encoding/json"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"

	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/apis"
)

// converseStreamAPI abstracts the one bedrockruntime.Client call this
// adapter uses. It returns *bedrockruntime.ConverseStreamEventStream (rather
// than the concrete *bedrockruntime.ConverseStreamOutput) because
// ConverseStreamOutput's event-stream field is unexported and populated only
// by the SDK's own deserialize middleware -- a hand-written fake could never
// produce a valid one. ConverseStreamEventStream's Reader field, by
// contrast, is exported and settable via bedrockruntime.
// NewConverseStreamEventStream, which is exactly the SDK's own documented
// testing/mocking hook. This lets tests substitute a fake client without a
// live AWS credential chain or network.
type converseStreamAPI interface {
	ConverseStream(ctx context.Context, params *bedrockruntime.ConverseStreamInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseStreamEventStream, error)
}

type sdkConverseStreamClient struct {
	client *bedrockruntime.Client
}

func (c sdkConverseStreamClient) ConverseStream(ctx context.Context, params *bedrockruntime.ConverseStreamInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseStreamEventStream, error) {
	out, err := c.client.ConverseStream(ctx, params, optFns...)
	if err != nil {
		return nil, err
	}
	return out.GetStream(), nil
}

// newConverseStreamClient constructs the client used for ConverseStream
// calls: resolveClientConfig computes the full auth matrix (region,
// profile, explicit credentials, endpoint, bearer token, custom headers) for
// this model/options pair, and newBedrockRuntimeClient applies it over the
// SDK's default credential chain (env vars, shared config/credentials
// files, IMDS, ECS container credentials, SSO, ...) for whatever it leaves
// unset. Overridable in tests (mirrors ai/apis/google/vertex's adcTokenFunc
// stubbing pattern) so tests can inject a fake without a live AWS config or
// network.
var newConverseStreamClient = func(ctx context.Context, model *ai.Model, opts *ai.StreamOptions) (converseStreamAPI, error) {
	client, err := newBedrockRuntimeClient(ctx, resolveClientConfig(model, opts))
	if err != nil {
		return nil, err
	}
	return sdkConverseStreamClient{client}, nil
}

// Stream implements ai.StreamFunc for the bedrock-converse-stream wire
// protocol.
func Stream(ctx context.Context, model *ai.Model, chat ai.Context, opts *ai.StreamOptions) *ai.Stream {
	out := ai.NewStream()
	go run(ctx, out, model, chat, opts)
	return out
}

// StreamSimple implements ai.SimpleStreamFunc, mapping the abstract
// Reasoning level to Bedrock's adaptive-thinking effort or a budget-based
// thinking config for Anthropic Claude models (other Bedrock models pass the
// reasoning level straight through, same as upstream). Ports the
// streamSimple half of bedrock-converse-stream.ts.
func StreamSimple(ctx context.Context, model *ai.Model, chat ai.Context, opts *ai.SimpleStreamOptions) *ai.Stream {
	base := apis.BuildBaseOptions(model, chat, opts, "")

	if opts == nil || opts.Reasoning == "" {
		base.BedrockReasoning = ""
		return Stream(ctx, model, chat, &base)
	}

	if !isAnthropicClaudeModel(model) {
		base.BedrockReasoning = opts.Reasoning
		base.BedrockThinkingBudgets = opts.ThinkingBudgets
		return Stream(ctx, model, chat, &base)
	}

	if supportsAdaptiveThinking(model.ID, model.Name) {
		base.BedrockReasoning = opts.Reasoning
		base.BedrockThinkingBudgets = opts.ThinkingBudgets
		return Stream(ctx, model, chat, &base)
	}

	// Undefined means the caller did not request an output cap; let the
	// helper use the model cap. Do not coerce to 0, or the thinking budget
	// would become the entire max_tokens value.
	maxTokens, thinkingBudget := apis.AdjustMaxTokensForThinking(base.MaxTokens, model.MaxTokens, opts.Reasoning, opts.ThinkingBudgets)
	clamped := apis.ClampMaxTokensToContext(model, chat, maxTokens)
	base.MaxTokens = &clamped
	base.BedrockReasoning = opts.Reasoning

	level := apis.ClampReasoning(opts.Reasoning)
	budget := minInt(thinkingBudget, maxInt(0, clamped-1024))
	merged := ai.ThinkingBudgets{}
	if opts.ThinkingBudgets != nil {
		merged = *opts.ThinkingBudgets
	}
	switch level {
	case ai.ThinkingMinimal:
		merged.Minimal = &budget
	case ai.ThinkingLow:
		merged.Low = &budget
	case ai.ThinkingMedium:
		merged.Medium = &budget
	default:
		merged.High = &budget
	}
	base.BedrockThinkingBudgets = &merged

	return Stream(ctx, model, chat, &base)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func run(ctx context.Context, out *ai.Stream, model *ai.Model, chat ai.Context, opts *ai.StreamOptions) {
	output := &ai.AssistantMessage{
		Content:    []ai.AssistantContentPart{},
		Api:        model.Api,
		Provider:   model.Provider,
		Model:      model.ID,
		StopReason: ai.StopReasonStop,
		Timestamp:  time.Now().UnixMilli(),
	}

	fail := func(err error) {
		reason := ai.StopReasonError
		if ctx.Err() != nil {
			reason = ai.StopReasonAborted
		}
		output.StopReason = reason
		output.ErrorMessage = err.Error()
		out.Push(ai.ErrorEvent{Reason: reason, Error: output})
	}

	req := buildParams(model, chat, opts)

	var payload any = req
	if opts != nil && opts.OnPayload != nil {
		next, err := opts.OnPayload(ctx, payload, model)
		if err != nil {
			fail(err)
			return
		}
		if next != nil {
			payload = next
		}
	}
	if replaced, ok := payload.(*wireRequest); ok {
		req = replaced
	} else {
		// OnPayload may replace the payload with any JSON-shaped value (not
		// necessarily a *wireRequest); round-trip through JSON to recover a
		// typed request, matching how the raw-HTTP sibling adapters accept
		// an arbitrary OnPayload replacement by marshaling whatever value
		// they got directly.
		raw, err := json.Marshal(payload)
		if err != nil {
			fail(err)
			return
		}
		var replacedReq wireRequest
		if err := json.Unmarshal(raw, &replacedReq); err != nil {
			fail(err)
			return
		}
		req = &replacedReq
	}

	input := toConverseStreamInput(req)

	client, err := newConverseStreamClient(ctx, model, opts)
	if err != nil {
		fail(formatErr(err))
		return
	}

	stream, err := client.ConverseStream(ctx, input)
	if err != nil {
		fail(formatErr(err))
		return
	}

	// Upstream pushes "start" only once the first messageStart event
	// arrives (rather than eagerly after a successful response, as the
	// sibling raw-HTTP adapters do), since messageStart is itself part of
	// the ConverseStream event union.
	DecodeStream(out, output, model, stream.Events())

	if streamErr := stream.Err(); streamErr != nil {
		fail(formatErr(streamErr))
		return
	}

	if ctx.Err() != nil {
		fail(errAborted)
		return
	}
	if output.StopReason == ai.StopReasonAborted || output.StopReason == ai.StopReasonError {
		fail(errUnknown)
		return
	}

	out.Push(ai.DoneEvent{Reason: output.StopReason, Message: output})
}

// buildParams builds the wireRequest for one Stream call. Ports the
// commandInput literal from bedrock-converse-stream.ts's stream function.
func buildParams(model *ai.Model, chat ai.Context, opts *ai.StreamOptions) *wireRequest {
	cacheRetention := ai.CacheRetentionShort
	var (
		maxTokens           *int
		temperature         *float64
		toolChoice          string
		toolChoiceFunction  string
		reasoning           ai.ThinkingLevel
		thinkingBudgets     *ai.ThinkingBudgets
		interleavedThinking *bool
		thinkingDisplay     string
		region              string
		requestMetadata     map[string]string
		env                 ai.ProviderEnv
	)
	if opts != nil {
		cacheRetention = opts.EffectiveCacheRetention()
		maxTokens = opts.MaxTokens
		temperature = opts.Temperature
		toolChoice = opts.BedrockToolChoice
		toolChoiceFunction = opts.BedrockToolChoiceFunction
		reasoning = opts.BedrockReasoning
		thinkingBudgets = opts.BedrockThinkingBudgets
		interleavedThinking = opts.BedrockInterleavedThinking
		thinkingDisplay = opts.BedrockThinkingDisplay
		region = opts.BedrockRegion
		requestMetadata = opts.BedrockRequestMetadata
		env = opts.Env
	}

	req := &wireRequest{
		ModelID:  model.ID,
		Messages: convertMessages(chat, model, cacheRetention, env),
		System:   buildSystemPrompt(chat.SystemPrompt, model, cacheRetention, env),
		InferenceConfig: wireInference{
			MaxTokens:   maxTokens,
			Temperature: temperature,
		},
		ToolConfig:                   convertToolConfig(chat.Tools, toolChoice, toolChoiceFunction),
		AdditionalModelRequestFields: buildAdditionalModelRequestFields(model, reasoning, thinkingBudgets, interleavedThinking, thinkingDisplay, region, env),
		RequestMetadata:              requestMetadata,
	}
	return req
}
