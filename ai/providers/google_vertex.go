package providers

// Ports: packages/ai/src/providers/google-vertex.ts. Vertex accepts an
// explicit API key (stored credential, or GOOGLE_CLOUD_API_KEY) or
// Application Default Credentials (`gcloud auth application-default
// login`), which additionally requires GOOGLE_CLOUD_PROJECT (or
// GCLOUD_PROJECT) and GOOGLE_CLOUD_LOCATION to be set. No provider-level
// BaseURL: the adapter resolves the project/location-scoped REST host per
// request (see ai/apis/google/vertex).

import (
	"context"

	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/apis/google/vertex"
	"github.com/kern-ia/kern-link/ai/catalog"
)

const vertexADCPath = "~/.config/gcloud/application_default_credentials.json"

func resolveVertexAuth(_ context.Context, input ai.APIKeyResolveInput) (*ai.AuthResult, error) {
	if input.Credential != nil && input.Credential.Key != "" {
		return &ai.AuthResult{Auth: ai.ModelAuth{APIKey: input.Credential.Key}, Source: "stored credential"}, nil
	}
	if key := input.Ctx.Env("GOOGLE_CLOUD_API_KEY"); key != "" {
		return &ai.AuthResult{Auth: ai.ModelAuth{APIKey: key}, Source: "GOOGLE_CLOUD_API_KEY"}, nil
	}

	adcPath := input.Ctx.Env("GOOGLE_APPLICATION_CREDENTIALS")
	if adcPath == "" {
		adcPath = vertexADCPath
	}
	hasCredentials := input.Ctx.FileExists(adcPath)
	hasProject := input.Ctx.Env("GOOGLE_CLOUD_PROJECT") != "" || input.Ctx.Env("GCLOUD_PROJECT") != ""
	hasLocation := input.Ctx.Env("GOOGLE_CLOUD_LOCATION") != ""
	if hasCredentials && hasProject && hasLocation {
		return &ai.AuthResult{Auth: ai.ModelAuth{}, Source: "gcloud application default credentials"}, nil
	}
	return nil, nil
}

// GoogleVertexProvider builds the Google Vertex AI provider binding.
func GoogleVertexProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:   "google-vertex",
		Name: "Google Vertex AI",
		Auth: ai.ProviderAuth{APIKey: &ai.APIKeyAuth{
			Name:    "Google Cloud credentials",
			Resolve: resolveVertexAuth,
		}},
		Models: catalog.BuiltinModels("google-vertex"),
		Api:    ai.StreamFuncs{StreamFunc: vertex.Stream, StreamSimpleFunc: vertex.StreamSimple},
	})
}
