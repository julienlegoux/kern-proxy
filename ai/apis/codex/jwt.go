package codex

// Ports: the extractAccountId helper from
// packages/ai/src/api/openai-codex-responses.ts.

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

// jwtClaimPath is the JWT claim namespace Codex's access token carries the
// ChatGPT account id under.
const jwtClaimPath = "https://api.openai.com/auth"

// errExtractAccountID is returned for any failure extracting the accountId,
// matching upstream's single collapsed error message (extractAccountId's
// catch-all `throw new Error("Failed to extract accountId from token")`).
var errExtractAccountID = errors.New("Failed to extract accountId from token")

// extractAccountID decodes the ChatGPT account id from a Codex JWT access
// token's payload segment. The token's middle segment is decoded leniently
// (standard or URL-safe base64, padded or not) since real Codex tokens use
// base64url while some test fixtures use standard base64 -- upstream's atob
// only handles the latter, but this Go port isn't bound to atob's stricter
// behavior and both encodings decode the same underlying bytes.
func extractAccountID(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", errExtractAccountID
	}

	payload, ok := decodeJWTSegment(parts[1])
	if !ok {
		return "", errExtractAccountID
	}

	var claims map[string]json.RawMessage
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", errExtractAccountID
	}

	authClaimRaw, ok := claims[jwtClaimPath]
	if !ok {
		return "", errExtractAccountID
	}

	var authClaim struct {
		ChatGPTAccountID string `json:"chatgpt_account_id"`
	}
	if err := json.Unmarshal(authClaimRaw, &authClaim); err != nil || authClaim.ChatGPTAccountID == "" {
		return "", errExtractAccountID
	}

	return authClaim.ChatGPTAccountID, nil
}

// decodeJWTSegment tries every base64 variant a JWT segment might use.
func decodeJWTSegment(segment string) ([]byte, bool) {
	decoders := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, enc := range decoders {
		if decoded, err := enc.DecodeString(segment); err == nil {
			return decoded, true
		}
	}
	return nil, false
}
