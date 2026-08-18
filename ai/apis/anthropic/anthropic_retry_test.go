package anthropic

// Proves the anthropic adapter honors StreamOptions.MaxRetries/MaxRetryDelay
// through the shared request-level retry loop
// (ai/apis/internal/httpretry). Mirrors the codex retry tests, which are the
// pattern this adapter now follows.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/apis/internal/httpretry"
)

// TestMain shrinks the shared backoff base for every test in this package: the
// error-mapping tests now exercise the default of 2 retries, and none of them
// should wait on real 1s/2s sleeps.
func TestMain(m *testing.M) {
	httpretry.BaseDelay = time.Millisecond
	os.Exit(m.Run())
}

func retryChat() ai.Context {
	return ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
}

// retryServer answers with `statuses` in order, then streams a minimal
// successful SSE response for every attempt after that.
func retryServer(t *testing.T, attempts *int32, respond func(w http.ResponseWriter, attempt int32) bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(attempts, 1)
		if respond(w, n) {
			return
		}
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(writeSSE(minimalAnthropicEvents())))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func streamResult(t *testing.T, srv *httptest.Server, opts *ai.StreamOptions) *ai.AssistantMessage {
	t.Helper()
	opts.APIKey = "sk-ant-test"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream := Stream(ctx, testModel(srv.URL), retryChat(), opts)
	result, err := stream.Result(ctx)
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	return result
}

func TestStream_Retries429ThenSucceeds(t *testing.T) {
	var attempts int32
	srv := retryServer(t, &attempts, func(w http.ResponseWriter, n int32) bool {
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return true
		}
		return false
	})

	result := streamResult(t, srv, &ai.StreamOptions{})
	if result.StopReason != ai.StopReasonStop {
		t.Fatalf("stopReason = %q, want stop (errorMessage=%q)", result.StopReason, result.ErrorMessage)
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}

func TestStream_Retries500ThenSucceedsWithinDefaultMaxRetries(t *testing.T) {
	var attempts int32
	srv := retryServer(t, &attempts, func(w http.ResponseWriter, n int32) bool {
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return true
		}
		return false
	})

	// No MaxRetries set: the adapter default of 2 must cover two failures.
	result := streamResult(t, srv, &ai.StreamOptions{})
	if result.StopReason != ai.StopReasonStop {
		t.Fatalf("stopReason = %q, want stop (errorMessage=%q)", result.StopReason, result.ErrorMessage)
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("attempts = %d, want 3 (initial + 2 default retries)", got)
	}
}

func TestStream_ExhaustedRetriesSurfaceInBandErrorEvent(t *testing.T) {
	var attempts int32
	srv := retryServer(t, &attempts, func(w http.ResponseWriter, _ int32) bool {
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`))
		return true
	})

	maxRetries := 1
	result := streamResult(t, srv, &ai.StreamOptions{MaxRetries: &maxRetries})
	if result.StopReason != ai.StopReasonError {
		t.Fatalf("stopReason = %q, want error", result.StopReason)
	}
	want := `503 {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`
	if result.ErrorMessage != want {
		t.Fatalf("errorMessage = %q, want %q", result.ErrorMessage, want)
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("attempts = %d, want 2 (initial + 1 retry)", got)
	}
	if !ai.IsRetryableAssistantError(result) {
		t.Error("IsRetryableAssistantError(result) = false, want true")
	}
}

func TestStream_MaxRetriesZeroDisablesRetries(t *testing.T) {
	var attempts int32
	srv := retryServer(t, &attempts, func(w http.ResponseWriter, _ int32) bool {
		w.WriteHeader(http.StatusInternalServerError)
		return true
	})

	zero := 0
	result := streamResult(t, srv, &ai.StreamOptions{MaxRetries: &zero})
	if result.StopReason != ai.StopReasonError {
		t.Fatalf("stopReason = %q, want error", result.StopReason)
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Fatalf("attempts = %d, want 1", got)
	}
}

func TestStream_RetryAfterIsHonoredAndClampedByMaxRetryDelay(t *testing.T) {
	var attempts int32
	srv := retryServer(t, &attempts, func(w http.ResponseWriter, n int32) bool {
		if n == 1 {
			w.Header().Set("retry-after", "3600")
			w.WriteHeader(http.StatusTooManyRequests)
			return true
		}
		return false
	})

	maxRetries := 1
	maxDelay := 10 * time.Millisecond
	start := time.Now()
	result := streamResult(t, srv, &ai.StreamOptions{MaxRetries: &maxRetries, MaxRetryDelay: &maxDelay})
	elapsed := time.Since(start)

	if result.StopReason != ai.StopReasonStop {
		t.Fatalf("stopReason = %q, want stop (errorMessage=%q)", result.StopReason, result.ErrorMessage)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("elapsed = %v: MaxRetryDelay did not clamp the 3600s retry-after", elapsed)
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}

func TestStream_QuotaErrorIsNotRetried(t *testing.T) {
	var attempts int32
	srv := retryServer(t, &attempts, func(w http.ResponseWriter, _ int32) bool {
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"billing_error","message":"credit balance is too low"}}`))
		return true
	})

	maxRetries := 3
	result := streamResult(t, srv, &ai.StreamOptions{MaxRetries: &maxRetries})
	if result.StopReason != ai.StopReasonError {
		t.Fatalf("stopReason = %q, want error", result.StopReason)
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Fatalf("attempts = %d, want 1 (billing errors are terminal)", got)
	}
}

func TestStream_ContextCancellationAbortsPendingBackoff(t *testing.T) {
	original := httpretry.BaseDelay
	httpretry.BaseDelay = 30 * time.Second // a backoff no test should ever wait out
	t.Cleanup(func() { httpretry.BaseDelay = original })

	var attempts int32
	srv := retryServer(t, &attempts, func(w http.ResponseWriter, _ int32) bool {
		w.WriteHeader(http.StatusInternalServerError)
		return true
	})

	ctx, cancel := context.WithCancel(context.Background())
	maxRetries := 5
	stream := Stream(ctx, testModel(srv.URL), retryChat(), &ai.StreamOptions{APIKey: "sk-ant-test", MaxRetries: &maxRetries})

	done := make(chan *ai.AssistantMessage, 1)
	go func() {
		result, err := stream.Result(context.Background())
		if err != nil {
			done <- nil
			return
		}
		done <- result
	}()

	// Give the first attempt time to fail and enter the 30s backoff.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case result := <-done:
		if result != nil && result.StopReason != ai.StopReasonAborted && result.StopReason != ai.StopReasonError {
			t.Fatalf("stopReason = %q, want aborted or error", result.StopReason)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stream did not finish promptly after context cancellation: the backoff sleep ignored ctx.Done()")
	}
}

// TestStream_RequestBodyIsResentOnRetry guards the replay of the marshalled
// payload across attempts.
func TestStream_RequestBodyIsResentOnRetry(t *testing.T) {
	var bodies []string
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		bodies = append(bodies, string(buf))
		if atomic.AddInt32(&attempts, 1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(writeSSE(minimalAnthropicEvents())))
	}))
	t.Cleanup(srv.Close)

	maxRetries := 1
	result := streamResult(t, srv, &ai.StreamOptions{MaxRetries: &maxRetries})
	if result.StopReason != ai.StopReasonStop {
		t.Fatalf("stopReason = %q, want stop", result.StopReason)
	}
	if len(bodies) != 2 || bodies[0] != bodies[1] || bodies[0] == "" {
		t.Fatalf("bodies = %q, want the same non-empty body twice", bodies)
	}
}
