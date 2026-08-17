package codex

// Ports: the buildRequestBody / applyReasoning / getServiceTierCostMultiplier /
// resolveCodexServiceTier request-building half of
// packages/ai/src/api/openai-codex-responses.ts.

import (
	"strings"

	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/apis/openairesponses"
)

// codexToolCallProviders ports CODEX_TOOL_CALL_PROVIDERS: the providers whose
// tool-call ids use this family's `{callId}|{itemId}` convention.
var codexToolCallProviders = map[string]bool{
	"openai":       true,
	"openai-codex": true,
	"opencode":     true,
}

const defaultTextVerbosity = "low"

type wireReasoning struct {
	Effort  string `json:"effort"`
	Summary string `json:"summary,omitempty"`
}

type wireText struct {
	Verbosity string `json:"verbosity,omitempty"`
}

// wireRequest is the Codex Responses streaming request body. Unlike the
// shared openairesponses core's wireRequest, Codex never sends
// max_output_tokens, always sends store/stream/instructions/text/include/
// tool_choice/parallel_tool_calls, and only sends prompt_cache_key when a
// sessionId is present (ports RequestBody + buildRequestBody).
type wireRequest struct {
	Model             string         `json:"model"`
	Store             bool           `json:"store"`
	Stream            bool           `json:"stream"`
	Instructions      string         `json:"instructions,omitempty"`
	Input             []any          `json:"input"`
	Text              *wireText      `json:"text,omitempty"`
	Include           []string       `json:"include,omitempty"`
	PromptCacheKey    string         `json:"prompt_cache_key,omitempty"`
	ToolChoice        string         `json:"tool_choice,omitempty"`
	ParallelToolCalls bool           `json:"parallel_tool_calls,omitempty"`
	Temperature       *float64       `json:"temperature,omitempty"`
	ServiceTier       string         `json:"service_tier,omitempty"`
	Tools             []any          `json:"tools,omitempty"`
	Reasoning         *wireReasoning `json:"reasoning,omitempty"`
}

// buildRequestBody ports buildRequestBody: request/message shaping over the
// shared openairesponses.ConvertMessages/ConvertTools, with
// includeSystemPrompt:false (the system prompt travels in the top-level
// `instructions` field instead of as an input item, unlike the core/Azure
// variants).
//
// Tool conversion reuses the shared ConvertTools with its default (false)
// strict flag rather than upstream's `convertResponsesTools(tools, { strict:
// null })`: the wire difference is `"strict": false` (this port) vs
// upstream's `"strict": null`, which no upstream or ported test asserts on --
// see docs/PORTING.md's Codex row for this documented deviation.
func buildRequestBody(model *ai.Model, chat ai.Context, opts *ai.StreamOptions) *wireRequest {
	items := openairesponses.ConvertMessages(model, chat, openairesponses.ConvertMessagesOptions{
		IncludeSystemPrompt:      false,
		AllowedToolCallProviders: codexToolCallProviders,
	})
	if items == nil {
		items = []any{}
	}

	instructions := chat.SystemPrompt
	if instructions == "" {
		instructions = "You are a helpful assistant."
	}

	var sessionID, textVerbosity string
	var temperature *float64
	var serviceTier string
	if opts != nil {
		sessionID = opts.SessionID
		textVerbosity = opts.TextVerbosity
		temperature = opts.Temperature
		serviceTier = opts.ServiceTier
	}
	if textVerbosity == "" {
		textVerbosity = defaultTextVerbosity
	}

	body := &wireRequest{
		Model:             model.ID,
		Store:             false,
		Stream:            true,
		Instructions:      instructions,
		Input:             items,
		Text:              &wireText{Verbosity: textVerbosity},
		Include:           []string{"reasoning.encrypted_content"},
		PromptCacheKey:    clampOpenAIPromptCacheKey(sessionID),
		ToolChoice:        "auto",
		ParallelToolCalls: true,
	}

	if temperature != nil {
		body.Temperature = temperature
	}
	if serviceTier != "" {
		body.ServiceTier = serviceTier
	}
	if len(chat.Tools) > 0 {
		tools := openairesponses.ConvertTools(chat.Tools, openairesponses.ConvertToolsOptions{})
		wireTools := make([]any, len(tools))
		for i, t := range tools {
			wireTools[i] = t
		}
		body.Tools = wireTools
	}

	applyReasoning(body, model, opts)

	return body
}

// applyReasoning ports the reasoningEffort conditional in buildRequestBody:
// unlike the shared core's applyReasoning (which always sends a reasoning
// field when model.Reasoning is true, defaulting to the model's "off"
// mapping), Codex only sends `reasoning` when an effort was explicitly
// requested -- there is no automatic default-off fallback.
func applyReasoning(body *wireRequest, model *ai.Model, opts *ai.StreamOptions) {
	if opts == nil || opts.ReasoningEffort == "" {
		return
	}

	var effort *string
	if opts.ReasoningEffort == ai.ThinkingOff {
		// Upstream's reasoningEffort "none" maps through
		// model.thinkingLevelMap?.off, defaulting to the literal "none".
		if v, present := model.ThinkingLevelMap[ai.ThinkingOff]; present {
			effort = v
		} else {
			none := "none"
			effort = &none
		}
	} else if v, present := model.ThinkingLevelMap[opts.ReasoningEffort]; present {
		effort = v
	} else {
		raw := string(opts.ReasoningEffort)
		effort = &raw
	}

	if effort == nil {
		// An explicit nil mapping (e.g. ThinkingLevelMap{ThinkingOff: nil})
		// suppresses the reasoning field entirely, matching upstream's
		// `if (effort !== null)` guard.
		return
	}

	summary := opts.ReasoningSummary
	if summary == "" {
		summary = "auto"
	}
	body.Reasoning = &wireReasoning{Effort: *effort, Summary: summary}
}

// resolveCodexServiceTier ports resolveCodexServiceTier: Codex's backend
// echoes "default" for a request-sent flex/priority tier under some
// conditions, so the client-requested tier is preferred over a reported
// "default" -- otherwise the response's own tier wins, falling back to the
// requested tier when the response reports none.
func resolveCodexServiceTier(responseServiceTier, requestServiceTier string) string {
	if responseServiceTier == "default" && (requestServiceTier == "flex" || requestServiceTier == "priority") {
		return requestServiceTier
	}
	if responseServiceTier != "" {
		return responseServiceTier
	}
	return requestServiceTier
}

// openAIPromptCacheKeyMaxLength duplicates
// openairesponses.openAIPromptCacheKeyMaxLength (see that package's doc
// comment on clampOpenAIPromptCacheKey for why: Go package boundaries don't
// share unexported constants the way the two TS files share one module).
const openAIPromptCacheKeyMaxLength = 64

// clampOpenAIPromptCacheKey duplicates
// openairesponses.clampOpenAIPromptCacheKey: truncates key to OpenAI's
// 64-character limit, counting Unicode code points. An empty key returns
// empty (the "unset" case upstream represents as `undefined`).
func clampOpenAIPromptCacheKey(key string) string {
	runes := []rune(key)
	if len(runes) <= openAIPromptCacheKeyMaxLength {
		return key
	}
	return string(runes[:openAIPromptCacheKeyMaxLength])
}

// resolveCodexURL ports resolveCodexUrl: appends the /codex/responses path
// unless the base URL already ends with it (or with /codex).
func resolveCodexURL(baseURL string) string {
	raw := baseURL
	if raw == "" {
		raw = defaultCodexBaseURL
	}
	normalized := strings.TrimRight(raw, "/")
	switch {
	case strings.HasSuffix(normalized, "/codex/responses"):
		return normalized
	case strings.HasSuffix(normalized, "/codex"):
		return normalized + "/responses"
	default:
		return normalized + "/codex/responses"
	}
}
