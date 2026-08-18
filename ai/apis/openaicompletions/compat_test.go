package openaicompletions

// New Go tests (not literal upstream ports): upstream's detectCompat/getCompat
// are pure functions with no dedicated upstream test file of their own — the
// TS test suite exercises the matrix indirectly through the model catalog
// (getModel("<provider>", ...)), which is Epic 11's concern here (see the
// issue file's "Out of scope" note on catalog enforcement). These tests cover
// the same fidelity bar directly: one case per vendor branch in detectCompat,
// plus getCompat's override-wins-over-detection contract.

import (
	"reflect"
	"testing"

	"github.com/kern-ia/kern-link/ai"
)

func compatModel(provider, baseURL, id string) *ai.Model {
	return &ai.Model{ID: id, Provider: ai.ProviderId(provider), BaseURL: baseURL}
}

func TestDetectCompat_VendorMatrix(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		baseURL  string
		modelID  string
		want     resolvedCompat
	}{
		{
			name:     "openai default",
			provider: "openai",
			baseURL:  "https://api.openai.com/v1",
			modelID:  "gpt-4o-mini",
			want: resolvedCompat{
				supportsStore:              true,
				supportsDeveloperRole:      true,
				supportsReasoningEffort:    true,
				supportsUsageInStreaming:   true,
				maxTokensField:             "max_completion_tokens",
				thinkingFormat:             ai.ThinkingFormatOpenAI,
				supportsStrictMode:         true,
				supportsLongCacheRetention: true,
			},
		},
		{
			name:     "together by provider id",
			provider: "together",
			baseURL:  "https://api.together.xyz/v1",
			modelID:  "meta-llama/Llama-3",
			want: resolvedCompat{
				supportsStore:              false,
				supportsDeveloperRole:      false,
				supportsReasoningEffort:    false,
				supportsUsageInStreaming:   true,
				maxTokensField:             "max_tokens",
				thinkingFormat:             ai.ThinkingFormatTogether,
				supportsStrictMode:         false,
				supportsLongCacheRetention: false,
			},
		},
		{
			name:     "together detected from baseUrl only",
			provider: "custom-vllm",
			baseURL:  "https://api.together.ai/v1",
			modelID:  "some-model",
			want: resolvedCompat{
				supportsStore:              false,
				supportsDeveloperRole:      false,
				supportsReasoningEffort:    false,
				supportsUsageInStreaming:   true,
				maxTokensField:             "max_tokens",
				thinkingFormat:             ai.ThinkingFormatTogether,
				supportsStrictMode:         false,
				supportsLongCacheRetention: false,
			},
		},
		{
			name:     "moonshotai",
			provider: "moonshotai",
			baseURL:  "https://api.moonshot.cn/v1",
			modelID:  "kimi-k2",
			want: resolvedCompat{
				supportsStore:              false,
				supportsDeveloperRole:      false,
				supportsReasoningEffort:    false,
				supportsUsageInStreaming:   true,
				maxTokensField:             "max_tokens",
				thinkingFormat:             ai.ThinkingFormatOpenAI,
				supportsStrictMode:         false,
				supportsLongCacheRetention: true,
			},
		},
		{
			name:     "zai by baseUrl",
			provider: "custom",
			baseURL:  "https://api.z.ai/v1",
			modelID:  "glm-4.6",
			want: resolvedCompat{
				supportsStore:              false,
				supportsDeveloperRole:      false,
				supportsReasoningEffort:    false,
				supportsUsageInStreaming:   true,
				maxTokensField:             "max_completion_tokens",
				thinkingFormat:             ai.ThinkingFormatZai,
				supportsStrictMode:         true,
				supportsLongCacheRetention: true,
			},
		},
		{
			name:     "deepseek",
			provider: "deepseek",
			baseURL:  "https://api.deepseek.com/v1",
			modelID:  "deepseek-chat",
			want: resolvedCompat{
				supportsStore:                               false,
				supportsDeveloperRole:                       false,
				supportsReasoningEffort:                     true,
				supportsUsageInStreaming:                    true,
				maxTokensField:                              "max_completion_tokens",
				requiresReasoningContentOnAssistantMessages: true,
				thinkingFormat:                              ai.ThinkingFormatDeepseek,
				supportsStrictMode:                          true,
				supportsLongCacheRetention:                  true,
			},
		},
		{
			name:     "xai grok",
			provider: "xai",
			baseURL:  "https://api.x.ai/v1",
			modelID:  "grok-4",
			want: resolvedCompat{
				supportsStore:              false,
				supportsDeveloperRole:      false,
				supportsReasoningEffort:    false,
				supportsUsageInStreaming:   true,
				maxTokensField:             "max_completion_tokens",
				thinkingFormat:             ai.ThinkingFormatOpenAI,
				supportsStrictMode:         true,
				supportsLongCacheRetention: true,
			},
		},
		{
			name:     "cerebras",
			provider: "cerebras",
			baseURL:  "https://api.cerebras.ai/v1",
			modelID:  "llama-3.3-70b",
			want: resolvedCompat{
				supportsStore:              false,
				supportsDeveloperRole:      false,
				supportsReasoningEffort:    true,
				supportsUsageInStreaming:   true,
				maxTokensField:             "max_completion_tokens",
				thinkingFormat:             ai.ThinkingFormatOpenAI,
				supportsStrictMode:         true,
				supportsLongCacheRetention: true,
			},
		},
		{
			name:     "cloudflare workers ai",
			provider: "cloudflare-workers-ai",
			baseURL:  "https://api.cloudflare.com/client/v4",
			modelID:  "llama-3",
			want: resolvedCompat{
				supportsStore:              false,
				supportsDeveloperRole:      false,
				supportsReasoningEffort:    true,
				supportsUsageInStreaming:   true,
				maxTokensField:             "max_completion_tokens",
				thinkingFormat:             ai.ThinkingFormatOpenAI,
				supportsStrictMode:         true,
				supportsLongCacheRetention: false,
			},
		},
		{
			name:     "cloudflare ai gateway",
			provider: "cloudflare-ai-gateway",
			baseURL:  "https://gateway.ai.cloudflare.com/v1",
			modelID:  "llama-3",
			want: resolvedCompat{
				supportsStore:              false,
				supportsDeveloperRole:      false,
				supportsReasoningEffort:    false,
				supportsUsageInStreaming:   true,
				maxTokensField:             "max_tokens",
				thinkingFormat:             ai.ThinkingFormatOpenAI,
				supportsStrictMode:         false,
				supportsLongCacheRetention: false,
			},
		},
		{
			name:     "nvidia",
			provider: "nvidia",
			baseURL:  "https://integrate.api.nvidia.com/v1",
			modelID:  "llama-3.1-nemotron",
			want: resolvedCompat{
				supportsStore:              false,
				supportsDeveloperRole:      false,
				supportsReasoningEffort:    false,
				supportsUsageInStreaming:   true,
				maxTokensField:             "max_tokens",
				thinkingFormat:             ai.ThinkingFormatOpenAI,
				supportsStrictMode:         false,
				supportsLongCacheRetention: false,
			},
		},
		{
			name:     "ant-ling",
			provider: "ant-ling",
			baseURL:  "https://api.ant-ling.com/v1",
			modelID:  "Ring-2.6-1T",
			want: resolvedCompat{
				supportsStore:              false,
				supportsDeveloperRole:      false,
				supportsReasoningEffort:    false,
				supportsUsageInStreaming:   true,
				maxTokensField:             "max_tokens",
				thinkingFormat:             ai.ThinkingFormatAntLing,
				supportsStrictMode:         true,
				supportsLongCacheRetention: false,
			},
		},
		{
			name:     "opencode",
			provider: "opencode",
			baseURL:  "https://opencode.ai/v1",
			modelID:  "grok-build-0.1",
			want: resolvedCompat{
				supportsStore:              false,
				supportsDeveloperRole:      false,
				supportsReasoningEffort:    true,
				supportsUsageInStreaming:   true,
				maxTokensField:             "max_completion_tokens",
				thinkingFormat:             ai.ThinkingFormatOpenAI,
				supportsStrictMode:         true,
				supportsLongCacheRetention: true,
			},
		},
		{
			name:     "openrouter generic model",
			provider: "openrouter",
			baseURL:  "https://openrouter.ai/api/v1",
			modelID:  "meta-llama/llama-3.3-70b",
			want: resolvedCompat{
				supportsStore:              true,
				supportsDeveloperRole:      false,
				supportsReasoningEffort:    true,
				supportsUsageInStreaming:   true,
				maxTokensField:             "max_completion_tokens",
				thinkingFormat:             ai.ThinkingFormatOpenRouter,
				supportsStrictMode:         true,
				supportsLongCacheRetention: true,
			},
		},
		{
			name:     "openrouter anthropic-prefixed model gets developer role and anthropic cache format",
			provider: "openrouter",
			baseURL:  "https://openrouter.ai/api/v1",
			modelID:  "anthropic/claude-sonnet-4.5",
			want: resolvedCompat{
				supportsStore:              true,
				supportsDeveloperRole:      true,
				supportsReasoningEffort:    true,
				supportsUsageInStreaming:   true,
				maxTokensField:             "max_completion_tokens",
				thinkingFormat:             ai.ThinkingFormatOpenRouter,
				cacheControlFormat:         "anthropic",
				supportsStrictMode:         true,
				supportsLongCacheRetention: true,
			},
		},
		{
			name:     "openrouter openai-prefixed model gets developer role but no cache format",
			provider: "openrouter",
			baseURL:  "https://openrouter.ai/api/v1",
			modelID:  "openai/gpt-5.2-codex",
			want: resolvedCompat{
				supportsStore:              true,
				supportsDeveloperRole:      true,
				supportsReasoningEffort:    true,
				supportsUsageInStreaming:   true,
				maxTokensField:             "max_completion_tokens",
				thinkingFormat:             ai.ThinkingFormatOpenRouter,
				supportsStrictMode:         true,
				supportsLongCacheRetention: true,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := compatModel(tc.provider, tc.baseURL, tc.modelID)
			got := detectCompat(model)
			if got.supportsStore != tc.want.supportsStore {
				t.Errorf("supportsStore = %v, want %v", got.supportsStore, tc.want.supportsStore)
			}
			if got.supportsDeveloperRole != tc.want.supportsDeveloperRole {
				t.Errorf("supportsDeveloperRole = %v, want %v", got.supportsDeveloperRole, tc.want.supportsDeveloperRole)
			}
			if got.supportsReasoningEffort != tc.want.supportsReasoningEffort {
				t.Errorf("supportsReasoningEffort = %v, want %v", got.supportsReasoningEffort, tc.want.supportsReasoningEffort)
			}
			if got.supportsUsageInStreaming != tc.want.supportsUsageInStreaming {
				t.Errorf("supportsUsageInStreaming = %v, want %v", got.supportsUsageInStreaming, tc.want.supportsUsageInStreaming)
			}
			if got.maxTokensField != tc.want.maxTokensField {
				t.Errorf("maxTokensField = %q, want %q", got.maxTokensField, tc.want.maxTokensField)
			}
			if got.requiresReasoningContentOnAssistantMessages != tc.want.requiresReasoningContentOnAssistantMessages {
				t.Errorf("requiresReasoningContentOnAssistantMessages = %v, want %v",
					got.requiresReasoningContentOnAssistantMessages, tc.want.requiresReasoningContentOnAssistantMessages)
			}
			if got.thinkingFormat != tc.want.thinkingFormat {
				t.Errorf("thinkingFormat = %q, want %q", got.thinkingFormat, tc.want.thinkingFormat)
			}
			if got.cacheControlFormat != tc.want.cacheControlFormat {
				t.Errorf("cacheControlFormat = %q, want %q", got.cacheControlFormat, tc.want.cacheControlFormat)
			}
			if got.supportsStrictMode != tc.want.supportsStrictMode {
				t.Errorf("supportsStrictMode = %v, want %v", got.supportsStrictMode, tc.want.supportsStrictMode)
			}
			if got.supportsLongCacheRetention != tc.want.supportsLongCacheRetention {
				t.Errorf("supportsLongCacheRetention = %v, want %v", got.supportsLongCacheRetention, tc.want.supportsLongCacheRetention)
			}
			// Constants that never vary by vendor, matching upstream's detectCompat.
			if got.requiresToolResultName != false {
				t.Errorf("requiresToolResultName = %v, want false", got.requiresToolResultName)
			}
			if got.requiresAssistantAfterToolResult != false {
				t.Errorf("requiresAssistantAfterToolResult = %v, want false", got.requiresAssistantAfterToolResult)
			}
			if got.requiresThinkingAsText != false {
				t.Errorf("requiresThinkingAsText = %v, want false", got.requiresThinkingAsText)
			}
			if got.zaiToolStream != false {
				t.Errorf("zaiToolStream = %v, want false", got.zaiToolStream)
			}
			if got.sendSessionAffinityHeaders != false {
				t.Errorf("sendSessionAffinityHeaders = %v, want false", got.sendSessionAffinityHeaders)
			}
		})
	}
}

// TestGetCompat_NilModelCompatReturnsDetected verifies getCompat returns the
// auto-detected defaults verbatim when the model has no explicit Compat.
func TestGetCompat_NilModelCompatReturnsDetected(t *testing.T) {
	model := compatModel("together", "https://api.together.xyz/v1", "meta-llama/Llama-3")
	got := getCompat(model)
	want := detectCompat(model)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("getCompat() = %#v, want detectCompat() = %#v", got, want)
	}
}

// TestGetCompat_ExplicitOverridesWinOverAutoDetection ports upstream's
// "explicit model.compat entries override these detected values" contract
// from getCompat's doc comment.
func TestGetCompat_ExplicitOverridesWinOverAutoDetection(t *testing.T) {
	model := compatModel("together", "https://api.together.xyz/v1", "meta-llama/Llama-3")
	trueVal := true
	model.Compat = &ai.Compat{
		SupportsStore:      &trueVal,
		MaxTokensField:     "max_completion_tokens",
		SupportsStrictMode: &trueVal,
	}

	got := getCompat(model)
	if !got.supportsStore {
		t.Errorf("supportsStore = %v, want true (explicit override)", got.supportsStore)
	}
	if got.maxTokensField != "max_completion_tokens" {
		t.Errorf("maxTokensField = %q, want max_completion_tokens (explicit override)", got.maxTokensField)
	}
	if !got.supportsStrictMode {
		t.Errorf("supportsStrictMode = %v, want true (explicit override)", got.supportsStrictMode)
	}
	// Fields with no override still come from auto-detection.
	if got.supportsDeveloperRole {
		t.Errorf("supportsDeveloperRole = %v, want false (untouched auto-detection)", got.supportsDeveloperRole)
	}
	if got.thinkingFormat != ai.ThinkingFormatTogether {
		t.Errorf("thinkingFormat = %q, want together (untouched auto-detection)", got.thinkingFormat)
	}
}

// TestGetCompat_ExplicitFalseOverridesAutoDetectedTrue verifies a *bool
// override can flip an auto-detected true to false (not just false-to-true),
// i.e. the merge is a real tri-state override, not an OR.
func TestGetCompat_ExplicitFalseOverridesAutoDetectedTrue(t *testing.T) {
	model := compatModel("openai", "https://api.openai.com/v1", "gpt-4o-mini")
	falseVal := false
	model.Compat = &ai.Compat{SupportsStore: &falseVal}

	got := getCompat(model)
	if got.supportsStore {
		t.Errorf("supportsStore = %v, want false (explicit override of auto-detected true)", got.supportsStore)
	}
}
