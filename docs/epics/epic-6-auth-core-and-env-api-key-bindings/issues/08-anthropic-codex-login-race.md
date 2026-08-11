---
type: Issue
title: "Collapse the anthropic and codex login flows onto upstream's always-racing manual-code prompt"
description: "Port the moved auth/oauth/anthropic.ts and openai-codex.ts content: the optional manual-code branch and its text-prompt fallback are gone, the manual prompt always races the callback server, and abort cancels the wait."
tags: [epic-6]
timestamp: 2026-08-11T20:00:00Z
epic: 6
issue: 08
slug: anthropic-codex-login-race
size: L
status: open
gh_issue: 177
resource: https://github.com/kern-ia/kern-link/issues/177
depends_on: ["01"]
---

# Collapse the anthropic and codex login flows onto upstream's always-racing manual-code prompt

## Summary

Upstream moved `utils/oauth/anthropic.ts` and `utils/oauth/openai-codex.ts` to
`auth/oauth/` **with edits**. The move is a no-op for this port —
`ai/auth/oauth` already sits at upstream's destination, which is
[decision 08](../../../planning/scope/08-auth-restructure.md)'s whole point —
but the edits are not. Most of the churn is upstream deleting its parallel
`OAuthProviderInterface` export (`anthropicOAuthProvider`,
`openaiCodexOAuthProvider`), which kern-link never ported, and threading
`AbortSignal` through, which Go's `ctx` already does. Strip those two away and
one real content change remains, in both files:

**The manual-code prompt is no longer optional, and the text-prompt fallback is
gone.** Before, `onManualCodeInput` was an optional callback; when absent the
flow waited on the callback server and then fell back to a plain `text` prompt
("Paste the authorization code or full redirect URL:"). Now there is exactly one
path: a `manual_code` prompt is always started, always races
`server.waitForCode()`, and whichever settles first wins. The fallback prompt
and its message are deleted.

kern-link mirrors the *old* shape, deliberately and with tests:
`ai/auth/oauth/anthropic.go:80-98` documents `OnManualCodeInput` as optional and
`:134` branches on it; `ai/auth/oauth/codex.go:483-499` and `:542` do the same.
So this is a real simplification with real deletions, not a rename.

Three smaller edits ride along, all in the same functions:

- **`redirectUriForExchange` is gone** from the Anthropic flow. It was always
  `REDIRECT_URI`; upstream inlined it.
- **Abort cancels the wait.** Upstream registers
  `interaction.signal.addEventListener("abort", () => server.cancelWait())`, so
  cancelling the login unblocks the callback server rather than waiting for its
  timeout. Go's equivalent is watching `ctx.Done()` alongside the manual
  channel; check whether `ai/auth/oauth/callback.go` already does this before
  writing anything.
- **The state check is uniform.** Both flows now compare
  `parsed.state !== verifier` (anthropic) / `!== state` (codex) on **every**
  manual path, with the same errors — `OAuth state mismatch` and
  `State mismatch` respectively. The two strings differ between the two files;
  that is upstream's, keep it.

Codex keeps its `originator` parameter at `936aff00` (defaulted to `"pi"`), so
`codexOriginator` in `ai/auth/oauth/codex.go:533` stays as it is.

## Scope

- `ai/auth/oauth/anthropic.go`:
  - delete the `OnManualCodeInput != nil` branch (`:134-...`) and the
    `OnPrompt` text fallback; always start the manual-code prompt and race it.
  - delete `redirectUriForExchange`, passing `REDIRECT_URI` directly.
  - cancel the callback wait on `ctx.Done()`.
  - if `OnPrompt` becomes unused, remove it from the options struct — a field
    no path reads is worse than a breaking change.
- `ai/auth/oauth/codex.go` — the same three edits in `loginOpenAICodex`
  (`:483-...`, `:542`). The device-code branch is unchanged apart from ctx
  threading.
- `ai/auth/oauth/devicecode.go` — upstream's `device-code.ts` change in range is
  `signal?: AbortSignal` → `signal: AbortSignal` (91% similarity, +12/-12), i.e.
  the signal became mandatory. Go's `ctx` has always been mandatory here.
  Confirm in one line in the PR body; write no code unless the confirmation
  fails.
- Update both files' `// Ports:` headers to upstream's new paths
  (`packages/ai/src/auth/oauth/anthropic.ts`,
  `packages/ai/src/auth/oauth/openai-codex.ts`) and drop any reference to the
  deleted `utils/oauth/types.ts`.
- The tests that pin the old shape (`ai/auth/oauth/anthropic_test.go`,
  `codex_test.go` — 415 and 738 lines) get rewritten to the new one, not
  deleted wholesale. Upstream's own tests changed by +30 and +91 in range; read
  those diffs for the cases it now considers load-bearing.

## Out of scope

- The Copilot flow — [issue 09](/epic-6-auth-core-and-env-api-key-bindings/issues/09-copilot-model-availability.md).
- `IsSubscription` on these two flows —
  [issue 01](/epic-6-auth-core-and-env-api-key-bindings/issues/01-auth-contract-surface.md)
  sets it.
- `auth/oauth/load.ts` (new, 68 lines) — a bundler-opaque dynamic-import
  registry, the same category as `lazyOAuth`, which `docs/PORTING.md:42`
  already dispositions as not ported. It gets a row in
  [issue 11](/epic-6-auth-core-and-env-api-key-bindings/issues/11-porting-paths-and-dispositions.md),
  not code here.
- The four new flows — [Epic 7](/epic-7-four-new-oauth-flows/EPIC_7.md).
- `oauth-page.ts` and `pkce.ts`, which moved with **zero** content change (the
  diff is `similarity index 100%`). Their `// Ports:` paths are issue 11's.

## Acceptance criteria / Definition of done

- [ ] `grep -n "OnManualCodeInput" ai/auth/oauth/*.go` returns nothing, or
      returns only a field that every caller sets.
- [ ] `TestAnthropicLoginPrefersCallbackServerOverManualCode` — with both
      resolving, the callback server's code is the one exchanged.
- [ ] `TestAnthropicLoginFallsBackToManualCode` — the callback server never
      fires; a manual paste of a full redirect URL completes the login.
- [ ] `TestAnthropicLoginRejectsStateMismatch` — a manual paste whose `state`
      differs from the verifier fails with exactly `OAuth state mismatch`.
- [ ] `TestCodexLoginRejectsStateMismatch` — the same for codex, with exactly
      `State mismatch`. The two strings differ; assert both.
- [ ] `TestCodexBrowserLoginAlwaysOffersManualCode` — the browser branch issues
      an `ai.AuthPromptManualCode` prompt even when the callback server is going
      to win, proving the prompt is no longer conditional.
- [ ] `TestAnthropicLoginCancelUnblocksTheCallbackWait` — cancelling the context
      while the flow waits returns promptly with the context's error, and the
      callback server is closed.
- [ ] `TestCodexDeviceCodeLoginUnchanged` — the device-code branch still
      completes; this issue must not regress it.
- [ ] Both files' `// Ports:` headers name `packages/ai/src/auth/oauth/*.ts`.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `refactor(oauth)!: always race the manual-code prompt against the callback server`.

## Relevant files / areas

- `ai/auth/oauth/anthropic.go:80-98` the options doc, `:134-170` the race,
  `:277-310` the `ai.OAuthAuth` value.
- `ai/auth/oauth/codex.go:483-560` the login options and race, `:533`
  `codexOriginator`.
- `ai/auth/oauth/callback.go` — `waitForCode`/`cancelWait` equivalents; check
  what it already does on context cancellation.
- `ai/auth/oauth/devicecode.go` — the shared poller.
- `ai/auth/oauth/anthropic_test.go` (415), `codex_test.go` (738).
- Upstream: `src/auth/oauth/anthropic.ts` (64% similarity, +232), 
  `src/auth/oauth/openai-codex.ts` (+270), `src/auth/oauth/device-code.ts`
  (+12) at `936aff00`.
- Upstream tests: `test/anthropic-oauth.test.ts` (+30),
  `test/openai-codex-oauth.test.ts` (+91), `test/oauth-device-code.test.ts` (+8).

## Dependencies

- **Blocked by**: [Issue 01](/epic-6-auth-core-and-env-api-key-bindings/issues/01-auth-contract-surface.md)
  (the `AuthLoginCallbacks` → `AuthInteraction` rename runs through both files).
- **Blocks**: Nothing.

## PR size note

`L` — ~800 changed lines: `anthropic_test.go` (415) and `codex_test.go` (738)
are 1153 lines pinning the optional-manual-code shape and all of it is
rewritten, on top of deleting a branch, `redirectUriForExchange` and an options
field from each of the two flows. Re-sized from `M` per REPORT_6. `L` is the
ceiling, not the target: the prompt-race contract is one behavior, and a
half-migrated tree would ship two different login shapes at once. If it trends
past ~1000 the honest cut is 08a anthropic / 08b codex — the files share no code.
