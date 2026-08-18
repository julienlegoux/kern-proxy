// Package images is the image-generation stack: a pipeline parallel to (and
// separate from) text streaming, with its own model descriptor, request/
// result shapes, per-API adapter registry, and provider/auth-resolution
// collection built on ai's ResolveProviderAuth.
//
// Ports: packages/ai/src/images.ts, packages/ai/src/images-models.ts,
// packages/ai/src/images-api-registry.ts, packages/ai/src/image-models.ts,
// packages/ai/src/api/openrouter-images.ts, packages/ai/src/providers/openrouter-images.ts
package images

import (
	"context"
	"time"

	"github.com/kern-ia/kern-link/ai"
)

// Model is an image-generation model descriptor: the image-side counterpart
// of ai.Model (TS ImagesModel<TApi>), dropping chat-only fields (Reasoning,
// ThinkingLevelMap, ContextWindow, MaxTokens, Compat) in favor of Output
// modalities. Field tags match the embedded catalog's JSON shape
// (ai/catalog/data/images/*.json).
type Model struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Api      ai.Api            `json:"api"`
	Provider ai.ProviderId     `json:"provider"`
	BaseURL  string            `json:"baseUrl"`
	Input    []ai.Modality     `json:"input"`
	Output   []ai.Modality     `json:"output"`
	Cost     ai.ModelCost      `json:"cost"`
	Headers  map[string]string `json:"headers,omitempty"`
}

// SupportsOutput reports whether the model produces the given output
// modality (e.g. ai.ModalityText for text-plus-image responses).
func (m *Model) SupportsOutput(modality ai.Modality) bool {
	for _, mod := range m.Output {
		if mod == modality {
			return true
		}
	}
	return false
}

// Context is the full input of an image-generation request. Input blocks
// reuse ai.UserContentPart (TextContent | ImageContent) since TS's
// ImagesInputContent/ImagesOutputContent unions are structurally identical
// to it.
type Context struct {
	Input []ai.UserContentPart
}

// StopReason mirrors TS's ImagesStopReason, a strict subset of
// ai.StopReason: image generation never produces "length" or "toolUse".
type StopReason = ai.StopReason

const (
	StopReasonStop    = ai.StopReasonStop
	StopReasonError   = ai.StopReasonError
	StopReasonAborted = ai.StopReasonAborted
)

// AssistantImages is the final (never partial/streaming) result of an image
// generation request.
type AssistantImages struct {
	Api          ai.Api
	Provider     ai.ProviderId
	Model        string
	Output       []ai.UserContentPart
	ResponseID   string
	Usage        *ai.Usage
	StopReason   StopReason
	ErrorMessage string
	Timestamp    int64
}

// Options configures one GenerateImages call. Cancellation is carried by the
// context.Context parameter (the Go equivalent of TS AbortSignal), same as
// ai.StreamOptions.
type Options struct {
	// APIKey overrides resolved auth for this request.
	APIKey string
	// Env holds provider-scoped environment values that take precedence over
	// the process environment.
	Env ai.ProviderEnv
	// OnPayload can inspect or replace the provider payload before sending.
	// Return (nil, nil) to keep the payload unchanged.
	OnPayload func(ctx context.Context, payload any, model *Model) (any, error)
	// OnResponse is invoked after an HTTP response is received and before its
	// body is consumed.
	OnResponse func(ctx context.Context, response ai.ProviderResponse, model *Model) error
	// Headers are merged over provider defaults; a nil value suppresses a
	// default header with the same name.
	Headers ai.ProviderHeaders
	// Timeout is the HTTP request timeout (TS timeoutMs). Zero = default.
	Timeout time.Duration
	// MaxRetries is the client-side retry attempt cap; nil uses the adapter
	// default.
	MaxRetries *int
	// MaxRetryDelay caps server-requested retry waits.
	MaxRetryDelay *time.Duration
	// Metadata is passed through; adapters extract the fields they
	// understand and ignore the rest.
	Metadata map[string]any
}

// Function is the uniform per-API adapter signature (TS ImagesFunction):
// never fails — request/model/runtime failures are encoded in the returned
// AssistantImages (StopReason "error"/"aborted"), matching the contract
// documented on AssistantImages upstream.
type Function func(ctx context.Context, model *Model, imgCtx Context, opts *Options) *AssistantImages

// errorResult builds an AssistantImages{StopReason: "error"} for model,
// shared by every failure path in this package that has no partial output to
// preserve.
func errorResult(model *Model, msg string) *AssistantImages {
	return &AssistantImages{
		Api:          model.Api,
		Provider:     model.Provider,
		Model:        model.ID,
		StopReason:   StopReasonError,
		ErrorMessage: msg,
		Timestamp:    time.Now().UnixMilli(),
	}
}
