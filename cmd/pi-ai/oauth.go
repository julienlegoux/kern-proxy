package main

// Ports: packages/ai/src/cli.ts's login()/main()'s "login" branch and the
// callbacks object passed to provider.login() (onAuth/onDeviceCode/
// onPrompt/onSelect/onProgress) — merged into the ai.AuthLoginCallbacks
// shape already used by ai/auth/oauth (Epic 12).

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/auth/oauth"
)

// oauthProviderEntry is one selectable login target: an id (used on the
// command line and as the credential-store key) plus the ai.OAuthAuth
// strategy that drives it (ai/auth/oauth, Epic 12).
type oauthProviderEntry struct {
	ID   string
	Auth *ai.OAuthAuth
}

// oauthProviders lists the built-in OAuth login targets in a stable order.
//
// Ports: packages/ai/src/cli.ts's PROVIDERS (getOAuthProviders(), reading
// utils/oauth/index.ts's provider registry). That registry is keyed the
// same way this port's ai/auth/oauth strategies are: "anthropic",
// "github-copilot", "openai-codex" (matching ai/providers' provider ids for
// the same services), so `pi-ai login <id>` accepts the same names as
// `pi-ai list`'s provider column for these three.
func oauthProviders() []oauthProviderEntry {
	return []oauthProviderEntry{
		{ID: "anthropic", Auth: oauth.AnthropicOAuth},
		{ID: "github-copilot", Auth: oauth.CopilotOAuth},
		{ID: "openai-codex", Auth: oauth.CodexOAuth},
	}
}

// runLogin drives one OAuth provider's Login flow (ai/auth/oauth, Epic 12)
// through ai.AuthLoginCallbacks and persists the resulting credential via
// store (ai/auth.FileCredentialStore in production, Epic 3's file-backed
// credential store).
//
// Ports: packages/ai/src/cli.ts's login()/the "login" branch of main() —
// upstream prompts for a provider when none is given on the command line,
// runs the flow, and persists the result to a local ./auth.json. This port
// persists to the shared ~/.pi/agent/auth.json store instead, the same file
// every other ai/auth consumer reads (see PORTING.md's cli.ts row for this
// deviation — a local per-invocation file would fragment credentials from
// the rest of the port).
func runLogin(
	ctx context.Context,
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	store ai.CredentialStore,
	entries []oauthProviderEntry,
) error {
	callbacks := interactiveCallbacks(stdin, stdout)

	selected, err := resolveLoginProvider(ctx, args, callbacks, entries)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Logging in to %s...\n", selected.ID)
	credential, err := selected.Auth.Login(ctx, callbacks)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	if _, err := store.Modify(ctx, selected.ID, func(ai.Credential) (ai.Credential, error) {
		return credential, nil
	}); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}

	fmt.Fprintf(stdout, "\nCredentials saved for %s\n", selected.ID)
	return nil
}

// resolveLoginProvider picks the login target: the id named on the command
// line, or an interactive numbered prompt over entries when none was given.
func resolveLoginProvider(
	ctx context.Context,
	args []string,
	callbacks ai.AuthLoginCallbacks,
	entries []oauthProviderEntry,
) (*oauthProviderEntry, error) {
	if len(args) > 0 && args[0] != "" {
		id := args[0]
		for i := range entries {
			if entries[i].ID == id {
				return &entries[i], nil
			}
		}
		return nil, fmt.Errorf("unknown provider: %s (use 'pi-ai list' to see available providers)", id)
	}

	options := make([]ai.AuthPromptOption, len(entries))
	for i, e := range entries {
		options[i] = ai.AuthPromptOption{ID: e.ID, Label: e.Auth.Name}
	}
	selectedID, err := callbacks.Prompt(ctx, ai.AuthPrompt{
		Type:    ai.AuthPromptSelect,
		Message: "Select a provider:",
		Options: options,
	})
	if err != nil {
		return nil, err
	}
	for i := range entries {
		if entries[i].ID == selectedID {
			return &entries[i], nil
		}
	}
	return nil, fmt.Errorf("invalid selection: %q", selectedID)
}

// interactiveCallbacks builds an ai.AuthLoginCallbacks that reads prompts
// from reader and writes progress/auth-url/device-code notifications to
// stdout. It is shared by every OAuth flow (Anthropic PKCE, Copilot device
// code, Codex's dual flow) since AuthLoginCallbacks is flow-agnostic.
//
// Ports: packages/ai/src/cli.ts's prompt()/the login() callbacks object
// (onAuth/onDeviceCode/onPrompt/onSelect/onProgress) — merged into one
// Prompt/Notify pair matching ai.AuthLoginCallbacks' shape.
func interactiveCallbacks(stdin io.Reader, stdout io.Writer) ai.AuthLoginCallbacks {
	reader := bufio.NewReader(stdin)
	return ai.AuthLoginCallbacks{
		Prompt: func(ctx context.Context, p ai.AuthPrompt) (string, error) {
			if p.Type == ai.AuthPromptSelect {
				fmt.Fprintf(stdout, "\n%s\n", p.Message)
				for i, opt := range p.Options {
					fmt.Fprintf(stdout, "  %d. %s\n", i+1, opt.Label)
				}
				line, err := readLine(ctx, p, reader, stdout, fmt.Sprintf("Enter number (1-%d): ", len(p.Options)))
				if err != nil {
					return "", err
				}
				index, convErr := strconv.Atoi(line)
				if convErr != nil || index < 1 || index > len(p.Options) {
					return "", fmt.Errorf("invalid selection: %q", line)
				}
				return p.Options[index-1].ID, nil
			}

			label := p.Message
			if p.Placeholder != "" {
				label += fmt.Sprintf(" (%s)", p.Placeholder)
			}
			return readLine(ctx, p, reader, stdout, label+": ")
		},
		Notify: func(event ai.AuthEvent) {
			switch event.Type {
			case ai.AuthEventAuthURL:
				fmt.Fprintf(stdout, "\nOpen this URL in your browser:\n%s\n", event.URL)
				if event.Instructions != "" {
					fmt.Fprintln(stdout, event.Instructions)
				}
				fmt.Fprintln(stdout)
			case ai.AuthEventDeviceCode:
				fmt.Fprintf(stdout, "\nOpen this URL in your browser:\n%s\n", event.VerificationURI)
				fmt.Fprintf(stdout, "Enter code: %s\n\n", event.UserCode)
			case ai.AuthEventProgress:
				fmt.Fprintln(stdout, event.Message)
			}
		},
	}
}

// readLine writes label to stdout, then reads one line from reader,
// trimming its trailing newline/whitespace. The read runs in a goroutine so
// a cancelled ctx (or a resolved p.Ctx — set for manual_code prompts raced
// against a callback server, see ai.AuthPrompt's doc comment) aborts the
// prompt instead of blocking forever on stdin.
func readLine(ctx context.Context, p ai.AuthPrompt, reader *bufio.Reader, stdout io.Writer, label string) (string, error) {
	fmt.Fprint(stdout, label)

	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		line, err := reader.ReadString('\n')
		ch <- result{line, err}
	}()

	var promptDone <-chan struct{}
	if p.Ctx != nil {
		promptDone = p.Ctx.Done()
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-promptDone:
		return "", p.Ctx.Err()
	case r := <-ch:
		if r.err != nil && r.line == "" {
			return "", r.err
		}
		return strings.TrimSpace(r.line), nil
	}
}
