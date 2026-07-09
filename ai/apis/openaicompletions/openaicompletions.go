// Package openaicompletions implements the core of the openai-completions
// wire adapter: chat-completions request building and SSE stream decoding
// into the unified event protocol, over raw net/http and the hand-rolled SSE
// parser from ai/internal/sse.
//
// Issue 01 (epic 6) ported the base adapter: request/message building,
// dual-map (index and id) tool-call delta correlation, partial tool-arg JSON
// re-parsing, and prompt/cache usage math. Issue 02 added the vendor compat
// auto-detection matrix (compat.go: detectCompat/getCompat, ~18 tri-state
// flags auto-detected from provider/baseUrl, overridden by explicit
// ai.Model.Compat entries) and wired it into the flags this adapter already
// reads: store, developer role, strict mode, usage-in-streaming, the
// max_tokens/max_completion_tokens field choice, tool-result name, and the
// synthetic post-tool-result assistant bridge. Issue 03 added the 10-value
// thinkingFormat enum's per-format request encoding and same-model thinking
// replay. Issue 04 (final issue of epic 6) added OpenAI's own
// prompt_cache_key/prompt_cache_retention fields (promptcache.go), Anthropic-
// style cache_control replication for vendors whose compat.cacheControlFormat
// is "anthropic" (applyCaching/applyAnthropicCacheControl in this file), and
// session-affinity headers (buildHeaders) — closing out this package's
// upstream fidelity bar.
//
// Ports: packages/ai/src/api/openai-completions.ts, openai-prompt-cache.ts
package openaicompletions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis"
	"github.com/julienlegoux/kern-proxy/ai/apis/internal/httpretry"
	"github.com/julienlegoux/kern-proxy/ai/internal/partialjson"
	"github.com/julienlegoux/kern-proxy/ai/internal/sse"
)

const chatCompletionsPath = "/chat/completions"

// Stream implements ai.StreamFunc for the openai-completions wire protocol.
func Stream(ctx context.Context, model *ai.Model, chat ai.Context, opts *ai.StreamOptions) *ai.Stream {
	out := ai.NewStream()
	go run(ctx, out, model, chat, opts)
	return out
}

// StreamSimple implements ai.SimpleStreamFunc, translating the abstract
// Reasoning level into StreamOptions.ReasoningEffort via ai.ClampThinkingLevel
// (dropped entirely when the model has no supported level to clamp to, i.e.
// clamps to "off"). Ports the streamSimple half of openai-completions.ts;
// toolChoice is out of scope (no upstream epic 6 issue reads it yet).
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

	if err := decodeEvents(out, output, model, resp.Body); err != nil {
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
			msg = "Provider returned an error stop reason"
		}
		fail(errors.New(msg))
		return
	}

	out.Push(ai.DoneEvent{Reason: output.StopReason, Message: output})
}

// assertRequestAuth mirrors the anthropic adapter's assertRequestAuth: an
// empty apiKey is only acceptable when the caller supplied their own auth
// header. Ports getClientApiKey's error path from openai-completions.ts.
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
// anthropic adapter's approach: see its statusError doc comment for why
// this reconstructs the SDK-shaped "<status> <body>" text inline rather than
// through a shared ai/internal/httpx normalizer (deferred to whichever
// later issue first needs multi-SDK error-shape probing).
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

// --- request building ---------------------------------------------------------

// wireRequest is the OpenAI chat-completions streaming request body: the base
// subset, the thinking-format fields from issue 03 (reasoning_effort/thinking/
// enable_thinking/chat_template_kwargs/reasoning/tool_stream, whose shape
// varies per Model.Compat.ThinkingFormat — see applyThinkingFormat), and
// (issue 04) the OpenAI prompt-cache fields.
type wireRequest struct {
	Model               string             `json:"model"`
	Messages            []wireMessage      `json:"messages"`
	Stream              bool               `json:"stream"`
	StreamOptions       *wireStreamOptions `json:"stream_options,omitempty"`
	Store               *bool              `json:"store,omitempty"`
	MaxTokens           *int               `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int               `json:"max_completion_tokens,omitempty"`
	Temperature         *float64           `json:"temperature,omitempty"`
	Tools               *[]wireTool        `json:"tools,omitempty"`
	ToolStream          *bool              `json:"tool_stream,omitempty"`

	// --- thinking-format fields (issue 03): shape depends on
	// compat.thinkingFormat, so Thinking/Reasoning are typed `any` rather than
	// a fixed struct (mirrors upstream's `as any` casts per branch). ---
	ReasoningEffort    string         `json:"reasoning_effort,omitempty"`
	Thinking           any            `json:"thinking,omitempty"`
	EnableThinking     *bool          `json:"enable_thinking,omitempty"`
	ChatTemplateKwargs map[string]any `json:"chat_template_kwargs,omitempty"`
	Reasoning          any            `json:"reasoning,omitempty"`

	// --- OpenAI prompt cache fields (issue 04) ---
	PromptCacheKey       string `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention string `json:"prompt_cache_retention,omitempty"`
}

// wireThinkingObj is the `thinking` object shape used by the "zai" and
// "deepseek" thinkingFormats.
type wireThinkingObj struct {
	Type          string `json:"type"`
	ClearThinking *bool  `json:"clear_thinking,omitempty"`
}

// wireReasoningEffort is the `reasoning` object shape used by the
// "openrouter" and "ant-ling" thinkingFormats.
type wireReasoningEffort struct {
	Effort string `json:"effort"`
}

// wireReasoningEnabled is the `reasoning` object shape used by the
// "together" thinkingFormat.
type wireReasoningEnabled struct {
	Enabled bool `json:"enabled"`
}

type wireStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type wireTool struct {
	Type     string       `json:"type"`
	Function wireFunction `json:"function"`
	// CacheControl marks this tool as an Anthropic-style cache breakpoint
	// (issue 04, cacheControlFormat: "anthropic"); only ever set on the last
	// tool in the list.
	CacheControl *wireCacheControl `json:"cache_control,omitempty"`
}

// wireCacheControl is the Anthropic-style ephemeral cache_control breakpoint
// some completions-compatible vendors (notably OpenRouter's Anthropic models)
// accept when compat.cacheControlFormat is "anthropic". Ports
// OpenAICompatCacheControl from openai-completions.ts.
type wireCacheControl struct {
	Type string `json:"type"`
	TTL  string `json:"ttl,omitempty"`
}

type wireFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
	Strict      *bool          `json:"strict,omitempty"`
}

type wireMessage struct {
	Role       string         `json:"role"`
	Content    any            `json:"content,omitempty"`
	ToolCalls  []wireToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Name       string         `json:"name,omitempty"`

	// --- thinking replay fields (issue 03) ---

	// ReasoningContent/Reasoning/ReasoningText mirror the three known
	// decode-side reasoning field names (see rawDelta.reasoningField): a
	// replayed thinking block is written back under whichever field name its
	// ThinkingSignature recorded. Pointers (not plain strings) so an
	// explicitly forced empty string (requiresReasoningContentOnAssistantMessages)
	// is distinguishable from "not set" and still serializes as `""`.
	ReasoningContent *string `json:"reasoning_content,omitempty"`
	Reasoning        *string `json:"reasoning,omitempty"`
	ReasoningText    *string `json:"reasoning_text,omitempty"`
	// ReasoningDetails replays Google-style encrypted reasoning metadata
	// attached to tool calls (ToolCall.ThoughtSignature), one raw JSON object
	// per tool call that has one.
	ReasoningDetails []json.RawMessage `json:"reasoning_details,omitempty"`
}

type wireContentPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *wireImageURL `json:"image_url,omitempty"`
	// CacheControl marks this text part as an Anthropic-style cache
	// breakpoint (issue 04); only ever set by applyAnthropicCacheControl.
	CacheControl *wireCacheControl `json:"cache_control,omitempty"`
}

type wireImageURL struct {
	URL string `json:"url"`
}

type wireToolCall struct {
	ID       string               `json:"id"`
	Type     string               `json:"type"`
	Function wireToolCallFunction `json:"function"`
}

type wireToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func boolPtr(b bool) *bool { return &b }

func buildParams(model *ai.Model, chat ai.Context, opts *ai.StreamOptions) *wireRequest {
	compat := getCompat(model)

	req := &wireRequest{
		Model:    model.ID,
		Messages: convertMessages(chat, model, compat),
		Stream:   true,
	}

	if compat.supportsUsageInStreaming {
		req.StreamOptions = &wireStreamOptions{IncludeUsage: true}
	}
	if compat.supportsStore {
		req.Store = boolPtr(false)
	}

	maxTokens := model.MaxTokens
	if opts != nil && opts.MaxTokens != nil {
		maxTokens = *opts.MaxTokens
	}
	if maxTokens > 0 {
		if compat.maxTokensField == "max_tokens" {
			req.MaxTokens = &maxTokens
		} else {
			req.MaxCompletionTokens = &maxTokens
		}
	}

	if opts != nil && opts.Temperature != nil {
		t := *opts.Temperature
		req.Temperature = &t
	}

	if len(chat.Tools) > 0 {
		tools := convertTools(chat.Tools, compat)
		req.Tools = &tools
		if compat.zaiToolStream {
			req.ToolStream = boolPtr(true)
		}
	} else if hasToolHistory(chat.Messages) {
		empty := []wireTool{}
		req.Tools = &empty
	}

	applyThinkingFormat(req, model, compat, opts)
	applyCaching(req, model, compat, opts)

	return req
}

// --- prompt caching and cache_control (issue 04) ----------------------------

// applyCaching resolves the effective cache retention and wires OpenAI's own
// prompt_cache_key/prompt_cache_retention fields plus (for vendors whose
// compat.cacheControlFormat is "anthropic") Anthropic-style cache_control
// breakpoints. Ports the caching-related portion of buildParams from
// openai-completions.ts.
func applyCaching(req *wireRequest, model *ai.Model, compat resolvedCompat, opts *ai.StreamOptions) {
	var sessionID string
	var env ai.ProviderEnv
	var rawRetention ai.CacheRetention
	if opts != nil {
		sessionID = opts.SessionID
		env = opts.Env
		rawRetention = opts.CacheRetention
	}
	retention := resolveCacheRetention(rawRetention, env)

	isOpenAI := strings.Contains(model.BaseURL, "api.openai.com")
	longRetentionOK := retention == ai.CacheRetentionLong && compat.supportsLongCacheRetention
	if (isOpenAI && retention != ai.CacheRetentionNone) || longRetentionOK {
		req.PromptCacheKey = clampOpenAIPromptCacheKey(sessionID)
	}
	if longRetentionOK {
		req.PromptCacheRetention = "24h"
	}

	if cacheControl := getCompatCacheControl(compat, retention); cacheControl != nil {
		applyAnthropicCacheControl(req.Messages, req.Tools, cacheControl)
	}
}

// resolveCacheRetention resolves the caller's preference against
// PI_CACHE_RETENTION (checked in env first, then the process environment),
// defaulting to "short". Ports resolveCacheRetention from
// openai-completions.ts.
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
// falling back to the process environment. Mirrors the anthropic package's
// helper of the same name/behavior (see its doc comment for the
// provider-env.ts Bun-sandbox deviation this intentionally omits).
func providerEnvValue(name string, env ai.ProviderEnv) string {
	if v, ok := env[name]; ok && v != "" {
		return v
	}
	return os.Getenv(name)
}

// getCompatCacheControl builds the cache_control breakpoint to replicate
// across the request (system prompt, last tool, last conversation message),
// or nil when the vendor doesn't use the Anthropic cache_control format or
// caching is disabled for this request. Ports getCompatCacheControl.
func getCompatCacheControl(compat resolvedCompat, retention ai.CacheRetention) *wireCacheControl {
	if compat.cacheControlFormat != "anthropic" || retention == ai.CacheRetentionNone {
		return nil
	}
	cc := &wireCacheControl{Type: "ephemeral"}
	if retention == ai.CacheRetentionLong && compat.supportsLongCacheRetention {
		cc.TTL = "1h"
	}
	return cc
}

// applyAnthropicCacheControl replicates cacheControl across the system/
// developer instruction message, the last tool, and the last user/assistant
// message with text content — the same three breakpoints Anthropic's own
// adapter marks. Ports applyAnthropicCacheControl.
func applyAnthropicCacheControl(messages []wireMessage, tools *[]wireTool, cacheControl *wireCacheControl) {
	addCacheControlToSystemPrompt(messages, cacheControl)
	addCacheControlToLastTool(tools, cacheControl)
	addCacheControlToLastConversationMessage(messages, cacheControl)
}

func addCacheControlToSystemPrompt(messages []wireMessage, cacheControl *wireCacheControl) {
	for i := range messages {
		if messages[i].Role == "system" || messages[i].Role == "developer" {
			addCacheControlToTextContent(&messages[i], cacheControl)
			return
		}
	}
}

func addCacheControlToLastTool(tools *[]wireTool, cacheControl *wireCacheControl) {
	if tools == nil || len(*tools) == 0 {
		return
	}
	(*tools)[len(*tools)-1].CacheControl = cacheControl
}

func addCacheControlToLastConversationMessage(messages []wireMessage, cacheControl *wireCacheControl) {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" && messages[i].Role != "assistant" {
			continue
		}
		if addCacheControlToTextContent(&messages[i], cacheControl) {
			return
		}
	}
}

// addCacheControlToTextContent marks msg's text content with cacheControl:
// a plain non-empty string becomes a single-part array (matching upstream's
// string -> [{type:"text",...}] promotion), and an existing part array gets
// its last text part marked. Reports whether a breakpoint was placed.
func addCacheControlToTextContent(msg *wireMessage, cacheControl *wireCacheControl) bool {
	switch content := msg.Content.(type) {
	case string:
		if content == "" {
			return false
		}
		msg.Content = []wireContentPart{{Type: "text", Text: content, CacheControl: cacheControl}}
		return true
	case []wireContentPart:
		for i := len(content) - 1; i >= 0; i-- {
			if content[i].Type == "text" {
				content[i].CacheControl = cacheControl
				return true
			}
		}
		return false
	default:
		return false
	}
}

// --- thinking-format encoding (issue 03) -----------------------------------

// applyThinkingFormat ports openai-completions.ts buildParams' thinkingFormat
// if/else-if chain: exactly one of the 10 ThinkingFormat encodings applies,
// selected by compat.thinkingFormat, and every branch is gated on
// model.Reasoning (non-reasoning models never get a reasoning/thinking
// field).
func applyThinkingFormat(req *wireRequest, model *ai.Model, compat resolvedCompat, opts *ai.StreamOptions) {
	if !model.Reasoning {
		return
	}
	var reasoningEffort ai.ThinkingLevel
	if opts != nil {
		reasoningEffort = opts.ReasoningEffort
	}
	hasEffort := reasoningEffort != ""

	switch compat.thinkingFormat {
	case ai.ThinkingFormatZai:
		if hasEffort {
			req.Thinking = wireThinkingObj{Type: "enabled", ClearThinking: boolPtr(false)}
		} else {
			req.Thinking = wireThinkingObj{Type: "disabled"}
		}
		if hasEffort && compat.supportsReasoningEffort && !thinkingLevelExplicitlyNull(model, reasoningEffort) {
			req.ReasoningEffort = mappedThinkingOrRaw(model, reasoningEffort)
		}

	case ai.ThinkingFormatQwen:
		req.EnableThinking = boolPtr(hasEffort)

	case ai.ThinkingFormatQwenChatTemplate:
		req.ChatTemplateKwargs = map[string]any{"enable_thinking": hasEffort, "preserve_thinking": true}

	case ai.ThinkingFormatChatTemplate:
		if kwargs := buildChatTemplateKwargs(model, compat, reasoningEffort); kwargs != nil {
			req.ChatTemplateKwargs = kwargs
		}

	case ai.ThinkingFormatDeepseek:
		if hasEffort {
			req.Thinking = wireThinkingObj{Type: "enabled"}
		} else if !thinkingLevelExplicitlyNull(model, ai.ThinkingOff) {
			req.Thinking = wireThinkingObj{Type: "disabled"}
		}
		if hasEffort && compat.supportsReasoningEffort {
			req.ReasoningEffort = mappedThinkingOrRaw(model, reasoningEffort)
		}

	case ai.ThinkingFormatOpenRouter:
		if hasEffort {
			req.Reasoning = wireReasoningEffort{Effort: mappedThinkingOrRaw(model, reasoningEffort)}
		} else if !thinkingLevelExplicitlyNull(model, ai.ThinkingOff) {
			req.Reasoning = wireReasoningEffort{Effort: mappedThinkingOrDefault(model, ai.ThinkingOff, "none")}
		}

	case ai.ThinkingFormatAntLing:
		if hasEffort {
			if mapped, ok := mappedThinkingLevel(model, reasoningEffort); ok {
				req.Reasoning = wireReasoningEffort{Effort: mapped}
			}
		}

	case ai.ThinkingFormatTogether:
		req.Reasoning = wireReasoningEnabled{Enabled: hasEffort}
		if hasEffort && compat.supportsReasoningEffort {
			req.ReasoningEffort = mappedThinkingOrRaw(model, reasoningEffort)
		}

	case ai.ThinkingFormatStringThinking:
		if hasEffort {
			req.Thinking = mappedThinkingOrRaw(model, reasoningEffort)
		} else if !thinkingLevelExplicitlyNull(model, ai.ThinkingOff) {
			req.Thinking = mappedThinkingOrDefault(model, ai.ThinkingOff, "none")
		}

	default: // "openai": plain reasoning_effort.
		if hasEffort && compat.supportsReasoningEffort {
			req.ReasoningEffort = mappedThinkingOrRaw(model, reasoningEffort)
		} else if !hasEffort && compat.supportsReasoningEffort {
			if off, ok := mappedThinkingLevel(model, ai.ThinkingOff); ok {
				req.ReasoningEffort = off
			}
		}
	}
}

// mappedThinkingLevel looks up model.ThinkingLevelMap[level]. ok is true only
// for an explicit non-nil string mapping; both "absent" and "explicit null"
// (an unsupported-level marker) return ok=false.
func mappedThinkingLevel(model *ai.Model, level ai.ThinkingLevel) (string, bool) {
	v, present := model.ThinkingLevelMap[level]
	if !present || v == nil {
		return "", false
	}
	return *v, true
}

// thinkingLevelExplicitlyNull reports whether level is present in
// model.ThinkingLevelMap mapped to an explicit nil (TS `null`), the marker
// some formats use to suppress a field entirely rather than send a default.
func thinkingLevelExplicitlyNull(model *ai.Model, level ai.ThinkingLevel) bool {
	v, present := model.ThinkingLevelMap[level]
	return present && v == nil
}

// mappedThinkingOrRaw is the `model.thinkingLevelMap?.[level] ?? raw` pattern
// shared by most thinkingFormat branches: fall back to the raw level string
// when unmapped (including explicit null).
func mappedThinkingOrRaw(model *ai.Model, level ai.ThinkingLevel) string {
	if v, ok := mappedThinkingLevel(model, level); ok {
		return v
	}
	return string(level)
}

// mappedThinkingOrDefault is the `model.thinkingLevelMap?.off ?? def` pattern
// used by the "off" default value in openrouter/string-thinking.
func mappedThinkingOrDefault(model *ai.Model, level ai.ThinkingLevel, def string) string {
	if v, ok := mappedThinkingLevel(model, level); ok {
		return v
	}
	return def
}

// buildChatTemplateKwargs resolves compat.chatTemplateKwargs into concrete
// values for the generic "chat-template" thinkingFormat. Ports
// buildChatTemplateKwargs/resolveChatTemplateKwargValue.
func buildChatTemplateKwargs(model *ai.Model, compat resolvedCompat, level ai.ThinkingLevel) map[string]any {
	if len(compat.chatTemplateKwargs) == 0 {
		return nil
	}
	out := map[string]any{}
	for k, v := range compat.chatTemplateKwargs {
		if resolved, ok := resolveChatTemplateKwargValue(model, level, v); ok {
			out[k] = resolved
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// resolveChatTemplateKwargValue resolves one configured chat_template_kwargs
// value. ok=false means the key is omitted entirely (TS `undefined`); ok=true
// with a nil resolved value means an explicit JSON null is kept.
func resolveChatTemplateKwargValue(model *ai.Model, level ai.ThinkingLevel, value ai.ChatTemplateKwargValue) (any, bool) {
	obj, isObj := value.(map[string]any)
	if !isObj {
		return value, true
	}
	hasEffort := level != ""
	if !hasEffort {
		if omit, _ := obj["omitWhenOff"].(bool); omit {
			return nil, false
		}
	}
	if v, ok := obj["$var"]; ok && v == "thinking.enabled" {
		return hasEffort, true
	}
	key := ai.ThinkingOff
	if hasEffort {
		key = level
	}
	mapped, present := model.ThinkingLevelMap[key]
	if !present {
		if hasEffort {
			return string(level), true
		}
		return nil, false
	}
	if mapped == nil {
		return nil, false
	}
	return *mapped, true
}

func convertTools(tools []ai.Tool, compat resolvedCompat) []wireTool {
	strict := compat.supportsStrictMode
	out := make([]wireTool, 0, len(tools))
	for _, t := range tools {
		var params map[string]any
		if len(t.Parameters) > 0 {
			_ = json.Unmarshal(t.Parameters, &params)
		}
		if params == nil {
			params = map[string]any{}
		}
		fn := wireFunction{Name: t.Name, Description: t.Description, Parameters: params}
		if strict {
			fn.Strict = boolPtr(false)
		}
		out = append(out, wireTool{Type: "function", Function: fn})
	}
	return out
}

// hasToolHistory reports whether messages contain any tool calls or tool
// results, in which case some providers (Anthropic via proxy) require the
// tools param present even when no tools are offered this turn.
func hasToolHistory(messages []ai.Message) bool {
	for _, msg := range messages {
		switch m := msg.(type) {
		case *ai.ToolResultMessage:
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

// normalizeToolCallID adapts a tool-call id for OpenAI's ~40-char limit and
// strips pipe-separated ids (the {call_id}|{id} shape produced by
// openai-responses-family providers replaying into openai-completions).
func normalizeToolCallID(id string, model *ai.Model, _ *ai.AssistantMessage) string {
	if idx := strings.IndexByte(id, '|'); idx >= 0 {
		callID := id[:idx]
		var b strings.Builder
		for _, r := range callID {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
				b.WriteRune(r)
			} else {
				b.WriteRune('_')
			}
		}
		out := b.String()
		if len(out) > 40 {
			out = out[:40]
		}
		return out
	}
	if model.Provider == "openai" && len(id) > 40 {
		return id[:40]
	}
	return id
}

func convertMessages(chat ai.Context, model *ai.Model, compat resolvedCompat) []wireMessage {
	transformed := apis.TransformMessages(chat.Messages, model, normalizeToolCallID)
	out := make([]wireMessage, 0, len(transformed)+1)

	if chat.SystemPrompt != "" {
		role := "system"
		if model.Reasoning && compat.supportsDeveloperRole {
			role = "developer"
		}
		out = append(out, wireMessage{Role: role, Content: ai.SanitizeSurrogates(chat.SystemPrompt)})
	}

	// lastRole tracks the previous message's role so a toolResult -> user
	// transition can be bridged with a synthetic assistant message on
	// providers that require it (compat.requiresAssistantAfterToolResult).
	lastRole := ""
	for i := 0; i < len(transformed); i++ {
		msg := transformed[i]

		if _, isUser := msg.(*ai.UserMessage); isUser && compat.requiresAssistantAfterToolResult && lastRole == "toolResult" {
			out = append(out, wireMessage{Role: "assistant", Content: "I have processed the tool results."})
		}

		switch m := msg.(type) {
		case *ai.UserMessage:
			out = append(out, convertUserMessage(m))
			lastRole = "user"

		case *ai.AssistantMessage:
			wireMsg, ok := convertAssistantMessage(m, model, compat)
			if ok {
				out = append(out, wireMsg)
			}
			lastRole = "assistant"

		case *ai.ToolResultMessage:
			j := i
			for j < len(transformed) {
				next, ok := transformed[j].(*ai.ToolResultMessage)
				if !ok {
					break
				}
				out = append(out, convertToolResultMessage(next, compat))
				j++
			}
			i = j - 1
			lastRole = "toolResult"
		}
	}

	return out
}

func convertUserMessage(m *ai.UserMessage) wireMessage {
	if m.Content.Plain != nil {
		return wireMessage{Role: "user", Content: ai.SanitizeSurrogates(*m.Content.Plain)}
	}
	var parts []wireContentPart
	for _, block := range m.Content.Blocks {
		switch b := block.(type) {
		case ai.TextContent:
			parts = append(parts, wireContentPart{Type: "text", Text: ai.SanitizeSurrogates(b.Text)})
		case ai.ImageContent:
			parts = append(parts, imagePart(b))
		}
	}
	return wireMessage{Role: "user", Content: parts}
}

func imagePart(b ai.ImageContent) wireContentPart {
	return wireContentPart{
		Type:     "image_url",
		ImageURL: &wireImageURL{URL: fmt.Sprintf("data:%s;base64,%s", b.MimeType, b.Data)},
	}
}

// convertAssistantMessage builds the wire assistant message, including
// (issue 03) same-model thinking-block replay: as plain text content parts
// when compat.requiresThinkingAsText, otherwise under the thinking block's
// own signature field (reasoning_content/reasoning/reasoning_text), plus
// replayed Google-style reasoning_details on tool calls. Cross-model thinking
// downgrade already happened upstream in apis.TransformMessages (Epic 4), so
// every ai.ThinkingContent block reaching this function is same-model replay.
func convertAssistantMessage(m *ai.AssistantMessage, model *ai.Model, compat resolvedCompat) (wireMessage, bool) {
	var textParts []wireContentPart
	var textRaw []string
	for _, block := range m.Content {
		if t, ok := block.(ai.TextContent); ok && strings.TrimSpace(t.Text) != "" {
			sanitized := ai.SanitizeSurrogates(t.Text)
			textParts = append(textParts, wireContentPart{Type: "text", Text: sanitized})
			textRaw = append(textRaw, sanitized)
		}
	}
	assistantText := strings.Join(textRaw, "")

	var thinkingBlocks []ai.ThinkingContent
	for _, block := range m.Content {
		if t, ok := block.(ai.ThinkingContent); ok && strings.TrimSpace(t.Thinking) != "" {
			thinkingBlocks = append(thinkingBlocks, t)
		}
	}

	wireMsg := wireMessage{Role: "assistant"}
	hasArrayContent := false

	switch {
	case len(thinkingBlocks) > 0 && compat.requiresThinkingAsText:
		var thinkingTexts []string
		for _, b := range thinkingBlocks {
			thinkingTexts = append(thinkingTexts, ai.SanitizeSurrogates(b.Thinking))
		}
		parts := append([]wireContentPart{{Type: "text", Text: strings.Join(thinkingTexts, "\n\n")}}, textParts...)
		wireMsg.Content = parts
		hasArrayContent = true

	case len(thinkingBlocks) > 0:
		if assistantText != "" {
			wireMsg.Content = assistantText
		}
		signature := thinkingBlocks[0].ThinkingSignature
		if model.Provider == "opencode-go" && signature == "reasoning" {
			signature = "reasoning_content"
		}
		if signature != "" {
			var joined []string
			for _, b := range thinkingBlocks {
				joined = append(joined, b.Thinking)
			}
			text := strings.Join(joined, "\n")
			switch signature {
			case "reasoning_content":
				wireMsg.ReasoningContent = &text
			case "reasoning":
				wireMsg.Reasoning = &text
			case "reasoning_text":
				wireMsg.ReasoningText = &text
			}
		}

	case assistantText != "":
		wireMsg.Content = assistantText
	}

	var toolCalls []wireToolCall
	var reasoningDetails []json.RawMessage
	for _, block := range m.Content {
		tc, ok := block.(ai.ToolCall)
		if !ok {
			continue
		}
		args := tc.Arguments
		if args == nil {
			args = map[string]any{}
		}
		argsJSON, _ := json.Marshal(args)
		toolCalls = append(toolCalls, wireToolCall{
			ID:   tc.ID,
			Type: "function",
			Function: wireToolCallFunction{
				Name:      tc.Name,
				Arguments: string(argsJSON),
			},
		})
		if tc.ThoughtSignature != "" && json.Valid([]byte(tc.ThoughtSignature)) {
			reasoningDetails = append(reasoningDetails, json.RawMessage(tc.ThoughtSignature))
		}
	}
	wireMsg.ToolCalls = toolCalls
	if len(reasoningDetails) > 0 {
		wireMsg.ReasoningDetails = reasoningDetails
	}

	if compat.requiresReasoningContentOnAssistantMessages && model.Reasoning && wireMsg.ReasoningContent == nil {
		empty := ""
		wireMsg.ReasoningContent = &empty
	}

	hasContent := hasArrayContent
	if !hasContent {
		if s, ok := wireMsg.Content.(string); ok && s != "" {
			hasContent = true
		}
	}
	if !hasContent && len(toolCalls) == 0 {
		return wireMessage{}, false
	}
	return wireMsg, true
}

func convertToolResultMessage(m *ai.ToolResultMessage, compat resolvedCompat) wireMessage {
	var texts []string
	hasImages := false
	for _, c := range m.Content {
		switch b := c.(type) {
		case ai.TextContent:
			texts = append(texts, b.Text)
		case ai.ImageContent:
			hasImages = true
		}
	}
	text := strings.Join(texts, "\n")
	if text == "" {
		if hasImages {
			text = "(see attached image)"
		} else {
			text = "(no tool output)"
		}
	}
	wireMsg := wireMessage{Role: "tool", Content: ai.SanitizeSurrogates(text), ToolCallID: m.ToolCallID}
	if compat.requiresToolResultName && m.ToolName != "" {
		wireMsg.Name = m.ToolName
	}
	return wireMsg
}

// --- headers -------------------------------------------------------------

// buildHeaders builds the request headers, including (issue 04) the
// session-affinity headers used by proxies/gateways that route by session:
// session_id, x-client-request-id, and x-session-affinity, all set to the
// session id when compat.sendSessionAffinityHeaders is enabled and caching
// isn't disabled for this request. Ports the header-building half of
// createClient from openai-completions.ts.
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
	if sessionID != "" && retention != ai.CacheRetentionNone && compat.sendSessionAffinityHeaders {
		defaults["session_id"] = sessionID
		defaults["x-client-request-id"] = sessionID
		defaults["x-session-affinity"] = sessionID
	}

	var optHeaders ai.ProviderHeaders
	if opts != nil {
		optHeaders = opts.Headers
	}
	return ai.MergeProviderHeaders(defaults, optHeaders)
}

// --- streaming decode ------------------------------------------------------

// toolCallBlock tracks the scratch state for one streaming tool-call content
// block: the partial-JSON argument buffer plus the stream index/id keys used
// to correlate future deltas back to it. Kept out of ai.ToolCall itself so
// output.Content never carries scratch fields that would need stripping
// before persistence.
type toolCallBlock struct {
	contentIndex int
	partialArgs  strings.Builder
}

type textBlock struct {
	contentIndex int
}

type thinkingBlock struct {
	contentIndex int
}

func decodeEvents(out *ai.Stream, output *ai.AssistantMessage, model *ai.Model, body io.Reader) error {
	reader := sse.NewReader(body)

	var tb *textBlock
	var think *thinkingBlock
	var thinkField string
	toolByIndex := map[int]*toolCallBlock{}
	toolByID := map[string]*toolCallBlock{}
	var toolOrder []*toolCallBlock
	hasFinishReason := false
	// pendingReasoningDetails buffers encrypted reasoning_details (see
	// rawReasoningDetail) that arrive before their matching tool-call id is
	// known, keyed by tool-call id; applied as soon as a block with that id
	// exists (see ensureToolCallBlock).
	pendingReasoningDetails := map[string]string{}

	ensureTextBlock := func() *textBlock {
		if tb == nil {
			output.Content = append(output.Content, ai.TextContent{})
			tb = &textBlock{contentIndex: len(output.Content) - 1}
			out.Push(ai.TextStartEvent{ContentIndex: tb.contentIndex, Partial: output.Clone()})
		}
		return tb
	}

	ensureThinkingBlock := func(field string) *thinkingBlock {
		if think == nil {
			output.Content = append(output.Content, ai.ThinkingContent{ThinkingSignature: field})
			think = &thinkingBlock{contentIndex: len(output.Content) - 1}
			thinkField = field
			out.Push(ai.ThinkingStartEvent{ContentIndex: think.contentIndex, Partial: output.Clone()})
		}
		return think
	}

	ensureToolCallBlock := func(delta rawToolCallDelta) *toolCallBlock {
		var block *toolCallBlock
		if delta.Index != nil {
			block = toolByIndex[*delta.Index]
		}
		if block == nil && delta.ID != "" {
			block = toolByID[delta.ID]
		}
		if block == nil {
			name := ""
			if delta.Function != nil {
				name = delta.Function.Name
			}
			output.Content = append(output.Content, ai.ToolCall{ID: delta.ID, Name: name, Arguments: map[string]any{}})
			block = &toolCallBlock{contentIndex: len(output.Content) - 1}
			toolOrder = append(toolOrder, block)
			out.Push(ai.ToolCallStartEvent{ContentIndex: block.contentIndex, Partial: output.Clone()})
		}
		if delta.Index != nil {
			toolByIndex[*delta.Index] = block
		}
		if delta.ID != "" {
			toolByID[delta.ID] = block
		}
		// Apply any reasoning_details buffered before this block's id was
		// known. Uses the id already stored on the block's ai.ToolCall (set
		// at creation from delta.ID above), matching upstream's
		// applyPendingReasoningDetail(block), which reads block.id at this
		// same point — a block whose id only becomes known on a *later*
		// delta does not get a second chance here (upstream quirk, ported
		// faithfully).
		if id := output.Content[block.contentIndex].(ai.ToolCall).ID; id != "" {
			if sig, ok := pendingReasoningDetails[id]; ok {
				cur := output.Content[block.contentIndex].(ai.ToolCall)
				cur.ThoughtSignature = sig
				output.Content[block.contentIndex] = cur
				delete(pendingReasoningDetails, id)
			}
		}
		return block
	}

	for {
		ev, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if ev.Data == "[DONE]" {
			continue
		}

		var chunk rawChunk
		if err := json.Unmarshal([]byte(ev.Data), &chunk); err != nil {
			// Malformed/non-object payload: skip, matching upstream's
			// `if (!chunk || typeof chunk !== "object") continue;` guard.
			continue
		}

		if chunk.ID != "" {
			if output.ResponseID == "" {
				output.ResponseID = chunk.ID
			}
		}
		if chunk.Model != "" && chunk.Model != model.ID && output.ResponseModel == "" {
			output.ResponseModel = chunk.Model
		}
		if chunk.Usage != nil {
			output.Usage = parseChunkUsage(*chunk.Usage, model)
		}

		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]

		if chunk.Usage == nil && choice.Usage != nil {
			output.Usage = parseChunkUsage(*choice.Usage, model)
		}

		if choice.FinishReason != nil {
			reason, errMsg := mapStopReason(*choice.FinishReason)
			output.StopReason = reason
			if errMsg != "" {
				output.ErrorMessage = errMsg
			}
			hasFinishReason = true
		}

		if choice.Delta == nil {
			continue
		}
		delta := choice.Delta

		if delta.Content != nil && *delta.Content != "" {
			block := ensureTextBlock()
			cur := output.Content[block.contentIndex].(ai.TextContent)
			cur.Text += *delta.Content
			output.Content[block.contentIndex] = cur
			out.Push(ai.TextDeltaEvent{ContentIndex: block.contentIndex, Delta: *delta.Content, Partial: output.Clone()})
		}

		if field, text := delta.reasoningField(); field != "" {
			if think != nil && thinkField != field {
				// Only the first non-empty reasoning field is used, matching
				// upstream's "use the first non-empty reasoning field" rule.
			} else {
				block := ensureThinkingBlock(field)
				cur := output.Content[block.contentIndex].(ai.ThinkingContent)
				cur.Thinking += text
				output.Content[block.contentIndex] = cur
				out.Push(ai.ThinkingDeltaEvent{ContentIndex: block.contentIndex, Delta: text, Partial: output.Clone()})
			}
		}

		for _, tc := range delta.ToolCalls {
			block := ensureToolCallBlock(tc)
			cur := output.Content[block.contentIndex].(ai.ToolCall)
			if cur.ID == "" && tc.ID != "" {
				cur.ID = tc.ID
			}
			if tc.Function != nil && cur.Name == "" && tc.Function.Name != "" {
				cur.Name = tc.Function.Name
			}
			argDelta := ""
			if tc.Function != nil && tc.Function.Arguments != "" {
				argDelta = tc.Function.Arguments
				block.partialArgs.WriteString(argDelta)
				cur.Arguments = partialjson.ParseStreamingObject(block.partialArgs.String())
			}
			output.Content[block.contentIndex] = cur
			out.Push(ai.ToolCallDeltaEvent{ContentIndex: block.contentIndex, Delta: argDelta, Partial: output.Clone()})
		}

		for _, raw := range delta.ReasoningDetails {
			var detail rawReasoningDetail
			if err := json.Unmarshal(raw, &detail); err != nil || !detail.isEncrypted() {
				continue
			}
			serialized, err := json.Marshal(detail)
			if err != nil {
				continue
			}
			if block, ok := toolByID[detail.ID]; ok {
				cur := output.Content[block.contentIndex].(ai.ToolCall)
				cur.ThoughtSignature = string(serialized)
				output.Content[block.contentIndex] = cur
			} else {
				pendingReasoningDetails[detail.ID] = string(serialized)
			}
		}
	}

	if tb != nil {
		cur := output.Content[tb.contentIndex].(ai.TextContent)
		out.Push(ai.TextEndEvent{ContentIndex: tb.contentIndex, Content: cur.Text, Partial: output.Clone()})
	}
	if think != nil {
		cur := output.Content[think.contentIndex].(ai.ThinkingContent)
		out.Push(ai.ThinkingEndEvent{ContentIndex: think.contentIndex, Content: cur.Thinking, Partial: output.Clone()})
	}
	for _, block := range toolOrder {
		cur := output.Content[block.contentIndex].(ai.ToolCall)
		cur.Arguments = partialjson.ParseStreamingObject(block.partialArgs.String())
		output.Content[block.contentIndex] = cur
		out.Push(ai.ToolCallEndEvent{ContentIndex: block.contentIndex, ToolCall: cur, Partial: output.Clone()})
	}

	if !hasFinishReason {
		return errors.New("Stream ended without finish_reason")
	}
	return nil
}

// --- wire event decoding ------------------------------------------------------

type rawUsage struct {
	PromptTokens            int                     `json:"prompt_tokens"`
	CompletionTokens        int                     `json:"completion_tokens"`
	PromptCacheHitTokens    int                     `json:"prompt_cache_hit_tokens"`
	PromptTokensDetails     *rawPromptTokenDetails  `json:"prompt_tokens_details"`
	CompletionTokensDetails *rawCompletionTokenInfo `json:"completion_tokens_details"`
}

type rawPromptTokenDetails struct {
	CachedTokens     int `json:"cached_tokens"`
	CacheWriteTokens int `json:"cache_write_tokens"`
}

type rawCompletionTokenInfo struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

type rawToolCallDelta struct {
	Index    *int                  `json:"index"`
	ID       string                `json:"id"`
	Function *rawToolCallDeltaFunc `json:"function"`
}

type rawToolCallDeltaFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type rawDelta struct {
	Content          *string            `json:"content"`
	ReasoningContent string             `json:"reasoning_content"`
	Reasoning        string             `json:"reasoning"`
	ReasoningText    string             `json:"reasoning_text"`
	ToolCalls        []rawToolCallDelta `json:"tool_calls"`
	ReasoningDetails []json.RawMessage  `json:"reasoning_details"`
}

// rawReasoningDetail is a Google-style encrypted reasoning-details entry
// (isEncryptedReasoningDetail upstream): opaque metadata for reusing thought
// context on a subsequent turn, attached to whichever tool call shares its id.
type rawReasoningDetail struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Data string `json:"data"`
}

// isEncrypted validates the shape upstream's isEncryptedReasoningDetail
// checks: a non-empty id and data on the "reasoning.encrypted" type.
func (d rawReasoningDetail) isEncrypted() bool {
	return d.Type == "reasoning.encrypted" && d.ID != "" && d.Data != ""
}

// reasoningField returns the first non-empty reasoning field name and text,
// checked in upstream's documented order (reasoning_content, reasoning,
// reasoning_text) to avoid double-counting providers that echo the same text
// on more than one field.
func (d *rawDelta) reasoningField() (string, string) {
	if d.ReasoningContent != "" {
		return "reasoning_content", d.ReasoningContent
	}
	if d.Reasoning != "" {
		return "reasoning", d.Reasoning
	}
	if d.ReasoningText != "" {
		return "reasoning_text", d.ReasoningText
	}
	return "", ""
}

type rawChoice struct {
	Index        int       `json:"index"`
	Delta        *rawDelta `json:"delta"`
	FinishReason *string   `json:"finish_reason"`
	Usage        *rawUsage `json:"usage"`
}

type rawChunk struct {
	ID      string      `json:"id"`
	Model   string      `json:"model"`
	Choices []rawChoice `json:"choices"`
	Usage   *rawUsage   `json:"usage"`
}

// parseChunkUsage computes the unified Usage from a chunk's raw usage block.
// input = prompt_tokens - cached - cache_write (never negative); output is
// completion_tokens as-is (it already includes reasoning tokens upstream, so
// no separate subtraction is needed).
func parseChunkUsage(raw rawUsage, model *ai.Model) ai.Usage {
	cacheRead := raw.PromptCacheHitTokens
	cacheWrite := 0
	if raw.PromptTokensDetails != nil {
		cacheRead = raw.PromptTokensDetails.CachedTokens
		cacheWrite = raw.PromptTokensDetails.CacheWriteTokens
	}
	input := raw.PromptTokens - cacheRead - cacheWrite
	if input < 0 {
		input = 0
	}
	output := raw.CompletionTokens
	reasoning := 0
	if raw.CompletionTokensDetails != nil {
		reasoning = raw.CompletionTokensDetails.ReasoningTokens
	}
	usage := ai.Usage{
		Input:       input,
		Output:      output,
		CacheRead:   cacheRead,
		CacheWrite:  cacheWrite,
		Reasoning:   &reasoning,
		TotalTokens: input + output + cacheRead + cacheWrite,
	}
	ai.CalculateCost(model, &usage)
	return usage
}

func mapStopReason(reason string) (ai.StopReason, string) {
	switch reason {
	case "stop", "end":
		return ai.StopReasonStop, ""
	case "length":
		return ai.StopReasonLength, ""
	case "function_call", "tool_calls":
		return ai.StopReasonToolUse, ""
	case "content_filter":
		return ai.StopReasonError, "Provider finish_reason: content_filter"
	case "network_error":
		return ai.StopReasonError, "Provider finish_reason: network_error"
	default:
		return ai.StopReasonError, "Provider finish_reason: " + reason
	}
}
