package ai

import "encoding/json"

// Ports: packages/ai/src/utils/estimate.ts

// ContextUsageEstimate is a best-effort token estimate for a Context.
type ContextUsageEstimate struct {
	// Tokens is the estimated total context tokens.
	Tokens int
	// UsageTokens is the count reported by the most recent assistant usage block.
	UsageTokens int
	// TrailingTokens is the estimate for messages after that usage block.
	TrailingTokens int
	// LastUsageIndex is the index of the message that provided usage, or -1.
	LastUsageIndex int
}

const (
	charsPerToken       = 4
	estimatedImageChars = 4800
)

// CalculateContextTokens derives total context tokens from a usage block.
func CalculateContextTokens(usage Usage) int {
	if usage.TotalTokens != 0 {
		return usage.TotalTokens
	}
	return usage.Input + usage.Output + usage.CacheRead + usage.CacheWrite
}

func safeJSONStringifyForEstimate(value any) string {
	b, err := json.Marshal(value)
	if err != nil {
		return "[unserializable]"
	}
	return string(b)
}

// stringLengthUTF16 counts UTF-16 code units, matching JS String.length so
// token estimates agree with the TS implementation.
func stringLengthUTF16(s string) int {
	n := 0
	for _, r := range s {
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
	}
	return n
}

func estimateUserContentChars(content UserContent) int {
	if content.Plain != nil {
		return stringLengthUTF16(*content.Plain)
	}
	chars := 0
	for _, block := range content.Blocks {
		switch b := block.(type) {
		case TextContent:
			chars += stringLengthUTF16(b.Text)
		default:
			chars += estimatedImageChars
		}
	}
	return chars
}

func estimateBlockListChars(blocks []UserContentPart) int {
	chars := 0
	for _, block := range blocks {
		switch b := block.(type) {
		case TextContent:
			chars += stringLengthUTF16(b.Text)
		default:
			chars += estimatedImageChars
		}
	}
	return chars
}

func ceilDiv(chars, per int) int {
	return (chars + per - 1) / per
}

// EstimateTextTokens estimates tokens for plain text (chars/4, rounded up).
func EstimateTextTokens(text string) int {
	return ceilDiv(stringLengthUTF16(text), charsPerToken)
}

// EstimateMessageTokens estimates tokens for one message.
func EstimateMessageTokens(message Message) int {
	switch m := message.(type) {
	case *UserMessage:
		return ceilDiv(estimateUserContentChars(m.Content), charsPerToken)
	case *ToolResultMessage:
		return ceilDiv(estimateBlockListChars(m.Content), charsPerToken)
	case *AssistantMessage:
		chars := 0
		for _, block := range m.Content {
			switch b := block.(type) {
			case TextContent:
				chars += stringLengthUTF16(b.Text)
			case ThinkingContent:
				chars += stringLengthUTF16(b.Thinking)
			case ToolCall:
				chars += stringLengthUTF16(b.Name) + stringLengthUTF16(safeJSONStringifyForEstimate(b.Arguments))
			}
		}
		return ceilDiv(chars, charsPerToken)
	default:
		return 0
	}
}

func lastAssistantUsage(messages []Message) (Usage, int) {
	for i := len(messages) - 1; i >= 0; i-- {
		assistant, ok := messages[i].(*AssistantMessage)
		if !ok {
			continue
		}
		if assistant.StopReason == StopReasonAborted || assistant.StopReason == StopReasonError {
			continue
		}
		if CalculateContextTokens(assistant.Usage) > 0 {
			return assistant.Usage, i
		}
	}
	return Usage{}, -1
}

// EstimateMessagesTokens estimates context tokens for a message list,
// anchoring on the most recent non-failed assistant usage block and estimating
// only the trailing messages after it.
func EstimateMessagesTokens(messages []Message) ContextUsageEstimate {
	usage, index := lastAssistantUsage(messages)
	if index >= 0 {
		usageTokens := CalculateContextTokens(usage)
		trailing := 0
		for i := index + 1; i < len(messages); i++ {
			trailing += EstimateMessageTokens(messages[i])
		}
		return ContextUsageEstimate{
			Tokens:         usageTokens + trailing,
			UsageTokens:    usageTokens,
			TrailingTokens: trailing,
			LastUsageIndex: index,
		}
	}

	tokens := 0
	for _, m := range messages {
		tokens += EstimateMessageTokens(m)
	}
	return ContextUsageEstimate{Tokens: tokens, TrailingTokens: tokens, LastUsageIndex: -1}
}

// EstimateContextTokens estimates tokens for a full Context. When no usage
// anchor exists, the system prompt and tool definitions are added to the
// estimate.
func EstimateContextTokens(chat Context) ContextUsageEstimate {
	estimate := EstimateMessagesTokens(chat.Messages)
	if estimate.LastUsageIndex >= 0 {
		return estimate
	}

	prefixTokens := 0
	if chat.SystemPrompt != "" {
		prefixTokens = EstimateTextTokens(chat.SystemPrompt)
	}
	if len(chat.Tools) > 0 {
		prefixTokens += EstimateTextTokens(safeJSONStringifyForEstimate(chat.Tools))
	}

	estimate.Tokens += prefixTokens
	estimate.TrailingTokens += prefixTokens
	return estimate
}
