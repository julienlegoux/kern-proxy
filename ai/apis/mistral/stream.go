package mistral

// Ports: packages/ai/src/api/mistral-conversations.ts

// Ports the streaming half of mistral-conversations.ts's consumeChatStream:
// upstream iterates the @mistralai/mistralai SDK's AsyncIterable<CompletionEvent>;
// this Go port has no such SDK, so it frames the same JSON chunks itself off
// an SSE body (ai/internal/sse), matching the established decodeEvents idiom
// in the sibling anthropic/openaicompletions/google packages. The chat
// completions SSE stream is OpenAI-shaped (delta.content, delta.tool_calls,
// finish_reason, a trailing "data: [DONE]"), except that delta.content may
// be a plain string OR an array of typed content parts (Magistral's
// "thinking" parts interleaved with "text" parts).

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/internal/partialjson"
	"github.com/kern-ia/kern-link/ai/internal/sse"
)

// blockKind discriminates the in-flight text/thinking block DecodeStream
// accumulates across chunks (mirrors the TS `currentBlock` local).
type blockKind int

const (
	blockNone blockKind = iota
	blockText
	blockThinking
)

// toolCallBlock tracks the scratch state needed to assemble one streaming
// tool call: partialArgs accumulates the raw (possibly fragmented) JSON
// arguments string for incremental re-parsing via partialjson.
type toolCallBlock struct {
	contentIndex int
	partialArgs  strings.Builder
}

// DecodeStream reads a Mistral chat/completions SSE body and pushes
// ai.Events reflecting its content, mutating output in place. Ports
// consumeChatStream from mistral-conversations.ts.
func DecodeStream(out *ai.Stream, output *ai.AssistantMessage, model *ai.Model, body io.Reader) error {
	reader := sse.NewReader(body)

	kind := blockNone
	contentIndex := -1
	toolBlocksByKey := map[string]*toolCallBlock{}
	var toolOrder []*toolCallBlock

	closeCurrent := func() {
		if kind == blockNone {
			return
		}
		switch kind {
		case blockText:
			cur := output.Content[contentIndex].(ai.TextContent)
			out.Push(ai.TextEndEvent{ContentIndex: contentIndex, Content: cur.Text, Partial: output.Clone()})
		case blockThinking:
			cur := output.Content[contentIndex].(ai.ThinkingContent)
			out.Push(ai.ThinkingEndEvent{ContentIndex: contentIndex, Content: cur.Thinking, Partial: output.Clone()})
		}
		kind = blockNone
		contentIndex = -1
	}

	appendDelta := func(wantKind blockKind, text string) {
		if kind != wantKind {
			closeCurrent()
			kind = wantKind
			switch kind {
			case blockText:
				output.Content = append(output.Content, ai.TextContent{})
				contentIndex = len(output.Content) - 1
				out.Push(ai.TextStartEvent{ContentIndex: contentIndex, Partial: output.Clone()})
			case blockThinking:
				output.Content = append(output.Content, ai.ThinkingContent{})
				contentIndex = len(output.Content) - 1
				out.Push(ai.ThinkingStartEvent{ContentIndex: contentIndex, Partial: output.Clone()})
			}
		}
		switch kind {
		case blockText:
			cur := output.Content[contentIndex].(ai.TextContent)
			cur.Text += text
			output.Content[contentIndex] = cur
			out.Push(ai.TextDeltaEvent{ContentIndex: contentIndex, Delta: text, Partial: output.Clone()})
		case blockThinking:
			cur := output.Content[contentIndex].(ai.ThinkingContent)
			cur.Thinking += text
			output.Content[contentIndex] = cur
			out.Push(ai.ThinkingDeltaEvent{ContentIndex: contentIndex, Delta: text, Partial: output.Clone()})
		}
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
			return fmt.Errorf("could not parse Mistral stream chunk: %w; data=%s", err, ev.Data)
		}

		if output.ResponseID == "" && chunk.ID != "" {
			output.ResponseID = chunk.ID
		}

		if chunk.Usage != nil {
			promptTokens := chunk.Usage.PromptTokens
			cached := cachedPromptTokens(chunk.Usage)
			if cached > promptTokens {
				cached = promptTokens
			}
			if cached < 0 {
				cached = 0
			}
			input := promptTokens - cached
			if input < 0 {
				input = 0
			}
			total := chunk.Usage.TotalTokens
			if total == 0 {
				total = input + chunk.Usage.CompletionTokens + cached
			}
			output.Usage.Input = input
			output.Usage.Output = chunk.Usage.CompletionTokens
			output.Usage.CacheRead = cached
			output.Usage.CacheWrite = 0
			output.Usage.TotalTokens = total
			ai.CalculateCost(model, &output.Usage)
		}

		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]

		if choice.FinishReason != "" {
			output.StopReason = mapChatStopReason(choice.FinishReason)
		}

		delta := choice.Delta
		for _, item := range decodeContentItems(delta.Content) {
			if item.thinking {
				appendDelta(blockThinking, item.text)
			} else {
				appendDelta(blockText, item.text)
			}
		}

		if len(delta.ToolCalls) > 0 {
			closeCurrent()
		}
		for _, tc := range delta.ToolCalls {
			index := 0
			if tc.Index != nil {
				index = *tc.Index
			}
			callID := tc.ID
			if callID == "" || callID == "null" {
				callID = deriveMistralToolCallID(fmt.Sprintf("toolcall:%d", index), 0)
			}
			key := fmt.Sprintf("%s:%d", callID, index)

			block, ok := toolBlocksByKey[key]
			if !ok {
				name := ""
				if tc.Function != nil {
					name = tc.Function.Name
				}
				output.Content = append(output.Content, ai.ToolCall{ID: callID, Name: name, Arguments: map[string]any{}})
				block = &toolCallBlock{contentIndex: len(output.Content) - 1}
				toolBlocksByKey[key] = block
				toolOrder = append(toolOrder, block)
				out.Push(ai.ToolCallStartEvent{ContentIndex: block.contentIndex, Partial: output.Clone()})
			}

			argsDelta := ""
			if tc.Function != nil && tc.Function.Arguments != "" {
				argsDelta = tc.Function.Arguments
				block.partialArgs.WriteString(argsDelta)
				cur := output.Content[block.contentIndex].(ai.ToolCall)
				cur.Arguments = partialjson.ParseStreamingObject(block.partialArgs.String())
				output.Content[block.contentIndex] = cur
			}
			out.Push(ai.ToolCallDeltaEvent{ContentIndex: block.contentIndex, Delta: argsDelta, Partial: output.Clone()})
		}
	}

	closeCurrent()
	for _, block := range toolOrder {
		cur := output.Content[block.contentIndex].(ai.ToolCall)
		cur.Arguments = partialjson.ParseStreamingObject(block.partialArgs.String())
		output.Content[block.contentIndex] = cur
		out.Push(ai.ToolCallEndEvent{ContentIndex: block.contentIndex, ToolCall: cur, Partial: output.Clone()})
	}

	return nil
}

// cachedPromptTokens extracts the cached-prompt-token count from usage,
// probing the documented `prompt_tokens_details.cached_tokens` field. Ports
// the fidelity-preserving subset of getMistralCachedPromptTokens from
// mistral-conversations.ts: upstream additionally probes several alternate
// casings (numCachedTokens, prompt_token_details, ...) defensively across
// SDK versions, which this raw-HTTP port does not need to replicate since it
// targets one documented wire shape (deviation noted for PORTING.md parity).
func cachedPromptTokens(usage *rawUsage) int {
	if usage.PromptTokensDetails != nil {
		return usage.PromptTokensDetails.CachedTokens
	}
	return 0
}

// contentItem is one decoded delta.content entry: either a plain string
// (treated as text) or a typed {type: "text"|"thinking", ...} part.
type contentItem struct {
	thinking bool
	text     string
}

// decodeContentItems decodes a chat/completions delta.content field, which
// may be a plain string, an array of plain strings, or an array of typed
// content parts (Magistral's thinking/text parts). Ports the
// `typeof delta.content === "string" ? [delta.content] : delta.content`
// normalization plus the per-item type switch from consumeChatStream.
func decodeContentItems(raw json.RawMessage) []contentItem {
	if len(raw) == 0 {
		return nil
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		if asString == "" {
			return nil
		}
		return []contentItem{{text: asString}}
	}

	var rawItems []json.RawMessage
	if err := json.Unmarshal(raw, &rawItems); err != nil {
		return nil
	}

	var out []contentItem
	for _, item := range rawItems {
		var asStr string
		if err := json.Unmarshal(item, &asStr); err == nil {
			if asStr != "" {
				out = append(out, contentItem{text: asStr})
			}
			continue
		}

		var typed struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			Thinking []struct {
				Text string `json:"text"`
			} `json:"thinking"`
		}
		if err := json.Unmarshal(item, &typed); err != nil {
			continue
		}
		switch typed.Type {
		case "thinking":
			var b strings.Builder
			for _, part := range typed.Thinking {
				if part.Text != "" {
					b.WriteString(part.Text)
				}
			}
			if b.Len() > 0 {
				out = append(out, contentItem{thinking: true, text: b.String()})
			}
		case "text":
			if typed.Text != "" {
				out = append(out, contentItem{text: typed.Text})
			}
		}
	}
	return out
}

// --- wire event decoding ------------------------------------------------------

type rawPromptTokenDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

type rawUsage struct {
	PromptTokens        int                    `json:"prompt_tokens"`
	CompletionTokens    int                    `json:"completion_tokens"`
	TotalTokens         int                    `json:"total_tokens"`
	PromptTokensDetails *rawPromptTokenDetails `json:"prompt_tokens_details"`
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
	Content   json.RawMessage    `json:"content"`
	ToolCalls []rawToolCallDelta `json:"tool_calls"`
}

type rawChoice struct {
	Delta        rawDelta `json:"delta"`
	FinishReason string   `json:"finish_reason"`
}

// rawChunk is one chat/completions streamed chunk.
type rawChunk struct {
	ID      string      `json:"id"`
	Choices []rawChoice `json:"choices"`
	Usage   *rawUsage   `json:"usage"`
}
