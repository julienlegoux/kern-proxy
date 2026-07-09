package ai

// Ports the resolution-precedence semantics of packages/ai/src/auth/resolve.ts
// (exercised upstream by test/oauth-auth.test.ts).

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

type mapAuthContext map[string]string

func (m mapAuthContext) Env(name string) string { return m[name] }
func (m mapAuthContext) FileExists(string) bool { return false }

func envKeyAuth(envVar string) *APIKeyAuth {
	return &APIKeyAuth{
		Name: "test key",
		Resolve: func(_ context.Context, input APIKeyResolveInput) (*AuthResult, error) {
			if input.Credential != nil && input.Credential.Key != "" {
				return &AuthResult{Auth: ModelAuth{APIKey: input.Credential.Key}, Source: "stored"}, nil
			}
			if v := input.Ctx.Env(envVar); v != "" {
				return &AuthResult{Auth: ModelAuth{APIKey: v}, Source: envVar}, nil
			}
			return nil, nil
		},
	}
}

func withAuthClock(t *testing.T, now int64) {
	t.Helper()
	prev := authClock
	authClock = func() int64 { return now }
	t.Cleanup(func() { authClock = prev })
}

func TestResolveOverrideAPIKeyWins(t *testing.T) {
	store := NewInMemoryCredentialStore()
	_, _ = store.Modify(context.Background(), "p", func(Credential) (Credential, error) {
		return &APIKeyCredential{Key: "stored-key"}, nil
	})
	key := "override-key"
	result, err := ResolveProviderAuth(context.Background(), "p",
		ProviderAuth{APIKey: envKeyAuth("TEST_KEY")}, testModel("p", "m", "a"),
		store, mapAuthContext{}, &AuthResolutionOverrides{APIKey: &key})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.Auth.APIKey != "override-key" {
		t.Errorf("result = %+v", result)
	}
}

func TestResolveStoredCredentialOwnsProvider(t *testing.T) {
	store := NewInMemoryCredentialStore()
	_, _ = store.Modify(context.Background(), "p", func(Credential) (Credential, error) {
		return &APIKeyCredential{Key: "stored-key"}, nil
	})
	result, err := ResolveProviderAuth(context.Background(), "p",
		ProviderAuth{APIKey: envKeyAuth("TEST_KEY")}, testModel("p", "m", "a"),
		store, mapAuthContext{"TEST_KEY": "env-key"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.Auth.APIKey != "stored-key" {
		t.Errorf("stored credential must win over env: %+v", result)
	}
}

func TestResolveAmbientEnvOnlyWhenNothingStored(t *testing.T) {
	store := NewInMemoryCredentialStore()
	result, err := ResolveProviderAuth(context.Background(), "p",
		ProviderAuth{APIKey: envKeyAuth("TEST_KEY")}, testModel("p", "m", "a"),
		store, mapAuthContext{"TEST_KEY": "env-key"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.Auth.APIKey != "env-key" || result.Source != "TEST_KEY" {
		t.Errorf("result = %+v", result)
	}
}

func TestResolveStoredOAuthWithoutHandlerReturnsNil(t *testing.T) {
	store := NewInMemoryCredentialStore()
	_, _ = store.Modify(context.Background(), "p", func(Credential) (Credential, error) {
		return &OAuthCredential{Access: "a", Refresh: "r", Expires: 9e15}, nil
	})
	// Provider has no oauth handler: stored credential still owns the
	// provider — no silent env fallback.
	result, err := ResolveProviderAuth(context.Background(), "p",
		ProviderAuth{APIKey: envKeyAuth("TEST_KEY")}, testModel("p", "m", "a"),
		store, mapAuthContext{"TEST_KEY": "env-key"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Errorf("expected nil, got %+v", result)
	}
}

func TestResolveValidOAuthSkipsRefresh(t *testing.T) {
	withAuthClock(t, 1000)
	store := NewInMemoryCredentialStore()
	_, _ = store.Modify(context.Background(), "p", func(Credential) (Credential, error) {
		return &OAuthCredential{Access: "valid-token", Refresh: "r", Expires: 2000}, nil
	})
	refreshed := false
	oauth := &OAuthAuth{
		Name: "test oauth",
		Refresh: func(_ context.Context, c *OAuthCredential) (*OAuthCredential, error) {
			refreshed = true
			return c, nil
		},
		ToAuth: func(_ context.Context, c *OAuthCredential) (ModelAuth, error) {
			return ModelAuth{APIKey: c.Access}, nil
		},
	}
	result, err := ResolveProviderAuth(context.Background(), "p",
		ProviderAuth{OAuth: oauth}, testModel("p", "m", "a"), store, mapAuthContext{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed {
		t.Error("valid token must not refresh")
	}
	if result == nil || result.Auth.APIKey != "valid-token" || result.Source != "OAuth" {
		t.Errorf("result = %+v", result)
	}
}

func TestResolveExpiredOAuthRefreshesOnceUnderLock(t *testing.T) {
	withAuthClock(t, 5000)
	store := NewInMemoryCredentialStore()
	_, _ = store.Modify(context.Background(), "p", func(Credential) (Credential, error) {
		return &OAuthCredential{Access: "old", Refresh: "r", Expires: 1000}, nil
	})
	var refreshCount int
	var mu sync.Mutex
	oauth := &OAuthAuth{
		Name: "test oauth",
		Refresh: func(_ context.Context, c *OAuthCredential) (*OAuthCredential, error) {
			mu.Lock()
			refreshCount++
			mu.Unlock()
			return &OAuthCredential{Access: "new", Refresh: c.Refresh, Expires: 10000}, nil
		},
		ToAuth: func(_ context.Context, c *OAuthCredential) (ModelAuth, error) {
			return ModelAuth{APIKey: c.Access}, nil
		},
	}

	var wg sync.WaitGroup
	results := make([]*AuthResult, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result, err := ResolveProviderAuth(context.Background(), "p",
				ProviderAuth{OAuth: oauth}, testModel("p", "m", "a"), store, mapAuthContext{}, nil)
			if err != nil {
				t.Errorf("resolve: %v", err)
				return
			}
			results[i] = result
		}(i)
	}
	wg.Wait()

	if refreshCount != 1 {
		t.Errorf("refresh count = %d, want 1 (double-checked locking)", refreshCount)
	}
	for i, r := range results {
		if r == nil || r.Auth.APIKey != "new" {
			t.Errorf("result[%d] = %+v", i, r)
		}
	}
	stored, _ := store.Read(context.Background(), "p")
	if cred, ok := stored.(*OAuthCredential); !ok || cred.Access != "new" {
		t.Errorf("rotated credential not persisted: %#v", stored)
	}
}

func TestResolveFailedRefreshIsOAuthError(t *testing.T) {
	withAuthClock(t, 5000)
	store := NewInMemoryCredentialStore()
	_, _ = store.Modify(context.Background(), "p", func(Credential) (Credential, error) {
		return &OAuthCredential{Access: "old", Refresh: "r", Expires: 1000}, nil
	})
	oauth := &OAuthAuth{
		Name: "test oauth",
		Refresh: func(context.Context, *OAuthCredential) (*OAuthCredential, error) {
			return nil, fmt.Errorf("invalid_grant")
		},
		ToAuth: func(_ context.Context, c *OAuthCredential) (ModelAuth, error) {
			return ModelAuth{APIKey: c.Access}, nil
		},
	}
	_, err := ResolveProviderAuth(context.Background(), "p",
		ProviderAuth{OAuth: oauth}, testModel("p", "m", "a"), store, mapAuthContext{}, nil)
	var me *ModelsError
	if !errors.As(err, &me) || me.ErrCode != ModelsErrorOAuth {
		t.Errorf("err = %v", err)
	}
	// The stored credential is preserved for retry.
	stored, _ := store.Read(context.Background(), "p")
	if cred, ok := stored.(*OAuthCredential); !ok || cred.Access != "old" {
		t.Errorf("credential must be preserved after failed refresh: %#v", stored)
	}
}

func TestResolveLoggedOutDuringRefreshReturnsNil(t *testing.T) {
	withAuthClock(t, 5000)
	store := NewInMemoryCredentialStore()
	// Store an api_key credential but resolve with an oauth credential read
	// race: simulate logout-between-read-and-lock by storing oauth then
	// deleting inside a modified store wrapper. Simpler: stored is oauth but
	// gets deleted before Modify runs.
	_, _ = store.Modify(context.Background(), "p", func(Credential) (Credential, error) {
		return &OAuthCredential{Access: "old", Refresh: "r", Expires: 1000}, nil
	})
	oauth := &OAuthAuth{
		Name: "test oauth",
		Refresh: func(_ context.Context, c *OAuthCredential) (*OAuthCredential, error) {
			return &OAuthCredential{Access: "new", Refresh: c.Refresh, Expires: 10000}, nil
		},
		ToAuth: func(_ context.Context, c *OAuthCredential) (ModelAuth, error) {
			return ModelAuth{APIKey: c.Access}, nil
		},
	}
	// Delete before resolution reaches Modify: current inside the lock is nil.
	wrapped := &logoutRacingStore{InMemoryCredentialStore: store}
	result, err := ResolveProviderAuth(context.Background(), "p",
		ProviderAuth{OAuth: oauth}, testModel("p", "m", "a"), wrapped, mapAuthContext{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Errorf("logged-out-meanwhile must resolve to nil, got %+v", result)
	}
}

// logoutRacingStore deletes the credential right before every Modify to
// simulate a logout racing the refresh.
type logoutRacingStore struct {
	*InMemoryCredentialStore
}

func (s *logoutRacingStore) Modify(ctx context.Context, providerID string, fn func(Credential) (Credential, error)) (Credential, error) {
	_ = s.Delete(ctx, providerID)
	return s.InMemoryCredentialStore.Modify(ctx, providerID, fn)
}

func TestCredentialJSONRoundTrip(t *testing.T) {
	oauth := &OAuthCredential{Access: "a", Refresh: "r", Expires: 123, Extra: map[string]any{"accountId": "acct"}}
	data, err := oauth.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	back, err := UnmarshalCredential(data)
	if err != nil {
		t.Fatal(err)
	}
	cred, ok := back.(*OAuthCredential)
	if !ok || cred.Access != "a" || cred.Expires != 123 || cred.Extra["accountId"] != "acct" {
		t.Errorf("round trip = %#v", back)
	}

	apiKey := &APIKeyCredential{Key: "k", Env: ProviderEnv{"ACCOUNT": "x"}}
	data, err = apiKey.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	back, err = UnmarshalCredential(data)
	if err != nil {
		t.Fatal(err)
	}
	if cred, ok := back.(*APIKeyCredential); !ok || cred.Key != "k" || cred.Env["ACCOUNT"] != "x" {
		t.Errorf("round trip = %#v", back)
	}
}
