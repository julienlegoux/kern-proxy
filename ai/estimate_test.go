package ai

// Ports the estimation semantics of packages/ai/src/utils/estimate.ts
// (exercised upstream by test/tokens.test.ts / context-overflow.test.ts).

import "testing"

func assistantWithUsage(total int, stop StopReason) *AssistantMessage {
	return &AssistantMessage{
		Content:    []AssistantContentPart{TextContent{Text: "reply"}},
		StopReason: stop,
		Usage:      Usage{TotalTokens: total},
	}
}

func TestCalculateContextTokens(t *testing.T) {
	if got := CalculateContextTokens(Usage{TotalTokens: 42}); got != 42 {
		t.Errorf("totalTokens preferred, got %d", got)
	}
	if got := CalculateContextTokens(Usage{Input: 1, Output: 2, CacheRead: 3, CacheWrite: 4}); got != 10 {
		t.Errorf("fallback sum = %d, want 10", got)
	}
}

func TestEstimateTextTokens(t *testing.T) {
	if got := EstimateTextTokens("abcd"); got != 1 {
		t.Errorf("4 chars = %d tokens, want 1", got)
	}
	if got := EstimateTextTokens("abcde"); got != 2 {
		t.Errorf("5 chars = %d tokens, want 2 (ceil)", got)
	}
	// Astral characters count as 2 UTF-16 units, like JS String.length.
	if got := EstimateTextTokens("🙈🙈"); got != 1 {
		t.Errorf("2 emoji (4 utf16 units) = %d tokens, want 1", got)
	}
}

func TestEstimateAnchorsOnLastUsage(t *testing.T) {
	messages := []Message{
		&UserMessage{Content: UserText("earlier message that is long enough to matter")},
		assistantWithUsage(1000, StopReasonStop),
		&UserMessage{Content: UserText("12345678")}, // 2 tokens trailing
	}
	est := EstimateMessagesTokens(messages)
	if est.LastUsageIndex != 1 {
		t.Errorf("lastUsageIndex = %d, want 1", est.LastUsageIndex)
	}
	if est.UsageTokens != 1000 || est.TrailingTokens != 2 || est.Tokens != 1002 {
		t.Errorf("est = %+v", est)
	}
}

func TestEstimateSkipsFailedAssistantTurns(t *testing.T) {
	messages := []Message{
		assistantWithUsage(500, StopReasonStop),
		assistantWithUsage(9000, StopReasonError),
		assistantWithUsage(9000, StopReasonAborted),
	}
	est := EstimateMessagesTokens(messages)
	if est.LastUsageIndex != 0 || est.UsageTokens != 500 {
		t.Errorf("est = %+v (must anchor on the non-failed turn)", est)
	}
}

func TestEstimateWithoutUsageAnchor(t *testing.T) {
	messages := []Message{
		&UserMessage{Content: UserText("abcdefgh")},                               // 2 tokens
		&ToolResultMessage{Content: []UserContentPart{TextContent{Text: "abcd"}}}, // 1 token
	}
	est := EstimateMessagesTokens(messages)
	if est.LastUsageIndex != -1 || est.Tokens != 3 || est.TrailingTokens != 3 {
		t.Errorf("est = %+v", est)
	}
}

func TestEstimateImagesUseFixedCost(t *testing.T) {
	messages := []Message{
		&UserMessage{Content: UserBlocks(ImageContent{Data: "AAAA", MimeType: "image/png"})},
	}
	est := EstimateMessagesTokens(messages)
	if est.Tokens != 1200 { // 4800 chars / 4
		t.Errorf("image estimate = %d, want 1200", est.Tokens)
	}
}

func TestEstimateContextAddsPrefixWithoutAnchor(t *testing.T) {
	chat := Context{
		SystemPrompt: "abcdefgh",                                         // 2 tokens
		Messages:     []Message{&UserMessage{Content: UserText("abcd")}}, // 1 token
	}
	est := EstimateContextTokens(chat)
	if est.Tokens != 3 {
		t.Errorf("tokens = %d, want 3", est.Tokens)
	}

	// With a usage anchor the prefix is already covered by reported usage.
	chat.Messages = append(chat.Messages, assistantWithUsage(100, StopReasonStop))
	est = EstimateContextTokens(chat)
	if est.Tokens != 100 {
		t.Errorf("tokens with anchor = %d, want 100", est.Tokens)
	}
}

func TestEstimateAssistantToolCallCountsNameAndArgs(t *testing.T) {
	msg := &AssistantMessage{
		StopReason: StopReasonToolUse,
		Content: []AssistantContentPart{
			ToolCall{ID: "1", Name: "grep", Arguments: map[string]any{"q": "x"}},
		},
	}
	// chars = len("grep") + len(`{"q":"x"}`) = 4 + 9 = 13 -> ceil(13/4) = 4
	if got := EstimateMessageTokens(msg); got != 4 {
		t.Errorf("tokens = %d, want 4", got)
	}
}
