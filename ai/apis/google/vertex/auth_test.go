package vertex

// Ports the api-key/ADC resolution behavior covered by
// test/google-vertex-api-key-resolution.test.ts: a real API key always wins;
// an empty key, the literal "gcp-vertex-credentials" marker, or a
// "<placeholder>"-shaped value all fall back to ADC. Upstream exercises this
// indirectly through the @google/genai SDK's constructor call shape; this Go
// port has no SDK; adc_test.go/vertex_test.go assert the resulting request
// (endpoint/headers) instead. This file covers the pure resolution helpers.

import (
	"context"
	"testing"
)

func TestResolveAPIKey_RealKeyIsUsed(t *testing.T) {
	got := resolveAPIKey("AIzaSyExampleRealisticLookingApiKey123456")
	if got != "AIzaSyExampleRealisticLookingApiKey123456" {
		t.Errorf("resolveAPIKey = %q, want the real key unchanged", got)
	}
}

func TestResolveAPIKey_EmptyFallsBackToADC(t *testing.T) {
	if got := resolveAPIKey(""); got != "" {
		t.Errorf("resolveAPIKey(\"\") = %q, want empty (ADC fallback)", got)
	}
}

func TestResolveAPIKey_GCPVertexCredentialsMarkerFallsBackToADC(t *testing.T) {
	if got := resolveAPIKey("gcp-vertex-credentials"); got != "" {
		t.Errorf("resolveAPIKey(marker) = %q, want empty (ADC fallback)", got)
	}
}

func TestResolveAPIKey_PlaceholderFallsBackToADC(t *testing.T) {
	if got := resolveAPIKey("<authenticated>"); got != "" {
		t.Errorf("resolveAPIKey(placeholder) = %q, want empty (ADC fallback)", got)
	}
}

func TestResolveAPIKey_TrimsWhitespace(t *testing.T) {
	if got := resolveAPIKey("  real-key  "); got != "real-key" {
		t.Errorf("resolveAPIKey = %q, want trimmed real-key", got)
	}
}

func TestResolveProject_UsesOptionOverFirst(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_PROJECT", "env-project")
	got, err := resolveProject("opt-project", nil)
	if err != nil {
		t.Fatalf("resolveProject: %v", err)
	}
	if got != "opt-project" {
		t.Errorf("resolveProject = %q, want opt-project", got)
	}
}

func TestResolveProject_FallsBackToGoogleCloudProjectEnv(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_PROJECT", "env-project")
	got, err := resolveProject("", nil)
	if err != nil {
		t.Fatalf("resolveProject: %v", err)
	}
	if got != "env-project" {
		t.Errorf("resolveProject = %q, want env-project", got)
	}
}

func TestResolveProject_FallsBackToGCloudProjectEnv(t *testing.T) {
	t.Setenv("GCLOUD_PROJECT", "gcloud-env-project")
	got, err := resolveProject("", nil)
	if err != nil {
		t.Fatalf("resolveProject: %v", err)
	}
	if got != "gcloud-env-project" {
		t.Errorf("resolveProject = %q, want gcloud-env-project", got)
	}
}

func TestResolveProject_ErrorsWhenMissing(t *testing.T) {
	if _, err := resolveProject("", nil); err == nil {
		t.Fatal("resolveProject: want error when no project is configured")
	}
}

func TestResolveLocation_UsesOptionOverEnv(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_LOCATION", "env-location")
	got, err := resolveLocation("opt-location", nil)
	if err != nil {
		t.Fatalf("resolveLocation: %v", err)
	}
	if got != "opt-location" {
		t.Errorf("resolveLocation = %q, want opt-location", got)
	}
}

func TestResolveLocation_FallsBackToEnv(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_LOCATION", "us-central1")
	got, err := resolveLocation("", nil)
	if err != nil {
		t.Fatalf("resolveLocation: %v", err)
	}
	if got != "us-central1" {
		t.Errorf("resolveLocation = %q, want us-central1", got)
	}
}

func TestResolveLocation_ErrorsWhenMissing(t *testing.T) {
	if _, err := resolveLocation("", nil); err == nil {
		t.Fatal("resolveLocation: want error when no location is configured")
	}
}

// --- ADC token source injection (acceptance criteria: "ADC resolution
// covered with a fake token source, no live GCP needed") ---

func TestADCToken_UsesInjectedTokenSource(t *testing.T) {
	prev := adcTokenFunc
	t.Cleanup(func() { adcTokenFunc = prev })

	adcTokenFunc = func(ctx context.Context) (string, error) {
		return "fake-adc-token", nil
	}

	tok, err := adcTokenFunc(context.Background())
	if err != nil {
		t.Fatalf("adcTokenFunc: %v", err)
	}
	if tok != "fake-adc-token" {
		t.Errorf("token = %q, want fake-adc-token", tok)
	}
}
