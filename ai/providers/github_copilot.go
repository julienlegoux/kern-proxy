package providers

// Ports: packages/ai/src/providers/github-copilot.ts (binding). Upstream's
// only auth strategy is `lazyOAuth({ name: "GitHub Copilot", load:
// loadGitHubCopilotOAuth })`, now bound to the real ai/auth/oauth strategy
// (oauth.CopilotOAuth: device-code login, refresh, and token derivation),
// alongside the COPILOT_GITHUB_TOKEN api-key fallback upstream also wires.
// RefreshModels is native: no upstream provider file
// wires a `refreshModels` hook, but src/utils/oauth/github-copilot.ts's
// fetchAvailableGitHubCopilotModelIds/isSelectableCopilotModel document the
// same authenticated GET {baseUrl}/models response shape and
// model_picker_enabled/policy.state/capabilities.supports.tool_calls
// selection rule ("Copilot policy" filtering) used there after an OAuth
// token refresh; this reuses that rule with the COPILOT_GITHUB_TOKEN
// env-key fallback already available without Epic 12's OAuth flow, per
// docs/epics/epic-11-catalog-all-providers/issues/03-vendor-bindings-refreshmodels.md.

import (
	"context"
	"os"

	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/apis/anthropic"
	"github.com/kern-ia/kern-link/ai/apis/openaicompletions"
	"github.com/kern-ia/kern-link/ai/apis/openairesponses"
	"github.com/kern-ia/kern-link/ai/auth"
	"github.com/kern-ia/kern-link/ai/auth/oauth"
	"github.com/kern-ia/kern-link/ai/catalog"
)

// githubCopilotModelsBaseURL is a var so tests can point RefreshModels at an
// httptest server.
var githubCopilotModelsBaseURL = "https://api.individual.githubcopilot.com"

type githubCopilotModelsResponse struct {
	Data []githubCopilotModelPolicy `json:"data"`
}

type githubCopilotModelPolicy struct {
	ID                 string                          `json:"id"`
	ModelPickerEnabled bool                            `json:"model_picker_enabled"`
	Policy             *githubCopilotModelPolicyDetail `json:"policy"`
	Capabilities       *githubCopilotModelCapabilities `json:"capabilities"`
}

type githubCopilotModelPolicyDetail struct {
	State string `json:"state"`
}

type githubCopilotModelCapabilities struct {
	Supports *githubCopilotModelSupports `json:"supports"`
}

type githubCopilotModelSupports struct {
	ToolCalls *bool `json:"tool_calls"`
}

// isSelectableCopilotModel mirrors upstream's isSelectableCopilotModel:
// selectable unless the picker has it disabled, its policy explicitly
// disables it, or it explicitly lacks tool-call support.
func isSelectableCopilotModel(m githubCopilotModelPolicy) bool {
	if !m.ModelPickerEnabled {
		return false
	}
	if m.Policy != nil && m.Policy.State == "disabled" {
		return false
	}
	if m.Capabilities != nil && m.Capabilities.Supports != nil &&
		m.Capabilities.Supports.ToolCalls != nil && !*m.Capabilities.Supports.ToolCalls {
		return false
	}
	return true
}

// refreshGitHubCopilotModels narrows the embedded catalog down to models
// selectable under the account's live Copilot policy. Without a
// COPILOT_GITHUB_TOKEN (the OAuth flow this normally rides on is deferred to
// Epic 12), it returns the static catalog unchanged rather than failing.
func refreshGitHubCopilotModels(ctx context.Context) ([]*ai.Model, error) {
	catalogModels := catalog.BuiltinModels("github-copilot")

	token := os.Getenv("COPILOT_GITHUB_TOKEN")
	if token == "" {
		out := make([]*ai.Model, len(catalogModels))
		copy(out, catalogModels)
		return out, nil
	}

	headers := map[string]string{
		"Accept":               "application/json",
		"Authorization":        "Bearer " + token,
		"X-GitHub-Api-Version": "2026-06-01",
	}
	var resp githubCopilotModelsResponse
	if err := fetchJSON(ctx, githubCopilotModelsBaseURL+"/models", headers, &resp); err != nil {
		return nil, err
	}

	selectable := make(map[string]bool, len(resp.Data))
	for _, m := range resp.Data {
		if isSelectableCopilotModel(m) {
			selectable[m.ID] = true
		}
	}

	models := make([]*ai.Model, 0, len(catalogModels))
	for _, m := range catalogModels {
		if selectable[m.ID] {
			models = append(models, m)
		}
	}
	return models, nil
}

// GitHubCopilotProvider builds the GitHub Copilot provider binding,
// dispatching on each model's api across the anthropic-messages,
// openai-completions, and openai-responses wire adapters.
func GitHubCopilotProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "github-copilot",
		Name:    "GitHub Copilot",
		BaseURL: "https://api.individual.githubcopilot.com",
		Auth: ai.ProviderAuth{
			APIKey: auth.EnvAPIKeyAuth("GitHub Copilot token", []string{"COPILOT_GITHUB_TOKEN"}),
			OAuth:  oauth.CopilotOAuth,
		},
		Models:        catalog.BuiltinModels("github-copilot"),
		RefreshModels: refreshGitHubCopilotModels,
		ApiByProtocol: map[ai.Api]ai.ProviderStreams{
			ai.ApiAnthropicMessages: ai.StreamFuncs{StreamFunc: anthropic.Stream, StreamSimpleFunc: anthropic.StreamSimple},
			ai.ApiOpenAICompletions: ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple},
			ai.ApiOpenAIResponses:   ai.StreamFuncs{StreamFunc: openairesponses.Stream, StreamSimpleFunc: openairesponses.StreamSimple},
		},
	})
}
