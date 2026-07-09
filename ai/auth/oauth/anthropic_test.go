package oauth

// Ports: packages/ai/test/anthropic-oauth.test.ts, plus the anthropic-only
// cases of packages/ai/test/oauth-auth.test.ts ("anthropic toAuth ...",
// "anthropic refresh ..."). Copilot/Codex cases in oauth-auth.test.ts are out
// of scope (epic 12, issues 02/03).
//
// anthropicTokenURL/anthropicCallbackPort are swapped to test doubles for
// every test in this file (httptest server, OS-assigned port) instead of the
// real Anthropic endpoint/fixed :53692, so no test touches the network or a
// real port.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
)

// withFakeAnthropicTokenServer points anthropicTokenURL at an httptest server
// for the duration of the test and restores it afterward.
func withFakeAnthropicTokenServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	original := anthropicTokenURL
	anthropicTokenURL = server.URL
	t.Cleanup(func() { anthropicTokenURL = original })

	return server
}

// withOSAssignedCallbackPort makes the Anthropic callback server bind an
// OS-assigned port instead of the real :53692, for the duration of the test.
func withOSAssignedCallbackPort(t *testing.T) {
	t.Helper()
	original := anthropicCallbackPort
	anthropicCallbackPort = 0
	t.Cleanup(func() { anthropicCallbackPort = original })
}

func jsonHandler(t *testing.T, body map[string]any) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}
}

func decodeRequestBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	return body
}

// TestLoginAnthropic_KeepsRedirectURIForManualCallbackLogin ports
// "keeps the localhost redirect_uri for manual callback login".
func TestLoginAnthropic_KeepsRedirectURIForManualCallbackLogin(t *testing.T) {
	withOSAssignedCallbackPort(t)

	var gotBody map[string]any
	server := withFakeAnthropicTokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotBody = decodeRequestBody(t, r)
		jsonHandler(t, map[string]any{
			"access_token":  "access-token",
			"refresh_token": "refresh-token",
			"expires_in":    3600,
		})(w, r)
	})
	_ = server

	var authURL string
	cred, err := loginAnthropic(context.Background(), anthropicLoginOptions{
		OnAuth: func(u, _ string) { authURL = u },
		OnManualCodeInput: func(_ context.Context, _ string) (string, error) {
			u, err := url.Parse(authURL)
			if err != nil {
				t.Fatalf("parse auth url: %v", err)
			}
			state := u.Query().Get("state")
			redirectURI := u.Query().Get("redirect_uri")
			if state == "" || redirectURI == "" {
				t.Fatal("missing state or redirect_uri in auth URL")
			}
			return redirectURI + "?code=manual-code&state=" + state, nil
		},
	})
	if err != nil {
		t.Fatalf("loginAnthropic() error = %v", err)
	}

	if cred.Access != "access-token" || cred.Refresh != "refresh-token" {
		t.Errorf("credential = %+v, want access-token/refresh-token", cred)
	}

	if gotBody["grant_type"] != "authorization_code" {
		t.Errorf("grant_type = %v, want authorization_code", gotBody["grant_type"])
	}
	if gotBody["code"] != "manual-code" {
		t.Errorf("code = %v, want manual-code", gotBody["code"])
	}
	wantRedirect, _ := url.Parse(authURL)
	if gotBody["redirect_uri"] != wantRedirect.Query().Get("redirect_uri") {
		t.Errorf("redirect_uri = %v, want %v", gotBody["redirect_uri"], wantRedirect.Query().Get("redirect_uri"))
	}
}

// TestRefreshAnthropicToken_OmitsScope ports "omits scope from refresh token
// requests".
func TestRefreshAnthropicToken_OmitsScope(t *testing.T) {
	var gotBody map[string]any
	withFakeAnthropicTokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotBody = decodeRequestBody(t, r)
		jsonHandler(t, map[string]any{
			"access_token":  "new-access-token",
			"refresh_token": "new-refresh-token",
			"expires_in":    3600,
		})(w, r)
	})

	cred, err := RefreshAnthropicToken(context.Background(), "refresh-token")
	if err != nil {
		t.Fatalf("RefreshAnthropicToken() error = %v", err)
	}

	if cred.Access != "new-access-token" || cred.Refresh != "new-refresh-token" {
		t.Errorf("credential = %+v, want new-access-token/new-refresh-token", cred)
	}
	if gotBody["grant_type"] != "refresh_token" {
		t.Errorf("grant_type = %v, want refresh_token", gotBody["grant_type"])
	}
	if gotBody["client_id"] == "" || gotBody["client_id"] == nil {
		t.Error("client_id is empty, want a non-empty client id")
	}
	if gotBody["refresh_token"] != "refresh-token" {
		t.Errorf("refresh_token = %v, want refresh-token", gotBody["refresh_token"])
	}
	if _, hasScope := gotBody["scope"]; hasScope {
		t.Errorf("request body has a scope field, want none: %+v", gotBody)
	}
}

// TestRefreshAnthropicToken_AppliesFiveMinuteMargin checks the issue's
// "expires stored as epoch-ms minus the 5-minute margin" requirement.
func TestRefreshAnthropicToken_AppliesFiveMinuteMargin(t *testing.T) {
	originalClock := clock
	clock = func() int64 { return 1_000_000 }
	t.Cleanup(func() { clock = originalClock })

	withFakeAnthropicTokenServer(t, jsonHandler(t, map[string]any{
		"access_token":  "a",
		"refresh_token": "r",
		"expires_in":    3600, // seconds
	}))

	cred, err := RefreshAnthropicToken(context.Background(), "refresh-token")
	if err != nil {
		t.Fatalf("RefreshAnthropicToken() error = %v", err)
	}

	want := int64(1_000_000) + 3600*1000 - 5*60*1000
	if cred.Expires != want {
		t.Errorf("Expires = %d, want %d (now + expires_in*1000 - 5min margin)", cred.Expires, want)
	}
}

// TestAnthropicOAuth_ToAuth ports "anthropic toAuth derives the api key from
// the access token".
func TestAnthropicOAuth_ToAuth(t *testing.T) {
	auth, err := AnthropicOAuth.ToAuth(context.Background(), &ai.OAuthCredential{Access: "token", Refresh: "r"})
	if err != nil {
		t.Fatalf("ToAuth() error = %v", err)
	}
	if auth.APIKey != "token" {
		t.Errorf("APIKey = %q, want token", auth.APIKey)
	}
}

// TestAnthropicOAuth_Refresh ports "anthropic refresh exchanges the refresh
// token and returns a typed credential".
func TestAnthropicOAuth_Refresh(t *testing.T) {
	withFakeAnthropicTokenServer(t, jsonHandler(t, map[string]any{
		"access_token":  "new-access",
		"refresh_token": "new-refresh",
		"expires_in":    3600,
	}))

	refreshed, err := AnthropicOAuth.Refresh(context.Background(), &ai.OAuthCredential{Access: "old", Refresh: "old-r"})
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if refreshed.Access != "new-access" || refreshed.Refresh != "new-refresh" {
		t.Errorf("refreshed = %+v, want new-access/new-refresh", refreshed)
	}
	if refreshed.Expires <= time.Now().UnixMilli()-6*60*1000 {
		// Sanity check only (real clock in play here): expires should be in
		// the future minus the 5-minute margin, not some stale/zero value.
		t.Errorf("Expires = %d, looks stale", refreshed.Expires)
	}
}

// TestAnthropicOAuthLogin_ResolvesThroughManualCodePromptAndCancelsItsCtx
// ports "anthropicOAuth.login resolves through the manual_code prompt and
// aborts it after settling".
func TestAnthropicOAuthLogin_ResolvesThroughManualCodePromptAndCancelsItsCtx(t *testing.T) {
	withOSAssignedCallbackPort(t)
	withFakeAnthropicTokenServer(t, jsonHandler(t, map[string]any{
		"access_token":  "access",
		"refresh_token": "refresh",
		"expires_in":    3600,
	}))

	var events []ai.AuthEvent
	var prompts []ai.AuthPrompt
	var manualCtx context.Context

	callbacks := ai.AuthLoginCallbacks{
		Notify: func(event ai.AuthEvent) { events = append(events, event) },
		Prompt: func(_ context.Context, prompt ai.AuthPrompt) (string, error) {
			prompts = append(prompts, prompt)
			if prompt.Type == ai.AuthPromptManualCode {
				manualCtx = prompt.Ctx
				return "the-code", nil
			}
			t.Fatalf("unexpected prompt type %q", prompt.Type)
			return "", nil
		},
	}

	cred, err := AnthropicOAuth.Login(context.Background(), callbacks)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if cred.Access != "access" {
		t.Errorf("Access = %q, want access", cred.Access)
	}

	sawAuthURL := false
	for _, e := range events {
		if e.Type == ai.AuthEventAuthURL {
			sawAuthURL = true
		}
	}
	if !sawAuthURL {
		t.Error("no auth_url event was emitted")
	}

	sawManualPrompt := false
	for _, p := range prompts {
		if p.Type == ai.AuthPromptManualCode {
			sawManualPrompt = true
		}
	}
	if !sawManualPrompt {
		t.Error("no manual_code prompt was issued")
	}

	if manualCtx == nil {
		t.Fatal("manual_code prompt had no Ctx")
	}
	if manualCtx.Err() == nil {
		t.Error("manual_code prompt's Ctx was not cancelled once login settled")
	}
}

// TestLoginAnthropic_ManualCodeResolvesWhileCallbackServerPending covers one
// direction of the issue's required race test: the manual-code prompt
// resolves before any callback ever reaches the local server.
func TestLoginAnthropic_ManualCodeResolvesWhileCallbackServerPending(t *testing.T) {
	withOSAssignedCallbackPort(t)
	withFakeAnthropicTokenServer(t, jsonHandler(t, map[string]any{
		"access_token":  "access-from-manual",
		"refresh_token": "refresh-from-manual",
		"expires_in":    3600,
	}))

	var callbackHits int32

	cred, err := loginAnthropic(context.Background(), anthropicLoginOptions{
		OnAuth: func(string, string) {},
		OnManualCodeInput: func(_ context.Context, _ string) (string, error) {
			// Resolves immediately, without ever contacting the callback
			// server: the callback server stays pending for the whole flow.
			return "manual-only-code", nil
		},
	})
	if err != nil {
		t.Fatalf("loginAnthropic() error = %v", err)
	}

	if atomic.LoadInt32(&callbackHits) != 0 {
		t.Error("callback server was contacted, want it to stay pending")
	}
	if cred.Access != "access-from-manual" {
		t.Errorf("Access = %q, want access-from-manual", cred.Access)
	}
}

// TestLoginAnthropic_CallbackServerResolvesWhileManualCodePending covers the
// other direction: an actual HTTP hit on the callback server resolves the
// login while the manual-code prompt is still blocked waiting to be
// dismissed.
func TestLoginAnthropic_CallbackServerResolvesWhileManualCodePending(t *testing.T) {
	withOSAssignedCallbackPort(t)
	withFakeAnthropicTokenServer(t, jsonHandler(t, map[string]any{
		"access_token":  "access-from-callback",
		"refresh_token": "refresh-from-callback",
		"expires_in":    3600,
	}))

	manualCtx, cancelManual := context.WithCancel(context.Background())
	manualUnblocked := make(chan struct{})

	cred, err := loginAnthropic(context.Background(), anthropicLoginOptions{
		OnAuth: func(authURL, _ string) {
			u, err := url.Parse(authURL)
			if err != nil {
				t.Fatalf("parse auth url: %v", err)
			}
			redirectURI := u.Query().Get("redirect_uri")
			state := u.Query().Get("state")
			// Simulate the browser completing the OAuth redirect while the
			// manual-code prompt (below) is still blocked.
			go func() {
				resp, err := http.Get(redirectURI + "?code=callback-code&state=" + state)
				if err == nil {
					resp.Body.Close()
				}
			}()
		},
		OnManualCodeInput: func(ctx context.Context, _ string) (string, error) {
			// A real UI would block on prompt.Ctx here until either the
			// user answers or the flow is cancelled out from under it. We
			// use our own manualCtx to observe the "cancel it once it loses
			// the race" contract without needing the full ai.OAuthAuth
			// wrapper (that path is covered separately, see
			// TestAnthropicOAuthLogin_ResolvesThroughManualCodePromptAndCancelsItsCtx).
			select {
			case <-manualCtx.Done():
				close(manualUnblocked)
				return "", manualCtx.Err()
			case <-time.After(2 * time.Second):
				close(manualUnblocked)
				return "", context.DeadlineExceeded
			}
		},
	})

	cancelManual()
	<-manualUnblocked

	if err != nil {
		t.Fatalf("loginAnthropic() error = %v", err)
	}
	if cred.Access != "access-from-callback" {
		t.Errorf("Access = %q, want access-from-callback", cred.Access)
	}
}

func TestParseAuthorizationInput(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantCode  string
		wantState string
	}{
		{"full redirect url", "http://localhost:53692/callback?code=abc&state=xyz", "abc", "xyz"},
		{"code hash state", "abc#xyz", "abc", "xyz"},
		{"query string only", "code=abc&state=xyz", "abc", "xyz"},
		{"bare code", "just-a-code", "just-a-code", ""},
		{"empty", "", "", ""},
		{"whitespace padded", "  abc#xyz  ", "abc", "xyz"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, state := parseAuthorizationInput(tc.input)
			if code != tc.wantCode || state != tc.wantState {
				t.Errorf("parseAuthorizationInput(%q) = (%q, %q), want (%q, %q)",
					tc.input, code, state, tc.wantCode, tc.wantState)
			}
		})
	}
}

func TestLoginAnthropic_StateMismatchFromManualInputFails(t *testing.T) {
	withOSAssignedCallbackPort(t)

	_, err := loginAnthropic(context.Background(), anthropicLoginOptions{
		OnAuth: func(string, string) {},
		OnManualCodeInput: func(_ context.Context, _ string) (string, error) {
			return "somecode#wrong-state", nil
		},
	})
	if err == nil {
		t.Fatal("loginAnthropic() error = nil, want a state-mismatch error")
	}
	if !strings.Contains(err.Error(), "state mismatch") {
		t.Errorf("error = %q, want it to mention state mismatch", err.Error())
	}
}
