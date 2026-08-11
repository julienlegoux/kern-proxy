---
type: Issue
title: "Bind the radius provider over pi-messages, with its two-phase catalog refresh"
description: "Port src/providers/radius.ts as ai/providers/radius.go — a gateway-parameterised binding with env-API-key and OAuth auth, no static catalog, and a refresh that restores the stored catalog before it ever touches the network."
tags: [epic-8]
timestamp: 2026-08-11T20:00:00Z
epic: 8
issue: 04
slug: radius-provider-binding
size: M
status: open
gh_issue: 190
resource: https://github.com/kern-ia/kern-link/issues/190
depends_on: ["02", "03"]
---

# Bind the radius provider over pi-messages, with its two-phase catalog refresh

## Summary

The 36th binding, and the first one that is **purely dynamic**: Radius has no
entry in the embedded catalog, so before a refresh it lists nothing. Upstream
marks the same distinction — `providers/all.ts:50-52` says `BuiltinProvider`
covers the generated catalog while `KnownProvider` additionally includes
"purely dynamic providers (e.g. `radius`) that have no static catalog entry".

`EPIC_8.md:31-33` says Radius fetches its gateway config "at provider setup";
here that means the `RefreshModels` pipeline below, not construction —
`RadiusProvider` itself makes no network call, matching the project-wide rule
that construction touches no network.

That single fact breaks an existing test, and it is better to meet it here than
in CI: `TestBuiltinModelsRegistersEveryProviderWithModels`
(`ai/providers/providers_test.go:94`) asserts `len(all) == 35` **and** that no
registered provider has an empty model list (`:110-112`). The count bump is
mechanical; the empty-list assertion needs the upstream distinction encoded —
exempt providers that carry no static catalog, and name radius in the exemption
with a comment pointing at `all.ts:50-52` so the next dynamic binding knows the
rule. Do not weaken the assertion for everyone.

**The refresh is the design point this epic owns.** Upstream's `refreshModels`
runs in phases, and the ordering is the whole point — a user offline with a
stored catalog still gets their models:

1. `context.stored` present → publish the stored models filtered to this
   provider id, and **stop if `publish` returns false** (the provider was
   superseded mid-flight).
2. No stored entry but an OAuth credential → import the catalog the
   pre-`ModelsStore` implementation cached on the credential
   (`getRadiusModels`, from
   [issue 03](/epic-8-pi-messages-and-radius/issues/03-radius-gateway-config.md)),
   persisting it.
3. `context.allowNetwork` false, or the context cancelled → return. **The
   network call is gated, not unconditional.**
4. Fetch `/v1/config` with the effective key (an OAuth credential's access
   token, else the api key), re-check cancellation, publish the fetched catalog
   with `persist`.

That maps one-to-one onto the `RefreshModelsContext`/`ModelsPublication`
contract [epic 2 issue 08](/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md)
lands (`Credential`, `Stored`, `Publish`, `AllowNetwork`): `08:30-35` carries
`publish(publication): Promise<boolean>` as part of the interface this issue
ports, and `08:70-72`, `:87` add `RefreshModelsContext` to `ai` and pass it
into `FetchModels(ctx, rc)` — a shape that returns *one* list per call, which
is why radius calls `rc.Publish` more than once rather than returning more
than one list. Use `rc.Publish` and follow upstream exactly. If the landed
signature differs from this contract by the time this PR is written, report it
in the PR body rather than quietly collapsing the phases into a single network
fetch — that is exactly the offline regression the phases exist to prevent.

Auth declares both methods, as upstream does: `RADIUS_API_KEY` through
`auth.EnvAPIKeyAuth`, and the flow from
[epic 7 issue 04](/epic-7-four-new-oauth-flows/issues/04-radius-gateway-oauth.md)
bound to this binding's gateway. Upstream wraps the flow in `lazyOAuth`;
kern-link constructs `ai.OAuthAuth` directly (`docs/PORTING.md`, the
`helpers.ts` row) — no lazy shim.

## Scope

- `ai/providers/radius.go` (new) — `// Ports: packages/ai/src/providers/radius.ts`:
  - `RadiusProviderOptions{ID, Name, Gateway string}` and
    `RadiusProvider(options *RadiusProviderOptions) ai.Provider`. A nil/zero
    options value must behave exactly like upstream's `radiusProvider()`:
    id `radius`, name `Radius`, gateway `DefaultRadiusGateway`. This is the
    only binding in `ai/providers` taking a parameter — note why in the doc
    comment (self-hosted gateways, one deployment per base URL).
  - The gateway is run through `oauth.NormalizeGatewayURL` **once**, at
    construction, and the normalized value is what both the auth strategy and
    the config fetch see.
  - `ai.CreateProviderOptions`: `BaseURL` = the normalized gateway, `Models`
    empty (no `catalog.BuiltinModels("radius")` — there is no such catalog
    file, by design), `Auth` = `{APIKey: auth.EnvAPIKeyAuth("Radius API key",
    []string{"RADIUS_API_KEY"}), OAuth: oauth.RadiusOAuth(...)}`, and
    `Api: ai.StreamFuncs{StreamFunc: pimessages.Stream, StreamSimpleFunc:
    pimessages.StreamSimple}`.
  - The refresh function, in the four phases above.
- `ai/providers/all.go` — register the binding in `Providers()` in the file's
  existing alphabetical position, and update the provider count in the package
  doc comment to whatever it is after this PR (epic 6 issue 10 adds four env-api-key
  bindings; take the current number rather than hardcoding 36).
- `ai/providers/providers_test.go` — bump `wantProviderCount` and exempt
  catalog-less providers from the "has no models" assertion, per the Summary.
- `ai/providers/radius_test.go` (new) — the criteria below.

## Out of scope

- `radius_config.go` and the gateway-config fetch —
  [issue 03](/epic-8-pi-messages-and-radius/issues/03-radius-gateway-config.md).
- The pi-messages adapter —
  [issues 01](/epic-8-pi-messages-and-radius/issues/01-pimessages-wire-and-converter.md)
  and [02](/epic-8-pi-messages-and-radius/issues/02-pimessages-stream-entry.md).
  This PR wires `pimessages.Stream`/`StreamSimple` in and changes nothing
  inside that package.
- The OAuth flow — [epic 7](/epic-7-four-new-oauth-flows/EPIC_7.md). If
  `oauth.RadiusOAuth`'s signature does not fit, say so in the PR body rather
  than editing it from here.
- Changing the `RefreshModelsContext` contract. It is
  [epic 2](/epic-2-core-types-and-models-contracts/EPIC_2.md)'s; this binding
  is its first non-trivial consumer, and a gap is a report, not a local patch.
- Registering a second Radius provider for another gateway, or exposing gateway
  configuration through the CLI.
- `docs/auth.md`'s `RADIUS_API_KEY` row and `docs/PORTING.md` —
  [issue 05](/epic-8-pi-messages-and-radius/issues/05-porting-and-docs.md).

## Acceptance criteria / Definition of done

- [ ] `TestRadiusProviderDefaults` — `RadiusProvider(nil)` has `ID() ==
      "radius"`, `Name() == "Radius"`, `BaseURL() == "https://radius.pi.dev"`,
      a non-nil `Auth().APIKey` **and** a non-nil `Auth().OAuth`, and
      `CanRefreshModels() == true`.
- [ ] `TestRadiusProviderOptionsOverrideIdentityAndNormalizeTheGateway` — options
      `{ID: "radius-dev", Name: "Radius Dev", Gateway: "radius.example.com///"}`
      yield that id and name and a `BaseURL()` of
      `https://radius.example.com`.
- [ ] `TestRadiusListsNoModelsBeforeARefresh` — `GetModels()` is empty, and
      `Models(nil)` still constructs and finds the provider by id.
- [ ] `TestRadiusRefreshRestoresTheStoredCatalogBeforeAnyNetworkCall` — with a
      stored entry seeded, the stored models are published before the gateway
      is contacted (assert ordering via a handler that records when it ran),
      and entries belonging to another provider id are filtered out.
- [ ] `TestRadiusRefreshImportsTheCredentialCarriedCatalog` — with no stored
      entry and an OAuth credential whose `Extra["gatewayConfig"]` holds a
      config, those models are published **and persisted**.
- [ ] `TestRadiusRefreshFetchesTheGatewayConfigWithTheEffectiveKey` — against an
      `httptest` gateway: an OAuth credential sends
      `authorization: Bearer <access>`, an api-key credential sends the key,
      and the fetched models replace the list with `Api == ai.ApiPiMessages`.
- [ ] `TestRadiusRefreshSkipsTheNetworkWhenNotAllowed` — `AllowNetwork` false
      makes zero requests to the gateway and leaves the restored list in place.
- [ ] `TestRadiusRefreshStopsWhenPublishReportsSuperseded` — a first `Publish`
      returning false ends the refresh: no gateway request follows.
- [ ] `TestRadiusRefreshKeepsLastKnownModelsWhenTheGatewayFails` — a 503 from
      `/v1/config` surfaces the error and leaves `GetModels()` at its
      last-known value, the contract in `ai/provider.go:31-35`.
- [ ] `TestRadiusStreamsThroughThePiMessagesAdapter` — a model from the fetched
      catalog, streamed through the provider against an `httptest` pi-messages
      backend, resolves a terminal message; this is the assertion that the
      binding is actually wired to the adapter and not merely compiling.
- [ ] `TestBuiltinModelsRegistersEveryProviderWithModels` passes with the
      updated count and the catalog-less exemption, and still fails if a
      catalog-backed provider loses its models.
- [ ] Tests are offline, stdlib-only (`testing` + `net/http/httptest`), no
      `t.Parallel()`, no build tags, discrete named functions.
- [ ] `// Ports: packages/ai/src/providers/radius.ts` after the `package`
      clause; `all.go`'s existing porting header and provider-count comment
      updated rather than duplicated.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(providers): bind the radius gateway over pi-messages`.

## Relevant files / areas

- `ai/providers/all.go:20-60` `Providers()` and `:63-70` `Models()`; its
  package doc comment carries the provider count.
- `ai/providers/providers_test.go:94-113` — the count and
  every-provider-has-models assertions this PR must update.
- `ai/providers/openrouter.go:132-143` and `ai/providers/github_copilot.go:120-130`
  — the two closest dynamic-binding precedents, including how each reaches for
  a credential today (Copilot reads `COPILOT_GITHUB_TOKEN` from the
  environment, a workaround the new refresh contract makes unnecessary here).
- `ai/provider.go:26-41` the `GetModels`/`RefreshModels` contract prose,
  `:346-367` `CreateProviderOptions`, `:476-489` `StreamFuncs`.
- `ai/providers/radius_config.go` — from
  [issue 03](/epic-8-pi-messages-and-radius/issues/03-radius-gateway-config.md):
  `DefaultRadiusGateway`, `getRadiusModels`, `getRadiusModelsFromConfig`,
  `loadRadiusGatewayConfig`.
- `ai/auth/oauth/radius.go` — `RadiusOAuth`, `RadiusOAuthOptions`,
  `NormalizeGatewayURL` (epic 7 issue 04).
- `ai/auth` — `EnvAPIKeyAuth`, as used by every env-key binding.
- Upstream at `936aff00`: `src/providers/radius.ts` (82),
  `src/providers/all.ts:37,48,50-52,121`.

## Dependencies

- **Blocked by**: [Issue 02](/epic-8-pi-messages-and-radius/issues/02-pimessages-stream-entry.md)
  (the exported `Stream`/`StreamSimple`),
  [issue 03](/epic-8-pi-messages-and-radius/issues/03-radius-gateway-config.md);
  [epic 7 issue 04](/epic-7-four-new-oauth-flows/issues/04-radius-gateway-oauth.md)
  (`oauth.RadiusOAuth`);
  [epic 2 issue 08](/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md)
  (`RefreshModelsContext`, `ModelsPublication`) and
  [issue 07](/epic-2-core-types-and-models-contracts/issues/07-models-store.md)
  (`ai.ModelsStore`, which the stored phase reads).
- **Blocks**: [Issue 05](/epic-8-pi-messages-and-radius/issues/05-porting-and-docs.md).

## PR size note

`M` — ~450 changed lines: an 82-line upstream binding whose four-phase
`RefreshModels` carries most of the weight, ten named tests, and the two
`ai/providers/providers_test.go:94-113` assertions this binding invalidates —
the provider count and the every-provider-has-models check radius is the first
exemption to. Split past ~500, and the seam is the binding with its defaults
and auth first, the refresh phases and their tests second.
