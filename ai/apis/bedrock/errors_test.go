package bedrock

// Ports formatBedrockError's prefix-mapping behavior from
// bedrock-converse-stream.ts (BEDROCK_ERROR_PREFIXES / formatBedrockError).
// Upstream probes SDK-error-shape fields via normalizeProviderError; this Go
// port uses smithy.APIError's ErrorCode()/ErrorMessage() directly, since the
// AWS SDK already exposes a structured error instead of an ad hoc thrown
// object (see errors.go's package doc for the full deviation note).

import (
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

func TestFormatBedrockError_KnownExceptionPrefixes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"internal server", &types.InternalServerException{Message: strPtr("boom")}, "Internal server error: boom"},
		{"model stream error", &types.ModelStreamErrorException{Message: strPtr("stream broke")}, "Model stream error: stream broke"},
		{"validation", &types.ValidationException{Message: strPtr("bad input")}, "Validation error: bad input"},
		{"throttling", &types.ThrottlingException{Message: strPtr("slow down")}, "Throttling error: slow down"},
		{"service unavailable", &types.ServiceUnavailableException{Message: strPtr("down")}, "Service unavailable: down"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatBedrockError(tc.err)
			if got != tc.want {
				t.Errorf("formatBedrockError(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

func TestFormatBedrockError_AddsDataRetentionHint(t *testing.T) {
	err := &types.ValidationException{Message: strPtr("data retention mode 'default' is not available for this model")}
	got := formatBedrockError(err)
	if got == "" || !strings.Contains(got, bedrockDataRetentionDocsURL) {
		t.Errorf("formatBedrockError = %q, want it to contain the docs URL", got)
	}
}

func TestFormatBedrockError_FallsBackToPlainErrorForNonAPIErrors(t *testing.T) {
	err := errors.New("connection refused")
	if got := formatBedrockError(err); got != "connection refused" {
		t.Errorf("formatBedrockError = %q, want %q", got, "connection refused")
	}
}
