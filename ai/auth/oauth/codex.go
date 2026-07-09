package oauth

// Ports: packages/ai/src/utils/oauth/openai-codex.ts. Reuses this package's
// shared scaffolding: GeneratePKCE (pkce.ts, pkce.go), StartCallbackServer
// (anthropic.ts's startCallbackServer, generalized in issue 01 so this
// flow's :1455 callback can reuse it — callback.go), PollDeviceCodeFlow
// (device-code.ts, devicecode.go), and parseAuthorizationInput (anthropic.go
// — Codex's own upstream parseAuthorizationInput is functionally identical,
// so it is reused here rather than duplicated).
//
// Codex is this package's one flow with two independent login paths — a
// PKCE/callback-server flow racing a manual-code prompt (same shape as
// Anthropic's), and a device-code flow (same shape as Copilot's) — selected
// by an ai.AuthPromptSelect prompt, matching upstream's dual openaiCodexOAuth
// login. It is also this package's one **expires exception**: every other
// flow subtracts a 5-minute margin from the token's expiry; Codex's does
// not, matching upstream's readTokenResponse (`Date.now() + expires_in *
// 1000`, verbatim, no margin).
//
// Not ported: upstream's legacy openaiCodexOAuthProvider object (the
// pre-ai.OAuthAuth provider-interface shape) — like the Anthropic/Copilot
// issues before it, only the ai.OAuthAuth-shaped openaiCodexOAuth export
// (CodexOAuth below) is a live consumer in this port.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/julienlegoux/kern-proxy/ai"
)

const (
	// codexClientID is upstream's literal CLIENT_ID (the pi CLI's registered
	// OpenAI OAuth app id; unlike Anthropic/Copilot's client ids, this one is
	// not base64-obfuscated upstream).
	codexClientID = "app_EMoamEEZ73f0CkXaXp7hrann"

	codexCallbackPath = "/auth/callback"
	codexScope        = "openid profile email offline_access"
	codexOriginator   = "pi"

	// codexDeviceCodeTimeoutSeconds matches upstream's DEVICE_CODE_TIMEOUT_SECONDS.
	codexDeviceCodeTimeoutSeconds = 15 * 60

	// codexJWTAccountClaim is the JWT claim path carrying chatgpt_account_id,
	// matching upstream's JWT_CLAIM_PATH.
	codexJWTAccountClaim = "https://api.openai.com/auth"

	// Login method ids for the AuthPromptSelect prompt, matching upstream's
	// OPENAI_CODEX_BROWSER_LOGIN_METHOD / OPENAI_CODEX_DEVICE_CODE_LOGIN_METHOD.
	codexBrowserLoginMethod    = "browser"
	codexDeviceCodeLoginMethod = "device_code"
)

// The Codex endpoint URLs and callback port are package vars (not
// constants), so tests can redirect every request to httptest servers and
// bind the callback server on an OS-assigned port instead of the real :1455.
var (
	codexAuthorizeURL          = "https://auth.openai.com/oauth/authorize"
	codexTokenURL              = "https://auth.openai.com/oauth/token"
	codexDeviceUserCodeURL     = "https://auth.openai.com/api/accounts/deviceauth/usercode"
	codexDeviceTokenURL        = "https://auth.openai.com/api/accounts/deviceauth/token"
	codexDeviceVerificationURI = "https://auth.openai.com/codex/device"
	codexDeviceRedirectURI     = "https://auth.openai.com/deviceauth/callback"

	codexCallbackPort = 1455
)

// codexCreateState generates a random hex login state, matching upstream's
// createState (randomBytes(16).toString("hex")).
func codexCreateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("oauth: generate codex state: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// codexDecodeAccountID extracts chatgpt_account_id from an unverified JWT's
// payload segment, matching upstream's decodeJwt/getAccountId. This port
// never validates the JWT signature, same as upstream (which only ever reads
// tokens it just received from OpenAI's own token endpoint).
func codexDecodeAccountID(accessToken string) (string, bool) {
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return "", false
	}

	payload, err := codexDecodeBase64Segment(parts[1])
	if err != nil {
		return "", false
	}

	var claims map[string]json.RawMessage
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", false
	}
	raw, ok := claims[codexJWTAccountClaim]
	if !ok {
		return "", false
	}
	var auth struct {
		ChatGPTAccountID string `json:"chatgpt_account_id"`
	}
	if err := json.Unmarshal(raw, &auth); err != nil || auth.ChatGPTAccountID == "" {
		return "", false
	}
	return auth.ChatGPTAccountID, true
}

// codexDecodeBase64Segment mirrors upstream's atob() call: real-world JWT
// segments are base64url without padding, but atob decodes standard base64.
// Trying both (with and without padding) covers upstream's literal behavior
// and interoperates with real tokens.
func codexDecodeBase64Segment(segment string) ([]byte, error) {
	if b, err := base64.StdEncoding.DecodeString(segment); err == nil {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(segment); err == nil {
		return b, nil
	}
	if b, err := base64.URLEncoding.DecodeString(segment); err == nil {
		return b, nil
	}
	return base64.RawURLEncoding.DecodeString(segment)
}

// codexFlexibleNumber decodes a JSON number that upstream's fixtures send as
// either a number or a numeric string (the device-auth response's
// `interval`), matching upstream's `typeof json?.interval === "string" ?
// Number(...) : json?.interval`.
type codexFlexibleNumber float64

func (n *codexFlexibleNumber) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*n = 0
		return nil
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return fmt.Errorf("oauth: invalid numeric string %q: %w", s, err)
		}
		*n = codexFlexibleNumber(f)
		return nil
	}
	var f float64
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	*n = codexFlexibleNumber(f)
	return nil
}

// codexToken is the raw token-endpoint result before accountId extraction,
// matching upstream's OAuthToken.
type codexToken struct {
	Access  string
	Refresh string
	Expires int64
}

// codexRequestToken POSTs a form-encoded grant to codexTokenURL and returns
// the resulting token, matching upstream's readTokenResponse (shared by both
// exchangeAuthorizationCode and refreshAccessToken). Expires carries no
// margin -- the Codex exception to every other flow in this package. A
// transport failure while ctx is already done is reported as
// ErrDeviceCodeCancelled rather than a raw network error, matching
// upstream's fetchWithLoginCancellation (which upstream applies to the
// authorization-code exchange but not the plain refresh call; this port
// applies it uniformly to both, since Go's ctx-based cancellation makes that
// the more natural shape here).
func codexRequestToken(ctx context.Context, form url.Values, operation string) (codexToken, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, codexTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return codexToken{}, fmt.Errorf("oauth: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return codexToken{}, ErrDeviceCodeCancelled
		}
		return codexToken{}, fmt.Errorf("oauth: OpenAI Codex token %s error: %w", operation, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return codexToken{}, fmt.Errorf("oauth: read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		text := strings.TrimSpace(string(body))
		if text == "" {
			text = resp.Status
		}
		return codexToken{}, fmt.Errorf("oauth: OpenAI Codex token %s failed (%d): %s", operation, resp.StatusCode, text)
	}

	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return codexToken{}, fmt.Errorf("oauth: OpenAI Codex token %s returned invalid json: %w", operation, err)
	}
	if raw.AccessToken == "" || raw.RefreshToken == "" {
		return codexToken{}, fmt.Errorf("oauth: OpenAI Codex token %s response missing fields: %s", operation, body)
	}

	return codexToken{
		Access:  raw.AccessToken,
		Refresh: raw.RefreshToken,
		Expires: clock() + raw.ExpiresIn*1000,
	}, nil
}

func exchangeCodexAuthorizationCode(ctx context.Context, code, verifier, redirectURI string) (codexToken, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", codexClientID)
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("redirect_uri", redirectURI)
	return codexRequestToken(ctx, form, "exchange")
}

func refreshCodexAccessToken(ctx context.Context, refreshToken string) (codexToken, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", codexClientID)
	return codexRequestToken(ctx, form, "refresh")
}

// codexCredentialFromToken derives the stored credential from a raw token,
// extracting accountId into Extra -- matching upstream's
// credentialsFromToken, including its hard failure when no accountId can be
// extracted (a Codex access token always carries one).
func codexCredentialFromToken(tok codexToken) (*ai.OAuthCredential, error) {
	accountID, ok := codexDecodeAccountID(tok.Access)
	if !ok {
		return nil, fmt.Errorf("oauth: failed to extract accountId from token")
	}
	return &ai.OAuthCredential{
		Access:  tok.Access,
		Refresh: tok.Refresh,
		Expires: tok.Expires,
		Extra:   map[string]any{"accountId": accountID},
	}, nil
}

func exchangeCodexAuthorizationCodeForCredential(ctx context.Context, code, verifier, redirectURI string) (*ai.OAuthCredential, error) {
	tok, err := exchangeCodexAuthorizationCode(ctx, code, verifier, redirectURI)
	if err != nil {
		return nil, err
	}
	return codexCredentialFromToken(tok)
}

// RefreshCodexToken exchanges a refresh token for a new Codex OAuth
// credential, re-deriving accountId from the refreshed access token.
func RefreshCodexToken(ctx context.Context, refreshToken string) (*ai.OAuthCredential, error) {
	tok, err := refreshCodexAccessToken(ctx, refreshToken)
	if err != nil {
		return nil, err
	}
	return codexCredentialFromToken(tok)
}

// codexDeviceAuthInfo is the device-code grant's initiation response,
// matching upstream's DeviceAuthInfo.
type codexDeviceAuthInfo struct {
	DeviceAuthID    string
	UserCode        string
	IntervalSeconds int
}

// startCodexDeviceAuth initiates the Codex device-code grant, matching
// upstream's startOpenAICodexDeviceAuth.
func startCodexDeviceAuth(ctx context.Context) (*codexDeviceAuthInfo, error) {
	payload, err := json.Marshal(map[string]any{"client_id": codexClientID})
	if err != nil {
		return nil, fmt.Errorf("oauth: encode device code request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, codexDeviceUserCodeURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("oauth: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ErrDeviceCodeCancelled
		}
		return nil, fmt.Errorf("oauth: http request failed. url=%s: %w", codexDeviceUserCodeURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("oauth: read response body: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.New("oauth: OpenAI Codex device code login is not enabled for this server. Use browser login or verify the server URL.")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := fmt.Sprintf("oauth: OpenAI Codex device code request failed with status %d", resp.StatusCode)
		if len(body) > 0 {
			msg += ": " + string(body)
		}
		return nil, errors.New(msg)
	}

	var raw struct {
		DeviceAuthID string              `json:"device_auth_id"`
		UserCode     string              `json:"user_code"`
		Interval     codexFlexibleNumber `json:"interval"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("oauth: invalid OpenAI Codex device code response: %s", body)
	}
	if raw.DeviceAuthID == "" || raw.UserCode == "" || raw.Interval < 0 {
		return nil, fmt.Errorf("oauth: invalid OpenAI Codex device code response: %s", body)
	}

	return &codexDeviceAuthInfo{
		DeviceAuthID:    raw.DeviceAuthID,
		UserCode:        raw.UserCode,
		IntervalSeconds: int(raw.Interval),
	}, nil
}

// codexDeviceAuthResult is the device-code poll's success value, matching
// upstream's DeviceTokenSuccess.
type codexDeviceAuthResult struct {
	AuthorizationCode string
	CodeVerifier      string
}

// pollCodexDeviceAuth polls the Codex device-auth token endpoint via the
// shared generic poller, matching upstream's pollOpenAICodexDeviceAuth: a
// 403/404 response is always "pending" regardless of body; otherwise a
// nested `error.code` of "deviceauth_authorization_pending"/"slow_down"
// drives the poll status, and any other response is a hard failure carrying
// the response body.
func pollCodexDeviceAuth(ctx context.Context, device *codexDeviceAuthInfo) (codexDeviceAuthResult, error) {
	return PollDeviceCodeFlow(ctx, DeviceCodePollOptions[codexDeviceAuthResult]{
		IntervalSeconds:  device.IntervalSeconds,
		ExpiresInSeconds: codexDeviceCodeTimeoutSeconds,
		Poll: func(ctx context.Context) (DeviceCodePollResult[codexDeviceAuthResult], error) {
			payload, err := json.Marshal(map[string]any{
				"device_auth_id": device.DeviceAuthID,
				"user_code":      device.UserCode,
			})
			if err != nil {
				return DeviceCodePollResult[codexDeviceAuthResult]{}, fmt.Errorf("oauth: encode device auth poll request: %w", err)
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, codexDeviceTokenURL, bytes.NewReader(payload))
			if err != nil {
				return DeviceCodePollResult[codexDeviceAuthResult]{}, fmt.Errorf("oauth: build request: %w", err)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := httpClient.Do(req)
			if err != nil {
				if ctx.Err() != nil {
					return DeviceCodePollResult[codexDeviceAuthResult]{}, ErrDeviceCodeCancelled
				}
				return DeviceCodePollResult[codexDeviceAuthResult]{}, fmt.Errorf("oauth: http request failed. url=%s: %w", codexDeviceTokenURL, err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return DeviceCodePollResult[codexDeviceAuthResult]{}, fmt.Errorf("oauth: read response body: %w", err)
			}

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				var raw struct {
					AuthorizationCode string `json:"authorization_code"`
					CodeVerifier      string `json:"code_verifier"`
				}
				if err := json.Unmarshal(body, &raw); err != nil || raw.AuthorizationCode == "" || raw.CodeVerifier == "" {
					return DeviceCodePollResult[codexDeviceAuthResult]{
						Status:  DeviceCodePollFailed,
						Message: fmt.Sprintf("oauth: invalid OpenAI Codex device auth token response: %s", body),
					}, nil
				}
				return DeviceCodePollResult[codexDeviceAuthResult]{
					Status: DeviceCodePollComplete,
					Value:  codexDeviceAuthResult{AuthorizationCode: raw.AuthorizationCode, CodeVerifier: raw.CodeVerifier},
				}, nil
			}

			if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
				return DeviceCodePollResult[codexDeviceAuthResult]{Status: DeviceCodePollPending}, nil
			}

			switch codexExtractErrorCode(body) {
			case "deviceauth_authorization_pending":
				return DeviceCodePollResult[codexDeviceAuthResult]{Status: DeviceCodePollPending}, nil
			case "slow_down":
				return DeviceCodePollResult[codexDeviceAuthResult]{Status: DeviceCodePollSlowDown}, nil
			default:
				msg := fmt.Sprintf("oauth: OpenAI Codex device auth failed with status %d", resp.StatusCode)
				if len(body) > 0 {
					msg += ": " + string(body)
				}
				return DeviceCodePollResult[codexDeviceAuthResult]{Status: DeviceCodePollFailed, Message: msg}, nil
			}
		},
	})
}

// codexExtractErrorCode reads a device-auth error response's `error` field,
// which upstream's server sends as either a bare string or `{code: string}`.
func codexExtractErrorCode(body []byte) string {
	var raw struct {
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || len(raw.Error) == 0 {
		return ""
	}
	var code string
	if err := json.Unmarshal(raw.Error, &code); err == nil {
		return code
	}
	var obj struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(raw.Error, &obj); err == nil {
		return obj.Code
	}
	return ""
}

// loginCodexDeviceCode runs the Codex device-code login path: initiate,
// notify the caller with the user code/verification URI, poll, and exchange
// the resulting authorization code for a credential -- matching upstream's
// loginOpenAICodexDeviceCode. verificationUri and expiresInSeconds are
// constants (codexDeviceVerificationURI, codexDeviceCodeTimeoutSeconds), not
// server-reported, matching upstream exactly.
func loginCodexDeviceCode(ctx context.Context, onDeviceCode func(userCode, verificationURI string, intervalSeconds, expiresInSeconds int)) (*ai.OAuthCredential, error) {
	device, err := startCodexDeviceAuth(ctx)
	if err != nil {
		return nil, err
	}

	onDeviceCode(device.UserCode, codexDeviceVerificationURI, device.IntervalSeconds, codexDeviceCodeTimeoutSeconds)

	result, err := pollCodexDeviceAuth(ctx, device)
	if err != nil {
		return nil, err
	}

	return exchangeCodexAuthorizationCodeForCredential(ctx, result.AuthorizationCode, result.CodeVerifier, codexDeviceRedirectURI)
}

// codexBrowserLoginOptions mirrors upstream loginOpenAICodex's callback
// shape, decoupled from ai.AuthLoginCallbacks so the callback-vs-manual-code
// race is testable without the full ai.OAuthAuth wiring (same design as
// anthropicLoginOptions).
type codexBrowserLoginOptions struct {
	OnAuth func(authURL, instructions string)
	// OnPrompt is the last-resort fallback used only when no
	// OnManualCodeInput is supplied (or it produced no code and the callback
	// server never fired either).
	OnPrompt func(ctx context.Context, message, placeholder string) (string, error)
	// OnManualCodeInput, when set, races the local callback server: whichever
	// resolves first wins.
	OnManualCodeInput func(ctx context.Context, redirectURI string) (string, error)
}

// loginCodexBrowser runs the Codex PKCE authorization-code flow on the local
// callback server (real port :1455, OS-assigned in tests), racing it against
// a manual-code prompt -- matching upstream's loginOpenAICodex. Unlike
// Anthropic's flow, Codex's own state is independently random (not the PKCE
// verifier reused as state), and the final exchange needs no separate
// "missing oauth state" guard: the callback server itself only ever delivers
// a code once its state already matched (see callback.go), and the token
// exchange call takes no state parameter at all.
func loginCodexBrowser(ctx context.Context, opts codexBrowserLoginOptions) (*ai.OAuthCredential, error) {
	verifier, challenge, err := GeneratePKCE()
	if err != nil {
		return nil, err
	}
	state, err := codexCreateState()
	if err != nil {
		return nil, err
	}

	server, err := StartCallbackServer(anthropicCallbackHost(), codexCallbackPort, codexCallbackPath, state)
	if err != nil {
		return nil, fmt.Errorf("oauth: start codex callback server: %w", err)
	}
	defer func() { _ = server.Close() }()

	redirectURI := server.RedirectURI

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", codexClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", codexScope)
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", "S256")
	params.Set("state", state)
	params.Set("id_token_add_organizations", "true")
	params.Set("codex_cli_simplified_flow", "true")
	params.Set("originator", codexOriginator)

	authURL := codexAuthorizeURL + "?" + params.Encode()
	if opts.OnAuth != nil {
		opts.OnAuth(authURL, "A browser window should open. Complete login to finish.")
	}

	var code string

	if opts.OnManualCodeInput != nil {
		type manualResult struct {
			input string
			err   error
		}
		manualCh := make(chan manualResult, 1)
		go func() {
			input, mErr := opts.OnManualCodeInput(ctx, redirectURI)
			manualCh <- manualResult{input, mErr}
			server.Cancel()
		}()

		result, waitErr := server.WaitForCode(ctx)
		if waitErr != nil {
			return nil, waitErr
		}

		if result != nil && result.Code != "" {
			code = result.Code
		} else {
			select {
			case m := <-manualCh:
				if m.err != nil {
					return nil, m.err
				}
				if m.input != "" {
					parsedCode, parsedState := parseAuthorizationInput(m.input)
					if parsedState != "" && parsedState != state {
						return nil, fmt.Errorf("oauth: state mismatch")
					}
					code = parsedCode
				}
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	} else {
		result, waitErr := server.WaitForCode(ctx)
		if waitErr != nil {
			return nil, waitErr
		}
		if result != nil && result.Code != "" {
			code = result.Code
		}
	}

	if code == "" {
		if opts.OnPrompt == nil {
			return nil, fmt.Errorf("oauth: missing authorization code")
		}
		input, err := opts.OnPrompt(ctx, "Paste the authorization code (or full redirect URL):", redirectURI)
		if err != nil {
			return nil, err
		}
		parsedCode, parsedState := parseAuthorizationInput(input)
		if parsedState != "" && parsedState != state {
			return nil, fmt.Errorf("oauth: state mismatch")
		}
		code = parsedCode
	}

	if code == "" {
		return nil, fmt.Errorf("oauth: missing authorization code")
	}

	return exchangeCodexAuthorizationCodeForCredential(ctx, code, verifier, redirectURI)
}

// CodexOAuth is the ai.OAuthAuth strategy for OpenAI Codex (ChatGPT
// Plus/Pro): a login-method select prompt choosing between the PKCE/browser
// flow (racing the local :1455 callback server against a manual-code
// prompt) and the device-code flow, token exchange, and refresh. Expires
// carries no 5-minute margin (the Codex exception).
var CodexOAuth = &ai.OAuthAuth{
	Name: "OpenAI (ChatGPT Plus/Pro)",

	Login: func(ctx context.Context, callbacks ai.AuthLoginCallbacks) (*ai.OAuthCredential, error) {
		method, err := callbacks.Prompt(ctx, ai.AuthPrompt{
			Type:    ai.AuthPromptSelect,
			Message: "Select OpenAI Codex login method:",
			Options: []ai.AuthPromptOption{
				{ID: codexBrowserLoginMethod, Label: "Browser login (default)"},
				{ID: codexDeviceCodeLoginMethod, Label: "Device code login (headless)"},
			},
		})
		if err != nil {
			return nil, err
		}

		switch method {
		case codexDeviceCodeLoginMethod:
			return loginCodexDeviceCode(ctx, func(userCode, verificationURI string, intervalSeconds, expiresInSeconds int) {
				callbacks.Notify(ai.AuthEvent{
					Type:             ai.AuthEventDeviceCode,
					UserCode:         userCode,
					VerificationURI:  verificationURI,
					IntervalSeconds:  intervalSeconds,
					ExpiresInSeconds: expiresInSeconds,
				})
			})

		case codexBrowserLoginMethod:
			// The manual_code prompt races the local callback server; its Ctx
			// is cancelled once login settles either way (matches upstream's
			// manualAbort.abort() in a finally block).
			manualCtx, cancelManual := context.WithCancel(ctx)
			defer cancelManual()

			return loginCodexBrowser(ctx, codexBrowserLoginOptions{
				OnAuth: func(authURL, instructions string) {
					callbacks.Notify(ai.AuthEvent{Type: ai.AuthEventAuthURL, URL: authURL, Instructions: instructions})
				},
				OnPrompt: func(_ context.Context, message, placeholder string) (string, error) {
					return callbacks.Prompt(ctx, ai.AuthPrompt{Type: ai.AuthPromptText, Message: message, Placeholder: placeholder})
				},
				OnManualCodeInput: func(_ context.Context, redirectURI string) (string, error) {
					return callbacks.Prompt(ctx, ai.AuthPrompt{
						Type:        ai.AuthPromptManualCode,
						Message:     "Complete login in your browser, or paste the authorization code / redirect URL here:",
						Placeholder: redirectURI,
						Ctx:         manualCtx,
					})
				},
			})

		default:
			return nil, fmt.Errorf("oauth: unknown OpenAI Codex login method: %q", method)
		}
	},

	Refresh: func(ctx context.Context, credential *ai.OAuthCredential) (*ai.OAuthCredential, error) {
		return RefreshCodexToken(ctx, credential.Refresh)
	},

	ToAuth: func(_ context.Context, credential *ai.OAuthCredential) (ai.ModelAuth, error) {
		return ai.ModelAuth{APIKey: credential.Access}, nil
	},
}
