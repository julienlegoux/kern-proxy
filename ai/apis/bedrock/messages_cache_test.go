package bedrock

// Ports the cache-point assertions from test/bedrock-thinking-payload.test.ts's
// "Application inference profile support" describe block: system prompts and
// the last user message get a cachePoint appended for supported Claude
// models when caching is enabled.

import (
	"testing"

	"github.com/kern-ia/kern-link/ai"
)

func TestBuildSystemPrompt_AppendsCachePointForSupportedClaudeModel(t *testing.T) {
	blocks := buildSystemPrompt("You are helpful.", testModel(), ai.CacheRetentionShort, nil)
	if len(blocks) != 2 {
		t.Fatalf("len(blocks) = %d, want 2", len(blocks))
	}
	if blocks[0].Text != "You are helpful." {
		t.Errorf("blocks[0].Text = %q", blocks[0].Text)
	}
	if blocks[1].CachePoint == nil || blocks[1].CachePoint.Type != "default" {
		t.Errorf("blocks[1] = %+v, want a default cachePoint", blocks[1])
	}
}

func TestBuildSystemPrompt_OmitsCachePointWhenRetentionNone(t *testing.T) {
	blocks := buildSystemPrompt("You are helpful.", testModel(), ai.CacheRetentionNone, nil)
	if len(blocks) != 1 {
		t.Fatalf("len(blocks) = %d, want 1 (no cache point)", len(blocks))
	}
}

func TestBuildSystemPrompt_LongRetentionSetsOneHourTTL(t *testing.T) {
	blocks := buildSystemPrompt("hi", testModel(), ai.CacheRetentionLong, nil)
	if len(blocks) != 2 || blocks[1].CachePoint == nil || blocks[1].CachePoint.TTL != "1h" {
		t.Fatalf("blocks = %+v, want a cachePoint with ttl=1h", blocks)
	}
}

func TestConvertMessages_AppendsCachePointToLastUserMessage(t *testing.T) {
	chat := ai.Context{
		SystemPrompt: "You are helpful.",
		Messages: []ai.Message{
			&ai.UserMessage{Content: ai.UserText("Hello"), Timestamp: now()},
		},
	}
	got := convertMessages(chat, testModel(), ai.CacheRetentionShort, nil)
	if len(got) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(got))
	}
	content := got[len(got)-1].Content
	last := content[len(content)-1]
	if last.CachePoint == nil {
		t.Errorf("last content block = %+v, want a cachePoint", last)
	}
}

func TestConvertMessages_NoCachePointForNonCachingModel(t *testing.T) {
	model := testModel()
	model.ID = "amazon.titan-text-express-v1"
	model.Name = "Titan Text Express"
	chat := ai.Context{Messages: []ai.Message{
		&ai.UserMessage{Content: ai.UserText("Hello"), Timestamp: now()},
	}}
	got := convertMessages(chat, model, ai.CacheRetentionShort, nil)
	last := got[len(got)-1].Content
	if last[len(last)-1].CachePoint != nil {
		t.Errorf("expected no cachePoint for non-caching model, got %+v", last[len(last)-1])
	}
}

func TestConvertToolConfig_NilWhenNoTools(t *testing.T) {
	if got := convertToolConfig(nil, "", ""); got != nil {
		t.Errorf("convertToolConfig(nil tools) = %+v, want nil", got)
	}
}

func TestConvertToolConfig_NilWhenToolChoiceNone(t *testing.T) {
	tools := []ai.Tool{{Name: "get_weather", Description: "d", Parameters: ai.JSONSchema(`{"type":"object"}`)}}
	if got := convertToolConfig(tools, "none", ""); got != nil {
		t.Errorf("convertToolConfig(toolChoice=none) = %+v, want nil", got)
	}
}

func TestConvertToolConfig_BuildsToolSpecAndAutoChoice(t *testing.T) {
	tools := []ai.Tool{{Name: "get_weather", Description: "Gets weather", Parameters: ai.JSONSchema(`{"type":"object","properties":{}}`)}}
	got := convertToolConfig(tools, "auto", "")
	if got == nil {
		t.Fatal("convertToolConfig returned nil")
	}
	if len(got.Tools) != 1 || got.Tools[0].ToolSpec.Name != "get_weather" {
		t.Errorf("tools = %+v", got.Tools)
	}
	if got.ToolChoice == nil || got.ToolChoice.Auto == nil {
		t.Errorf("toolChoice = %+v, want auto", got.ToolChoice)
	}
}

func TestConvertToolConfig_ToolChoiceFunctionOverridesToolChoice(t *testing.T) {
	tools := []ai.Tool{{Name: "get_weather", Parameters: ai.JSONSchema(`{}`)}}
	got := convertToolConfig(tools, "auto", "get_weather")
	if got.ToolChoice == nil || got.ToolChoice.Tool == nil || got.ToolChoice.Tool.Name != "get_weather" {
		t.Errorf("toolChoice = %+v, want tool:get_weather", got.ToolChoice)
	}
}
