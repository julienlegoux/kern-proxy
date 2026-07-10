// Package oauth ports the shared scaffolding behind kern-link's interactive
// OAuth login flows (PKCE, a local callback server, and JSON token-exchange
// plumbing), plus the Anthropic PKCE flow that is the first consumer.
//
// Ports: packages/ai/src/utils/oauth/*. The Copilot device-code flow and the
// Codex dual flow reuse this scaffolding but are ported in later issues
// (epic 12, issues 02-03); device-code.ts itself is not ported here.
package oauth

// Ports: packages/ai/src/utils/oauth/pkce.ts

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// GeneratePKCE generates a PKCE code verifier and its S256 challenge,
// matching upstream's Web Crypto based generatePKCE: 32 random bytes,
// base64url-encoded (no padding) for the verifier; the challenge is the
// base64url SHA-256 digest of the verifier.
func GeneratePKCE() (verifier string, challenge string, err error) {
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return "", "", fmt.Errorf("oauth: generate pkce verifier: %w", err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(verifierBytes)

	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])

	return verifier, challenge, nil
}
