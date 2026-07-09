package oauth

// Ports: packages/ai/test/openai-codex-oauth.test.ts. The legacy
// openaiCodexOAuthProvider object (the pre-ai.OAuthAuth provider-interface
// shape used by two of that file's cases) is not ported — like the
// Anthropic/Copilot issues before it, only the ai.OAuthAuth-shaped
// openaiCodexOAuth export (CodexOAuth here) is a live consumer in this port,
// so those two cases are re-expressed against CodexOAuth.Login instead.
//
// codexTokenURL/codexDeviceUserCodeURL/codexDeviceTokenURL/
// codexDeviceVerificationURI/codexDeviceRedirectURI/codexAuthorizeURL/
// codexCallbackPort are swapped to test doubles for every test in this file,
// so no test touches the network or the real :1455 port.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/julienlegoux/kern-proxy/ai"
)

// withFakeCodexTokenServer points codexTokenURL at an httptest server for the
// duration of the test and restores it afterward.
func withFakeCodexTokenServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	original := codexTokenURL
	codexTokenURL = server.URL
	t.Cleanup(func() { codexTokenURL = original })

	return server
}

// withOSAssignedCodexCallbackPort makes the Codex browser callback server
// bind an OS-assigned port instead of the real :1455, for the duration of the
// test.
func withOSAssignedCodexCallbackPort(t *testing.T) {
	t.Helper()
	original := codexCallbackPort
	codexCallbackPort = 0
	t.Cleanup(func() { codexCallbackPort = original })
}

// codexAccessTokenWithAccountID builds a minimal unsigned JWT carrying
// chatgpt_account_id under the claim path this port (and upstream) reads,
// matching the ported test fixture's Buffer(...).toString("base64") shape.
func codexAccessTokenWithAccountID(accountID string) string {
	header := base64.StdEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payloadJSON, _ := json.Marshal(map[string]any{
		codexJWTAccountClaim: map[string]any{"chatgpt_account_id": accountID},
	})
	payload := base64.StdEncoding.EncodeToString(payloadJSON)
	return header + "." + payload + ".signature"
}

func codexJSONHandler(t *testing.T, status int, body map[string]any) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}
}

// TestExchangeCodexAuthorizationCode_NoExpiryMargin ports the issue's
// "expires asserted margin-free" acceptance criterion: unlike every other
// flow in this package, Codex's expires carries no 5-minute margin.
func TestExchangeCodexAuthorizationCode_NoExpiryMargin(t *testing.T) {
	originalClock := clock
	clock = func() int64 { return 1_000_000 }
	t.Cleanup(func() { clock = originalClock })

	accessToken := codexAccessTokenWithAccountID("account-123")
	var gotBody url.Values
	withFakeCodexTokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotBody = r.PostForm
		codexJSONHandler(t, http.StatusOK, map[string]any{
			"access_token":  accessToken,
			"refresh_token": "refresh-token",
			"expires_in":    3600,
		})(w, r)
	})

	cred, err := exchangeCodexAuthorizationCodeForCredential(context.Background(), "the-code", "the-verifier", "http://localhost:1455/auth/callback")
	if err != nil {
		t.Fatalf("exchangeCodexAuthorizationCodeForCredential() error = %v", err)
	}

	want := int64(1_000_000) + 3600*1000
	if cred.Expires != want {
		t.Errorf("Expires = %d, want %d (now + expires_in*1000, no margin)", cred.Expires, want)
	}
	if gotBody.Get("grant_type") != "authorization_code" {
		t.Errorf("grant_type = %v, want authorization_code", gotBody.Get("grant_type"))
	}
	if gotBody.Get("code") != "the-code" {
		t.Errorf("code = %v, want the-code", gotBody.Get("code"))
	}
	if gotBody.Get("code_verifier") != "the-verifier" {
		t.Errorf("code_verifier = %v, want the-verifier", gotBody.Get("code_verifier"))
	}
	if gotBody.Get("redirect_uri") != "http://localhost:1455/auth/callback" {
		t.Errorf("redirect_uri = %v, want http://localhost:1455/auth/callback", gotBody.Get("redirect_uri"))
	}
}

// TestRefreshCodexToken_PreservesAccountIDAndNoMargin ports the issue's
// "Extra round-trips accountId" acceptance criterion for the refresh path.
func TestRefreshCodexToken_PreservesAccountIDAndNoMargin(t *testing.T) {
	originalClock := clock
	clock = func() int64 { return 2_000_000 }
	t.Cleanup(func() { clock = originalClock })

	accessToken := codexAccessTokenWithAccountID("account-456")
	var gotBody url.Values
	withFakeCodexTokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotBody = r.PostForm
		codexJSONHandler(t, http.StatusOK, map[string]any{
			"access_token":  accessToken,
			"refresh_token": "new-refresh",
			"expires_in":    1800,
		})(w, r)
	})

	cred, err := RefreshCodexToken(context.Background(), "old-refresh")
	if err != nil {
		t.Fatalf("RefreshCodexToken() error = %v", err)
	}
	if gotBody.Get("grant_type") != "refresh_token" {
		t.Errorf("grant_type = %v, want refresh_token", gotBody.Get("grant_type"))
	}
	if gotBody.Get("refresh_token") != "old-refresh" {
		t.Errorf("refresh_token = %v, want old-refresh", gotBody.Get("refresh_token"))
	}
	if got, _ := cred.Extra["accountId"].(string); got != "account-456" {
		t.Errorf("Extra[accountId] = %v, want account-456", cred.Extra["accountId"])
	}
	want := int64(2_000_000) + 1800*1000
	if cred.Expires != want {
		t.Errorf("Expires = %d, want %d (no margin)", cred.Expires, want)
	}
}

// TestExchangeCodexAuthorizationCode_FailsWithoutAccountID matches upstream's
// credentialsFromToken throwing when the access token carries no accountId.
func TestExchangeCodexAuthorizationCode_FailsWithoutAccountID(t *testing.T) {
	withFakeCodexTokenServer(t, codexJSONHandler(t, http.StatusOK, map[string]any{
		"access_token":  "not-a-jwt",
		"refresh_token": "refresh-token",
		"expires_in":    3600,
	}))

	_, err := exchangeCodexAuthorizationCodeForCredential(context.Background(), "code", "verifier", "redirect")
	if err == nil {
		t.Fatal("exchangeCodexAuthorizationCodeForCredential() error = nil, want an accountId extraction error")
	}
	if !strings.Contains(err.Error(), "accountId") {
		t.Errorf("error = %q, want it to mention accountId", err.Error())
	}
}

// TestCodexRequestToken_ErrorIncludesResponseBody ports "does not write token
// refresh failures to stderr" (the stderr assertion has no Go equivalent;
// what's ported is the message-format assertion it also makes).
func TestCodexRequestToken_ErrorIncludesResponseBody(t *testing.T) {
	withFakeCodexTokenServer(t, codexJSONHandler(t, http.StatusUnauthorized, map[string]any{
		"error": map[string]any{
			"message": "Could not validate your token. Please try signing in again.",
			"type":    "invalid_request_error",
		},
	}))

	_, err := RefreshCodexToken(context.Background(), "invalid-refresh-token")
	if err == nil {
		t.Fatal("RefreshCodexToken() error = nil, want a token refresh failure")
	}
	if !strings.Contains(err.Error(), "OpenAI Codex token refresh failed (401)") {
		t.Errorf("error = %q, want it to mention status 401", err.Error())
	}
	if !strings.Contains(err.Error(), "Could not validate your token") {
		t.Errorf("error = %q, want it to include the response body", err.Error())
	}
}

// TestStartCodexDeviceAuth_ParsesStringInterval matches upstream's usercode
// response fixture, which sends interval as a JSON string ("5").
func TestStartCodexDeviceAuth_ParsesStringInterval(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		codexJSONHandler(t, http.StatusOK, map[string]any{
			"device_auth_id": "device-auth-id",
			"user_code":      "ABCD-1234",
			"interval":       "5",
		})(w, r)
	}))
	defer server.Close()

	original := codexDeviceUserCodeURL
	codexDeviceUserCodeURL = server.URL
	t.Cleanup(func() { codexDeviceUserCodeURL = original })

	device, err := startCodexDeviceAuth(context.Background())
	if err != nil {
		t.Fatalf("startCodexDeviceAuth() error = %v", err)
	}
	if device.DeviceAuthID != "device-auth-id" {
		t.Errorf("DeviceAuthID = %q, want device-auth-id", device.DeviceAuthID)
	}
	if device.UserCode != "ABCD-1234" {
		t.Errorf("UserCode = %q, want ABCD-1234", device.UserCode)
	}
	if device.IntervalSeconds != 5 {
		t.Errorf("IntervalSeconds = %d, want 5", device.IntervalSeconds)
	}
	if gotBody["client_id"] != codexClientID {
		t.Errorf("client_id = %v, want %v", gotBody["client_id"], codexClientID)
	}
}

func TestStartCodexDeviceAuth_404IsNotEnabledError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	original := codexDeviceUserCodeURL
	codexDeviceUserCodeURL = server.URL
	t.Cleanup(func() { codexDeviceUserCodeURL = original })

	_, err := startCodexDeviceAuth(context.Background())
	if err == nil {
		t.Fatal("startCodexDeviceAuth() error = nil, want a not-enabled error")
	}
	if !strings.Contains(err.Error(), "not enabled") {
		t.Errorf("error = %q, want it to mention the login is not enabled", err.Error())
	}
}

// TestPollCodexDeviceAuth_Treats403And404AsPending ports "treats OpenAI Codex
// device auth 403 and 404 responses as pending".
func TestPollCodexDeviceAuth_Treats403And404AsPending(t *testing.T) {
	withVirtualDeviceCodeClock(t, 0)

	responses := []struct {
		status int
		body   string
	}{
		{http.StatusForbidden, `{"error":"access_denied","error_description":"denied"}`},
		{http.StatusNotFound, "not ready"},
		{http.StatusOK, `{"authorization_code":"oauth-code","code_verifier":"device-code-verifier"}`},
	}
	idx := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if idx >= len(responses) {
			t.Fatalf("unexpected extra poll at index %d", idx)
		}
		resp := responses[idx]
		idx++
		w.WriteHeader(resp.status)
		_, _ = w.Write([]byte(resp.body))
	}))
	defer server.Close()

	original := codexDeviceTokenURL
	codexDeviceTokenURL = server.URL
	t.Cleanup(func() { codexDeviceTokenURL = original })

	device := &codexDeviceAuthInfo{DeviceAuthID: "device-auth-id", UserCode: "ABCD-1234", IntervalSeconds: 1}
	result, err := pollCodexDeviceAuth(context.Background(), device)
	if err != nil {
		t.Fatalf("pollCodexDeviceAuth() error = %v", err)
	}
	if result.AuthorizationCode != "oauth-code" || result.CodeVerifier != "device-code-verifier" {
		t.Errorf("result = %+v, want oauth-code/device-code-verifier", result)
	}
	if idx != 3 {
		t.Errorf("polled %d times, want 3", idx)
	}
}

// TestPollCodexDeviceAuth_IncludesResponseBodyInFailureMessage ports
// "includes the response body in OpenAI Codex device auth poll failures".
func TestPollCodexDeviceAuth_IncludesResponseBodyInFailureMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"server_error","error_description":"try again later"}`))
	}))
	defer server.Close()

	original := codexDeviceTokenURL
	codexDeviceTokenURL = server.URL
	t.Cleanup(func() { codexDeviceTokenURL = original })

	device := &codexDeviceAuthInfo{DeviceAuthID: "device-auth-id", UserCode: "ABCD-1234", IntervalSeconds: 5}
	_, err := pollCodexDeviceAuth(context.Background(), device)
	if err == nil {
		t.Fatal("pollCodexDeviceAuth() error = nil, want a failure")
	}
	want := `OpenAI Codex device auth failed with status 500: {"error":"server_error","error_description":"try again later"}`
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to contain %q", err.Error(), want)
	}
}

// TestLoginCodexDeviceCode_PollsAndExchanges ports "logs in with the OpenAI
// Codex device code flow".
func TestLoginCodexDeviceCode_PollsAndExchanges(t *testing.T) {
	withVirtualDeviceCodeClock(t, 1_000_000)
	originalVerificationURI := codexDeviceVerificationURI
	t.Cleanup(func() { codexDeviceVerificationURI = originalVerificationURI })
	codexDeviceVerificationURI = "https://auth.openai.com/codex/device"

	accessToken := codexAccessTokenWithAccountID("account-123")

	userCodeServer := httptest.NewServer(codexJSONHandler(t, http.StatusOK, map[string]any{
		"device_auth_id": "device-auth-id",
		"user_code":      "ABCD-1234",
		"interval":       "5",
	}))
	defer userCodeServer.Close()

	pollResponses := []map[string]any{
		{"error": map[string]any{"code": "deviceauth_authorization_pending"}},
		{"authorization_code": "oauth-code", "code_verifier": "device-code-verifier"},
	}
	pollIdx := 0
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "deviceauth") {
			resp := pollResponses[pollIdx]
			pollIdx++
			status := http.StatusOK
			if _, isErr := resp["error"]; isErr {
				status = http.StatusForbidden
			}
			codexJSONHandler(t, status, resp)(w, r)
			return
		}
		codexJSONHandler(t, http.StatusOK, map[string]any{
			"access_token":  accessToken,
			"refresh_token": "refresh-token",
			"expires_in":    3600,
		})(w, r)
	}))
	defer tokenServer.Close()

	originalDeviceTokenURL := codexDeviceTokenURL
	codexDeviceTokenURL = tokenServer.URL + "/deviceauth"
	t.Cleanup(func() { codexDeviceTokenURL = originalDeviceTokenURL })
	originalUserCodeURL := codexDeviceUserCodeURL
	codexDeviceUserCodeURL = userCodeServer.URL
	t.Cleanup(func() { codexDeviceUserCodeURL = originalUserCodeURL })
	originalTokenURL := codexTokenURL
	codexTokenURL = tokenServer.URL
	t.Cleanup(func() { codexTokenURL = originalTokenURL })

	var deviceUserCode, deviceVerificationURI string
	var deviceInterval, deviceExpires int
	cred, err := loginCodexDeviceCode(context.Background(), func(userCode, verificationURI string, intervalSeconds, expiresInSeconds int) {
		deviceUserCode = userCode
		deviceVerificationURI = verificationURI
		deviceInterval = intervalSeconds
		deviceExpires = expiresInSeconds
	})
	if err != nil {
		t.Fatalf("loginCodexDeviceCode() error = %v", err)
	}

	if deviceUserCode != "ABCD-1234" {
		t.Errorf("userCode = %q, want ABCD-1234", deviceUserCode)
	}
	if deviceVerificationURI != "https://auth.openai.com/codex/device" {
		t.Errorf("verificationURI = %q, want https://auth.openai.com/codex/device", deviceVerificationURI)
	}
	if deviceInterval != 5 {
		t.Errorf("intervalSeconds = %d, want 5", deviceInterval)
	}
	if deviceExpires != codexDeviceCodeTimeoutSeconds {
		t.Errorf("expiresInSeconds = %d, want %d", deviceExpires, codexDeviceCodeTimeoutSeconds)
	}
	if cred.Access != accessToken {
		t.Errorf("Access = %q, want the access token", cred.Access)
	}
	if got, _ := cred.Extra["accountId"].(string); got != "account-123" {
		t.Errorf("Extra[accountId] = %v, want account-123", cred.Extra["accountId"])
	}
	wantExpires := int64(1_000_000) + 5000 + 3600*1000
	if cred.Expires != wantExpires {
		t.Errorf("Expires = %d, want %d (no margin, plus the 5s poll wait)", cred.Expires, wantExpires)
	}
}

// TestLoginCodexDeviceCode_TimesOutAfterFifteenMinutes ports "times out the
// OpenAI Codex device code flow after 15 minutes".
func TestLoginCodexDeviceCode_TimesOutAfterFifteenMinutes(t *testing.T) {
	withVirtualDeviceCodeClock(t, 0)

	userCodeServer := httptest.NewServer(codexJSONHandler(t, http.StatusOK, map[string]any{
		"device_auth_id": "device-auth-id",
		"user_code":      "ABCD-1234",
		"interval":       "60",
	}))
	defer userCodeServer.Close()

	pending := httptest.NewServer(codexJSONHandler(t, http.StatusForbidden, map[string]any{
		"error": map[string]any{"code": "deviceauth_authorization_pending"},
	}))
	defer pending.Close()

	original := codexDeviceUserCodeURL
	codexDeviceUserCodeURL = userCodeServer.URL
	t.Cleanup(func() { codexDeviceUserCodeURL = original })
	originalToken := codexDeviceTokenURL
	codexDeviceTokenURL = pending.URL
	t.Cleanup(func() { codexDeviceTokenURL = originalToken })

	_, err := loginCodexDeviceCode(context.Background(), func(string, string, int, int) {})
	if err == nil {
		t.Fatal("loginCodexDeviceCode() error = nil, want a timeout")
	}
	if err.Error() != errDeviceCodeTimeout.Error() {
		t.Errorf("error = %q, want %q", err.Error(), errDeviceCodeTimeout.Error())
	}
}

// TestLoginCodexDeviceCode_CancelsWhilePolling ports "cancels the OpenAI
// Codex device code flow while waiting".
func TestLoginCodexDeviceCode_CancelsWhilePolling(t *testing.T) {
	userCodeServer := httptest.NewServer(codexJSONHandler(t, http.StatusOK, map[string]any{
		"device_auth_id": "device-auth-id",
		"user_code":      "ABCD-1234",
		"interval":       "5",
	}))
	defer userCodeServer.Close()

	pollCh := make(chan struct{}, 1)
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Respond fully before signaling the test to cancel, so cancellation
		// lands during the poller's inter-request sleep rather than racing
		// the in-flight HTTP round trip itself (which would surface as a
		// generic "context canceled" transport error instead of the
		// poller's own ErrDeviceCodeCancelled).
		codexJSONHandler(t, http.StatusForbidden, map[string]any{
			"error": map[string]any{"code": "deviceauth_authorization_pending"},
		})(w, r)
		select {
		case pollCh <- struct{}{}:
		default:
		}
	}))
	defer tokenServer.Close()

	original := codexDeviceUserCodeURL
	codexDeviceUserCodeURL = userCodeServer.URL
	t.Cleanup(func() { codexDeviceUserCodeURL = original })
	originalToken := codexDeviceTokenURL
	codexDeviceTokenURL = tokenServer.URL
	t.Cleanup(func() { codexDeviceTokenURL = originalToken })

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := loginCodexDeviceCode(ctx, func(string, string, int, int) {})
		errCh <- err
	}()

	<-pollCh
	cancel()

	err := <-errCh
	if !errors.Is(err, ErrDeviceCodeCancelled) {
		t.Fatalf("loginCodexDeviceCode() error = %v, want ErrDeviceCodeCancelled", err)
	}
}

// TestCodexOAuth_Login_SelectDeviceCode ports "offers browser login first
// and uses the selected OpenAI Codex device code flow".
func TestCodexOAuth_Login_SelectDeviceCode(t *testing.T) {
	withVirtualDeviceCodeClock(t, 0)

	accessToken := codexAccessTokenWithAccountID("account-456")

	userCodeServer := httptest.NewServer(codexJSONHandler(t, http.StatusOK, map[string]any{
		"device_auth_id": "device-auth-id",
		"user_code":      "WXYZ-7890",
		"interval":       "5",
	}))
	defer userCodeServer.Close()
	tokenPollServer := httptest.NewServer(codexJSONHandler(t, http.StatusOK, map[string]any{
		"authorization_code": "oauth-code",
		"code_verifier":      "device-code-verifier",
	}))
	defer tokenPollServer.Close()
	exchangeServer := httptest.NewServer(codexJSONHandler(t, http.StatusOK, map[string]any{
		"access_token":  accessToken,
		"refresh_token": "refresh-token",
		"expires_in":    3600,
	}))
	defer exchangeServer.Close()

	for _, swap := range []struct {
		target *string
		value  string
	}{
		{&codexDeviceUserCodeURL, userCodeServer.URL},
		{&codexDeviceTokenURL, tokenPollServer.URL},
		{&codexTokenURL, exchangeServer.URL},
	} {
		original := *swap.target
		*swap.target = swap.value
		t.Cleanup(func(orig string, tgt *string) func() { return func() { *tgt = orig } }(original, swap.target))
	}

	var selectPrompt ai.AuthPrompt
	var deviceEvent *ai.AuthEvent
	sawAuthURL := false
	callbacks := ai.AuthLoginCallbacks{
		Prompt: func(_ context.Context, prompt ai.AuthPrompt) (string, error) {
			if prompt.Type == ai.AuthPromptSelect {
				selectPrompt = prompt
				return codexDeviceCodeLoginMethod, nil
			}
			t.Fatalf("unexpected prompt type %q (browser login should not start)", prompt.Type)
			return "", nil
		},
		Notify: func(event ai.AuthEvent) {
			if event.Type == ai.AuthEventAuthURL {
				sawAuthURL = true
			}
			if event.Type == ai.AuthEventDeviceCode {
				e := event
				deviceEvent = &e
			}
		},
	}

	cred, err := CodexOAuth.Login(context.Background(), callbacks)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if cred.Access != accessToken {
		t.Errorf("Access = %q, want the access token", cred.Access)
	}
	if got, _ := cred.Extra["accountId"].(string); got != "account-456" {
		t.Errorf("Extra[accountId] = %v, want account-456", cred.Extra["accountId"])
	}
	if sawAuthURL {
		t.Error("auth_url event was emitted, want browser login not to start")
	}
	if selectPrompt.Message != "Select OpenAI Codex login method:" {
		t.Errorf("select prompt message = %q", selectPrompt.Message)
	}
	if len(selectPrompt.Options) != 2 || selectPrompt.Options[0].ID != codexBrowserLoginMethod || selectPrompt.Options[1].ID != codexDeviceCodeLoginMethod {
		t.Errorf("select prompt options = %+v, want browser then device_code", selectPrompt.Options)
	}
	if deviceEvent == nil {
		t.Fatal("no AuthEventDeviceCode was notified")
	}
	if deviceEvent.UserCode != "WXYZ-7890" {
		t.Errorf("UserCode = %q, want WXYZ-7890", deviceEvent.UserCode)
	}
	if deviceEvent.VerificationURI != codexDeviceVerificationURI {
		t.Errorf("VerificationURI = %q, want %q", deviceEvent.VerificationURI, codexDeviceVerificationURI)
	}
}

// TestCodexOAuth_Login_SelectCancelled ports "cancels when OpenAI Codex login
// method selection is cancelled": a cancelled select prompt is a Prompt
// error in this port's callback contract (AuthLoginCallbacks.Prompt "fails
// on cancel/abort"), which Login simply propagates.
func TestCodexOAuth_Login_SelectCancelled(t *testing.T) {
	wantErr := errors.New("login cancelled")
	callbacks := ai.AuthLoginCallbacks{
		Prompt: func(context.Context, ai.AuthPrompt) (string, error) { return "", wantErr },
		Notify: func(ai.AuthEvent) {},
	}

	_, err := CodexOAuth.Login(context.Background(), callbacks)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Login() error = %v, want %v", err, wantErr)
	}
}

// TestCodexOAuth_Login_SelectBrowser_ManualCodeWins exercises the browser/PKCE
// path (issue Scope: "PKCE flow on :1455 and the device-code alternative"),
// including the manual-code-vs-callback race this package's other flows use.
func TestCodexOAuth_Login_SelectBrowser_ManualCodeWins(t *testing.T) {
	withOSAssignedCodexCallbackPort(t)

	accessToken := codexAccessTokenWithAccountID("account-789")
	var gotBody url.Values
	withFakeCodexTokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotBody = r.PostForm
		codexJSONHandler(t, http.StatusOK, map[string]any{
			"access_token":  accessToken,
			"refresh_token": "refresh-token",
			"expires_in":    3600,
		})(w, r)
	})

	var authURL string
	var manualCtx context.Context
	callbacks := ai.AuthLoginCallbacks{
		Prompt: func(_ context.Context, prompt ai.AuthPrompt) (string, error) {
			switch prompt.Type {
			case ai.AuthPromptSelect:
				return codexBrowserLoginMethod, nil
			case ai.AuthPromptManualCode:
				manualCtx = prompt.Ctx
				u, err := url.Parse(authURL)
				if err != nil {
					t.Fatalf("parse auth url: %v", err)
				}
				return u.Query().Get("redirect_uri") + "?code=manual-code&state=" + u.Query().Get("state"), nil
			}
			t.Fatalf("unexpected prompt type %q", prompt.Type)
			return "", nil
		},
		Notify: func(event ai.AuthEvent) {
			if event.Type == ai.AuthEventAuthURL {
				authURL = event.URL
			}
		},
	}

	cred, err := CodexOAuth.Login(context.Background(), callbacks)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if cred.Access != accessToken {
		t.Errorf("Access = %q, want the access token", cred.Access)
	}
	if authURL == "" {
		t.Fatal("no auth_url event was emitted")
	}
	u, _ := url.Parse(authURL)
	if u.Query().Get("client_id") != codexClientID {
		t.Errorf("client_id = %q, want %q", u.Query().Get("client_id"), codexClientID)
	}
	if u.Query().Get("code_challenge_method") != "S256" {
		t.Errorf("code_challenge_method = %q, want S256", u.Query().Get("code_challenge_method"))
	}
	if gotBody.Get("code") != "manual-code" {
		t.Errorf("code = %v, want manual-code", gotBody.Get("code"))
	}
	if gotBody.Get("redirect_uri") != u.Query().Get("redirect_uri") {
		t.Errorf("redirect_uri = %v, want %v", gotBody.Get("redirect_uri"), u.Query().Get("redirect_uri"))
	}
	if manualCtx == nil {
		t.Fatal("manual_code prompt had no Ctx")
	}
	if manualCtx.Err() == nil {
		t.Error("manual_code prompt's Ctx was not cancelled once login settled")
	}
}

// TestLoginCodexBrowser_CallbackResolvesWhileManualCodePending covers the
// other race direction: the local callback server wins while the manual-code
// prompt is still pending.
func TestLoginCodexBrowser_CallbackResolvesWhileManualCodePending(t *testing.T) {
	withOSAssignedCodexCallbackPort(t)
	accessToken := codexAccessTokenWithAccountID("account-race")
	withFakeCodexTokenServer(t, codexJSONHandler(t, http.StatusOK, map[string]any{
		"access_token":  accessToken,
		"refresh_token": "refresh-from-callback",
		"expires_in":    3600,
	}))

	manualCtx, cancelManual := context.WithCancel(context.Background())
	manualUnblocked := make(chan struct{})

	cred, err := loginCodexBrowser(context.Background(), codexBrowserLoginOptions{
		OnAuth: func(authURL string, _ string) {
			u, err := url.Parse(authURL)
			if err != nil {
				t.Fatalf("parse auth url: %v", err)
			}
			redirectURI := u.Query().Get("redirect_uri")
			state := u.Query().Get("state")
			go func() {
				resp, err := http.Get(redirectURI + "?code=callback-code&state=" + state)
				if err == nil {
					resp.Body.Close()
				}
			}()
		},
		OnManualCodeInput: func(context.Context, string) (string, error) {
			<-manualCtx.Done()
			close(manualUnblocked)
			return "", manualCtx.Err()
		},
	})

	cancelManual()
	<-manualUnblocked

	if err != nil {
		t.Fatalf("loginCodexBrowser() error = %v", err)
	}
	if cred.Access != accessToken {
		t.Errorf("Access = %q, want the access token", cred.Access)
	}
}

func TestCodexOAuth_ToAuth(t *testing.T) {
	auth, err := CodexOAuth.ToAuth(context.Background(), &ai.OAuthCredential{Access: "token"})
	if err != nil {
		t.Fatalf("ToAuth() error = %v", err)
	}
	if auth.APIKey != "token" {
		t.Errorf("APIKey = %q, want token", auth.APIKey)
	}
}
