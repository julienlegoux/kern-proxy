// Package openairesponses implements the core of the openai-responses wire
// adapter: Responses API request building and SSE stream decoding into the
// unified event protocol, over raw net/http and the hand-rolled SSE parser
// from ai/internal/sse.
//
// This is the base of the Responses family (epic 7): the Azure
// (ai/apis/azure, issue 02) and Codex (ai/apis/codex, issues 03-04) variants
// build on the exported ConvertMessages/ConvertTools/DecodeStream in this
// package rather than duplicating request/response shaping.
//
// Ports: packages/ai/src/api/openai-responses.ts, openai-responses-shared.ts
package openairesponses

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis"
	"github.com/julienlegoux/kern-proxy/ai/apis/internal/httpretry"
)

const responsesPath = "/responses"

// openAIResponsesMinOutputTokens is OpenAI's minimum accepted
// max_output_tokens; see https://github.com/earendil-works/pi/issues/6265
// (ported verbatim as OPENAI_RESPONSES_MIN_OUTPUT_TOKENS).
const openAIResponsesMinOutputTokens = 16

// openAIToolCallProviders ports OPENAI_TOOL_CALL_PROVIDERS: the providers
// whose tool-call ids use this family's `{callId}|{itemId}` convention.
var openAIToolCallProviders = map[string]bool{
	"openai":       true,
	"openai-codex": true,
	"opencode":     true,
}

// Stream implements ai.StreamFunc for the openai-responses wire protocol.
func Stream(ctx context.Context, model *ai.Model, chat ai.Context, opts *ai.StreamOptions) *ai.Stream {
	out := ai.NewStream()
	go run(ctx, out, model, chat, opts)
	return out
}

// StreamSimple implements ai.SimpleStreamFunc, translating the abstract
// Reasoning level into StreamOptions.ReasoningEffort via ai.ClampThinkingLevel
// (dropped entirely when the model has no supported level to clamp to, i.e.
// clamps to "off"). Ports the streamSimple half of openai-responses.ts;
// unlike upstream, the redundant synchronous getClientApiKey pre-check is
// omitted -- Stream (via run's assertRequestAuth) already reports a missing
// key as an in-band error event, matching ai.StreamFunc's contract that every
// failure surfaces through the returned stream, never a Go/JS-style throw
// (see the sibling openaicompletions package for the same deviation).
func StreamSimple(ctx context.Context, model *ai.Model, chat ai.Context, opts *ai.SimpleStreamOptions) *ai.Stream {
	apiKey := ""
	if opts != nil {
		apiKey = opts.APIKey
	}
	base := apis.BuildBaseOptions(model, chat, opts, apiKey)
	if opts != nil && opts.Reasoning != "" {
		if clamped := ai.ClampThinkingLevel(model, opts.Reasoning); clamped != ai.ThinkingOff {
			base.ReasoningEffort = clamped
		}
	}
	return Stream(ctx, model, chat, &base)
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
	var headers ai.ProviderHeaders
	if opts != nil {
		apiKey = opts.APIKey
		headers = opts.Headers
	}
	if err := assertRequestAuth(model.Provider, apiKey, headers); err != nil {
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

	url := strings.TrimRight(model.BaseURL, "/") + responsesPath
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

	serviceTier := ""
	if opts != nil {
		serviceTier = opts.ServiceTier
	}
	if err := DecodeStream(out, output, model, resp.Body, ServiceTierOptions{RequestServiceTier: serviceTier}); err != nil {
		fail(err)
		return
	}

	if ctx.Err() != nil {
		fail(errors.New("Request was aborted"))
		return
	}
	if output.StopReason == ai.StopReasonAborted {
		fail(errors.New("Request was aborted"))
		return
	}
	if output.StopReason == ai.StopReasonError {
		msg := output.ErrorMessage
		if msg == "" {
			msg = "An unknown error occurred"
		}
		fail(errors.New(msg))
		return
	}

	out.Push(ai.DoneEvent{Reason: output.StopReason, Message: output})
}

// assertRequestAuth mirrors the sibling anthropic/openaicompletions
// packages' helper of the same name: an empty apiKey is only acceptable when
// the caller supplied their own auth header. Ports getClientApiKey's error
// path from openai-responses.ts.
func assertRequestAuth(provider, apiKey string, headers ai.ProviderHeaders) error {
	if apiKey != "" {
		return nil
	}
	if hasHeader(headers, "authorization") || hasHeader(headers, "cf-aig-authorization") {
		return nil
	}
	return fmt.Errorf("No API key for provider: %s", provider)
}

func hasHeader(headers ai.ProviderHeaders, name string) bool {
	if headers == nil {
		return false
	}
	lower := strings.ToLower(name)
	for k, v := range headers {
		if strings.ToLower(k) == lower && v != nil && strings.TrimSpace(*v) != "" {
			return true
		}
	}
	return false
}

// statusError composes an error for a non-2xx response, matching the
// anthropic/openaicompletions packages' approach: see anthropic.go's
// httpStatusError doc comment for why this reconstructs the SDK-shaped
// "<status> <body>" text inline rather than through a shared ai/internal/httpx
// normalizer (deferred to whichever later issue first needs multi-SDK
// error-shape probing).
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

// --- compat resolution -------------------------------------------------------

// resolvedCompat is the openai-responses subset of ai.Compat with defaults
// applied. Ports getCompat from openai-responses.ts; unlike the
// openai-completions compat matrix (epic 6), there is no vendor
// auto-detection here -- every flag defaults to true unless explicitly
// overridden.
type resolvedCompat struct {
	supportsDeveloperRole      bool
	sendSessionIDHeader        bool
	supportsLongCacheRetention bool
}

func getCompat(model *ai.Model) resolvedCompat {
	r := resolvedCompat{supportsDeveloperRole: true, sendSessionIDHeader: true, supportsLongCacheRetention: true}
	c := model.Compat
	if c == nil {
		return r
	}
	if c.SupportsDeveloperRole != nil {
		r.supportsDeveloperRole = *c.SupportsDeveloperRole
	}
	if c.SendSessionIDHeader != nil {
		r.sendSessionIDHeader = *c.SendSessionIDHeader
	}
	if c.SupportsLongCacheRetention != nil {
		r.supportsLongCacheRetention = *c.SupportsLongCacheRetention
	}
	return r
}

// --- prompt caching ----------------------------------------------------------

// openAIPromptCacheKeyMaxLength is OpenAI's prompt_cache_key length limit.
// Duplicated from ai/apis/openaicompletions/promptcache.go: both ports read
// the same upstream openai-prompt-cache.ts, but Go package boundaries don't
// share unexported helpers the way the two TS files share one module.
const openAIPromptCacheKeyMaxLength = 64

// clampOpenAIPromptCacheKey truncates key to OpenAI's 64-character limit,
// counting Unicode code points (not bytes) to match upstream's
// `Array.from(key)` splitting.
func clampOpenAIPromptCacheKey(key string) string {
	runes := []rune(key)
	if len(runes) <= openAIPromptCacheKeyMaxLength {
		return key
	}
	return string(runes[:openAIPromptCacheKeyMaxLength])
}

// getPromptCacheRetention ports the ternary in openai-responses.ts's
// getPromptCacheRetention: OpenAI's own 24h extended retention, gated on both
// the caller's preference and compat.supportsLongCacheRetention.
func getPromptCacheRetention(compat resolvedCompat, retention ai.CacheRetention) string {
	if retention == ai.CacheRetentionLong && compat.supportsLongCacheRetention {
		return "24h"
	}
	return ""
}

// resolveCacheRetention resolves the caller's preference against
// PI_CACHE_RETENTION (checked in env first, then the process environment),
// defaulting to "short". Ports resolveCacheRetention from
// openai-responses.ts (identical to the openaicompletions package's helper
// of the same name/behavior).
func resolveCacheRetention(retention ai.CacheRetention, env ai.ProviderEnv) ai.CacheRetention {
	if retention != "" {
		return retention
	}
	if providerEnvValue("PI_CACHE_RETENTION", env) == "long" {
		return ai.CacheRetentionLong
	}
	return ai.CacheRetentionShort
}

// providerEnvValue reads name from the request-scoped env override first,
// falling back to the process environment.
func providerEnvValue(name string, env ai.ProviderEnv) string {
	if v, ok := env[name]; ok && v != "" {
		return v
	}
	return os.Getenv(name)
}

// --- request building --------------------------------------------------------

type wireReasoning struct {
	Effort  string `json:"effort"`
	Summary string `json:"summary,omitempty"`
}

// wireRequest is the OpenAI Responses streaming request body.
type wireRequest struct {
	Model                string          `json:"model"`
	Input                []wireInputItem `json:"input"`
	Stream               bool            `json:"stream"`
	Store                *bool           `json:"store,omitempty"`
	PromptCacheKey       string          `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention string          `json:"prompt_cache_retention,omitempty"`
	MaxOutputTokens      *int            `json:"max_output_tokens,omitempty"`
	Temperature          *float64        `json:"temperature,omitempty"`
	ServiceTier          string          `json:"service_tier,omitempty"`
	Tools                *[]wireTool     `json:"tools,omitempty"`
	Reasoning            *wireReasoning  `json:"reasoning,omitempty"`
	Include              []string        `json:"include,omitempty"`
}

func boolPtr(b bool) *bool { return &b }

func buildParams(model *ai.Model, chat ai.Context, opts *ai.StreamOptions) *wireRequest {
	compat := getCompat(model)

	systemPromptRole := "system"
	if model.Reasoning && compat.supportsDeveloperRole {
		systemPromptRole = "developer"
	}
	items := ConvertMessages(model, chat, ConvertMessagesOptions{
		IncludeSystemPrompt:      true,
		SystemPromptRole:         systemPromptRole,
		AllowedToolCallProviders: openAIToolCallProviders,
	})

	var sessionID string
	var env ai.ProviderEnv
	var rawRetention ai.CacheRetention
	if opts != nil {
		sessionID = opts.SessionID
		env = opts.Env
		rawRetention = opts.CacheRetention
	}
	retention := resolveCacheRetention(rawRetention, env)

	params := &wireRequest{
		Model:  model.ID,
		Input:  items,
		Stream: true,
		Store:  boolPtr(false),
	}

	if retention != ai.CacheRetentionNone {
		params.PromptCacheKey = clampOpenAIPromptCacheKey(sessionID)
	}
	params.PromptCacheRetention = getPromptCacheRetention(compat, retention)

	if opts != nil && opts.MaxTokens != nil && *opts.MaxTokens > 0 {
		maxTokens := *opts.MaxTokens
		if maxTokens < openAIResponsesMinOutputTokens {
			maxTokens = openAIResponsesMinOutputTokens
		}
		params.MaxOutputTokens = &maxTokens
	}

	if opts != nil && opts.Temperature != nil {
		t := *opts.Temperature
		params.Temperature = &t
	}

	if opts != nil && opts.ServiceTier != "" {
		params.ServiceTier = opts.ServiceTier
	}

	if len(chat.Tools) > 0 {
		tools := ConvertTools(chat.Tools, ConvertToolsOptions{})
		params.Tools = &tools
	} else if hasToolHistory(chat.Messages) {
		empty := []wireTool{}
		params.Tools = &empty
	}

	applyReasoning(params, model, opts)

	return params
}

// applyReasoning ports the model.reasoning-gated if/else-if in
// openai-responses.ts's buildParams: a requested effort or summary sends the
// full {effort, summary} shape plus the encrypted-content include directive;
// otherwise (no request) the model's own "off" mapping is sent as a bare
// effort, unless explicitly suppressed (ThinkingLevelMap["off"] = nil).
//
// Upstream additionally gates the "no request" branch on
// `model.provider !== "github-copilot"`; github-copilot bindings don't exist
// yet (epic 11), so that branch is unconditionally reachable here and this
// gate is deferred rather than speculatively implemented.
func applyReasoning(params *wireRequest, model *ai.Model, opts *ai.StreamOptions) {
	if !model.Reasoning {
		return
	}
	var effortRequested ai.ThinkingLevel
	var summaryRequested string
	if opts != nil {
		effortRequested = opts.ReasoningEffort
		summaryRequested = opts.ReasoningSummary
	}

	if effortRequested != "" || summaryRequested != "" {
		effort := "medium"
		if effortRequested != "" {
			if mapped, ok := mappedThinkingLevel(model, effortRequested); ok {
				effort = mapped
			} else {
				effort = string(effortRequested)
			}
		}
		summary := summaryRequested
		if summary == "" {
			summary = "auto"
		}
		params.Reasoning = &wireReasoning{Effort: effort, Summary: summary}
		params.Include = []string{"reasoning.encrypted_content"}
		return
	}

	if v, present := model.ThinkingLevelMap[ai.ThinkingOff]; present && v == nil {
		return
	}
	effort := "none"
	if v, ok := mappedThinkingLevel(model, ai.ThinkingOff); ok {
		effort = v
	}
	params.Reasoning = &wireReasoning{Effort: effort}
}

// mappedThinkingLevel looks up model.ThinkingLevelMap[level]. ok is true only
// for an explicit non-nil string mapping; both "absent" and "explicit null"
// return ok=false.
func mappedThinkingLevel(model *ai.Model, level ai.ThinkingLevel) (string, bool) {
	v, present := model.ThinkingLevelMap[level]
	if !present || v == nil {
		return "", false
	}
	return *v, true
}

// hasToolHistory reports whether messages contain any tool calls or tool
// results, in which case some proxies require the tools param present even
// when no tools are offered this turn.
func hasToolHistory(messages []ai.Message) bool {
	for _, msg := range messages {
		switch m := msg.(type) {
		case ai.ToolResultMessage:
			return true
		case *ai.AssistantMessage:
			for _, b := range m.Content {
				if _, ok := b.(ai.ToolCall); ok {
					return true
				}
			}
		}
	}
	return false
}

// --- headers -----------------------------------------------------------------

// buildHeaders builds the request headers, including the session-affinity
// headers used by proxies/gateways that route by session:
// x-client-request-id is sent whenever a session id is present and caching
// isn't disabled for this request; session_id is additionally gated on
// compat.sendSessionIdHeader. Ports the header-building half of createClient
// from openai-responses.ts (github-copilot's dynamic headers are deferred:
// see applyReasoning's doc comment).
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

	compat := getCompat(model)
	var sessionID string
	var env ai.ProviderEnv
	var rawRetention ai.CacheRetention
	if opts != nil {
		sessionID = opts.SessionID
		env = opts.Env
		rawRetention = opts.CacheRetention
	}
	retention := resolveCacheRetention(rawRetention, env)
	if sessionID != "" && retention != ai.CacheRetentionNone {
		if compat.sendSessionIDHeader {
			defaults["session_id"] = sessionID
		}
		defaults["x-client-request-id"] = sessionID
	}

	var optHeaders ai.ProviderHeaders
	if opts != nil {
		optHeaders = opts.Headers
	}
	return ai.MergeProviderHeaders(defaults, optHeaders)
}
