// Package anthropic implements the core of the anthropic-messages wire
// adapter: request building (messages, system, tools, sampling params) and
// SSE stream decoding into the unified event protocol, over raw net/http and
// the hand-rolled SSE parser from ai/internal/sse.
//
// Out of scope (follow-up issues in epic 5, layered on top of this core):
//   - Adaptive thinking and cache_control — epic 5 issue 02.
//   - OAuth Claude Code impersonation — epic 5 issue 03.
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
)

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

// Stream implements ai.StreamFunc for the anthropic-messages wire protocol
// using API-key auth only (OAuth impersonation is epic 5 issue 03).
func Stream(ctx context.Context, model *ai.Model, chat ai.Context, opts *ai.StreamOptions) *ai.Stream {
	out := ai.NewStream()
	go run(ctx, out, model, chat, opts)
	return out
}

// StreamSimple implements ai.SimpleStreamFunc. Reasoning-level mapping
// (adaptive thinking / budget-based thinking) lands in epic 5 issue 02; until
// then this clamps maxTokens to the context window and delegates to Stream,
// ignoring SimpleStreamOptions.Reasoning.
func StreamSimple(ctx context.Context, model *ai.Model, chat ai.Context, opts *ai.SimpleStreamOptions) *ai.Stream {
	apiKey := ""
	if opts != nil {
		apiKey = opts.APIKey
	}
	base := apis.BuildBaseOptions(model, chat, opts, apiKey)
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

	if err := decodeEvents(out, output, model, resp.Body); err != nil {
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

func decodeEvents(out *ai.Stream, output *ai.AssistantMessage, model *ai.Model, body io.Reader) error {
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
			case "tool_use":
				var input map[string]any
				if len(blk.ContentBlock.Input) > 0 {
					_ = json.Unmarshal(blk.ContentBlock.Input, &input)
				}
				if input == nil {
					input = map[string]any{}
				}
				output.Content = append(output.Content, ai.ToolCall{
					ID:        blk.ContentBlock.ID,
					Name:      blk.ContentBlock.Name,
					Arguments: input,
				})
				blocks = append(blocks, &blockState{kind: "toolCall", eventIndex: blk.Index})
				idx := len(output.Content) - 1
				out.Push(ai.ToolCallStartEvent{ContentIndex: idx, Partial: output.Clone()})
			default:
				// "thinking" / "redacted_thinking": deferred to epic 5 issue 02.
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
			default:
				// "thinking_delta" / "signature_delta": deferred to issue 02.
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

func applyUsage(usage *ai.Usage, raw rawUsage) {
	usage.Input = intOr0(raw.InputTokens)
	usage.Output = intOr0(raw.OutputTokens)
	usage.CacheRead = intOr0(raw.CacheReadInputTokens)
	usage.CacheWrite = intOr0(raw.CacheCreationInputTokens)
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

type rawUsage struct {
	InputTokens              *int `json:"input_tokens"`
	OutputTokens             *int `json:"output_tokens"`
	CacheReadInputTokens     *int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens *int `json:"cache_creation_input_tokens"`
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
	} `json:"content_block"`
}

type rawContentBlockDelta struct {
	Index int `json:"index"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
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

// wireRequest is the Anthropic Messages API streaming request body (the core
// subset: no thinking, cache_control, or OAuth identity fields).
type wireRequest struct {
	Model       string          `json:"model"`
	Messages    []wireMessage   `json:"messages"`
	MaxTokens   int             `json:"max_tokens"`
	Stream      bool            `json:"stream"`
	System      []wireTextBlock `json:"system,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	Tools       []wireTool      `json:"tools,omitempty"`
}

type wireTextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type wireMessage struct {
	Role string `json:"role"`
	// Content is a string (plain user text) or []map[string]any (blocks).
	Content any `json:"content"`
}

type wireTool struct {
	Name                string          `json:"name"`
	Description         string          `json:"description"`
	EagerInputStreaming bool            `json:"eager_input_streaming,omitempty"`
	InputSchema         wireInputSchema `json:"input_schema"`
}

type wireInputSchema struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	Required   []string       `json:"required,omitempty"`
}

func buildParams(model *ai.Model, chat ai.Context, opts *ai.StreamOptions) *wireRequest {
	maxTokens := model.MaxTokens
	if opts != nil && opts.MaxTokens != nil {
		maxTokens = *opts.MaxTokens
	}
	req := &wireRequest{
		Model:     model.ID,
		Messages:  convertMessages(chat.Messages, model),
		MaxTokens: maxTokens,
		Stream:    true,
	}
	if chat.SystemPrompt != "" {
		req.System = []wireTextBlock{{Type: "text", Text: ai.SanitizeSurrogates(chat.SystemPrompt)}}
	}
	if opts != nil && opts.Temperature != nil && supportsTemperature(model) {
		t := *opts.Temperature
		req.Temperature = &t
	}
	if len(chat.Tools) > 0 {
		req.Tools = convertTools(chat.Tools, model)
	}
	return req
}

func supportsTemperature(model *ai.Model) bool {
	if model.Compat == nil || model.Compat.SupportsTemperature == nil {
		return true
	}
	return *model.Compat.SupportsTemperature
}

func supportsEagerToolInputStreaming(model *ai.Model) bool {
	if model.Compat == nil || model.Compat.SupportsEagerToolInputStreaming == nil {
		return true
	}
	return *model.Compat.SupportsEagerToolInputStreaming
}

func convertTools(tools []ai.Tool, model *ai.Model) []wireTool {
	eager := supportsEagerToolInputStreaming(model)
	out := make([]wireTool, 0, len(tools))
	for _, t := range tools {
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
		out = append(out, wireTool{
			Name:                t.Name,
			Description:         t.Description,
			EagerInputStreaming: eager,
			InputSchema:         wireInputSchema{Type: "object", Properties: props, Required: schema.Required},
		})
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

func convertMessages(messages []ai.Message, model *ai.Model) []wireMessage {
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
			blocks := convertAssistantBlocks(m.Content)
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
	return out
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

func convertAssistantBlocks(content []ai.AssistantContentPart) []map[string]any {
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
			if b.ThinkingSignature == "" {
				// No signature to replay: downgrade to plain text, matching
				// upstream's default (allowEmptySignature compat is issue 02).
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
			out = append(out, map[string]any{"type": "tool_use", "id": b.ID, "name": b.Name, "input": args})
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
	if apiKey != "" {
		defaults["x-api-key"] = apiKey
	}
	if needsFineGrainedToolStreamingBeta(model, chat) {
		defaults["anthropic-beta"] = fineGrainedToolStreamingBeta
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
