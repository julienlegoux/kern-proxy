package oauth

// Ports: packages/ai/src/utils/oauth/pkce.ts (no direct upstream test file;
// pkce.ts is exercised indirectly through anthropic-oauth.test.ts. This test
// covers the PKCE contract directly since it's now a standalone Go helper.)

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

func TestGeneratePKCE_ProducesURLSafeVerifierAndChallenge(t *testing.T) {
	verifier, challenge, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE() error = %v", err)
	}

	if verifier == "" {
		t.Fatal("verifier is empty")
	}
	if challenge == "" {
		t.Fatal("challenge is empty")
	}

	for _, forbidden := range []string{"+", "/", "="} {
		if strings.Contains(verifier, forbidden) {
			t.Errorf("verifier %q contains non-base64url character %q", verifier, forbidden)
		}
		if strings.Contains(challenge, forbidden) {
			t.Errorf("challenge %q contains non-base64url character %q", challenge, forbidden)
		}
	}
}

func TestGeneratePKCE_ChallengeIsSHA256OfVerifier(t *testing.T) {
	verifier, challenge, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE() error = %v", err)
	}

	sum := sha256.Sum256([]byte(verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])

	if challenge != want {
		t.Errorf("challenge = %q, want %q (sha256(verifier) base64url)", challenge, want)
	}
}

func TestGeneratePKCE_ProducesDistinctVerifiers(t *testing.T) {
	verifier1, _, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE() error = %v", err)
	}
	verifier2, _, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE() error = %v", err)
	}

	if verifier1 == verifier2 {
		t.Error("two calls to GeneratePKCE produced the same verifier")
	}
}
