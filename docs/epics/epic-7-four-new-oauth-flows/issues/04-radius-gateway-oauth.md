---
type: Issue
title: "Port the Radius gateway OAuth flow: discovery, browser PKCE, and device code"
description: "Port src/auth/oauth/radius.ts as ai/auth/oauth/radius.go — a gateway-parameterised strategy factory whose login prompts between a browser PKCE flow on port 1456 and an RFC 8628 device flow, both against the gateway's own token endpoint."
tags: [epic-7]
timestamp: 2026-08-11T18:15:00Z
epic: 7
issue: 04
slug: radius-gateway-oauth
size: L
status: open
gh_issue: 184
resource: https://github.com/kern-ia/kern-link/issues/184
depends_on: [1]
---

# Port the Radius gateway OAuth flow: discovery, browser PKCE, and device code

## Summary

The largest of the four (403 upstream lines) and the only one that is a
**factory**, not a value: `createRadiusOAuth({ name, gateway })` returns a
strategy bound to one gateway, because Radius is a self-hosted pi-messages
gateway and every deployment has its own base URL. Go form:

```go
type RadiusOAuthOptions struct{ Name, Gateway string }
func RadiusOAuth(opts RadiusOAuthOptions) *ai.OAuthAuth
```

`login` prompts the user to choose between two complete login methods and runs
whichever they pick:

- **browser** — discover the authorization endpoint from the gateway, PKCE
  with a `state`, redirect to a fixed local callback on `127.0.0.1:1456`,
  exchange the code at the gateway.
- **device code** — RFC 8628 against the gateway's own device endpoint, no
  discovery, polling the same token endpoint.

Everything except the authorization endpoint lives at fixed paths under the
gateway (`/v1/oauth`, `/v1/oauth/token`, `/v1/oauth/device`); **only** the
interactive browser authorization URL is discovered. Upstream's own test is
named for that fact, and the device path must never call `/v1/oauth` — a
discovery request on the device path is a real regression with a real test.

**The one cross-epic snag: `normalizeRadiusGatewayUrl`.** Upstream imports it
from `src/providers/radius-config.ts`, which is
[Epic 8](/epic-8-pi-messages-and-radius/EPIC_8.md)'s file. Go cannot follow
that import: `ai/providers` imports `ai/auth/oauth` (every binding that
declares OAuth does), so `ai/auth/oauth` importing `ai/providers` back is an
import cycle. Port the normalisation — prepend `https://` when there is no
`http(s)://` scheme, strip trailing slashes — as an **exported**
`NormalizeGatewayURL` in `ai/auth/oauth/radius.go`, and say in its doc comment
that epic 8's `radius-config` port must call this rather than write a second
copy. Note it in the PR body too, so epic 8's implementer meets the decision
before they duplicate it.

Reuse `ai/auth/oauth/callback.go`'s `CallbackServer` for the browser path: it
already does one-shot delivery, `state` validation, and the success/error
pages. Two adaptations:

- Its `RedirectURI` is built from the literal host `localhost`; Radius's
  `redirect_uri` must be `http://127.0.0.1:1456/oauth/callback` exactly, since
  it is sent to the gateway in both the authorize URL and the token exchange
  and must match byte for byte. Build the string from the flow's own
  constants; use `RedirectURI` only if it already matches.
- Its page copy differs slightly from upstream's Radius strings
  (`Signed in to Radius. You may now close this page.`). Reuse the shared
  pages — HTML copy is not classifier-matched text — and record the choice in
  the PR body in one line.

Make the callback port a package var (`radiusCallbackPort = 1456`), the way
`anthropicCallbackPort` is (`ai/auth/oauth/anthropic.go:38`), so tests never
bind a real fixed port. When the listener cannot bind, upstream degrades to a
server that yields no code and the login fails
`OAuth callback did not complete.`; match that observable failure, wrapping
the listen error as the cause.

Errors are shaped by an `OAuthResponseError` carrying `status` and the OAuth
`error` code, because the **device poll switches on that code**
(`authorization_pending`, `slow_down`, `expired_token`, `access_denied`,
default → propagate). Port it as a small unexported error type with a pointer
receiver — `CONVENTIONS.md`'s rule for error types that carry fields — and
match on it with `errors.As`.

`expires` is `now + expires_in*1000 - 60s` — a one-minute skew, unlike the
five-minute one xAI uses and the zero Kimi uses. Three flows, three margins,
all upstream's. The response's `scope` goes into `OAuthCredential.Extra`,
which is exactly what that map is for (`ai/auth.go:48-56`).

## Why this is sized L

~400 upstream lines plus a ported test file, against Go's more verbose
`httptest` scaffolding: roughly 350 lines of flow and 250–300 of tests. The
obvious split — device-code path first, browser path second — was considered
and rejected: both paths share the token-request function, the credential
parser, the error type and the factory, so the second PR would rewrite the
first PR's `login` and touch every file it created. The seam is not where the
line count is. Keep it one PR, and if it passes ~1000 changed lines, split by
**test file** (`radius_test.go` / `radius_browser_test.go`) rather than by
flow half.

## Scope

- `ai/auth/oauth/radius.go` (new) —
  `// Ports: packages/ai/src/auth/oauth/radius.ts`:
  - Constants: callback host `127.0.0.1`, port `1456` (package var), path
    `/oauth/callback`; client id `pi-gateway`; scope `gateway offline_access`;
    device grant type `urn:ietf:params:oauth:grant-type:device_code`; expiry
    skew 60 seconds.
  - `NormalizeGatewayURL(string) string`, exported, per the Summary.
  - `loadDiscovery(ctx, gateway)` — GET `<gateway>/v1/oauth` with
    `accept: application/json`, requiring a string `authorizationEndpoint`.
    Failures: `Could not load Radius OAuth config from <gateway>: <status> <body>`
    and `Invalid Radius OAuth config from <gateway>`.
  - `requestToken(ctx, gateway, form)` — POST `<gateway>/v1/oauth/token`
    (`postForm`, from [issue 01](/epic-7-four-new-oauth-flows/issues/01-xai-device-code-oauth.md)),
    returning an `*ai.OAuthCredential` with the skew applied and `scope`
    carried in `Extra`. Non-2xx becomes the `OAuthResponseError`, message
    `Radius OAuth token request failed: <detail>`, where detail is
    `<error>: <description>`, else `<error>`, else `<description>`, else the
    status.
  - Browser login: `GeneratePKCE()`, a random `state`, the authorize URL with
    `response_type=code`, `client_id`, `redirect_uri`, `scope`,
    `code_challenge`, `code_challenge_method=S256`, `handoff=url`, `state`;
    a `progress` notify naming the redirect URI and an `auth_url` notify with
    instructions `Continue in your browser.`; wait for the callback; exchange
    `grant_type=authorization_code` with the verifier. No code →
    `Login cancelled` when the context is done, otherwise
    `OAuth callback did not complete.` The callback server is always closed.
  - Device login: POST `client_id`+`scope` to `<gateway>/v1/oauth/device`,
    requiring `device_code`, `user_code`, `verification_uri`, `expires_in`
    (`Radius OAuth device authorization response is missing required fields`,
    and `Radius OAuth device authorization failed: <detail>` on non-2xx);
    notify `device_code`; then `PollDeviceCodeFlow` **without**
    `WaitBeforeFirstPoll` (upstream omits it here — it polls immediately,
    unlike xAI and Kimi), mapping the OAuth error codes to pending /
    slow-down / `Device authorization expired.` / `Device authorization was denied.`
    and propagating anything else.
  - `login`: an `ai.AuthPromptSelect` prompt, message `Sign in to <name>:`,
    options `browser` → `Sign in with browser (recommended)` and
    `device-code` → `Sign in with device code (when signing in from another device)`;
    anything else → `Unknown <name> sign-in method: <choice>`.
  - `refresh`: `grant_type=refresh_token` against the same token endpoint. No
    discovery.
  - `toAuth`: `ai.ModelAuth{APIKey: credential.Access}`.
  - `Name` comes from the options; **no** `IsSubscription`, **no**
    `LoginLabel` — upstream sets neither, and issue 06 documents why (a Radius
    gateway credential is not a consumer subscription).
- `ai/auth/oauth/radius_test.go` (new) — the port of
  `test/radius-oauth.test.ts` (129 lines) plus the Go-side coverage the
  criteria below name.

## Out of scope

- **The `radius` provider binding, `radius-config.ts`, and the `pi-messages`
  adapter** — [Epic 8](/epic-8-pi-messages-and-radius/EPIC_8.md). This issue
  ships the flow and nothing that consumes it; there is no `ai/providers/radius.go`
  to attach it to yet, and creating a stub one is not this epic's call.
- The gateway *catalog* (`RadiusGatewayConfig`, `getRadiusModels`, the
  `gatewayConfig` credential field). Epic 8 owns model loading — this flow
  stores only what the token endpoint returns.
- `cmd/pi-ai login` wiring — [issue 05](/epic-7-four-new-oauth-flows/issues/05-cli-login-new-flows.md),
  which registers Radius against the default gateway.
- `docs/auth.md` / `docs/PORTING.md` — [issue 06](/epic-7-four-new-oauth-flows/issues/06-auth-docs-and-porting.md).
- Changing `CallbackServer`'s contract. Reuse it as-is; if it genuinely cannot
  serve this flow, write the reason in the PR body rather than editing shared
  scaffolding three other flows depend on.

## Acceptance criteria / Definition of done

- [ ] `TestRadiusDeviceLoginUsesGatewayEndpointsDirectly` — the device path
      hits `/v1/oauth/device` then `/v1/oauth/token` on an `httptest` gateway
      and **never** requests `/v1/oauth`; assert the recorded request paths,
      since "no discovery" is the claim.
- [ ] `TestRadiusBrowserLoginDiscoversOnlyTheAuthorizationEndpoint` — the
      browser path requests `/v1/oauth` exactly once, and the authorize URL
      built from it carries `client_id=pi-gateway`,
      `scope=gateway offline_access`, `code_challenge_method=S256`,
      `handoff=url`, a `state`, and
      `redirect_uri=http://127.0.0.1:<port>/oauth/callback`.
- [ ] `TestRadiusBrowserLoginExchangesTheCallbackCode` — driving the callback
      URL with the matching `state` yields a credential whose `Expires ==
      stubbedNow + expires_in*1000 - 60000` and whose `Extra["scope"]` is the
      response's scope.
- [ ] `TestRadiusBrowserLoginRejectsAStateMismatch` — a callback with the wrong
      `state` does not complete the login and makes no token request.
- [ ] `TestRadiusBrowserLoginFailsWhenTheCallbackNeverArrives` — closing the
      login without a callback fails with message exactly
      `OAuth callback did not complete.`, and a cancelled context fails
      `Login cancelled` instead.
- [ ] `TestRadiusRefreshesDirectlyThroughTheGatewayWithoutDiscovery` —
      `Refresh` posts `grant_type=refresh_token` to `/v1/oauth/token` and
      requests no other path.
- [ ] `TestRadiusDeviceLoginMapsOAuthErrorCodes` — a poll returning
      `authorization_pending` then `slow_down` then a token succeeds;
      `expired_token` fails `Device authorization expired.`; `access_denied`
      fails `Device authorization was denied.`; an unrecognised error
      propagates the `Radius OAuth token request failed: …` message.
- [ ] `TestRadiusDeviceLoginPollsWithoutWaitingFirst` — the first poll happens
      before any interval elapses (the difference from
      [issue 01](/epic-7-four-new-oauth-flows/issues/01-xai-device-code-oauth.md)'s
      flow), driven through the `deviceCodeSleep` seam.
- [ ] `TestRadiusLoginRejectsAnUnknownSignInMethod` — a `Prompt` returning
      `"carrier-pigeon"` fails with message exactly
      `Unknown Radius sign-in method: carrier-pigeon`.
- [ ] `TestNormalizeGatewayURL` — table-driven (this is a pure
      input→output function, which is where `CONVENTIONS.md` allows tables):
      `radius.pi.dev` → `https://radius.pi.dev`;
      `https://radius.pi.dev///` → `https://radius.pi.dev`;
      `http://localhost:8080/` → `http://localhost:8080` (scheme preserved);
      `HTTPS://Radius.Pi.Dev` keeps its scheme (the match is
      case-insensitive upstream).
- [ ] `TestRadiusMissingDeviceFieldsAreRejected` — a device response without
      `user_code` fails with message exactly
      `Radius OAuth device authorization response is missing required fields`.
- [ ] No test binds port 1456; every test overrides the port var and uses
      `httptest`. Tests are offline, stdlib-only, no `t.Parallel()`, no build
      tags, discrete named functions except the one table above.
- [ ] `// Ports: packages/ai/src/auth/oauth/radius.ts` header on `radius.go`,
      naming `normalizeRadiusGatewayUrl`'s upstream home
      (`src/providers/radius-config.ts`) as a partial second source, and
      `// Ports: packages/ai/test/radius-oauth.test.ts` on `radius_test.go`.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(auth): port the radius gateway oauth flow`.

## Relevant files / areas

- `ai/auth/oauth/callback.go` — `StartCallbackServer(host, port, path,
  expectedState)`, its one-shot `WaitForCode`/`Cancel`/`Close`, and its
  `RedirectURI` built with the literal `localhost`.
- `ai/auth/oauth/devicecode.go` — `PollDeviceCodeFlow`, `DeviceCodePollResult`,
  `deviceCodeSleep`.
- `ai/auth/oauth/token.go` — `postForm` (issue 01), `httpClient`, `clock`.
- `ai/auth/oauth/pkce.go` — `GeneratePKCE()`.
- `ai/auth/oauth/anthropic.go:36-39` — the package-var callback port pattern.
- `ai/auth/oauth/codex.go` — the existing dual browser/device flow with a
  select prompt; the closest structural precedent in the repo.
- `ai/auth.go:48-56` `OAuthCredential.Extra`, `:180-196` `AuthPromptSelect`
  and `AuthPromptOption`.
- Upstream at `936aff00`: `src/auth/oauth/radius.ts` (403),
  `src/providers/radius-config.ts` (`normalizeRadiusGatewayUrl`,
  `DEFAULT_RADIUS_GATEWAY`), `test/radius-oauth.test.ts` (129).

## Dependencies

- **Blocked by**: [Issue 01](/epic-7-four-new-oauth-flows/issues/01-xai-device-code-oauth.md)
  (`postForm`); [epic 6](/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md)
  issue 01 (`AuthInteraction`).
- **Blocks**: [Issue 05](/epic-7-four-new-oauth-flows/issues/05-cli-login-new-flows.md),
  [issue 06](/epic-7-four-new-oauth-flows/issues/06-auth-docs-and-porting.md),
  and [Epic 8](/epic-8-pi-messages-and-radius/EPIC_8.md), whose provider
  binding consumes both `RadiusOAuth` and `NormalizeGatewayURL`.

## PR size note

`L` — ~600–650 changed lines: roughly 350 of flow from 403 upstream lines plus
250–300 of tests over the 129-line `test/radius-oauth.test.ts`, covering
discovery, browser PKCE and device code behind one factory. `L` is the
ceiling, not the target: both login paths share the token request, the
credential parser, the error type and the factory, so a split by login method
would rewrite the first PR's `login` (see "Why this is sized L"); if it trends
past ~1000 the honest cut is by test file — `radius_test.go` and
`radius_browser_test.go`.
