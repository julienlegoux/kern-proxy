package oauth

// Ports: packages/ai/test/github-copilot-oauth.test.ts (device-code
// initiation, polling with slow_down/authorization_pending, and the token
// exchange/refresh/per-credential-baseUrl cases). The "filters models to the
// authenticated account picker catalog" case is not ported: it exercises
// upstream's post-login model-enabling/available-model-id bookkeeping
// (enableAllGitHubCopilotModels, fetchAvailableGitHubCopilotModelIds), which
// is this issue's policy/RefreshModels concern deferred to Epic 11's already
//-landed COPILOT_GITHUB_TOKEN-based ai/providers/github_copilot.go
// RefreshModels (see this issue's "Out of scope").

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/julienlegoux/kern-proxy/ai"
)

// withFakeCopilotURLs points copilotURLsFunc at a fixed set of URLs
// (typically all on one httptest server, differentiated by path) for the
// duration of the test and restores it afterward.
func withFakeCopilotURLs(t *testing.T, deviceCodeURL, accessTokenURL, copilotTokenURL string) {
	t.Helper()
	original := copilotURLsFunc
	copilotURLsFunc = func(string) (string, string, string) {
		return deviceCodeURL, accessTokenURL, copilotTokenURL
	}
	t.Cleanup(func() { copilotURLsFunc = original })
}

func TestNormalizeDomain(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"   ", ""},
		{"github.com", "github.com"},
		{"company.ghe.com", "company.ghe.com"},
		{"https://company.ghe.com/", "company.ghe.com"},
		{"not a domain", ""},
	}
	for _, tc := range cases {
		if got := normalizeDomain(tc.input); got != tc.want {
			t.Errorf("normalizeDomain(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestGetGitHubCopilotBaseURL(t *testing.T) {
	cases := []struct {
		name             string
		token            string
		enterpriseDomain string
		want             string
	}{
		{
			name:  "token carries proxy-ep",
			token: "tid=test;exp=9999999999;proxy-ep=proxy.individual.githubcopilot.com;",
			want:  "https://api.individual.githubcopilot.com",
		},
		{
			name:             "no proxy-ep falls back to enterprise domain",
			token:            "tid=test;exp=9999999999;",
			enterpriseDomain: "corp.ghe.com",
			want:             "https://copilot-api.corp.ghe.com",
		},
		{
			name: "no token, no enterprise domain",
			want: "https://api.individual.githubcopilot.com",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := getGitHubCopilotBaseURL(tc.token, tc.enterpriseDomain); got != tc.want {
				t.Errorf("getGitHubCopilotBaseURL(%q, %q) = %q, want %q", tc.token, tc.enterpriseDomain, got, tc.want)
			}
		})
	}
}

func TestStartCopilotDeviceFlow_SendsRequestAndParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", ct)
		}
		if accept := r.Header.Get("Accept"); accept != "application/json" {
			t.Errorf("Accept = %q, want application/json", accept)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if got := r.PostForm.Get("client_id"); got != copilotClientID {
			t.Errorf("client_id = %q, want %q", got, copilotClientID)
		}
		if got := r.PostForm.Get("scope"); got != "read:user" {
			t.Errorf("scope = %q, want read:user", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code":      "device-code",
			"user_code":        "ABCD-EFGH",
			"verification_uri": "https://github.com/login/device",
			"interval":         5,
			"expires_in":       900,
		})
	}))
	defer server.Close()

	withFakeCopilotURLs(t, server.URL, "", "")

	device, err := startCopilotDeviceFlow(context.Background(), "github.com")
	if err != nil {
		t.Fatalf("startCopilotDeviceFlow() error = %v", err)
	}
	if device.DeviceCode != "device-code" {
		t.Errorf("DeviceCode = %q, want device-code", device.DeviceCode)
	}
	if device.UserCode != "ABCD-EFGH" {
		t.Errorf("UserCode = %q, want ABCD-EFGH", device.UserCode)
	}
	if device.VerificationURI != "https://github.com/login/device" {
		t.Errorf("VerificationURI = %q, want https://github.com/login/device", device.VerificationURI)
	}
	if device.IntervalSeconds != 5 {
		t.Errorf("IntervalSeconds = %d, want 5", device.IntervalSeconds)
	}
	if device.ExpiresInSeconds != 900 {
		t.Errorf("ExpiresInSeconds = %d, want 900", device.ExpiresInSeconds)
	}
}

func TestStartCopilotDeviceFlow_RejectsUntrustedVerificationURI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code":      "device-code",
			"user_code":        "ABCD-EFGH",
			"verification_uri": "javascript:alert(1)",
			"interval":         5,
			"expires_in":       900,
		})
	}))
	defer server.Close()

	withFakeCopilotURLs(t, server.URL, "", "")

	_, err := startCopilotDeviceFlow(context.Background(), "github.com")
	if err == nil {
		t.Fatal("startCopilotDeviceFlow() error = nil, want an untrusted verification_uri error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "untrusted verification_uri") {
		t.Errorf("error = %q, want it to mention untrusted verification_uri", err.Error())
	}
}

func TestStartCopilotDeviceFlow_NormalizesVerificationURI(t *testing.T) {
	rawURI := "https://github.com/login/device oauth"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code":      "device-code",
			"user_code":        "ABCD-EFGH",
			"verification_uri": rawURI,
			"interval":         5,
			"expires_in":       900,
		})
	}))
	defer server.Close()

	withFakeCopilotURLs(t, server.URL, "", "")

	device, err := startCopilotDeviceFlow(context.Background(), "github.com")
	if err != nil {
		t.Fatalf("startCopilotDeviceFlow() error = %v", err)
	}
	want, parseErr := url.Parse(rawURI)
	if parseErr != nil {
		t.Fatalf("url.Parse(%q) error = %v", rawURI, parseErr)
	}
	if device.VerificationURI != want.String() {
		t.Errorf("VerificationURI = %q, want normalized %q", device.VerificationURI, want.String())
	}
	if device.VerificationURI == rawURI {
		t.Errorf("VerificationURI = %q, want it normalized (different from raw)", device.VerificationURI)
	}
}

func TestPollForGitHubAccessToken_WaitsAndAppliesSlowDown(t *testing.T) {
	withVirtualDeviceCodeClock(t, 0)

	responses := []map[string]any{
		{"error": "authorization_pending"},
		{"error": "slow_down", "interval": 7},
		{"access_token": "ghu_refresh_token"},
	}
	var pollTimes []int64
	idx := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pollTimes = append(pollTimes, clock())
		if idx >= len(responses) {
			t.Fatalf("unexpected extra poll at index %d", idx)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responses[idx])
		idx++
	}))
	defer server.Close()

	withFakeCopilotURLs(t, "", server.URL, "")

	device := &copilotDeviceCodeResponse{
		DeviceCode:       "device-code",
		UserCode:         "ABCD-EFGH",
		VerificationURI:  "https://github.com/login/device",
		IntervalSeconds:  5,
		ExpiresInSeconds: 900,
	}
	token, err := pollForGitHubAccessToken(context.Background(), "github.com", device)
	if err != nil {
		t.Fatalf("pollForGitHubAccessToken() error = %v", err)
	}
	if token != "ghu_refresh_token" {
		t.Errorf("token = %q, want ghu_refresh_token", token)
	}
	if want := []int64{5000, 10000, 17000}; !int64SliceEqual(pollTimes, want) {
		t.Errorf("pollTimes = %v, want %v", pollTimes, want)
	}
}

func TestPollForGitHubAccessToken_TimesOutAfterRepeatedSlowDown(t *testing.T) {
	withVirtualDeviceCodeClock(t, 0)

	responses := []map[string]any{
		{"error": "slow_down"},
		{"error": "slow_down"},
	}
	var pollTimes []int64
	idx := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pollTimes = append(pollTimes, clock())
		if idx >= len(responses) {
			t.Fatalf("unexpected extra poll at index %d", idx)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responses[idx])
		idx++
	}))
	defer server.Close()

	withFakeCopilotURLs(t, "", server.URL, "")

	device := &copilotDeviceCodeResponse{
		DeviceCode:       "device-code",
		UserCode:         "ABCD-EFGH",
		VerificationURI:  "https://github.com/login/device",
		IntervalSeconds:  5,
		ExpiresInSeconds: 25,
	}
	_, err := pollForGitHubAccessToken(context.Background(), "github.com", device)
	if err == nil {
		t.Fatal("pollForGitHubAccessToken() error = nil, want a slow_down timeout error")
	}
	if !strings.Contains(err.Error(), "slow_down") {
		t.Errorf("error = %q, want it to mention slow_down", err.Error())
	}
	if want := []int64{5000, 15000}; !int64SliceEqual(pollTimes, want) {
		t.Errorf("pollTimes = %v, want %v", pollTimes, want)
	}
}

func int64SliceEqual(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestExchangeCopilotToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer ghu_refresh_token" {
			t.Errorf("Authorization = %q, want Bearer ghu_refresh_token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      "tid=test;exp=9999999999;proxy-ep=proxy.individual.githubcopilot.com;",
			"expires_at": 9999999999,
		})
	}))
	defer server.Close()

	withFakeCopilotURLs(t, "", "", server.URL)

	cred, err := exchangeCopilotToken(context.Background(), "ghu_refresh_token", "")
	if err != nil {
		t.Fatalf("exchangeCopilotToken() error = %v", err)
	}
	if cred.Refresh != "ghu_refresh_token" {
		t.Errorf("Refresh = %q, want ghu_refresh_token", cred.Refresh)
	}
	if cred.Access != "tid=test;exp=9999999999;proxy-ep=proxy.individual.githubcopilot.com;" {
		t.Errorf("Access = %q, want the copilot token", cred.Access)
	}
	wantExpires := int64(9999999999)*1000 - 5*60*1000
	if cred.Expires != wantExpires {
		t.Errorf("Expires = %d, want %d", cred.Expires, wantExpires)
	}
	if cred.Extra != nil {
		t.Errorf("Extra = %v, want nil with no enterprise domain", cred.Extra)
	}
}

func TestExchangeCopilotToken_PreservesEnterpriseDomain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      "tid=test;exp=9999999999;",
			"expires_at": 9999999999,
		})
	}))
	defer server.Close()

	withFakeCopilotURLs(t, "", "", server.URL)

	cred, err := exchangeCopilotToken(context.Background(), "ghu_refresh_token", "corp.ghe.com")
	if err != nil {
		t.Fatalf("exchangeCopilotToken() error = %v", err)
	}
	if got, _ := cred.Extra["enterpriseUrl"].(string); got != "corp.ghe.com" {
		t.Errorf("Extra[enterpriseUrl] = %v, want corp.ghe.com", cred.Extra["enterpriseUrl"])
	}
}

func TestCopilotOAuth_Login(t *testing.T) {
	withVirtualDeviceCodeClock(t, 0)

	mux := http.NewServeMux()
	mux.HandleFunc("/login/device/code", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code":      "device-code",
			"user_code":        "ABCD-EFGH",
			"verification_uri": "https://github.com/login/device",
			"interval":         1,
			"expires_in":       900,
		})
	})
	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "ghu_refresh_token"})
	})
	mux.HandleFunc("/copilot_internal/v2/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      "tid=test;exp=9999999999;proxy-ep=proxy.individual.githubcopilot.com;",
			"expires_at": 9999999999,
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	original := copilotURLsFunc
	copilotURLsFunc = func(domain string) (string, string, string) {
		return server.URL + "/login/device/code", server.URL + "/login/oauth/access_token", server.URL + "/copilot_internal/v2/token"
	}
	t.Cleanup(func() { copilotURLsFunc = original })

	var events []ai.AuthEvent
	callbacks := ai.AuthLoginCallbacks{
		Prompt: func(context.Context, ai.AuthPrompt) (string, error) { return "", nil },
		Notify: func(event ai.AuthEvent) { events = append(events, event) },
	}

	cred, err := CopilotOAuth.Login(context.Background(), callbacks)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if cred.Refresh != "ghu_refresh_token" {
		t.Errorf("Refresh = %q, want ghu_refresh_token", cred.Refresh)
	}
	if cred.Access != "tid=test;exp=9999999999;proxy-ep=proxy.individual.githubcopilot.com;" {
		t.Errorf("Access = %q, want the copilot token", cred.Access)
	}
	if cred.Extra != nil {
		t.Errorf("Extra = %v, want nil (github.com, no enterprise domain)", cred.Extra)
	}

	var deviceCodeEvent *ai.AuthEvent
	for i := range events {
		if events[i].Type == ai.AuthEventDeviceCode {
			deviceCodeEvent = &events[i]
		}
	}
	if deviceCodeEvent == nil {
		t.Fatal("no AuthEventDeviceCode was notified")
	}
	if deviceCodeEvent.UserCode != "ABCD-EFGH" {
		t.Errorf("UserCode = %q, want ABCD-EFGH", deviceCodeEvent.UserCode)
	}
	if deviceCodeEvent.VerificationURI != "https://github.com/login/device" {
		t.Errorf("VerificationURI = %q, want https://github.com/login/device", deviceCodeEvent.VerificationURI)
	}
	if deviceCodeEvent.IntervalSeconds != 1 {
		t.Errorf("IntervalSeconds = %d, want 1", deviceCodeEvent.IntervalSeconds)
	}
	if deviceCodeEvent.ExpiresInSeconds != 900 {
		t.Errorf("ExpiresInSeconds = %d, want 900", deviceCodeEvent.ExpiresInSeconds)
	}
}

func TestCopilotOAuth_LoginWithEnterpriseDomain(t *testing.T) {
	withVirtualDeviceCodeClock(t, 0)

	var sawDomain string
	mux := http.NewServeMux()
	mux.HandleFunc("/login/device/code", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code":      "device-code",
			"user_code":        "ABCD-EFGH",
			"verification_uri": "https://corp.ghe.com/login/device",
			"interval":         1,
			"expires_in":       900,
		})
	})
	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "ghu_refresh_token"})
	})
	mux.HandleFunc("/copilot_internal/v2/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      "tid=test;exp=9999999999;",
			"expires_at": 9999999999,
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	original := copilotURLsFunc
	copilotURLsFunc = func(domain string) (string, string, string) {
		sawDomain = domain
		return server.URL + "/login/device/code", server.URL + "/login/oauth/access_token", server.URL + "/copilot_internal/v2/token"
	}
	t.Cleanup(func() { copilotURLsFunc = original })

	callbacks := ai.AuthLoginCallbacks{
		Prompt: func(context.Context, ai.AuthPrompt) (string, error) { return "corp.ghe.com", nil },
		Notify: func(ai.AuthEvent) {},
	}

	cred, err := CopilotOAuth.Login(context.Background(), callbacks)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if sawDomain != "corp.ghe.com" {
		t.Errorf("domain used = %q, want corp.ghe.com", sawDomain)
	}
	if got, _ := cred.Extra["enterpriseUrl"].(string); got != "corp.ghe.com" {
		t.Errorf("Extra[enterpriseUrl] = %v, want corp.ghe.com", cred.Extra["enterpriseUrl"])
	}
}

func TestCopilotOAuth_Refresh(t *testing.T) {
	var sawAuth, sawDomain string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      "tid=new;exp=9999999999;",
			"expires_at": 9999999999,
		})
	}))
	defer server.Close()

	original := copilotURLsFunc
	copilotURLsFunc = func(domain string) (string, string, string) {
		sawDomain = domain
		return "", "", server.URL
	}
	t.Cleanup(func() { copilotURLsFunc = original })

	credential := &ai.OAuthCredential{
		Refresh: "ghu_stored_refresh",
		Extra:   map[string]any{"enterpriseUrl": "corp.ghe.com"},
	}
	refreshed, err := CopilotOAuth.Refresh(context.Background(), credential)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if sawAuth != "Bearer ghu_stored_refresh" {
		t.Errorf("Authorization = %q, want Bearer ghu_stored_refresh", sawAuth)
	}
	if sawDomain != "corp.ghe.com" {
		t.Errorf("domain used = %q, want corp.ghe.com", sawDomain)
	}
	if refreshed.Access != "tid=new;exp=9999999999;" {
		t.Errorf("Access = %q, want the refreshed token", refreshed.Access)
	}
}

func TestCopilotOAuth_ToAuth(t *testing.T) {
	cases := []struct {
		name       string
		credential *ai.OAuthCredential
		wantBase   string
	}{
		{
			name:       "derives baseUrl from token proxy-ep",
			credential: &ai.OAuthCredential{Access: "tid=test;exp=9999999999;proxy-ep=proxy.individual.githubcopilot.com;"},
			wantBase:   "https://api.individual.githubcopilot.com",
		},
		{
			name: "falls back to per-credential enterprise domain",
			credential: &ai.OAuthCredential{
				Access: "tid=test;exp=9999999999;",
				Extra:  map[string]any{"enterpriseUrl": "corp.ghe.com"},
			},
			wantBase: "https://copilot-api.corp.ghe.com",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth, err := CopilotOAuth.ToAuth(context.Background(), tc.credential)
			if err != nil {
				t.Fatalf("ToAuth() error = %v", err)
			}
			if auth.APIKey != tc.credential.Access {
				t.Errorf("APIKey = %q, want %q", auth.APIKey, tc.credential.Access)
			}
			if auth.BaseURL != tc.wantBase {
				t.Errorf("BaseURL = %q, want %q", auth.BaseURL, tc.wantBase)
			}
		})
	}
}
