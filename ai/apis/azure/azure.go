// Package azure implements the Azure OpenAI variant of the Responses API
// adapter: endpoint/deployment resolution, auth, and request shaping over the
// shared openai-responses core (ai/apis/openairesponses), whose exported
// ConvertMessages/ConvertTools/DecodeStream this package reuses rather than
// duplicating message conversion or SSE decoding.
//
// Ports: packages/ai/src/api/azure-openai-responses.ts
package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	neturl "net/url"
	"strings"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis"
	"github.com/julienlegoux/kern-proxy/ai/apis/internal/httpretry"
	"github.com/julienlegoux/kern-proxy/ai/apis/openairesponses"
)

const responsesPath = "/responses"

// openAIResponsesMinOutputTokens duplicates
// openairesponses.openAIResponsesMinOutputTokens (see that file's doc
// comment on clampOpenAIPromptCacheKey for why: Go package boundaries don't
// share unexported constants the way the two TS files share one module).
const openAIResponsesMinOutputTokens = 16

// openAIPromptCacheKeyMaxLength duplicates
// openairesponses.openAIPromptCacheKeyMaxLength.
const openAIPromptCacheKeyMaxLength = 64

// azureToolCallProviders ports AZURE_TOOL_CALL_PROVIDERS.
var azureToolCallProviders = map[string]bool{
	"openai":                 true,
	"openai-codex":           true,
	"opencode":               true,
	"azure-openai-responses": true,
}

// Stream implements ai.StreamFunc for the Azure OpenAI Responses wire
// protocol.
func Stream(ctx context.Context, model *ai.Model, chat ai.Context, opts *ai.StreamOptions) *ai.Stream {
	out := ai.NewStream()
	go run(ctx, out, model, chat, opts)
	return out
}

// StreamSimple implements ai.SimpleStreamFunc. Ports the streamSimple half of
// azure-openai-responses.ts; like the sibling openairesponses/anthropic
// packages, the redundant synchronous apiKey pre-check is omitted -- Stream
// (via run's assertRequestAuth) already reports a missing key as an in-band
// error event, matching ai.StreamFunc's contract that every failure surfaces
// through the returned stream, never a Go/JS-style throw.
func StreamSimple(ctx context.Context, model *ai.Model, chat ai.Context, opts *ai.SimpleStreamOptions) *ai.Stream {
	apiKey := ""
	if opts != nil {
		apiKey = opts.APIKey
	}
	base := apis.BuildBaseOptions(model, chat, opts, apiKey)
	if opts != nil && opts.Reasoning != "" {
		if clamped := ai.ClampThinkingLevel(model, opts.Reasoning); clamped != ai.ThinkingOff {
			base.ReasoningEffort = clamped
		}
	}
	return Stream(ctx, model, chat, &base)
}

func run(ctx context.Context, out *ai.Stream, model *ai.Model, chat ai.Context, opts *ai.StreamOptions) {
	output := &ai.AssistantMessage{
		Content:    []ai.AssistantContentPart{},
		Api:        ai.ApiAzureOpenAIResponses,
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
	if opts != nil {
		apiKey = opts.APIKey
	}
	if err := assertRequestAuth(model.Provider, apiKey); err != nil {
		fail(err)
		return
	}

	baseURL, apiVersion, err := resolveAzureConfig(model, opts)
	if err != nil {
		fail(err)
		return
	}
	deploymentName := resolveDeploymentName(model, opts)

	params := buildParams(model, chat, opts, deploymentName)
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

	url := strings.TrimRight(baseURL, "/") + responsesPath
	// Azure's unversioned "v1" surface (the default) needs no api-version
	// query parameter; any other explicit version does. Neither upstream nor
	// its test suite exercises the actual outgoing request for a dated
	// version (azure-openai-base-url.test.ts only inspects the AzureOpenAI
	// SDK client's constructor arguments before the request is ever built),
	// so this follows Azure's documented REST convention rather than a
	// captured golden.
	if apiVersion != "" && apiVersion != defaultAzureAPIVersion {
		url += "?api-version=" + neturl.QueryEscape(apiVersion)
	}

	resp, err := httpretry.Do(ctx, httpretry.Request{
		URL:     url,
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

	// Azure has no service-tier concept of its own (azure-openai-responses.ts
	// never sets/reads service_tier); the zero-value ServiceTierOptions
	// leaves DecodeStream's tier multiplier at its 1x default unless the
	// response itself reports a tier.
	if err := openairesponses.DecodeStream(out, output, model, resp.Body, openairesponses.ServiceTierOptions{}); err != nil {
		fail(err)
		return
	}

	if ctx.Err() != nil {
		fail(errors.New("Request was aborted"))
		return
	}
	if output.StopReason == ai.StopReasonAborted {
		fail(errors.New("Request was aborted"))
		return
	}
	if output.StopReason == ai.StopReasonError {
		msg := output.ErrorMessage
		if msg == "" {
			msg = "An unknown error occurred"
		}
		fail(errors.New(msg))
		return
	}

	out.Push(ai.DoneEvent{Reason: output.StopReason, Message: output})
}

// assertRequestAuth ports the synchronous apiKey check at the top of
// azure-openai-responses.ts's stream(): unlike the openai-responses core (and
// its own custom-auth-header bypass), Azure has no such fallback -- an empty
// apiKey always fails.
func assertRequestAuth(provider, apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("No API key for provider: %s", provider)
	}
	return nil
}

// statusError composes an error for a non-2xx response, matching the
// sibling adapter packages' approach (see openairesponses.go's
// httpStatusError doc comment for why this reconstructs the SDK-shaped
// "<status> <body>" text inline).
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

type wireReasoning struct {
	Effort  string `json:"effort"`
	Summary string `json:"summary,omitempty"`
}

// wireRequest is the Azure OpenAI Responses streaming request body. Unlike
// the openai-responses core's wireRequest, prompt_cache_key and store are
// always sent (azure-openai-responses.ts's buildParams sets them
// unconditionally, with no cache-retention gate), and there is no
// service_tier field (Azure has none).
type wireRequest struct {
	Model           string         `json:"model"`
	Input           []any          `json:"input"`
	Stream          bool           `json:"stream"`
	PromptCacheKey  string         `json:"prompt_cache_key"`
	Store           bool           `json:"store"`
	MaxOutputTokens *int           `json:"max_output_tokens,omitempty"`
	Temperature     *float64       `json:"temperature,omitempty"`
	Tools           any            `json:"tools,omitempty"`
	Reasoning       *wireReasoning `json:"reasoning,omitempty"`
	Include         []string       `json:"include,omitempty"`
}

// clampOpenAIPromptCacheKey duplicates
// openairesponses.clampOpenAIPromptCacheKey: truncates key to OpenAI's
// 64-character limit, counting Unicode code points.
func clampOpenAIPromptCacheKey(key string) string {
	runes := []rune(key)
	if len(runes) <= openAIPromptCacheKeyMaxLength {
		return key
	}
	return string(runes[:openAIPromptCacheKeyMaxLength])
}

// buildParams ports azure-openai-responses.ts's buildParams: it reuses the
// shared openairesponses.ConvertMessages/ConvertTools for message/tool
// conversion (with Azure's own AZURE_TOOL_CALL_PROVIDERS set) but builds its
// own wire request shape, since Azure's request differs from the core's in
// several fields (see wireRequest's doc comment) and predates/diverges from
// the core's own buildParams rather than delegating to it.
func buildParams(model *ai.Model, chat ai.Context, opts *ai.StreamOptions, deploymentName string) *wireRequest {
	role := "system"
	if model.Reasoning && supportsDeveloperRole(model) {
		role = "developer"
	}
	items := openairesponses.ConvertMessages(model, chat, openairesponses.ConvertMessagesOptions{
		IncludeSystemPrompt:      true,
		SystemPromptRole:         role,
		AllowedToolCallProviders: azureToolCallProviders,
	})

	var sessionID string
	if opts != nil {
		sessionID = opts.SessionID
	}

	params := &wireRequest{
		Model:          deploymentName,
		Input:          items,
		Stream:         true,
		Store:          false,
		PromptCacheKey: clampOpenAIPromptCacheKey(sessionID),
	}

	if opts != nil && opts.MaxTokens != nil && *opts.MaxTokens > 0 {
		maxTokens := *opts.MaxTokens
		if maxTokens < openAIResponsesMinOutputTokens {
			maxTokens = openAIResponsesMinOutputTokens
		}
		params.MaxOutputTokens = &maxTokens
	}

	if opts != nil && opts.Temperature != nil {
		t := *opts.Temperature
		params.Temperature = &t
	}

	if len(chat.Tools) > 0 {
		params.Tools = openairesponses.ConvertTools(chat.Tools, openairesponses.ConvertToolsOptions{})
	}

	applyReasoning(params, model, opts)

	return params
}

// supportsDeveloperRole ports the inline `compat?.supportsDeveloperRole !==
// false` check in convertResponsesMessages, which defaults to true absent an
// explicit override (mirroring openairesponses.getCompat's default for the
// same flag).
func supportsDeveloperRole(model *ai.Model) bool {
	if model.Compat == nil || model.Compat.SupportsDeveloperRole == nil {
		return true
	}
	return *model.Compat.SupportsDeveloperRole
}

// applyReasoning ports azure-openai-responses.ts buildParams' model.reasoning
// gate; identical in shape to openairesponses.applyReasoning (duplicated
// rather than shared -- see wireRequest's doc comment on why the two
// variants' buildParams aren't unified).
func applyReasoning(params *wireRequest, model *ai.Model, opts *ai.StreamOptions) {
	if !model.Reasoning {
		return
	}
	var effortRequested ai.ThinkingLevel
	var summaryRequested string
	if opts != nil {
		effortRequested = opts.ReasoningEffort
		summaryRequested = opts.ReasoningSummary
	}

	if effortRequested != "" || summaryRequested != "" {
		effort := "medium"
		if effortRequested != "" {
			if mapped, ok := mappedThinkingLevel(model, effortRequested); ok {
				effort = mapped
			} else {
				effort = string(effortRequested)
			}
		}
		summary := summaryRequested
		if summary == "" {
			summary = "auto"
		}
		params.Reasoning = &wireReasoning{Effort: effort, Summary: summary}
		params.Include = []string{"reasoning.encrypted_content"}
		return
	}

	if v, present := model.ThinkingLevelMap[ai.ThinkingOff]; present && v == nil {
		return
	}
	effort := "none"
	if v, ok := mappedThinkingLevel(model, ai.ThinkingOff); ok {
		effort = v
	}
	params.Reasoning = &wireReasoning{Effort: effort}
}

// mappedThinkingLevel duplicates openairesponses.mappedThinkingLevel.
func mappedThinkingLevel(model *ai.Model, level ai.ThinkingLevel) (string, bool) {
	v, present := model.ThinkingLevelMap[level]
	if !present || v == nil {
		return "", false
	}
	return *v, true
}

// --- headers -----------------------------------------------------------------

// buildHeaders builds the request headers. Ports the header-building half of
// azure-openai-responses.ts's createClient (model.headers merged with
// options.headers) plus the api-key auth header the AzureOpenAI SDK would
// otherwise inject from its apiKey constructor field -- see this repo's raw
// net/http deviation documented in PORTING.md's "Vendor SDKs" section.
func buildHeaders(model *ai.Model, opts *ai.StreamOptions, apiKey string) map[string]string {
	defaults := map[string]string{
		"content-type": "application/json",
		"accept":       "text/event-stream",
	}
	if apiKey != "" {
		defaults["api-key"] = apiKey
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
