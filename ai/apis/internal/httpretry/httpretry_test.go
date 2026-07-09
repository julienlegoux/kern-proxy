package httpretry_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis/internal/httpretry"
)

func shrinkBaseDelay(t *testing.T) {
	t.Helper()
	original := httpretry.BaseDelay
	httpretry.BaseDelay = time.Millisecond
	t.Cleanup(func() { httpretry.BaseDelay = original })
}

func intPtr(v int) *int { return &v }

func durPtr(d time.Duration) *time.Duration { return &d }

func post(url string) httpretry.Request {
	return httpretry.Request{URL: url, Body: []byte(`{}`)}
}

// TestRetriesTransientStatusesThenSucceeds covers the core criterion: a 429
// followed by a 500 followed by a 200 succeeds within MaxRetries.
func TestRetriesTransientStatusesThenSucceeds(t *testing.T) {
	shrinkBaseDelay(t)

	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		switch atomic.AddInt32(&attempts, 1) {
		case 1:
			w.WriteHeader(http.StatusTooManyRequests)
		case 2:
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "ok")
		}
	}))
	defer srv.Close()

	opts := &ai.StreamOptions{MaxRetries: intPtr(2)}
	resp, err := httpretry.Do(context.Background(), post(srv.URL), httpretry.Config{Opts: opts})
	if err != nil {
		t.Fatalf("Do: unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("attempts = %d, want 3", got)
	}
}

// TestExhaustedRetriesSurfaceTheFailure asserts the caller sees the final
// error body once the retry budget runs out.
func TestExhaustedRetriesSurfaceTheFailure(t *testing.T) {
	shrinkBaseDelay(t)

	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, "service unavailable")
	}))
	defer srv.Close()

	opts := &ai.StreamOptions{MaxRetries: intPtr(1)}
	_, err := httpretry.Do(context.Background(), post(srv.URL), httpretry.Config{Opts: opts})
	if err == nil {
		t.Fatal("Do: expected an error after the retry budget is exhausted")
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("attempts = %d, want 2 (initial + 1 retry)", got)
	}
	if err.Error() != "service unavailable" {
		t.Fatalf("error = %q, want the response body text", err.Error())
	}
}

// TestDefaultMaxRetriesUsedWhenOptsUnset pins the documented default of 2.
func TestDefaultMaxRetriesUsedWhenOptsUnset(t *testing.T) {
	shrinkBaseDelay(t)

	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := httpretry.Do(context.Background(), post(srv.URL), httpretry.Config{
		DefaultMaxRetries: httpretry.DefaultMaxRetries,
	})
	if err == nil {
		t.Fatal("Do: expected an error")
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("attempts = %d, want 3 (initial + DefaultMaxRetries=2)", got)
	}
}

// TestConfigDefaultMaxRetriesZeroDisablesRetries is the codex default.
func TestConfigDefaultMaxRetriesZeroDisablesRetries(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := httpretry.Do(context.Background(), post(srv.URL), httpretry.Config{DefaultMaxRetries: 0})
	if err == nil {
		t.Fatal("Do: expected an error")
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Fatalf("attempts = %d, want 1 (no retries)", got)
	}
}

// TestQuotaAndBillingErrorsAreNotRetried covers the non-retryable
// subscription/billing short-circuit on 429.
func TestQuotaAndBillingErrorsAreNotRetried(t *testing.T) {
	shrinkBaseDelay(t)

	for _, body := range []string{
		`{"error":{"type":"GoUsageLimitError"}}`,
		`{"error":{"code":"insufficient_quota"}}`,
		"Monthly usage limit reached",
		"your billing account is not active",
	} {
		t.Run(body, func(t *testing.T) {
			var attempts int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&attempts, 1)
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = io.WriteString(w, body)
			}))
			defer srv.Close()

			opts := &ai.StreamOptions{MaxRetries: intPtr(3)}
			_, err := httpretry.Do(context.Background(), post(srv.URL), httpretry.Config{Opts: opts})
			if err == nil {
				t.Fatal("Do: expected an error")
			}
			if got := atomic.LoadInt32(&attempts); got != 1 {
				t.Fatalf("attempts = %d, want 1 (quota errors are terminal)", got)
			}
		})
	}
}

// TestRetryAfterHeaderIsHonored asserts the server-requested delay replaces
// the exponential backoff.
func TestRetryAfterHeaderIsHonored(t *testing.T) {
	shrinkBaseDelay(t)

	for _, tc := range []struct {
		name   string
		header string
		value  string
		want   time.Duration
	}{
		{"retry-after seconds", "retry-after", "2", 2 * time.Second},
		{"retry-after-ms wins", "retry-after-ms", "150", 150 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			h.Set(tc.header, tc.value)
			got, ok := httpretry.RetryAfterDelay(h)
			if !ok {
				t.Fatalf("RetryAfterDelay(%v): ok = false, want true", h)
			}
			if got != tc.want {
				t.Fatalf("RetryAfterDelay = %v, want %v", got, tc.want)
			}
		})
	}

	// retry-after-ms takes precedence over retry-after.
	h := http.Header{}
	h.Set("retry-after", "60")
	h.Set("retry-after-ms", "5")
	got, _ := httpretry.RetryAfterDelay(h)
	if got != 5*time.Millisecond {
		t.Fatalf("RetryAfterDelay = %v, want retry-after-ms to win", got)
	}
}

// TestMaxRetryDelayClampsServerRequestedDelay asserts a hostile 429
// Retry-After cannot stall the caller past MaxRetryDelay.
func TestMaxRetryDelayClampsServerRequestedDelay(t *testing.T) {
	shrinkBaseDelay(t)

	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			w.Header().Set("retry-after", "3600")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	opts := &ai.StreamOptions{MaxRetries: intPtr(1), MaxRetryDelay: durPtr(5 * time.Millisecond)}

	start := time.Now()
	resp, err := httpretry.Do(context.Background(), post(srv.URL), httpretry.Config{Opts: opts})
	if err != nil {
		t.Fatalf("Do: unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Fatalf("elapsed = %v: MaxRetryDelay did not clamp the 3600s retry-after", elapsed)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

// TestContextCancellationAbortsBackoffSleep asserts a pending backoff sleep
// unblocks promptly when the caller cancels.
func TestContextCancellationAbortsBackoffSleep(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	// BaseDelay stays at its 1s default here: without the ctx.Done() select in
	// the sleep, this test would take a full second.
	opts := &ai.StreamOptions{MaxRetries: intPtr(5)}

	done := make(chan error, 1)
	go func() {
		_, err := httpretry.Do(ctx, post(srv.URL), httpretry.Config{Opts: opts})
		done <- err
	}()

	time.Sleep(20 * time.Millisecond) // let the first attempt fail and enter backoff
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Do: expected an abort error")
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("Do did not return promptly after context cancellation")
	}
}

// TestOnResponseCallbackIsInvoked asserts the helper preserves the
// StreamOptions.OnResponse contract, including aborting on callback error.
func TestOnResponseCallbackIsInvoked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("x-trace", "abc")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var seen ai.ProviderResponse
	opts := &ai.StreamOptions{
		OnResponse: func(_ context.Context, r ai.ProviderResponse, _ *ai.Model) error {
			seen = r
			return nil
		},
	}
	resp, err := httpretry.Do(context.Background(), post(srv.URL), httpretry.Config{Opts: opts})
	if err != nil {
		t.Fatalf("Do: unexpected error: %v", err)
	}
	resp.Body.Close()
	if seen.Status != http.StatusOK {
		t.Fatalf("OnResponse status = %d, want 200", seen.Status)
	}

	sentinel := errors.New("callback rejected")
	opts.OnResponse = func(context.Context, ai.ProviderResponse, *ai.Model) error { return sentinel }
	if _, err := httpretry.Do(context.Background(), post(srv.URL), httpretry.Config{Opts: opts}); !errors.Is(err, sentinel) {
		t.Fatalf("Do error = %v, want %v", err, sentinel)
	}
}

// TestParseErrorHookShapesTheTerminalError lets adapters keep their own
// error-body formatting.
func TestParseErrorHookShapesTheTerminalError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "bad model")
	}))
	defer srv.Close()

	cfg := httpretry.Config{
		ParseError: func(status int, _, body string) error {
			return fmt.Errorf("adapter says %d: %s", status, body)
		},
	}
	_, err := httpretry.Do(context.Background(), post(srv.URL), cfg)
	if err == nil || err.Error() != "adapter says 400: bad model" {
		t.Fatalf("Do error = %v, want the ParseError-shaped error", err)
	}
}

// TestHeadersAndMethodAreApplied covers request shaping.
func TestHeadersAndMethodAreApplied(t *testing.T) {
	var gotAuth, gotMethod string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("authorization")
		gotMethod = r.Method
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req := httpretry.Request{
		URL:     srv.URL,
		Body:    []byte(`{"hello":1}`),
		Headers: map[string]string{"authorization": "Bearer tok"},
	}
	resp, err := httpretry.Do(context.Background(), req, httpretry.Config{})
	if err != nil {
		t.Fatalf("Do: unexpected error: %v", err)
	}
	resp.Body.Close()

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotAuth != "Bearer tok" {
		t.Fatalf("authorization = %q", gotAuth)
	}
	if string(gotBody) != `{"hello":1}` {
		t.Fatalf("body = %q", gotBody)
	}
}

// TestBodyIsResentOnRetry guards against a reader being consumed by the first
// attempt.
func TestBodyIsResentOnRetry(t *testing.T) {
	shrinkBaseDelay(t)

	var bodies []string
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		if atomic.AddInt32(&attempts, 1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	opts := &ai.StreamOptions{MaxRetries: intPtr(1)}
	resp, err := httpretry.Do(context.Background(), post(srv.URL), httpretry.Config{Opts: opts})
	if err != nil {
		t.Fatalf("Do: unexpected error: %v", err)
	}
	resp.Body.Close()

	if len(bodies) != 2 || bodies[0] != bodies[1] {
		t.Fatalf("bodies = %q, want the same body twice", bodies)
	}
}

// TestTransportErrorsAreRetried covers the "connection refused before any
// response" path.
func TestTransportErrorsAreRetried(t *testing.T) {
	shrinkBaseDelay(t)

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing is listening now

	opts := &ai.StreamOptions{MaxRetries: intPtr(2)}
	start := time.Now()
	_, err := httpretry.Do(context.Background(), post(url), httpretry.Config{Opts: opts})
	if err == nil {
		t.Fatal("Do: expected a transport error")
	}
	// 3 attempts with a 1ms base delay: the point is only that it returned.
	if time.Since(start) > 5*time.Second {
		t.Fatalf("Do took %v", time.Since(start))
	}
}

// TestHeaderTimeoutDoesNotCancelBodyRead is the streaming-critical property:
// Opts.Timeout bounds time-to-headers, not the SSE body that follows.
func TestHeaderTimeoutDoesNotCancelBodyRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(80 * time.Millisecond) // body arrives well after the timeout
		_, _ = io.WriteString(w, "late")
	}))
	defer srv.Close()

	opts := &ai.StreamOptions{Timeout: 40 * time.Millisecond}
	resp, err := httpretry.Do(context.Background(), post(srv.URL), httpretry.Config{Opts: opts})
	if err != nil {
		t.Fatalf("Do: unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body after the header timeout elapsed: %v", err)
	}
	if string(body) != "late" {
		t.Fatalf("body = %q, want %q", body, "late")
	}
}

// TestExponentialDelay pins the doubling schedule.
func TestExponentialDelay(t *testing.T) {
	shrinkBaseDelay(t)
	for attempt, want := range []time.Duration{time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond} {
		if got := httpretry.ExponentialDelay(attempt); got != want {
			t.Fatalf("ExponentialDelay(%d) = %v, want %v", attempt, got, want)
		}
	}
}

// TestIsRetryable pins the shared classification the adapters rely on.
func TestIsRetryable(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		want   bool
	}{
		{http.StatusTooManyRequests, "slow down", true},
		{http.StatusTooManyRequests, "insufficient_quota", false},
		{http.StatusTooManyRequests, `{"error":{"type":"FreeUsageLimitError"}}`, false},
		{http.StatusInternalServerError, "", true},
		{http.StatusBadGateway, "", true},
		{http.StatusServiceUnavailable, "", true},
		{http.StatusGatewayTimeout, "", true},
		{http.StatusBadRequest, "unknown model", false},
		// A rejected request whose body merely *contains* the digits of a
		// transient status must not be retried: ai/retry.go's bare "500"/"429"
		// patterns exist to match the status prefix of a composed assistant
		// error message, not to be applied to a raw HTTP error body.
		{http.StatusBadRequest, `{"error":{"message":"max_tokens: 15000 > 8192"}}`, false},
		{http.StatusBadRequest, "prompt is too long: 429000 tokens", false},
		{http.StatusUnauthorized, "invalid api key 50024", false},
		{http.StatusBadRequest, "upstream connect error", true},
		{http.StatusForbidden, "", false},
	} {
		if got := httpretry.IsRetryable(tc.status, tc.body); got != tc.want {
			t.Fatalf("IsRetryable(%d, %q) = %v, want %v", tc.status, tc.body, got, tc.want)
		}
	}
}
