---
type: Issue
title: "Port the Models refresh contract: the two-phase refresh, ModelsPublication types, and the provider overlay"
description: "Replace refreshModels() with the context-carrying refresh contract, add ModelsRefreshOptions/Result, and rebuild createProvider around a static baseline plus a dynamic overlay — the generation-checked publish machinery is issue 14."
tags: [epic-2]
timestamp: 2026-08-11T18:15:00Z
epic: 2
issue: 08
slug: models-refresh-contract
size: L
status: open
gh_issue: 136
resource: https://github.com/kern-ia/kern-link/issues/136
depends_on: [5, 7]
---

# Port the Models refresh contract: the two-phase refresh, ModelsPublication types, and the provider overlay

## Summary

Upstream replaced `Provider.refreshModels(): Promise<void>` and
`Models.refresh(provider?): Promise<void>` with a transactional pipeline:

```ts
export interface ModelsPublication {
	persist?: ModelsStoreEntry | null;   // omit = leave storage alone; null = delete
	update?: () => void;                 // synchronous in-memory update, runs only after persistence
}
export interface RefreshModelsContext {
	credential?: Credential;
	stored?: Readonly<ModelsStoreEntry>;
	publish(publication: ModelsPublication): Promise<boolean>;   // false when superseded
	allowNetwork: boolean;
	force?: boolean;
	signal: AbortSignal;                 // always present
}
export interface ModelsRefreshOptions { allowNetwork?; providers?; force?; signal? }
export interface ModelsRefreshResult { aborted: boolean; errors: ReadonlyMap<string, Error> }
```

This issue is one half of a pre-split
([Epic 0 issue 05](/epic-0-plan-remediation/issues/05-pre-split-epic-2-issues-05-and-08.md)):
it ports the contract types, the two-phase refresh (cache-only restore before
auth resolution, then the network phase), refresh-no-longer-rejects, and the
`createProvider` baseline-plus-overlay rework. The generation-checked
concurrency behind `publish()` — the part that makes a superseded refresh
return `false` and mutate nothing, and cancellation of an in-flight refresh —
is [issue 14](/epic-2-core-types-and-models-contracts/issues/14-refresh-generation-and-publication.md),
which depends on this one and lands after it. There is no interim fallback: the
split was decided before either PR opened, so this issue does not ship an
"unconditional apply" stand-in for what issue 14 builds — `publish()` applies
its `ModelsPublication` directly here, with the supersede check itself absent
until issue 14 lands.

kern-link has four dynamic providers (`openrouter`, `vercel-ai-gateway`,
`nvidia`, `github-copilot`) whose native `RefreshModels` is itself a recorded
deviation from upstream, so this rework lands on top of code the port already
owns.

## Scope

- `ai/provider.go` — replace `Provider.RefreshModels(ctx) error` with the
  context-carrying form (`RefreshModels(ctx context.Context, rc *RefreshModelsContext) error`
  or equivalent); keep `CanRefreshModels()`, which has no upstream counterpart
  but is how Go callers ask the same question.
- Add `ModelsPublication`, `RefreshModelsContext`, `ModelsRefreshOptions`,
  `ModelsRefreshResult`. Cancellation is `context.Context`, so
  `RefreshModelsContext` carries no signal field — the ctx is the first
  parameter. Record that in the type's doc comment.
- `Models.Refresh` becomes
  `Refresh(ctx context.Context, opts *ModelsRefreshOptions) (*ModelsRefreshResult, error)`
  (or returns the result alone) — it no longer fails on a provider's fetch
  failure, and its doc comment must say so in the contract style
  `ai/provider.go` already uses ("Must not fail", "Concurrent calls share one
  in-flight fetch").
- `modelsImpl` — port the two-phase refresh (`allowNetwork: false` restore of
  `context.stored` before auth resolution or any network access, then the
  network phase with the effective credential) and a `publish()` that applies
  a `ModelsPublication` to the store and in-memory state. This issue's
  `publish()` is **not** generation-checked — it has no supersede detection —
  that is issue 14's entire scope.
- `CreateProviderOptions` — `RefreshModels func(ctx) ([]*Model, error)`
  (`ai/provider.go:361`) becomes upstream's `FetchModels(ctx, rc) ([]*Model, error)`;
  `providerImpl` keeps `baselineModels` and `dynamicModels` and merges them in
  `GetModels()` by ID (dynamic wins on collision, new IDs append).
- Update the four dynamic bindings in `ai/providers` to the new
  `FetchModels` shape, mechanically — no behavior change beyond what the
  contract forces.

## Out of scope

- **Generation-checked publication and cancellation.** `setProvider`/
  `deleteProvider`/`clearProviders` superseding an in-flight refresh, a
  `publish()` that returns `false` and mutates nothing when superseded, a
  per-provider generation counter, serialized publications per provider, and
  the `context.CancelFunc` machinery for an in-flight refresh — all of
  [issue 14](/epic-2-core-types-and-models-contracts/issues/14-refresh-generation-and-publication.md),
  which depends on the contract this issue ships.
- **`Provider.filterModels` and `Models.getAvailable` / `checkAuth` / `login` /
  `logout`, and the `getAuth(providerId, …)` overload.** These arrive in the same
  upstream diff but depend on `AuthCheck`, `AuthType`, `AuthInteraction`, and
  `AuthOperationOptions` from `src/auth/types.ts`, which
  [Epic 6](/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md) owns
  ("Port the content of `auth/credential-store.ts`, `auth/helpers.ts`,
  `auth/resolve.ts`, `auth/types.ts` …"). Epic 2's scope names four contracts —
  `ModelsPublication`, `RefreshModelsContext`, `ModelsRefreshOptions`,
  `ModelsRequestTransforms` — and these are not among them. Leave
  `Models.GetAuth(ctx, model)` at its current signature and say so in the PR
  body; epic 6 completes the surface.
- **Credential resolution inside refresh.** Upstream's
  `resolveRefreshCredential` refreshes an expired OAuth credential before the
  network phase. kern-link already has that logic in
  `ai/resolve.go` (`resolveStoredOAuth`, double-checked lock). Reuse it; do not
  re-port `auth/resolve.ts`, which is epic 6's file.
- ETag / `If-None-Match` conditional fetching by the dynamic providers — that is
  provider-side behavior on top of `ModelsStoreEntry.ETag`, and belongs with the
  bindings in [Epic 5](/epic-5-remaining-adapters/EPIC_5.md) unless a binding
  breaks without it.
- `ModelsRequestTransforms` — [issue 09](/epic-2-core-types-and-models-contracts/issues/09-models-request-transforms.md).

## Acceptance criteria / Definition of done

- [ ] `TestRefreshRestoresStoredCatalogBeforeNetwork` — with a `ModelsStore`
      pre-seeded for a provider, the first phase publishes the stored models with
      `allowNetwork` false, before any fetch is attempted (assert ordering, e.g.
      via a fetch that records when it ran).
- [ ] `TestRefreshPersistsFetchedCatalog` — after a successful network phase the
      store holds the fetched models with `CheckedAt` set.
- [ ] `TestRefreshReturnsProviderErrorsWithoutFailing` — one failing provider
      among several yields a non-nil result whose error map has exactly that
      provider's ID, and `Refresh` itself does not return an error.
- [ ] `TestRefreshHonorsProviderFilter` — `ModelsRefreshOptions.Providers`
      restricts the run; unknown and static providers are skipped silently.
- [ ] `TestGetModelsMergesDynamicOverlayOverBaseline` — a dynamic model sharing
      an ID with a baseline model replaces it in place; a new ID appends; the
      baseline is never lost after a failed refresh.
- [ ] The four dynamic bindings still list models:
      `GOTMPDIR=$PWD/.gotmp go test ./ai/providers/...` green.
- [ ] `Models.Refresh`'s and `Provider.RefreshModels`' doc comments specify
      failure and concurrency behavior to the bar set by `ai/provider.go:26-41`,
      and note explicitly that generation-checked supersede handling ships in
      issue 14.
- [ ] This issue's own test list above contains no supersede-detection or
      cancellation-reporting test — both belong to
      [issue 14](/epic-2-core-types-and-models-contracts/issues/14-refresh-generation-and-publication.md),
      which names them.
- [ ] CI green: `go test ./... -race -v` — **the race detector is the point
      here**, and its verdict only ever arrives from CI — plus
      `bash upstream/sync_test.sh` and `golangci-lint` v2.12.2.
- [ ] `gofmt -l .` prints nothing; `ai/provider.go` keeps its `// Ports:` header.
- [ ] Conventional Commit, e.g.
      `feat(ai)!: publish model refreshes through the two-phase refresh contract`.

## Relevant files / areas

- `ai/provider.go` (489 lines) — `Provider` (`:15-42`), `Models` (`:47-76`),
  `CreateModelsOptions` (`:87`), `modelsImpl` (`:93-344`) including
  `Refresh` (`:188`), `CreateProviderOptions` (`:347-367`), `providerImpl`
  (`:369-474`) including `RefreshModels` (`:408`) and its in-flight sharing.
- `ai/modelsstore.go` — from
  [issue 07](/epic-2-core-types-and-models-contracts/issues/07-models-store.md).
- `ai/resolve.go:141` `resolveStoredOAuth`, `:188` `resolveAPIKey` — existing
  credential resolution to reuse.
- `ai/providers/` — the four dynamic bindings (`openrouter`,
  `vercel_ai_gateway`, `nvidia`, `github_copilot`); one file per binding.
- `ai/provider_test.go` — existing registry and refresh coverage.
- Upstream: `src/models.ts` at `936aff00` (`ModelsImpl.refresh`,
  `publishProviderModels`, `runProviderRefreshPhase`, `createProvider`).

## Dependencies

- **Blocked by**: [Issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md),
  [Issue 07](/epic-2-core-types-and-models-contracts/issues/07-models-store.md).
- **Blocks**: [Issue 09](/epic-2-core-types-and-models-contracts/issues/09-models-request-transforms.md),
  [Issue 10](/epic-2-core-types-and-models-contracts/issues/10-deferred-response-dispatch.md)
  (both edit `ai/provider.go` in the same regions), and
  [Issue 14](/epic-2-core-types-and-models-contracts/issues/14-refresh-generation-and-publication.md),
  which hardens the `publish()` this issue ships with generation checking.

## PR size note

`L` — ~700 changed lines even after the pre-split moved generation checking to
issue 14: a `Provider` interface change, four contract types, the two-phase
refresh in `modelsImpl` (`ai/provider.go` is 489 lines), the `createProvider`
baseline-plus-overlay rework, five named tests, and mechanical updates to all
four dynamic bindings. `L` is the ceiling, not the target: the interface change
and the bindings cannot land apart without an uncompilable tree. Past ~1000, cut
the `providerImpl` baseline/dynamic overlay out first — it merges by ID and is
green under today's `RefreshModels` shape.
