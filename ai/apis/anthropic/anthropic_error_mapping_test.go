package anthropic

// Ports: packages/ai/src/utils/error-body.ts (documented SDK contract) and
// epic 5 issue 04's "wire retry/overflow classification" scope.
//
// Upstream's anthropic-messages.ts never composes this text itself: the
// @anthropic-ai/sdk's `.asResponse()` call throws an SDK-internal APIError
// for a non-2xx response before iterateAnthropicEvents ever runs, and
// error-body.ts's normalizeProviderError comment documents that Anthropic's
// SDK error message already "folds the body into the message" (status code +
// the JSON-stringified error body, or "<status> status code (no body)" when
// the body is empty — the exact wording that comment calls out for a bodyless
// 403). kern-proxy has no vendored SDK, so this package composes the
// equivalent text itself from the raw net/http response, in the same shape,
// so ai/retry.go and ai/overflow.go's text-pattern classifiers (already
// ported from Epic 1) can fire against real Anthropic error responses:
//   - 413 request_too_large -> ai.IsContextOverflow matches the literal
//     `error.type` value embedded in the JSON.
//   - 400 invalid_request_error "prompt is too long: ..." -> matches on
//     `error.message`.
//   - 429/500/502/503/504/529 -> ai.IsRetryableAssistantError matches the
//     numeric status or "overloaded"/"rate limit" text.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
)

func jsonErrorServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(status)
		if body != "" {
			_, _ = w.Write([]byte(body))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func streamAndResult(t *testing.T, srv *httptest.Server) *ai.AssistantMessage {
	t.Helper()
	model := testModel(srv.URL)
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "sk-ant-test"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	return result
}

func TestStream_NonOKStatusWithJSONErrorBodyIsClassifiedAsOverflow(t *testing.T) {
	srv := jsonErrorServer(t, http.StatusBadRequest,
		`{"type":"error","error":{"type":"invalid_request_error","message":"prompt is too long: 220000 tokens > 200000 maximum"}}`)
	result := streamAndResult(t, srv)

	if result.StopReason != ai.StopReasonError {
		t.Errorf("stopReason = %q, want error", result.StopReason)
	}
	want := `400 {"type":"error","error":{"type":"invalid_request_error","message":"prompt is too long: 220000 tokens > 200000 maximum"}}`
	if result.ErrorMessage != want {
		t.Errorf("errorMessage = %q, want %q", result.ErrorMessage, want)
	}
	if !ai.IsContextOverflow(result, 200000) {
		t.Errorf("IsContextOverflow(result, 200000) = false, want true")
	}
}

func TestStream_NonOKStatusWithRequestTooLargeTypeIsClassifiedAsOverflow(t *testing.T) {
	// The 413 case: the human-readable message doesn't itself contain
	// "request_too_large" text, only the error envelope's `type` field does
	// — ai/overflow.go's pattern matches that literal type string, so the
	// composed error text must retain the whole JSON object, not just
	// `error.message`.
	srv := jsonErrorServer(t, http.StatusRequestEntityTooLarge,
		`{"type":"error","error":{"type":"request_too_large","message":"Request body too large. Maximum allowed length is 32 MB."}}`)
	result := streamAndResult(t, srv)

	if result.StopReason != ai.StopReasonError {
		t.Errorf("stopReason = %q, want error", result.StopReason)
	}
	if !ai.IsContextOverflow(result, 200000) {
		t.Errorf("IsContextOverflow(result, 200000) = false, want true")
	}
}

func TestStream_NonOKStatusWithOverloadedErrorIsClassifiedAsRetryable(t *testing.T) {
	srv := jsonErrorServer(t, http.StatusTooManyRequests,
		`{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`)
	result := streamAndResult(t, srv)

	if result.StopReason != ai.StopReasonError {
		t.Errorf("stopReason = %q, want error", result.StopReason)
	}
	want := `429 {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`
	if result.ErrorMessage != want {
		t.Errorf("errorMessage = %q, want %q", result.ErrorMessage, want)
	}
	if !ai.IsRetryableAssistantError(result) {
		t.Errorf("IsRetryableAssistantError(result) = false, want true")
	}
}

func TestStream_NonOKStatusWithEmptyBodyUsesStatusCodeNoBodyMessage(t *testing.T) {
	srv := jsonErrorServer(t, http.StatusServiceUnavailable, "")
	result := streamAndResult(t, srv)

	if result.StopReason != ai.StopReasonError {
		t.Errorf("stopReason = %q, want error", result.StopReason)
	}
	want := "503 status code (no body)"
	if result.ErrorMessage != want {
		t.Errorf("errorMessage = %q, want %q", result.ErrorMessage, want)
	}
	if !ai.IsRetryableAssistantError(result) {
		t.Errorf("IsRetryableAssistantError(result) = false, want true")
	}
}

func TestStream_NonOKStatusWithNonJSONBodyIsPassedThroughVerbatim(t *testing.T) {
	srv := jsonErrorServer(t, http.StatusBadGateway, "<html>Bad Gateway</html>")
	result := streamAndResult(t, srv)

	if result.StopReason != ai.StopReasonError {
		t.Errorf("stopReason = %q, want error", result.StopReason)
	}
	want := "502 <html>Bad Gateway</html>"
	if result.ErrorMessage != want {
		t.Errorf("errorMessage = %q, want %q", result.ErrorMessage, want)
	}
	if !ai.IsRetryableAssistantError(result) {
		t.Errorf("IsRetryableAssistantError(result) = false, want true")
	}
}
