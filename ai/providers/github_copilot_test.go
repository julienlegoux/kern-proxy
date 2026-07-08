package providers

// Tests the native RefreshModels behavior described in github_copilot.go's
// doc comment: without a COPILOT_GITHUB_TOKEN, return the static catalog
// unchanged; with one, narrow it down to models selectable under the
// account's live Copilot policy.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/catalog"
)

func TestGitHubCopilotProviderIsDynamic(t *testing.T) {
	provider := GitHubCopilotProvider()
	if !provider.CanRefreshModels() {
		t.Fatal("CanRefreshModels() = false, want true")
	}
}

func TestGitHubCopilotProviderAdvertisesOAuthPendingEpic12(t *testing.T) {
	provider := GitHubCopilotProvider()
	oauth := provider.Auth().OAuth
	if oauth == nil {
		t.Fatal("Auth().OAuth is nil, want a pending-epic-12 stub")
	}
	if oauth.Name != "GitHub Copilot" {
		t.Errorf("oauth.Name = %q", oauth.Name)
	}
	if _, err := oauth.Login(context.Background(), ai.AuthLoginCallbacks{}); err == nil {
		t.Error("Login() = nil error, want a not-implemented error")
	}
	if _, err := oauth.Refresh(context.Background(), &ai.OAuthCredential{}); err == nil {
		t.Error("Refresh() = nil error, want a not-implemented error")
	}
	if _, err := oauth.ToAuth(context.Background(), &ai.OAuthCredential{}); err == nil {
		t.Error("ToAuth() = nil error, want a not-implemented error")
	}
}

func TestRefreshGitHubCopilotModelsWithoutTokenReturnsCatalogUnchanged(t *testing.T) {
	t.Setenv("COPILOT_GITHUB_TOKEN", "")

	models, err := refreshGitHubCopilotModels(context.Background())
	if err != nil {
		t.Fatalf("refreshGitHubCopilotModels: %v", err)
	}
	catalogModels := catalog.BuiltinModels("github-copilot")
	if len(models) != len(catalogModels) {
		t.Fatalf("len(models) = %d, want %d (unchanged catalog)", len(models), len(catalogModels))
	}
}

func TestRefreshGitHubCopilotModelsAppliesPolicyFiltering(t *testing.T) {
	catalogModels := catalog.BuiltinModels("github-copilot")
	if len(catalogModels) < 2 {
		t.Fatalf("github-copilot catalog has %d models, need >= 2 to exercise filtering", len(catalogModels))
	}
	enabledID := catalogModels[0].ID
	disabledID := catalogModels[1].ID

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization header = %q", got)
		}
		if r.URL.Path != "/models" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{
					"id": "` + enabledID + `",
					"model_picker_enabled": true,
					"policy": {"state": "enabled"},
					"capabilities": {"supports": {"tool_calls": true}}
				},
				{
					"id": "` + disabledID + `",
					"model_picker_enabled": true,
					"policy": {"state": "disabled"}
				},
				{
					"id": "not-in-catalog",
					"model_picker_enabled": true
				}
			]
		}`))
	}))
	defer srv.Close()

	t.Setenv("COPILOT_GITHUB_TOKEN", "test-token")
	orig := githubCopilotModelsBaseURL
	githubCopilotModelsBaseURL = srv.URL
	defer func() { githubCopilotModelsBaseURL = orig }()

	models, err := refreshGitHubCopilotModels(context.Background())
	if err != nil {
		t.Fatalf("refreshGitHubCopilotModels: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(models))
	}
	if models[0].ID != enabledID {
		t.Errorf("models[0].ID = %q, want %q", models[0].ID, enabledID)
	}
}

func TestIsSelectableCopilotModel(t *testing.T) {
	falseVal := false
	trueVal := true

	cases := []struct {
		name string
		m    githubCopilotModelPolicy
		want bool
	}{
		{"picker disabled", githubCopilotModelPolicy{ModelPickerEnabled: false}, false},
		{"no policy or capabilities", githubCopilotModelPolicy{ModelPickerEnabled: true}, true},
		{
			"policy state disabled",
			githubCopilotModelPolicy{
				ModelPickerEnabled: true,
				Policy:             &githubCopilotModelPolicyDetail{State: "disabled"},
			},
			false,
		},
		{
			"tool_calls explicitly false",
			githubCopilotModelPolicy{
				ModelPickerEnabled: true,
				Capabilities: &githubCopilotModelCapabilities{
					Supports: &githubCopilotModelSupports{ToolCalls: &falseVal},
				},
			},
			false,
		},
		{
			"tool_calls explicitly true",
			githubCopilotModelPolicy{
				ModelPickerEnabled: true,
				Capabilities: &githubCopilotModelCapabilities{
					Supports: &githubCopilotModelSupports{ToolCalls: &trueVal},
				},
			},
			true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isSelectableCopilotModel(c.m); got != c.want {
				t.Errorf("isSelectableCopilotModel(%+v) = %v, want %v", c.m, got, c.want)
			}
		})
	}
}
