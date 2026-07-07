// Package anthropic implements the core of the anthropic-messages wire
// adapter: request building (messages, system, tools, sampling params,
// adaptive/budget-based thinking, cache_control), OAuth Claude Code
// impersonation, and SSE stream decoding into the unified event protocol,
// over raw net/http and the hand-rolled SSE parser from ai/internal/sse.
//
// Out of scope (follow-up issue in epic 5, layered on top of this core):
//   - Retry/overflow classifier integration and the live smoke — epic 5
//     issue 04.
//
// Ports: packages/ai/src/api/anthropic-messages.ts
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis"
	"github.com/julienlegoux/kern-proxy/ai/internal/partialjson"
	"github.com/julienlegoux/kern-proxy/ai/internal/sse"
)

const (
	anthropicVersion             = "2023-06-01"
	messagesPath                 = "/v1/messages"
	fineGrainedToolStreamingBeta = "fine-grained-tool-streaming-2025-05-14"

	// claudeCodeBeta and oauthBeta are unconditionally required on every OAuth
	// (Claude Pro/Max) request, in this order, ahead of any other beta
	// features. Ports the hardcoded prefix of createClient's OAuth
	// betaFeatures list in anthropic-messages.ts.
	claudeCodeBeta = "claude-code-20250219"
	oauthBeta      = "oauth-2025-04-20"

	// claudeCodeVersion is the user-agent version string OAuth requests
	// impersonate. Ports the claudeCodeVersion stealth-mode constant.
	claudeCodeVersion = "2.1.75"

	// claudeCodeIdentitySystemBlock is injected as the first system block on
	// every OAuth request; Anthropic requires it to authorize Claude Code
	// impersonation. Ports the inline string from buildParams' isOAuthToken
	// branch.
	claudeCodeIdentitySystemBlock = "You are Claude Code, Anthropic's official CLI for Claude."
)

// claudeCodeTools is Claude Code 2.x's canonical tool-name casing, used to
// impersonate CC's tool vocabulary on the wire when running on an OAuth
// credential. Source: https://cchistory.mariozechner.at/data/prompts-2.1.11.md
// (see upstream's own comment). Ports the claudeCodeTools table from
// anthropic-messages.ts.
var claudeCodeTools = []string{
	"Read", "Write", "Edit", "Bash", "Grep", "Glob",
	"AskUserQuestion", "EnterPlanMode", "ExitPlanMode", "KillShell",
	"NotebookEdit", "Skill", "Task", "TaskOutput", "TodoWrite",
	"WebFetch", "WebSearch",
}

// ccToolLookup maps a lowercased tool name to its Claude Code canonical
// casing, built once from claudeCodeTools. Ports the ccToolLookup Map.
var ccToolLookup = buildCCToolLookup()

func buildCCToolLookup() map[string]string {
	m := make(map[string]string, len(claudeCodeTools))
	for _, name := range claudeCodeTools {
		m[strings.ToLower(name)] = name
	}
	return m
}

// isOAuthToken reports whether apiKey is a Claude Pro/Max OAuth access token
// (as opposed to a plain API key), which triggers Claude Code impersonation
// mode: Bearer auth, the identity system block, impersonation beta headers,
// the claude-cli user-agent, and tool-name remapping. Ports isOAuthToken from
// anthropic-messages.ts.
func isOAuthToken(apiKey string) bool {
	return strings.Contains(apiKey, "sk-ant-oat")
}

// toClaudeCodeName maps name to Claude Code's canonical casing when it
// matches one of CC's tools case-insensitively; other names pass through
// unchanged. Ports toClaudeCodeName from anthropic-messages.ts.
func toClaudeCodeName(name string) string {
	if cc, ok := ccToolLookup[strings.ToLower(name)]; ok {
		return cc
	}
	return name
}

// fromClaudeCodeName reverses toClaudeCodeName using the caller's own tool
// list: a case-insensitive match against tools returns that tool's original
// casing; no match returns name unchanged. This is a case-insensitive lookup,
// not a name-to-name mapping table, so it round-trips correctly regardless of
// what casing the caller originally used. Ports fromClaudeCodeName from
// anthropic-messages.ts.
func fromClaudeCodeName(name string, tools []ai.Tool) string {
	if len(tools) == 0 {
		return name
	}
	lower := strings.ToLower(name)
	for _, t := range tools {
		if strings.ToLower(t.Name) == lower {
			return t.Name
		}
	}
	return name
}

// messageEvents is the set of SSE event names carrying Anthropic message
// protocol payloads. Everything else (vendor/proxy events such as "done" or
// "proxy.stats") is ignored; "error" is special-cased before this check.
var messageEvents = map[string]bool{
	"message_start":       true,
	"message_delta":       true,
	"message_stop":        true,
	"content_block_start": true,
	"content_block_delta": true,
	"content_block_stop":  true,
}

// Stream implements ai.StreamFunc for the anthropic-messages wire protocol.
// It supports both API-key auth and OAuth (Claude Pro/Max) Claude Code
// impersonation mode, selected automatically from the shape of opts.APIKey
// (see isOAuthToken).
func Stream(ctx context.Context, model *ai.Model, chat ai.Context, opts *ai.StreamOptions) *ai.Stream {
	out := ai.NewStream()
	go run(ctx, out, model, chat, opts)
	return out
}

// StreamSimple implements ai.SimpleStreamFunc, mapping the abstract Reasoning
// level to an adaptive-thinking effort or a budget-based thinking config.
// Ports the streamSimple half of anthropic-messages.ts.
func StreamSimple(ctx context.Context, model *ai.Model, chat ai.Context, opts *ai.SimpleStreamOptions) *ai.Stream {
	apiKey := ""
	if opts != nil {
		apiKey = opts.APIKey
	}
	base := apis.BuildBaseOptions(model, chat, opts, apiKey)

	if opts == nil || opts.Reasoning == "" {
		off := false
		base.ThinkingEnabled = &off
		return Stream(ctx, model, chat, &base)
	}

	if forceAdaptiveThinking(model) {
		on := true
		base.ThinkingEnabled = &on
		base.Effort = mapThinkingLevelToEffort(model, opts.Reasoning)
		return Stream(ctx, model, chat, &base)
	}

	// Undefined means the caller did not request an output cap; let the
	// helper use the model cap. Do not coerce to 0, or the thinking budget
	// would become the entire max_tokens value.
	maxTokens, thinkingBudget := apis.AdjustMaxTokensForThinking(base.MaxTokens, model.MaxTokens, opts.Reasoning, opts.ThinkingBudgets)
	clamped := apis.ClampMaxTokensToContext(model, chat, maxTokens)
	base.MaxTokens = &clamped
	on := true
	base.ThinkingEnabled = &on
	budget := minInt(thinkingBudget, maxInt(0, clamped-1024))
	base.ThinkingBudgetTokens = &budget
	return Stream(ctx, model, chat, &base)
}

// mapThinkingLevelToEffort maps the abstract ThinkingLevel to an Anthropic
// adaptive-thinking effort, preferring an explicit model.ThinkingLevelMap
// override (needed for e.g. "xhigh", which otherwise falls back to "high").
func mapThinkingLevelToEffort(model *ai.Model, level ai.ThinkingLevel) ai.AnthropicEffort {
	if model.ThinkingLevelMap != nil {
		if mapped, ok := model.ThinkingLevelMap[level]; ok && mapped != nil {
			return ai.AnthropicEffort(*mapped)
		}
	}
	switch level {
	case ai.ThinkingMinimal, ai.ThinkingLow:
		return ai.AnthropicEffortLow
	case ai.ThinkingMedium:
		return ai.AnthropicEffortMedium
	case ai.ThinkingHigh:
		return ai.AnthropicEffortHigh
	default:
		return ai.AnthropicEffortHigh
	}
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
	isOAuth := isOAuthToken(apiKey)

	params := buildParams(model, chat, opts, isOAuth)
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

	url := strings.TrimRight(model.BaseURL, "/") + messagesPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		fail(err)
		return
	}
	for k, v := range buildHeaders(model, chat, opts, apiKey) {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	if opts != nil && opts.Timeout > 0 {
		client.Timeout = opts.Timeout
	}
	resp, err := client.Do(req)
	if err != nil {
		fail(err)
		return
	}
	defer resp.Body.Close()

	if opts != nil && opts.OnResponse != nil {
		respMeta := ai.ProviderResponse{Status: resp.StatusCode, Headers: ai.HeadersToRecord(resp.Header)}
		if err := opts.OnResponse(ctx, respMeta, model); err != nil {
			fail(err)
			return
		}
	}

	out.Push(ai.StartEvent{Partial: output.Clone()})

	if err := decodeEvents(out, output, model, resp.Body, isOAuth, chat.Tools); err != nil {
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

// assertRequestAuth mirrors upstream's assertRequestAuth: an empty apiKey is
// only acceptable when the caller supplied their own auth header.
func assertRequestAuth(provider, apiKey string, headers ai.ProviderHeaders) error {
	if apiKey != "" {
		return nil
	}
	if hasHeader(headers, "authorization") || hasHeader(headers, "x-api-key") || hasHeader(headers, "cf-aig-authorization") {
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

// --- block accumulation ------------------------------------------------------

// blockState tracks the scratch state needed to assemble one streaming
// content block; eventIndex mirrors Anthropic's content_block "index" field
// so deltas can be matched back to the right position in output.Content.
type blockState struct {
	kind        string // "text" | "toolCall"
	eventIndex  int
	partialJSON strings.Builder
}

func decodeEvents(out *ai.Stream, output *ai.AssistantMessage, model *ai.Model, body io.Reader, isOAuth bool, tools []ai.Tool) error {
	reader := sse.NewReader(body)
	var blocks []*blockState
	sawStart := false
	sawStop := false

	findBlock := func(index int) (int, *blockState) {
		for i, b := range blocks {
			if b != nil && b.eventIndex == index {
				return i, b
			}
		}
		return -1, nil
	}

	for {
		ev, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if ev.Event == "error" {
			return errors.New(ev.Data)
		}
		if !messageEvents[ev.Event] {
			continue
		}

		var probe struct {
			Type string `json:"type"`
		}
		if err := decodeWithRepair(ev.Data, &probe); err != nil {
			return fmt.Errorf("could not parse Anthropic SSE event %s: %w; data=%s", ev.Event, err, ev.Data)
		}

		switch probe.Type {
		case "message_start":
			sawStart = true
			var msg rawMessageStart
			if err := decodeWithRepair(ev.Data, &msg); err != nil {
				return fmt.Errorf("could not parse Anthropic SSE event %s: %w; data=%s", ev.Event, err, ev.Data)
			}
			output.ResponseID = msg.Message.ID
			applyUsage(&output.Usage, msg.Message.Usage)
			output.Usage.TotalTokens = output.Usage.Input + output.Usage.Output + output.Usage.CacheRead + output.Usage.CacheWrite
			ai.CalculateCost(model, &output.Usage)

		case "content_block_start":
			var blk rawContentBlockStart
			if err := decodeWithRepair(ev.Data, &blk); err != nil {
				return fmt.Errorf("could not parse Anthropic SSE event %s: %w; data=%s", ev.Event, err, ev.Data)
			}
			switch blk.ContentBlock.Type {
			case "text":
				output.Content = append(output.Content, ai.TextContent{})
				blocks = append(blocks, &blockState{kind: "text", eventIndex: blk.Index})
				idx := len(output.Content) - 1
				out.Push(ai.TextStartEvent{ContentIndex: idx, Partial: output.Clone()})
			case "thinking":
				output.Content = append(output.Content, ai.ThinkingContent{})
				blocks = append(blocks, &blockState{kind: "thinking", eventIndex: blk.Index})
				idx := len(output.Content) - 1
				out.Push(ai.ThinkingStartEvent{ContentIndex: idx, Partial: output.Clone()})
			case "redacted_thinking":
				output.Content = append(output.Content, ai.ThinkingContent{
					Thinking:          "[Reasoning redacted]",
					ThinkingSignature: blk.ContentBlock.Data,
					Redacted:          true,
				})
				blocks = append(blocks, &blockState{kind: "thinking", eventIndex: blk.Index})
				idx := len(output.Content) - 1
				out.Push(ai.ThinkingStartEvent{ContentIndex: idx, Partial: output.Clone()})
			case "tool_use":
				var input map[string]any
				if len(blk.ContentBlock.Input) > 0 {
					_ = json.Unmarshal(blk.ContentBlock.Input, &input)
				}
				if input == nil {
					input = map[string]any{}
				}
				name := blk.ContentBlock.Name
				if isOAuth {
					name = fromClaudeCodeName(name, tools)
				}
				output.Content = append(output.Content, ai.ToolCall{
					ID:        blk.ContentBlock.ID,
					Name:      name,
					Arguments: input,
				})
				blocks = append(blocks, &blockState{kind: "toolCall", eventIndex: blk.Index})
				idx := len(output.Content) - 1
				out.Push(ai.ToolCallStartEvent{ContentIndex: idx, Partial: output.Clone()})
			default:
				// unknown content_block type: ignored, matching upstream.
			}

		case "content_block_delta":
			var delta rawContentBlockDelta
			if err := decodeWithRepair(ev.Data, &delta); err != nil {
				return fmt.Errorf("could not parse Anthropic SSE event %s: %w; data=%s", ev.Event, err, ev.Data)
			}
			pos, b := findBlock(delta.Index)
			if b == nil {
				continue
			}
			switch delta.Delta.Type {
			case "text_delta":
				if b.kind != "text" {
					continue
				}
				cur := output.Content[pos].(ai.TextContent)
				cur.Text += delta.Delta.Text
				output.Content[pos] = cur
				out.Push(ai.TextDeltaEvent{ContentIndex: pos, Delta: delta.Delta.Text, Partial: output.Clone()})
			case "input_json_delta":
				if b.kind != "toolCall" {
					continue
				}
				b.partialJSON.WriteString(delta.Delta.PartialJSON)
				cur := output.Content[pos].(ai.ToolCall)
				cur.Arguments = partialjson.ParseStreamingObject(b.partialJSON.String())
				output.Content[pos] = cur
				out.Push(ai.ToolCallDeltaEvent{ContentIndex: pos, Delta: delta.Delta.PartialJSON, Partial: output.Clone()})
			case "thinking_delta":
				if b.kind != "thinking" {
					continue
				}
				cur := output.Content[pos].(ai.ThinkingContent)
				cur.Thinking += delta.Delta.Thinking
				output.Content[pos] = cur
				out.Push(ai.ThinkingDeltaEvent{ContentIndex: pos, Delta: delta.Delta.Thinking, Partial: output.Clone()})
			case "signature_delta":
				// No stream event: signature accrual is internal bookkeeping
				// for multi-turn replay, matching upstream (which pushes no
				// event for signature_delta).
				if b.kind != "thinking" {
					continue
				}
				cur := output.Content[pos].(ai.ThinkingContent)
				cur.ThinkingSignature += delta.Delta.Signature
				output.Content[pos] = cur
			default:
				// unknown delta type: ignored, matching upstream.
			}

		case "content_block_stop":
			var stop rawContentBlockStop
			if err := decodeWithRepair(ev.Data, &stop); err != nil {
				return fmt.Errorf("could not parse Anthropic SSE event %s: %w; data=%s", ev.Event, err, ev.Data)
			}
			pos, b := findBlock(stop.Index)
			if b == nil {
				continue
			}
			switch b.kind {
			case "text":
				cur := output.Content[pos].(ai.TextContent)
				out.Push(ai.TextEndEvent{ContentIndex: pos, Content: cur.Text, Partial: output.Clone()})
			case "toolCall":
				cur := output.Content[pos].(ai.ToolCall)
				cur.Arguments = partialjson.ParseStreamingObject(b.partialJSON.String())
				output.Content[pos] = cur
				out.Push(ai.ToolCallEndEvent{ContentIndex: pos, ToolCall: cur, Partial: output.Clone()})
			case "thinking":
				cur := output.Content[pos].(ai.ThinkingContent)
				out.Push(ai.ThinkingEndEvent{ContentIndex: pos, Content: cur.Thinking, Partial: output.Clone()})
			}

		case "message_delta":
			var delta rawMessageDelta
			if err := decodeWithRepair(ev.Data, &delta); err != nil {
				return fmt.Errorf("could not parse Anthropic SSE event %s: %w; data=%s", ev.Event, err, ev.Data)
			}
			if delta.Delta.StopReason != nil {
				reason, errMsg, err := mapStopReason(*delta.Delta.StopReason, delta.Delta.StopDetails)
				if err != nil {
					return err
				}
				output.StopReason = reason
				if errMsg != "" {
					output.ErrorMessage = errMsg
				}
			}
			applyUsageDelta(&output.Usage, delta.Usage)
			output.Usage.TotalTokens = output.Usage.Input + output.Usage.Output + output.Usage.CacheRead + output.Usage.CacheWrite
			ai.CalculateCost(model, &output.Usage)

		case "message_stop":
			sawStop = true
		}
	}

	if sawStart && !sawStop {
		return errors.New("Anthropic stream ended before message_stop")
	}
	return nil
}

// applyUsage seeds usage from message_start. CacheWrite1h is always set here
// (defaulting to 0), matching upstream's `|| 0`: message_delta never reports
// the cache_creation breakdown, so this is the only place it is populated.
func applyUsage(usage *ai.Usage, raw rawUsage) {
	usage.Input = intOr0(raw.InputTokens)
	usage.Output = intOr0(raw.OutputTokens)
	usage.CacheRead = intOr0(raw.CacheReadInputTokens)
	usage.CacheWrite = intOr0(raw.CacheCreationInputTokens)
	oneHour := 0
	if raw.CacheCreation != nil {
		oneHour = intOr0(raw.CacheCreation.Ephemeral1hInputTokens)
	}
	usage.CacheWrite1h = &oneHour
}

// applyUsageDelta only overwrites fields the server actually sent (non-null),
// preserving input_tokens et al. from message_start when a proxy omits them
// on message_delta.
func applyUsageDelta(usage *ai.Usage, raw rawUsage) {
	if raw.InputTokens != nil {
		usage.Input = *raw.InputTokens
	}
	if raw.OutputTokens != nil {
		usage.Output = *raw.OutputTokens
	}
	if raw.CacheReadInputTokens != nil {
		usage.CacheRead = *raw.CacheReadInputTokens
	}
	if raw.CacheCreationInputTokens != nil {
		usage.CacheWrite = *raw.CacheCreationInputTokens
	}
}

func intOr0(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// --- wire event decoding ------------------------------------------------------

type rawCacheCreation struct {
	Ephemeral1hInputTokens *int `json:"ephemeral_1h_input_tokens"`
}

type rawUsage struct {
	InputTokens              *int              `json:"input_tokens"`
	OutputTokens             *int              `json:"output_tokens"`
	CacheReadInputTokens     *int              `json:"cache_read_input_tokens"`
	CacheCreationInputTokens *int              `json:"cache_creation_input_tokens"`
	CacheCreation            *rawCacheCreation `json:"cache_creation"`
}

type rawMessageStart struct {
	Message struct {
		ID    string   `json:"id"`
		Usage rawUsage `json:"usage"`
	} `json:"message"`
}

type rawContentBlockStart struct {
	Index        int `json:"index"`
	ContentBlock struct {
		Type  string          `json:"type"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
		Data  string          `json:"data"` // redacted_thinking opaque payload
	} `json:"content_block"`
}

type rawContentBlockDelta struct {
	Index int `json:"index"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
		Thinking    string `json:"thinking"`
		Signature   string `json:"signature"`
	} `json:"delta"`
}

type rawContentBlockStop struct {
	Index int `json:"index"`
}

type rawStopDetails struct {
	Explanation string `json:"explanation"`
}

type rawMessageDelta struct {
	Delta struct {
		StopReason  *string         `json:"stop_reason"`
		StopDetails *rawStopDetails `json:"stop_details"`
	} `json:"delta"`
	Usage rawUsage `json:"usage"`
}

// decodeWithRepair parses data into v, retrying once with partialjson.Repair
// applied when the strict parse fails. Ports parseJsonWithRepair from
// json-parse.ts.
func decodeWithRepair(data string, v any) error {
	if err := json.Unmarshal([]byte(data), v); err == nil {
		return nil
	} else if repaired := partialjson.Repair(data); repaired != data {
		if err2 := json.Unmarshal([]byte(repaired), v); err2 == nil {
			return nil
		}
		return err
	} else {
		return err
	}
}

func mapStopReason(reason string, details *rawStopDetails) (ai.StopReason, string, error) {
	switch reason {
	case "end_turn":
		return ai.StopReasonStop, "", nil
	case "max_tokens":
		return ai.StopReasonLength, "", nil
	case "tool_use":
		return ai.StopReasonToolUse, "", nil
	case "refusal":
		msg := "The model refused to complete the request"
		if details != nil && details.Explanation != "" {
			msg = details.Explanation
		}
		return ai.StopReasonError, msg, nil
	case "pause_turn":
		return ai.StopReasonStop, "", nil
	case "stop_sequence":
		return ai.StopReasonStop, "", nil
	case "sensitive":
		return ai.StopReasonError, "", nil
	default:
		return "", "", fmt.Errorf("Unhandled stop reason: %s", reason)
	}
}

// --- request building ---------------------------------------------------------

// wireRequest is the Anthropic Messages API streaming request body: the core
// subset plus adaptive thinking/cache_control and the OAuth identity system
// block (carried in System like any other block).
type wireRequest struct {
	Model        string            `json:"model"`
	Messages     []wireMessage     `json:"messages"`
	MaxTokens    int               `json:"max_tokens"`
	Stream       bool              `json:"stream"`
	System       []wireTextBlock   `json:"system,omitempty"`
	Temperature  *float64          `json:"temperature,omitempty"`
	Tools        []wireTool        `json:"tools,omitempty"`
	Thinking     *wireThinking     `json:"thinking,omitempty"`
	OutputConfig *wireOutputConfig `json:"output_config,omitempty"`
}

type wireTextBlock struct {
	Type         string            `json:"type"`
	Text         string            `json:"text"`
	CacheControl *wireCacheControl `json:"cache_control,omitempty"`
}

type wireMessage struct {
	Role string `json:"role"`
	// Content is a string (plain user text) or []map[string]any (blocks).
	Content any `json:"content"`
}

type wireTool struct {
	Name                string            `json:"name"`
	Description         string            `json:"description"`
	EagerInputStreaming bool              `json:"eager_input_streaming,omitempty"`
	InputSchema         wireInputSchema   `json:"input_schema"`
	CacheControl        *wireCacheControl `json:"cache_control,omitempty"`
}

type wireInputSchema struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	Required   []string       `json:"required,omitempty"`
}

// wireCacheControl is an Anthropic ephemeral cache_control breakpoint. TTL is
// "1h" for long retention on models that support it, or omitted for the
// default (5m) retention.
type wireCacheControl struct {
	Type string `json:"type"`
	TTL  string `json:"ttl,omitempty"`
}

// wireThinking is the Anthropic `thinking` request field: "adaptive" (model
// decides), "enabled" (budget-based), or "disabled".
type wireThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
	Display      string `json:"display,omitempty"`
}

// wireOutputConfig carries the adaptive-thinking effort level.
type wireOutputConfig struct {
	Effort string `json:"effort,omitempty"`
}

func buildParams(model *ai.Model, chat ai.Context, opts *ai.StreamOptions, isOAuth bool) *wireRequest {
	maxTokens := model.MaxTokens
	if opts != nil && opts.MaxTokens != nil {
		maxTokens = *opts.MaxTokens
	}

	var cacheRetention ai.CacheRetention
	var env ai.ProviderEnv
	if opts != nil {
		cacheRetention = opts.CacheRetention
		env = opts.Env
	}
	cacheControl := getCacheControl(model, cacheRetention, env)

	req := &wireRequest{
		Model:     model.ID,
		Messages:  convertMessages(chat.Messages, model, cacheControl, isOAuth),
		MaxTokens: maxTokens,
		Stream:    true,
	}
	// For OAuth tokens, Anthropic requires the Claude Code identity block as
	// the first system block; the caller's own system prompt (if any) follows
	// as a second block. Both share the same cache_control breakpoint. Ports
	// the isOAuthToken branch of buildParams in anthropic-messages.ts.
	if isOAuth {
		req.System = []wireTextBlock{{Type: "text", Text: claudeCodeIdentitySystemBlock, CacheControl: cacheControl}}
		if chat.SystemPrompt != "" {
			req.System = append(req.System, wireTextBlock{Type: "text", Text: ai.SanitizeSurrogates(chat.SystemPrompt), CacheControl: cacheControl})
		}
	} else if chat.SystemPrompt != "" {
		req.System = []wireTextBlock{{Type: "text", Text: ai.SanitizeSurrogates(chat.SystemPrompt), CacheControl: cacheControl}}
	}

	thinkingEnabled := opts != nil && opts.ThinkingEnabled != nil && *opts.ThinkingEnabled
	if opts != nil && opts.Temperature != nil && !thinkingEnabled && supportsTemperature(model) {
		t := *opts.Temperature
		req.Temperature = &t
	}
	if len(chat.Tools) > 0 {
		req.Tools = convertTools(chat.Tools, model, cacheControl, isOAuth)
	}

	// Configure thinking mode: adaptive, budget-based, or explicitly disabled.
	if model.Reasoning && opts != nil {
		if thinkingEnabled {
			display := string(ai.AnthropicThinkingSummarized)
			if opts.ThinkingDisplay != "" {
				display = string(opts.ThinkingDisplay)
			}
			if forceAdaptiveThinking(model) {
				req.Thinking = &wireThinking{Type: "adaptive", Display: display}
				if opts.Effort != "" {
					req.OutputConfig = &wireOutputConfig{Effort: string(opts.Effort)}
				}
			} else {
				budget := 1024
				if opts.ThinkingBudgetTokens != nil {
					budget = *opts.ThinkingBudgetTokens
				}
				req.Thinking = &wireThinking{Type: "enabled", BudgetTokens: budget, Display: display}
			}
		} else if opts.ThinkingEnabled != nil && !*opts.ThinkingEnabled && sendDisabledThinking(model) {
			req.Thinking = &wireThinking{Type: "disabled"}
		}
	}

	return req
}

func supportsTemperature(model *ai.Model) bool {
	if model.Compat == nil || model.Compat.SupportsTemperature == nil {
		return true
	}
	return *model.Compat.SupportsTemperature
}

// forceAdaptiveThinking reports whether model.Compat.ForceAdaptiveThinking is
// explicitly true (an override; the default is model-catalog driven upstream,
// which this Go port does not yet embed — epic 11).
func forceAdaptiveThinking(model *ai.Model) bool {
	return model.Compat != nil && model.Compat.ForceAdaptiveThinking != nil && *model.Compat.ForceAdaptiveThinking
}

// sendDisabledThinking reports whether thinking.type=disabled should be sent
// when thinking is explicitly turned off. A model.ThinkingLevelMap["off"]
// entry present but nil (TS null) marks the level unsupported, so some models
// (e.g. Claude Fable 5) must omit the field entirely rather than send it.
func sendDisabledThinking(model *ai.Model) bool {
	if model.ThinkingLevelMap == nil {
		return true
	}
	mapped, present := model.ThinkingLevelMap[ai.ThinkingOff]
	return !(present && mapped == nil)
}

// --- cache_control -----------------------------------------------------------

// resolveCacheRetention resolves the caller's preference against
// PI_CACHE_RETENTION (checked in env first, then the process environment),
// defaulting to "short". Ports resolveCacheRetention from
// anthropic-messages.ts.
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
// falling back to the process environment. The Bun sandbox /proc/self/environ
// fallback from provider-env.ts is an intentional deviation (see PORTING.md);
// Go reads the process environment directly.
func providerEnvValue(name string, env ai.ProviderEnv) string {
	if v, ok := env[name]; ok && v != "" {
		return v
	}
	return os.Getenv(name)
}

// getCacheControl builds the cache_control breakpoint to apply across the
// request (system prompt, last tool, last user-message block), or nil when
// retention is "none". Ports getCacheControl from anthropic-messages.ts.
func getCacheControl(model *ai.Model, retention ai.CacheRetention, env ai.ProviderEnv) *wireCacheControl {
	resolved := resolveCacheRetention(retention, env)
	if resolved == ai.CacheRetentionNone {
		return nil
	}
	cc := &wireCacheControl{Type: "ephemeral"}
	if resolved == ai.CacheRetentionLong && supportsLongCacheRetention(model) {
		cc.TTL = "1h"
	}
	return cc
}

func supportsLongCacheRetention(model *ai.Model) bool {
	if model.Compat == nil || model.Compat.SupportsLongCacheRetention == nil {
		return true
	}
	return *model.Compat.SupportsLongCacheRetention
}

func supportsCacheControlOnTools(model *ai.Model) bool {
	if model.Compat == nil || model.Compat.SupportsCacheControlOnTools == nil {
		return true
	}
	return *model.Compat.SupportsCacheControlOnTools
}

// allowEmptySignature reports whether model.Compat.AllowEmptySignature is
// explicitly true (default false, matching upstream).
func allowEmptySignature(model *ai.Model) bool {
	return model.Compat != nil && model.Compat.AllowEmptySignature != nil && *model.Compat.AllowEmptySignature
}

func supportsEagerToolInputStreaming(model *ai.Model) bool {
	if model.Compat == nil || model.Compat.SupportsEagerToolInputStreaming == nil {
		return true
	}
	return *model.Compat.SupportsEagerToolInputStreaming
}

func convertTools(tools []ai.Tool, model *ai.Model, cacheControl *wireCacheControl, isOAuth bool) []wireTool {
	eager := supportsEagerToolInputStreaming(model)
	applyCacheControl := cacheControl != nil && supportsCacheControlOnTools(model)
	out := make([]wireTool, 0, len(tools))
	for i, t := range tools {
		var schema struct {
			Properties map[string]any `json:"properties"`
			Required   []string       `json:"required"`
		}
		if len(t.Parameters) > 0 {
			_ = json.Unmarshal(t.Parameters, &schema)
		}
		props := schema.Properties
		if props == nil {
			props = map[string]any{}
		}
		name := t.Name
		if isOAuth {
			name = toClaudeCodeName(name)
		}
		tool := wireTool{
			Name:                name,
			Description:         t.Description,
			EagerInputStreaming: eager,
			InputSchema:         wireInputSchema{Type: "object", Properties: props, Required: schema.Required},
		}
		if applyCacheControl && i == len(tools)-1 {
			tool.CacheControl = cacheControl
		}
		out = append(out, tool)
	}
	return out
}

// normalizeToolCallID adapts a tool-call id for Anthropic's
// ^[a-zA-Z0-9_-]+$, max-64-char requirement when replaying cross-model.
func normalizeToolCallID(id string, _ *ai.Model, _ *ai.AssistantMessage) string {
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	out := b.String()
	if len(out) > 64 {
		out = out[:64]
	}
	return out
}

func convertMessages(messages []ai.Message, model *ai.Model, cacheControl *wireCacheControl, isOAuth bool) []wireMessage {
	transformed := apis.TransformMessages(messages, model, normalizeToolCallID)
	out := make([]wireMessage, 0, len(transformed))
	for i := 0; i < len(transformed); i++ {
		switch m := transformed[i].(type) {
		case ai.UserMessage:
			if m.Content.Plain != nil {
				text := ai.SanitizeSurrogates(*m.Content.Plain)
				if strings.TrimSpace(text) == "" {
					continue
				}
				out = append(out, wireMessage{Role: "user", Content: text})
				continue
			}
			blocks := convertUserBlocks(m.Content.Blocks)
			if len(blocks) == 0 {
				continue
			}
			out = append(out, wireMessage{Role: "user", Content: blocks})

		case *ai.AssistantMessage:
			blocks := convertAssistantBlocks(m.Content, model, isOAuth)
			if len(blocks) == 0 {
				continue
			}
			out = append(out, wireMessage{Role: "assistant", Content: blocks})

		case ai.ToolResultMessage:
			results := []map[string]any{toolResultBlock(m)}
			j := i + 1
			for j < len(transformed) {
				next, ok := transformed[j].(ai.ToolResultMessage)
				if !ok {
					break
				}
				results = append(results, toolResultBlock(next))
				j++
			}
			i = j - 1
			out = append(out, wireMessage{Role: "user", Content: results})
		}
	}
	applyCacheControlToLastUserMessage(out, cacheControl)
	return out
}

// applyCacheControlToLastUserMessage caches conversation history by adding
// cache_control to the last block of the last message, if it is a user turn.
// A plain-string message is upgraded to a one-block array so the breakpoint
// has somewhere to attach. Ports the trailing block of convertMessages in
// anthropic-messages.ts.
func applyCacheControlToLastUserMessage(messages []wireMessage, cacheControl *wireCacheControl) {
	if cacheControl == nil || len(messages) == 0 {
		return
	}
	last := &messages[len(messages)-1]
	if last.Role != "user" {
		return
	}
	switch content := last.Content.(type) {
	case string:
		last.Content = []map[string]any{{"type": "text", "text": content, "cache_control": cacheControl}}
	case []map[string]any:
		if len(content) == 0 {
			return
		}
		lastBlock := content[len(content)-1]
		switch lastBlock["type"] {
		case "text", "image", "tool_result":
			lastBlock["cache_control"] = cacheControl
		}
	}
}

func convertUserBlocks(blocks []ai.UserContentPart) []map[string]any {
	var out []map[string]any
	for _, block := range blocks {
		switch b := block.(type) {
		case ai.TextContent:
			text := ai.SanitizeSurrogates(b.Text)
			if strings.TrimSpace(text) == "" {
				continue
			}
			out = append(out, map[string]any{"type": "text", "text": text})
		case ai.ImageContent:
			out = append(out, imageBlock(b))
		}
	}
	return out
}

func convertAssistantBlocks(content []ai.AssistantContentPart, model *ai.Model, isOAuth bool) []map[string]any {
	var out []map[string]any
	for _, block := range content {
		switch b := block.(type) {
		case ai.TextContent:
			text := ai.SanitizeSurrogates(b.Text)
			if strings.TrimSpace(text) == "" {
				continue
			}
			out = append(out, map[string]any{"type": "text", "text": text})

		case ai.ThinkingContent:
			if b.Redacted {
				out = append(out, map[string]any{"type": "redacted_thinking", "data": b.ThinkingSignature})
				continue
			}
			if strings.TrimSpace(b.Thinking) == "" {
				continue
			}
			if strings.TrimSpace(b.ThinkingSignature) == "" {
				// No signature to replay: some compatible providers accept
				// (and expect) an empty signature on marked models, so
				// compat.AllowEmptySignature preserves the thinking block;
				// otherwise downgrade to plain text (Anthropic's own default).
				if allowEmptySignature(model) {
					out = append(out, map[string]any{
						"type":      "thinking",
						"thinking":  ai.SanitizeSurrogates(b.Thinking),
						"signature": "",
					})
					continue
				}
				out = append(out, map[string]any{"type": "text", "text": ai.SanitizeSurrogates(b.Thinking)})
				continue
			}
			out = append(out, map[string]any{
				"type":      "thinking",
				"thinking":  ai.SanitizeSurrogates(b.Thinking),
				"signature": b.ThinkingSignature,
			})

		case ai.ToolCall:
			args := b.Arguments
			if args == nil {
				args = map[string]any{}
			}
			name := b.Name
			if isOAuth {
				name = toClaudeCodeName(name)
			}
			out = append(out, map[string]any{"type": "tool_use", "id": b.ID, "name": name, "input": args})
		}
	}
	return out
}

func imageBlock(b ai.ImageContent) map[string]any {
	return map[string]any{
		"type": "image",
		"source": map[string]any{
			"type":       "base64",
			"media_type": b.MimeType,
			"data":       b.Data,
		},
	}
}

func toolResultBlock(m ai.ToolResultMessage) map[string]any {
	return map[string]any{
		"type":        "tool_result",
		"tool_use_id": m.ToolCallID,
		"content":     convertToolResultContent(m.Content),
		"is_error":    m.IsError,
	}
}

func convertToolResultContent(content []ai.UserContentPart) any {
	hasImages := false
	for _, c := range content {
		if _, ok := c.(ai.ImageContent); ok {
			hasImages = true
			break
		}
	}
	if !hasImages {
		var texts []string
		for _, c := range content {
			if t, ok := c.(ai.TextContent); ok {
				texts = append(texts, t.Text)
			}
		}
		return ai.SanitizeSurrogates(strings.Join(texts, "\n"))
	}

	var blocks []map[string]any
	for _, c := range content {
		switch b := c.(type) {
		case ai.TextContent:
			blocks = append(blocks, map[string]any{"type": "text", "text": ai.SanitizeSurrogates(b.Text)})
		case ai.ImageContent:
			blocks = append(blocks, imageBlock(b))
		}
	}
	hasText := false
	for _, blk := range blocks {
		if blk["type"] == "text" {
			hasText = true
			break
		}
	}
	if !hasText {
		blocks = append([]map[string]any{{"type": "text", "text": "(see attached image)"}}, blocks...)
	}
	return blocks
}

// --- headers -------------------------------------------------------------

func buildHeaders(model *ai.Model, chat ai.Context, opts *ai.StreamOptions, apiKey string) map[string]string {
	defaults := map[string]string{
		"content-type":      "application/json",
		"accept":            "text/event-stream",
		"anthropic-version": anthropicVersion,
	}

	var betas []string
	if needsFineGrainedToolStreamingBeta(model, chat) {
		betas = append(betas, fineGrainedToolStreamingBeta)
	}

	if isOAuthToken(apiKey) {
		// OAuth (Claude Pro/Max): Bearer auth plus the Claude Code identity
		// headers Anthropic requires to authorize impersonation. Ports the
		// OAuth branch of createClient in anthropic-messages.ts.
		defaults["authorization"] = "Bearer " + apiKey
		defaults["anthropic-dangerous-direct-browser-access"] = "true"
		defaults["user-agent"] = "claude-cli/" + claudeCodeVersion
		defaults["x-app"] = "cli"
		betas = append([]string{claudeCodeBeta, oauthBeta}, betas...)
	} else if apiKey != "" {
		defaults["x-api-key"] = apiKey
	}

	if len(betas) > 0 {
		defaults["anthropic-beta"] = strings.Join(betas, ",")
	}

	for k, v := range model.Headers {
		defaults[k] = v
	}
	var optHeaders ai.ProviderHeaders
	if opts != nil {
		optHeaders = opts.Headers
	}
	return ai.MergeProviderHeaders(defaults, optHeaders)
}

func needsFineGrainedToolStreamingBeta(model *ai.Model, chat ai.Context) bool {
	return len(chat.Tools) > 0 && !supportsEagerToolInputStreaming(model)
}
