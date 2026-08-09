---
type: Issue
title: "Extend the auth contract: AuthCheck, AuthType, subscription metadata, and the info auth event"
description: "Port auth/types.ts's 0.84.1 additions into ai/auth.go — the availability-check contract, the auth-type discriminator, OAuth subscription metadata, and the info event — and rename AuthLoginCallbacks to AuthInteraction."
tags: [epic-6]
timestamp: 2026-08-09T10:24:00Z
epic: 6
issue: 01
slug: auth-contract-surface
size: M
status: open
gh_issue: 170
resource: https://github.com/kern-ia/kern-link/issues/170
depends_on: []
---

# Extend the auth contract: AuthCheck, AuthType, subscription metadata, and the info auth event

## Summary

`src/auth/types.ts` gained +82/-25 in range, and almost everything downstream in
this epic depends on the types it added. This issue lands the **type surface
only**, so the four issues that consume it (05, 06, 08, 09) start from a
compiling contract rather than each inventing its own.

Upstream's additions, and what each is for:

| Upstream | Purpose |
|---|---|
| `AuthType = "api_key" \| "oauth"` | discriminator `Models.login(providerId, type, …)` takes |
| `AuthCheck { source?, type }` | result of a **side-effect-free** availability check |
| `ApiKeyAuth.check?({ ctx, credential, signal })` | the optional hook that produces it — "use this when `resolve()` may execute commands or perform other request-time work" |
| `OAuthAuth.isSubscription?` | whether access is backed by a provider subscription |
| `OAuthAuth.loginLabel?` | selector label, e.g. `"Sign in with SuperGrok or X Premium"` |
| `AuthInfoLink { url, label? }` + `AuthEvent` variant `{ type: "info", message, links? }` | a login-progress event carrying links |
| `AuthLoginCallbacks` → `AuthInteraction`, plus `ProviderAuthInteraction = AuthInteraction & { signal: AbortSignal }` | the rename that runs through every flow |

Two of these are **no-ops in Go and must be recorded as such rather than
ported**: `AuthOperationOptions { signal? }` and `ProviderAuthInteraction`'s
non-optional `signal` both exist because upstream's public auth API took an
optional `AbortSignal`. Every Go signature in this tree already takes
`ctx context.Context` as its first argument, which
[docs/PORTING.md](../../../../docs/PORTING.md)'s "Cancellation" deviation
already covers. Do not add a parallel options struct.

`AuthEvent` is a Go struct with a `Type` discriminator and a flat field set
(`ai/auth.go:218-230`), not a TS union, so the `info` variant is a new
`AuthEventInfo` constant plus a `Links []AuthInfoLink` field — `Message` is
already there.

`isSubscription` is set on exactly three of the flows this port already has
(`auth/oauth/anthropic.ts:357`, `github-copilot.ts:402`,
`openai-codex.ts:517`), so this issue sets it on all three; the field is not
left dead. `loginLabel` is set only on flows that arrive in
[Epic 7](/epic-7-four-new-oauth-flows/EPIC_7.md) (`kimi-coding`, `openrouter`,
`xai`), so the field lands here unused and epic 7 fills it — say so in a doc
comment so its absence does not read as an oversight.

## Scope

- `ai/auth.go`:
  - `type AuthType string` with `AuthTypeAPIKey`/`AuthTypeOAuth` constants
    valued `"api_key"`/`"oauth"`. Reuse the existing `CredentialType`
    constants' spellings — they are the same two strings — but keep the types
    distinct, matching upstream (`Credential["type"]` vs `AuthType`).
  - `type AuthCheck struct { Source string; Type AuthType }`.
  - `APIKeyAuth.Check func(ctx context.Context, input APIKeyCheckInput) (*AuthCheck, error)`,
    nil when the provider has no cheap check. Its input mirrors
    `APIKeyResolveInput` (`Ctx AuthContext`, `Credential *APIKeyCredential`);
    **do not give it a `Model` field** — upstream's `check` never had one, and
    [issue 03](/epic-6-auth-core-and-env-api-key-bindings/issues/03-provider-scoped-apikey-resolution.md)
    removes it from `resolve` too.
  - `OAuthAuth.IsSubscription bool` and `OAuthAuth.LoginLabel string`.
  - `AuthInfoLink struct { URL, Label string }`, `AuthEventInfo AuthEventType
    = "info"`, and `Links []AuthInfoLink` on `AuthEvent`.
  - Rename `AuthLoginCallbacks` → `AuthInteraction`. This is a breaking rename
    of an exported type, which the port's two-level flow already accommodates
    (`CONVENTIONS.md`, "Commits & branches": `!` marks breaking changes). No
    alias is left behind — the port does not ship back-compat shims
    (`docs/PORTING.md`, "Intentional deviations", `src/compat.ts`).
- Set `IsSubscription: true` on `oauth.AnthropicOAuth`
  (`ai/auth/oauth/anthropic.go`), the Copilot flow (`ai/auth/oauth/copilot.go`)
  and the Codex flow (`ai/auth/oauth/codex.go`).
- Mechanical sweep of the rename across `ai/auth/oauth/*.go`,
  `ai/auth/oauth/*_test.go`, `cmd/pi-ai/oauth.go`, `cmd/pi-ai/oauth_test.go`,
  and any provider binding that names the type.

## Out of scope

- `Models.CheckAuth` / `GetAvailable`, which *consume* `AuthCheck` —
  [issue 05](/epic-6-auth-core-and-env-api-key-bindings/issues/05-models-availability.md).
- `Models.Login` / `Logout`, which consume `AuthType` —
  [issue 06](/epic-6-auth-core-and-env-api-key-bindings/issues/06-models-login-logout.md).
- Writing an actual `Check` implementation for any provider. Upstream ships
  none in `packages/ai` at `936aff00` either — the hook exists for apps whose
  `resolve` shells out. Adding one speculatively is scope creep.
- `APIKeyResolveInput.Model` removal and everything it forces —
  [issue 03](/epic-6-auth-core-and-env-api-key-bindings/issues/03-provider-scoped-apikey-resolution.md).
- `CredentialInfo` / `CredentialStore.List` —
  [issue 02](/epic-6-auth-core-and-env-api-key-bindings/issues/02-credential-store-list.md),
  which owns every `CredentialStore` edit so the interface and its two
  implementations change in one compiling PR.

## Acceptance criteria / Definition of done

- [ ] `grep -rn "AuthLoginCallbacks" --include=*.go .` returns nothing.
- [ ] `TestOAuthFlowsDeclareSubscriptionAccess` — `oauth.AnthropicOAuth`, the
      Copilot flow and the Codex flow each report `IsSubscription == true`;
      assert on the exported flow values, not on a copy.
- [ ] `TestAuthEventInfoCarriesLinks` — an `AuthEvent{Type: ai.AuthEventInfo,
      Message: …, Links: []ai.AuthInfoLink{{URL: …, Label: …}}}` round-trips
      through a `Notify` func unchanged, and `AuthEventInfo == "info"` asserted
      as a string (the constant's wire spelling is the contract).
- [ ] `TestAuthTypeConstantsMatchCredentialTypes` — `string(ai.AuthTypeAPIKey)
      == string(ai.CredentialTypeAPIKey)` and the same for oauth, `got`/`want`.
      This is the invariant `Models.Login` will rely on.
- [ ] A nil `APIKeyAuth.Check` is legal and is what all 35 existing bindings
      have: `TestAPIKeyAuthCheckIsOptional` constructs a binding without it and
      asserts nothing panics when it is inspected.
- [ ] `ai/auth.go` keeps its `// Ports:` header and its upstream path is
      current (`packages/ai/src/auth/types.ts` — unchanged location; the
      `packages/ai/src/utils/oauth/types.ts` half of the existing header is now
      `packages/ai/src/auth/types.ts` too, since upstream folded
      `OAuthCredentials` in. Fix that line here rather than leaving it dangling
      for [issue 11](/epic-6-auth-core-and-env-api-key-bindings/issues/11-porting-paths-and-dispositions.md)).
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(ai)!: extend the auth contract with AuthCheck, AuthType and subscription metadata`.

## Relevant files / areas

- `ai/auth.go` — `:180-238` the prompt/event/callbacks block, `:240-279`
  `APIKeyResolveInput`, `APIKeyAuth`, `OAuthAuth`, `ProviderAuth`.
- `ai/auth/oauth/anthropic.go:277+`, `ai/auth/oauth/copilot.go`,
  `ai/auth/oauth/codex.go` — the three exported `ai.OAuthAuth` values.
- `cmd/pi-ai/oauth.go:125` — `interactiveCallbacks`, the only production
  constructor of the renamed type.
- Upstream: `packages/ai/src/auth/types.ts` at `936aff00`.
- Upstream test: `test/oauth-auth.test.ts` (+67/- in range).

## Dependencies

- **Blocked by**: None.
- **Blocks**: [Issue 03](/epic-6-auth-core-and-env-api-key-bindings/issues/03-provider-scoped-apikey-resolution.md),
  [05](/epic-6-auth-core-and-env-api-key-bindings/issues/05-models-availability.md),
  [06](/epic-6-auth-core-and-env-api-key-bindings/issues/06-models-login-logout.md),
  [08](/epic-6-auth-core-and-env-api-key-bindings/issues/08-anthropic-codex-login-race.md),
  [09](/epic-6-auth-core-and-env-api-key-bindings/issues/09-copilot-model-availability.md).
  Land it first; the rename alone conflicts with every one of them.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
