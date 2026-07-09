// Package vertex implements the Vertex AI variant of the Google adapter:
// Vertex endpoint shaping over the shared google-shared converters
// (ai/apis/google), plus Application Default Credentials (ADC) auth via
// golang.org/x/oauth2/google. Upstream delegates both transport and ADC
// discovery to the @google/genai SDK; this Go port has no such SDK, so it
// speaks the Vertex AI REST API directly over net/http + ai/internal/sse
// (reusing ai/apis/google's ConvertMessages/ConvertTools/DecodeStream), and
// sources ADC tokens itself via golang.org/x/oauth2/google (see
// PORTING.md's "Vendor SDKs" deviation).
//
// Ports: packages/ai/src/api/google-vertex.ts
package vertex

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"golang.org/x/oauth2/google"
)

// vertexAIScope is the OAuth scope ADC tokens are minted for; it covers all
// Cloud Platform APIs including Vertex AI generateContent calls.
const vertexAIScope = "https://www.googleapis.com/auth/cloud-platform"

// gcpVertexCredentialsMarker is a sentinel apiKey value meaning "no explicit
// key was configured -- use ADC". Ports GCP_VERTEX_CREDENTIALS_MARKER from
// google-vertex.ts (there it also gates the @google/genai SDK's constructor
// branch; here it gates the REST endpoint/auth-header choice in vertex.go).
const gcpVertexCredentialsMarker = "gcp-vertex-credentials"

// placeholderAPIKeyPattern matches an unresolved "<...>" template value, the
// shape a caller's config placeholder takes before real credentials are
// filled in. Ports isPlaceholderApiKey from google-vertex.ts.
var placeholderAPIKeyPattern = regexp.MustCompile(`^<[^>]+>$`)

// resolveAPIKey returns the trimmed API key when it's a real credential, or
// "" when it's empty, the gcp-vertex-credentials marker, or a placeholder --
// all of which mean "fall back to ADC". Ports resolveApiKey from
// google-vertex.ts (the options.apiKey half; GOOGLE_CLOUD_API_KEY env
// resolution belongs to the google-vertex provider binding, epic 11, not this
// adapter -- see google-vertex.ts's own resolveApiKey, which likewise never
// reads process.env itself).
func resolveAPIKey(apiKey string) string {
	trimmed := strings.TrimSpace(apiKey)
	if trimmed == "" || trimmed == gcpVertexCredentialsMarker || placeholderAPIKeyPattern.MatchString(trimmed) {
		return ""
	}
	return trimmed
}

// resolveProject resolves the Vertex AI project id: the option value, else
// GOOGLE_CLOUD_PROJECT, else GCLOUD_PROJECT. Ports resolveProject from
// google-vertex.ts; only reached on the ADC path (the explicit-API-key path
// never calls this, matching createClientWithApiKey not taking a project
// argument).
func resolveProject(option string, env map[string]string) (string, error) {
	if option != "" {
		return option, nil
	}
	if v := providerEnvValue("GOOGLE_CLOUD_PROJECT", env); v != "" {
		return v, nil
	}
	if v := providerEnvValue("GCLOUD_PROJECT", env); v != "" {
		return v, nil
	}
	return "", errors.New("Vertex AI requires a project ID. Set GOOGLE_CLOUD_PROJECT/GCLOUD_PROJECT or pass project in options")
}

// resolveLocation resolves the Vertex AI region: the option value, else
// GOOGLE_CLOUD_LOCATION. Ports resolveLocation from google-vertex.ts; only
// reached on the ADC path, same as resolveProject.
func resolveLocation(option string, env map[string]string) (string, error) {
	if option != "" {
		return option, nil
	}
	if v := providerEnvValue("GOOGLE_CLOUD_LOCATION", env); v != "" {
		return v, nil
	}
	return "", errors.New("Vertex AI requires a location. Set GOOGLE_CLOUD_LOCATION or pass location in options")
}

// providerEnvValue reads name from the per-request env override first, then
// the process environment. Mirrors the same-named helper duplicated across
// the sibling azure/openaicompletions/openairesponses packages.
func providerEnvValue(name string, env map[string]string) string {
	if v, ok := env[name]; ok && v != "" {
		return v
	}
	return os.Getenv(name)
}

// adcTokenFunc sources an ADC access token, scoped for Cloud Platform APIs.
// It is a package variable (mirroring ai/resolve.go's authClock stubbing
// pattern) so tests can inject a fake token source and cover ADC resolution
// without live GCP credentials or network access.
var adcTokenFunc = func(ctx context.Context) (string, error) {
	ts, err := google.DefaultTokenSource(ctx, vertexAIScope)
	if err != nil {
		return "", fmt.Errorf("resolve Application Default Credentials: %w", err)
	}
	tok, err := ts.Token()
	if err != nil {
		return "", fmt.Errorf("fetch ADC token: %w", err)
	}
	return tok.AccessToken, nil
}
