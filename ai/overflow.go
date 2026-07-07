package ai

import "regexp"

// Ports: packages/ai/src/utils/overflow.ts

// overflowPatterns detect context-overflow error messages across providers.
// See the upstream file for example error text per provider.
var overflowPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)prompt is too long`),                                                                        // Anthropic token overflow
	regexp.MustCompile(`(?i)request_too_large`),                                                                         // Anthropic request byte-size overflow (HTTP 413)
	regexp.MustCompile(`(?i)input is too long for requested model`),                                                     // Amazon Bedrock
	regexp.MustCompile(`(?i)exceeds the context window`),                                                                // OpenAI (Completions & Responses API)
	regexp.MustCompile(`(?i)exceeds (?:the )?(?:model'?s )?maximum context length(?: of [\d,]+ tokens?|\s*\([\d,]+\))`), // OpenAI-compatible proxies (LiteLLM)
	regexp.MustCompile(`(?i)input token count.*exceeds the maximum`),                                                    // Google (Gemini)
	regexp.MustCompile(`(?i)maximum prompt length is \d+`),                                                              // xAI (Grok)
	regexp.MustCompile(`(?i)reduce the length of the messages`),                                                         // Groq
	regexp.MustCompile(`(?i)maximum context length is \d+ tokens`),                                                      // OpenRouter (most backends)
	regexp.MustCompile(`(?i)exceeds (?:the )?maximum allowed input length of [\d,]+ tokens?`),                           // OpenRouter/Poolside
	regexp.MustCompile(`(?i)input \(\d+ tokens\) is longer than the model'?s context length \(\d+ tokens\)`),            // Together AI
	regexp.MustCompile(`(?i)exceeds the limit of \d+`),                                                                  // GitHub Copilot
	regexp.MustCompile(`(?i)exceeds the available context size`),                                                        // llama.cpp server
	regexp.MustCompile(`(?i)greater than the context length`),                                                           // LM Studio
	regexp.MustCompile(`(?i)context window exceeds limit`),                                                              // MiniMax
	regexp.MustCompile(`(?i)exceeded model token limit`),                                                                // Kimi For Coding
	regexp.MustCompile(`(?i)too large for model with \d+ maximum context length`),                                       // Mistral
	regexp.MustCompile(`(?i)prompt has [\d,]+ tokens?, but the configured context size is [\d,]+ tokens?`),              // DS4 server
	regexp.MustCompile(`(?i)model_context_window_exceeded`),                                                             // z.ai non-standard finish_reason surfaced as error text
	regexp.MustCompile(`(?i)prompt too long; exceeded (?:max )?context length`),                                         // Ollama explicit overflow error
	regexp.MustCompile(`(?i)context[_ ]length[_ ]exceeded`),                                                             // Generic fallback
	regexp.MustCompile(`(?i)too many tokens`),                                                                           // Generic fallback
	regexp.MustCompile(`(?i)token limit exceeded`),                                                                      // Generic fallback
	regexp.MustCompile(`(?i)^4(?:00|13)\s*(?:status code)?\s*\(no body\)`),                                              // Cerebras: 400/413 with no body
}

// nonOverflowPatterns exclude messages that would false-match an overflow
// pattern (e.g. Bedrock throttling: "Too many tokens, please wait before
// trying again.").
var nonOverflowPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^(Throttling error|Service unavailable):`), // AWS Bedrock non-overflow errors
	regexp.MustCompile(`(?i)rate limit`),                               // Generic rate limiting
	regexp.MustCompile(`(?i)too many requests`),                        // Generic HTTP 429 style
}

// IsContextOverflow reports whether the assistant message represents a
// context-overflow failure.
//
// Three detection modes:
//  1. Error-based: StopReason "error" with a recognized overflow message.
//  2. Silent overflow (z.ai): StopReason "stop" but input+cacheRead exceeds
//     the context window (requires contextWindow > 0).
//  3. Length-stop overflow (Xiaomi MiMo): the server truncates oversized input
//     to fill the window, returning StopReason "length" with zero output and
//     input+cacheRead at >=99% of the window (requires contextWindow > 0).
func IsContextOverflow(message *AssistantMessage, contextWindow int) bool {
	if message.StopReason == StopReasonError && message.ErrorMessage != "" {
		isNonOverflow := false
		for _, p := range nonOverflowPatterns {
			if p.MatchString(message.ErrorMessage) {
				isNonOverflow = true
				break
			}
		}
		if !isNonOverflow {
			for _, p := range overflowPatterns {
				if p.MatchString(message.ErrorMessage) {
					return true
				}
			}
		}
	}

	if contextWindow > 0 && message.StopReason == StopReasonStop {
		if message.Usage.Input+message.Usage.CacheRead > contextWindow {
			return true
		}
	}

	if contextWindow > 0 && message.StopReason == StopReasonLength && message.Usage.Output == 0 {
		inputTokens := message.Usage.Input + message.Usage.CacheRead
		if float64(inputTokens) >= float64(contextWindow)*0.99 {
			return true
		}
	}

	return false
}

// OverflowPatterns returns a copy of the overflow patterns for testing.
func OverflowPatterns() []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(overflowPatterns))
	copy(out, overflowPatterns)
	return out
}
