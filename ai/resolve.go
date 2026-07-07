package ai

// Ports: packages/ai/src/auth/resolve.ts

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ModelsErrorCode classifies ModelsError failures.
type ModelsErrorCode string

const (
	ModelsErrorModelSource     ModelsErrorCode = "model_source"
	ModelsErrorModelValidation ModelsErrorCode = "model_validation"
	ModelsErrorProvider        ModelsErrorCode = "provider"
	ModelsErrorStream          ModelsErrorCode = "stream"
	ModelsErrorAuth            ModelsErrorCode = "auth"
	ModelsErrorOAuth           ModelsErrorCode = "oauth"
)

// ModelsError is the typed error of the Models collection and auth
// resolution.
type ModelsError struct {
	ErrCode ModelsErrorCode
	Message string
	Cause   error
}

func NewModelsError(code ModelsErrorCode, message string, cause error) *ModelsError {
	return &ModelsError{ErrCode: code, Message: message, Cause: cause}
}

func (e *ModelsError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *ModelsError) Unwrap() error { return e.Cause }

// Code exposes the error code (also feeds DiagnosticErrorInfo.Code).
func (e *ModelsError) Code() string { return string(e.ErrCode) }

// AuthResolutionOverrides carries per-request overrides (CLI --api-key,
// request env) into auth resolution.
type AuthResolutionOverrides struct {
	APIKey *string
	Env    ProviderEnv
}

// authClock is stubbed in tests to control OAuth expiry checks.
var authClock = func() int64 { return time.Now().UnixMilli() }

// ResolveProviderAuth resolves request auth for a model. Precedence:
//
//  1. an explicit APIKey override (built into a synthetic api_key credential);
//  2. the stored credential — a stored credential owns the provider: oauth
//     goes through the locked refresh, api_key merges override env; a stored
//     type without a matching handler resolves to nil (no silent env
//     fallback);
//  3. ambient sources (env vars, AWS profiles, ADC files), only when nothing
//     is stored.
func ResolveProviderAuth(
	ctx context.Context,
	providerID string,
	auth ProviderAuth,
	model *Model,
	credentials CredentialStore,
	authContext AuthContext,
	overrides *AuthResolutionOverrides,
) (*AuthResult, error) {
	requestAuthContext := authContext
	if overrides != nil && overrides.Env != nil {
		requestAuthContext = overlayEnvAuthContext(authContext, overrides.Env)
	}

	if overrides != nil && overrides.APIKey != nil && auth.APIKey != nil {
		return resolveAPIKey(ctx, requestAuthContext, auth.APIKey, model, &APIKeyCredential{
			Key: *overrides.APIKey,
			Env: overrides.Env,
		})
	}

	stored, err := readCredential(ctx, credentials, providerID)
	if err != nil {
		return nil, err
	}
	if stored != nil {
		if oauthCred, ok := stored.(*OAuthCredential); ok && auth.OAuth != nil {
			return resolveStoredOAuth(ctx, credentials, providerID, auth.OAuth, oauthCred)
		}
		if apiKeyCred, ok := stored.(*APIKeyCredential); ok && auth.APIKey != nil {
			credential := apiKeyCred
			if overrides != nil && overrides.Env != nil {
				merged := make(ProviderEnv, len(apiKeyCred.Env)+len(overrides.Env))
				for k, v := range apiKeyCred.Env {
					merged[k] = v
				}
				for k, v := range overrides.Env {
					merged[k] = v
				}
				credential = &APIKeyCredential{Key: apiKeyCred.Key, Env: merged}
			}
			return resolveAPIKey(ctx, requestAuthContext, auth.APIKey, model, credential)
		}
		return nil, nil
	}

	// Ambient (env vars, AWS profiles, ADC files).
	if auth.APIKey != nil {
		return resolveAPIKey(ctx, requestAuthContext, auth.APIKey, model, nil)
	}
	return nil, nil
}

type overlayAuthContext struct {
	base AuthContext
	env  ProviderEnv
}

func (o overlayAuthContext) Env(name string) string {
	if v, ok := o.env[name]; ok && v != "" {
		return v
	}
	return o.base.Env(name)
}

func (o overlayAuthContext) FileExists(path string) bool { return o.base.FileExists(path) }

func overlayEnvAuthContext(base AuthContext, env ProviderEnv) AuthContext {
	return overlayAuthContext{base: base, env: env}
}

// resolveStoredOAuth refreshes with double-checked locking: valid tokens cost
// zero locks; expired tokens lock, re-check expiry under the lock, refresh
// once globally, and persist the rotated credential before release.
func resolveStoredOAuth(
	ctx context.Context,
	credentials CredentialStore,
	providerID string,
	oauth *OAuthAuth,
	stored *OAuthCredential,
) (*AuthResult, error) {
	credential := stored

	if authClock() >= credential.Expires {
		// Optimistic check said expired; the authoritative check runs under
		// the lock.
		post, err := credentials.Modify(ctx, providerID, func(current Credential) (Credential, error) {
			currentOAuth, ok := current.(*OAuthCredential)
			if !ok {
				return nil, nil // logged out meanwhile
			}
			if authClock() < currentOAuth.Expires {
				return nil, nil // another process/request refreshed
			}
			refreshed, refreshErr := oauth.Refresh(ctx, currentOAuth)
			if refreshErr != nil {
				return nil, NewModelsError(ModelsErrorOAuth, fmt.Sprintf("OAuth refresh failed for %s", providerID), refreshErr)
			}
			return refreshed, nil
		})
		if err != nil {
			var me *ModelsError
			if errors.As(err, &me) {
				return nil, err
			}
			return nil, NewModelsError(ModelsErrorAuth, fmt.Sprintf("Credential store modify failed for %s", providerID), err)
		}
		postOAuth, ok := post.(*OAuthCredential)
		if !ok {
			return nil, nil // logged out meanwhile
		}
		credential = postOAuth
	}

	auth, err := oauth.ToAuth(ctx, credential)
	if err != nil {
		return nil, NewModelsError(ModelsErrorOAuth, fmt.Sprintf("OAuth auth derivation failed for %s", providerID), err)
	}
	return &AuthResult{Auth: auth, Source: "OAuth"}, nil
}

func resolveAPIKey(
	ctx context.Context,
	authContext AuthContext,
	apiKey *APIKeyAuth,
	model *Model,
	credential *APIKeyCredential,
) (*AuthResult, error) {
	result, err := apiKey.Resolve(ctx, APIKeyResolveInput{Model: model, Ctx: authContext, Credential: credential})
	if err != nil {
		provider := ""
		if model != nil {
			provider = model.Provider
		}
		return nil, NewModelsError(ModelsErrorAuth, fmt.Sprintf("API key auth failed for provider %s", provider), err)
	}
	return result, nil
}

func readCredential(ctx context.Context, credentials CredentialStore, providerID string) (Credential, error) {
	credential, err := credentials.Read(ctx, providerID)
	if err != nil {
		return nil, NewModelsError(ModelsErrorAuth, fmt.Sprintf("Credential store read failed for %s", providerID), err)
	}
	return credential, nil
}
