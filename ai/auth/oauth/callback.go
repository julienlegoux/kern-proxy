package oauth

// Ports: packages/ai/src/utils/oauth/anthropic.ts (startCallbackServer). This
// is the generic local-callback-server plumbing shared by every port-based
// OAuth flow (Anthropic here on :53692; Codex reuses it on :1455 in a later
// issue).

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
)

// CallbackResult is the code/state pair delivered by a completed OAuth
// redirect callback.
type CallbackResult struct {
	Code  string
	State string
}

// CallbackServer is a local HTTP server that waits for exactly one OAuth
// redirect callback on its path, validating the callback's state against
// the expected value. It mirrors upstream's startCallbackServer: a single
// result (or none, if Cancel wins the race) is ever delivered through
// WaitForCode, matching the callback-vs-manual-code race the Anthropic flow
// runs.
type CallbackServer struct {
	ln       net.Listener
	srv      *http.Server
	resultCh chan *CallbackResult
	settle   sync.Once

	// RedirectURI is this server's callback URL (http://localhost:<port><path>),
	// using the actual bound port — significant when port 0 was requested.
	RedirectURI string
}

// StartCallbackServer starts listening on host:port (port 0 picks a free
// port, used by tests so they never collide with a real OAuth flow's fixed
// port) and serves path, delivering the callback's code/state once a request
// with a matching state arrives.
func StartCallbackServer(host string, port int, path, expectedState string) (*CallbackServer, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return nil, fmt.Errorf("oauth: listen on %s:%d: %w", host, port, err)
	}

	boundPort := ln.Addr().(*net.TCPAddr).Port
	cs := &CallbackServer{
		ln:          ln,
		resultCh:    make(chan *CallbackResult, 1),
		RedirectURI: fmt.Sprintf("http://localhost:%d%s", boundPort, path),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		if errParam := q.Get("error"); errParam != "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(oauthErrorHTML("Authentication did not complete.", "Error: "+errParam)))
			return
		}

		code := q.Get("code")
		state := q.Get("state")
		if code == "" || state == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(oauthErrorHTML("Missing code or state parameter.")))
			return
		}
		if state != expectedState {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(oauthErrorHTML("State mismatch.")))
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(oauthSuccessHTML("Authentication completed. You can close this window.")))
		cs.settleResult(&CallbackResult{Code: code, State: state})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(oauthErrorHTML("Callback route not found.")))
	})

	cs.srv = &http.Server{Handler: mux}
	go func() {
		_ = cs.srv.Serve(ln)
	}()

	return cs, nil
}

// WaitForCode blocks until a callback arrives, Cancel is called, or ctx is
// done, matching upstream's single-settle waitForCodePromise. A nil result
// with a nil error means Cancel won the race (no code delivered).
func (cs *CallbackServer) WaitForCode(ctx context.Context) (*CallbackResult, error) {
	select {
	case res := <-cs.resultCh:
		return res, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (cs *CallbackServer) settleResult(res *CallbackResult) {
	cs.settle.Do(func() {
		cs.resultCh <- res
	})
}

// Cancel unblocks a pending WaitForCode with no result. Used when a
// concurrent manual-code entry wins the race against the callback.
func (cs *CallbackServer) Cancel() {
	cs.settleResult(nil)
}

// Close shuts down the server and releases its listener.
func (cs *CallbackServer) Close() error {
	return cs.srv.Close()
}
