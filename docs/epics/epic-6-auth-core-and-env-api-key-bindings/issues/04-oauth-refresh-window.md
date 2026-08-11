---
type: Issue
title: "Refresh OAuth credentials five minutes before expiry, with a MinOAuthValidity override and a bounded refresh"
description: "Port resolve.ts's new expiry window: refresh when a token expires soon rather than only when expired, honor a caller-requested minimum validity, and cap the refresh call at fifteen seconds."
tags: [epic-6]
timestamp: 2026-08-11T20:00:00Z
epic: 6
issue: 04
slug: oauth-refresh-window
size: M
status: open
gh_issue: 173
resource: https://github.com/kern-ia/kern-link/issues/173
depends_on: ["03"]
---

# Refresh OAuth credentials five minutes before expiry, with a MinOAuthValidity override and a bounded refresh

## Summary

`src/auth/resolve.ts`'s OAuth path changed behavior in range, not just shape.
Today's rule is "refresh when expired"; upstream's is "refresh when expiring
soon", with a caller-tunable floor:

```ts
const DEFAULT_OAUTH_MINIMUM_VALIDITY_MS = 5 * 60 * 1000;
const DEFAULT_OAUTH_REFRESH_TIMEOUT_MS = 15_000;

const minimumValidityMs = Math.max(DEFAULT_OAUTH_MINIMUM_VALIDITY_MS, minOAuthValidityMs ?? 0);
const expiresSoon = (c) => Date.now() + minimumValidityMs >= c.expires;
```

Three distinct changes, all in `resolveStoredOAuth`:

1. **The window.** Both the optimistic check and the authoritative re-check
   under the lock use `expiresSoon`, so a token with four minutes left refreshes
   instead of being handed to a request that will outlive it.
2. **`minOAuthValidityMs`**, a new `AuthResolutionOverrides` field. Callers that
   export a bearer token elsewhere (upstream's stated use) can demand more
   headroom. Note the asymmetry upstream comments on: the *default* five-minute
   window triggers a refresh but imposes no contract, while an **explicit**
   `minOAuthValidityMs` is enforced after the refresh —
   `throw new ModelsError("oauth", "OAuth refresh returned a token that expires too soon for " + providerId)`.
3. **A bounded refresh.** The `oauth.refresh` call runs under
   `AbortSignal.any([signal, AbortSignal.timeout(15_000)])`, so a hung provider
   cannot hold the credential-store lock open indefinitely.

This interacts with a deviation the port already records:
`docs/PORTING.md:43` notes "Codex stores `expires` without the 5-minute margin,
matching upstream". Anthropic and Copilot bake a five-minute margin into the
stored `Expires`; Codex does not. Under the new rule a *second* five minutes is
subtracted at resolution time for the two that already subtracted one. That is
upstream's behavior too — upstream's `anthropic.ts` still writes
`Date.now() + expires_in * 1000 - 5 * 60 * 1000` at `936aff00` — so the port
matches by doing the same thing. **Do not "fix" it by removing a margin**;
record the doubling in the doc comment so the next reader does not.

Go's `authClock` (`ai/resolve.go:56`) is the injectable seam
`CONVENTIONS.md` requires; every test here overrides it rather than sleeping.
The 15-second cap is `context.WithTimeout` on the refresh call — not a
`time.After` race — so cancellation propagates into the provider's HTTP client.

## Scope

- `ai/resolve.go`:
  - `const defaultOAuthMinimumValidity = 5 * time.Minute` and
    `const defaultOAuthRefreshTimeout = 15 * time.Second`, both unexported,
    each with the reason inline (`CONVENTIONS.md`: "Comments explain decisions").
  - `AuthResolutionOverrides.MinOAuthValidity time.Duration` (`:50-53`). Zero
    means "unset" — the field is a duration, not a pointer, because zero is not
    a meaningful minimum; document that.
  - `resolveStoredOAuth` (`:141-186`): replace both `authClock() >= expires`
    comparisons with an `expiresSoon` closure over
    `max(defaultOAuthMinimumValidity, overrides.MinOAuthValidity)`; thread the
    overrides in from `ResolveProviderAuth`.
  - Wrap the `oauth.Refresh` call in a `context.WithTimeout(ctx,
    defaultOAuthRefreshTimeout)` — inside the `Modify` callback, so the lock is
    held for a bounded time.
  - After a successful refresh, when `MinOAuthValidity` was **explicitly** set
    and the new credential still expires within it, fail with
    `NewModelsError(ModelsErrorOAuth, fmt.Sprintf("OAuth refresh returned a token that expires too soon for %s", providerID), nil)`.
    **Verbatim upstream text** — error strings are load-bearing here.
- `ai/provider.go` — pass `MinOAuthValidity` through wherever
  `AuthResolutionOverrides` is constructed, if anywhere does today.

## Out of scope

- `withCauseDetail`. Upstream now folds the cause into `ModelsError.message`
  because "callers surface `error.message` only". Go's `(*ModelsError).Error()`
  already renders `"message: cause"` (`ai/resolve.go:36-41`) and `Unwrap()`
  keeps `errors.Is`/`errors.As` working — the behavior upstream just bought
  itself, the port has had since day one. Confirm that in one line in the PR
  body; write no code.
- `operationSignal` / `raceWithAbortSignal` (`src/utils/abort.ts`, new in
  range). Pure `AbortSignal` ergonomics for an API whose signal is optional;
  Go's mandatory `ctx` first argument is the existing recorded deviation.
  [Issue 11](/epic-6-auth-core-and-env-api-key-bindings/issues/11-porting-paths-and-dispositions.md)
  dispositions the file.
- The `model` parameter removal —
  [issue 03](/epic-6-auth-core-and-env-api-key-bindings/issues/03-provider-scoped-apikey-resolution.md),
  which lands first.
- Changing what any flow stores in `Expires` — see the Summary.

## Acceptance criteria / Definition of done

- [ ] `TestResolveRefreshesTokenExpiringWithinFiveMinutes` — a stored credential
      with `Expires` four minutes ahead of a stubbed `authClock` triggers
      exactly one `Refresh` call and returns the refreshed token's auth.
- [ ] `TestResolveDoesNotRefreshTokenValidBeyondTheWindow` — six minutes ahead,
      zero `Refresh` calls, and the stored credential's access token is what
      `ToAuth` receives.
- [ ] `TestResolveExpiredOAuthRefreshesOnceUnderLock` (existing, or its current
      name) still passes — concurrent resolvers cause exactly one refresh.
- [ ] `TestMinOAuthValidityWidensTheWindow` — `MinOAuthValidity: 30 * time.Minute`
      with a token twenty minutes out refreshes; the same token with no override
      does not.
- [ ] `TestMinOAuthValidityRejectsShortLivedRefresh` — an explicit
      `MinOAuthValidity` whose refresh returns a token inside the window fails
      with a `*ModelsError`, `Code() == "oauth"`, message exactly
      `OAuth refresh returned a token that expires too soon for <id>`.
- [ ] `TestDefaultWindowDoesNotRejectShortLivedRefresh` — the same short-lived
      refreshed token with **no** override resolves successfully. This is the
      asymmetry; without this test the previous one over-constrains.
- [ ] `TestOAuthRefreshIsBounded` — a `Refresh` that blocks on its `ctx` until
      cancelled fails within the timeout rather than hanging, and the stored
      credential is left unchanged so a retry can succeed. Drive it with a
      short timeout injected the same way `authClock` is, or by cancelling the
      caller's context — do not sleep for fifteen seconds.
- [ ] The doubled margin for Anthropic/Copilot is documented in
      `resolveStoredOAuth`'s comment, naming `docs/PORTING.md:43`.
- [ ] Every expiry-window case in upstream's `test/oauth-auth.test.ts` (+67/- in
      range) — the refresh-window, `minOAuthValidityMs` and bounded-refresh
      scenarios — is represented by a Go test in this PR or explicitly
      dispositioned in the PR body. This file's types/`Check` cases belong to
      [issue 01](/epic-6-auth-core-and-env-api-key-bindings/issues/01-auth-contract-surface.md)
      and its `checkAuth`/`getAvailable` cases to
      [issue 05](/epic-6-auth-core-and-env-api-key-bindings/issues/05-models-availability.md) —
      do not duplicate either here.
- [ ] `ai/resolve.go`'s `// Ports:` header still describes the file after this
      change.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(ai): refresh oauth credentials before they expire, not after`.

## Relevant files / areas

- `ai/resolve.go:48-56` `AuthResolutionOverrides` and `authClock`, `:138-186`
  `resolveStoredOAuth`.
- `ai/resolve_test.go` — the existing OAuth refresh tests and the `authClock`
  override pattern.
- `ai/auth.go:52-57` — `OAuthCredential.Expires` (epoch milliseconds).
- `docs/PORTING.md:43` — the recorded Codex margin deviation.
- Upstream: `src/auth/resolve.ts` at `936aff00` (`DEFAULT_OAUTH_MINIMUM_VALIDITY_MS`,
  `DEFAULT_OAUTH_REFRESH_TIMEOUT_MS`, `expiresSoon`).
- Upstream test: `test/oauth-auth.test.ts` (+67/- in range).

## Dependencies

- **Blocked by**: [Issue 03](/epic-6-auth-core-and-env-api-key-bindings/issues/03-provider-scoped-apikey-resolution.md)
  (rewrites `ResolveProviderAuth`'s signature and the same function body).
- **Blocks**: Nothing.

## PR size note

`M` — ~330 changed lines, and the ratio is unusual: the production change is
small (two constants, one `AuthResolutionOverrides` field, and the
`expiresSoon` rewrite of `resolveStoredOAuth`, `ai/resolve.go:141-186`), while
the seven named tests carry the weight — each stubs `authClock` and a `Refresh`
counter, including the bounded-refresh and default-vs-explicit asymmetry cases.
Split past ~500.
