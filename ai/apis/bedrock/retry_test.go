package bedrock

// Bedrock speaks the AWS SDK rather than raw net/http, so it cannot adopt the
// shared ai/apis/internal/httpretry loop the sibling adapters use. Instead
// StreamOptions.MaxRetries/MaxRetryDelay are mapped onto the SDK's own
// retryer. These tests pin both halves: the retryer configuration
// (inspectable through Client.Options(), the same no-network technique
// client_test.go uses) and the retry loop it actually drives, against an
// httptest server standing in for the Bedrock runtime endpoint.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/julienlegoux/kern-link/ai"
)

func intPtr(v int) *int { return &v }

func durPtr(d time.Duration) *time.Duration { return &d }

// --- retryer configuration ---------------------------------------------------

func TestResolveClientConfig_MaxRetriesDefaultsToSharedDefault(t *testing.T) {
	cfg := resolveClientConfig(testModel(), nil)
	if cfg.MaxRetries != defaultMaxRetries {
		t.Fatalf("MaxRetries = %d, want the shared default of %d", cfg.MaxRetries, defaultMaxRetries)
	}
}

func TestResolveClientConfig_MaxRetriesAndDelayComeFromOptions(t *testing.T) {
	cfg := resolveClientConfig(testModel(), &ai.StreamOptions{
		MaxRetries:    intPtr(5),
		MaxRetryDelay: durPtr(3 * time.Second),
	})
	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", cfg.MaxRetries)
	}
	if cfg.MaxRetryDelay != 3*time.Second {
		t.Errorf("MaxRetryDelay = %v, want 3s", cfg.MaxRetryDelay)
	}
}

func TestResolveClientConfig_ZeroMaxRetriesIsHonoredNotTreatedAsUnset(t *testing.T) {
	cfg := resolveClientConfig(testModel(), &ai.StreamOptions{MaxRetries: intPtr(0)})
	if cfg.MaxRetries != 0 {
		t.Fatalf("MaxRetries = %d, want 0 (an explicit zero disables retries)", cfg.MaxRetries)
	}
}

// TestNewBedrockRuntimeClient_MaxAttemptsIsMaxRetriesPlusOne pins the
// off-by-one that separates the two vocabularies: StreamOptions counts
// *retries*, the AWS SDK counts *attempts*.
func TestNewBedrockRuntimeClient_MaxAttemptsIsMaxRetriesPlusOne(t *testing.T) {
	for _, tc := range []struct{ maxRetries, wantAttempts int }{
		{0, 1},
		{2, 3},
		{5, 6},
	} {
		client, err := newBedrockRuntimeClient(context.Background(), clientConfig{
			Region:     "us-east-1",
			MaxRetries: tc.maxRetries,
		})
		if err != nil {
			t.Fatalf("newBedrockRuntimeClient: %v", err)
		}
		retryer := client.Options().Retryer
		if retryer == nil {
			t.Fatal("Options().Retryer = nil, want a configured retryer")
		}
		if got := retryer.MaxAttempts(); got != tc.wantAttempts {
			t.Errorf("MaxRetries %d: MaxAttempts() = %d, want %d", tc.maxRetries, got, tc.wantAttempts)
		}
	}
}

// --- the retry loop the config drives ----------------------------------------

// bedrockTestModel points the adapter at srv, which stands in for the Bedrock
// runtime endpoint. skipAuth keeps resolveClientConfig from reaching for real
// credentials.
func bedrockTestModel(srvURL string) *ai.Model {
	model := testModel()
	model.BaseURL = srvURL
	return model
}

func bedrockOpts(opts *ai.StreamOptions) *ai.StreamOptions {
	opts.BedrockRegion = "us-east-1"
	opts.Env = ai.ProviderEnv{"AWS_BEDROCK_SKIP_AUTH": "1"}
	return opts
}

func streamResult(t *testing.T, srv *httptest.Server, opts *ai.StreamOptions) *ai.AssistantMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}
	stream := Stream(ctx, bedrockTestModel(srv.URL), chat, bedrockOpts(opts))
	result, err := stream.Result(ctx)
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	return result
}

// writeEmptyEventStream answers with a well-formed but empty ConverseStream
// response: the SDK's event-stream decoder sees immediate EOF and closes the
// stream with no events, which is a *successful* call as far as the retry
// loop is concerned. That is all these tests need — event decoding itself is
// stream_test.go's job.
func writeEmptyEventStream(w http.ResponseWriter) {
	w.Header().Set("content-type", "application/vnd.amazon.eventstream")
	w.WriteHeader(http.StatusOK)
}

func TestStream_Retries500ThenSucceedsWithinDefaultMaxRetries(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&attempts, 1) <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		writeEmptyEventStream(w)
	}))
	t.Cleanup(srv.Close)

	// No MaxRetries set: the shared default of 2 must cover two failures.
	result := streamResult(t, srv, &ai.StreamOptions{MaxRetryDelay: durPtr(10 * time.Millisecond)})
	if result.StopReason == ai.StopReasonError {
		t.Fatalf("stopReason = error (errorMessage=%q), want a successful call after 2 retries", result.ErrorMessage)
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("attempts = %d, want 3 (initial + 2 default retries)", got)
	}
}

func TestStream_ExhaustedRetriesSurfaceInBandErrorEvent(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	result := streamResult(t, srv, &ai.StreamOptions{
		MaxRetries:    intPtr(1),
		MaxRetryDelay: durPtr(10 * time.Millisecond),
	})
	if result.StopReason != ai.StopReasonError {
		t.Fatalf("stopReason = %q, want error", result.StopReason)
	}
	if result.ErrorMessage == "" {
		t.Error("ErrorMessage is empty, want the SDK's surfaced failure")
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("attempts = %d, want 2 (initial + 1 retry)", got)
	}
}

func TestStream_MaxRetriesZeroDisablesRetries(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	result := streamResult(t, srv, &ai.StreamOptions{MaxRetries: intPtr(0)})
	if result.StopReason != ai.StopReasonError {
		t.Fatalf("stopReason = %q, want error", result.StopReason)
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Fatalf("attempts = %d, want 1 (no retries)", got)
	}
}

// TestStream_MaxRetryDelayCapsBackoff asserts the cap is wired to the SDK's
// backoff rather than ignored: 5 retries with a 5ms cap cannot take anywhere
// near the SDK's default 20s maximum backoff.
func TestStream_MaxRetryDelayCapsBackoff(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	start := time.Now()
	result := streamResult(t, srv, &ai.StreamOptions{
		MaxRetries:    intPtr(5),
		MaxRetryDelay: durPtr(5 * time.Millisecond),
	})
	elapsed := time.Since(start)

	if result.StopReason != ai.StopReasonError {
		t.Fatalf("stopReason = %q, want error", result.StopReason)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("elapsed = %v across 5 retries: MaxRetryDelay did not cap the SDK backoff", elapsed)
	}
}
