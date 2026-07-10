package bedrock

// Ports: packages/ai/src/api/bedrock-converse-stream.ts

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"regexp"
	"strings"

	"github.com/julienlegoux/kern-link/ai"
)

// providerEnvValue reads name from the request-scoped env override first,
// falling back to the process environment. Duplicated from the sibling
// anthropic/azure/openaicompletions/openairesponses packages: Go package
// boundaries don't share unexported helpers the way upstream's single
// provider-env.ts module does.
func providerEnvValue(name string, env ai.ProviderEnv) string {
	if v, ok := env[name]; ok && v != "" {
		return v
	}
	return os.Getenv(name)
}

// nonAlnumSeparator matches the run of separator characters
// getModelMatchCandidates normalizes to a single "-".
var nonAlnumSeparator = regexp.MustCompile(`[\s_.:]+`)

// modelMatchCandidates returns lower-cased matching candidates for a model's
// id and (if present) name, each alongside a punctuation-normalized variant
// (separators collapsed to "-"), matching getModelMatchCandidates from
// bedrock-converse-stream.ts. This supports both raw model ids/ARNs and
// human-readable names using different separator conventions.
func modelMatchCandidates(modelID, modelName string) []string {
	values := []string{modelID}
	if modelName != "" {
		values = append(values, modelName)
	}
	out := make([]string, 0, len(values)*2)
	for _, v := range values {
		lower := strings.ToLower(v)
		out = append(out, lower, nonAlnumSeparator.ReplaceAllString(lower, "-"))
	}
	return out
}

// imageFormat maps a MIME type to Bedrock's image format enum value.
// Deviation: upstream's createImageBlock throws synchronously on an
// unrecognized MIME type (a Go-unreachable "unknown content type" style
// case, since ai.ImageContent.MimeType is caller-supplied free text in both
// ports); this port instead passes an unrecognized value through unchanged,
// letting Bedrock's own request validation reject it -- surfacing as a
// regular error event via formatBedrockError, rather than adding an error
// return to every message-conversion function for an edge case no upstream
// or ported test exercises.
func imageFormat(mimeType string) string {
	switch mimeType {
	case "image/jpeg", "image/jpg":
		return "jpeg"
	case "image/png":
		return "png"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	default:
		return mimeType
	}
}

// decodeBase64Image decodes a base64-encoded image payload; an undecodable
// payload yields an empty byte slice (see imageFormat's deviation note --
// Bedrock's own validation rejects the resulting empty image).
func decodeBase64Image(data string) []byte {
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil
	}
	return decoded
}

// decodeJSONSchema decodes a tool's JSON Schema document into a plain Go
// value suitable for document.NewLazyDocument.
func decodeJSONSchema(schema ai.JSONSchema) map[string]any {
	if len(schema) == 0 {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(schema, &m); err != nil {
		return map[string]any{}
	}
	return m
}
