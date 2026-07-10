package providers

// Ports: packages/ai/src/providers/google.ts

import (
	"github.com/julienlegoux/kern-link/ai"
	"github.com/julienlegoux/kern-link/ai/apis/google"
	"github.com/julienlegoux/kern-link/ai/auth"
	"github.com/julienlegoux/kern-link/ai/catalog"
)

// GoogleProvider builds the Google (Gemini) provider binding, over the
// google-generative-ai wire adapter.
func GoogleProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:      "google",
		Name:    "Google",
		BaseURL: "https://generativelanguage.googleapis.com/v1beta",
		Auth:    ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth("Gemini API key", []string{"GEMINI_API_KEY"})},
		Models:  catalog.BuiltinModels("google"),
		Api:     ai.StreamFuncs{StreamFunc: google.Stream, StreamSimpleFunc: google.StreamSimple},
	})
}
