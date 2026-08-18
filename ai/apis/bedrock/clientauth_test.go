package bedrock

// Ports test/bedrock-endpoint-resolution.test.ts's assertions (region/
// profile/endpoint resolution) onto resolveClientConfig directly -- a pure
// function over (model, opts) needs no BedrockRuntimeClient-constructor
// mock the way upstream's vi.mock("@aws-sdk/client-bedrock-runtime") does.
// Also covers the explicit-credentials/bearer-token/skip-auth/custom-headers
// cells of the auth matrix, which upstream leaves to this port's own
// coverage (see clientauth.go's package doc for why no upstream test exists
// for them).

import (
	"testing"

	"github.com/kern-ia/kern-link/ai"
)

// withAmbientEnv sets real process-environment AWS_* variables for the
// duration of the test (restoring the previous values on cleanup), mirroring
// bedrock-endpoint-resolution.test.ts's beforeEach/afterEach save-restore of
// process.env.AWS_REGION/AWS_DEFAULT_REGION/AWS_PROFILE. hasAmbientConfiguredProfile
// (and, by extension, AWS_REGION/AWS_DEFAULT_REGION availability without a
// request-scoped env override) can only be exercised through the real
// process environment, since resolveClientConfig intentionally checks it
// without threading the request's env override -- see clientauth.go.
func withAmbientEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func euModel() *ai.Model {
	return &ai.Model{
		ID:      "eu.anthropic.claude-sonnet-4-5-20250929-v1:0",
		Name:    "Claude Sonnet 4.5",
		BaseURL: "https://bedrock-runtime.eu-central-1.amazonaws.com",
	}
}

func TestResolveClientConfig_DoesNotPinStandardEndpointWhenRegionConfigured(t *testing.T) {
	withAmbientEnv(t, map[string]string{"AWS_REGION": "us-east-2"})

	cfg := resolveClientConfig(testModel(), &ai.StreamOptions{})

	if cfg.Region != "us-east-2" {
		t.Errorf("Region = %q, want us-east-2", cfg.Region)
	}
	if cfg.Endpoint != "" {
		t.Errorf("Endpoint = %q, want empty (should not pin standard endpoint)", cfg.Endpoint)
	}
}

func TestResolveClientConfig_DerivesRegionFromBuiltInEUEndpoint(t *testing.T) {
	cfg := resolveClientConfig(euModel(), &ai.StreamOptions{})

	if cfg.Endpoint != "https://bedrock-runtime.eu-central-1.amazonaws.com" {
		t.Errorf("Endpoint = %q", cfg.Endpoint)
	}
	if cfg.Region != "eu-central-1" {
		t.Errorf("Region = %q, want eu-central-1", cfg.Region)
	}
}

func TestResolveClientConfig_HandlesMissingRegionsForExplicitScopedAndAmbientProfiles(t *testing.T) {
	t.Run("explicit option profile", func(t *testing.T) {
		cfg := resolveClientConfig(euModel(), &ai.StreamOptions{BedrockProfile: "bedrock-profile"})
		if cfg.Profile != "bedrock-profile" {
			t.Errorf("Profile = %q", cfg.Profile)
		}
		if cfg.Endpoint != "https://bedrock-runtime.eu-central-1.amazonaws.com" {
			t.Errorf("Endpoint = %q", cfg.Endpoint)
		}
		if cfg.Region != "eu-central-1" {
			t.Errorf("Region = %q, want eu-central-1", cfg.Region)
		}
	})

	t.Run("request-scoped env override profile", func(t *testing.T) {
		cfg := resolveClientConfig(euModel(), &ai.StreamOptions{Env: ai.ProviderEnv{"AWS_PROFILE": "scoped-bedrock-profile"}})
		if cfg.Profile != "scoped-bedrock-profile" {
			t.Errorf("Profile = %q", cfg.Profile)
		}
		// hasAmbientConfiguredProfile only reflects the real process
		// environment, not a request-scoped override, so endpoint pinning
		// still behaves as if no profile were configured.
		if cfg.Endpoint != "https://bedrock-runtime.eu-central-1.amazonaws.com" {
			t.Errorf("Endpoint = %q", cfg.Endpoint)
		}
		if cfg.Region != "eu-central-1" {
			t.Errorf("Region = %q, want eu-central-1", cfg.Region)
		}
	})

	t.Run("ambient process-env profile", func(t *testing.T) {
		withAmbientEnv(t, map[string]string{"AWS_PROFILE": "ambient-bedrock-profile"})
		cfg := resolveClientConfig(euModel(), &ai.StreamOptions{})
		if cfg.Profile != "ambient-bedrock-profile" {
			t.Errorf("Profile = %q", cfg.Profile)
		}
		if cfg.Endpoint != "" {
			t.Errorf("Endpoint = %q, want empty (ambient profile suppresses endpoint pinning)", cfg.Endpoint)
		}
		if cfg.Region != "" {
			t.Errorf("Region = %q, want empty", cfg.Region)
		}
	})
}

func TestResolveClientConfig_PassesCustomBedrockEndpointsThrough(t *testing.T) {
	withAmbientEnv(t, map[string]string{"AWS_REGION": "us-west-2"})
	model := testModel()
	model.BaseURL = "https://bedrock-vpc.example.com"

	cfg := resolveClientConfig(model, &ai.StreamOptions{})

	if cfg.Endpoint != "https://bedrock-vpc.example.com" {
		t.Errorf("Endpoint = %q", cfg.Endpoint)
	}
	if cfg.Region != "us-west-2" {
		t.Errorf("Region = %q, want us-west-2", cfg.Region)
	}
}

func TestResolveClientConfig_ExtractsRegionFromInferenceProfileARN(t *testing.T) {
	withAmbientEnv(t, map[string]string{"AWS_REGION": "us-east-1"})
	model := testModel()
	model.ID = "arn:aws:bedrock:us-west-2:123456789012:application-inference-profile/abc123"

	cfg := resolveClientConfig(model, &ai.StreamOptions{})

	if cfg.Region != "us-west-2" {
		t.Errorf("Region = %q, want us-west-2 (from ARN, overriding AWS_REGION)", cfg.Region)
	}
}

func TestResolveClientConfig_ExtractsRegionFromGovCloudInferenceProfileARN(t *testing.T) {
	withAmbientEnv(t, map[string]string{"AWS_REGION": "us-east-1"})
	model := testModel()
	model.ID = "arn:aws-us-gov:bedrock:us-gov-west-1:123456789012:application-inference-profile/abc123"

	cfg := resolveClientConfig(model, &ai.StreamOptions{})

	if cfg.Region != "us-gov-west-1" {
		t.Errorf("Region = %q, want us-gov-west-1", cfg.Region)
	}
}

// --- Cells with no upstream test (see clientauth.go's package doc) ---

func TestResolveClientConfig_BearerTokenOptionTakesPrecedenceOverEnv(t *testing.T) {
	cfg := resolveClientConfig(testModel(), &ai.StreamOptions{
		BedrockBearerToken: "tok-explicit",
		Env:                ai.ProviderEnv{"AWS_BEARER_TOKEN_BEDROCK": "tok-env"},
	})
	if cfg.BearerToken != "tok-explicit" {
		t.Errorf("BearerToken = %q, want tok-explicit", cfg.BearerToken)
	}
}

func TestResolveClientConfig_BearerTokenFallsBackToEnvVar(t *testing.T) {
	cfg := resolveClientConfig(testModel(), &ai.StreamOptions{
		Env: ai.ProviderEnv{"AWS_BEARER_TOKEN_BEDROCK": "tok-env"},
	})
	if cfg.BearerToken != "tok-env" {
		t.Errorf("BearerToken = %q, want tok-env", cfg.BearerToken)
	}
}

func TestResolveClientConfig_ExplicitStaticCredentialsFromEnv(t *testing.T) {
	cfg := resolveClientConfig(testModel(), &ai.StreamOptions{
		Env: ai.ProviderEnv{
			"AWS_ACCESS_KEY_ID":     "AKIAEXAMPLE",
			"AWS_SECRET_ACCESS_KEY": "secretexample",
			"AWS_SESSION_TOKEN":     "sessiontoken",
		},
	})
	if cfg.AccessKeyID != "AKIAEXAMPLE" || cfg.SecretAccessKey != "secretexample" || cfg.SessionToken != "sessiontoken" {
		t.Errorf("credentials = %+v", cfg)
	}
}

func TestResolveClientConfig_CredentialsRequireBothAccessKeyAndSecret(t *testing.T) {
	cfg := resolveClientConfig(testModel(), &ai.StreamOptions{
		Env: ai.ProviderEnv{"AWS_ACCESS_KEY_ID": "AKIAEXAMPLE"},
	})
	if cfg.AccessKeyID != "" || cfg.SecretAccessKey != "" {
		t.Errorf("credentials = %+v, want empty (secret missing)", cfg)
	}
}

func TestResolveClientConfig_SkipAuthForcesDummyCredentialsAndSuppressesBearerToken(t *testing.T) {
	cfg := resolveClientConfig(testModel(), &ai.StreamOptions{
		Env: ai.ProviderEnv{
			"AWS_BEDROCK_SKIP_AUTH":    "1",
			"AWS_BEARER_TOKEN_BEDROCK": "tok-env",
		},
	})
	if cfg.AccessKeyID != "dummy-access-key" || cfg.SecretAccessKey != "dummy-secret-key" {
		t.Errorf("credentials = %+v, want dummy", cfg)
	}
	if cfg.BearerToken != "" {
		t.Errorf("BearerToken = %q, want empty (skip-auth suppresses bearer token)", cfg.BearerToken)
	}
}

func TestResolveClientConfig_HeadersPassThroughDroppingSuppressed(t *testing.T) {
	cfg := resolveClientConfig(testModel(), &ai.StreamOptions{
		Headers: ai.ProviderHeaders{
			"x-custom":     ai.HeaderValue("v"),
			"x-suppressed": nil,
		},
	})
	if len(cfg.Headers) != 1 || cfg.Headers["x-custom"] != "v" {
		t.Errorf("Headers = %+v", cfg.Headers)
	}
}

func TestResolveClientConfig_NoHeadersMeansNilMap(t *testing.T) {
	cfg := resolveClientConfig(testModel(), &ai.StreamOptions{})
	if len(cfg.Headers) != 0 {
		t.Errorf("Headers = %+v, want empty", cfg.Headers)
	}
}
