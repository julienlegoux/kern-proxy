package vertex

// Ports the stream/streamSimple/buildParams/client-construction half of
// google-vertex.ts. Upstream builds a GenerateContentParameters and hands it
// to the @google/genai SDK's GoogleGenAI client (constructed either with an
// explicit apiKey or with vertexai:true + project + location, which the SDK
// resolves into a REST call under the hood); this Go port has no such SDK, so
// it builds the same JSON request shape (reusing ai/apis/google's exported
// ConvertMessages/ConvertTools/DecodeStream) and POSTs it directly to the
// Vertex AI REST API. The thinking-level/budget helpers and wireRequest
// shape below are a deliberate duplicate of the sibling ai/apis/google
// package's equivalents, mirroring upstream's own duplication between
// google-generative-ai.ts and google-vertex.ts (only google-shared.ts is
// actually shared upstream).
//
// Deviation from upstream: the exact REST path the @google/genai SDK builds
// when a custom httpOptions.baseUrl is supplied together with project/
// location (see google-vertex-api-key-resolution.test.ts's baseUrl tests) is
// an internal SDK detail this port cannot introspect (no vendored source).
// This port instead documents and tests its own, simpler contract: a real
// baseUrl override replaces the default host outright (the remainder of the
// path -- /v1[/projects/{project}/locations/{location}]/publishers/google/
// models/{model}:streamGenerateContent -- is always appended), and a
// template baseUrl still containing "{location}" is ignored in favor of the
// default host, matching the observable behavior upstream's tests assert
// (custom hosts are honored; unsubstituted catalog templates are not).
//
// Ports: packages/ai/src/api/google-vertex.ts
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis"
	"github.com/julienlegoux/kern-proxy/ai/apis/google"
)

// defaultVertexHost is the Vertex AI REST host template; {location} is
// substituted with the resolved region. Ports the @google/genai SDK's
// default (non-override) Vertex host construction.
const defaultVertexHostTemplate = "https://%s-aiplatform.googleapis.com"

// globalVertexHost is used for the explicit-API-key path, which addresses
// models by publisher only (no project/location in the path). Ports
// createClientWithApiKey's project-less client construction.
const globalVertexHost = "https://aiplatform.googleapis.com"

// Stream implements ai.StreamFunc for the google-vertex wire protocol.
func Stream(ctx context.Context, model *ai.Model, chat ai.Context, opts *ai.StreamOptions) *ai.Stream {
	out := ai.NewStream()
	go run(ctx, out, model, chat, opts)
	return out
}

// StreamSimple implements ai.SimpleStreamFunc, mirroring
// ai/apis/google's StreamSimple (same thinking-level/budget resolution;
// google-vertex.ts's streamSimple is itself a near-verbatim duplicate of
// google-generative-ai.ts's).
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

	if isGemini3ProModel(model) || isGemini3FlashModel(model) {
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

	var rawAPIKey, project, location string
	var headers ai.ProviderHeaders
	var env map[string]string
	if opts != nil {
		rawAPIKey = opts.APIKey
		project = opts.GoogleVertexProject
		location = opts.GoogleVertexLocation
		headers = opts.Headers
		env = opts.Env
	}
	apiKey := resolveAPIKey(rawAPIKey)

	var reqHeaders map[string]string
	if apiKey != "" {
		reqHeaders = buildHeaders(model, headers, apiKey)
	} else {
		resolvedProject, err := resolveProject(project, env)
		if err != nil {
			fail(err)
			return
		}
		resolvedLocation, err := resolveLocation(location, env)
		if err != nil {
			fail(err)
			return
		}
		project, location = resolvedProject, resolvedLocation

		token, err := adcTokenFunc(ctx)
		if err != nil {
			fail(err)
			return
		}
		reqHeaders = buildADCHeaders(model, headers, token)
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

	url := requestURL(model, apiKey != "", project, location)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		fail(err)
		return
	}
	for k, v := range reqHeaders {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	if opts != nil && opts.Timeout > 0 {
		client.Timeout = opts.Timeout
	}
	resp, err := client.Do(req)
	if err != nil {
		fail(err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fail(httpStatusError(resp))
		return
	}

	if opts != nil && opts.OnResponse != nil {
		respMeta := ai.ProviderResponse{Status: resp.StatusCode, Headers: ai.HeadersToRecord(resp.Header)}
		if err := opts.OnResponse(ctx, respMeta, model); err != nil {
			fail(err)
			return
		}
	}

	out.Push(ai.StartEvent{Partial: output.Clone()})

	if err := google.DecodeStream(out, output, model, resp.Body); err != nil {
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

// httpStatusError composes an error for a non-2xx response, matching the
// sibling google/anthropic/openairesponses packages' approach.
func httpStatusError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return fmt.Errorf("%d status code (no body)", resp.StatusCode)
	}
	var compact bytes.Buffer
	if json.Compact(&compact, trimmed) == nil {
		return fmt.Errorf("%d %s", resp.StatusCode, compact.String())
	}
	return fmt.Errorf("%d %s", resp.StatusCode, string(trimmed))
}

// --- request building (duplicated from ai/apis/google/google.go, matching
// upstream's own duplication between google-generative-ai.ts and
// google-vertex.ts; only convertMessages/convertTools -- ai/apis/google's
// ConvertMessages/ConvertTools -- are actually shared upstream) ------------

type wireFunctionCallingConfig struct {
	Mode string `json:"mode"`
}

type wireToolConfig struct {
	FunctionCallingConfig wireFunctionCallingConfig `json:"functionCallingConfig"`
}

type wireThinkingConfig struct {
	IncludeThoughts bool   `json:"includeThoughts,omitempty"`
	ThinkingLevel   string `json:"thinkingLevel,omitempty"`
	ThinkingBudget  *int   `json:"thinkingBudget,omitempty"`
}

type wireGenerationConfig struct {
	Temperature     *float64            `json:"temperature,omitempty"`
	MaxOutputTokens *int                `json:"maxOutputTokens,omitempty"`
	ThinkingConfig  *wireThinkingConfig `json:"thinkingConfig,omitempty"`
}

// wireRequest is the Vertex GenerateContentRequest body -- structurally
// identical to the Generative Language API's (ai/apis/google's wireRequest),
// since Vertex serves the same GenerateContentRequest/Response shape.
type wireRequest struct {
	Model             string                   `json:"-"`
	Contents          []google.GoogleContent   `json:"contents"`
	SystemInstruction *google.GoogleContent    `json:"systemInstruction,omitempty"`
	Tools             []google.GoogleToolGroup `json:"tools,omitempty"`
	ToolConfig        *wireToolConfig          `json:"toolConfig,omitempty"`
	GenerationConfig  *wireGenerationConfig    `json:"generationConfig,omitempty"`
}

func buildParams(model *ai.Model, chat ai.Context, opts *ai.StreamOptions) *wireRequest {
	req := &wireRequest{Model: model.ID, Contents: google.ConvertMessages(model, chat)}

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
		req.SystemInstruction = &google.GoogleContent{Parts: []map[string]any{{"text": ai.SanitizeSurrogates(chat.SystemPrompt)}}}
	}

	if len(chat.Tools) > 0 {
		req.Tools = google.ConvertTools(chat.Tools, false)
		if toolChoice != "" {
			req.ToolConfig = &wireToolConfig{FunctionCallingConfig: wireFunctionCallingConfig{Mode: google.MapToolChoice(toolChoice)}}
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

// disabledThinkingConfig ports getDisabledThinkingConfig from
// google-vertex.ts (a duplicate of google-generative-ai.ts's; this port
// doesn't yet distinguish a Gemma-4-on-Vertex case since Gemma isn't part of
// the Vertex catalog upstream serves).
func disabledThinkingConfig(model *ai.Model) *wireThinkingConfig {
	if isGemini3ProModel(model) {
		return &wireThinkingConfig{ThinkingLevel: "LOW"}
	}
	if isGemini3FlashModel(model) {
		return &wireThinkingConfig{ThinkingLevel: "MINIMAL"}
	}
	zero := 0
	return &wireThinkingConfig{ThinkingBudget: &zero}
}

var (
	gemini3ProPattern   = regexp.MustCompile(`gemini-3(?:\.\d+)?-pro`)
	gemini3FlashPattern = regexp.MustCompile(`gemini-3(?:\.\d+)?-flash`)
)

func isGemini3ProModel(model *ai.Model) bool {
	return gemini3ProPattern.MatchString(strings.ToLower(model.ID))
}

func isGemini3FlashModel(model *ai.Model) bool {
	id := strings.ToLower(model.ID)
	return gemini3FlashPattern.MatchString(id) || id == "gemini-flash-latest" || id == "gemini-flash-lite-latest"
}

// getThinkingLevel ports getGemini3ThinkingLevel from google-vertex.ts.
func getThinkingLevel(effort ai.ThinkingLevel, model *ai.Model) ai.GoogleThinkingLevel {
	if isGemini3ProModel(model) {
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

// getGoogleBudget ports getGoogleBudget from google-vertex.ts.
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

// --- endpoint + headers ------------------------------------------------

// requestURL builds the streamGenerateContent SSE endpoint. useAPIKey
// selects the project-less publisher path (createClientWithApiKey); the ADC
// path is project/location-scoped (createClient). See the package doc
// comment for the custom-baseUrl deviation from upstream's SDK internals.
func requestURL(model *ai.Model, useAPIKey bool, project, location string) string {
	if useAPIKey {
		host := customHost(model.BaseURL)
		if host == "" {
			host = globalVertexHost
		}
		return fmt.Sprintf("%s/v1/publishers/google/models/%s:streamGenerateContent?alt=sse", host, model.ID)
	}
	host := customHost(model.BaseURL)
	if host == "" {
		host = fmt.Sprintf(defaultVertexHostTemplate, location)
	}
	return fmt.Sprintf("%s/v1/projects/%s/locations/%s/publishers/google/models/%s:streamGenerateContent?alt=sse",
		host, project, location, model.ID)
}

// customHost returns model.BaseURL trimmed of a trailing slash when it's a
// genuine override, or "" when it's empty or still contains the catalog's
// unsubstituted "{location}" template (resolveCustomBaseUrl from
// google-vertex.ts).
func customHost(baseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" || strings.Contains(trimmed, "{location}") {
		return ""
	}
	return trimmed
}

// buildHeaders builds request headers for the explicit-API-key path. Ports
// the header-building half of createClientWithApiKey.
func buildHeaders(model *ai.Model, optHeaders ai.ProviderHeaders, apiKey string) map[string]string {
	defaults := map[string]string{
		"content-type":   "application/json",
		"accept":         "text/event-stream",
		"x-goog-api-key": apiKey,
	}
	for k, v := range model.Headers {
		defaults[k] = v
	}
	return ai.MergeProviderHeaders(defaults, optHeaders)
}

// buildADCHeaders builds request headers for the ADC path: a Bearer
// Authorization header carrying the ADC access token in place of
// x-goog-api-key. Ports the header-building half of createClient.
func buildADCHeaders(model *ai.Model, optHeaders ai.ProviderHeaders, token string) map[string]string {
	defaults := map[string]string{
		"content-type":  "application/json",
		"accept":        "text/event-stream",
		"authorization": "Bearer " + token,
	}
	for k, v := range model.Headers {
		defaults[k] = v
	}
	return ai.MergeProviderHeaders(defaults, optHeaders)
}
