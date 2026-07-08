package google

// Ports the streaming half of google-generative-ai.ts's `stream` function:
// upstream iterates the @google/genai SDK's AsyncIterable<GenerateContentResponse>;
// this Go port has no such SDK, so it frames the same JSON chunks itself off
// an SSE body (ai/internal/sse), matching the established decodeEvents idiom
// in the sibling anthropic/openairesponses packages.
//
// DecodeStream is exported so the Vertex variant (epic 8, issue 02) can reuse
// it unchanged; only transport/auth differ between the two.

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"sync/atomic"

	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/internal/sse"
)

// blockKind discriminates the in-flight text/thinking block DecodeStream
// accumulates across chunks (mirrors the TS `currentBlock` local).
type blockKind int

const (
	blockNone blockKind = iota
	blockText
	blockThinking
)

var toolCallCounter int64

// nextToolCallID synthesizes a unique tool-call id when Gemini's response
// omits one (the common case for native Gemini models) or repeats one
// already seen. Ports the needsNewId branch of google-generative-ai.ts's
// stream function.
func nextToolCallID(name string, provided string, output *ai.AssistantMessage) string {
	if provided != "" {
		dup := false
		for _, b := range output.Content {
			if tc, ok := b.(ai.ToolCall); ok && tc.ID == provided {
				dup = true
				break
			}
		}
		if !dup {
			return provided
		}
	}
	n := atomic.AddInt64(&toolCallCounter, 1)
	return name + "_" + strconv.FormatInt(n, 10)
}

// DecodeStream reads a Gemini streamGenerateContent?alt=sse body and pushes
// ai.Events reflecting its content, mutating output in place. Ports the
// per-chunk for-await loop of google-generative-ai.ts's stream function.
func DecodeStream(out *ai.Stream, output *ai.AssistantMessage, model *ai.Model, body io.Reader) error {
	reader := sse.NewReader(body)

	kind := blockNone
	contentIndex := -1

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

	for {
		ev, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		var chunk rawChunk
		if err := json.Unmarshal([]byte(ev.Data), &chunk); err != nil {
			return fmt.Errorf("could not parse Gemini stream chunk: %w; data=%s", err, ev.Data)
		}

		if output.ResponseID == "" && chunk.ResponseID != "" {
			output.ResponseID = chunk.ResponseID
		}

		var candidate *rawCandidate
		if len(chunk.Candidates) > 0 {
			candidate = &chunk.Candidates[0]
		}

		if candidate != nil && candidate.Content != nil {
			for _, part := range candidate.Content.Parts {
				if part.Text != nil {
					isThinking := IsThinkingPart(part.Thought, part.ThoughtSignature)
					wantKind := blockText
					if isThinking {
						wantKind = blockThinking
					}
					if kind != wantKind {
						closeCurrent()
						kind = wantKind
						switch kind {
						case blockThinking:
							output.Content = append(output.Content, ai.ThinkingContent{})
							contentIndex = len(output.Content) - 1
							out.Push(ai.ThinkingStartEvent{ContentIndex: contentIndex, Partial: output.Clone()})
						case blockText:
							output.Content = append(output.Content, ai.TextContent{})
							contentIndex = len(output.Content) - 1
							out.Push(ai.TextStartEvent{ContentIndex: contentIndex, Partial: output.Clone()})
						}
					}
					switch kind {
					case blockThinking:
						cur := output.Content[contentIndex].(ai.ThinkingContent)
						cur.Thinking += *part.Text
						cur.ThinkingSignature = RetainThoughtSignature(cur.ThinkingSignature, part.ThoughtSignature)
						output.Content[contentIndex] = cur
						out.Push(ai.ThinkingDeltaEvent{ContentIndex: contentIndex, Delta: *part.Text, Partial: output.Clone()})
					case blockText:
						cur := output.Content[contentIndex].(ai.TextContent)
						cur.Text += *part.Text
						cur.TextSignature = RetainThoughtSignature(cur.TextSignature, part.ThoughtSignature)
						output.Content[contentIndex] = cur
						out.Push(ai.TextDeltaEvent{ContentIndex: contentIndex, Delta: *part.Text, Partial: output.Clone()})
					}
				}

				if part.FunctionCall != nil {
					closeCurrent()

					var args map[string]any
					if len(part.FunctionCall.Args) > 0 {
						_ = json.Unmarshal(part.FunctionCall.Args, &args)
					}
					if args == nil {
						args = map[string]any{}
					}

					id := nextToolCallID(part.FunctionCall.Name, part.FunctionCall.ID, output)
					tc := ai.ToolCall{ID: id, Name: part.FunctionCall.Name, Arguments: args}
					if part.ThoughtSignature != "" {
						tc.ThoughtSignature = part.ThoughtSignature
					}
					output.Content = append(output.Content, tc)
					idx := len(output.Content) - 1
					out.Push(ai.ToolCallStartEvent{ContentIndex: idx, Partial: output.Clone()})
					argsJSON, _ := json.Marshal(args)
					out.Push(ai.ToolCallDeltaEvent{ContentIndex: idx, Delta: string(argsJSON), Partial: output.Clone()})
					out.Push(ai.ToolCallEndEvent{ContentIndex: idx, ToolCall: tc, Partial: output.Clone()})
				}
			}
		}

		if candidate != nil && candidate.FinishReason != "" {
			output.StopReason = MapStopReasonString(candidate.FinishReason)
			if hasToolCallContent(output) {
				output.StopReason = ai.StopReasonToolUse
			}
		}

		if chunk.UsageMetadata != nil {
			u := chunk.UsageMetadata
			reasoning := u.ThoughtsTokenCount
			output.Usage = ai.Usage{
				Input:       u.PromptTokenCount - u.CachedContentTokenCount,
				Output:      u.CandidatesTokenCount + u.ThoughtsTokenCount,
				CacheRead:   u.CachedContentTokenCount,
				CacheWrite:  0,
				Reasoning:   &reasoning,
				TotalTokens: u.TotalTokenCount,
			}
			ai.CalculateCost(model, &output.Usage)
		}
	}

	closeCurrent()
	return nil
}

func hasToolCallContent(output *ai.AssistantMessage) bool {
	for _, b := range output.Content {
		if _, ok := b.(ai.ToolCall); ok {
			return true
		}
	}
	return false
}

// --- wire response decoding ---------------------------------------------------

type rawFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
	ID   string          `json:"id"`
}

type rawPart struct {
	Text             *string          `json:"text"`
	Thought          bool             `json:"thought"`
	ThoughtSignature string           `json:"thoughtSignature"`
	FunctionCall     *rawFunctionCall `json:"functionCall"`
}

type rawContent struct {
	Role  string    `json:"role"`
	Parts []rawPart `json:"parts"`
}

type rawCandidate struct {
	Content      *rawContent `json:"content"`
	FinishReason string      `json:"finishReason"`
	Index        int         `json:"index"`
}

type rawUsageMetadata struct {
	PromptTokenCount        int `json:"promptTokenCount"`
	CandidatesTokenCount    int `json:"candidatesTokenCount"`
	CachedContentTokenCount int `json:"cachedContentTokenCount"`
	ThoughtsTokenCount      int `json:"thoughtsTokenCount"`
	TotalTokenCount         int `json:"totalTokenCount"`
}

// rawChunk is one GenerateContentResponse JSON object as streamed by
// streamGenerateContent?alt=sse.
type rawChunk struct {
	ResponseID    string            `json:"responseId"`
	Candidates    []rawCandidate    `json:"candidates"`
	UsageMetadata *rawUsageMetadata `json:"usageMetadata"`
}
