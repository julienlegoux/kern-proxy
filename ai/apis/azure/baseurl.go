package azure

// Ports: packages/ai/src/api/azure-openai-responses.ts (the base-URL/
// deployment-name resolution half: normalizeAzureBaseUrl, buildDefaultBaseUrl,
// resolveAzureConfig, resolveDeploymentName, parseDeploymentNameMap).

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/kern-ia/kern-link/ai"
)

// defaultAzureAPIVersion is DEFAULT_AZURE_API_VERSION: Azure's unversioned
// "v1" preview surface, which needs no api-version query parameter.
const defaultAzureAPIVersion = "v1"

// azureHostSuffixes are the Azure-managed hostnames normalizeAzureBaseURL
// recognizes as needing the /openai/v1 base path.
var azureHostSuffixes = []string{".openai.azure.com", ".cognitiveservices.azure.com", ".ai.azure.com"}

// normalizeAzureBaseURL ports normalizeAzureBaseUrl: recognized Azure hosts
// at their bare root (or /openai, or /openai/v1/responses) get rewritten to
// /openai/v1 with any query stripped; anything else (including non-Azure
// proxy URLs) passes through unchanged apart from trailing-slash trimming.
func normalizeAzureBaseURL(raw string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("Invalid Azure OpenAI base URL: %s", raw)
	}

	isAzureHost := false
	host := u.Hostname()
	for _, suffix := range azureHostSuffixes {
		if strings.HasSuffix(host, suffix) {
			isAzureHost = true
			break
		}
	}

	normalizedPath := strings.TrimRight(u.Path, "/")
	if isAzureHost && (normalizedPath == "" || normalizedPath == "/openai" || normalizedPath == "/openai/v1/responses") {
		u.Path = "/openai/v1"
		u.RawQuery = ""
	}

	return strings.TrimRight(u.String(), "/"), nil
}

// buildDefaultBaseURL ports buildDefaultBaseUrl.
func buildDefaultBaseURL(resourceName string) string {
	return fmt.Sprintf("https://%s.openai.azure.com/openai/v1", resourceName)
}

// resolveAzureConfig ports resolveAzureConfig: resolves the request's base
// URL (AzureBaseURL option -> AZURE_OPENAI_BASE_URL env -> a URL built from
// AzureResourceName/AZURE_OPENAI_RESOURCE_NAME -> Model.BaseURL, in that
// order) and API version (AzureAPIVersion option -> AZURE_OPENAI_API_VERSION
// env -> "v1" default), then normalizes the resolved base URL.
func resolveAzureConfig(model *ai.Model, opts *ai.StreamOptions) (baseURL, apiVersion string, err error) {
	var optAPIVersion, optBaseURL, optResourceName string
	var env ai.ProviderEnv
	if opts != nil {
		optAPIVersion = opts.AzureAPIVersion
		optBaseURL = opts.AzureBaseURL
		optResourceName = opts.AzureResourceName
		env = opts.Env
	}

	apiVersion = optAPIVersion
	if apiVersion == "" {
		apiVersion = providerEnvValue("AZURE_OPENAI_API_VERSION", env)
	}
	if apiVersion == "" {
		apiVersion = defaultAzureAPIVersion
	}

	resolved := strings.TrimSpace(optBaseURL)
	if resolved == "" {
		resolved = strings.TrimSpace(providerEnvValue("AZURE_OPENAI_BASE_URL", env))
	}

	resourceName := optResourceName
	if resourceName == "" {
		resourceName = providerEnvValue("AZURE_OPENAI_RESOURCE_NAME", env)
	}
	if resolved == "" && resourceName != "" {
		resolved = buildDefaultBaseURL(resourceName)
	}

	if resolved == "" && model.BaseURL != "" {
		resolved = model.BaseURL
	}

	if resolved == "" {
		return "", "", errors.New(
			"Azure OpenAI base URL is required. Set AZURE_OPENAI_BASE_URL or AZURE_OPENAI_RESOURCE_NAME, " +
				"or pass AzureBaseURL, AzureResourceName, or model.BaseURL",
		)
	}

	normalized, err := normalizeAzureBaseURL(resolved)
	if err != nil {
		return "", "", err
	}
	return normalized, apiVersion, nil
}

// parseDeploymentNameMap ports parseDeploymentNameMap: a
// "modelId=deploymentName,..." list into a lookup map, skipping malformed
// entries.
func parseDeploymentNameMap(value string) map[string]string {
	m := map[string]string{}
	if value == "" {
		return m
	}
	for _, entry := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		modelID := strings.TrimSpace(parts[0])
		deploymentName := strings.TrimSpace(parts[1])
		if modelID == "" || deploymentName == "" {
			continue
		}
		m[modelID] = deploymentName
	}
	return m
}

// resolveDeploymentName ports resolveDeploymentName: an explicit
// AzureDeploymentName option wins, then a AZURE_OPENAI_DEPLOYMENT_NAME_MAP
// entry keyed by the model ID, then the model ID itself.
func resolveDeploymentName(model *ai.Model, opts *ai.StreamOptions) string {
	var deploymentName string
	var env ai.ProviderEnv
	if opts != nil {
		deploymentName = opts.AzureDeploymentName
		env = opts.Env
	}
	if deploymentName != "" {
		return deploymentName
	}
	mapped := parseDeploymentNameMap(providerEnvValue("AZURE_OPENAI_DEPLOYMENT_NAME_MAP", env))
	if v, ok := mapped[model.ID]; ok && v != "" {
		return v
	}
	return model.ID
}

// providerEnvValue reads name from the request-scoped env override first,
// falling back to the process environment. Duplicated from the sibling
// anthropic/openaicompletions/openairesponses packages: Go package
// boundaries don't share unexported helpers the way upstream's single
// provider-env.ts module does.
func providerEnvValue(name string, env ai.ProviderEnv) string {
	if v, ok := env[name]; ok && v != "" {
		return v
	}
	return os.Getenv(name)
}
