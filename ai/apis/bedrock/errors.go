package bedrock

// Ports: packages/ai/src/api/bedrock-converse-stream.ts

// Ports formatBedrockError from bedrock-converse-stream.ts.
//
// Deviation: upstream's formatBedrockError uses normalizeProviderError to
// defensively probe an arbitrary thrown value's shape (statusCode/body
// fields on SDK-style errors) because JS has no static error typing. The AWS
// Go SDK instead returns errors implementing smithy.APIError uniformly
// (ErrorCode/ErrorMessage/ErrorFault) for both request-level failures and
// stream-level exceptions, so this port reads those directly instead of
// re-implementing upstream's shape-probing.

import (
	"errors"
	"regexp"

	"github.com/aws/smithy-go"
)

// bedrockErrorPrefixes maps a smithy error code to the human-readable prefix
// downstream retry/overflow classifiers match against (patterns like
// `server.?error`, `service.?unavailable`). Ports BEDROCK_ERROR_PREFIXES.
var bedrockErrorPrefixes = map[string]string{
	"InternalServerException":     "Internal server error",
	"ModelStreamErrorException":   "Model stream error",
	"ValidationException":         "Validation error",
	"ThrottlingException":         "Throttling error",
	"ServiceUnavailableException": "Service unavailable",
}

// bedrockDataRetentionDocsURL points users at AWS's data-retention-mode docs
// when a model rejects the account's configured retention mode.
const bedrockDataRetentionDocsURL = "https://docs.aws.amazon.com/bedrock/latest/userguide/data-retention.html"

var dataRetentionModePattern = regexp.MustCompile(`(?i)data retention mode`)

// formatBedrockError formats err with a human-readable prefix, matching
// formatBedrockError's BEDROCK_ERROR_PREFIXES lookup and data-retention hint.
func formatBedrockError(err error) string {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return err.Error()
	}

	code := apiErr.ErrorCode()
	prefix, ok := bedrockErrorPrefixes[code]
	if !ok {
		prefix = code
	}
	core := prefix + ": " + apiErr.ErrorMessage()
	if dataRetentionModePattern.MatchString(core) {
		core += " See " + bedrockDataRetentionDocsURL + " for supported data retention modes."
	}
	return core
}

// bedrockError is a plain string error, used to wrap formatBedrockError's
// output (fail() in bedrock.go calls err.Error(), so the formatted string
// must round-trip unchanged).
type bedrockError string

func (e bedrockError) Error() string { return string(e) }

// formatErr formats err via formatBedrockError and wraps it back into an
// error for run()'s fail().
func formatErr(err error) error {
	return bedrockError(formatBedrockError(err))
}

// errAborted and errUnknown match upstream's literal fallback error messages
// ("Request was aborted" / "An unknown error occurred") for the
// signal-aborted and unknown-stop-reason paths in bedrock-converse-stream.ts.
var (
	errAborted = bedrockError("Request was aborted")
	errUnknown = bedrockError("An unknown error occurred")
)
