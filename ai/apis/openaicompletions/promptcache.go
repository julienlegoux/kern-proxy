// Ports: packages/ai/src/api/openai-prompt-cache.ts
package openaicompletions

// openAIPromptCacheKeyMaxLength is OpenAI's prompt_cache_key length limit.
const openAIPromptCacheKeyMaxLength = 64

// clampOpenAIPromptCacheKey truncates key to OpenAI's 64-character limit,
// counting Unicode code points (not bytes) to match upstream's
// `Array.from(key)` splitting. An empty key is returned unchanged (the
// "unset" case is represented by the caller never invoking this at all, since
// Go has no undefined).
func clampOpenAIPromptCacheKey(key string) string {
	runes := []rune(key)
	if len(runes) <= openAIPromptCacheKeyMaxLength {
		return key
	}
	return string(runes[:openAIPromptCacheKeyMaxLength])
}
