package bedrock

// Ports: packages/ai/src/api/bedrock-converse-stream.ts

// Ports the message-conversion half of bedrock-converse-stream.ts:
// convertMessages, convertToolResultContent, createRequiredTextBlock,
// createNonBlankTextBlock, buildSystemPrompt, supportsPromptCaching, and the
// tool-call id normalizer.
//
// Deviation: upstream's test suite includes cases feeding a user/assistant
// message an `{ type: "unknown", ... }` content block to verify it is skipped
// rather than throwing. ai.UserContentPart/ai.AssistantContentPart are closed
// Go interfaces (TextContent/ImageContent and TextContent/ThinkingContent/
// ToolCall respectively) that structurally cannot hold an unrecognized block
// type, so those particular cases have no Go equivalent input to construct;
// the surrounding blank/placeholder-handling behavior they were bundled with
// is ported below and covered by messages_test.go.

import (
	"regexp"
	"strings"

	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis"
)

const emptyTextPlaceholder = "<empty>"

// normalizeToolCallID sanitizes a tool-call id to Bedrock's
// ^[a-zA-Z0-9_-]+$, max 64 chars constraint. Ports normalizeToolCallId from
// bedrock-converse-stream.ts (renamed to avoid colliding with the
// package-level normalizeToolCallID var name convention used elsewhere).
var invalidToolCallIDChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func normalizeToolCallID(id string, _ *ai.Model, _ *ai.AssistantMessage) string {
	sanitized := invalidToolCallIDChars.ReplaceAllString(id, "_")
	if len(sanitized) > 64 {
		sanitized = sanitized[:64]
	}
	return sanitized
}

// createNonBlankTextBlock returns a text content block for text, or nil if
// sanitizing surrogates leaves nothing but whitespace. Ports
// createNonBlankTextBlock.
func createNonBlankTextBlock(text string) *wireContentBlock {
	sanitized := ai.SanitizeSurrogates(text)
	if strings.TrimSpace(sanitized) == "" {
		return nil
	}
	return &wireContentBlock{Text: sanitized}
}

// createRequiredTextBlock is createNonBlankTextBlock but never nil: blank
// input becomes the "<empty>" placeholder. Ports createRequiredTextBlock.
func createRequiredTextBlock(text string) wireContentBlock {
	if b := createNonBlankTextBlock(text); b != nil {
		return *b
	}
	return wireContentBlock{Text: emptyTextPlaceholder}
}

// convertToolResultContent converts tool-result content blocks
// (text/image only), falling back to the "<empty>" placeholder when nothing
// survives filtering. Ports convertToolResultContent.
func convertToolResultContent(content []ai.UserContentPart) []wireToolResultContent {
	result := make([]wireToolResultContent, 0, len(content))
	for _, c := range content {
		switch b := c.(type) {
		case ai.ImageContent:
			result = append(result, wireToolResultContent{Image: convertImageBlock(b)})
		case ai.TextContent:
			if tb := createNonBlankTextBlock(b.Text); tb != nil {
				result = append(result, wireToolResultContent{Text: tb.Text})
			}
		}
	}
	if len(result) == 0 {
		result = append(result, wireToolResultContent{Text: emptyTextPlaceholder})
	}
	return result
}

func convertImageBlock(c ai.ImageContent) *wireImageBlock {
	return &wireImageBlock{Format: imageFormat(c.MimeType), Source: wireImageSource{Bytes: decodeBase64Image(c.Data)}}
}

// isAnthropicClaudeModel reports whether model is an Anthropic Claude model
// on Bedrock, checked via both id and name (application inference profile
// ARNs don't contain the model name). Ports isAnthropicClaudeModel.
func isAnthropicClaudeModel(model *ai.Model) bool {
	id := strings.ToLower(model.ID)
	name := strings.ToLower(model.Name)
	return strings.Contains(id, "anthropic.claude") ||
		strings.Contains(id, "anthropic/claude") ||
		strings.Contains(name, "anthropic.claude") ||
		strings.Contains(name, "anthropic/claude") ||
		strings.Contains(name, "claude")
}

// supportsThinkingSignature reports whether model supports the
// reasoningContent.reasoningText.signature field (Anthropic Claude models
// only). Ports supportsThinkingSignature.
func supportsThinkingSignature(model *ai.Model) bool {
	return isAnthropicClaudeModel(model)
}

// supportsPromptCaching reports whether model supports Bedrock prompt-cache
// points. Ports supportsPromptCaching.
func supportsPromptCaching(model *ai.Model, env ai.ProviderEnv) bool {
	candidates := modelMatchCandidates(model.ID, model.Name)
	hasClaudeRef := containsAny(candidates, "claude")
	if !hasClaudeRef {
		return providerEnvValue("AWS_BEDROCK_FORCE_CACHE", env) == "1"
	}
	if containsAny(candidates, "fable-5") || containsAny(candidates, "sonnet-5") {
		return true
	}
	if containsAny(candidates, "-4-") {
		return true
	}
	if containsAny(candidates, "claude-3-7-sonnet") {
		return true
	}
	if containsAny(candidates, "claude-3-5-haiku") {
		return true
	}
	return false
}

func containsAny(candidates []string, substr string) bool {
	for _, c := range candidates {
		if strings.Contains(c, substr) {
			return true
		}
	}
	return false
}

// buildSystemPrompt builds the system content blocks, appending a cache
// point for supported Claude models when caching is enabled. Ports
// buildSystemPrompt.
func buildSystemPrompt(systemPrompt string, model *ai.Model, cacheRetention ai.CacheRetention, env ai.ProviderEnv) []wireSystemBlock {
	if systemPrompt == "" {
		return nil
	}
	blocks := []wireSystemBlock{{Text: ai.SanitizeSurrogates(systemPrompt)}}
	if cacheRetention != ai.CacheRetentionNone && supportsPromptCaching(model, env) {
		blocks = append(blocks, wireSystemBlock{CachePoint: cachePoint(cacheRetention)})
	}
	return blocks
}

func cachePoint(cacheRetention ai.CacheRetention) *wireCachePoint {
	cp := &wireCachePoint{Type: "default"}
	if cacheRetention == ai.CacheRetentionLong {
		cp.TTL = "1h"
	}
	return cp
}

// convertMessages converts a transformed conversation into Converse wire
// messages, merging consecutive toolResult messages into one user message
// (Bedrock requires all tool results for a turn in a single message) and
// appending a cache point to the last user message for supported Claude
// models when caching is enabled. Ports convertMessages.
func convertMessages(chat ai.Context, model *ai.Model, cacheRetention ai.CacheRetention, env ai.ProviderEnv) []wireMessage {
	transformed := apis.TransformMessages(chat.Messages, model, normalizeToolCallID)

	result := make([]wireMessage, 0, len(transformed))
	for i := 0; i < len(transformed); i++ {
		switch m := transformed[i].(type) {
		case ai.UserMessage:
			content := convertUserContent(m.Content)
			result = append(result, wireMessage{Role: "user", Content: content})

		case *ai.AssistantMessage:
			content := convertAssistantContent(m.Content, model)
			if len(content) == 0 {
				continue
			}
			result = append(result, wireMessage{Role: "assistant", Content: content})

		case ai.ToolResultMessage:
			blocks := []wireContentBlock{{ToolResult: convertOneToolResult(m)}}
			j := i + 1
			for j < len(transformed) {
				next, ok := transformed[j].(ai.ToolResultMessage)
				if !ok {
					break
				}
				blocks = append(blocks, wireContentBlock{ToolResult: convertOneToolResult(next)})
				j++
			}
			i = j - 1
			result = append(result, wireMessage{Role: "user", Content: blocks})
		}
	}

	if cacheRetention != ai.CacheRetentionNone && supportsPromptCaching(model, env) && len(result) > 0 {
		last := &result[len(result)-1]
		if last.Role == "user" {
			last.Content = append(last.Content, wireContentBlock{CachePoint: cachePoint(cacheRetention)})
		}
	}

	return result
}

func convertOneToolResult(m ai.ToolResultMessage) *wireToolResult {
	status := "success"
	if m.IsError {
		status = "error"
	}
	return &wireToolResult{
		ToolUseID: m.ToolCallID,
		Content:   convertToolResultContent(m.Content),
		Status:    status,
	}
}

func convertUserContent(content ai.UserContent) []wireContentBlock {
	if content.Plain != nil {
		return []wireContentBlock{createRequiredTextBlock(*content.Plain)}
	}

	blocks := make([]wireContentBlock, 0, len(content.Blocks))
	for _, c := range content.Blocks {
		switch b := c.(type) {
		case ai.TextContent:
			if tb := createNonBlankTextBlock(b.Text); tb != nil {
				blocks = append(blocks, *tb)
			}
		case ai.ImageContent:
			blocks = append(blocks, wireContentBlock{Image: convertImageBlock(b)})
		}
	}
	if len(blocks) == 0 {
		blocks = append(blocks, wireContentBlock{Text: emptyTextPlaceholder})
	}
	return blocks
}

func convertAssistantContent(content []ai.AssistantContentPart, model *ai.Model) []wireContentBlock {
	blocks := make([]wireContentBlock, 0, len(content))
	for _, c := range content {
		switch b := c.(type) {
		case ai.TextContent:
			if tb := createNonBlankTextBlock(b.Text); tb != nil {
				blocks = append(blocks, *tb)
			}
		case ai.ToolCall:
			blocks = append(blocks, wireContentBlock{ToolUse: &wireToolUse{
				ToolUseID: b.ID,
				Name:      b.Name,
				Input:     b.Arguments,
			}})
		case ai.ThinkingContent:
			thinking := ai.SanitizeSurrogates(b.Thinking)
			if strings.TrimSpace(thinking) == "" {
				continue
			}
			if supportsThinkingSignature(model) {
				if b.ThinkingSignature == "" || strings.TrimSpace(b.ThinkingSignature) == "" {
					blocks = append(blocks, wireContentBlock{Text: thinking})
				} else {
					blocks = append(blocks, wireContentBlock{ReasoningContent: &wireReasoningContent{
						ReasoningText: &wireReasoningText{Text: thinking, Signature: b.ThinkingSignature},
					}})
				}
			} else {
				blocks = append(blocks, wireContentBlock{ReasoningContent: &wireReasoningContent{
					ReasoningText: &wireReasoningText{Text: thinking},
				}})
			}
		}
	}
	return blocks
}

// convertToolConfig converts tools/toolChoice into a wireToolConfig, or nil
// when there are no tools or toolChoice is "none". Ports convertToolConfig.
func convertToolConfig(tools []ai.Tool, toolChoice, toolChoiceFunction string) *wireToolConfig {
	if len(tools) == 0 || toolChoice == "none" {
		return nil
	}

	wireTools := make([]wireTool, len(tools))
	for i, t := range tools {
		wireTools[i] = wireTool{ToolSpec: wireToolSpec{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: wireInputSchema{JSON: decodeJSONSchema(t.Parameters)},
		}}
	}

	var choice *wireToolChoice
	if toolChoiceFunction != "" {
		choice = &wireToolChoice{Tool: &wireToolChoiceName{Name: toolChoiceFunction}}
	} else {
		switch toolChoice {
		case "auto":
			choice = &wireToolChoice{Auto: &wireEmpty{}}
		case "any":
			choice = &wireToolChoice{Any: &wireEmpty{}}
		}
	}

	return &wireToolConfig{Tools: wireTools, ToolChoice: choice}
}
