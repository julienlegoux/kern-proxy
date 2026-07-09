package google

// Ports request building, headers, and StreamSimple's thinking-level
// resolution from google-generative-ai.ts. Upstream builds a
// GenerateContentParameters and hands it to the @google/genai SDK's
// GoogleGenAI client; this Go port builds the same JSON shape and POSTs it
// directly to the Generative Language REST API
// (`{baseUrl}/v1beta/models/{model}:streamGenerateContent?alt=sse`, auth via
// the `x-goog-api-key` header) since there is no Go SDK to delegate to (see
// PORTING.md's "Vendor SDKs" deviation).
//
// Ports: packages/ai/src/api/google-generative-ai.ts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis"
	"github.com/julienlegoux/kern-proxy/ai/apis/internal/httpretry"
)

// Stream implements ai.StreamFunc for the google-generative-ai wire protocol.
func Stream(ctx context.Context, model *ai.Model, chat ai.Context, opts *ai.StreamOptions) *ai.Stream {
	out := ai.NewStream()
	go run(ctx, out, model, chat, opts)
	return out
}

// StreamSimple implements ai.SimpleStreamFunc, translating the abstract
// Reasoning level into either a Gemini 3 discrete thinking level or a
// token-budget-based thinking config, matching the model family. Ports the
// streamSimple half of google-generative-ai.ts.
func StreamSimple(ctx context.Context, model *ai.Model, chat ai.Context, opts *ai.SimpleStreamOptions) *ai.Stream {
	apiKey := ""
	if opts != nil {
		apiKey = opts.APIKey
	}
	base := apis.BuildBaseOptions(model, chat, opts, apiKey)

	if opts == nil || opts.Reasoning == "" {
		off := false
		base.ThinkingEnabled = &off
		return Stream(ctx, model, chat, &base)
	}

	clamped := ai.ClampThinkingLevel(model, opts.Reasoning)
	effort := clamped
	if effort == ai.ThinkingOff {
		effort = ai.ThinkingHigh
	}
	on := true
	base.ThinkingEnabled = &on

	if isGemini3ProModel(model) || isGemini3FlashModel(model) || isGemma4Model(model) {
		base.GoogleThinkingLevel = getThinkingLevel(effort, model)
		return Stream(ctx, model, chat, &base)
	}

	budget := getGoogleBudget(model, effort, opts.ThinkingBudgets)
	base.ThinkingBudgetTokens = &budget
	return Stream(ctx, model, chat, &base)
}

func run(ctx context.Context, out *ai.Stream, model *ai.Model, chat ai.Context, opts *ai.StreamOptions) {
	output := &ai.AssistantMessage{
		Content:    []ai.AssistantContentPart{},
		Api:        model.Api,
		Provider:   model.Provider,
		Model:      model.ID,
		StopReason: ai.StopReasonStop,
		Timestamp:  time.Now().UnixMilli(),
	}

	fail := func(err error) {
		reason := ai.StopReasonError
		if ctx.Err() != nil {
			reason = ai.StopReasonAborted
		}
		output.StopReason = reason
		output.ErrorMessage = err.Error()
		out.Push(ai.ErrorEvent{Reason: reason, Error: output})
	}

	apiKey := ""
	var headers ai.ProviderHeaders
	if opts != nil {
		apiKey = opts.APIKey
		headers = opts.Headers
	}
	if err := assertRequestAuth(model.Provider, apiKey, headers); err != nil {
		fail(err)
		return
	}

	params := buildParams(model, chat, opts)
	var payload any = params
	if opts != nil && opts.OnPayload != nil {
		next, err := opts.OnPayload(ctx, payload, model)
		if err != nil {
			fail(err)
			return
		}
		if next != nil {
			payload = next
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		fail(err)
		return
	}

	resp, err := httpretry.Do(ctx, httpretry.Request{
		URL:     requestURL(model),
		Body:    body,
		Headers: buildHeaders(model, opts, apiKey),
	}, httpretry.Config{
		Opts:              opts,
		Model:             model,
		DefaultMaxRetries: httpretry.DefaultMaxRetries,
		ParseError:        statusError,
	})
	if err != nil {
		fail(err)
		return
	}
	defer resp.Body.Close()

	out.Push(ai.StartEvent{Partial: output.Clone()})

	if err := DecodeStream(out, output, model, resp.Body); err != nil {
		fail(err)
		return
	}

	if ctx.Err() != nil {
		fail(errors.New("Request was aborted"))
		return
	}
	if output.StopReason == ai.StopReasonAborted || output.StopReason == ai.StopReasonError {
		msg := output.ErrorMessage
		if msg == "" {
			msg = "An unknown error occurred"
		}
		fail(errors.New(msg))
		return
	}

	out.Push(ai.DoneEvent{Reason: output.StopReason, Message: output})
}

// assertRequestAuth mirrors the sibling anthropic/openairesponses packages'
// helper of the same name: an empty apiKey is only acceptable when the
// caller supplied their own auth header.
func assertRequestAuth(provider, apiKey string, headers ai.ProviderHeaders) error {
	if apiKey != "" {
		return nil
	}
	if hasHeader(headers, "x-goog-api-key") || hasHeader(headers, "authorization") {
		return nil
	}
	return fmt.Errorf("No API key for provider: %s", provider)
}

func hasHeader(headers ai.ProviderHeaders, name string) bool {
	if headers == nil {
		return false
	}
	lower := strings.ToLower(name)
	for k, v := range headers {
		if strings.ToLower(k) == lower && v != nil && strings.TrimSpace(*v) != "" {
			return true
		}
	}
	return false
}

// statusError composes an error for a non-2xx response, matching the
// anthropic/openairesponses packages' approach (see anthropic.go's
// httpStatusError doc comment for the full rationale).
func statusError(status int, _, body string) error {
	trimmed := bytes.TrimSpace([]byte(body))
	if len(trimmed) == 0 {
		return fmt.Errorf("%d status code (no body)", status)
	}
	var compact bytes.Buffer
	if json.Compact(&compact, trimmed) == nil {
		return fmt.Errorf("%d %s", status, compact.String())
	}
	return fmt.Errorf("%d %s", status, string(trimmed))
}

// --- request building --------------------------------------------------------

// wireFunctionCallingConfig is the Gemini `toolConfig.functionCallingConfig`
// shape.
type wireFunctionCallingConfig struct {
	Mode string `json:"mode"`
}

type wireToolConfig struct {
	FunctionCallingConfig wireFunctionCallingConfig `json:"functionCallingConfig"`
}

// wireThinkingConfig is the Gemini `generationConfig.thinkingConfig` shape:
// either a discrete ThinkingLevel (Gemini 3) or a token ThinkingBudget
// (Gemini 2.x), never both.
type wireThinkingConfig struct {
	IncludeThoughts bool   `json:"includeThoughts,omitempty"`
	ThinkingLevel   string `json:"thinkingLevel,omitempty"`
	ThinkingBudget  *int   `json:"thinkingBudget,omitempty"`
}

// wireGenerationConfig is the Gemini `generationConfig` object: sampling
// params plus the thinking config, nested under the request's
// `generationConfig` key.
type wireGenerationConfig struct {
	Temperature     *float64            `json:"temperature,omitempty"`
	MaxOutputTokens *int                `json:"maxOutputTokens,omitempty"`
	ThinkingConfig  *wireThinkingConfig `json:"thinkingConfig,omitempty"`
}

// wireRequest is the Gemini `GenerateContentRequest` body. Ports the
// {model, contents, config} split of @google/genai's GenerateContentParameters:
// model travels in the URL path instead (see requestURL), and config's
// sub-fields map onto their REST siblings -- systemInstruction/tools/
// toolConfig sit alongside contents at the top level, while temperature/
// maxOutputTokens/thinkingConfig nest under generationConfig.
type wireRequest struct {
	// Model is not part of the wire body (Gemini takes it from the URL
	// path); kept here purely so tests and OnPayload hooks can see which
	// model this request targets.
	Model             string                `json:"-"`
	Contents          []GoogleContent       `json:"contents"`
	SystemInstruction *GoogleContent        `json:"systemInstruction,omitempty"`
	Tools             []GoogleToolGroup     `json:"tools,omitempty"`
	ToolConfig        *wireToolConfig       `json:"toolConfig,omitempty"`
	GenerationConfig  *wireGenerationConfig `json:"generationConfig,omitempty"`
}

func buildParams(model *ai.Model, chat ai.Context, opts *ai.StreamOptions) *wireRequest {
	req := &wireRequest{Model: model.ID, Contents: ConvertMessages(model, chat)}

	gc := wireGenerationConfig{}
	var toolChoice string
	if opts != nil {
		if opts.Temperature != nil {
			t := *opts.Temperature
			gc.Temperature = &t
		}
		if opts.MaxTokens != nil {
			m := *opts.MaxTokens
			gc.MaxOutputTokens = &m
		}
		toolChoice = opts.GoogleToolChoice
	}

	if chat.SystemPrompt != "" {
		req.SystemInstruction = &GoogleContent{Parts: []map[string]any{{"text": ai.SanitizeSurrogates(chat.SystemPrompt)}}}
	}

	if len(chat.Tools) > 0 {
		req.Tools = ConvertTools(chat.Tools, false)
		if toolChoice != "" {
			req.ToolConfig = &wireToolConfig{FunctionCallingConfig: wireFunctionCallingConfig{Mode: MapToolChoice(toolChoice)}}
		}
	}

	if model.Reasoning && opts != nil {
		thinkingEnabled := opts.ThinkingEnabled != nil && *opts.ThinkingEnabled
		if thinkingEnabled {
			tc := &wireThinkingConfig{IncludeThoughts: true}
			if opts.GoogleThinkingLevel != "" {
				tc.ThinkingLevel = string(opts.GoogleThinkingLevel)
			} else if opts.ThinkingBudgetTokens != nil {
				tc.ThinkingBudget = opts.ThinkingBudgetTokens
			}
			gc.ThinkingConfig = tc
		} else if opts.ThinkingEnabled != nil && !*opts.ThinkingEnabled {
			gc.ThinkingConfig = disabledThinkingConfig(model)
		}
	}

	if gc.Temperature != nil || gc.MaxOutputTokens != nil || gc.ThinkingConfig != nil {
		req.GenerationConfig = &gc
	}

	return req
}

// disabledThinkingConfig ports getDisabledThinkingConfig: Gemini 3.1 Pro
// cannot disable thinking at all, and Gemini 3 Flash/Flash-Lite/Gemma 4 don't
// support a full thinking-off either, so those use the lowest supported
// thinkingLevel without includeThoughts (hidden thinking stays invisible).
// Gemini 2.x supports disabling via thinkingBudget=0.
func disabledThinkingConfig(model *ai.Model) *wireThinkingConfig {
	if isGemini3ProModel(model) {
		return &wireThinkingConfig{ThinkingLevel: "LOW"}
	}
	if isGemini3FlashModel(model) {
		return &wireThinkingConfig{ThinkingLevel: "MINIMAL"}
	}
	if isGemma4Model(model) {
		return &wireThinkingConfig{ThinkingLevel: "MINIMAL"}
	}
	zero := 0
	return &wireThinkingConfig{ThinkingBudget: &zero}
}

var (
	gemma4Pattern       = regexp.MustCompile(`gemma-?4`)
	gemini3ProPattern   = regexp.MustCompile(`gemini-3(?:\.\d+)?-pro`)
	gemini3FlashPattern = regexp.MustCompile(`gemini-3(?:\.\d+)?-flash`)
)

func isGemma4Model(model *ai.Model) bool {
	return gemma4Pattern.MatchString(strings.ToLower(model.ID))
}

func isGemini3ProModel(model *ai.Model) bool {
	return gemini3ProPattern.MatchString(strings.ToLower(model.ID))
}

func isGemini3FlashModel(model *ai.Model) bool {
	id := strings.ToLower(model.ID)
	return gemini3FlashPattern.MatchString(id) || id == "gemini-flash-latest" || id == "gemini-flash-lite-latest"
}

// getThinkingLevel maps an abstract effort to a GoogleThinkingLevel for
// Gemini 3 Pro / Gemma 4 models (which only support LOW/HIGH), or the direct
// 1:1 mapping for everything else. Ports getThinkingLevel from
// google-generative-ai.ts.
func getThinkingLevel(effort ai.ThinkingLevel, model *ai.Model) ai.GoogleThinkingLevel {
	if isGemini3ProModel(model) || isGemma4Model(model) {
		switch effort {
		case ai.ThinkingMinimal, ai.ThinkingLow:
			return ai.GoogleThinkingLevelLow
		default:
			return ai.GoogleThinkingLevelHigh
		}
	}
	switch effort {
	case ai.ThinkingMinimal:
		return ai.GoogleThinkingLevelMinimal
	case ai.ThinkingLow:
		return ai.GoogleThinkingLevelLow
	case ai.ThinkingMedium:
		return ai.GoogleThinkingLevelMedium
	default:
		return ai.GoogleThinkingLevelHigh
	}
}

// getGoogleBudget resolves the token thinking budget for token-budget-based
// models (everything except Gemini 3 Pro/Flash and Gemma 4), honoring a
// caller-supplied override before falling back to the model-specific budget
// table (2.5-pro / 2.5-flash-lite / 2.5-flash); unrecognized models get -1
// (dynamic budget). Ports getGoogleBudget from google-generative-ai.ts.
func getGoogleBudget(model *ai.Model, effort ai.ThinkingLevel, custom *ai.ThinkingBudgets) int {
	if custom != nil {
		if v := budgetOverride(custom, effort); v != nil {
			return *v
		}
	}

	id := model.ID
	switch {
	case strings.Contains(id, "2.5-pro"):
		return budgetTable(effort, 128, 2048, 8192, 32768)
	case strings.Contains(id, "2.5-flash-lite"):
		return budgetTable(effort, 512, 2048, 8192, 24576)
	case strings.Contains(id, "2.5-flash"):
		return budgetTable(effort, 128, 2048, 8192, 24576)
	default:
		return -1
	}
}

func budgetOverride(custom *ai.ThinkingBudgets, effort ai.ThinkingLevel) *int {
	switch effort {
	case ai.ThinkingMinimal:
		return custom.Minimal
	case ai.ThinkingLow:
		return custom.Low
	case ai.ThinkingMedium:
		return custom.Medium
	case ai.ThinkingHigh:
		return custom.High
	default:
		return nil
	}
}

func budgetTable(effort ai.ThinkingLevel, minimal, low, medium, high int) int {
	switch effort {
	case ai.ThinkingMinimal:
		return minimal
	case ai.ThinkingLow:
		return low
	case ai.ThinkingMedium:
		return medium
	default:
		return high
	}
}

// requestURL builds the streamGenerateContent SSE endpoint. Ports the
// baseUrl-aware client construction in google-generative-ai.ts's
// createClient (there, the SDK appends its own path; here the full REST path
// is built directly since there is no SDK).
func requestURL(model *ai.Model) string {
	base := strings.TrimRight(model.BaseURL, "/")
	if base == "" {
		base = "https://generativelanguage.googleapis.com"
	}
	// The embedded catalog / provider base URL already carries the "/v1beta"
	// segment that this function appends below; strip a trailing one so the
	// path is not doubled into ".../v1beta/v1beta/models/..." (which 404s).
	base = strings.TrimSuffix(base, "/v1beta")
	return fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse", base, model.ID)
}

// --- headers -------------------------------------------------------------

// buildHeaders builds the request headers, including Gemini's own API-key
// header. Ports the header-building half of createClient from
// google-generative-ai.ts.
func buildHeaders(model *ai.Model, opts *ai.StreamOptions, apiKey string) map[string]string {
	defaults := map[string]string{
		"content-type": "application/json",
		"accept":       "text/event-stream",
	}
	if apiKey != "" {
		defaults["x-goog-api-key"] = apiKey
	}
	for k, v := range model.Headers {
		defaults[k] = v
	}
	var optHeaders ai.ProviderHeaders
	if opts != nil {
		optHeaders = opts.Headers
	}
	return ai.MergeProviderHeaders(defaults, optHeaders)
}
