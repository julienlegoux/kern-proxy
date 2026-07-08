package codex

// Ports the HTTP/SSE-path tests from
// upstream/.upstream-clone/packages/ai/test/openai-codex-stream.test.ts --
// every case here uses (or is equivalent to) an explicit `transport: "sse"`
// upstream, so none of it exercises the WebSocket transport (epic 7 issue
// 04's scope). The zstd request-compression test is deferred to issue 04
// alongside the WebSocket work (see codex.go's package doc).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
)

func mockCodexToken(t *testing.T, accountID string) string {
	t.Helper()
	if accountID == "" {
		accountID = "acc_test"
	}
	return buildFixtureToken(t, accountID)
}

func codexModel(baseURL string) *ai.Model {
	return &ai.Model{
		ID:            "gpt-5.1-codex",
		Name:          "GPT-5.1 Codex",
		Api:           ai.ApiOpenAICodexResponses,
		Provider:      "openai-codex",
		BaseURL:       baseURL,
		Reasoning:     true,
		Input:         []ai.Modality{ai.ModalityText},
		ContextWindow: 400000,
		MaxTokens:     128000,
	}
}

// codexSSEPayload builds the same event shape as upstream's buildSSEPayload
// helper: a message with one text delta, terminated by response.completed
// (or response.incomplete).
func codexSSEPayload(status string) string {
	terminal := "response.completed"
	extra := ""
	if status == "incomplete" {
		terminal = "response.incomplete"
		extra = `,"incomplete_details":{"reason":"max_output_tokens"}`
	}
	events := []string{
		`{"type":"response.output_item.added","output_index":0,"item":{"type":"message","id":"msg_1","role":"assistant","status":"in_progress","content":[]}}`,
		`{"type":"response.output_text.delta","output_index":0,"delta":"Hello"}`,
		`{"type":"response.output_item.done","output_index":0,"item":{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"Hello"}]}}`,
		`{"type":"` + terminal + `","response":{"status":"` + status + `"` + extra + `,"usage":{"input_tokens":5,"output_tokens":3,"total_tokens":8,"input_tokens_details":{"cached_tokens":0}}}}`,
	}
	var b strings.Builder
	for _, e := range events {
		b.WriteString("data: ")
		b.WriteString(e)
		b.WriteString("\n\n")
	}
	return b.String()
}

type capturedCodexRequest struct {
	headers http.Header
	body    map[string]any
}

func codexSSEServer(t *testing.T, status string) (*httptest.Server, *capturedCodexRequest) {
	t.Helper()
	captured := &capturedCodexRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.headers = r.Header.Clone()
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		captured.body = body

		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(codexSSEPayload(status)))
	}))
	t.Cleanup(srv.Close)
	return srv, captured
}

func TestStream_BasicSSERoundTripAndHeaders(t *testing.T) {
	srv, captured := codexSSEServer(t, "completed")
	model := codexModel(srv.URL)
	token := mockCodexToken(t, "acc_test")
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("Say hello"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: token})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonStop {
		t.Fatalf("stopReason = %q, want stop (errorMessage=%q)", result.StopReason, result.ErrorMessage)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content = %#v, want 1 text block", result.Content)
	}
	text, ok := result.Content[0].(ai.TextContent)
	if !ok || text.Text != "Hello" {
		t.Errorf("content[0] = %#v, want text Hello", result.Content[0])
	}

	if got := captured.headers.Get("Authorization"); got != "Bearer "+token {
		t.Errorf("Authorization = %q, want Bearer token", got)
	}
	if got := captured.headers.Get("chatgpt-account-id"); got != "acc_test" {
		t.Errorf("chatgpt-account-id = %q, want acc_test", got)
	}
	if got := captured.headers.Get("OpenAI-Beta"); got != "responses=experimental" {
		t.Errorf("OpenAI-Beta = %q, want responses=experimental", got)
	}
	if got := captured.headers.Get("originator"); got != "pi" {
		t.Errorf("originator = %q, want pi", got)
	}
	if got := captured.headers.Get("accept"); got != "text/event-stream" {
		t.Errorf("accept = %q, want text/event-stream", got)
	}
	if captured.headers.Get("x-api-key") != "" {
		t.Errorf("x-api-key present, want absent")
	}
}

func TestStream_IncompleteMapsToStopReasonLength(t *testing.T) {
	srv, _ := codexSSEServer(t, "incomplete")
	model := codexModel(srv.URL)
	token := mockCodexToken(t, "")
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("Say hello"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: token})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonLength {
		t.Fatalf("stopReason = %q, want length", result.StopReason)
	}
	text, _ := result.Content[0].(ai.TextContent)
	if text.Text != "Hello" {
		t.Errorf("content text = %q, want Hello", text.Text)
	}
}

func TestStream_SetsSessionHeadersAndPromptCacheKeyWhenSessionIDProvided(t *testing.T) {
	srv, captured := codexSSEServer(t, "completed")
	model := codexModel(srv.URL)
	token := mockCodexToken(t, "")
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("Say hello"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: token, SessionID: "test-session-123"})
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}

	if got := captured.headers.Get("session-id"); got != "test-session-123" {
		t.Errorf("session-id = %q, want test-session-123", got)
	}
	if got := captured.headers.Get("x-client-request-id"); got != "test-session-123" {
		t.Errorf("x-client-request-id = %q, want test-session-123", got)
	}
	if captured.headers.Get("session_id") != "" {
		t.Errorf("session_id present, want absent (Codex uses session-id, not session_id)")
	}
	if captured.body["prompt_cache_key"] != "test-session-123" {
		t.Errorf("prompt_cache_key = %v, want test-session-123", captured.body["prompt_cache_key"])
	}
}

func TestStream_NoSessionHeadersWhenSessionIDAbsent(t *testing.T) {
	srv, captured := codexSSEServer(t, "completed")
	model := codexModel(srv.URL)
	token := mockCodexToken(t, "")
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("Say hello"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: token})
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}

	for _, h := range []string{"session-id", "session_id", "x-client-request-id"} {
		if captured.headers.Get(h) != "" {
			t.Errorf("%s present, want absent when no sessionId", h)
		}
	}
}

func TestStream_ClampsPromptCacheKeyTo64Characters(t *testing.T) {
	srv, _ := codexSSEServer(t, "completed")
	model := codexModel(srv.URL)
	token := mockCodexToken(t, "")
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("Say hello"), Timestamp: time.Now().UnixMilli()}}}
	sessionID := strings.Repeat("x", 67)

	var captured map[string]any
	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{
		APIKey:    token,
		SessionID: sessionID,
		OnPayload: func(_ context.Context, payload any, _ *ai.Model) (any, error) {
			raw, _ := json.Marshal(payload)
			_ = json.Unmarshal(raw, &captured)
			return nil, nil
		},
	})
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}
	key, _ := captured["prompt_cache_key"].(string)
	if key != strings.Repeat("x", 64) {
		t.Errorf("prompt_cache_key = %q (len %d), want 64 x's", key, len(key))
	}
}

func TestStream_PreservesXHighReasoningEffortFromSimpleOptions(t *testing.T) {
	srv, captured := codexSSEServer(t, "completed")
	model := codexModel(srv.URL)
	model.ID = "gpt-5.5"
	model.ThinkingLevelMap = ai.ThinkingLevelMap{ai.ThinkingXHigh: strPtr("xhigh")}
	token := mockCodexToken(t, "")
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("Say hello"), Timestamp: time.Now().UnixMilli()}}}

	stream := StreamSimple(context.Background(), model, chat, &ai.SimpleStreamOptions{
		StreamOptions: ai.StreamOptions{APIKey: token},
		Reasoning:     ai.ThinkingXHigh,
	})
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}
	reasoning, ok := captured.body["reasoning"].(map[string]any)
	if !ok {
		t.Fatalf("reasoning = %#v, want present", captured.body["reasoning"])
	}
	if reasoning["effort"] != "xhigh" || reasoning["summary"] != "auto" {
		t.Errorf("reasoning = %#v, want {effort:xhigh summary:auto}", reasoning)
	}
}

func TestStream_ClampsMinimalReasoningEffortToLow(t *testing.T) {
	for _, modelID := range []string{"gpt-5.3-codex", "gpt-5.4", "gpt-5.5"} {
		t.Run(modelID, func(t *testing.T) {
			srv, captured := codexSSEServer(t, "completed")
			model := codexModel(srv.URL)
			model.ID = modelID
			model.ThinkingLevelMap = ai.ThinkingLevelMap{ai.ThinkingMinimal: strPtr("low")}
			token := mockCodexToken(t, "")
			chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("Say hello"), Timestamp: time.Now().UnixMilli()}}}

			stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: token, ReasoningEffort: ai.ThinkingMinimal})
			if _, err := stream.Result(context.Background()); err != nil {
				t.Fatalf("Result: %v", err)
			}
			reasoning, ok := captured.body["reasoning"].(map[string]any)
			if !ok || reasoning["effort"] != "low" {
				t.Errorf("reasoning = %#v, want effort low", captured.body["reasoning"])
			}
		})
	}
}

func TestStream_ServiceTierCostMultiplier(t *testing.T) {
	cases := []struct {
		modelID     string
		serviceTier string
		multiplier  float64
	}{
		{"gpt-5.1-codex", "flex", 0.5},
		{"gpt-5.1-codex", "priority", 2},
		{"gpt-5.5", "flex", 0.5},
		{"gpt-5.5", "priority", 2.5},
	}
	for _, tc := range cases {
		t.Run(tc.modelID+"_"+tc.serviceTier, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("content-type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				events := `data: {"type":"response.output_item.added","output_index":0,"item":{"type":"message","id":"msg_1","role":"assistant","status":"in_progress","content":[]}}

data: {"type":"response.output_text.delta","output_index":0,"delta":"Hello"}

data: {"type":"response.output_item.done","output_index":0,"item":{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"Hello"}]}}

data: {"type":"response.completed","response":{"status":"completed","service_tier":"default","usage":{"input_tokens":1000000,"output_tokens":1000000,"total_tokens":2000000,"input_tokens_details":{"cached_tokens":0}}}}

`
				_, _ = w.Write([]byte(events))
			}))
			t.Cleanup(srv.Close)

			model := codexModel(srv.URL)
			model.ID = tc.modelID
			model.Cost = ai.ModelCost{Input: 1, Output: 2}
			token := mockCodexToken(t, "")
			chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("Say hello"), Timestamp: time.Now().UnixMilli()}}}

			stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: token, ServiceTier: tc.serviceTier})
			result, err := stream.Result(context.Background())
			if err != nil {
				t.Fatalf("Result: %v", err)
			}
			if got := result.Usage.Cost.Input; got != 1*tc.multiplier {
				t.Errorf("cost.input = %v, want %v", got, 1*tc.multiplier)
			}
			if got := result.Usage.Cost.Output; got != 2*tc.multiplier {
				t.Errorf("cost.output = %v, want %v", got, 2*tc.multiplier)
			}
			if got := result.Usage.Cost.Total; got != 3*tc.multiplier {
				t.Errorf("cost.total = %v, want %v", got, 3*tc.multiplier)
			}
		})
	}
}

func TestStream_MissingAPIKeyReturnsErrorEvent(t *testing.T) {
	model := codexModel("https://example.invalid")
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonError {
		t.Errorf("stopReason = %q, want error", result.StopReason)
	}
	if !strings.Contains(result.ErrorMessage, "No API key for provider") {
		t.Errorf("errorMessage = %q, want the missing-key message", result.ErrorMessage)
	}
}

func TestStream_InvalidTokenReturnsErrorEvent(t *testing.T) {
	model := codexModel("https://example.invalid")
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: "not-a-jwt"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonError {
		t.Errorf("stopReason = %q, want error", result.StopReason)
	}
	if !strings.Contains(result.ErrorMessage, "Failed to extract accountId") {
		t.Errorf("errorMessage = %q, want the accountId extraction failure", result.ErrorMessage)
	}
}

// TestStream_HeaderTimeoutWhenResponseHeadersDoNotArrive ports "aborts SSE
// fetch after the configured HTTP timeout when response headers do not
// arrive": the server never responds, so the header-only timeout should fire
// and produce a distinct error message.
func TestStream_HeaderTimeoutWhenResponseHeadersDoNotArrive(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer close(block)
	t.Cleanup(srv.Close)

	model := codexModel(srv.URL)
	token := mockCodexToken(t, "")
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: token, Timeout: 30 * time.Millisecond})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonError {
		t.Fatalf("stopReason = %q, want error (errorMessage=%q)", result.StopReason, result.ErrorMessage)
	}
	if !strings.Contains(result.ErrorMessage, "response headers timed out") {
		t.Errorf("errorMessage = %q, want a headers-timed-out message", result.ErrorMessage)
	}
}

// TestStream_AbortsBodyReadAfterHeadersArrive ports "aborts SSE body reads
// after response headers arrive": once headers (and a first delta) arrive,
// cancelling the caller's context should abort the in-flight body read
// without the header timeout (which doesn't apply here) getting in the way.
func TestStream_AbortsBodyReadAfterHeadersArrive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":{\"type\":\"message\",\"id\":\"msg_1\",\"role\":\"assistant\",\"status\":\"in_progress\",\"content\":[]}}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"output_index\":0,\"delta\":\"one\"}\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	model := codexModel(srv.URL)
	token := mockCodexToken(t, "")
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := Stream(ctx, model, chat, &ai.StreamOptions{APIKey: token})

	sawFirstDelta := false
	for ev := range stream.Events() {
		if d, ok := ev.(ai.TextDeltaEvent); ok {
			sawFirstDelta = true
			if d.Delta == "one" {
				cancel()
			}
		}
	}
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if !sawFirstDelta {
		t.Fatal("expected at least one text delta before abort")
	}
	if result.StopReason != ai.StopReasonAborted {
		t.Errorf("stopReason = %q, want aborted (errorMessage=%q)", result.StopReason, result.ErrorMessage)
	}
}

// TestStream_RetriesOnRetryAfterHeaderVariants ports "uses %s for SSE
// retries": a 429 with a retry-after(-ms) header should be retried once and
// then succeed. Unlike the upstream vitest fake-timer version, this doesn't
// assert the exact scheduled delay (Go has no fake-timer equivalent here) --
// it shrinks the retry delay via a tiny retry-after-ms value and asserts the
// retry actually happens and the final result succeeds.
func TestStream_RetriesOnRetryAfterHeaderVariants(t *testing.T) {
	cases := []struct {
		name    string
		headers func() map[string]string
	}{
		{"retry-after-ms", func() map[string]string { return map[string]string{"retry-after-ms": "5"} }},
		{"retry-after seconds (zero)", func() map[string]string { return map[string]string{"retry-after": "0"} }},
		{"retry-after HTTP date (past)", func() map[string]string {
			return map[string]string{"retry-after": time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requests int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := atomic.AddInt32(&requests, 1)
				if n == 1 {
					for k, v := range tc.headers() {
						w.Header().Set(k, v)
					}
					w.Header().Set("content-type", "application/json")
					w.WriteHeader(http.StatusTooManyRequests)
					_, _ = w.Write([]byte(`{"error":{"code":"rate_limit_exceeded","message":"rate limited"}}`))
					return
				}
				w.Header().Set("content-type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(codexSSEPayload("completed")))
			}))
			t.Cleanup(srv.Close)

			model := codexModel(srv.URL)
			token := mockCodexToken(t, "")
			chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
			maxRetries := 1

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			stream := Stream(ctx, model, chat, &ai.StreamOptions{APIKey: token, MaxRetries: &maxRetries})
			result, err := stream.Result(ctx)
			if err != nil {
				t.Fatalf("Result: %v", err)
			}
			if result.StopReason != ai.StopReasonStop {
				t.Fatalf("stopReason = %q, want stop after retry (errorMessage=%q)", result.StopReason, result.ErrorMessage)
			}
			if atomic.LoadInt32(&requests) != 2 {
				t.Errorf("requests = %d, want 2 (one retry)", requests)
			}
		})
	}
}

// TestStream_ExponentialBackoffAcrossRepeatedRetries ports "uses exponential
// backoff across repeated SSE retries without retry headers": 3 successive
// 429s (no retry-after header) with maxRetries:3 should all be retried and
// the 4th attempt succeeds. The package-level baseRetryDelay is shrunk for
// the duration of the test so this doesn't wait on real 1s/2s/4s sleeps.
func TestStream_ExponentialBackoffAcrossRepeatedRetries(t *testing.T) {
	original := baseRetryDelay
	baseRetryDelay = time.Millisecond
	t.Cleanup(func() { baseRetryDelay = original })

	var requests int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requests, 1)
		if n <= 3 {
			w.Header().Set("content-type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"code":"rate_limit_exceeded","message":"rate limited"}}`))
			return
		}
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(codexSSEPayload("completed")))
	}))
	t.Cleanup(srv.Close)

	model := codexModel(srv.URL)
	token := mockCodexToken(t, "")
	chat := ai.Context{Messages: []ai.Message{ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	maxRetries := 3

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream := Stream(ctx, model, chat, &ai.StreamOptions{APIKey: token, MaxRetries: &maxRetries})
	result, err := stream.Result(ctx)
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonStop {
		t.Fatalf("stopReason = %q, want stop after retries (errorMessage=%q)", result.StopReason, result.ErrorMessage)
	}
	if atomic.LoadInt32(&requests) != 4 {
		t.Errorf("requests = %d, want 4 (3 retries then success)", requests)
	}
}

// TestStream_LiveSmoke exercises a real Codex request end to end. It is
// env-gated so it never runs in CI: set OPENAI_CODEX_ACCESS_TOKEN to a valid
// Codex OAuth access token to run it locally.
func TestStream_LiveSmoke(t *testing.T) {
	token := os.Getenv("OPENAI_CODEX_ACCESS_TOKEN")
	if token == "" {
		t.Skip("OPENAI_CODEX_ACCESS_TOKEN not set; skipping live smoke test")
	}

	model := &ai.Model{
		ID:            "gpt-5.1-codex",
		Name:          "GPT-5.1 Codex",
		Api:           ai.ApiOpenAICodexResponses,
		Provider:      "openai-codex",
		Reasoning:     true,
		Input:         []ai.Modality{ai.ModalityText},
		ContextWindow: 400000,
		MaxTokens:     128000,
	}
	chat := ai.Context{
		SystemPrompt: "You are a helpful assistant. Reply exactly as requested.",
		Messages: []ai.Message{
			ai.UserMessage{Content: ai.UserText("Reply with exactly: codex live smoke success"), Timestamp: time.Now().UnixMilli()},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	stream := Stream(ctx, model, chat, &ai.StreamOptions{APIKey: token})
	result, err := stream.Result(ctx)
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason == ai.StopReasonError {
		t.Fatalf("live smoke returned an error stop reason: %s", result.ErrorMessage)
	}
	if len(result.Content) == 0 {
		t.Fatalf("live smoke returned no content")
	}
}
