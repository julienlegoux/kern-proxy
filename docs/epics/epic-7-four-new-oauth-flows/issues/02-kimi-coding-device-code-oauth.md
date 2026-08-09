---
type: Issue
title: "Port the Kimi Code device-code OAuth flow with its retrying refresh, and bind it to kimi-coding"
description: "Port src/auth/oauth/kimi-coding.ts as ai/auth/oauth/kimicoding.go — device authorization against a host-overridable endpoint, a bearer-header ToAuth, and a refresh that retries 429/5xx with exponential backoff."
tags: [epic-7]
timestamp: 2026-08-09T13:40:00Z
epic: 7
issue: 02
slug: kimi-coding-device-code-oauth
size: M
status: open
gh_issue: 182
resource: https://github.com/kern-ia/kern-link/issues/182
depends_on: [1]
---

# Port the Kimi Code device-code OAuth flow with its retrying refresh, and bind it to kimi-coding

## Summary

The second device-code flow (310 upstream lines). Structurally it is
[issue 01](/epic-7-four-new-oauth-flows/issues/01-xai-device-code-oauth.md)'s
twin — RFC 8628 against `https://auth.kimi.com`, driven by
`PollDeviceCodeFlow` — but it differs in three ways that each carry their own
test, and those differences are the whole issue:

1. **The OAuth host is overridable by environment.** Upstream reads
   `KIMI_CODE_OAUTH_HOST`, falling back to `KIMI_OAUTH_HOST`, falling back to
   `https://auth.kimi.com`, and strips trailing slashes. It is read *per call*
   (login and refresh both call `getOauthHost()`), not captured at
   construction — keep that, it is what makes the override testable and what
   lets a user point at a staging gateway without restarting. Go's equivalent
   of `getProviderEnvValue` here is plain `os.Getenv`, the same call
   `ai/auth/oauth/anthropic.go:45` makes for `PI_OAUTH_CALLBACK_HOST`.
2. **`ToAuth` returns a header, not an API key.** `{ headers: { Authorization:
   "Bearer <access>" } }` — the access token authenticates against
   `https://api.kimi.com/coding`, which the `kimi-coding` binding already
   points at over the `anthropic-messages` adapter. This is the only one of
   the four flows that does not resolve to `ModelAuth.APIKey`; getting it
   wrong produces a login that succeeds and a request that 401s.
3. **Refresh retries.** Up to three retries (four attempts total) with
   exponential backoff `1s, 2s, 4s`, retrying on 429 and 5xx **and** on
   transport errors, but failing immediately and unretryably on 401, 403, or
   `error: "invalid_grant"` — the case where the stored credential is dead and
   the user must log in again. The distinction is load-bearing: a retried
   `invalid_grant` would hold the credential-store lock through three sleeps
   for a credential that will never come back, and
   [epic 6 issue 04](/epic-6-auth-core-and-env-api-key-bindings/issues/04-oauth-refresh-window.md)
   caps the whole refresh at fifteen seconds.

That backoff needs a seam, or the test suite sleeps for seven seconds.
`ai/auth/oauth/devicecode.go` already has `deviceCodeSleep` as a package-level
var for exactly this reason; reuse it rather than adding a second sleep var
with the same job, and say so in a comment where the retry loop calls it.

`expires` is `now + expires_in*1000`, with **no** safety margin — Kimi is in
the Codex category here, not the Anthropic one. Do not add a margin to "make
it consistent"; `docs/PORTING.md:43` records that this difference is
upstream's and matching it is the point.

## Scope

- `ai/auth/oauth/kimicoding.go` (new) —
  `// Ports: packages/ai/src/auth/oauth/kimi-coding.ts`:
  - Constants: client id `17e5f671-d194-4dfb-9706-5516cb48c098`; default host
    `https://auth.kimi.com`; device-code timeout 15 minutes; default poll
    interval 5 seconds; request timeout 30 seconds; max refresh retries 3.
  - `kimiOAuthHost()` — `KIMI_CODE_OAUTH_HOST` then `KIMI_OAUTH_HOST` then the
    default, with trailing slashes trimmed. Read per call.
  - Device authorization: POST `client_id` to
    `<host>/api/oauth/device_authorization`. All four of `device_code`,
    `user_code`, `verification_uri`, `verification_uri_complete` are required
    **and both URIs must parse as http(s)**; otherwise
    `Invalid Kimi Code device authorization response: <json>`. Non-2xx is
    `Kimi Code device authorization failed with status <status>[: <body>]`.
  - Non-positive or malformed `interval` / `expires_in` fall back to the
    defaults rather than failing (same rule as issue 01).
  - `login`: notify `ai.AuthEventDeviceCode` with
    `verification_uri_complete` as the verification URI — Kimi always sends it
    and upstream always uses it, unlike xAI's fallback — then
    `PollDeviceCodeFlow` with `WaitBeforeFirstPoll: true` against
    `<host>/api/oauth/token`.
  - Poll mapping: `>= 500` → failed
    `Kimi Code device token request failed with status <status>[: <body>]`;
    2xx with an `access_token` → complete; `authorization_pending` → pending;
    `slow_down` → slow-down with the server's interval when positive;
    `expired_token` → failed
    `Kimi Code device authorization expired. Please restart login.`;
    `access_denied` → failed `Kimi Code login was denied.`; otherwise failed
    `Kimi Code device token request failed (status <status>)[: <error>[: <description>]]`.
  - Token parsing shared by poll and refresh, requiring non-empty
    `access_token`, non-empty `refresh_token` and a positive `expires_in`,
    erroring `Kimi Code token <operation> response missing fields: <json>`
    with operation `poll` or `refresh`.
  - `refresh`: `grant_type=refresh_token` with the retry rules above;
    unauthorized is
    `Kimi Code token refresh unauthorized (status <status>)[: <description>]`,
    an aborted context is `Kimi Code token refresh aborted`, exhausted retries
    surface the last error.
  - The exported strategy `KimiCodingOAuth *ai.OAuthAuth` with
    `Name: "Kimi Code (subscription)"`, `IsSubscription: true`,
    `LoginLabel: "Sign in with Kimi Code"`.
- `ai/providers/kimi_coding.go` — add `OAuth: oauth.KimiCodingOAuth` alongside
  the existing `APIKey`.
- `ai/auth/oauth/kimicoding_test.go` (new) — the port of
  `test/kimi-coding-oauth.test.ts` (270 lines).

File name is `kimicoding.go`, not `kimi-coding.go` or `kimi_coding.go`:
`CONVENTIONS.md` reserves underscores for `ai/providers` bindings named after
a provider id, and this is not one.

## Out of scope

- `cmd/pi-ai login` wiring — [issue 05](/epic-7-four-new-oauth-flows/issues/05-cli-login-new-flows.md).
- `docs/auth.md` / `docs/PORTING.md` — [issue 06](/epic-7-four-new-oauth-flows/issues/06-auth-docs-and-porting.md).
- The `moonshotai` / `moonshotai-cn` bindings. They are separate providers with
  their own API keys; upstream attaches this flow to `kimi-coding` only.
- Any change to `ai/apis/anthropic` for the bearer header. `ToAuth` returning
  `ModelAuth.Headers` is already how Copilot works
  (`ai/auth/oauth/copilot.go`); the adapter needs nothing.
- Widening `postForm` (from [issue 01](/epic-7-four-new-oauth-flows/issues/01-xai-device-code-oauth.md)).
  If Kimi needs the raw response *text* for its `[: <body>]` error suffixes and
  `postForm` only returns a decoded map, extend it there in a small, separate
  commit and say so — do not fork a second helper.

## Acceptance criteria / Definition of done

- [ ] `TestKimiCodingLoginCompletesTheDeviceFlow` — device authorization, one
      `authorization_pending`, then a token; the credential carries the access
      and refresh tokens and `Expires == stubbedNow + expires_in*1000` with
      **no** margin subtracted.
- [ ] `TestKimiCodingLoginFailsWhenTheDeviceCodeExpires` — an `expired_token`
      poll fails with message exactly
      `Kimi Code device authorization expired. Please restart login.`
- [ ] `TestKimiCodingLoginFailsWhenTheUserDenies` — `access_denied` fails with
      message exactly `Kimi Code login was denied.`
- [ ] `TestKimiCodingHonorsTheOAuthHostOverride` — with `KIMI_CODE_OAUTH_HOST`
      set to an `httptest` URL (with a trailing slash, to pin the trimming),
      both the device-authorization and token requests hit that server;
      `KIMI_OAUTH_HOST` alone does the same, and `KIMI_CODE_OAUTH_HOST` wins
      when both are set. Use `t.Setenv`.
- [ ] `TestKimiCodingRejectsANonHTTPVerificationUri` — a `verification_uri_complete`
      that is not http(s) fails with an
      `Invalid Kimi Code device authorization response` error.
- [ ] `TestKimiCodingToAuthReturnsABearerHeader` — `ToAuth` returns
      `ModelAuth{Headers: {"Authorization": "Bearer <access>"}}` and an empty
      `APIKey`. Assert the empty `APIKey` explicitly; that is the regression
      this test exists to catch.
- [ ] `TestKimiCodingRefreshRetriesOn429ThenSucceeds` — a 429 followed by a
      200 yields the refreshed credential in exactly two requests, with the
      backoff driven through the `deviceCodeSleep` seam (the test must not
      sleep).
- [ ] `TestKimiCodingRefreshFailsImmediatelyOnInvalidGrant` — a 400 carrying
      `{"error":"invalid_grant"}` makes **exactly one** request and fails with
      a `Kimi Code token refresh unauthorized (status 400)` message; assert the
      request count, since "does not retry" is the claim.
- [ ] `TestKimiCodingRefreshGivesUpAfterThreeRetries` — persistent 500s make
      exactly four requests and surface the last error.
- [ ] `TestKimiCodingParsesTokenResponseFields` — a response missing
      `refresh_token` fails with a
      `Kimi Code token <operation> response missing fields` message naming the
      right operation for both the poll and the refresh path.
- [ ] `TestKimiCodingProviderDeclaresBothAuthMethods` in `ai/providers` —
      non-nil `APIKey` and non-nil `OAuth` with `IsSubscription` true.
- [ ] Tests are offline, stdlib-only, no `t.Parallel()`, no build tags,
      discrete named functions.
- [ ] `// Ports: packages/ai/src/auth/oauth/kimi-coding.ts` header on
      `kimicoding.go`.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(auth): port the kimi code device-code oauth flow`.

## Relevant files / areas

- `ai/auth/oauth/devicecode.go` — `PollDeviceCodeFlow` and the
  `deviceCodeSleep` seam the refresh backoff reuses.
- `ai/auth/oauth/token.go` — `postForm` (issue 01), `httpClient`, `clock`.
- `ai/auth/oauth/copilot.go` — the existing flow whose `ToAuth` also returns
  headers rather than an API key, and whose device-code call site is the
  closest model for this one.
- `ai/auth/oauth/anthropic.go:41-49` — the `os.Getenv` env-override pattern.
- `ai/providers/kimi_coding.go` — the binding (`https://api.kimi.com/coding`,
  `anthropic-messages` adapter), currently `APIKey`-only.
- `docs/PORTING.md:43` — the expiry-margin note this flow falls on the Codex
  side of.
- Upstream at `936aff00`: `src/auth/oauth/kimi-coding.ts` (310),
  `test/kimi-coding-oauth.test.ts` (270).

## Dependencies

- **Blocked by**: [Issue 01](/epic-7-four-new-oauth-flows/issues/01-xai-device-code-oauth.md)
  (`postForm`); [epic 6](/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md)
  issue 01 (`AuthInteraction`, `IsSubscription`, `LoginLabel`).
- **Blocks**: [Issue 05](/epic-7-four-new-oauth-flows/issues/05-cli-login-new-flows.md),
  [issue 06](/epic-7-four-new-oauth-flows/issues/06-auth-docs-and-porting.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
