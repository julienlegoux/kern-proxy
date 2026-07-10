package oauth

// Ports: packages/ai/src/utils/oauth/github-copilot.ts -- the device-code
// grant, GitHub-token -> Copilot-token exchange, and per-credential baseUrl
// derived from the Copilot token's proxy-ep. Reuses the generic device-code
// poller (devicecode.go, ported from device-code.ts as this issue's first
// consumer).
//
// Not ported (deferred to Epic 11's already-landed policy/RefreshModels
// handling in ai/providers/github_copilot.go, per this issue's "Out of
// scope"): enableGitHubCopilotModel/enableAllGitHubCopilotModels (the
// post-login model-policy-enable calls) and
// fetchAvailableGitHubCopilotModelIds/isSelectableCopilotModel/
// CopilotCredentials.availableModelIds (the OAuth-credential-scoped model
// picker filtering upstream's githubCopilotOAuthProvider.modifyModels
// applies) -- ai/providers/github_copilot.go's RefreshModels already
// implements the same isSelectableCopilotModel filtering rule against the
// COPILOT_GITHUB_TOKEN env fallback; wiring it to the OAuth-derived token
// too is a natural follow-up once this flow is consumed (Epic 14).

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/julienlegoux/kern-link/ai"
)

// copilotClientID is upstream's atob-decoded client id literal
// (decode("SXYxLmI1MDdhMDhjODdlY2ZlOTg=")); see anthropic.go's
// anthropicClientID comment for why the base64 obfuscation is dropped here.
const copilotClientID = "Iv1.b507a08c87ecfe98"

// copilotHeaders are sent on every Copilot-branded request (device code,
// access token, and the Copilot token exchange/refresh), matching
// upstream's COPILOT_HEADERS.
var copilotHeaders = map[string]string{
	"User-Agent":             "GitHubCopilotChat/0.35.0",
	"Editor-Version":         "vscode/1.107.0",
	"Editor-Plugin-Version":  "copilot-chat/0.35.0",
	"Copilot-Integration-Id": "vscode-chat",
}

// copilotURLsFunc builds the three endpoint URLs for a given GitHub
// domain, matching upstream's getUrls. It's a package var so tests can
// redirect every request to an httptest server instead of the real
// github.com / api.github.com hosts.
var copilotURLsFunc = func(domain string) (deviceCodeURL, accessTokenURL, copilotTokenURL string) {
	return fmt.Sprintf("https://%s/login/device/code", domain),
		fmt.Sprintf("https://%s/login/oauth/access_token", domain),
		fmt.Sprintf("https://api.%s/copilot_internal/v2/token", domain)
}

// copilotProxyEpPattern extracts the proxy-ep field from a Copilot access
// token, matching upstream's /proxy-ep=([^;]+)/.
var copilotProxyEpPattern = regexp.MustCompile(`proxy-ep=([^;]+)`)

// normalizeDomain trims and validates a user-entered GitHub Enterprise
// URL/domain, returning "" for a blank or unparsable value -- matching
// upstream's normalizeDomain (which returns null in both cases; callers
// distinguish "blank is fine" from "non-blank and invalid" the same way
// upstream's loginGitHubCopilot does).
func normalizeDomain(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	raw := trimmed
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return ""
	}
	return u.Hostname()
}

// getBaseUrlFromToken extracts the proxy-ep field from a Copilot access
// token and converts it to an API host, matching upstream's
// getBaseUrlFromToken. Returns "" if the token carries no proxy-ep.
func getBaseUrlFromToken(token string) string {
	m := copilotProxyEpPattern.FindStringSubmatch(token)
	if m == nil {
		return ""
	}
	proxyHost := m[1]
	// Matches upstream's proxyHost.replace(/^proxy\./, "api."): swap a
	// leading "proxy." for "api.", or leave the host unchanged if it has no
	// such prefix.
	apiHost := proxyHost
	if strings.HasPrefix(proxyHost, "proxy.") {
		apiHost = "api." + strings.TrimPrefix(proxyHost, "proxy.")
	}
	return "https://" + apiHost
}

// getGitHubCopilotBaseURL derives the Copilot API base URL: the token's own
// proxy-ep wins when present, otherwise an enterprise domain, otherwise the
// individual-plan default -- matching upstream's getGitHubCopilotBaseUrl.
func getGitHubCopilotBaseURL(token, enterpriseDomain string) string {
	if token != "" {
		if fromToken := getBaseUrlFromToken(token); fromToken != "" {
			return fromToken
		}
	}
	if enterpriseDomain != "" {
		return "https://copilot-api." + enterpriseDomain
	}
	return "https://api.individual.githubcopilot.com"
}

// copilotDeviceCodeResponse is GitHub's device-code grant initiation
// response, matching upstream's DeviceCodeResponse.
type copilotDeviceCodeResponse struct {
	DeviceCode       string
	UserCode         string
	VerificationURI  string
	IntervalSeconds  int
	ExpiresInSeconds int
}

type rawCopilotDeviceCodeResponse struct {
	DeviceCode       string `json:"device_code"`
	UserCode         string `json:"user_code"`
	VerificationURI  string `json:"verification_uri"`
	Interval         *int   `json:"interval"`
	ExpiresInSeconds int    `json:"expires_in"`
}

// copilotFetchJSON performs an HTTP request and returns its raw JSON body,
// erroring on a non-2xx status -- matching upstream's fetchJson.
func copilotFetchJSON(ctx context.Context, method, requestURL string, headers map[string]string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return nil, fmt.Errorf("oauth: build request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth: http request failed. url=%s: %w", requestURL, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("oauth: read response body. url=%s: %w", requestURL, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("oauth: %d %s: %s", resp.StatusCode, resp.Status, respBody)
	}
	return respBody, nil
}

// startCopilotDeviceFlow initiates the device-code grant against domain's
// /login/device/code endpoint, matching upstream's startDeviceFlow. The
// verification_uri is parsed and re-serialized (rejecting anything but an
// http(s) scheme) before it's ever handed to a caller that might launch it
// in a browser.
func startCopilotDeviceFlow(ctx context.Context, domain string) (*copilotDeviceCodeResponse, error) {
	deviceCodeURL, _, _ := copilotURLsFunc(domain)

	form := url.Values{}
	form.Set("client_id", copilotClientID)
	form.Set("scope", "read:user")

	headers := map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/x-www-form-urlencoded",
		"User-Agent":   copilotHeaders["User-Agent"],
	}
	respBody, err := copilotFetchJSON(ctx, http.MethodPost, deviceCodeURL, headers, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("oauth: copilot device code request: %w", err)
	}

	var raw rawCopilotDeviceCodeResponse
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("oauth: invalid device code response: %w", err)
	}
	if raw.DeviceCode == "" || raw.UserCode == "" || raw.VerificationURI == "" {
		return nil, fmt.Errorf("oauth: invalid device code response fields")
	}

	parsedURI, err := url.Parse(raw.VerificationURI)
	if err != nil || (parsedURI.Scheme != "http" && parsedURI.Scheme != "https") {
		return nil, fmt.Errorf("oauth: untrusted verification_uri in device code response")
	}

	interval := 0
	if raw.Interval != nil {
		interval = *raw.Interval
	}

	return &copilotDeviceCodeResponse{
		DeviceCode:       raw.DeviceCode,
		UserCode:         raw.UserCode,
		VerificationURI:  parsedURI.String(),
		IntervalSeconds:  interval,
		ExpiresInSeconds: raw.ExpiresInSeconds,
	}, nil
}

type copilotDeviceTokenResponse struct {
	AccessToken string `json:"access_token"`
	Error       string `json:"error"`
	Description string `json:"error_description"`
	Interval    *int   `json:"interval"`
}

// pollForGitHubAccessToken polls domain's /login/oauth/access_token
// endpoint for the device code's corresponding GitHub access token,
// matching upstream's pollForGitHubAccessToken.
func pollForGitHubAccessToken(ctx context.Context, domain string, device *copilotDeviceCodeResponse) (string, error) {
	_, accessTokenURL, _ := copilotURLsFunc(domain)

	headers := map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/x-www-form-urlencoded",
		"User-Agent":   copilotHeaders["User-Agent"],
	}

	return PollDeviceCodeFlow(ctx, DeviceCodePollOptions[string]{
		IntervalSeconds:     device.IntervalSeconds,
		ExpiresInSeconds:    device.ExpiresInSeconds,
		WaitBeforeFirstPoll: true,
		Poll: func(ctx context.Context) (DeviceCodePollResult[string], error) {
			form := url.Values{}
			form.Set("client_id", copilotClientID)
			form.Set("device_code", device.DeviceCode)
			form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

			respBody, err := copilotFetchJSON(ctx, http.MethodPost, accessTokenURL, headers, strings.NewReader(form.Encode()))
			if err != nil {
				return DeviceCodePollResult[string]{}, err
			}

			var raw copilotDeviceTokenResponse
			if err := json.Unmarshal(respBody, &raw); err != nil {
				return DeviceCodePollResult[string]{}, fmt.Errorf("oauth: invalid device token response: %w", err)
			}

			if raw.AccessToken != "" {
				return DeviceCodePollResult[string]{Status: DeviceCodePollComplete, Value: raw.AccessToken}, nil
			}
			switch raw.Error {
			case "authorization_pending":
				return DeviceCodePollResult[string]{Status: DeviceCodePollPending}, nil
			case "slow_down":
				interval := 0
				if raw.Interval != nil {
					interval = *raw.Interval
				}
				return DeviceCodePollResult[string]{Status: DeviceCodePollSlowDown, IntervalSeconds: interval}, nil
			case "":
				return DeviceCodePollResult[string]{Status: DeviceCodePollFailed, Message: "oauth: invalid device token response"}, nil
			default:
				msg := "oauth: device flow failed: " + raw.Error
				if raw.Description != "" {
					msg += ": " + raw.Description
				}
				return DeviceCodePollResult[string]{Status: DeviceCodePollFailed, Message: msg}, nil
			}
		},
	})
}

type copilotTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

// exchangeCopilotToken exchanges a GitHub access token (the device flow's
// result, or a previously stored one on refresh) for a short-lived Copilot
// API token, matching upstream's refreshGitHubCopilotAccessToken. expires is
// epoch-ms minus the 5-minute margin, matching every other flow in this
// package.
func exchangeCopilotToken(ctx context.Context, githubAccessToken, enterpriseDomain string) (*ai.OAuthCredential, error) {
	_, _, copilotTokenURL := copilotURLsFunc(enterpriseDomainOrDefault(enterpriseDomain))

	headers := map[string]string{
		"Accept":        "application/json",
		"Authorization": "Bearer " + githubAccessToken,
	}
	for k, v := range copilotHeaders {
		headers[k] = v
	}

	respBody, err := copilotFetchJSON(ctx, http.MethodGet, copilotTokenURL, headers, nil)
	if err != nil {
		return nil, fmt.Errorf("oauth: copilot token exchange: %w", err)
	}

	var raw copilotTokenResponse
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("oauth: invalid copilot token response: %w", err)
	}
	if raw.Token == "" || raw.ExpiresAt == 0 {
		return nil, fmt.Errorf("oauth: invalid copilot token response fields")
	}

	var extra map[string]any
	if enterpriseDomain != "" {
		extra = map[string]any{"enterpriseUrl": enterpriseDomain}
	}

	return &ai.OAuthCredential{
		Refresh: githubAccessToken,
		Access:  raw.Token,
		Expires: raw.ExpiresAt*1000 - 5*60*1000,
		Extra:   extra,
	}, nil
}

// enterpriseDomainOrDefault mirrors upstream's `enterpriseDomain || "github.com"`.
func enterpriseDomainOrDefault(enterpriseDomain string) string {
	if enterpriseDomain != "" {
		return enterpriseDomain
	}
	return "github.com"
}

// copilotCredentialEnterpriseDomain re-normalizes the stored enterpriseUrl,
// matching upstream's copilotEnterpriseDomain (which re-runs normalizeDomain
// even though the stored value is already normalized, in case an older
// stored credential predates normalization).
func copilotCredentialEnterpriseDomain(credential *ai.OAuthCredential) string {
	if credential == nil || credential.Extra == nil {
		return ""
	}
	enterpriseURL, _ := credential.Extra["enterpriseUrl"].(string)
	return normalizeDomain(enterpriseURL)
}

// CopilotOAuth is the ai.OAuthAuth strategy for GitHub Copilot: device-code
// login (with an enterprise-domain prompt first), the GitHub-token ->
// Copilot-token exchange, refresh, and a per-credential baseUrl derived
// from the Copilot token's proxy-ep (or the stored enterprise domain).
var CopilotOAuth = &ai.OAuthAuth{
	Name: "GitHub Copilot",

	Login: func(ctx context.Context, callbacks ai.AuthLoginCallbacks) (*ai.OAuthCredential, error) {
		input, err := callbacks.Prompt(ctx, ai.AuthPrompt{
			Type:        ai.AuthPromptText,
			Message:     "GitHub Enterprise URL/domain (blank for github.com)",
			Placeholder: "company.ghe.com",
		})
		if err != nil {
			return nil, err
		}

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		trimmed := strings.TrimSpace(input)
		enterpriseDomain := normalizeDomain(input)
		if trimmed != "" && enterpriseDomain == "" {
			return nil, fmt.Errorf("oauth: invalid GitHub Enterprise URL/domain")
		}
		domain := enterpriseDomainOrDefault(enterpriseDomain)

		device, err := startCopilotDeviceFlow(ctx, domain)
		if err != nil {
			return nil, err
		}

		callbacks.Notify(ai.AuthEvent{
			Type:             ai.AuthEventDeviceCode,
			UserCode:         device.UserCode,
			VerificationURI:  device.VerificationURI,
			IntervalSeconds:  device.IntervalSeconds,
			ExpiresInSeconds: device.ExpiresInSeconds,
		})

		githubAccessToken, err := pollForGitHubAccessToken(ctx, domain, device)
		if err != nil {
			return nil, err
		}

		return exchangeCopilotToken(ctx, githubAccessToken, enterpriseDomain)
	},

	Refresh: func(ctx context.Context, credential *ai.OAuthCredential) (*ai.OAuthCredential, error) {
		return exchangeCopilotToken(ctx, credential.Refresh, copilotCredentialEnterpriseDomain(credential))
	},

	ToAuth: func(_ context.Context, credential *ai.OAuthCredential) (ai.ModelAuth, error) {
		domain := copilotCredentialEnterpriseDomain(credential)
		return ai.ModelAuth{
			APIKey:  credential.Access,
			BaseURL: getGitHubCopilotBaseURL(credential.Access, domain),
		}, nil
	},
}
