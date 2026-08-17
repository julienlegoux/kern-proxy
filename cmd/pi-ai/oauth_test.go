package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/kern-ia/kern-link/ai"
)

// fakeOAuthAuth builds an *ai.OAuthAuth whose Login is scripted, for tests
// that never touch a real provider or the network.
func fakeOAuthAuth(name string, login func(ctx context.Context, callbacks ai.AuthLoginCallbacks) (*ai.OAuthCredential, error)) *ai.OAuthAuth {
	return &ai.OAuthAuth{Name: name, Login: login}
}

func fakeEntries() []oauthProviderEntry {
	return []oauthProviderEntry{
		{ID: "alpha", Auth: fakeOAuthAuth("Alpha Provider", func(ctx context.Context, callbacks ai.AuthLoginCallbacks) (*ai.OAuthCredential, error) {
			return &ai.OAuthCredential{Access: "alpha-access", Refresh: "alpha-refresh", Expires: 123}, nil
		})},
		{ID: "beta", Auth: fakeOAuthAuth("Beta Provider", func(ctx context.Context, callbacks ai.AuthLoginCallbacks) (*ai.OAuthCredential, error) {
			return &ai.OAuthCredential{Access: "beta-access", Refresh: "beta-refresh", Expires: 456}, nil
		})},
	}
}

func TestRunLogin_UnknownProviderErrors(t *testing.T) {
	store := ai.NewInMemoryCredentialStore()
	var stdout bytes.Buffer

	err := runLogin(context.Background(), []string{"nope"}, strings.NewReader(""), &stdout, store, fakeEntries())
	if err == nil {
		t.Fatal("expected an error for an unknown provider")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error should name the unknown provider, got: %v", err)
	}
}

func TestRunLogin_ExplicitProviderPersistsCredential(t *testing.T) {
	store := ai.NewInMemoryCredentialStore()
	var stdout bytes.Buffer

	err := runLogin(context.Background(), []string{"beta"}, strings.NewReader(""), &stdout, store, fakeEntries())
	if err != nil {
		t.Fatalf("runLogin: %v", err)
	}

	cred, err := store.Read(context.Background(), "beta")
	if err != nil {
		t.Fatalf("store.Read: %v", err)
	}
	oauthCred, ok := cred.(*ai.OAuthCredential)
	if !ok {
		t.Fatalf("expected *ai.OAuthCredential, got %T", cred)
	}
	if oauthCred.Access != "beta-access" {
		t.Errorf("Access = %q, want %q", oauthCred.Access, "beta-access")
	}
	if !strings.Contains(stdout.String(), "beta") {
		t.Errorf("expected stdout to mention the provider, got:\n%s", stdout.String())
	}
}

func TestRunLogin_InteractiveSelectionPersistsCredential(t *testing.T) {
	store := ai.NewInMemoryCredentialStore()
	var stdout bytes.Buffer

	// No provider arg: the CLI should prompt a numbered select; "2" picks
	// the second entry (beta).
	err := runLogin(context.Background(), nil, strings.NewReader("2\n"), &stdout, store, fakeEntries())
	if err != nil {
		t.Fatalf("runLogin: %v", err)
	}

	cred, err := store.Read(context.Background(), "beta")
	if err != nil {
		t.Fatalf("store.Read: %v", err)
	}
	if cred == nil {
		t.Fatal("expected a credential to be stored for beta")
	}
	if !strings.Contains(stdout.String(), "Alpha Provider") || !strings.Contains(stdout.String(), "Beta Provider") {
		t.Errorf("expected both provider names listed in the selection prompt, got:\n%s", stdout.String())
	}
}

func TestRunLogin_LoginFailurePropagates(t *testing.T) {
	store := ai.NewInMemoryCredentialStore()
	var stdout bytes.Buffer
	wantErr := errors.New("boom")
	entries := []oauthProviderEntry{
		{ID: "gamma", Auth: fakeOAuthAuth("Gamma", func(ctx context.Context, callbacks ai.AuthLoginCallbacks) (*ai.OAuthCredential, error) {
			return nil, wantErr
		})},
	}

	err := runLogin(context.Background(), []string{"gamma"}, strings.NewReader(""), &stdout, store, entries)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected login failure to propagate, got: %v", err)
	}
}

func TestInteractiveCallbacks_PromptTextTrimsInput(t *testing.T) {
	var stdout bytes.Buffer
	callbacks := interactiveCallbacks(strings.NewReader("hello world  \n"), &stdout)

	got, err := callbacks.Prompt(context.Background(), ai.AuthPrompt{Type: ai.AuthPromptText, Message: "Enter something"})
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if got != "hello world" {
		t.Errorf("Prompt = %q, want %q", got, "hello world")
	}
}

func TestInteractiveCallbacks_PromptSelectReturnsOptionID(t *testing.T) {
	var stdout bytes.Buffer
	callbacks := interactiveCallbacks(strings.NewReader("2\n"), &stdout)

	got, err := callbacks.Prompt(context.Background(), ai.AuthPrompt{
		Type:    ai.AuthPromptSelect,
		Message: "Pick one",
		Options: []ai.AuthPromptOption{{ID: "one", Label: "One"}, {ID: "two", Label: "Two"}},
	})
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if got != "two" {
		t.Errorf("Prompt = %q, want %q", got, "two")
	}
}

func TestInteractiveCallbacks_PromptSelectInvalidChoiceErrors(t *testing.T) {
	var stdout bytes.Buffer
	callbacks := interactiveCallbacks(strings.NewReader("9\n"), &stdout)

	_, err := callbacks.Prompt(context.Background(), ai.AuthPrompt{
		Type:    ai.AuthPromptSelect,
		Message: "Pick one",
		Options: []ai.AuthPromptOption{{ID: "one", Label: "One"}},
	})
	if err == nil {
		t.Fatal("expected an error for an out-of-range selection")
	}
}

func TestInteractiveCallbacks_NotifyWritesAuthURLAndDeviceCode(t *testing.T) {
	var stdout bytes.Buffer
	callbacks := interactiveCallbacks(strings.NewReader(""), &stdout)

	callbacks.Notify(ai.AuthEvent{Type: ai.AuthEventAuthURL, URL: "https://example.com/auth", Instructions: "paste the code"})
	callbacks.Notify(ai.AuthEvent{Type: ai.AuthEventDeviceCode, UserCode: "ABCD-1234", VerificationURI: "https://example.com/device"})
	callbacks.Notify(ai.AuthEvent{Type: ai.AuthEventProgress, Message: "still working..."})

	out := stdout.String()
	for _, want := range []string{"https://example.com/auth", "paste the code", "ABCD-1234", "https://example.com/device", "still working..."} {
		if !strings.Contains(out, want) {
			t.Errorf("Notify output missing %q, got:\n%s", want, out)
		}
	}
}

func TestInteractiveCallbacks_PromptRespectsContextCancellation(t *testing.T) {
	var stdout bytes.Buffer
	// pipeReader never yields a line, forcing the ctx-cancellation path.
	pr, _ := io.Pipe()
	callbacks := interactiveCallbacks(pr, &stdout)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := callbacks.Prompt(ctx, ai.AuthPrompt{Type: ai.AuthPromptText, Message: "Enter something"})
	if err == nil {
		t.Fatal("expected the prompt to fail once ctx is cancelled")
	}
}
