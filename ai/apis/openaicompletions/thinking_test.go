package openaicompletions

// New Go tests (not literal upstream ports): upstream has no dedicated test
// file exercising each of the 10 thinkingFormat request-encode branches in
// isolation (they are reachable only via hand-authored catalog entries, an
// Epic 11 concern here) — see compat_test.go's analogous note. These tests
// port the *logic* of each branch in openai-completions.ts's buildParams
// (the `compat.thinkingFormat === "..."` chain) into one fixture-locked case
// per format, asserting the resulting wire-request shape directly.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/julienlegoux/kern-link/ai"
)

func reasoningModel(thinkingFormat ai.ThinkingFormat, thinkingLevelMap ai.ThinkingLevelMap, supportsReasoningEffort *bool) *ai.Model {
	return &ai.Model{
		ID:               "reasoning-model",
		Api:              ai.ApiOpenAICompletions,
		Provider:         "custom-vendor",
		BaseURL:          "https://example.invalid",
		Reasoning:        true,
		ThinkingLevelMap: thinkingLevelMap,
		Input:            []ai.Modality{ai.ModalityText},
		ContextWindow:    32000,
		MaxTokens:        4096,
		Compat: &ai.Compat{
			ThinkingFormat:          thinkingFormat,
			SupportsReasoningEffort: supportsReasoningEffort,
		},
	}
}

func strp(s string) *string { return &s }

func simpleChat() ai.Context {
	return ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
}

func TestBuildParams_ThinkingFormats(t *testing.T) {
	cases := []struct {
		name            string
		format          ai.ThinkingFormat
		levelMap        ai.ThinkingLevelMap
		reasoningEffort ai.ThinkingLevel
		supportsEffort  *bool
		check           func(t *testing.T, req *wireRequest)
	}{
		{
			name:            "openai: raw fallback reasoning_effort when unmapped",
			format:          ai.ThinkingFormatOpenAI,
			reasoningEffort: ai.ThinkingHigh,
			supportsEffort:  boolp(true),
			check: func(t *testing.T, req *wireRequest) {
				if req.ReasoningEffort != "high" {
					t.Errorf("reasoning_effort = %q, want high", req.ReasoningEffort)
				}
			},
		},
		{
			name:           "openai: explicit off mapping sent when reasoning off",
			format:         ai.ThinkingFormatOpenAI,
			levelMap:       ai.ThinkingLevelMap{ai.ThinkingOff: strp("none")},
			supportsEffort: boolp(true),
			check: func(t *testing.T, req *wireRequest) {
				if req.ReasoningEffort != "none" {
					t.Errorf("reasoning_effort = %q, want none", req.ReasoningEffort)
				}
			},
		},
		{
			name:           "openai: no off mapping means reasoning_effort omitted",
			format:         ai.ThinkingFormatOpenAI,
			supportsEffort: boolp(true),
			check: func(t *testing.T, req *wireRequest) {
				if req.ReasoningEffort != "" {
					t.Errorf("reasoning_effort = %q, want omitted", req.ReasoningEffort)
				}
			},
		},
		{
			name:            "zai: enabled sends thinking.type=enabled with clear_thinking=false",
			format:          ai.ThinkingFormatZai,
			reasoningEffort: ai.ThinkingLow,
			supportsEffort:  boolp(true),
			check: func(t *testing.T, req *wireRequest) {
				th, ok := req.Thinking.(wireThinkingObj)
				if !ok || th.Type != "enabled" || th.ClearThinking == nil || *th.ClearThinking != false {
					t.Errorf("thinking = %#v, want {enabled, clear_thinking:false}", req.Thinking)
				}
				if req.ReasoningEffort != "low" {
					t.Errorf("reasoning_effort = %q, want low", req.ReasoningEffort)
				}
			},
		},
		{
			name:   "zai: disabled sends thinking.type=disabled with no clear_thinking",
			format: ai.ThinkingFormatZai,
			check: func(t *testing.T, req *wireRequest) {
				th, ok := req.Thinking.(wireThinkingObj)
				if !ok || th.Type != "disabled" || th.ClearThinking != nil {
					t.Errorf("thinking = %#v, want {disabled} with no clear_thinking", req.Thinking)
				}
			},
		},
		{
			name:            "qwen: enable_thinking true when effort requested",
			format:          ai.ThinkingFormatQwen,
			reasoningEffort: ai.ThinkingMedium,
			check: func(t *testing.T, req *wireRequest) {
				if req.EnableThinking == nil || !*req.EnableThinking {
					t.Errorf("enable_thinking = %v, want true", req.EnableThinking)
				}
			},
		},
		{
			name:   "qwen: enable_thinking false (still present) when no effort",
			format: ai.ThinkingFormatQwen,
			check: func(t *testing.T, req *wireRequest) {
				if req.EnableThinking == nil || *req.EnableThinking {
					t.Errorf("enable_thinking = %v, want explicit false", req.EnableThinking)
				}
			},
		},
		{
			name:            "qwen-chat-template: enable_thinking + preserve_thinking kwargs",
			format:          ai.ThinkingFormatQwenChatTemplate,
			reasoningEffort: ai.ThinkingHigh,
			check: func(t *testing.T, req *wireRequest) {
				want := map[string]any{"enable_thinking": true, "preserve_thinking": true}
				if len(req.ChatTemplateKwargs) != len(want) {
					t.Fatalf("chat_template_kwargs = %#v, want %#v", req.ChatTemplateKwargs, want)
				}
				for k, v := range want {
					if req.ChatTemplateKwargs[k] != v {
						t.Errorf("chat_template_kwargs[%q] = %#v, want %#v", k, req.ChatTemplateKwargs[k], v)
					}
				}
			},
		},
		{
			name:            "deepseek: enabled thinking plus mapped reasoning_effort",
			format:          ai.ThinkingFormatDeepseek,
			levelMap:        ai.ThinkingLevelMap{ai.ThinkingHigh: strp("deep-high")},
			reasoningEffort: ai.ThinkingHigh,
			supportsEffort:  boolp(true),
			check: func(t *testing.T, req *wireRequest) {
				th, ok := req.Thinking.(wireThinkingObj)
				if !ok || th.Type != "enabled" {
					t.Errorf("thinking = %#v, want {enabled}", req.Thinking)
				}
				if req.ReasoningEffort != "deep-high" {
					t.Errorf("reasoning_effort = %q, want deep-high (mapped)", req.ReasoningEffort)
				}
			},
		},
		{
			name:     "deepseek: explicit null off mapping omits thinking entirely",
			format:   ai.ThinkingFormatDeepseek,
			levelMap: ai.ThinkingLevelMap{ai.ThinkingOff: nil},
			check: func(t *testing.T, req *wireRequest) {
				if req.Thinking != nil {
					t.Errorf("thinking = %#v, want omitted (off explicitly unsupported)", req.Thinking)
				}
			},
		},
		{
			name:   "deepseek: no off mapping sends thinking.type=disabled",
			format: ai.ThinkingFormatDeepseek,
			check: func(t *testing.T, req *wireRequest) {
				th, ok := req.Thinking.(wireThinkingObj)
				if !ok || th.Type != "disabled" {
					t.Errorf("thinking = %#v, want {disabled}", req.Thinking)
				}
			},
		},
		{
			name:            "openrouter: reasoning.effort mapped when effort requested",
			format:          ai.ThinkingFormatOpenRouter,
			levelMap:        ai.ThinkingLevelMap{ai.ThinkingLow: strp("or-low")},
			reasoningEffort: ai.ThinkingLow,
			check: func(t *testing.T, req *wireRequest) {
				r, ok := req.Reasoning.(wireReasoningEffort)
				if !ok || r.Effort != "or-low" {
					t.Errorf("reasoning = %#v, want {effort: or-low}", req.Reasoning)
				}
			},
		},
		{
			name:   "openrouter: defaults reasoning.effort to none when off unmapped",
			format: ai.ThinkingFormatOpenRouter,
			check: func(t *testing.T, req *wireRequest) {
				r, ok := req.Reasoning.(wireReasoningEffort)
				if !ok || r.Effort != "none" {
					t.Errorf("reasoning = %#v, want {effort: none}", req.Reasoning)
				}
			},
		},
		{
			name:     "openrouter: explicit null off mapping omits reasoning",
			format:   ai.ThinkingFormatOpenRouter,
			levelMap: ai.ThinkingLevelMap{ai.ThinkingOff: nil},
			check: func(t *testing.T, req *wireRequest) {
				if req.Reasoning != nil {
					t.Errorf("reasoning = %#v, want omitted", req.Reasoning)
				}
			},
		},
		{
			name:            "ant-ling: reasoning.effort only sent when explicitly mapped",
			format:          ai.ThinkingFormatAntLing,
			levelMap:        ai.ThinkingLevelMap{ai.ThinkingHigh: strp("al-high")},
			reasoningEffort: ai.ThinkingHigh,
			check: func(t *testing.T, req *wireRequest) {
				r, ok := req.Reasoning.(wireReasoningEffort)
				if !ok || r.Effort != "al-high" {
					t.Errorf("reasoning = %#v, want {effort: al-high}", req.Reasoning)
				}
			},
		},
		{
			name:            "ant-ling: omits reasoning entirely when no explicit mapping (no raw fallback)",
			format:          ai.ThinkingFormatAntLing,
			reasoningEffort: ai.ThinkingHigh,
			check: func(t *testing.T, req *wireRequest) {
				if req.Reasoning != nil {
					t.Errorf("reasoning = %#v, want omitted (unmapped ant-ling has no raw fallback)", req.Reasoning)
				}
			},
		},
		{
			name:            "together: reasoning.enabled true plus reasoning_effort",
			format:          ai.ThinkingFormatTogether,
			reasoningEffort: ai.ThinkingMedium,
			supportsEffort:  boolp(true),
			check: func(t *testing.T, req *wireRequest) {
				r, ok := req.Reasoning.(wireReasoningEnabled)
				if !ok || !r.Enabled {
					t.Errorf("reasoning = %#v, want {enabled: true}", req.Reasoning)
				}
				if req.ReasoningEffort != "medium" {
					t.Errorf("reasoning_effort = %q, want medium", req.ReasoningEffort)
				}
			},
		},
		{
			name:   "together: reasoning.enabled false and no reasoning_effort",
			format: ai.ThinkingFormatTogether,
			check: func(t *testing.T, req *wireRequest) {
				r, ok := req.Reasoning.(wireReasoningEnabled)
				if !ok || r.Enabled {
					t.Errorf("reasoning = %#v, want {enabled: false}", req.Reasoning)
				}
				if req.ReasoningEffort != "" {
					t.Errorf("reasoning_effort = %q, want omitted", req.ReasoningEffort)
				}
			},
		},
		{
			name:            "string-thinking: mapped string value sent when effort requested",
			format:          ai.ThinkingFormatStringThinking,
			levelMap:        ai.ThinkingLevelMap{ai.ThinkingHigh: strp("st-high")},
			reasoningEffort: ai.ThinkingHigh,
			check: func(t *testing.T, req *wireRequest) {
				s, ok := req.Thinking.(string)
				if !ok || s != "st-high" {
					t.Errorf("thinking = %#v, want st-high", req.Thinking)
				}
			},
		},
		{
			name:   "string-thinking: defaults to none when off unmapped",
			format: ai.ThinkingFormatStringThinking,
			check: func(t *testing.T, req *wireRequest) {
				s, ok := req.Thinking.(string)
				if !ok || s != "none" {
					t.Errorf("thinking = %#v, want none", req.Thinking)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := reasoningModel(tc.format, tc.levelMap, tc.supportsEffort)
			opts := &ai.StreamOptions{APIKey: "sk-test", ReasoningEffort: tc.reasoningEffort}
			req := buildParams(model, simpleChat(), opts)
			tc.check(t, req)
		})
	}
}

// TestBuildParams_ChatTemplateFormat_ResolvesConfiguredKwargs ports the
// generic "chat-template" format's per-key resolution: a `$var:
// thinking.enabled` placeholder, a plain scalar passthrough, and an
// `omitWhenOff` entry that disappears when reasoning is off.
func TestBuildParams_ChatTemplateFormat_ResolvesConfiguredKwargs(t *testing.T) {
	model := reasoningModel(ai.ThinkingFormatChatTemplate, ai.ThinkingLevelMap{ai.ThinkingHigh: strp("ct-high")}, nil)
	model.Compat.ChatTemplateKwargs = map[string]ai.ChatTemplateKwargValue{
		"enabled_flag": map[string]any{"$var": "thinking.enabled"},
		"static_value": "always-here",
		"effort_value": map[string]any{"omitWhenOff": true},
	}

	reqOn := buildParams(model, simpleChat(), &ai.StreamOptions{APIKey: "sk-test", ReasoningEffort: ai.ThinkingHigh})
	if reqOn.ChatTemplateKwargs["enabled_flag"] != true {
		t.Errorf("enabled_flag = %#v, want true", reqOn.ChatTemplateKwargs["enabled_flag"])
	}
	if reqOn.ChatTemplateKwargs["static_value"] != "always-here" {
		t.Errorf("static_value = %#v, want always-here", reqOn.ChatTemplateKwargs["static_value"])
	}
	if reqOn.ChatTemplateKwargs["effort_value"] != "ct-high" {
		t.Errorf("effort_value = %#v, want ct-high (mapped)", reqOn.ChatTemplateKwargs["effort_value"])
	}

	reqOff := buildParams(model, simpleChat(), &ai.StreamOptions{APIKey: "sk-test"})
	if reqOff.ChatTemplateKwargs["enabled_flag"] != false {
		t.Errorf("enabled_flag (off) = %#v, want false", reqOff.ChatTemplateKwargs["enabled_flag"])
	}
	if _, present := reqOff.ChatTemplateKwargs["effort_value"]; present {
		t.Errorf("effort_value (off) = %#v, want omitted (omitWhenOff)", reqOff.ChatTemplateKwargs["effort_value"])
	}
}

// TestBuildParams_ZaiToolStream_SetWhenCompatFlagAndToolsPresent ports the
// zaiToolStream compat flag: tool_stream is sent only alongside a non-empty
// tools array, and only when the flag is explicitly set.
func TestBuildParams_ZaiToolStream_SetWhenCompatFlagAndToolsPresent(t *testing.T) {
	model := reasoningModel(ai.ThinkingFormatZai, nil, nil)
	model.Compat.ZaiToolStream = boolp(true)
	chat := ai.Context{
		Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}},
		Tools:    []ai.Tool{{Name: "ping", Description: "ping", Parameters: []byte(`{"type":"object"}`)}},
	}

	req := buildParams(model, chat, &ai.StreamOptions{APIKey: "sk-test"})
	if req.ToolStream == nil || !*req.ToolStream {
		t.Errorf("tool_stream = %v, want true", req.ToolStream)
	}
}

func TestBuildParams_ZaiToolStream_OmittedWithoutTools(t *testing.T) {
	model := reasoningModel(ai.ThinkingFormatZai, nil, nil)
	model.Compat.ZaiToolStream = boolp(true)

	req := buildParams(model, simpleChat(), &ai.StreamOptions{APIKey: "sk-test"})
	if req.ToolStream != nil {
		t.Errorf("tool_stream = %v, want omitted (no tools offered)", req.ToolStream)
	}
}

// TestStreamSimple_ResolvesReasoningEffortFromClampedLevel verifies
// StreamSimple's translation of the abstract SimpleStreamOptions.Reasoning
// level into StreamOptions.ReasoningEffort via ai.ClampThinkingLevel,
// mirroring upstream streamSimple's clampThinkingLevel call.
func TestStreamSimple_ResolvesReasoningEffortFromClampedLevel(t *testing.T) {
	model := reasoningModel(ai.ThinkingFormatOpenAI, nil, boolp(true))
	model.MaxTokens = 4096

	var captured map[string]any
	capture := func(_ context.Context, payload any, _ *ai.Model) (any, error) {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		if err := json.Unmarshal(raw, &captured); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		return nil, nil
	}

	srv := sseServer(t, minimalTextChunks())
	model.BaseURL = srv.URL
	opts := &ai.SimpleStreamOptions{
		StreamOptions: ai.StreamOptions{APIKey: "sk-test", OnPayload: capture},
		Reasoning:     ai.ThinkingHigh,
	}

	stream := StreamSimple(context.Background(), model, simpleChat(), opts)
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}
	if captured["reasoning_effort"] != "high" {
		t.Errorf("reasoning_effort = %v, want high", captured["reasoning_effort"])
	}
}

// TestStreamSimple_OmitsReasoningEffortWhenNotRequested verifies the "off"
// path: an empty SimpleStreamOptions.Reasoning must not set ReasoningEffort.
func TestStreamSimple_OmitsReasoningEffortWhenNotRequested(t *testing.T) {
	model := reasoningModel(ai.ThinkingFormatOpenAI, nil, boolp(true))
	model.MaxTokens = 4096
	srv := sseServer(t, minimalTextChunks())
	model.BaseURL = srv.URL

	var captured map[string]any
	capture := func(_ context.Context, payload any, _ *ai.Model) (any, error) {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		if err := json.Unmarshal(raw, &captured); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		return nil, nil
	}
	opts := &ai.SimpleStreamOptions{StreamOptions: ai.StreamOptions{APIKey: "sk-test", OnPayload: capture}}

	stream := StreamSimple(context.Background(), model, simpleChat(), opts)
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}
	if _, present := captured["reasoning_effort"]; present {
		t.Errorf("reasoning_effort = %v, want omitted", captured["reasoning_effort"])
	}
}
