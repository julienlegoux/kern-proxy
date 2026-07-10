package openairesponses

// New Go tests covering ConvertMessages/ConvertTools directly (the exported
// entry points the Azure/Codex variants, epic 7 issues 02-04, will reuse),
// beyond what buildParams' own tests already exercise indirectly.

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/julienlegoux/kern-link/ai"
)

func TestConvertTools_EmptyParametersBecomeEmptyObject(t *testing.T) {
	tools := ConvertTools([]ai.Tool{{Name: "noop", Description: "does nothing"}}, ConvertToolsOptions{})
	if len(tools) != 1 {
		t.Fatalf("tools = %#v, want 1", tools)
	}
	if tools[0].Parameters == nil {
		t.Errorf("parameters = nil, want an empty object")
	}
}

func TestConvertTools_StrictOptionAlwaysSerializes(t *testing.T) {
	tools := ConvertTools([]ai.Tool{{Name: "noop"}}, ConvertToolsOptions{Strict: true})
	raw, _ := json.Marshal(tools[0])
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	if decoded["strict"] != true {
		t.Errorf("strict = %v, want true (always serialized, never omitted)", decoded["strict"])
	}
}

// TestConvertMessages_RemapsForeignToolCallIDToSyntheticItemID covers the
// isForeignToolCall branch of normalizeToolCallId: replaying a tool call
// produced by a different provider/api gets its item id replaced by a
// deterministic "fc_"-prefixed hash rather than the foreign id verbatim.
func TestConvertMessages_RemapsForeignToolCallIDToSyntheticItemID(t *testing.T) {
	model := testModel("https://example.invalid")
	chat := ai.Context{
		Messages: []ai.Message{
			&ai.UserMessage{Content: ai.UserText("go"), Timestamp: time.Now().UnixMilli()},
			&ai.AssistantMessage{
				Content:    []ai.AssistantContentPart{ai.ToolCall{ID: "call_1|codex_item_abc", Name: "noop", Arguments: map[string]any{}}},
				StopReason: ai.StopReasonToolUse,
				Api:        ai.ApiOpenAICodexResponses,
				Provider:   "openai-codex",
				Model:      "codex-mini",
			},
			&ai.ToolResultMessage{ToolCallID: "call_1|codex_item_abc", ToolName: "noop", Content: []ai.UserContentPart{ai.TextContent{Text: "ok"}}},
		},
	}
	items := ConvertMessages(model, chat, ConvertMessagesOptions{AllowedToolCallProviders: openAIToolCallProviders})
	if len(items) != 3 {
		t.Fatalf("items = %#v, want 3 (user, function_call, function_call_output)", items)
	}
	fc, ok := items[1].(wireFunctionCall)
	if !ok {
		t.Fatalf("items[1] = %#v, want wireFunctionCall", items[1])
	}
	if fc.CallID != "call_1" {
		t.Errorf("callID = %q, want call_1", fc.CallID)
	}
	if fc.ID == "codex_item_abc" || fc.ID == "" {
		t.Errorf("id = %q, want a synthetic fc_-prefixed hash (not the foreign id verbatim, not empty)", fc.ID)
	}
	if len(fc.ID) < 3 || fc.ID[:3] != "fc_" {
		t.Errorf("id = %q, want fc_ prefix", fc.ID)
	}
	fco, ok := items[2].(wireFunctionCallOutput)
	if !ok {
		t.Fatalf("items[2] = %#v, want wireFunctionCallOutput", items[2])
	}
	if fco.CallID != "call_1" {
		t.Errorf("function_call_output callID = %q, want call_1 (remapped consistently)", fco.CallID)
	}
}

// TestConvertMessages_NonAllowedProviderNormalizesWithoutPipeSplit covers the
// !allowedToolCallProviders branch: a provider outside the allowed set gets
// its id normalized as one opaque string, not split on '|'.
func TestConvertMessages_NonAllowedProviderNormalizesWithoutPipeSplit(t *testing.T) {
	model := testModel("https://example.invalid")
	model.Provider = "groq"
	chat := ai.Context{
		Messages: []ai.Message{
			&ai.UserMessage{Content: ai.UserText("go"), Timestamp: time.Now().UnixMilli()},
			&ai.AssistantMessage{
				Content:    []ai.AssistantContentPart{ai.ToolCall{ID: "weird|id|shape", Name: "noop", Arguments: map[string]any{}}},
				StopReason: ai.StopReasonToolUse,
				Api:        ai.ApiOpenAICompletions,
				Provider:   "openai",
				Model:      "gpt-4o",
			},
			&ai.ToolResultMessage{ToolCallID: "weird|id|shape", ToolName: "noop", Content: []ai.UserContentPart{ai.TextContent{Text: "ok"}}},
		},
	}
	items := ConvertMessages(model, chat, ConvertMessagesOptions{AllowedToolCallProviders: openAIToolCallProviders})
	fc, ok := items[1].(wireFunctionCall)
	if !ok {
		t.Fatalf("items[1] = %#v, want wireFunctionCall", items[1])
	}
	if fc.CallID != "weird_id_shape" {
		t.Errorf("callID = %q, want the whole id normalized as one opaque string (pipes become underscores)", fc.CallID)
	}
}
