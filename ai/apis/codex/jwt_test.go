package codex

// New Go tests covering extractAccountID against fixture JWTs, matching this
// issue's "accountId extraction covered against fixture JWTs" acceptance
// criterion. Mirrors the token-shape upstream test fixtures build (a 3-part
// dot-separated token whose middle segment is a base64-encoded JSON payload
// carrying the "https://api.openai.com/auth".chatgpt_account_id claim).

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func buildFixtureToken(t *testing.T, accountID string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		jwtClaimPath: map[string]any{"chatgpt_account_id": accountID},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(payload)
	return "header." + encoded + ".signature"
}

func TestExtractAccountID_ValidToken(t *testing.T) {
	token := buildFixtureToken(t, "account-123")
	accountID, err := extractAccountID(token)
	if err != nil {
		t.Fatalf("extractAccountID: %v", err)
	}
	if accountID != "account-123" {
		t.Errorf("accountID = %q, want account-123", accountID)
	}
}

func TestExtractAccountID_RawURLEncodingNoPadding(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{jwtClaimPath: map[string]any{"chatgpt_account_id": "acc_test"}})
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	token := "aaa." + encoded + ".bbb"
	accountID, err := extractAccountID(token)
	if err != nil {
		t.Fatalf("extractAccountID: %v", err)
	}
	if accountID != "acc_test" {
		t.Errorf("accountID = %q, want acc_test", accountID)
	}
}

func TestExtractAccountID_WrongPartCountFails(t *testing.T) {
	_, err := extractAccountID("not-a-jwt")
	if err == nil {
		t.Fatal("expected an error for a malformed token")
	}
}

func TestExtractAccountID_InvalidBase64PayloadFails(t *testing.T) {
	_, err := extractAccountID("header.not-base64!!!.signature")
	if err == nil {
		t.Fatal("expected an error for an invalid base64 payload")
	}
}

func TestExtractAccountID_MissingClaimFails(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{"other": "claim"})
	encoded := base64.StdEncoding.EncodeToString(payload)
	_, err := extractAccountID("header." + encoded + ".signature")
	if err == nil {
		t.Fatal("expected an error when the accountId claim is missing")
	}
}

func TestExtractAccountID_EmptyAccountIDFails(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{jwtClaimPath: map[string]any{"chatgpt_account_id": ""}})
	encoded := base64.StdEncoding.EncodeToString(payload)
	_, err := extractAccountID("header." + encoded + ".signature")
	if err == nil {
		t.Fatal("expected an error for an empty accountId claim")
	}
}
