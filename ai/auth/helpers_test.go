package auth

// Ports: packages/ai/test/providers.test.ts (the `describe("envApiKeyAuth", ...)`
// block).

import (
	"context"
	"testing"

	"github.com/julienlegoux/kern-link/ai"
)

type mapAuthContext map[string]string

func (m mapAuthContext) Env(name string) string { return m[name] }
func (m mapAuthContext) FileExists(string) bool { return false }

func TestEnvAPIKeyAuthPrefersStoredCredentialThenEnvVars(t *testing.T) {
	auth := EnvAPIKeyAuth("Test key", []string{"FIRST_KEY", "SECOND_KEY"})
	model := &ai.Model{Provider: "p1"}

	stored, err := auth.Resolve(context.Background(), ai.APIKeyResolveInput{
		Model:      model,
		Ctx:        mapAuthContext{"FIRST_KEY": "env"},
		Credential: &ai.APIKeyCredential{Key: "stored"},
	})
	if err != nil {
		t.Fatalf("Resolve (stored): %v", err)
	}
	if stored == nil || stored.Auth.APIKey != "stored" || stored.Source != "stored credential" {
		t.Fatalf("Resolve (stored) = %+v", stored)
	}

	second, err := auth.Resolve(context.Background(), ai.APIKeyResolveInput{
		Model: model,
		Ctx:   mapAuthContext{"SECOND_KEY": "second"},
	})
	if err != nil {
		t.Fatalf("Resolve (env fallback): %v", err)
	}
	if second == nil || second.Auth.APIKey != "second" || second.Source != "SECOND_KEY" {
		t.Fatalf("Resolve (env fallback) = %+v", second)
	}

	none, err := auth.Resolve(context.Background(), ai.APIKeyResolveInput{Model: model, Ctx: mapAuthContext{}})
	if err != nil {
		t.Fatalf("Resolve (unconfigured): %v", err)
	}
	if none != nil {
		t.Fatalf("Resolve (unconfigured) = %+v, want nil", none)
	}
}

func TestEnvAPIKeyAuthLoginPromptsForSecret(t *testing.T) {
	auth := EnvAPIKeyAuth("Test key", []string{"TEST_KEY"})

	var gotType ai.AuthPromptType
	credential, err := auth.Login(context.Background(), ai.AuthLoginCallbacks{
		Prompt: func(_ context.Context, prompt ai.AuthPrompt) (string, error) {
			gotType = prompt.Type
			return "entered-key", nil
		},
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if gotType != ai.AuthPromptSecret {
		t.Errorf("prompt type = %q, want %q", gotType, ai.AuthPromptSecret)
	}
	if credential == nil || credential.Key != "entered-key" {
		t.Fatalf("credential = %+v", credential)
	}
}
