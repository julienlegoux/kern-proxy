// Package mistral implements the mistral-conversations wire adapter: request
// building (messages, tools, tool choice, reasoning controls) and SSE stream
// decoding into the unified event protocol, over raw net/http and the
// hand-rolled SSE parser from ai/internal/sse. Upstream delegates transport
// to the @mistralai/mistralai SDK's `chat.stream` method; this Go port POSTs
// the same JSON shape directly to the chat completions endpoint
// (`{baseUrl}/v1/chat/completions`, Bearer auth, `stream: true`) since there
// is no Go SDK to delegate to (see PORTING.md's "Vendor SDKs" deviation).
//
// Ports: packages/ai/src/api/mistral-conversations.ts
package mistral

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/julienlegoux/kern-link/ai"
	"github.com/julienlegoux/kern-link/ai/apis"
	"github.com/julienlegoux/kern-link/ai/apis/internal/httpretry"
)

const chatCompletionsPath = "/v1/chat/completions"

// mistralToolCallIDLength is the fixed length Mistral requires for tool-call
// ids. Ports MISTRAL_TOOL_CALL_ID_LENGTH from mistral-conversations.ts.
const mistralToolCallIDLength = 9

// Stream implements ai.StreamFunc for the mistral-conversations wire protocol.
func Stream(ctx context.Context, model *ai.Model, chat ai.Context, opts *ai.StreamOptions) *ai.Stream {
	out := ai.NewStream()
	go run(ctx, out, model, chat, opts)
	return out
}

// StreamSimple implements ai.SimpleStreamFunc, translating the abstract
// Reasoning level into either Magistral's prompt_mode="reasoning" or the
// reasoning_effort-style models' binary "none"/"high" knob. Ports the
// streamSimple half of mistral-conversations.ts.
func StreamSimple(ctx context.Context, model *ai.Model, chat ai.Context, opts *ai.SimpleStreamOptions) *ai.Stream {
	apiKey := ""
	if opts != nil {
		apiKey = opts.APIKey
	}
	base := apis.BuildBaseOptions(model, chat, opts, apiKey)

	var reasoning ai.ThinkingLevel
	shouldUseReasoning := false
	if opts != nil && opts.Reasoning != "" {
		clamped := ai.ClampThinkingLevel(model, opts.Reasoning)
		if clamped != ai.ThinkingOff {
			reasoning = clamped
			shouldUseReasoning = model.Reasoning
		}
	}

	if shouldUseReasoning && usesPromptModeReasoning(model) {
		base.MistralPromptMode = "reasoning"
	}
	if shouldUseReasoning && usesReasoningEffort(model) {
		base.MistralReasoningEffort = mapReasoningEffort(model, reasoning)
	}
	return Stream(ctx, model, chat, &base)
}

// usesReasoningEffort reports whether model uses the binary
// reasoning_effort=("none"|"high") knob instead of prompt_mode="reasoning".
// Ports usesReasoningEffort from mistral-conversations.ts.
func usesReasoningEffort(model *ai.Model) bool {
	return model.ID == "mistral-small-2603" || model.ID == "mistral-small-latest" || model.ID == "mistral-medium-3.5"
}

// usesPromptModeReasoning reports whether model selects reasoning via
// prompt_mode="reasoning" (Magistral-style models; everything reasoning-
// capable except the reasoning_effort models above). Ports
// usesPromptModeReasoning from mistral-conversations.ts.
func usesPromptModeReasoning(model *ai.Model) bool {
	return model.Reasoning && !usesReasoningEffort(model)
}

// mapReasoningEffort maps the abstract thinking level to Mistral's
// reasoning_effort value, preferring an explicit model.ThinkingLevelMap
// override and falling back to "high". Ports mapReasoningEffort from
// mistral-conversations.ts.
func mapReasoningEffort(model *ai.Model, level ai.ThinkingLevel) string {
	if model.ThinkingLevelMap != nil {
		if mapped, ok := model.ThinkingLevelMap[level]; ok && mapped != nil {
			return *mapped
		}
	}
	return "high"
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

	apiKey := ""
	if opts != nil {
		apiKey = opts.APIKey
	}
	if err := assertRequestAuth(model.Provider, apiKey); err != nil {
		fail(err)
		return
	}

	params := buildParams(model, chat, opts)
	var payload any = params
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

	body, err := json.Marshal(payload)
	if err != nil {
		fail(err)
		return
	}

	url := strings.TrimRight(model.BaseURL, "/") + chatCompletionsPath
	resp, err := httpretry.Do(ctx, httpretry.Request{
		URL:     url,
		Body:    body,
		Headers: buildHeaders(model, opts, apiKey),
	}, httpretry.Config{
		Opts:              opts,
		Model:             model,
		DefaultMaxRetries: httpretry.DefaultMaxRetries,
		ParseError:        statusError,
	})
	if err != nil {
		fail(err)
		return
	}
	defer resp.Body.Close()

	out.Push(ai.StartEvent{Partial: output.Clone()})

	if err := DecodeStream(out, output, model, resp.Body); err != nil {
		fail(err)
		return
	}

	if ctx.Err() != nil {
		fail(errors.New("Request was aborted"))
		return
	}
	if output.StopReason == ai.StopReasonAborted || output.StopReason == ai.StopReasonError {
		msg := output.ErrorMessage
		if msg == "" {
			msg = "An unknown error occurred"
		}
		fail(errors.New(msg))
		return
	}

	out.Push(ai.DoneEvent{Reason: output.StopReason, Message: output})
}

// assertRequestAuth requires a non-empty apiKey. Unlike the sibling
// anthropic/google packages, upstream mistral-conversations.ts has no
// custom-header escape valve for this check (`if (!apiKey) throw ...` is
// unconditional in `stream`), so this is intentionally stricter than
// assertRequestAuth in ai/apis/anthropic and ai/apis/google.
func assertRequestAuth(provider, apiKey string) error {
	if apiKey != "" {
		return nil
	}
	return fmt.Errorf("No API key for provider: %s", provider)
}

// statusError composes an error for a non-2xx response, matching the
// anthropic/google packages' approach (see anthropic.go's httpStatusError
// doc comment for the full rationale). This replaces formatMistralError's
// SDK-error-shape probing (statusCode/body fields on the thrown error),
// which has no equivalent when talking raw HTTP directly.
func statusError(status int, _, body string) error {
	trimmed := bytes.TrimSpace([]byte(body))
	if len(trimmed) == 0 {
		return fmt.Errorf("%d status code (no body)", status)
	}
	var compact bytes.Buffer
	if json.Compact(&compact, trimmed) == nil {
		return fmt.Errorf("%d %s", status, compact.String())
	}
	return fmt.Errorf("%d %s", status, string(trimmed))
}

// --- request building ---------------------------------------------------

// wireRequest is the Mistral chat completions streaming request body.
type wireRequest struct {
	Model           string        `json:"model"`
	Stream          bool          `json:"stream"`
	Messages        []wireMessage `json:"messages"`
	Tools           []wireTool    `json:"tools,omitempty"`
	Temperature     *float64      `json:"temperature,omitempty"`
	MaxTokens       *int          `json:"max_tokens,omitempty"`
	ToolChoice      any           `json:"tool_choice,omitempty"`
	PromptMode      string        `json:"prompt_mode,omitempty"`
	ReasoningEffort string        `json:"reasoning_effort,omitempty"`
	PromptCacheKey  string        `json:"prompt_cache_key,omitempty"`
}

func buildParams(model *ai.Model, chat ai.Context, opts *ai.StreamOptions) *wireRequest {
	req := &wireRequest{
		Model:    model.ID,
		Stream:   true,
		Messages: toChatMessages(chat.Messages, model, newToolCallIDNormalizer()),
	}

	if len(chat.Tools) > 0 {
		req.Tools = toFunctionTools(chat.Tools)
	}

	if opts != nil {
		if opts.Temperature != nil {
			t := *opts.Temperature
			req.Temperature = &t
		}
		if opts.MaxTokens != nil {
			m := *opts.MaxTokens
			req.MaxTokens = &m
		}
		if choice := mapToolChoice(opts.MistralToolChoice, opts.MistralToolChoiceFunction); choice != nil {
			req.ToolChoice = choice
		}
		req.PromptMode = opts.MistralPromptMode
		req.ReasoningEffort = opts.MistralReasoningEffort
		if shouldUsePromptCaching(opts) {
			req.PromptCacheKey = opts.SessionID
		}
	}

	if chat.SystemPrompt != "" {
		req.Messages = append([]wireMessage{{Role: "system", Content: ai.SanitizeSurrogates(chat.SystemPrompt)}}, req.Messages...)
	}

	return req
}

// shouldUsePromptCaching reports whether prompt_cache_key should be sent:
// cache retention isn't explicitly disabled and a session id is present.
// Ports shouldUsePromptCaching from mistral-conversations.ts.
func shouldUsePromptCaching(opts *ai.StreamOptions) bool {
	return opts.CacheRetention != ai.CacheRetentionNone && opts.SessionID != ""
}

// --- headers -------------------------------------------------------------

// buildHeaders builds the request headers, including Bearer auth and the
// x-affinity KV-cache routing header (Ports the header-building half of
// buildRequestOptions from mistral-conversations.ts).
func buildHeaders(model *ai.Model, opts *ai.StreamOptions, apiKey string) map[string]string {
	defaults := map[string]string{
		"content-type": "application/json",
		"accept":       "text/event-stream",
	}
	if apiKey != "" {
		defaults["authorization"] = "Bearer " + apiKey
	}
	for k, v := range model.Headers {
		defaults[k] = v
	}

	var optHeaders ai.ProviderHeaders
	if opts != nil {
		optHeaders = opts.Headers
	}
	merged := ai.MergeProviderHeaders(defaults, optHeaders)

	// Mistral infrastructure uses x-affinity for KV-cache reuse (prefix
	// caching). Respect an explicit caller-provided header value.
	if opts != nil && shouldUsePromptCaching(opts) {
		if _, ok := merged["x-affinity"]; !ok {
			merged["x-affinity"] = opts.SessionID
		}
	}

	return merged
}

// --- tool-call id normalization -------------------------------------------

// newToolCallIDNormalizer returns a per-request stateful id remapper
// matching Mistral's ^[a-zA-Z0-9]{9}$ tool-call id constraint, with
// collision avoidance across distinct original ids. Ports
// createMistralToolCallIdNormalizer from mistral-conversations.ts.
func newToolCallIDNormalizer() apis.NormalizeToolCallID {
	idMap := make(map[string]string)
	reverseMap := make(map[string]string)

	return func(id string, _ *ai.Model, _ *ai.AssistantMessage) string {
		if existing, ok := idMap[id]; ok {
			return existing
		}
		attempt := 0
		for {
			candidate := deriveMistralToolCallID(id, attempt)
			owner, taken := reverseMap[candidate]
			if !taken || owner == id {
				idMap[id] = candidate
				reverseMap[candidate] = id
				return candidate
			}
			attempt++
		}
	}
}

// deriveMistralToolCallID derives a 9-char alphanumeric candidate id for id
// at the given collision-retry attempt. Ports deriveMistralToolCallId from
// mistral-conversations.ts.
func deriveMistralToolCallID(id string, attempt int) string {
	normalized := stripNonAlnum(id)
	if attempt == 0 && len(normalized) == mistralToolCallIDLength {
		return normalized
	}
	seedBase := normalized
	if seedBase == "" {
		seedBase = id
	}
	seed := seedBase
	if attempt != 0 {
		seed = fmt.Sprintf("%s:%d", seedBase, attempt)
	}
	hashed := stripNonAlnum(ai.ShortHash(seed))
	if len(hashed) > mistralToolCallIDLength {
		hashed = hashed[:mistralToolCallIDLength]
	}
	return hashed
}

func stripNonAlnum(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
