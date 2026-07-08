// Package openaicompletions implements the core of the openai-completions
// wire adapter: chat-completions request building and SSE stream decoding
// into the unified event protocol, over raw net/http and the hand-rolled SSE
// parser from ai/internal/sse.
//
// This issue (epic 6, issue 01) ports only the base adapter: request/message
// building, dual-map (index and id) tool-call delta correlation, partial
// tool-arg JSON re-parsing, and prompt/cache usage math. The vendor compat
// auto-detection matrix, thinking-format encodings, and
// cache_control/session-affinity/live-smoke wiring are follow-up issues (see
// docs/epics/epic-6-openai-completions/issues/02-04); this adapter reads
// explicit ai.Model.Compat overrides directly (no baseUrl/provider sniffing),
// matching the same incremental pattern the anthropic adapter used.
//
// Ports: packages/ai/src/api/openai-completions.ts (base subset)
package openaicompletions

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

const chatCompletionsPath = "/chat/completions"

// Stream implements ai.StreamFunc for the openai-completions wire protocol.
func Stream(ctx context.Context, model *ai.Model, chat ai.Context, opts *ai.StreamOptions) *ai.Stream {
	out := ai.NewStream()
	go run(ctx, out, model, chat, opts)
	return out
}

// StreamSimple implements ai.SimpleStreamFunc. Reasoning-level translation
// (thinking-format encodings) lands in issue 03; for now this forwards the
// base stream options only.
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

	url := strings.TrimRight(model.BaseURL, "/") + chatCompletionsPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		fail(err)
		return
	}
	for k, v := range buildHeaders(model, opts, apiKey) {
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fail(httpStatusError(resp))
		return
	}

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

// httpStatusError composes an error for a non-2xx response, matching the
// anthropic adapter's approach: see its httpStatusError doc comment for why
// this reconstructs the SDK-shaped "<status> <body>" text inline rather than
// through a shared ai/internal/httpx normalizer (deferred to whichever
// later issue first needs multi-SDK error-shape probing).
func httpStatusError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return fmt.Errorf("%d status code (no body)", resp.StatusCode)
	}
	var compact bytes.Buffer
	if json.Compact(&compact, trimmed) == nil {
		return fmt.Errorf("%d %s", resp.StatusCode, compact.String())
	}
	return fmt.Errorf("%d %s", resp.StatusCode, string(trimmed))
}

// --- request building ---------------------------------------------------------

// wireRequest is the OpenAI chat-completions streaming request body: the base
// subset (no cache_control, thinking-format, or compat-matrix fields yet).
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
}

type wireStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type wireTool struct {
	Type     string       `json:"type"`
	Function wireFunction `json:"function"`
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
}

type wireContentPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *wireImageURL `json:"image_url,omitempty"`
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

func maxTokensField(model *ai.Model) string {
	if model.Compat != nil && model.Compat.MaxTokensField != "" {
		return model.Compat.MaxTokensField
	}
	return "max_completion_tokens"
}

func supportsUsageInStreaming(model *ai.Model) bool {
	if model.Compat == nil || model.Compat.SupportsUsageInStreaming == nil {
		return true
	}
	return *model.Compat.SupportsUsageInStreaming
}

func supportsStore(model *ai.Model) bool {
	if model.Compat == nil || model.Compat.SupportsStore == nil {
		return true
	}
	return *model.Compat.SupportsStore
}

func supportsStrictMode(model *ai.Model) bool {
	if model.Compat == nil || model.Compat.SupportsStrictMode == nil {
		return true
	}
	return *model.Compat.SupportsStrictMode
}

func supportsDeveloperRole(model *ai.Model) bool {
	if model.Compat == nil || model.Compat.SupportsDeveloperRole == nil {
		return true
	}
	return *model.Compat.SupportsDeveloperRole
}

func buildParams(model *ai.Model, chat ai.Context, opts *ai.StreamOptions) *wireRequest {
	req := &wireRequest{
		Model:    model.ID,
		Messages: convertMessages(chat, model),
		Stream:   true,
	}

	if supportsUsageInStreaming(model) {
		req.StreamOptions = &wireStreamOptions{IncludeUsage: true}
	}
	if supportsStore(model) {
		req.Store = boolPtr(false)
	}

	maxTokens := model.MaxTokens
	if opts != nil && opts.MaxTokens != nil {
		maxTokens = *opts.MaxTokens
	}
	if maxTokens > 0 {
		if maxTokensField(model) == "max_tokens" {
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
		tools := convertTools(chat.Tools, model)
		req.Tools = &tools
	} else if hasToolHistory(chat.Messages) {
		empty := []wireTool{}
		req.Tools = &empty
	}

	return req
}

func convertTools(tools []ai.Tool, model *ai.Model) []wireTool {
	strict := supportsStrictMode(model)
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

func convertMessages(chat ai.Context, model *ai.Model) []wireMessage {
	transformed := apis.TransformMessages(chat.Messages, model, normalizeToolCallID)
	out := make([]wireMessage, 0, len(transformed)+1)

	if chat.SystemPrompt != "" {
		role := "system"
		if model.Reasoning && supportsDeveloperRole(model) {
			role = "developer"
		}
		out = append(out, wireMessage{Role: role, Content: ai.SanitizeSurrogates(chat.SystemPrompt)})
	}

	for i := 0; i < len(transformed); i++ {
		msg := transformed[i]
		switch m := msg.(type) {
		case ai.UserMessage:
			out = append(out, convertUserMessage(m))

		case *ai.AssistantMessage:
			wireMsg, ok := convertAssistantMessage(m)
			if ok {
				out = append(out, wireMsg)
			}

		case ai.ToolResultMessage:
			j := i
			for j < len(transformed) {
				next, ok := transformed[j].(ai.ToolResultMessage)
				if !ok {
					break
				}
				out = append(out, convertToolResultMessage(next))
				j++
			}
			i = j - 1
		}
	}

	return out
}

func convertUserMessage(m ai.UserMessage) wireMessage {
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

func convertAssistantMessage(m *ai.AssistantMessage) (wireMessage, bool) {
	var textParts []string
	for _, block := range m.Content {
		if t, ok := block.(ai.TextContent); ok && strings.TrimSpace(t.Text) != "" {
			textParts = append(textParts, ai.SanitizeSurrogates(t.Text))
		}
	}
	text := strings.Join(textParts, "")

	var toolCalls []wireToolCall
	for _, block := range m.Content {
		if tc, ok := block.(ai.ToolCall); ok {
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
		}
	}

	wireMsg := wireMessage{Role: "assistant", ToolCalls: toolCalls}
	if text != "" {
		wireMsg.Content = text
	}
	if text == "" && len(toolCalls) == 0 {
		return wireMessage{}, false
	}
	return wireMsg, true
}

func convertToolResultMessage(m ai.ToolResultMessage) wireMessage {
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
	return wireMessage{Role: "tool", Content: ai.SanitizeSurrogates(text), ToolCallID: m.ToolCallID}
}

// --- headers -------------------------------------------------------------

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
