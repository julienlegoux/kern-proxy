---
type: Issue
title: "Port the OpenRouter PKCE OAuth flow and bind it to both OpenRouter providers"
description: "Port src/auth/oauth/openrouter.ts as ai/auth/oauth/openrouter.go — a one-shot loopback callback on an ephemeral port raced against a manual-code prompt, exchanging the code for a permanent API key — and wire it into the text and image OpenRouter bindings."
tags: [epic-7]
timestamp: 2026-08-09T13:40:00Z
epic: 7
issue: 03
slug: openrouter-pkce-oauth
size: M
status: open
gh_issue: 183
resource: https://github.com/kern-ia/kern-link/issues/183
depends_on: []
---

# Port the OpenRouter PKCE OAuth flow and bind it to both OpenRouter providers

## Summary

OpenRouter is the odd one of the four: it is PKCE, not device code, and what
it hands back is **not a token pair**. The code exchange mints a permanent,
user-controlled API key, so upstream stores
`{ access: key, refresh: "", expires: Number.MAX_SAFE_INTEGER }` and makes
`refresh` a function that returns its argument unchanged. Two consequences
worth stating in the code, because both look like bugs otherwise:

- `Refresh` is a no-op, not an oversight. Go's version returns the credential
  it was given.
- `Expires` must be far enough out that
  [epic 6 issue 04](/epic-6-auth-core-and-env-api-key-bindings/issues/04-oauth-refresh-window.md)'s
  five-minute "expiring soon" window never fires. `math.MaxInt64` overflows on
  `now + window`; use `Number.MAX_SAFE_INTEGER`'s value (`9007199254740991`)
  exactly as upstream does — it is also what round-trips through the store's
  JSON without precision loss. Comment the choice.

The login itself is the callback-vs-manual race the Anthropic flow already
runs, with one structural difference that decides how much can be reused:
**the token exchange happens inside the callback handler**, so the browser
gets a real success or failure page (`oauthSuccessHtml` /
`oauthErrorHtml` with the exchange error), and a second request to the same
callback URL is answered `409` (`This OAuth callback has already been used.`)
rather than starting a second exchange. `ai/auth/oauth/callback.go`'s
`CallbackServer` delivers a `code`/`state` pair and lets the *caller* exchange
it, which cannot produce those pages and has no claim semantics. It also
validates `state`, which this flow does not use at all, and builds its
`RedirectURI` from the literal host `localhost` rather than the bound
interface.

**Decide this explicitly and record the decision in the PR body**: either
(a) write a small flow-local server in `openrouter.go`, leaving
`callback.go` untouched, or (b) generalise `CallbackServer` with an exchange
hook. Prefer (a) unless generalising costs less than ~40 lines — three flows
already depend on `callback.go`'s current behaviour, and
[issue 04](/epic-7-four-new-oauth-flows/issues/04-radius-gateway-oauth.md)
depends on it too; a shared-scaffolding change from a single-flow PR is how
that scaffolding breaks.

Two name collisions to plan around, because everything here lives in package
`oauth`:

- `parseAuthorizationInput` already exists (`ai/auth/oauth/anthropic.go`) and
  returns a `(code, state)` pair. OpenRouter's parser returns a code only and
  accepts a bare code as a last resort. Name the new one for its flow;
  do not overload the old one with a second meaning.
- `anthropicCallbackHost()` (`ai/auth/oauth/anthropic.go:44`) already reads
  `PI_OAUTH_CALLBACK_HOST` with a `127.0.0.1` fallback — exactly what this
  flow needs. Rename it to `callbackHost()` and use it from both, in one
  mechanical commit; that is a strictly smaller change than a second copy.

The manual-code path is not a fallback here — it is always started and always
races, the same shape
[epic 6 issue 08](/epic-6-auth-core-and-env-api-key-bindings/issues/08-anthropic-codex-login-race.md)
collapses the Anthropic and Codex flows onto. Land after it if both are in
flight, and copy its resolved shape rather than inventing a third.

## Scope

- `ai/auth/oauth/openrouter.go` (new) —
  `// Ports: packages/ai/src/auth/oauth/openrouter.ts`:
  - Constants: authorize URL `https://openrouter.ai/auth`; token URL
    `https://openrouter.ai/api/v1/auth/keys` (a package var for tests); login
    timeout 5 minutes; exchange timeout 30 seconds.
  - A one-shot loopback callback server on an **ephemeral port** (`:0`) bound
    to `callbackHost()`, serving a per-login random path
    (`/oauth/callback/<uuid>` upstream — any unguessable token; state the
    generator in a comment). Behaviour, all of which upstream tests assert:
    non-matching method or path → `404` + error page; already claimed or
    settled → `409` + `This OAuth callback has already been used.`;
    `?error=` → `400` + error page and the login fails
    `OpenRouter authorization failed: <description>`; no `code` → `400` +
    error page and the server keeps listening; a `code` → claim, exchange,
    then `200` + success page or `502` + the exchange error's message.
  - The exchange: POST **JSON** `{code, code_verifier, code_challenge_method:
    "S256"}` (this one is JSON, not a form — `PostJSON` is close but errors on
    non-2xx and this flow needs the body's `error_description`; extend or
    bypass deliberately and say which). Errors, verbatim:
    `OpenRouter OAuth token exchange timed out`,
    `OpenRouter OAuth returned invalid JSON`,
    `OpenRouter OAuth key exchange failed (HTTP <status>)[: <detail>]`,
    `OpenRouter OAuth response carries no "key"`, `Login cancelled`. The
    detail is drawn from `error_description`, then `message`, then `error`,
    then `error.message` — in that order.
  - `login`: `GeneratePKCE()`, start the server, notify a
    `progress` event naming the callback URL and an `auth_url` event carrying
    `<AUTHORIZE_URL>?callback_url=…&code_challenge=…&code_challenge_method=S256`
    with upstream's instructions string, start the `manual_code` prompt, and
    take whichever settles first. A manual input is parsed as a full redirect
    URL, then as a bare query string containing `code=`, then as a bare code;
    empty input fails `Missing authorization code`. Manual exchange emits a
    `progress` event first (`Exchanging authorization code for an API key...`).
  - Server-lifecycle errors: `OpenRouter OAuth login timed out` after five
    minutes, `Could not determine the OpenRouter OAuth callback port` if the
    listener has no usable address, `Login cancelled` when ctx is already done
    before the server starts.
  - `toAuth`: `ai.ModelAuth{APIKey: credential.Access}`.
  - The exported strategy `OpenRouterOAuth *ai.OAuthAuth` with
    `Name: "OpenRouter OAuth"` and
    `LoginLabel: "Sign in with OpenRouter"`. **No `IsSubscription`** —
    upstream does not set it, because what you get is a real API key. That
    absence is meaningful and issue 06 relies on it.
- `ai/providers/openrouter.go` — add `OAuth: oauth.OpenRouterOAuth` alongside
  the existing `APIKey`.
- `ai/images/builtin.go:20` — the same addition on the image provider.
  Upstream's test asserts both providers expose OAuth and that a single stored
  credential resolves for both; the two bindings share the `openrouter`
  credential-store key, so this is what makes one login serve both surfaces.
- `ai/auth/oauth/anthropic.go` — rename `anthropicCallbackHost` to
  `callbackHost` (mechanical; update its doc comment to name both callers).
- `ai/auth/oauth/openrouter_test.go` (new) — the port of
  `test/openrouter-oauth.test.ts` (322 lines).

## Out of scope

- `cmd/pi-ai login` wiring — [issue 05](/epic-7-four-new-oauth-flows/issues/05-cli-login-new-flows.md).
- `docs/auth.md` / `docs/PORTING.md` — [issue 06](/epic-7-four-new-oauth-flows/issues/06-auth-docs-and-porting.md).
- `RefreshModels` and the dynamic OpenRouter catalog. This issue changes how
  the provider *authenticates*, not what it lists.
- Changing `CallbackServer`'s existing contract for the three flows that use
  it. If option (b) above is chosen, it is additive and the existing tests
  stay green unmodified — if they need edits, that is the signal to take
  option (a).
- `ai/images`' own auth resolution. It reuses `ai.ProviderAuth`; adding the
  field is the whole change there.

## Acceptance criteria / Definition of done

- [ ] `TestOpenRouterLoginExchangesTheCallbackCodeForAPermanentKey` — a full
      PKCE round trip against an `httptest` token server: the browser hits the
      callback URL, the response body is the success page, and the returned
      credential has `Access == <key>`, `Refresh == ""`,
      `Expires == 9007199254740991`.
- [ ] `TestOpenRouterCallbackReportsExchangeFailureOnThePageAndInLogin` — a
      token server returning 500 makes the callback response a `502` carrying
      the exchange error text, **and** login fails with
      `OpenRouter OAuth key exchange failed (HTTP 500)…`. Both halves, one
      test — that pairing is upstream's own.
- [ ] `TestOpenRouterAllowsOnlyOneExchangePerCallback` — a second GET to the
      callback URL is answered `409` and the token server receives exactly one
      request.
- [ ] `TestOpenRouterRejectsAResponseWithoutAKey` — a 200 with no `key` fails
      with message exactly `OpenRouter OAuth response carries no "key"`.
- [ ] `TestOpenRouterMintsAKeyFromAPastedRedirectURL` — with no browser
      callback, a manual input of the full redirect URL completes the login.
- [ ] `TestOpenRouterAcceptsABareAuthorizationCode` — a manual input that is
      just the code completes the login.
- [ ] `TestOpenRouterRejectsEmptyManualInput` — empty manual input fails
      `Missing authorization code` and the token server receives **zero**
      requests.
- [ ] `TestOpenRouterFailsWhenTheManualPromptIsCancelled` — a `Prompt` that
      returns an error fails the login with that error.
- [ ] `TestOpenRouterClosesThePendingCallbackWhenLoginIsCancelled` — cancelling
      the context unblocks the login and the listener is closed (a follow-up
      dial to the callback address fails).
- [ ] `TestOpenRouterRejectsBeforeStartingAServerWhenAlreadyCancelled` — an
      already-cancelled context fails `Login cancelled` and binds no port.
- [ ] `TestOpenRouterUsesTheConfiguredCallbackHost` — with
      `PI_OAUTH_CALLBACK_HOST` set (via `t.Setenv`), the notified callback URL
      and the bound listener both use it.
- [ ] `TestOpenRouterRefreshReturnsTheCredentialUnchanged` — `Refresh` performs
      no HTTP request and returns an equal credential.
- [ ] `TestOpenRouterProvidersDeclareOAuth` — both `ai/providers`'
      `OpenRouterProvider()` and `ai/images`' `OpenRouterProvider()` expose a
      non-nil `OAuth` alongside `APIKey`, and a stored OAuth credential
      resolves to the same API key through both.
- [ ] The existing `ai/auth/oauth` tests still pass unmodified after the
      `callbackHost` rename.
- [ ] Tests are offline, stdlib-only, no `t.Parallel()`, no build tags,
      discrete named functions. No test binds a fixed port.
- [ ] `// Ports: packages/ai/src/auth/oauth/openrouter.ts` header on
      `openrouter.go`.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(auth): port the openrouter pkce oauth flow`.

## Relevant files / areas

- `ai/auth/oauth/callback.go` — `StartCallbackServer(host, port, path,
  expectedState)`, `RedirectURI`, `WaitForCode`, `Cancel`, `Close`: what can
  and cannot be reused, per the Summary.
- `ai/auth/oauth/page.go` — `oauthSuccessHTML` / `oauthErrorHTML`, the pages
  every callback response uses.
- `ai/auth/oauth/pkce.go` — `GeneratePKCE()`.
- `ai/auth/oauth/anthropic.go:44` `anthropicCallbackHost` (renamed here),
  `:53` `parseAuthorizationInput` (the colliding name).
- `ai/auth/oauth/token.go:29` `PostJSON` — close to the exchange's needs, but
  it errors on non-2xx and discards the parsed body.
- `ai/auth.go:52-58` `OAuthCredential`, `:180-207` `AuthPrompt` incl.
  `AuthPromptManualCode` and the `Ctx` field that cancels a losing prompt.
- `ai/providers/openrouter.go` and `ai/images/builtin.go:16-23` — the two
  bindings.
- Upstream at `936aff00`: `src/auth/oauth/openrouter.ts` (311),
  `test/openrouter-oauth.test.ts` (322).

## Dependencies

- **Blocked by**: [Epic 6](/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md)
  issue 01 (`AuthInteraction`, `LoginLabel`). Land after epic 6 issue 08 if
  both are in flight — it settles the always-racing manual-code shape this
  flow copies.
- **Blocks**: [Issue 05](/epic-7-four-new-oauth-flows/issues/05-cli-login-new-flows.md),
  [issue 06](/epic-7-four-new-oauth-flows/issues/06-auth-docs-and-porting.md).
- Touches `ai/auth/oauth/anthropic.go` (the `callbackHost` rename) — sequence
  against epic 6 issue 08, which rewrites that file's login body.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
