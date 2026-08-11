---
type: Issue
title: "Port the xAI device-code OAuth flow and bind it to the xai provider"
description: "Port src/auth/oauth/xai.ts as ai/auth/oauth/xai.go over the existing device-code poller, add the shared form-POST helper the other three flows reuse, and give the xai binding an OAuth strategy."
tags: [epic-7]
timestamp: 2026-08-11T13:00:00Z
epic: 7
issue: 01
slug: xai-device-code-oauth
size: M
status: open
gh_issue: 181
resource: https://github.com/kern-ia/kern-link/issues/181
depends_on: []
---

# Port the xAI device-code OAuth flow and bind it to the xai provider

## Summary

xAI's flow is the smallest of the four (239 upstream lines) and the one that
reuses the most of what `ai/auth/oauth` already has, so it goes first: it
proves the shape the other three follow, and it lands the one piece of shared
plumbing they all need.

It is a plain RFC 8628 device grant against `https://auth.x.ai`, driven
entirely by the poller already in `ai/auth/oauth/devicecode.go`
(`PollDeviceCodeFlow`, already consumed by the Copilot and Codex flows).
Everything upstream writes around
`pollOAuthDeviceCodeFlow` — the pending / `slow_down` / `expired_token` /
`access_denied` mapping — maps onto `DeviceCodePollResult` one for one.

**The one piece of new shared plumbing: a form-POST helper.** `token.go`'s
`PostJSON` (`ai/auth/oauth/token.go:29`) posts `application/json`; all three
device-code flows in this epic post `application/x-www-form-urlencoded` and
need the *body* of a non-2xx response rather than an error that has swallowed
it, because the OAuth `error` code is what drives the poll state machine.
`PostJSON` errors on non-2xx and bakes the body into a string, so it cannot
serve here. Add `postForm` **next to it in `token.go`**, returning
`(status int, body map[string]any, err error)`, and have it honour the same
30-second `postJSONTimeout` and the same package-level `httpClient` seam that
tests point at an `httptest` server. Issues 02 and 04 reuse it — do not let
each flow grow its own copy.

Three behaviours in upstream's file are easy to lose in translation, and each
has a test upstream:

1. **Non-positive `interval` falls back, it does not fail.** RFC 8628 permits
   `interval: 0`; upstream maps anything non-positive or malformed to
   `undefined` so the poller applies its own default (five seconds — Go:
   `deviceCodeDefaultIntervalSeconds`). Passing `0` straight through to
   `DeviceCodePollOptions.IntervalSeconds` already does the right thing
   (`devicecode.go` treats `<= 0` as "use the default"), but the *parse* must
   not reject it.
2. **The verification URI must be `https:`.** It is handed to the user to open
   in a browser, so upstream validates the scheme on both `verification_uri`
   and `verification_uri_complete` and throws
   `Untrusted verification URI in xAI OAuth response` otherwise. Keep the
   check and its inline reason; this is a security property, not a
   formatting nicety.
3. **A refresh may not rotate the refresh token.** When `refresh_token` is
   absent from a *refresh* response, the previous one is carried forward;
   when it is absent from the *login* response, that is an error. Same
   parser, one optional argument.

`expires` is `now + expires_in*1000 - 5min`, i.e. this flow bakes the
five-minute margin into the stored value, the way Anthropic and Copilot do
and Codex does not (`docs/PORTING.md:43`). Keep it — and note that
[epic 6 issue 04](/epic-6-auth-core-and-env-api-key-bindings/issues/04-oauth-refresh-window.md)
subtracts a second five minutes at resolution time for exactly these flows.
That doubling is upstream's; do not compensate for it here.

## Scope

- `ai/auth/oauth/token.go` — add `postForm(ctx, url string, fields url.Values)
  (int, []byte, map[string]any, error)`: `Accept: application/json`,
  `Content-Type: application/x-www-form-urlencoded`, the existing
  `postJSONTimeout`, the existing `httpClient` var, and the raw response body
  alongside its decoded form on **every** status — the `[: <body>]` error
  suffixes [issue 02](/epic-7-four-new-oauth-flows/issues/02-kimi-coding-device-code-oauth.md)
  pins need the raw bytes, which a decoded-only map cannot recover. A body
  that is not a JSON object decodes to an empty map, matching upstream's
  `parsed && typeof parsed === "object" && !Array.isArray` guard. Comment why
  it exists beside `PostJSON` rather than replacing it.
- `ai/auth/oauth/xai.go` (new) — `// Ports: packages/ai/src/auth/oauth/xai.ts`:
  - Constants, verbatim from upstream: client id
    `b1a00492-073a-47ea-816f-4c329264a828`; scope
    `openid profile email offline_access grok-cli:access api:access`; device
    URL `https://auth.x.ai/oauth2/device/code`; token URL
    `https://auth.x.ai/oauth2/token`; refresh skew 5 minutes; default token
    lifetime 3600 seconds. The two URLs are **package vars, not constants**, so
    tests can point them at an `httptest` server — the pattern
    `ai/auth/oauth/anthropic.go:36-39` already establishes.
  - Device authorization: POST `client_id`, `scope`, `referrer=pi`.
  - Field validation helpers for required strings and positive numbers,
    erroring `Invalid xAI OAuth response field: <field>`.
  - `login`: request the device code, `Notify` an `ai.AuthEvent` of type
    `ai.AuthEventDeviceCode` carrying `verification_uri_complete` when present
    and `verification_uri` otherwise, then `PollDeviceCodeFlow` with
    `WaitBeforeFirstPoll: true`.
  - Poll mapping: 2xx → complete; `authorization_pending` → pending;
    `slow_down` → slow-down (carrying the server's `interval` when numeric);
    `access_denied`/`authorization_denied` → failed
    `xAI device authorization was denied`; `expired_token` → failed
    `xAI device code expired`; anything else → failed with the generic
    request-failure text.
  - Request-failure text, verbatim:
    `xAI OAuth <action> failed (HTTP <status>)[: <error>[: <description>]]`
    with actions `device authorization`, `device token polling`,
    `token refresh`. Invalid JSON is
    `xAI OAuth returned invalid JSON (HTTP <status>)`; a cancelled request is
    `Login cancelled`.
  - `refresh`: `grant_type=refresh_token`, carrying the previous refresh token
    forward when the response omits one.
  - `toAuth`: `ai.ModelAuth{APIKey: credential.Access}`.
  - The exported strategy `XaiOAuth *ai.OAuthAuth` with
    `Name: "xAI (Grok/X subscription)"`, `IsSubscription: true`,
    `LoginLabel: "Sign in with SuperGrok or X Premium"` —
    [epic 6 issue 01](/epic-6-auth-core-and-env-api-key-bindings/issues/01-auth-contract-surface.md)
    already sets `IsSubscription` on the Anthropic, Copilot and Codex flows;
    `LoginLabel` is the field it lands unused, for exactly this.
- `ai/providers/xai.go` — add `OAuth: oauth.XaiOAuth` to the binding's
  `ai.ProviderAuth`, keeping `APIKey` (upstream declares both at `936aff00`).
- `ai/auth/oauth/xai_test.go` (new) — the port of
  `test/xai-oauth.test.ts` (335 lines).

## Out of scope

- `cmd/pi-ai login` wiring — [issue 05](/epic-7-four-new-oauth-flows/issues/05-cli-login-new-flows.md).
- `docs/auth.md` and `docs/PORTING.md` — [issue 06](/epic-7-four-new-oauth-flows/issues/06-auth-docs-and-porting.md).
  This issue writes the `// Ports:` header on `xai.go`; the mapping-table row
  is issue 06's, so four PRs do not fight over one table.
- Refactoring `PostJSON` or its callers. `postForm` is additive.
- `src/auth/oauth/load.ts` and `lazyOAuth`. Bundler-opaque dynamic imports have
  no Go counterpart — `docs/PORTING.md`'s `helpers.ts` row already records
  that Go providers construct `ai.OAuthAuth` directly. Issue 06 dispositions
  `load.ts`.
- Any change to the poller in `devicecode.go`. If a behaviour genuinely cannot
  be expressed through `DeviceCodePollOptions`, stop and say so rather than
  editing shared code from a per-flow PR.

## Acceptance criteria / Definition of done

- [ ] `TestXaiLoginUsesTheDeviceGrantAndDelaysTheFirstPoll` — an `httptest`
      server returns a device code, then `authorization_pending`, then
      `slow_down`, then a token; login succeeds, the first poll happens only
      after one interval (assert via the `deviceCodeSleep` seam, not by
      sleeping), and the `slow_down` response widens the interval.
- [ ] `TestXaiLoginFallsBackToTheDefaultIntervalWhenIntervalIsZero` — a device
      response with `interval: 0` polls on the poller's five-second default
      rather than failing.
- [ ] `TestXaiLoginPrefersVerificationUriComplete` — the `ai.AuthEvent`
      delivered to `Notify` carries `verification_uri_complete` when the
      server sends one, and `verification_uri` when it does not.
- [ ] `TestXaiLoginRejectsNonHTTPSVerificationUri` — a `verification_uri_complete`
      of `http://…` (and separately of `javascript:…`) fails with message
      exactly `Untrusted verification URI in xAI OAuth response`, and no poll
      request is made.
- [ ] `TestXaiLoginCancelledWhileWaitingForTheFirstPoll` — a context cancelled
      during the pre-poll wait returns without hitting the token endpoint.
- [ ] `TestXaiRefreshPreservesAnUnrotatedRefreshToken` — a refresh response
      with no `refresh_token` yields a credential keeping the previous one;
      one *with* a `refresh_token` replaces it.
- [ ] `TestXaiTokenResponseWithoutExpiresInAssumesOneHour` — with `clock`
      stubbed, `Expires == now + 3600*1000 - 300*1000`.
- [ ] `TestXaiRejectsTokenResponsesWithMissingFields` — a response with no
      `access_token` fails with `Invalid xAI OAuth response field: access_token`.
- [ ] `TestXaiRefreshFailureSurfacesTheErrorCodeAndDescription` — a 400 with
      `{"error":"invalid_grant","error_description":"expired"}` fails with
      message exactly
      `xAI OAuth token refresh failed (HTTP 400): invalid_grant: expired`.
- [ ] `TestXaiToAuthUsesTheAccessTokenAsAPIKey` — `ToAuth` returns
      `ModelAuth{APIKey: <access>}`, no headers.
- [ ] `TestXAIProviderDeclaresBothAuthMethods` in `ai/providers` — the binding
      exposes a non-nil `APIKey` **and** a non-nil `OAuth` whose `LoginLabel`
      is `Sign in with SuperGrok or X Premium`.
- [ ] `postForm` has at least one direct test proving it returns both the raw
      response bytes and the decoded body on a 4xx instead of erroring — the
      property the poll state machine and the `[: <body>]` error suffixes
      [issue 02](/epic-7-four-new-oauth-flows/issues/02-kimi-coding-device-code-oauth.md)
      pins both depend on.
- [ ] Every test is offline, stdlib-only (`testing` + `net/http/httptest`), has
      no `t.Parallel()` and no build tag, and is a discrete named function
      (`CONVENTIONS.md`, "Testing").
- [ ] `// Ports: packages/ai/src/auth/oauth/xai.ts` header on `xai.go`, after
      the `package` clause, and `// Ports: packages/ai/test/xai-oauth.test.ts`
      on `xai_test.go`.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g. `feat(auth): port the xAI device-code oauth flow`.

## Relevant files / areas

- `ai/auth/oauth/devicecode.go` — `PollDeviceCodeFlow`,
  `DeviceCodePollOptions`, `DeviceCodePollResult`, and the `deviceCodeSleep`
  var tests fast-forward instead of sleeping.
- `ai/auth/oauth/token.go:22` `httpClient`, `:26` `clock`, `:31` `PostJSON` —
  where `postForm` lands.
- `ai/auth/oauth/copilot.go:233` — the existing `PollDeviceCodeFlow` call site
  to copy the shape from; `:378` — the `ai.AuthEvent` notify shape.
- `ai/auth/oauth/anthropic.go:36-39` — the package-var-for-testability pattern
  for endpoint URLs.
- `ai/auth.go:52-58` `OAuthCredential`, `:200-230` `AuthPrompt`/`AuthEvent`,
  `:264-270` `OAuthAuth`.
- `ai/providers/xai.go` — the binding, currently `APIKey`-only.
- `docs/PORTING.md:43` — the Codex expiry-margin deviation this flow does
  *not* share.
- Upstream at `936aff00`: `src/auth/oauth/xai.ts` (239),
  `test/xai-oauth.test.ts` (335).

## Dependencies

- **Blocked by**: [Epic 6](/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md)
  issue 01 — `AuthLoginCallbacks` → `AuthInteraction`, plus `IsSubscription`
  and `LoginLabel` on `OAuthAuth`. Writing against the old names means a
  rename conflict on landing.
- **Blocks**: [Issue 02](/epic-7-four-new-oauth-flows/issues/02-kimi-coding-device-code-oauth.md)
  and [issue 04](/epic-7-four-new-oauth-flows/issues/04-radius-gateway-oauth.md),
  which both use `postForm`; [issue 05](/epic-7-four-new-oauth-flows/issues/05-cli-login-new-flows.md)
  and [issue 06](/epic-7-four-new-oauth-flows/issues/06-auth-docs-and-porting.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
