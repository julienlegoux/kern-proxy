---
type: Issue
title: "Reshape ai/images.Options onto ProviderRequestOptions, honor the fetch override, and carry the getAuth overrides"
description: "Adopt the two forced ai/images consequences of the upstream core-type and auth restructures — Options on the shared request base with its fetch injection, and Models.GetAuth's overrides plus provider-id form — neither of which any other epic accepts."
tags: [epic-3]
timestamp: 2026-08-11T21:30:00Z
epic: 3
issue: 07
slug: images-options-base-and-auth-overrides
size: M
status: open
gh_issue: 218
resource: https://github.com/kern-ia/kern-link/issues/218
depends_on: ["06"]
---

# Reshape ai/images.Options onto ProviderRequestOptions, honor the fetch override, and carry the getAuth overrides

## Summary

Two upstream restructures reach into `ai/images` and neither had an owner. Each
was deferred to an epic that disclaims it, so both would have closed green with
nothing behind them.

**1. `ImagesOptions` moves onto the shared request base.** At `936aff00`,
`packages/ai/src/types.ts:293-299` reads:

```ts
export interface ImagesOptions extends ProviderRequestOptions<ImagesModel<ImagesApi>> {
	metadata?: Record<string, unknown>;
}
```

Everything else `ImagesOptions` used to declare now comes from the base
(`types.ts:119-173`): `apiKey`, `fetch`, `env`, `onPayload`, `onResponse`,
`headers`, `timeoutMs`, `maxRetries`, `maxRetryDelayMs`. Go's
`images.Options` (`ai/images/types.go:81-106`) hand-duplicates eight of those
nine and is **missing `fetch` entirely**.

[Decision 14](../../../planning/scope/14-images-surface.md) calls this the
*forced* part of the images surface — "the type change is forced, so `ai/images`
must at minimum keep compiling" (`:40-41`) — and its Verdict folds it into this
epic (`:59-67`). It was nonetheless deferred in a circle:
[Epic 2 issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md)
pushed it here, and this epic's
[issues 05](/epic-3-catalog-schema-and-export-tooling/issues/05-regenerate-image-catalog.md)
and [06](/epic-3-catalog-schema-and-export-tooling/issues/06-images-adapter-retry.md)
pushed it back to Epic 2. The condition Epic 2 issue 05 carried at the time —
touch `ai/images` only if the refactor breaks compilation — could never fire:
`images.Options` is a standalone struct that does not embed `ai.StreamOptions`,
so nothing in `ai/images` stops compiling when `ai.StreamOptions` is split.
Epic 2 issue 05 now disclaims `ai/images` unconditionally and names this issue
as its owner, so the circle is broken here.

**2. `getAuth` gains overrides and a provider-id form.** Upstream's
`packages/ai/src/images-models.ts` at `936aff00` declares, on the images
`Models`:

```ts
getAuth(providerId: string, overrides?: AuthResolutionOverrides): Promise<AuthResult | undefined>;
getAuth(model: ImagesModel<ImagesApi>, overrides?: AuthResolutionOverrides): Promise<AuthResult | undefined>;
```

and `generateImages()` now routes its own resolution through it:

```ts
const resolution = await this.getAuth(model, { apiKey: options?.apiKey, env: options?.env, signal: options?.signal });
```

This is **not** TypeScript-only ergonomics, which is the other disposition the
check could have produced. Go's `images.Models.GetAuth(ctx, model)`
(`ai/images/provider.go:64-67`, `:213-219`) takes no overrides and has no
provider-id sibling, and `GenerateImages` (`:221-242`) builds
`ai.AuthResolutionOverrides` inline instead of going through `GetAuth` — so the
two entry points can drift. It is a change to `ai/images`' **public interface**,
in a package `EPIC_6.md` never mentions and
[Epic 6 issue 10](/epic-6-auth-core-and-env-api-key-bindings/issues/10-env-api-key-bindings.md)
hands back to this epic by name.

## Scope

- `ai/images/types.go` — embed `ai.ProviderRequestOptions` in `Options`, keeping
  only `Metadata` declared locally, mirroring upstream's one-field extension.
  Cancellation stays on `context.Context`; `telemetryContext` stays unported —
  both are established deviations
  ([Epic 2 issue 13](/epic-2-core-types-and-models-contracts/issues/13-fetch-sampling-and-deferred-options.md)
  records the second in `docs/PORTING.md`). Callers writing
  `&images.Options{APIKey: "k", Timeout: time.Second}` keep working through
  embedded-field promotion; **composite literals that set those fields by key
  do not**, so sweep the tree for them.
- `ai/images/openrouter.go` — honor `Options.Fetch`. `generateImagesOpenRouter`
  builds a bare `&http.Client{}` at `:239` today; after
  [issue 06](/epic-3-catalog-schema-and-export-tooling/issues/06-images-adapter-retry.md)
  the request goes through the shared retry loop, so `Fetch` is injected there
  rather than at the client. A nil `Fetch` keeps today's behavior exactly.
- `ai/images/provider.go` — the auth half:
  - `Models.GetAuth` gains an `overrides *ai.AuthResolutionOverrides`
    parameter. The type already exists (`ai/resolve.go:48-53`) and
    `GenerateImages` already builds one, so this is a signature change plus a
    call-site move, not a new concept.
  - Add the provider-id form. Go has no overloads, so it is a second method;
    name it `GetAuthForProvider(ctx, providerID string, overrides *ai.AuthResolutionOverrides)`,
    matching the name
    [Epic 6 issue 06](/epic-6-auth-core-and-env-api-key-bindings/issues/06-models-login-logout.md)
    gives the identical shaping on `ai.Models`. Two packages solving one
    upstream overload two different ways is exactly the drift the next sync
    pays for.
  - `GenerateImages` resolves through `GetAuth` instead of calling
    `ai.ResolveProviderAuth` directly, so the two paths cannot diverge.
- `docs/PORTING.md` — the `src/images-models.ts` row records the two-method
  shaping of the `getAuth` overload, pointing at the `ai.Models` decision rather
  than restating it.

## Out of scope

- **Anything `ai.ProviderRequestOptions` itself.** Declaring the base is
  [Epic 2 issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md);
  the `FetchFunction` type and the `telemetryContext` non-port are
  [Epic 2 issue 13](/epic-2-core-types-and-models-contracts/issues/13-fetch-sampling-and-deferred-options.md).
  This issue consumes both.
- **`ResolveProviderAuth`'s own signature change** (dropping `model`) —
  [Epic 6 issue 03](/epic-6-auth-core-and-env-api-key-bindings/issues/03-provider-scoped-apikey-resolution.md),
  which already lists `ai/images/`'s call sites and tests among its mechanical
  edits. This issue adds the `overrides` parameter to `images.Models.GetAuth`;
  it does not change what `ResolveProviderAuth` takes.
- **Retry behavior in the images adapter** —
  [issue 06](/epic-3-catalog-schema-and-export-tooling/issues/06-images-adapter-retry.md).
- **Catalog regeneration** — issues
  [04](/epic-3-catalog-schema-and-export-tooling/issues/04-regenerate-model-catalog.md)
  and [05](/epic-3-catalog-schema-and-export-tooling/issues/05-regenerate-image-catalog.md).
- **OpenRouter images OAuth** — [Epic 7](/epic-7-four-new-oauth-flows/EPIC_7.md)
  per [decision 09](../../../planning/scope/09-new-oauth-flows.md).

## Acceptance criteria / Definition of done

- [ ] `images.Options` embeds `ai.ProviderRequestOptions` and declares only
      `Metadata` of its own — `grep -c "MaxRetryDelay \*time.Duration" ai/images/types.go`
      returns `0`, and the field still resolves through promotion.
- [ ] `TestImagesOptionsPromotesTransportFields` — a caller writing
      `&images.Options{APIKey: "k", Timeout: time.Second}` compiles and both
      values reach the adapter.
- [ ] `TestGenerateImagesOpenRouterUsesInjectedFetch` — an `httptest` case where
      a non-nil `Options.Fetch` is what the request goes through (assert the
      injected doer saw it), and a nil `Fetch` still reaches the server through
      the default path.
- [ ] `TestImagesGetAuthAppliesOverrides` — `GetAuth(ctx, model, &ai.AuthResolutionOverrides{APIKey: …})`
      returns the override's key, and passing nil reproduces today's result.
- [ ] `TestImagesGetAuthForProviderResolvesWithoutAModel` — an ambient env key
      resolves from the provider id alone, and an unknown id returns
      `(nil, nil)` (the package's existing unconfigured contract,
      `ai/images/provider.go:20-22`).
- [ ] `TestGenerateImagesResolvesThroughGetAuth` — `GenerateImages` and
      `GetAuth` agree on the same request: the same override that changes one
      changes the other.
- [ ] `docs/PORTING.md` records the `getAuth` two-method shaping and names
      `ai.Models`' identical decision.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] `ai/images/types.go` and `ai/images/provider.go` keep their `// Ports:`
      headers, updated if the ported symbol list changed.
- [ ] Conventional Commit, e.g.
      `refactor(images)!: put Options on ProviderRequestOptions and carry auth overrides`.

## Relevant files / areas

- `ai/images/types.go:81-106` `Options` — the eight hand-duplicated transport
  fields and the missing `Fetch`.
- `ai/images/provider.go:45-74` the `Models` interface (`:64-67` `GetAuth`),
  `:213-219` its implementation, `:221-242` `GenerateImages` and its inline
  `ai.AuthResolutionOverrides` construction.
- `ai/images/openrouter.go:204-291` `generateImagesOpenRouter` — `:239` the bare
  `&http.Client{}`.
- `ai/resolve.go:48-53` `AuthResolutionOverrides`, `:67-75` `ResolveProviderAuth`.
- `ai/images/provider_test.go:127`, `:205` and `ai/images/builtin_test.go:31` —
  the existing `GetAuth` call sites that the signature change touches.
- `docs/PORTING.md`.
- Upstream at `936aff00`: `packages/ai/src/types.ts:119-173`
  (`ProviderRequestOptions`), `:293-299` (`ImagesOptions`), and
  `packages/ai/src/images-models.ts` (the `getAuth` overloads and
  `generateImages`).

## Dependencies

- **Blocked by**: [Issue 06](/epic-3-catalog-schema-and-export-tooling/issues/06-images-adapter-retry.md)
  — both rewrite `generateImagesOpenRouter`'s transport, and `Fetch` is injected
  into the retry loop issue 06 introduces.
- **Cross-epic** (`depends_on` carries intra-epic numbers only, so these edges
  live here in prose):
  [Epic 2 issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md)
  (#133) declares `ProviderRequestOptions` and
  [Epic 2 issue 13](/epic-2-core-types-and-models-contracts/issues/13-fetch-sampling-and-deferred-options.md)
  (#225) adds `Fetch` and the `FetchFunction` type;
  [Epic 6 issue 03](/epic-6-auth-core-and-env-api-key-bindings/issues/03-provider-scoped-apikey-resolution.md)
  (#172) settles `ResolveProviderAuth`'s signature; and
  [Epic 6 issue 06](/epic-6-auth-core-and-env-api-key-bindings/issues/06-models-login-logout.md)
  (#175) fixes the `GetAuthForProvider` name this issue reuses.
- **Blocks**: Nothing.
- **Scheduling consequence, recorded rather than hidden**: the Epic 6 edges mean
  this issue lands *after* Epic 6, not inside Epic 3's own window. It is the
  epic's tail issue by design — the alternative was leaving the images half of
  the auth restructure with no owner at all, which is the defect
  [Epic 0 issue 02](/epic-0-plan-remediation/issues/02-adopt-the-orphaned-scope-items.md)
  exists to close. Splitting the `Fetch` half out to land in Epic 3's window is
  a legitimate call for the implementer; say so in the PR body if you take it.

## PR size note

`M` — ~250–450 changed lines: a struct reshape with its call-site sweep, one
adapter transport change, two interface methods and six tests. If the
composite-literal sweep across `ai/images` and its tests turns out to be larger
than mechanical, land the `Options` reshape alone and open a follow-up for the
auth half; the two halves share no file except `ai/images/provider.go`.
