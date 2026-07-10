package images

// Ports: packages/ai/src/api/openrouter-images.ts,
// packages/ai/src/api/openrouter-images.lazy.ts (lazy wrapper not ported —
// see registry.go's RegisterAPIProvider doc comment),
// packages/ai/src/providers/openrouter-images.ts (the catalog/auth wiring is
// BuiltinProviders/OpenRouterProvider in builtin.go).
//
// The upstream adapter talks to OpenRouter's OpenAI-compatible
// /chat/completions endpoint through the `openai` SDK. This port uses raw
// net/http + encoding/json instead, matching every other adapter in this
// codebase's "Vendor SDKs" deviation (see docs/PORTING.md) — there is no Go
// equivalent of the openai npm package in this repo's dependency graph, and
// the wire shape is simple enough not to need one.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/julienlegoux/kern-link/ai"
)

func init() {
	RegisterAPIProvider("openrouter-images", generateImagesOpenRouter)
}

// --- request building ---------------------------------------------------

type openRouterImagesRequest struct {
	Model      string                `json:"model"`
	Messages   []openRouterImagesMsg `json:"messages"`
	Stream     bool                  `json:"stream"`
	Modalities []string              `json:"modalities"`
}

type openRouterImagesMsg struct {
	Role    string                    `json:"role"`
	Content []openRouterImagesContent `json:"content"`
}

type openRouterImagesContent struct {
	Type     string                `json:"type"`
	Text     string                `json:"text,omitempty"`
	ImageURL *openRouterImageURLIn `json:"image_url,omitempty"`
}

type openRouterImageURLIn struct {
	URL string `json:"url"`
}

func buildOpenRouterImagesParams(model *Model, imgCtx Context) openRouterImagesRequest {
	content := make([]openRouterImagesContent, 0, len(imgCtx.Input))
	for _, item := range imgCtx.Input {
		switch v := item.(type) {
		case ai.TextContent:
			content = append(content, openRouterImagesContent{Type: "text", Text: ai.SanitizeSurrogates(v.Text)})
		case ai.ImageContent:
			content = append(content, openRouterImagesContent{
				Type:     "image_url",
				ImageURL: &openRouterImageURLIn{URL: fmt.Sprintf("data:%s;base64,%s", v.MimeType, v.Data)},
			})
		}
	}

	modalities := []string{"image"}
	if model.SupportsOutput(ai.ModalityText) {
		modalities = []string{"image", "text"}
	}

	return openRouterImagesRequest{
		Model:      model.ID,
		Messages:   []openRouterImagesMsg{{Role: "user", Content: content}},
		Stream:     false,
		Modalities: modalities,
	}
}

func buildOpenRouterImagesHeaders(model *Model, opts *Options, apiKey string) map[string]string {
	defaults := map[string]string{"content-type": "application/json"}
	if apiKey != "" {
		defaults["authorization"] = "Bearer " + apiKey
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

// --- response decoding ----------------------------------------------------

type openRouterImagesResponse struct {
	ID      string                   `json:"id"`
	Usage   *openRouterImagesUsage   `json:"usage"`
	Choices []openRouterImagesChoice `json:"choices"`
}

type openRouterImagesUsage struct {
	PromptTokens        int                               `json:"prompt_tokens"`
	CompletionTokens    int                               `json:"completion_tokens"`
	PromptTokensDetails *openRouterImagesUsageTokenDetail `json:"prompt_tokens_details"`
}

type openRouterImagesUsageTokenDetail struct {
	CachedTokens     int `json:"cached_tokens"`
	CacheWriteTokens int `json:"cache_write_tokens"`
}

type openRouterImagesChoice struct {
	Message openRouterImagesRespMessage `json:"message"`
}

type openRouterImagesRespMessage struct {
	Content string                     `json:"content"`
	Images  []openRouterGeneratedImage `json:"images"`
}

type openRouterGeneratedImage struct {
	ImageURL openRouterImageURLOut `json:"image_url"`
}

// openRouterImageURLOut decodes a response `image_url` field that upstream
// (and OpenRouter's API) may send as either a bare string or `{ "url": ... }`.
type openRouterImageURLOut struct {
	URL string
}

func (v *openRouterImageURLOut) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		v.URL = s
		return nil
	}
	var obj struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	v.URL = obj.URL
	return nil
}

var dataURLPattern = regexp.MustCompile(`(?s)^data:([^;]+);base64,(.+)$`)

func parseOpenRouterImagesUsage(u *openRouterImagesUsage, model *Model) *ai.Usage {
	if u == nil {
		return nil
	}
	promptTokens := u.PromptTokens
	var reportedCachedTokens, cacheWriteTokens int
	if u.PromptTokensDetails != nil {
		reportedCachedTokens = u.PromptTokensDetails.CachedTokens
		cacheWriteTokens = u.PromptTokensDetails.CacheWriteTokens
	}
	cacheReadTokens := reportedCachedTokens
	if cacheWriteTokens > 0 {
		cacheReadTokens = reportedCachedTokens - cacheWriteTokens
		if cacheReadTokens < 0 {
			cacheReadTokens = 0
		}
	}
	input := promptTokens - cacheReadTokens - cacheWriteTokens
	if input < 0 {
		input = 0
	}
	output := u.CompletionTokens

	cost := ai.UsageCost{
		Input:      model.Cost.Input / 1_000_000 * float64(input),
		Output:     model.Cost.Output / 1_000_000 * float64(output),
		CacheRead:  model.Cost.CacheRead / 1_000_000 * float64(cacheReadTokens),
		CacheWrite: model.Cost.CacheWrite / 1_000_000 * float64(cacheWriteTokens),
	}
	cost.Total = cost.Input + cost.Output + cost.CacheRead + cost.CacheWrite

	return &ai.Usage{
		Input:       input,
		Output:      output,
		CacheRead:   cacheReadTokens,
		CacheWrite:  cacheWriteTokens,
		TotalTokens: input + output + cacheReadTokens + cacheWriteTokens,
		Cost:        cost,
	}
}

// --- adapter --------------------------------------------------------------

// generateImagesOpenRouter implements Function for the "openrouter-images"
// api: a non-streaming POST to OpenRouter's OpenAI-compatible
// /chat/completions endpoint requesting image (and optionally text)
// modalities, decoding the response's inline base64 data URLs into
// ai.ImageContent blocks.
func generateImagesOpenRouter(ctx context.Context, model *Model, imgCtx Context, opts *Options) *AssistantImages {
	var apiKey string
	if opts != nil {
		apiKey = opts.APIKey
	}
	if apiKey == "" {
		return errorResult(model, fmt.Sprintf("No API key for provider: %s", model.Provider))
	}

	params := buildOpenRouterImagesParams(model, imgCtx)
	var payload any = params
	if opts != nil && opts.OnPayload != nil {
		next, err := opts.OnPayload(ctx, payload, model)
		if err != nil {
			return abortAwareErrorResult(ctx, model, err)
		}
		if next != nil {
			payload = next
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return errorResult(model, err.Error())
	}

	url := strings.TrimRight(model.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return errorResult(model, err.Error())
	}
	for k, v := range buildOpenRouterImagesHeaders(model, opts, apiKey) {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	if opts != nil && opts.Timeout > 0 {
		client.Timeout = opts.Timeout
	}
	resp, err := client.Do(req)
	if err != nil {
		return abortAwareErrorResult(ctx, model, err)
	}
	defer resp.Body.Close()

	if opts != nil && opts.OnResponse != nil {
		respMeta := ai.ProviderResponse{Status: resp.StatusCode, Headers: ai.HeadersToRecord(resp.Header)}
		if err := opts.OnResponse(ctx, respMeta, model); err != nil {
			return abortAwareErrorResult(ctx, model, err)
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errorResult(model, openRouterImagesHTTPStatusError(resp).Error())
	}

	var wireResp openRouterImagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&wireResp); err != nil {
		return errorResult(model, err.Error())
	}

	out := &AssistantImages{
		Api:        model.Api,
		Provider:   model.Provider,
		Model:      model.ID,
		StopReason: StopReasonStop,
		ResponseID: wireResp.ID,
		Usage:      parseOpenRouterImagesUsage(wireResp.Usage, model),
		Timestamp:  time.Now().UnixMilli(),
	}

	if len(wireResp.Choices) > 0 {
		msg := wireResp.Choices[0].Message
		if msg.Content != "" {
			out.Output = append(out.Output, ai.TextContent{Text: msg.Content})
		}
		for _, img := range msg.Images {
			if !strings.HasPrefix(img.ImageURL.URL, "data:") {
				continue
			}
			matches := dataURLPattern.FindStringSubmatch(img.ImageURL.URL)
			if matches == nil {
				continue
			}
			out.Output = append(out.Output, ai.ImageContent{MimeType: matches[1], Data: matches[2]})
		}
	}

	return out
}

// abortAwareErrorResult mirrors every other adapter's fail() helper: a
// request-scoped error is reported as "aborted" when the context was
// canceled, "error" otherwise.
func abortAwareErrorResult(ctx context.Context, model *Model, err error) *AssistantImages {
	result := errorResult(model, err.Error())
	if ctx.Err() != nil {
		result.StopReason = StopReasonAborted
	}
	return result
}

// openRouterImagesHTTPStatusError composes an error for a non-2xx response,
// matching the openai-completions adapter's httpStatusError approach: see
// its doc comment for why this reconstructs "<status> <body>" text inline
// rather than through a shared multi-SDK error normalizer.
func openRouterImagesHTTPStatusError(resp *http.Response) error {
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
