package oauth

// Ports: packages/ai/src/utils/oauth/anthropic.ts (postJson — shared JSON
// POST helper reused by every flow's token exchange/refresh). Upstream has
// no standalone test for this helper; it's covered directly here since it's
// now a reusable Go function, and indirectly through anthropic_test.go.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostJSON_ReturnsBodyOnSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["grant_type"] != "authorization_code" {
			t.Errorf("grant_type = %v, want authorization_code", body["grant_type"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"tok"}`))
	}))
	defer server.Close()

	respBody, err := PostJSON(context.Background(), server.URL, map[string]any{
		"grant_type": "authorization_code",
	})
	if err != nil {
		t.Fatalf("PostJSON() error = %v", err)
	}

	var decoded map[string]string
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if decoded["access_token"] != "tok" {
		t.Errorf("access_token = %q, want tok", decoded["access_token"])
	}
}

func TestPostJSON_ErrorsOnNon2xxStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer server.Close()

	_, err := PostJSON(context.Background(), server.URL, map[string]any{"grant_type": "x"})
	if err == nil {
		t.Fatal("PostJSON() error = nil, want an error for a 400 response")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error = %q, want it to mention the 400 status", err.Error())
	}
	if !strings.Contains(err.Error(), "invalid_grant") {
		t.Errorf("error = %q, want it to include the response body", err.Error())
	}
}

func TestPostJSON_RespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := PostJSON(ctx, "http://127.0.0.1:1/unused", map[string]any{"x": "y"})
	if err == nil {
		t.Fatal("PostJSON() error = nil, want an error for a cancelled context")
	}
}
