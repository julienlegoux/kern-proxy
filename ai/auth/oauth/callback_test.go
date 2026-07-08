package oauth

// Ports: packages/ai/src/utils/oauth/anthropic.ts (startCallbackServer — the
// generic local-callback-server plumbing shared by every port-based OAuth
// flow). Upstream has no standalone test for this helper (it's only
// exercised indirectly through anthropic-oauth.test.ts); this file covers
// the server's contract directly since it is now a reusable Go type.

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func startTestCallbackServer(t *testing.T, expectedState string) *CallbackServer {
	t.Helper()
	server, err := StartCallbackServer("127.0.0.1", 0, "/callback", expectedState)
	if err != nil {
		t.Fatalf("StartCallbackServer() error = %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })
	return server
}

func TestCallbackServer_DeliversCodeAndStateOnMatchingCallback(t *testing.T) {
	server := startTestCallbackServer(t, "expected-state")

	resultCh := make(chan *CallbackResult, 1)
	errCh := make(chan error, 1)
	go func() {
		res, err := server.WaitForCode(context.Background())
		resultCh <- res
		errCh <- err
	}()

	resp, err := http.Get(server.RedirectURI + "?code=the-code&state=expected-state")
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	res := <-resultCh
	if err := <-errCh; err != nil {
		t.Fatalf("WaitForCode() error = %v", err)
	}
	if res == nil {
		t.Fatal("WaitForCode() result = nil, want a code/state result")
	}
	if res.Code != "the-code" || res.State != "expected-state" {
		t.Errorf("result = %+v, want Code=the-code State=expected-state", res)
	}
}

func TestCallbackServer_RejectsStateMismatchWithoutSettling(t *testing.T) {
	server := startTestCallbackServer(t, "expected-state")

	resp, err := http.Get(server.RedirectURI + "?code=the-code&state=wrong-state")
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}

	// A mismatched-state request must not settle the pending wait: a
	// subsequent, correct callback should still be able to win the race.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	res, err := server.WaitForCode(ctx)
	if err == nil {
		t.Fatalf("WaitForCode() = (%+v, nil), want a timeout since no valid callback arrived", res)
	}
}

func TestCallbackServer_CancelUnblocksWaitForCodeWithNilResult(t *testing.T) {
	server := startTestCallbackServer(t, "expected-state")

	resultCh := make(chan *CallbackResult, 1)
	errCh := make(chan error, 1)
	go func() {
		res, err := server.WaitForCode(context.Background())
		resultCh <- res
		errCh <- err
	}()

	server.Cancel()

	res := <-resultCh
	if err := <-errCh; err != nil {
		t.Fatalf("WaitForCode() error = %v", err)
	}
	if res != nil {
		t.Errorf("WaitForCode() result = %+v, want nil after Cancel", res)
	}
}

func TestCallbackServer_WaitForCodeRespectsContextCancellation(t *testing.T) {
	server := startTestCallbackServer(t, "expected-state")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := server.WaitForCode(ctx)
	if err == nil {
		t.Fatal("WaitForCode() error = nil, want context.Canceled")
	}
}

func TestCallbackServer_UnknownRouteReturns404(t *testing.T) {
	server := startTestCallbackServer(t, "expected-state")

	resp, err := http.Get(strings.Replace(server.RedirectURI, "/callback", "/nope", 1))
	if err != nil {
		t.Fatalf("GET unknown route: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}
