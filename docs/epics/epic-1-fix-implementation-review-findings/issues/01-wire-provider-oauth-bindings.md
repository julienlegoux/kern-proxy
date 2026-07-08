---
type: Issue
title: "Wire provider OAuth bindings to the real strategies"
description: "Point the Anthropic/Codex/Copilot provider auth bindings at the real ai/auth/oauth strategies and fix the stored-Anthropic-OAuth-blocks-API-key-fallback bug."
tags: [epic-1]
timestamp: 2026-07-08T19:44:00Z
epic: 1
issue: 01
slug: wire-provider-oauth-bindings
size: M
status: open
gh_issue: 77
resource: https://github.com/julienlegoux/kern-proxy/issues/77
depends_on: []
---

# Wire provider OAuth bindings to the real strategies

## Summary
`ai/auth/oauth` fully implements `AnthropicOAuth`, `CopilotOAuth`, and `CodexOAuth`,
and `cmd/pi-ai login` already uses them, but the bindings that
`ai.ResolveProviderAuth` (`ai/resolve.go:88-111`) consults are stale. OAuth logins do
not actually resolve, and a stored Anthropic credential silently breaks the API-key
fallback. This PR wires the provider bindings to the real strategies and adds a
regression test for the fallback bug. This is Fix 1 (CRITICAL) of the epic.

## Scope
- `ai/providers/anthropic.go` — import `ai/auth/oauth`; add `OAuth: oauth.AnthropicOAuth`
  to the `ai.ProviderAuth` literal, keeping the existing `APIKey` env strategy and its
  `ANTHROPIC_OAUTH_TOKEN` precedence. Update the header comment (currently says OAuth is
  "deferred to Epic 12").
- `ai/providers/openai_codex.go` — replace the three stub closures with
  `oauth.CodexOAuth`; delete `errCodexOAuthPending` and the now-unused `context`/`errors`
  imports. Update the header comment.
- `ai/providers/github_copilot.go` — replace the three stub closures in the `OAuth`
  field with `oauth.CopilotOAuth`; delete `errCopilotOAuthPending`. Keep the
  `COPILOT_GITHUB_TOKEN` `APIKey` fallback and `RefreshModels` logic untouched. Update
  the header comment.

Root cause of the fallback bug: a stored `*OAuthCredential` with `auth.OAuth == nil`
falls through to `resolve.go:110` returning `nil, nil`, so a saved Anthropic login
blocks the ambient API-key fallback. Adding the binding fixes it.

No import cycle: `ai/auth/oauth` imports `ai` only; `providers` already imports
`ai/auth`.

## Out of scope
- Any change to `ai/auth/oauth` itself (the strategies already work).
- The Google `/v1beta` fix and the `// Ports:` header pass (separate issues in this
  epic).
- Calling the real network `Login` from tests.

## Acceptance criteria / Definition of done
- TDD, test-first (red → green), per repo convention.
- Rework `TestOpenAICodexProviderAdvertisesOAuthPendingEpic12`
  (~`ai/providers/providers_test.go:306`) to assert
  `provider.Auth().OAuth == oauth.CodexOAuth` and `Name == "OpenAI (ChatGPT Plus/Pro)"`
  — do **not** call the real `Login`.
- Rework `TestGitHubCopilotProviderAdvertisesOAuthPendingEpic12`
  (~`ai/providers/github_copilot_test.go:25`) to assert
  `provider.Auth().OAuth == oauth.CopilotOAuth`.
- Add an Anthropic case asserting `AnthropicProvider().Auth().OAuth == oauth.AnthropicOAuth`.
- Leave `TestAnthropicAuthResolvesOAuthTokenPrecedence` (~line 216) unchanged.
- Add a regression test (in `providers_test.go` or `ai/resolve_test.go`): store an
  `*ai.OAuthCredential` for `"anthropic"` and assert `ResolveProviderAuth` returns
  `AuthResult{Source: "OAuth"}` with `Auth.APIKey == <access token>` (not the current
  `nil, nil`).
- The three stale header comments no longer claim OAuth is "deferred to Epic 12".
- `go build ./...` clean (no unused-import errors after removing `errors`/`context`).
- `go test ./... -race` and `go vet ./...` clean.

## Relevant files / areas
- `ai/providers/anthropic.go`, `ai/providers/openai_codex.go`,
  `ai/providers/github_copilot.go`
- `ai/resolve.go` (the `ResolveProviderAuth` fall-through at lines 88-111)
- `ai/auth/oauth` (strategies: `AnthropicOAuth`, `CopilotOAuth`, `CodexOAuth`)
- Tests: `ai/providers/providers_test.go`, `ai/providers/github_copilot_test.go`,
  optionally `ai/resolve_test.go`

## Dependencies
None. Independent of [Issue 02](./02-stop-doubling-google-v1beta.md) and
[Issue 03](./03-add-ports-headers-source-files.md). Part of
[Epic 1](/epic-1-fix-implementation-review-findings/EPIC_1.md).

## PR size note
Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.
Expected M — three small binding edits plus focused tests.
