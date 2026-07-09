// Ports: packages/ai/src/api/openai-completions.ts (detectCompat, getCompat)
package openaicompletions

import (
	"strings"

	"github.com/julienlegoux/kern-proxy/ai"
)

// resolvedCompat is the fully-resolved (no nil/tri-state) compatibility
// matrix for one model: detectCompat's auto-detected defaults, overridden
// field-by-field by any explicit ai.Model.Compat entries via getCompat.
// Mirrors upstream's ResolvedOpenAICompletionsCompat.
//
// Fields marked "wired in a later issue" are resolved here (so getCompat's
// override semantics are correct for them too) but not yet read by this
// package's request/message building; their auto-detection rules are still
// part of the matrix this issue ports.
type resolvedCompat struct {
	supportsStore                               bool
	supportsDeveloperRole                       bool
	supportsReasoningEffort                     bool
	supportsUsageInStreaming                    bool
	maxTokensField                              string // "max_tokens" | "max_completion_tokens"
	requiresToolResultName                      bool
	requiresAssistantAfterToolResult            bool
	requiresThinkingAsText                      bool
	requiresReasoningContentOnAssistantMessages bool
	thinkingFormat                              ai.ThinkingFormat
	// chatTemplateKwargs configures the "chat-template" thinkingFormat's
	// per-key chat_template_kwargs resolution. Always non-nil (detectCompat
	// defaults to an empty map, mirroring upstream's `chatTemplateKwargs: {}`).
	chatTemplateKwargs         map[string]ai.ChatTemplateKwargValue
	zaiToolStream              bool
	supportsStrictMode         bool
	cacheControlFormat         string // wired in issue 04 (cache/affinity)
	sendSessionAffinityHeaders bool   // wired in issue 04 (cache/affinity)
	supportsLongCacheRetention bool   // wired in issue 04 (cache/affinity)
}

// detectCompat auto-detects compatibility settings from provider name and
// baseUrl. Used as the base when model.Compat is not set (or leaves a field
// unset); explicit model.Compat entries override these detected values via
// getCompat.
//
// Ports: openai-completions.ts detectCompat.
func detectCompat(model *ai.Model) resolvedCompat {
	provider := string(model.Provider)
	baseURL := model.BaseURL

	isZai := provider == "zai" || provider == "zai-coding-cn" ||
		strings.Contains(baseURL, "api.z.ai") || strings.Contains(baseURL, "open.bigmodel.cn")
	isTogether := provider == "together" ||
		strings.Contains(baseURL, "api.together.ai") || strings.Contains(baseURL, "api.together.xyz")
	isMoonshot := provider == "moonshotai" || provider == "moonshotai-cn" || strings.Contains(baseURL, "api.moonshot.")
	isOpenRouter := provider == "openrouter" || strings.Contains(baseURL, "openrouter.ai")
	isCloudflareWorkersAI := provider == "cloudflare-workers-ai" || strings.Contains(baseURL, "api.cloudflare.com")
	isCloudflareAiGateway := provider == "cloudflare-ai-gateway" || strings.Contains(baseURL, "gateway.ai.cloudflare.com")
	isNvidia := provider == "nvidia" || strings.Contains(baseURL, "integrate.api.nvidia.com")
	isAntLing := provider == "ant-ling" || strings.Contains(baseURL, "api.ant-ling.com")

	isNonStandard := isNvidia ||
		provider == "cerebras" || strings.Contains(baseURL, "cerebras.ai") ||
		provider == "xai" || strings.Contains(baseURL, "api.x.ai") ||
		isTogether ||
		strings.Contains(baseURL, "chutes.ai") ||
		strings.Contains(baseURL, "deepseek.com") ||
		isZai ||
		isMoonshot ||
		provider == "opencode" || strings.Contains(baseURL, "opencode.ai") ||
		isCloudflareWorkersAI ||
		isCloudflareAiGateway ||
		isAntLing

	useMaxTokens := strings.Contains(baseURL, "chutes.ai") ||
		isMoonshot || isCloudflareAiGateway || isTogether || isNvidia || isAntLing

	isGrok := provider == "xai" || strings.Contains(baseURL, "api.x.ai")
	isDeepSeek := provider == "deepseek" || strings.Contains(baseURL, "deepseek.com")
	isOpenRouterDeveloperRoleModel := isOpenRouter &&
		(strings.HasPrefix(model.ID, "anthropic/") || strings.HasPrefix(model.ID, "openai/"))
	cacheControlFormat := ""
	if provider == "openrouter" && strings.HasPrefix(model.ID, "anthropic/") {
		cacheControlFormat = "anthropic"
	}

	maxTokensField := "max_completion_tokens"
	if useMaxTokens {
		maxTokensField = "max_tokens"
	}

	thinkingFormat := ai.ThinkingFormatOpenAI
	switch {
	case isDeepSeek:
		thinkingFormat = ai.ThinkingFormatDeepseek
	case isZai:
		thinkingFormat = ai.ThinkingFormatZai
	case isTogether:
		thinkingFormat = ai.ThinkingFormatTogether
	case isAntLing:
		thinkingFormat = ai.ThinkingFormatAntLing
	case isOpenRouter:
		thinkingFormat = ai.ThinkingFormatOpenRouter
	}

	return resolvedCompat{
		supportsStore:         !isNonStandard,
		supportsDeveloperRole: isOpenRouterDeveloperRoleModel || (!isNonStandard && !isOpenRouter),
		supportsReasoningEffort: !isGrok && !isZai && !isMoonshot && !isTogether &&
			!isCloudflareAiGateway && !isNvidia && !isAntLing,
		supportsUsageInStreaming:                    true,
		maxTokensField:                              maxTokensField,
		requiresToolResultName:                      false,
		requiresAssistantAfterToolResult:            false,
		requiresThinkingAsText:                      false,
		requiresReasoningContentOnAssistantMessages: isDeepSeek,
		thinkingFormat:                              thinkingFormat,
		chatTemplateKwargs:                          map[string]ai.ChatTemplateKwargValue{},
		zaiToolStream:                               false,
		supportsStrictMode:                          !isMoonshot && !isTogether && !isCloudflareAiGateway && !isNvidia,
		cacheControlFormat:                          cacheControlFormat,
		sendSessionAffinityHeaders:                  false,
		supportsLongCacheRetention:                  !isTogether && !isCloudflareWorkersAI && !isCloudflareAiGateway && !isNvidia && !isAntLing,
	}
}

// getCompat returns the resolved compatibility settings for a model:
// auto-detected from provider/baseUrl, then overridden field-by-field by any
// explicit model.Compat entries.
//
// Ports: openai-completions.ts getCompat.
func getCompat(model *ai.Model) resolvedCompat {
	detected := detectCompat(model)
	c := model.Compat
	if c == nil {
		return detected
	}

	result := detected
	if c.SupportsStore != nil {
		result.supportsStore = *c.SupportsStore
	}
	if c.SupportsDeveloperRole != nil {
		result.supportsDeveloperRole = *c.SupportsDeveloperRole
	}
	if c.SupportsReasoningEffort != nil {
		result.supportsReasoningEffort = *c.SupportsReasoningEffort
	}
	if c.SupportsUsageInStreaming != nil {
		result.supportsUsageInStreaming = *c.SupportsUsageInStreaming
	}
	if c.MaxTokensField != "" {
		result.maxTokensField = c.MaxTokensField
	}
	if c.RequiresToolResultName != nil {
		result.requiresToolResultName = *c.RequiresToolResultName
	}
	if c.RequiresAssistantAfterToolResult != nil {
		result.requiresAssistantAfterToolResult = *c.RequiresAssistantAfterToolResult
	}
	if c.RequiresThinkingAsText != nil {
		result.requiresThinkingAsText = *c.RequiresThinkingAsText
	}
	if c.RequiresReasoningContentOnAssistantMessages != nil {
		result.requiresReasoningContentOnAssistantMessages = *c.RequiresReasoningContentOnAssistantMessages
	}
	if c.ThinkingFormat != "" {
		result.thinkingFormat = c.ThinkingFormat
	}
	if c.ChatTemplateKwargs != nil {
		result.chatTemplateKwargs = c.ChatTemplateKwargs
	}
	if c.ZaiToolStream != nil {
		result.zaiToolStream = *c.ZaiToolStream
	}
	if c.SupportsStrictMode != nil {
		result.supportsStrictMode = *c.SupportsStrictMode
	}
	if c.CacheControlFormat != "" {
		result.cacheControlFormat = c.CacheControlFormat
	}
	if c.SendSessionAffinityHeaders != nil {
		result.sendSessionAffinityHeaders = *c.SendSessionAffinityHeaders
	}
	if c.SupportsLongCacheRetention != nil {
		result.supportsLongCacheRetention = *c.SupportsLongCacheRetention
	}
	return result
}
