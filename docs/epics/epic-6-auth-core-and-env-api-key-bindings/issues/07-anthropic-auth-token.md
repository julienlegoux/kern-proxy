---
type: Issue
title: "Anthropic: resolve ANTHROPIC_AUTH_TOKEN as a bearer header ahead of the API-key envs"
description: "Replace the Anthropic binding's generic env-key strategy with upstream's dedicated resolver, which sends ANTHROPIC_AUTH_TOKEN as Authorization: Bearer instead of as an x-api-key."
tags: [epic-6]
timestamp: 2026-08-11T13:05:00Z
epic: 6
issue: 07
slug: anthropic-auth-token
size: M
status: open
gh_issue: 176
resource: https://github.com/kern-ia/kern-link/issues/176
depends_on: [3]
---

# Anthropic: resolve ANTHROPIC_AUTH_TOKEN as a bearer header ahead of the API-key envs

## Summary

`src/env-api-keys.ts` added a third Anthropic environment variable in range, and
it is not interchangeable with the other two:

```ts
// ANTHROPIC_AUTH_TOKEN participates in env discovery/status, but
// getEnvApiKey() skips it because requests must pass it as Authorization: Bearer.
if (provider === "anthropic") {
	return [ANTHROPIC_AUTH_TOKEN_ENV, ANTHROPIC_OAUTH_TOKEN_ENV, ANTHROPIC_API_KEY_ENV];
}
```

`getEnvApiKey` then explicitly filters it back out:

```ts
const apiKeyEnv = provider === "anthropic" ? envKeys.find((k) => k !== ANTHROPIC_AUTH_TOKEN_ENV) : envKeys[0];
```

So `ANTHROPIC_AUTH_TOKEN` is **first in precedence for resolution** and **absent
from api-key lookup**, because a request carrying it must send
`Authorization: Bearer <token>`, not `x-api-key: <token>`. Upstream implemented
that by dropping `envApiKeyAuth` from the Anthropic binding entirely and writing
a dedicated `anthropicApiKeyAuth()` whose `resolve` returns
`{ auth: { headers: { Authorization: \`Bearer ${authToken}\` } }, source: ANTHROPIC_AUTH_TOKEN_ENV }`
— an `AuthResult` with **no `apiKey` at all**.

kern-link's binding is `auth.EnvAPIKeyAuth("Anthropic API key",
["ANTHROPIC_OAUTH_TOKEN", "ANTHROPIC_API_KEY"])`
(`ai/providers/anthropic.go:26`), so it gets the same treatment: a
binding-local resolver, not a widening of the shared helper. Widening
`EnvAPIKeyAuth` with an "except this one variable is a header" special case is
the wrong shape — upstream took the token *out* of the shared helper for exactly
that reason.

The path already works end to end on the adapter side:
`ai/apis/anthropic/anthropic.go:1215` only sets `x-api-key` when an api key is
present, and `:331` already recognises a caller-supplied `authorization` header.
Verify that rather than assuming it — the acceptance criteria pin it.

`docs/PORTING.md:59` describes `src/env-api-keys.ts` as ported "distributed:
each binding declares its own env-var precedence list". This issue keeps that
distribution and is the reason it stays defensible; the row itself is
[issue 11](/epic-6-auth-core-and-env-api-key-bindings/issues/11-porting-paths-and-dispositions.md)'s.

## Scope

- `ai/providers/anthropic.go` — replace the `auth.EnvAPIKeyAuth(...)` call with
  a package-local `anthropicAPIKeyAuth() *ai.APIKeyAuth` whose `Resolve`, in
  order:
  1. a stored credential's `Key` → `{Auth: {APIKey: key}, Env: credential.Env,
     Source: "stored credential"}`. **Note `Env`** — upstream added
     `env: credential.env` to the stored-credential branch in this same commit
     (`auth/helpers.ts`), and the same fix belongs in
     `ai/auth/helpers.go`'s `EnvAPIKeyAuth`; do that here too, in one line, so
     the two resolvers do not disagree.
  2. `ANTHROPIC_AUTH_TOKEN` → `{Auth: {Headers: {"Authorization": "Bearer " +
     token}}, Source: "ANTHROPIC_AUTH_TOKEN"}`, **no `APIKey`**.
  3. `ANTHROPIC_OAUTH_TOKEN`, then `ANTHROPIC_API_KEY` → `{Auth: {APIKey:
     value}, Source: <var name>}`.
  4. otherwise `(nil, nil)`.
  `Login` keeps prompting for a secret with the message `Enter Anthropic API
  key`, matching both the current behavior and upstream's.
- Name the three variables as exported constants if anything else needs them;
  otherwise keep them unexported in the binding. Upstream exports them from
  `env-api-keys.ts` because its tests import them — Go's tests live in the same
  package tree and do not need that.
- `ai/auth/helpers.go` — the one-line `Env: input.Credential.Env` addition to
  `EnvAPIKeyAuth`'s stored-credential branch (`:31-36`).

## Out of scope

- The other `env-api-keys.ts` map additions in range: `qwen-token-plan` ×3 and
  `baseten` belong to
  [issue 10](/epic-6-auth-core-and-env-api-key-bindings/issues/10-env-api-key-bindings.md);
  `radius: "RADIUS_API_KEY"` belongs to
  [Epic 8](/epic-8-pi-messages-and-radius/EPIC_8.md) with the radius binding.
  Note the radius one in the PR body so it is not lost.
- The Anthropic **adapter** (`ai/apis/anthropic`). This issue asserts its
  existing behavior; it does not change it. Adapter work is
  [Epic 5](/epic-5-remaining-adapters/EPIC_5.md).
- The Anthropic **OAuth flow** — `IsSubscription` is set in
  [issue 01](/epic-6-auth-core-and-env-api-key-bindings/issues/01-auth-contract-surface.md),
  the login race in
  [issue 08](/epic-6-auth-core-and-env-api-key-bindings/issues/08-anthropic-codex-login-race.md).
- Upstream's `getEnvApiKey`/`findEnvKeys` as *functions*. kern-link has no
  central env-key registry — that is the recorded distribution deviation. Do
  not create one to host this precedence list.

## Acceptance criteria / Definition of done

- [ ] `TestAnthropicAuthTokenResolvesAsBearerHeader` — with only
      `ANTHROPIC_AUTH_TOKEN=t` set, the `AuthResult` has
      `Auth.APIKey == ""`, `Auth.Headers["Authorization"] == "Bearer t"` and
      `Source == "ANTHROPIC_AUTH_TOKEN"`.
- [ ] `TestAnthropicAuthTokenOutranksOAuthAndAPIKey` — all three set: the bearer
      header wins and no `x-api-key` value is resolved.
- [ ] `TestAnthropicOAuthTokenOutranksAPIKey` — `ANTHROPIC_AUTH_TOKEN` unset,
      the other two set: `Auth.APIKey` is the OAuth token,
      `Source == "ANTHROPIC_OAUTH_TOKEN"`. (Today's behavior; this test proves
      the rewrite did not regress it.)
- [ ] `TestAnthropicStoredCredentialWinsAndCarriesEnv` — a stored
      `APIKeyCredential{Key: "k", Env: {"X": "y"}}` resolves to that key,
      `Source == "stored credential"`, and `AuthResult.Env["X"] == "y"`.
- [ ] `TestEnvAPIKeyAuthReturnsStoredCredentialEnv` — the same `Env`
      propagation, in `ai/auth/helpers_test.go`, for the shared helper.
- [ ] `TestAnthropicRequestSendsBearerNotApiKey` — an offline `httptest` request
      through the Anthropic adapter with the bearer-header auth applied carries
      `Authorization: Bearer t` and **no** `x-api-key` header. This is the
      end-to-end claim; without it the resolver could be right and the request
      still wrong.
- [ ] Every case in upstream's `test/anthropic-auth-token.test.ts` (new, 187
      lines) is represented by a Go test in this PR or explicitly dispositioned
      in the PR body.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] `ai/providers/anthropic.go` keeps its `// Ports:` header, updated: it
      currently says the api-key strategy is `envApiKeyAuth` with
      `ANTHROPIC_OAUTH_TOKEN` precedence, which stops being true.
- [ ] Conventional Commit, e.g.
      `feat(providers): send ANTHROPIC_AUTH_TOKEN as a bearer token`.

## Relevant files / areas

- `ai/providers/anthropic.go:1-31` — the whole binding, including the
  `// Ports:` header that describes the current precedence.
- `ai/auth/helpers.go:30-43` — `EnvAPIKeyAuth`'s `Resolve`.
- `ai/apis/anthropic/anthropic.go:1193` `buildHeaders`, `:1215` the `x-api-key`
  set, `:331` the existing `authorization`/`x-api-key` detection.
- `ai/auth.go` — `AuthResult`, `ModelAuth.Headers`.
- Upstream: `src/providers/anthropic.ts` (+53/-6) and `src/env-api-keys.ts`
  (+17/-5) at `936aff00`.
- Upstream tests: `test/anthropic-auth-token.test.ts` (new, 187 lines),
  `test/env-api-keys.test.ts` (+56).

## Dependencies

- **Blocked by**: [Issue 03](/epic-6-auth-core-and-env-api-key-bindings/issues/03-provider-scoped-apikey-resolution.md)
  — it changes `APIKeyResolveInput`, which this resolver is written against.
- **Blocks**: Nothing.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
