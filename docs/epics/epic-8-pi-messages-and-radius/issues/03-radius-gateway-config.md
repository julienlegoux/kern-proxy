---
type: Issue
title: "Port radius-config: the gateway catalog types, their sanitizer, and the gateway-config fetch"
description: "Port src/providers/radius-config.ts as ai/providers/radius_config.go — the RadiusGatewayConfig shape, a sanitizer that drops incomplete models rather than zero-filling them, credential-carried catalogs, and the network fetch of /v1/config with its failure paths."
tags: [epic-8]
timestamp: 2026-08-11T18:15:00Z
epic: 8
issue: 03
slug: radius-gateway-config
size: M
status: open
gh_issue: 189
resource: https://github.com/kern-ia/kern-link/issues/189
depends_on: [1]
---

# Port radius-config: the gateway catalog types, their sanitizer, and the gateway-config fetch

## Summary

Radius has no static catalog: its models come from the gateway, either
persisted on the stored OAuth credential or fetched live from `/v1/config`.
This issue lands that machinery on its own, ahead of the binding, because **the
network call at provider setup is the risk this epic exists to own**
([Epic 8](/epic-8-pi-messages-and-radius/EPIC_8.md), Notes) and it is far
easier to design when it is not tangled with the refresh contract.

**The Go trap here is `encoding/json`'s silence.** Upstream's
`isRadiusGatewayModel` is a runtime `typeof` guard that *drops* a model missing
`contextWindow`, and `sanitizeRadiusGatewayConfig` returns `undefined` for a
config whose `baseUrl` is not a string. Decoding straight into `ai.Model`-shaped
structs does the opposite: a missing `contextWindow` becomes `0`, a missing
`baseUrl` becomes `""`, and a broken gateway response becomes a catalog of
models that silently price at zero and truncate at zero tokens. Decode into an
intermediate whose required fields are pointers/optional (`*string`, `*bool`,
`*int`, `*ai.ModelCost`, `[]ai.Modality`), require each to be present, and drop
the entries that are not — that is the port, not the type declaration.

**`new URL("/v1/config", gateway)` is an absolute-path join.** It replaces any
path on the gateway URL: `https://host/base` yields `https://host/v1/config`,
not `https://host/base/v1/config`. `gateway + "/v1/config"` is therefore a
behavior change. Parse the gateway with `net/url` and set `Path = "/v1/config"`
(clearing `RawQuery`) so the two agree, and test it.

The gateway URL that arrives here is already normalized —
`oauth.NormalizeGatewayURL`, ported in
[epic 7 issue 04](/epic-7-four-new-oauth-flows/issues/04-radius-gateway-oauth.md)
because `ai/auth/oauth` cannot import `ai/providers` back without a cycle. Its
doc comment says this file must call it rather than write a second copy;
`ai/providers` importing `ai/auth/oauth` is the direction every OAuth-bearing
binding already goes.

`DEFAULT_RADIUS_GATEWAY` (`https://radius.pi.dev`) is upstream's constant in
this file. Check whether
[epic 7 issue 05](/epic-7-four-new-oauth-flows/issues/05-cli-login-new-flows.md)
already landed one in `ai/auth/oauth` to register Radius against — if it did,
reuse it; if it did not, declare it here and say so in the PR body. What must
not happen is two constants naming the same host.

## Scope

- `ai/providers/radius_config.go` (new) —
  `// Ports: packages/ai/src/providers/radius-config.ts`. A second file for one
  binding is a deliberate departure from `ai/providers`' one-file-per-binding
  rule (`docs/planning/CONVENTIONS.md`, "Naming & file layout"); it mirrors
  upstream's own split and keeps the binding in
  [issue 04](/epic-8-pi-messages-and-radius/issues/04-radius-provider-binding.md)
  readable. Note the reason in the file's doc comment.
  - `DefaultRadiusGateway` (unless epic 7 already owns it, per the Summary).
  - `RadiusGatewayModel` (id, name, reasoning, thinkingLevelMap, input, cost,
    contextWindow, maxTokens) and `RadiusGatewayConfig{BaseURL, Models}`.
  - `sanitizeRadiusGatewayConfig(raw []byte or any) (*RadiusGatewayConfig, bool)`
    implementing upstream's guards: a non-object, a missing/non-string
    `baseUrl`, or a non-array `models` yields "not a config"; individual models
    failing the field checks are dropped, leaving the rest.
  - `getRadiusCredentialConfig(*ai.OAuthCredential) *RadiusGatewayConfig` —
    reads `gatewayConfig` out of `OAuthCredential.Extra` (`ai/auth.go:52-56`,
    where `UnmarshalJSON` parks every unrecognized field) and runs it through
    the same sanitizer. This is the credential field
    [epic 7 issue 04](/epic-7-four-new-oauth-flows/issues/04-radius-gateway-oauth.md)
    deferred to this epic.
  - `getRadiusModelsFromConfig(providerID string, cfg *RadiusGatewayConfig) []*ai.Model`
    — stamps `Api: ai.ApiPiMessages`, `Provider: providerID`,
    `BaseURL: cfg.BaseURL` onto each entry; and `getRadiusModels(providerID,
    credential)` returning the credential-carried catalog or nil.
  - `loadRadiusGatewayConfig(ctx context.Context, gateway, apiKey string)
    (*RadiusGatewayConfig, error)` — GET `<gateway>/v1/config` with
    `accept: application/json` and `authorization: Bearer <apiKey>` only when a
    key is present, honoring `ctx` for cancellation. Errors, verbatim from
    upstream: `Could not load Radius config from <gateway>: <status>: <body>`
    with the body trimmed and truncated to 512 chars plus `…`, and
    `Invalid Radius config from <gateway>` when the payload does not sanitize.
    `fetchJSON` (`ai/providers/refreshmodels.go:26`) cannot be reused — its
    error text is `providers: GET …`, and this text is upstream's.
- `ai/providers/radius_config_test.go` (new) — the criteria below. Upstream
  ships no test for this file (`packages/ai/test` has only `pi-messages` and
  `radius-oauth`), so this coverage is original and is what epic 8's
  acceptance criterion 3 asks for.

## Out of scope

- **The provider binding, its auth, and `RefreshModels`** —
  [issue 04](/epic-8-pi-messages-and-radius/issues/04-radius-provider-binding.md).
  Nothing here registers a provider or touches `ai/providers/all.go`.
- `normalizeRadiusGatewayUrl` — already ported as `oauth.NormalizeGatewayURL`
  (epic 7 issue 04). Call it; do not re-implement it, and do not move it.
- The OAuth flow itself and anything under `ai/auth/oauth` —
  [epic 7](/epic-7-four-new-oauth-flows/EPIC_7.md).
- Persisting the fetched catalog. `ai.ModelsStore` writes happen through the
  refresh pipeline in issue 04; this file only produces values.
- ETag / `If-None-Match` conditional fetching. Upstream's
  `loadRadiusGatewayConfig` sends neither at `936aff00`.
- `docs/PORTING.md` / `docs/auth.md` —
  [issue 05](/epic-8-pi-messages-and-radius/issues/05-porting-and-docs.md).

## Acceptance criteria / Definition of done

- [ ] `TestLoadRadiusGatewayConfigRequestsTheConfigEndpoint` — against an
      `httptest` gateway, the recorded request path is `/v1/config`, `accept`
      is `application/json`, and `authorization` is `Bearer <key>` when a key
      is passed and **absent** when it is not.
- [ ] `TestLoadRadiusGatewayConfigIgnoresAPathOnTheGatewayURL` — a gateway of
      `<server>/base` still requests `/v1/config`, matching
      `new URL("/v1/config", gateway)`.
- [ ] `TestLoadRadiusGatewayConfigFailsOnNonSuccess` — a 503 with body
      `gateway down` fails with exactly
      `Could not load Radius config from <gateway>: 503: gateway down`; a body
      longer than 512 characters is truncated with a trailing `…`.
- [ ] `TestLoadRadiusGatewayConfigRejectsAMalformedPayload` — `{}`,
      `{"baseUrl":1,"models":[]}` and `[]` each fail with exactly
      `Invalid Radius config from <gateway>`.
- [ ] `TestSanitizeDropsIncompleteModelsInsteadOfZeroFilling` — a payload whose
      first model lacks `contextWindow` and whose second is complete yields
      exactly one model; **no** model with `ContextWindow == 0` survives. This
      is the `encoding/json` trap named in the Summary and the test that pins
      it.
- [ ] `TestLoadRadiusGatewayConfigHonorsContextCancellation` — a cancelled
      context returns an error and the handler's response is never consumed.
- [ ] `TestGetRadiusModelsFromConfigStampsApiProviderAndBaseURL` — every
      returned `*ai.Model` has `Api == ai.ApiPiMessages`, the given provider
      id, and the config's `baseUrl`, with cost, context window, max tokens and
      `thinkingLevelMap` carried through unchanged.
- [ ] `TestGetRadiusCredentialConfigReadsTheStoredGatewayConfig` — an
      `*ai.OAuthCredential` whose `Extra["gatewayConfig"]` holds a valid config
      yields those models; a credential with no `gatewayConfig`, with a
      malformed one, and a nil credential each yield none.
- [ ] Tests are offline, stdlib-only (`testing` + `net/http/httptest`), no
      `t.Parallel()`, no build tags, discrete named functions — except a
      table for the sanitizer's input→output matrix, which is exactly where
      `docs/planning/CONVENTIONS.md` allows one.
- [ ] `// Ports: packages/ai/src/providers/radius-config.ts` after the
      `package` clause on `radius_config.go`. `radius_config_test.go` carries
      **no** header — upstream ships no test for this file (`packages/ai/test`
      has only `pi-messages` and `radius-oauth`), so this coverage is
      original, and `docs/planning/CONVENTIONS.md:63-66` makes that absence
      meaningful.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(providers): port the radius gateway config loader`.

## Relevant files / areas

- `ai/providers/refreshmodels.go:26` `fetchJSON` — the existing GET helper, and
  why it is not reused here.
- `ai/providers/openrouter.go:78-130` — the closest precedent for decoding a
  remote catalog into `[]*ai.Model`, including its package-level URL var for
  `httptest`.
- `ai/auth.go:48-56` `OAuthCredential` and `Extra`, `:73-94` the
  `UnmarshalJSON` that fills it.
- `ai/model.go:54-71` `Model` (`ThinkingLevelMap`, `Input`, `Cost`,
  `ContextWindow`, `MaxTokens`, `Compat` — which stays nil for pi-messages).
- `ai/auth/oauth/radius.go` — `NormalizeGatewayURL` from
  [epic 7 issue 04](/epic-7-four-new-oauth-flows/issues/04-radius-gateway-oauth.md).
- `docs/planning/CONVENTIONS.md`, "Naming & file layout" — the
  one-file-per-binding rule this file departs from, and "Error handling" for
  the `%w` wrapping style.
- Upstream at `936aff00`: `src/providers/radius-config.ts` (96 —
  `DEFAULT_RADIUS_GATEWAY`, `isRadiusGatewayModel`,
  `sanitizeRadiusGatewayConfig`, `getRadiusCredentialConfig`,
  `getRadiusModelsFromConfig`, `getRadiusModels`, `truncateHttpBody`,
  `loadRadiusGatewayConfig`).

## Dependencies

- **Blocked by**: [Issue 01](/epic-8-pi-messages-and-radius/issues/01-pimessages-wire-and-converter.md)
  (`ai.ApiPiMessages`); [epic 7 issue 04](/epic-7-four-new-oauth-flows/issues/04-radius-gateway-oauth.md)
  (`oauth.NormalizeGatewayURL`).
- **Blocks**: [Issue 04](/epic-8-pi-messages-and-radius/issues/04-radius-provider-binding.md),
  [issue 05](/epic-8-pi-messages-and-radius/issues/05-porting-and-docs.md).

## PR size note

`M` — ~450 changed lines: upstream's `radius-config.ts` is only 96 TS lines,
but the port runs longer than its source because every required field becomes a
pointer in an intermediate struct so a missing `contextWindow` drops the model
instead of zero-filling it, and because the eight named tests plus the
sanitizer's input→output table are all original coverage (upstream ships no
test for this file). Split past ~500, and the seam is the types and sanitizer
first, `loadRadiusGatewayConfig` and its network cases second.
