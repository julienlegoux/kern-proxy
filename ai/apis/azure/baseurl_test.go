package azure

// Ports: packages/ai/test/azure-openai-base-url.test.ts. Upstream captures the
// AzureOpenAI SDK client's constructor baseURL argument; this port has no SDK
// client (this repo talks raw net/http, per PORTING.md's "Vendor SDKs"
// deviation), so normalizeAzureBaseURL is exercised directly against the same
// input/output pairs upstream asserts.

import (
	"strings"
	"testing"

	"github.com/julienlegoux/kern-proxy/ai"
)

// TestNormalizeAzureBaseURL is a table port of every case in
// azure-openai-base-url.test.ts.
func TestNormalizeAzureBaseURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"CognitiveServicesRoot", "https://marc-quicktests-resource.cognitiveservices.azure.com", "https://marc-quicktests-resource.cognitiveservices.azure.com/openai/v1"},
		{"FoundryRoot", "https://marc-quicktests-resource.ai.azure.com", "https://marc-quicktests-resource.ai.azure.com/openai/v1"},
		{"AzureOpenAIRoot", "https://my-resource.openai.azure.com", "https://my-resource.openai.azure.com/openai/v1"},
		{"OpenaiPathToV1", "https://my-resource.cognitiveservices.azure.com/openai", "https://my-resource.cognitiveservices.azure.com/openai/v1"},
		{"PreservesOpenaiV1", "https://my-resource.cognitiveservices.azure.com/openai/v1", "https://my-resource.cognitiveservices.azure.com/openai/v1"},
		{"ResponsesPathToV1", "https://my-resource.services.ai.azure.com/openai/v1/responses", "https://my-resource.services.ai.azure.com/openai/v1"},
		{"PreservesNonAzureProxyPath", "https://my-proxy.example.com/v1", "https://my-proxy.example.com/v1"},
		{"StripsQueryParamsOnAzureHosts", "https://my-resource.openai.azure.com/openai?api-version=2024-12-01", "https://my-resource.openai.azure.com/openai/v1"},
		{"PreservesQueryParamsOnNonAzureProxy", "https://my-proxy.example.com/v1?custom=true", "https://my-proxy.example.com/v1?custom=true"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := normalizeAzureBaseURL(c.in)
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestNormalizeAzureBaseURL_InvalidURLErrors(t *testing.T) {
	_, err := normalizeAzureBaseURL("not-a-url")
	if err == nil {
		t.Fatal("err = nil, want an error")
	}
	if !strings.Contains(err.Error(), "Invalid Azure OpenAI base URL") {
		t.Errorf("err = %q, want the invalid-URL message", err.Error())
	}
}

func TestBuildDefaultAzureBaseURL(t *testing.T) {
	got := buildDefaultBaseURL("my-resource")
	want := "https://my-resource.openai.azure.com/openai/v1"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// --- resolveAzureConfig ------------------------------------------------------

func testModel(baseURL string) *ai.Model {
	return &ai.Model{
		ID:            "gpt-4o-mini",
		Name:          "GPT-4o mini",
		Api:           ai.ApiAzureOpenAIResponses,
		Provider:      "azure-openai-responses",
		BaseURL:       baseURL,
		Input:         []ai.Modality{ai.ModalityText, ai.ModalityImage},
		ContextWindow: 128000,
		MaxTokens:     16000,
	}
}

func TestResolveAzureConfig_BuildsDefaultURLFromResourceName(t *testing.T) {
	model := testModel("")
	baseURL, apiVersion, err := resolveAzureConfig(model, &ai.StreamOptions{AzureResourceName: "my-resource"})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if baseURL != "https://my-resource.openai.azure.com/openai/v1" {
		t.Errorf("baseURL = %q", baseURL)
	}
	if apiVersion != "v1" {
		t.Errorf("apiVersion = %q, want v1 default", apiVersion)
	}
}

func TestResolveAzureConfig_ExplicitBaseURLWins(t *testing.T) {
	model := testModel("")
	baseURL, _, err := resolveAzureConfig(model, &ai.StreamOptions{
		AzureBaseURL:      "https://explicit.openai.azure.com",
		AzureResourceName: "ignored",
	})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if baseURL != "https://explicit.openai.azure.com/openai/v1" {
		t.Errorf("baseURL = %q", baseURL)
	}
}

func TestResolveAzureConfig_FallsBackToModelBaseURL(t *testing.T) {
	model := testModel("https://from-model.openai.azure.com")
	baseURL, _, err := resolveAzureConfig(model, &ai.StreamOptions{})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if baseURL != "https://from-model.openai.azure.com/openai/v1" {
		t.Errorf("baseURL = %q", baseURL)
	}
}

func TestResolveAzureConfig_ErrorsWithoutAnyBaseURLSource(t *testing.T) {
	model := testModel("")
	_, _, err := resolveAzureConfig(model, &ai.StreamOptions{})
	if err == nil {
		t.Fatal("err = nil, want a required-base-URL error")
	}
	if !strings.Contains(err.Error(), "Azure OpenAI base URL is required") {
		t.Errorf("err = %q", err.Error())
	}
}

func TestResolveAzureConfig_ExplicitAPIVersionOverridesDefault(t *testing.T) {
	model := testModel("")
	_, apiVersion, err := resolveAzureConfig(model, &ai.StreamOptions{
		AzureResourceName: "my-resource",
		AzureAPIVersion:   "2024-10-21",
	})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if apiVersion != "2024-10-21" {
		t.Errorf("apiVersion = %q, want 2024-10-21", apiVersion)
	}
}

// --- deployment name resolution ---------------------------------------------

func TestParseDeploymentNameMap(t *testing.T) {
	got := parseDeploymentNameMap("gpt-4o = my-gpt4o-deployment, gpt-4o-mini=mini-dep")
	if got["gpt-4o"] != "my-gpt4o-deployment" {
		t.Errorf("gpt-4o = %q", got["gpt-4o"])
	}
	if got["gpt-4o-mini"] != "mini-dep" {
		t.Errorf("gpt-4o-mini = %q", got["gpt-4o-mini"])
	}
}

func TestParseDeploymentNameMap_SkipsMalformedEntries(t *testing.T) {
	got := parseDeploymentNameMap("no-equals-sign, =missing-key, missing-value=, valid=ok")
	if len(got) != 1 || got["valid"] != "ok" {
		t.Errorf("got %#v, want only {valid: ok}", got)
	}
}

func TestParseDeploymentNameMap_Empty(t *testing.T) {
	got := parseDeploymentNameMap("")
	if len(got) != 0 {
		t.Errorf("got %#v, want empty map", got)
	}
}

func TestResolveDeploymentName_ExplicitOptionWins(t *testing.T) {
	model := testModel("")
	got := resolveDeploymentName(model, &ai.StreamOptions{AzureDeploymentName: "explicit-dep"})
	if got != "explicit-dep" {
		t.Errorf("got %q, want explicit-dep", got)
	}
}

func TestResolveDeploymentName_FallsBackToModelID(t *testing.T) {
	model := testModel("")
	got := resolveDeploymentName(model, &ai.StreamOptions{})
	if got != "gpt-4o-mini" {
		t.Errorf("got %q, want model ID gpt-4o-mini", got)
	}
}

func TestResolveDeploymentName_UsesEnvDeploymentMap(t *testing.T) {
	model := testModel("")
	got := resolveDeploymentName(model, &ai.StreamOptions{
		Env: ai.ProviderEnv{"AZURE_OPENAI_DEPLOYMENT_NAME_MAP": "gpt-4o-mini=mapped-dep"},
	})
	if got != "mapped-dep" {
		t.Errorf("got %q, want mapped-dep", got)
	}
}
