package ai

// Ports: packages/ai/src/auth/types.ts, packages/ai/src/utils/oauth/types.ts
// (OAuthCredentials).
//
// These types live in the ai package (not ai/auth) because the Provider
// interface references ProviderAuth; the ai/auth package builds persistent
// stores, env-key resolution, and OAuth flows on top of them.

import (
	"context"
	"encoding/json"
	"fmt"
)

// ModelAuth is the resolved request auth for a single model request. If a
// value cannot be expressed as APIKey, Headers, or BaseURL, it is provider
// config, not auth.
type ModelAuth struct {
	APIKey  string          `json:"apiKey,omitempty"`
	Headers ProviderHeaders `json:"headers,omitempty"`
	BaseURL string          `json:"baseUrl,omitempty"`
}

// CredentialType tags stored credentials.
type CredentialType string

const (
	CredentialTypeAPIKey CredentialType = "api_key"
	CredentialTypeOAuth  CredentialType = "oauth"
)

// Credential is the closed union of stored credentials — one per provider,
// the value shape of auth.json.
type Credential interface {
	CredentialType() CredentialType
}

// APIKeyCredential is a stored api-key credential. Env holds provider-scoped
// environment/config values such as Cloudflare account/gateway ids.
type APIKeyCredential struct {
	Key string      `json:"key,omitempty"`
	Env ProviderEnv `json:"env,omitempty"`
}

func (*APIKeyCredential) CredentialType() CredentialType { return CredentialTypeAPIKey }

// OAuthCredential is a stored OAuth credential. Expires is an absolute epoch
// in milliseconds (stored with a safety margin subtracted by most flows).
// Extra carries provider-specific fields (Copilot enterpriseUrl, Codex
// accountId, ...) and is flattened into the JSON object.
type OAuthCredential struct {
	Refresh string
	Access  string
	Expires int64
	Extra   map[string]any
}

func (*OAuthCredential) CredentialType() CredentialType { return CredentialTypeOAuth }

func (c *OAuthCredential) MarshalJSON() ([]byte, error) {
	out := make(map[string]any, len(c.Extra)+4)
	for k, v := range c.Extra {
		out[k] = v
	}
	out["type"] = string(CredentialTypeOAuth)
	out["refresh"] = c.Refresh
	out["access"] = c.Access
	out["expires"] = c.Expires
	return json.Marshal(out)
}

func (c *OAuthCredential) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if s, ok := raw["refresh"].(string); ok {
		c.Refresh = s
	}
	if s, ok := raw["access"].(string); ok {
		c.Access = s
	}
	if f, ok := raw["expires"].(float64); ok {
		c.Expires = int64(f)
	}
	delete(raw, "type")
	delete(raw, "refresh")
	delete(raw, "access")
	delete(raw, "expires")
	if len(raw) > 0 {
		c.Extra = raw
	} else {
		c.Extra = nil
	}
	return nil
}

func (c *APIKeyCredential) MarshalJSON() ([]byte, error) {
	type alias APIKeyCredential
	raw, err := json.Marshal((*alias)(c))
	if err != nil {
		return nil, err
	}
	if string(raw) == "{}" {
		return []byte(`{"type":"api_key"}`), nil
	}
	out := append([]byte(`{"type":"api_key",`), raw[1:]...)
	return out, nil
}

// UnmarshalCredential decodes a stored credential by its "type" tag.
func UnmarshalCredential(data []byte) (Credential, error) {
	var probe struct {
		Type CredentialType `json:"type"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, err
	}
	switch probe.Type {
	case CredentialTypeAPIKey:
		var v APIKeyCredential
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return &v, nil
	case CredentialTypeOAuth:
		var v OAuthCredential
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return &v, nil
	default:
		return nil, fmt.Errorf("ai: unknown credential type %q", probe.Type)
	}
}

// CredentialStore is app-owned credential storage, keyed by Provider.ID, one
// credential per provider. Modify is the only write path, so every mutation
// is a serialized read-modify-write; Models.GetAuth runs OAuth refresh inside
// Modify so concurrent requests cannot double-refresh a rotated token.
//
// Error semantics: Read returns (nil, nil) for missing entries. Methods fail
// only on storage failure; Models wraps such failures in a ModelsError with
// code "auth".
type CredentialStore interface {
	// Read returns the stored credential, possibly expired (display/status
	// use; resolved request auth comes from Models.GetAuth).
	Read(ctx context.Context, providerID string) (Credential, error)

	// Modify is the serialized write path. fn sees the current credential and
	// returns the new one, or nil to leave the entry unchanged. Mutual
	// exclusion per provider id, cross-process where the backing store
	// supports it (e.g. a file lock). Returns the post-write credential.
	Modify(ctx context.Context, providerID string, fn func(current Credential) (Credential, error)) (Credential, error)

	// Delete removes a credential (logout), serialized against Modify.
	Delete(ctx context.Context, providerID string) error
}

// AuthContext is environment access for auth resolution, injectable for
// tests. Env returns "" for unset variables (values are trimmed; empty
// means unset). FileExists supports a leading "~".
type AuthContext interface {
	Env(name string) string
	FileExists(path string) bool
}

// AuthResult is the result of resolving auth for a model.
type AuthResult struct {
	Auth ModelAuth
	// Env carries provider-scoped environment/config values resolved from
	// credentials and ambient context.
	Env ProviderEnv
	// Source is a human-readable label for status UI:
	// "ANTHROPIC_API_KEY", "OAuth", "~/.aws/credentials".
	Source string
}

// AuthPromptType discriminates login prompts.
type AuthPromptType string

const (
	AuthPromptText       AuthPromptType = "text"
	AuthPromptSecret     AuthPromptType = "secret"
	AuthPromptSelect     AuthPromptType = "select"
	AuthPromptManualCode AuthPromptType = "manual_code"
)

// AuthPromptOption is one choice of a select prompt.
type AuthPromptOption struct {
	ID          string
	Label       string
	Description string
}

// AuthPrompt is a prompt shown to the user during login. Ctx lets the flow
// cancel a pending prompt when an out-of-band event resolves the step (e.g. a
// manual_code prompt raced against a callback server).
type AuthPrompt struct {
	Type        AuthPromptType
	Message     string
	Placeholder string
	Options     []AuthPromptOption
	Ctx         context.Context
}

// AuthEventType discriminates login progress events.
type AuthEventType string

const (
	AuthEventAuthURL    AuthEventType = "auth_url"
	AuthEventDeviceCode AuthEventType = "device_code"
	AuthEventProgress   AuthEventType = "progress"
)

// AuthEvent is a login progress notification.
type AuthEvent struct {
	Type AuthEventType
	// auth_url fields
	URL          string
	Instructions string
	// device_code fields
	UserCode         string
	VerificationURI  string
	IntervalSeconds  int
	ExpiresInSeconds int
	// progress fields
	Message string
}

// AuthLoginCallbacks serves interactive login for both api-key and OAuth
// flows. Prompt returns the entered/selected string (select returns the
// option id) and fails on cancel/abort.
type AuthLoginCallbacks struct {
	Prompt func(ctx context.Context, prompt AuthPrompt) (string, error)
	Notify func(event AuthEvent)
}

// APIKeyResolveInput is the input to APIKeyAuth.Resolve.
type APIKeyResolveInput struct {
	Model      *Model
	Ctx        AuthContext
	Credential *APIKeyCredential
}

// APIKeyAuth resolves api-key auth from a stored key/provider env plus
// ambient sources (env vars, AWS profiles, ADC files). Ambient-only providers
// leave Login nil.
type APIKeyAuth struct {
	// Name is a display name, e.g. "Anthropic API key".
	Name string
	// Login runs interactive setup. nil = ambient-only.
	Login func(ctx context.Context, callbacks AuthLoginCallbacks) (*APIKeyCredential, error)
	// Resolve merges per field (credential.Key over env vars, credential.Env
	// values over ambient env). A nil result means not configured.
	Resolve func(ctx context.Context, input APIKeyResolveInput) (*AuthResult, error)
}

// OAuthAuth splits refresh from auth derivation so Models owns the locked
// refresh pattern: Refresh produces a credential, ToAuth derives request auth
// from whatever credential ends up stored (covers per-credential baseUrl,
// e.g. GitHub Copilot).
type OAuthAuth struct {
	// Name is a display name, e.g. "Anthropic (Claude Pro/Max)".
	Name    string
	Login   func(ctx context.Context, callbacks AuthLoginCallbacks) (*OAuthCredential, error)
	Refresh func(ctx context.Context, credential *OAuthCredential) (*OAuthCredential, error)
	ToAuth  func(ctx context.Context, credential *OAuthCredential) (ModelAuth, error)
}

// ProviderAuth is a provider's auth strategy. At least one of APIKey/OAuth
// must be present: even ambient-credential providers and keyless local
// servers provide APIKey auth whose Resolve reports whether the provider is
// configured.
type ProviderAuth struct {
	APIKey *APIKeyAuth
	OAuth  *OAuthAuth
}
