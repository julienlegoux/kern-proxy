package oauth

// Ports: packages/ai/src/utils/oauth/anthropic.ts, packages/ai/src/utils/oauth/pkce.ts
// (via GeneratePKCE), packages/ai/src/utils/oauth/types.ts (OAuthAuth shape,
// as ai.OAuthAuth here). Device-code polling (device-code.ts) is not part of
// the Anthropic flow and is out of scope for this issue; see epic 12 issues
// 02 (Copilot) and 03 (Codex).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/kern-ia/kern-link/ai"
)

const (
	// anthropicClientID is upstream's atob-decoded client id literal
	// (decode("OWQxYzI1MGEtZTYxYi00NGQ5LTg4ZWQtNTk0NGQxOTYyZjVl")). The
	// base64 obfuscation exists upstream only to dodge naive bundler/secret
	// scanners in a browser-shipped bundle; Go source is never bundled that
	// way, so it's kept as a plain constant here.
	anthropicClientID     = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	anthropicAuthorizeURL = "https://claude.ai/oauth/authorize"
	anthropicCallbackPath = "/callback"
	anthropicScopes       = "org:create_api_key user:profile user:inference user:sessions:claude_code user:mcp_servers user:file_upload"
)

// anthropicTokenURL and anthropicCallbackPort are package vars, not
// constants, so tests can redirect the token exchange to an httptest server
// and bind the callback server on an OS-assigned port instead of the real
// :53692.
var (
	anthropicTokenURL     = "https://platform.claude.com/v1/oauth/token"
	anthropicCallbackPort = 53692
)

// anthropicCallbackHost lets PI_OAUTH_CALLBACK_HOST override the loopback
// host the callback server binds, matching upstream's getProviderEnvValue
// fallback.
func anthropicCallbackHost() string {
	if v := strings.TrimSpace(os.Getenv("PI_OAUTH_CALLBACK_HOST")); v != "" {
		return v
	}
	return "127.0.0.1"
}

// parseAuthorizationInput extracts code/state from a pasted redirect URL, a
// "code#state" fragment form, a bare query string, or a plain code — matching
// upstream's parseAuthorizationInput.
func parseAuthorizationInput(input string) (code, state string) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", ""
	}

	if u, err := url.Parse(value); err == nil && u.Scheme != "" && u.Host != "" {
		q := u.Query()
		return q.Get("code"), q.Get("state")
	}

	if strings.Contains(value, "#") {
		parts := strings.SplitN(value, "#", 2)
		return parts[0], parts[1]
	}

	if strings.Contains(value, "code=") {
		if q, err := url.ParseQuery(value); err == nil {
			return q.Get("code"), q.Get("state")
		}
	}

	return value, ""
}

// anthropicLoginOptions mirrors upstream loginAnthropic's callback shape,
// decoupled from ai.AuthLoginCallbacks so the callback-vs-manual-code race
// is testable without the full ai.OAuthAuth wiring.
type anthropicLoginOptions struct {
	OnAuth     func(authURL, instructions string)
	OnProgress func(message string)
	// OnPrompt is the last-resort fallback used only when no
	// OnManualCodeInput is supplied (or it produced no code and the callback
	// server never fired either).
	OnPrompt func(ctx context.Context, message, placeholder string) (string, error)
	// OnManualCodeInput, when set, races the local callback server: whichever
	// resolves first wins. It receives the callback server's redirect URI to
	// show as a placeholder/hint.
	OnManualCodeInput func(ctx context.Context, redirectURI string) (string, error)
}

// loginAnthropic runs the Anthropic PKCE authorization-code flow: it starts
// the local callback server, emits the authorize URL, then races the
// callback against OnManualCodeInput (if provided) before falling back to
// OnPrompt, and exchanges whatever code that race produced for tokens.
func loginAnthropic(ctx context.Context, opts anthropicLoginOptions) (*ai.OAuthCredential, error) {
	verifier, challenge, err := GeneratePKCE()
	if err != nil {
		return nil, err
	}

	server, err := StartCallbackServer(anthropicCallbackHost(), anthropicCallbackPort, anthropicCallbackPath, verifier)
	if err != nil {
		return nil, fmt.Errorf("oauth: start anthropic callback server: %w", err)
	}
	defer func() { _ = server.Close() }()

	redirectURI := server.RedirectURI
	exchangeRedirectURI := redirectURI

	authParams := url.Values{}
	authParams.Set("code", "true")
	authParams.Set("client_id", anthropicClientID)
	authParams.Set("response_type", "code")
	authParams.Set("redirect_uri", redirectURI)
	authParams.Set("scope", anthropicScopes)
	authParams.Set("code_challenge", challenge)
	authParams.Set("code_challenge_method", "S256")
	// Upstream reuses the PKCE verifier as the OAuth state token too (not
	// just for PKCE); ported verbatim even though the double duty is
	// unusual, since the callback server validates against this same value.
	authParams.Set("state", verifier)

	authURL := anthropicAuthorizeURL + "?" + authParams.Encode()
	if opts.OnAuth != nil {
		opts.OnAuth(authURL, "Complete login in your browser. If the browser is on another machine, paste the final redirect URL here.")
	}

	var code, state string

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
			code, state = result.Code, result.State
			exchangeRedirectURI = redirectURI
		} else {
			select {
			case m := <-manualCh:
				if m.err != nil {
					return nil, m.err
				}
				if m.input != "" {
					parsedCode, parsedState := parseAuthorizationInput(m.input)
					if parsedState != "" && parsedState != verifier {
						return nil, fmt.Errorf("oauth: state mismatch")
					}
					code = parsedCode
					state = parsedState
					if state == "" {
						state = verifier
					}
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
			code, state = result.Code, result.State
			exchangeRedirectURI = redirectURI
		}
	}

	if code == "" {
		if opts.OnPrompt == nil {
			return nil, fmt.Errorf("oauth: missing authorization code")
		}
		input, err := opts.OnPrompt(ctx, "Paste the authorization code or full redirect URL:", redirectURI)
		if err != nil {
			return nil, err
		}
		parsedCode, parsedState := parseAuthorizationInput(input)
		if parsedState != "" && parsedState != verifier {
			return nil, fmt.Errorf("oauth: state mismatch")
		}
		code = parsedCode
		state = parsedState
		if state == "" {
			state = verifier
		}
	}

	if code == "" {
		return nil, fmt.Errorf("oauth: missing authorization code")
	}
	if state == "" {
		return nil, fmt.Errorf("oauth: missing oauth state")
	}

	if opts.OnProgress != nil {
		opts.OnProgress("Exchanging authorization code for tokens...")
	}

	return exchangeAuthorizationCode(ctx, code, state, verifier, exchangeRedirectURI)
}

type anthropicTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

func exchangeAuthorizationCode(ctx context.Context, code, state, verifier, redirectURI string) (*ai.OAuthCredential, error) {
	respBody, err := PostJSON(ctx, anthropicTokenURL, map[string]any{
		"grant_type":    "authorization_code",
		"client_id":     anthropicClientID,
		"code":          code,
		"state":         state,
		"redirect_uri":  redirectURI,
		"code_verifier": verifier,
	})
	if err != nil {
		return nil, fmt.Errorf("oauth: anthropic token exchange: %w", err)
	}

	var tok anthropicTokenResponse
	if err := json.Unmarshal(respBody, &tok); err != nil {
		return nil, fmt.Errorf("oauth: anthropic token exchange returned invalid json: %w", err)
	}

	return &ai.OAuthCredential{
		Refresh: tok.RefreshToken,
		Access:  tok.AccessToken,
		Expires: clock() + tok.ExpiresIn*1000 - 5*60*1000,
	}, nil
}

// RefreshAnthropicToken exchanges a refresh token for a new Anthropic OAuth
// credential. The request omits `scope` (upstream's refresh grant does not
// resend it).
func RefreshAnthropicToken(ctx context.Context, refreshToken string) (*ai.OAuthCredential, error) {
	respBody, err := PostJSON(ctx, anthropicTokenURL, map[string]any{
		"grant_type":    "refresh_token",
		"client_id":     anthropicClientID,
		"refresh_token": refreshToken,
	})
	if err != nil {
		return nil, fmt.Errorf("oauth: anthropic token refresh: %w", err)
	}

	var tok anthropicTokenResponse
	if err := json.Unmarshal(respBody, &tok); err != nil {
		return nil, fmt.Errorf("oauth: anthropic token refresh returned invalid json: %w", err)
	}

	return &ai.OAuthCredential{
		Refresh: tok.RefreshToken,
		Access:  tok.AccessToken,
		Expires: clock() + tok.ExpiresIn*1000 - 5*60*1000,
	}, nil
}

// AnthropicOAuth is the ai.OAuthAuth strategy for Anthropic (Claude
// Pro/Max): PKCE login racing the local callback server against a
// manual-code prompt, token exchange, and refresh.
var AnthropicOAuth = &ai.OAuthAuth{
	Name: "Anthropic (Claude Pro/Max)",

	Login: func(ctx context.Context, callbacks ai.AuthLoginCallbacks) (*ai.OAuthCredential, error) {
		// The manual_code prompt races the local callback server; its Ctx is
		// cancelled once login settles either way, so a UI showing the
		// prompt can dismiss it (matches upstream's manualAbort.abort() in
		// a finally block).
		manualCtx, cancelManual := context.WithCancel(ctx)
		defer cancelManual()

		return loginAnthropic(ctx, anthropicLoginOptions{
			OnAuth: func(authURL, instructions string) {
				callbacks.Notify(ai.AuthEvent{Type: ai.AuthEventAuthURL, URL: authURL, Instructions: instructions})
			},
			OnProgress: func(message string) {
				callbacks.Notify(ai.AuthEvent{Type: ai.AuthEventProgress, Message: message})
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
	},

	Refresh: func(ctx context.Context, credential *ai.OAuthCredential) (*ai.OAuthCredential, error) {
		return RefreshAnthropicToken(ctx, credential.Refresh)
	},

	ToAuth: func(_ context.Context, credential *ai.OAuthCredential) (ai.ModelAuth, error) {
		return ai.ModelAuth{APIKey: credential.Access}, nil
	},
}
