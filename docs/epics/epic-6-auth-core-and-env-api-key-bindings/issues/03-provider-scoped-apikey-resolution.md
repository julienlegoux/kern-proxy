---
type: Issue
title: "Provider-scope api-key resolution: drop Model from APIKeyResolveInput and rebuild the Cloudflare resolvers"
description: "Follow upstream in making auth resolution provider-scoped rather than model-scoped, and move Cloudflare's account/gateway placeholder substitution off the auth path it can no longer reach."
tags: [epic-6]
timestamp: 2026-08-11T18:15:00Z
epic: 6
issue: 03
slug: provider-scoped-apikey-resolution
size: M
status: open
gh_issue: 172
resource: https://github.com/kern-ia/kern-link/issues/172
depends_on: [1]
---

# Provider-scope api-key resolution: drop Model from APIKeyResolveInput and rebuild the Cloudflare resolvers

## Summary

Upstream removed the model from auth resolution entirely. `resolve.ts` deleted
its `AuthModel` type and dropped the `model` parameter from
`resolveProviderAuth`; `types.ts` rewrote the `ApiKeyAuth.resolve` doc comment
to say why:

> Resolution is provider-scoped; model-specific endpoint preparation happens
> after auth has been resolved.

That is a contract change with exactly one load-bearing consequence in this
port, and it is not optional: **`ai/providers/cloudflare_auth.go` resolves
`{CLOUDFLARE_ACCOUNT_ID}` / `{CLOUDFLARE_GATEWAY_ID}` out of `model.BaseURL` at
auth time** (`:24` `resolveCloudflareBaseURL`, called from both binding
resolvers at `:88` and `:109`). Without the model it cannot. Upstream made the
same removal and re-homed the substitution in
`src/providers/cloudflare-stream.ts`, at dispatch time.

**Read this before starting.**
[Epic 5 issue 11](/epic-5-remaining-adapters/issues/11-cloudflare-stream-classification.md)
(`#169`) owns `cloudflare-stream.ts` and recommends porting the dispatch-time
resolution while *keeping* the auth-time one. This issue removes the auth-time
one — they cannot both be true. The resolution:

- If **#169 landed first and ported dispatch-time resolution**, this issue just
  deletes the auth-time path and its `BaseURL` assertions, and updates #169's
  claim that auth-time resolution is preserved.
- If **#169 has not landed, or dispositioned `cloudflare-stream.ts` as a
  deviation**, this issue must carry the dispatch-time substitution itself —
  otherwise Cloudflare requests go out with literal `{CLOUDFLARE_ACCOUNT_ID}` in
  the URL. That is a regression, not a deviation, and it is the reason this
  issue is sized M rather than S.

Check `docs/PORTING.md` and `git log` for #169 before writing code, and say in
the PR body which branch you took.

Upstream's `cloudflare-auth.ts` changed in the same commit in a second way this
issue owns, because the same function is being rewritten: **the per-field
merge**. Today a non-nil credential short-circuits ambient lookup entirely
(`resolveValue` returns `credential.env?.[name]`, `undefined` included).
Upstream now falls through to `ctx.env(name)` when the credential does not carry
the field, with the reason in a comment: *"A credential carrying only the API
key must still pick up the account / gateway id from the environment."*
[Epic 5 issue 11](/epic-5-remaining-adapters/issues/11-cloudflare-stream-classification.md)
explicitly declined this change and asked for it to be flagged rather than lost;
this is where it lands.

## Scope

- `ai/auth.go` — remove `Model` from `APIKeyResolveInput`, leaving `Ctx` and
  `Credential`. Update the `APIKeyAuth.Resolve` doc comment to upstream's new
  sentence about provider-scoped resolution.
- `ai/resolve.go` — drop the `model *Model` parameter from
  `ResolveProviderAuth` (`:67-75`) and from `resolveAPIKey` (`:188-204`). The
  error text becomes `fmt.Sprintf("API key auth failed for provider %s",
  providerID)` using the id the function already receives — **the string stays
  byte-identical**, which is why this is safe: today it reads the same text off
  `model.Provider`, and `ai/retry.go`/`ai/overflow.go` classify on error text
  (`CONVENTIONS.md`, ST1005).
- `ai/provider.go` — `applyAuth`'s call site, and any other caller of
  `ResolveProviderAuth`.
- `ai/providers/cloudflare_auth.go` — rewrite `resolveCloudflareResolvedEnv`
  (`:42`) without the model:
  - per-field merge: credential value when present, otherwise `ctx.Env(name)`,
    per field, for `CLOUDFLARE_API_KEY`, `CLOUDFLARE_ACCOUNT_ID` and (gateway
    binding only) `CLOUDFLARE_GATEWAY_ID`;
  - `AuthResult.Auth.BaseURL` is no longer set by these resolvers;
  - `Source` keeps its current two values (`"stored credential"` when a
    credential supplied the key, else `CLOUDFLARE_API_KEY`) — upstream did not
    change that line.
  - keep `resolveCloudflareBaseURL` itself if the dispatch-time path uses it;
    delete it only if nothing calls it.
- Every other `Resolve` implementation is a mechanical signature edit: 13
  `Resolve: func` sites across `ai/auth/helpers.go`, `ai/providers/*.go`
  (`amazon_bedrock.go:17`, `google_vertex.go:21`, `faux/faux.go:277`) and
  the test files under `ai/`, `ai/images/`.

## Out of scope

- `resolveProviderAuth`'s OAuth-validity changes (five-minute window, refresh
  timeout, `minOAuthValidityMs`) — same file, but
  [issue 04](/epic-6-auth-core-and-env-api-key-bindings/issues/04-oauth-refresh-window.md).
  Land this first; 04 rebases onto it.
- `Models.GetAuth`'s new provider-id overload and its static-header merge —
  [issue 06](/epic-6-auth-core-and-env-api-key-bindings/issues/06-models-login-logout.md).
  Leave `Models.GetAuth(ctx, model)` as-is here.
- `ApiKeyAuth.check` — declared in
  [issue 01](/epic-6-auth-core-and-env-api-key-bindings/issues/01-auth-contract-surface.md),
  consumed in [issue 05](/epic-6-auth-core-and-env-api-key-bindings/issues/05-models-availability.md).
- The Anthropic binding's own resolver rewrite —
  [issue 07](/epic-6-auth-core-and-env-api-key-bindings/issues/07-anthropic-auth-token.md).
  Give it the mechanical signature edit only.

## Acceptance criteria / Definition of done

- [ ] `grep -rn "APIKeyResolveInput" --include=*.go . | grep -c "Model"`
      returns 0.
- [ ] `TestCloudflareCredentialFallsBackToEnvPerField` — a stored
      `APIKeyCredential{Key: "k"}` with **no** `Env`, plus an `AuthContext`
      supplying `CLOUDFLARE_ACCOUNT_ID` and `CLOUDFLARE_GATEWAY_ID`, resolves
      successfully with both ids in `AuthResult.Env`. Today this returns nil.
- [ ] `TestCloudflareCredentialFieldWinsOverEnv` — a credential carrying
      `Env{CLOUDFLARE_ACCOUNT_ID: "cred"}` beats an ambient
      `CLOUDFLARE_ACCOUNT_ID=env`, `got`/`want` on the resolved value.
- [ ] `TestCloudflareGatewayRequiresGatewayID` — the ai-gateway binding still
      resolves to nil when the gateway id is missing from both sources, and the
      workers-ai binding still ignores it.
- [ ] A Cloudflare request dispatched end to end reaches the adapter with a
      substituted base URL — either through #169's dispatch-time path (assert
      it and link the test) or through one added here.
      `TestCloudflareRequestURLHasNoPlaceholders` must exist somewhere in the
      tree and be named in the PR body.
- [ ] `TestResolveAPIKeyErrorNamesProvider` — a failing resolver produces a
      `*ModelsError` with code `auth` and message exactly
      `API key auth failed for provider <id>`, asserted as a string.
- [ ] `docs/PORTING.md`'s `src/api/cloudflare.ts` row (`:45`) no longer claims
      the templates are "resolved at runtime by `resolveCloudflareBaseURL`" if
      that stopped being true.
- [ ] `ai/auth.go`, `ai/resolve.go`, `ai/provider.go` and
      `ai/providers/cloudflare_auth.go` keep their `// Ports:` headers, still
      describing what each file ports after this change — `ai/resolve.go`'s in
      particular, since this issue changes what it ports.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `refactor(ai)!: resolve api-key auth per provider rather than per model`.

## Relevant files / areas

- `ai/auth.go:240-258` — `APIKeyResolveInput`, `APIKeyAuth`.
- `ai/resolve.go:58-118` `ResolveProviderAuth`, `:188-204` `resolveAPIKey`.
- `ai/provider.go` — `applyAuth`, the production call site.
- `ai/providers/cloudflare_auth.go:24` `resolveCloudflareBaseURL`, `:42`
  `resolveCloudflareResolvedEnv`, `:84`/`:105` the two bindings.
- `ai/providers/amazon_bedrock.go:17`, `ai/providers/google_vertex.go:21`,
  `ai/providers/faux/faux.go:277` — the other real resolvers.
- `ai/images/provider_test.go`, `ai/provider_test.go`, `ai/auth/helpers_test.go`
  — the test-side `Resolve` literals.
- Upstream: `src/auth/resolve.ts`, `src/auth/types.ts`,
  `src/providers/cloudflare-auth.ts` (+68/-35), `src/providers/cloudflare-stream.ts`
  (new), all at `936aff00`.
- Sibling issue: [Epic 5 issue 11](/epic-5-remaining-adapters/issues/11-cloudflare-stream-classification.md)
  (`#169`).

## Dependencies

- **Blocked by**: [Issue 01](/epic-6-auth-core-and-env-api-key-bindings/issues/01-auth-contract-surface.md)
  (both rewrite the `APIKeyAuth` declaration).
- **Blocks**: [Issue 04](/epic-6-auth-core-and-env-api-key-bindings/issues/04-oauth-refresh-window.md),
  [05](/epic-6-auth-core-and-env-api-key-bindings/issues/05-models-availability.md),
  [06](/epic-6-auth-core-and-env-api-key-bindings/issues/06-models-login-logout.md),
  [07](/epic-6-auth-core-and-env-api-key-bindings/issues/07-anthropic-auth-token.md),
  [11](/epic-6-auth-core-and-env-api-key-bindings/issues/11-porting-paths-and-dispositions.md)
  (which amends `docs/PORTING.md` after this issue's edit).
- **Coordinate with**: Epic 5's `#169`, per the Summary.

## PR size note

`M` — ~400 changed lines: one field removed from `APIKeyResolveInput` and two
signatures changed in `ai/resolve.go` cascade into 13 `Resolve: func` sites
across `ai/auth/helpers.go`, `ai/providers/*.go` and the `ai/`/`ai/images/`
tests, on top of the per-field rewrite of `resolveCloudflareResolvedEnv` and
five named tests. It only reaches the top of the band if `#169` has not landed
and this PR must carry the dispatch-time placeholder substitution. Split past
~500, and that substitution is the seam — it belongs to `cloudflare-stream.ts`.
