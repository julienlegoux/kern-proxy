package openaicompletions

// New Go tests (not literal upstream ports, per the same note in
// compat_test.go and openaicompletions_test.go): these verify the resolved
// compat matrix actually reaches request/message building (buildParams,
// convertMessages, convertToolResultMessage, convertTools), not just that
// detectCompat/getCompat compute the right values in isolation.

import (
	"testing"
	"time"

	"github.com/kern-ia/kern-link/ai"
)

func vendorModel(provider, baseURL string) *ai.Model {
	return &ai.Model{
		ID:            "some-model",
		Name:          "Some Model",
		Api:           ai.ApiOpenAICompletions,
		Provider:      ai.ProviderId(provider),
		BaseURL:       baseURL,
		Reasoning:     true,
		Input:         []ai.Modality{ai.ModalityText},
		ContextWindow: 32000,
		MaxTokens:     4096,
	}
}

func boolp(b bool) *bool { return &b }

// TestBuildParams_OmitsStoreForNonStandardVendor ports detectCompat's
// isNonStandard -> supportsStore:false rule into an observable wire-request
// assertion: a together.ai model must never receive `store`.
func TestBuildParams_OmitsStoreForNonStandardVendor(t *testing.T) {
	model := vendorModel("together", "https://api.together.xyz/v1")
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	if params.Store != nil {
		t.Errorf("store = %v, want omitted for a non-standard (together) vendor", params.Store)
	}
}

// TestBuildParams_UsesMaxTokensFieldForAutoDetectedVendor ports the
// useMaxTokens rule: an nvidia model must send max_tokens, not
// max_completion_tokens, with no explicit compat override needed.
func TestBuildParams_UsesMaxTokensFieldForAutoDetectedVendor(t *testing.T) {
	model := vendorModel("nvidia", "https://integrate.api.nvidia.com/v1")
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	maxTokens := 256

	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test", MaxTokens: &maxTokens})
	if params.MaxTokens == nil || *params.MaxTokens != 256 {
		t.Errorf("max_tokens = %v, want 256", params.MaxTokens)
	}
	if params.MaxCompletionTokens != nil {
		t.Errorf("max_completion_tokens = %v, want omitted (nvidia uses max_tokens)", params.MaxCompletionTokens)
	}
}

// TestBuildParams_OmitsStrictFieldForMoonshotProvider ports the
// supportsStrictMode auto-detection rule for moonshotai.
func TestBuildParams_OmitsStrictFieldForMoonshotProvider(t *testing.T) {
	model := vendorModel("moonshotai", "https://api.moonshot.cn/v1")
	chat := ai.Context{
		Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}},
		Tools: []ai.Tool{{
			Name:        "ping",
			Description: "Ping tool",
			Parameters:  []byte(`{"type":"object"}`),
		}},
	}

	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	if params.Tools == nil || len(*params.Tools) != 1 {
		t.Fatalf("tools = %#v, want 1 entry", params.Tools)
	}
	if (*params.Tools)[0].Function.Strict != nil {
		t.Errorf("strict = %v, want omitted for moonshotai", (*params.Tools)[0].Function.Strict)
	}
}

// TestBuildParams_KeepsSystemRoleForNonStandardVendorEvenWhenReasoning ports
// the supportsDeveloperRole auto-detection rule: a reasoning-capable together
// model must still use the plain "system" role (not "developer"), since
// isNonStandard vendors don't support the developer role.
func TestBuildParams_KeepsSystemRoleForNonStandardVendorEvenWhenReasoning(t *testing.T) {
	model := vendorModel("together", "https://api.together.xyz/v1")
	chat := ai.Context{
		SystemPrompt: "be helpful",
		Messages:     []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}},
	}

	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	if len(params.Messages) == 0 || params.Messages[0].Role != "system" {
		t.Fatalf("messages[0] = %#v, want role system", params.Messages[0])
	}
}

// TestBuildParams_UsesDeveloperRoleForStandardReasoningVendor is the
// contrasting case: a standard (openai) reasoning model keeps the existing
// "developer" role behavior.
func TestBuildParams_UsesDeveloperRoleForStandardReasoningVendor(t *testing.T) {
	model := vendorModel("openai", "https://api.openai.com/v1")
	chat := ai.Context{
		SystemPrompt: "be helpful",
		Messages:     []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}},
	}

	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	if len(params.Messages) == 0 || params.Messages[0].Role != "developer" {
		t.Fatalf("messages[0] = %#v, want role developer", params.Messages[0])
	}
}

// TestBuildParams_IncludesToolResultNameWhenCompatRequires ports the
// requiresToolResultName compat flag: when explicitly set, tool result
// messages must carry the "name" field.
func TestBuildParams_IncludesToolResultNameWhenCompatRequires(t *testing.T) {
	model := vendorModel("openai", "https://api.openai.com/v1")
	model.Compat = &ai.Compat{RequiresToolResultName: boolp(true)}
	chat := ai.Context{
		Messages: []ai.Message{
			&ai.UserMessage{Content: ai.UserText("use the tool"), Timestamp: time.Now().UnixMilli()},
			&ai.AssistantMessage{
				Content:    []ai.AssistantContentPart{ai.ToolCall{ID: "t1", Name: "noop", Arguments: map[string]any{}}},
				StopReason: ai.StopReasonToolUse,
				Api:        ai.ApiOpenAICompletions,
				Provider:   "openai",
				Model:      "some-model",
			},
			&ai.ToolResultMessage{ToolCallID: "t1", ToolName: "noop", Content: []ai.UserContentPart{ai.TextContent{Text: "done"}}},
		},
	}

	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	toolMsg := params.Messages[len(params.Messages)-1]
	if toolMsg.Role != "tool" || toolMsg.Name != "noop" {
		t.Errorf("tool message = %#v, want role=tool name=noop", toolMsg)
	}
}

// TestBuildParams_OmitsToolResultNameByDefault is the contrasting case: the
// flag is a constant-false default in detectCompat, so an unset compat must
// not add the name field (matching the existing pre-issue-02 behavior).
func TestBuildParams_OmitsToolResultNameByDefault(t *testing.T) {
	model := vendorModel("openai", "https://api.openai.com/v1")
	chat := ai.Context{
		Messages: []ai.Message{
			&ai.UserMessage{Content: ai.UserText("use the tool"), Timestamp: time.Now().UnixMilli()},
			&ai.AssistantMessage{
				Content:    []ai.AssistantContentPart{ai.ToolCall{ID: "t1", Name: "noop", Arguments: map[string]any{}}},
				StopReason: ai.StopReasonToolUse,
				Api:        ai.ApiOpenAICompletions,
				Provider:   "openai",
				Model:      "some-model",
			},
			&ai.ToolResultMessage{ToolCallID: "t1", ToolName: "noop", Content: []ai.UserContentPart{ai.TextContent{Text: "done"}}},
		},
	}

	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	toolMsg := params.Messages[len(params.Messages)-1]
	if toolMsg.Name != "" {
		t.Errorf("name = %q, want empty (requiresToolResultName defaults false)", toolMsg.Name)
	}
}

// TestBuildParams_InsertsSyntheticAssistantAfterToolResultWhenCompatRequires
// ports the requiresAssistantAfterToolResult compat flag: when set, a
// synthetic assistant message must bridge a tool-result -> user transition.
func TestBuildParams_InsertsSyntheticAssistantAfterToolResultWhenCompatRequires(t *testing.T) {
	model := vendorModel("openai", "https://api.openai.com/v1")
	model.Compat = &ai.Compat{RequiresAssistantAfterToolResult: boolp(true)}
	chat := ai.Context{
		Messages: []ai.Message{
			&ai.UserMessage{Content: ai.UserText("use the tool"), Timestamp: time.Now().UnixMilli()},
			&ai.AssistantMessage{
				Content:    []ai.AssistantContentPart{ai.ToolCall{ID: "t1", Name: "noop", Arguments: map[string]any{}}},
				StopReason: ai.StopReasonToolUse,
				Api:        ai.ApiOpenAICompletions,
				Provider:   "openai",
				Model:      "some-model",
			},
			&ai.ToolResultMessage{ToolCallID: "t1", ToolName: "noop", Content: []ai.UserContentPart{ai.TextContent{Text: "done"}}},
			&ai.UserMessage{Content: ai.UserText("continue"), Timestamp: time.Now().UnixMilli()},
		},
	}

	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	roles := make([]string, len(params.Messages))
	for i, m := range params.Messages {
		roles[i] = m.Role
	}
	want := []string{"user", "assistant", "tool", "assistant", "user"}
	if len(roles) != len(want) {
		t.Fatalf("roles = %#v, want %#v", roles, want)
	}
	for i := range want {
		if roles[i] != want[i] {
			t.Errorf("roles[%d] = %q, want %q (full: %#v)", i, roles[i], want[i], roles)
		}
	}
	synthetic := params.Messages[3]
	if synthetic.Content != "I have processed the tool results." {
		t.Errorf("synthetic assistant content = %#v, want the fixed upstream bridge text", synthetic.Content)
	}
}

// TestBuildParams_NoSyntheticAssistantByDefault is the contrasting case: the
// flag defaults false, so the tool-result -> user transition is untouched.
func TestBuildParams_NoSyntheticAssistantByDefault(t *testing.T) {
	model := vendorModel("openai", "https://api.openai.com/v1")
	chat := ai.Context{
		Messages: []ai.Message{
			&ai.UserMessage{Content: ai.UserText("use the tool"), Timestamp: time.Now().UnixMilli()},
			&ai.AssistantMessage{
				Content:    []ai.AssistantContentPart{ai.ToolCall{ID: "t1", Name: "noop", Arguments: map[string]any{}}},
				StopReason: ai.StopReasonToolUse,
				Api:        ai.ApiOpenAICompletions,
				Provider:   "openai",
				Model:      "some-model",
			},
			&ai.ToolResultMessage{ToolCallID: "t1", ToolName: "noop", Content: []ai.UserContentPart{ai.TextContent{Text: "done"}}},
			&ai.UserMessage{Content: ai.UserText("continue"), Timestamp: time.Now().UnixMilli()},
		},
	}

	params := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	if len(params.Messages) != 4 {
		t.Fatalf("messages = %#v, want 4 (no synthetic bridge inserted)", params.Messages)
	}
}
