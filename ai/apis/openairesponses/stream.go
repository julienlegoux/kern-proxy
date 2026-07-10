package openairesponses

// Ports: packages/ai/src/api/openai-responses-shared.ts (processResponsesStream,
// mapStopReason) and the service-tier pricing helpers from
// openai-responses.ts (getServiceTierCostMultiplier, applyServiceTierPricing).
//
// Exported (DecodeStream) so the Azure/Codex variants (epic 7, issues 02-04)
// can reuse the same SSE-to-event decode; each variant only differs in
// transport (Codex adds a WebSocket path with a zstd SSE fallback) and in
// which service-tier pricing table applies, not in how a Responses stream's
// JSON events become ai.Event pushes.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/julienlegoux/kern-link/ai"
	"github.com/julienlegoux/kern-link/ai/internal/partialjson"
	"github.com/julienlegoux/kern-link/ai/internal/sse"
)

// ServiceTierOptions carries the per-request service-tier context DecodeStream
// needs to apply OpenAI's tier pricing multiplier. Ports the
// OpenAIResponsesStreamOptions fields this package's Stream reads.
type ServiceTierOptions struct {
	// RequestServiceTier is the tier the request asked for; used only when
	// the response itself doesn't report one (or, with ResolveServiceTier
	// overridden, whenever the resolver prefers it).
	RequestServiceTier string
	// ResolveServiceTier overrides which tier wins between the
	// response-reported tier (empty when the response reports none) and
	// RequestServiceTier. nil uses the default this package's own Stream and
	// the Azure variant both rely on: prefer the server-reported tier over
	// the requested one. The Codex variant (epic 7, issue 03) overrides this
	// with its own resolveCodexServiceTier, since Codex's backend echoes
	// "default" for a request-sent flex/priority tier under some
	// conditions -- ports the resolveServiceTier hook from
	// processResponsesStream's options.
	ResolveServiceTier func(responseServiceTier, requestServiceTier string) string
}

// responsesSlotKind discriminates the in-flight output-item slot kinds
// DecodeStream tracks, keyed by the stream's output_index.
type responsesSlotKind int

const (
	slotThinking responsesSlotKind = iota + 1
	slotText
	slotToolCall
)

// responsesSlot tracks one streaming output item. partialArgs is only
// meaningful for slotToolCall (the scratch buffer for partial-JSON
// re-parsing, mirroring the openaicompletions package's toolCallBlock).
type responsesSlot struct {
	kind         responsesSlotKind
	contentIndex int
	partialArgs  strings.Builder
}

// DecodeStream reads a Responses API SSE body and pushes ai.Events reflecting
// its content, mutating output in place. Ports processResponsesStream (the
// upstream version consumes an already-JSON-decoded AsyncIterable; this
// version does its own SSE framing + JSON decode per event, matching this
// repo's established decodeEvents idiom in the anthropic/openaicompletions
// packages, since there is no openai SDK to do that decoding for us).
func DecodeStream(out *ai.Stream, output *ai.AssistantMessage, model *ai.Model, body io.Reader, opts ServiceTierOptions) error {
	reader := sse.NewReader(body)
	slots := map[int]*responsesSlot{}
	sawTerminal := false

	getSlot := func(idx int, kind responsesSlotKind) *responsesSlot {
		s, ok := slots[idx]
		if !ok || s.kind != kind {
			return nil
		}
		return s
	}

	createSlot := func(idx int, item rawItem) *responsesSlot {
		switch item.Type {
		case "reasoning":
			output.Content = append(output.Content, ai.ThinkingContent{})
			s := &responsesSlot{kind: slotThinking, contentIndex: len(output.Content) - 1}
			slots[idx] = s
			out.Push(ai.ThinkingStartEvent{ContentIndex: s.contentIndex, Partial: output.Clone()})
			return s
		case "message":
			output.Content = append(output.Content, ai.TextContent{})
			s := &responsesSlot{kind: slotText, contentIndex: len(output.Content) - 1}
			slots[idx] = s
			out.Push(ai.TextStartEvent{ContentIndex: s.contentIndex, Partial: output.Clone()})
			return s
		case "function_call":
			id := item.CallID + "|" + item.ID
			output.Content = append(output.Content, ai.ToolCall{ID: id, Name: item.Name, Arguments: map[string]any{}})
			s := &responsesSlot{kind: slotToolCall, contentIndex: len(output.Content) - 1}
			if item.Arguments != "" {
				s.partialArgs.WriteString(item.Arguments)
			}
			slots[idx] = s
			out.Push(ai.ToolCallStartEvent{ContentIndex: s.contentIndex, Partial: output.Clone()})
			return s
		default:
			return nil
		}
	}

	getOrCreateSlot := func(idx int, item rawItem) *responsesSlot {
		if s, ok := slots[idx]; ok {
			return s
		}
		return createSlot(idx, item)
	}

	for {
		ev, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		var evData rawEvent
		if err := json.Unmarshal([]byte(ev.Data), &evData); err != nil {
			continue
		}

		switch evData.Type {
		case "response.created":
			if evData.Response != nil && evData.Response.ID != "" {
				output.ResponseID = evData.Response.ID
			}

		case "response.output_item.added":
			var item rawItem
			if err := json.Unmarshal(evData.Item, &item); err == nil {
				createSlot(evData.OutputIndex, item)
			}

		case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
			if s := getSlot(evData.OutputIndex, slotThinking); s != nil {
				cur := output.Content[s.contentIndex].(ai.ThinkingContent)
				cur.Thinking += evData.Delta
				output.Content[s.contentIndex] = cur
				out.Push(ai.ThinkingDeltaEvent{ContentIndex: s.contentIndex, Delta: evData.Delta, Partial: output.Clone()})
			}

		case "response.reasoning_summary_part.done":
			if s := getSlot(evData.OutputIndex, slotThinking); s != nil {
				cur := output.Content[s.contentIndex].(ai.ThinkingContent)
				cur.Thinking += "\n\n"
				output.Content[s.contentIndex] = cur
				out.Push(ai.ThinkingDeltaEvent{ContentIndex: s.contentIndex, Delta: "\n\n", Partial: output.Clone()})
			}

		case "response.output_text.delta", "response.refusal.delta":
			if s := getSlot(evData.OutputIndex, slotText); s != nil {
				cur := output.Content[s.contentIndex].(ai.TextContent)
				cur.Text += evData.Delta
				output.Content[s.contentIndex] = cur
				out.Push(ai.TextDeltaEvent{ContentIndex: s.contentIndex, Delta: evData.Delta, Partial: output.Clone()})
			}

		case "response.function_call_arguments.delta":
			if s := getSlot(evData.OutputIndex, slotToolCall); s != nil {
				s.partialArgs.WriteString(evData.Delta)
				cur := output.Content[s.contentIndex].(ai.ToolCall)
				cur.Arguments = partialjson.ParseStreamingObject(s.partialArgs.String())
				output.Content[s.contentIndex] = cur
				out.Push(ai.ToolCallDeltaEvent{ContentIndex: s.contentIndex, Delta: evData.Delta, Partial: output.Clone()})
			}

		case "response.function_call_arguments.done":
			if s := getSlot(evData.OutputIndex, slotToolCall); s != nil {
				previous := s.partialArgs.String()
				s.partialArgs.Reset()
				s.partialArgs.WriteString(evData.Arguments)
				cur := output.Content[s.contentIndex].(ai.ToolCall)
				cur.Arguments = partialjson.ParseStreamingObject(evData.Arguments)
				output.Content[s.contentIndex] = cur
				if strings.HasPrefix(evData.Arguments, previous) {
					if delta := evData.Arguments[len(previous):]; delta != "" {
						out.Push(ai.ToolCallDeltaEvent{ContentIndex: s.contentIndex, Delta: delta, Partial: output.Clone()})
					}
				}
			}

		case "response.output_item.done":
			var item rawItem
			if err := json.Unmarshal(evData.Item, &item); err != nil {
				continue
			}
			slot := getOrCreateSlot(evData.OutputIndex, item)
			if slot == nil {
				continue
			}
			switch {
			case item.Type == "reasoning" && slot.kind == slotThinking:
				cur := output.Content[slot.contentIndex].(ai.ThinkingContent)
				if summary := joinTexts(summaryTexts(item.Summary), "\n\n"); summary != "" {
					cur.Thinking = summary
				} else if content := joinTexts(itemContentTexts(item.Content), "\n\n"); content != "" {
					cur.Thinking = content
				}
				cur.ThinkingSignature = string(evData.Item)
				output.Content[slot.contentIndex] = cur
				out.Push(ai.ThinkingEndEvent{ContentIndex: slot.contentIndex, Content: cur.Thinking, Partial: output.Clone()})
				delete(slots, evData.OutputIndex)

			case item.Type == "message" && slot.kind == slotText:
				cur := output.Content[slot.contentIndex].(ai.TextContent)
				cur.Text = joinMessageContent(item.Content)
				cur.TextSignature = encodeTextSignatureV1(item.ID, item.Phase)
				output.Content[slot.contentIndex] = cur
				out.Push(ai.TextEndEvent{ContentIndex: slot.contentIndex, Content: cur.Text, Partial: output.Clone()})
				delete(slots, evData.OutputIndex)

			case item.Type == "function_call" && slot.kind == slotToolCall:
				args := item.Arguments
				if args == "" {
					args = slot.partialArgs.String()
				}
				if args == "" {
					args = "{}"
				}
				cur := output.Content[slot.contentIndex].(ai.ToolCall)
				cur.Arguments = partialjson.ParseStreamingObject(args)
				output.Content[slot.contentIndex] = cur
				out.Push(ai.ToolCallEndEvent{ContentIndex: slot.contentIndex, ToolCall: cur, Partial: output.Clone()})
				delete(slots, evData.OutputIndex)
			}

		case "response.completed", "response.incomplete":
			sawTerminal = true
			finalizeResponse(output, evData.Response, model, opts)

		case "error":
			msg := evData.Message
			if msg == "" {
				msg = "Unknown error"
			}
			return fmt.Errorf("Error Code %s: %s", evData.Code, msg)

		case "response.failed":
			return responseFailedError(evData.Response)
		}
	}

	if !sawTerminal {
		return errors.New("OpenAI Responses stream ended before a terminal response event")
	}
	return nil
}

// responseFailedError ports the response.failed error-message construction:
// prefer the response's error, then its incomplete_details reason, then a
// generic fallback.
func responseFailedError(resp *rawResponse) error {
	if resp == nil {
		return errors.New("Unknown error (no error details in response)")
	}
	if resp.Error != nil {
		code := resp.Error.Code
		if code == "" {
			code = "unknown"
		}
		msg := resp.Error.Message
		if msg == "" {
			msg = "no message"
		}
		return fmt.Errorf("%s: %s", code, msg)
	}
	if resp.IncompleteDetails != nil && resp.IncompleteDetails.Reason != "" {
		return fmt.Errorf("incomplete: %s", resp.IncompleteDetails.Reason)
	}
	return errors.New("Unknown error (no error details in response)")
}

// finalizeResponse ports the finalizeResponse closure: usage accounting, cost
// calculation, service-tier pricing, and the stop-reason mapping (including
// the toolUse upgrade when tool calls are present and the mapped reason was
// otherwise "stop").
func finalizeResponse(output *ai.AssistantMessage, resp *rawResponse, model *ai.Model, opts ServiceTierOptions) {
	var status string
	if resp != nil {
		if resp.ID != "" {
			output.ResponseID = resp.ID
		}
		status = resp.Status
		if resp.Usage != nil {
			cachedTokens := 0
			if resp.Usage.InputTokensDetails != nil {
				cachedTokens = resp.Usage.InputTokensDetails.CachedTokens
			}
			input := resp.Usage.InputTokens - cachedTokens
			if input < 0 {
				input = 0
			}
			reasoning := 0
			if resp.Usage.OutputTokensDetails != nil {
				reasoning = resp.Usage.OutputTokensDetails.ReasoningTokens
			}
			output.Usage = ai.Usage{
				Input:       input,
				Output:      resp.Usage.OutputTokens,
				CacheRead:   cachedTokens,
				CacheWrite:  0,
				Reasoning:   &reasoning,
				TotalTokens: resp.Usage.TotalTokens,
			}
		}
	}

	ai.CalculateCost(model, &output.Usage)

	responseServiceTier := ""
	if resp != nil {
		responseServiceTier = resp.ServiceTier
	}
	var serviceTier string
	if opts.ResolveServiceTier != nil {
		serviceTier = opts.ResolveServiceTier(responseServiceTier, opts.RequestServiceTier)
	} else {
		serviceTier = opts.RequestServiceTier
		if responseServiceTier != "" {
			serviceTier = responseServiceTier
		}
	}
	applyServiceTierPricing(&output.Usage, serviceTier, model)

	reason, errMsg := mapStopReason(status)
	output.StopReason = reason
	if errMsg != "" {
		output.ErrorMessage = errMsg
	}
	if reason == ai.StopReasonStop && hasToolCallContent(output) {
		output.StopReason = ai.StopReasonToolUse
	}
}

func hasToolCallContent(output *ai.AssistantMessage) bool {
	for _, b := range output.Content {
		if _, ok := b.(ai.ToolCall); ok {
			return true
		}
	}
	return false
}

// mapStopReason ports mapStopReason from openai-responses-shared.ts. Unlike
// the TS exhaustive-switch (which throws on an unrecognized status), an
// unmapped status here degrades to StopReasonError with a diagnostic message
// -- consistent with this repo's openaicompletions.mapStopReason, since a Go
// port has no compile-time exhaustiveness check to lean on and mustn't panic
// on an unexpected wire value.
func mapStopReason(status string) (ai.StopReason, string) {
	switch status {
	case "", "completed", "in_progress", "queued":
		return ai.StopReasonStop, ""
	case "incomplete":
		return ai.StopReasonLength, ""
	case "failed", "cancelled":
		return ai.StopReasonError, ""
	default:
		return ai.StopReasonError, "Unhandled response status: " + status
	}
}

// applyServiceTierPricing ports applyServiceTierPricing: scales the
// already-calculated cost fields by the tier multiplier and recomputes total.
func applyServiceTierPricing(usage *ai.Usage, serviceTier string, model *ai.Model) {
	multiplier := serviceTierCostMultiplier(model, serviceTier)
	if multiplier == 1 {
		return
	}
	usage.Cost.Input *= multiplier
	usage.Cost.Output *= multiplier
	usage.Cost.CacheRead *= multiplier
	usage.Cost.CacheWrite *= multiplier
	usage.Cost.Total = usage.Cost.Input + usage.Cost.Output + usage.Cost.CacheRead + usage.Cost.CacheWrite
}

// serviceTierCostMultiplier ports getServiceTierCostMultiplier.
func serviceTierCostMultiplier(model *ai.Model, serviceTier string) float64 {
	switch serviceTier {
	case "flex":
		return 0.5
	case "priority":
		if model.ID == "gpt-5.5" {
			return 2.5
		}
		return 2
	default:
		return 1
	}
}

func summaryTexts(parts []rawSummaryPart) []string {
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = p.Text
	}
	return out
}

func itemContentTexts(parts []rawItemContent) []string {
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = p.Text
	}
	return out
}

func joinTexts(texts []string, sep string) string {
	return strings.Join(texts, sep)
}

// joinMessageContent ports item.content?.map((c) => c.type === "output_text"
// ? c.text : c.refusal).join(""): output_text parts contribute their text,
// anything else (refusal) contributes its refusal text, concatenated with no
// separator.
func joinMessageContent(parts []rawItemContent) string {
	var b strings.Builder
	for _, p := range parts {
		if p.Type == "output_text" {
			b.WriteString(p.Text)
		} else {
			b.WriteString(p.Refusal)
		}
	}
	return b.String()
}

// encodeTextSignatureV1 ports encodeTextSignatureV1.
func encodeTextSignatureV1(id, phase string) string {
	v := ai.TextSignatureV1{V: 1, ID: id}
	if phase == "commentary" || phase == "final_answer" {
		v.Phase = phase
	}
	raw, _ := json.Marshal(v)
	return string(raw)
}

// --- wire event decoding ------------------------------------------------------

type rawItemContent struct {
	Type    string `json:"type"` // "output_text" | "refusal"
	Text    string `json:"text"`
	Refusal string `json:"refusal"`
}

type rawSummaryPart struct {
	Text string `json:"text"`
}

// rawItem is one Responses output item (response.output_item.added/done).
type rawItem struct {
	Type      string           `json:"type"` // "message" | "reasoning" | "function_call"
	ID        string           `json:"id"`
	CallID    string           `json:"call_id"`
	Name      string           `json:"name"`
	Arguments string           `json:"arguments"`
	Phase     string           `json:"phase"`
	Content   []rawItemContent `json:"content"`
	Summary   []rawSummaryPart `json:"summary"`
}

type rawResponseTokenDetails struct {
	CachedTokens    int `json:"cached_tokens"`
	ReasoningTokens int `json:"reasoning_tokens"`
}

type rawResponseUsage struct {
	InputTokens         int                      `json:"input_tokens"`
	OutputTokens        int                      `json:"output_tokens"`
	TotalTokens         int                      `json:"total_tokens"`
	InputTokensDetails  *rawResponseTokenDetails `json:"input_tokens_details"`
	OutputTokensDetails *rawResponseTokenDetails `json:"output_tokens_details"`
}

type rawResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type rawIncompleteDetails struct {
	Reason string `json:"reason"`
}

// rawResponse is the terminal `response` object carried by
// response.completed/incomplete/failed events.
type rawResponse struct {
	ID                string                `json:"id"`
	Status            string                `json:"status"`
	ServiceTier       string                `json:"service_tier"`
	Usage             *rawResponseUsage     `json:"usage"`
	Error             *rawResponseError     `json:"error"`
	IncompleteDetails *rawIncompleteDetails `json:"incomplete_details"`
}

// rawEvent is one Responses API SSE event payload, discriminated by Type.
// Item is kept as raw bytes (rather than eagerly decoded) so a reasoning
// item's ThinkingSignature can replay the exact wire bytes verbatim.
type rawEvent struct {
	Type        string          `json:"type"`
	Response    *rawResponse    `json:"response"`
	OutputIndex int             `json:"output_index"`
	Item        json.RawMessage `json:"item"`
	Delta       string          `json:"delta"`
	Arguments   string          `json:"arguments"`
	Code        string          `json:"code"`
	Message     string          `json:"message"`
}
