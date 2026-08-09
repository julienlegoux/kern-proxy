---
type: Issue
title: "Port the Models refresh contract: RefreshModelsContext, ModelsPublication, and the provider overlay"
description: "Replace refreshModels() with the generation-checked publish pipeline, add ModelsRefreshOptions/Result, and rebuild createProvider around a static baseline plus a dynamic overlay."
tags: [epic-2]
timestamp: 2026-08-09T04:31:17Z
epic: 2
issue: 08
slug: models-refresh-contract
size: L
status: open
gh_issue: 136
resource: https://github.com/kern-ia/kern-link/issues/136
depends_on: [5, 7]
---

# Port the Models refresh contract: RefreshModelsContext, ModelsPublication, and the provider overlay

## Summary

This is the largest single contract change in the sync. Upstream replaced
`Provider.refreshModels(): Promise<void>` and
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

Four behaviors come with it, and none of them is optional if the contract is to
mean anything:

1. **Two phases per provider.** A cache-only phase (`allowNetwork: false`)
   restores `context.stored` *before* auth resolution or any network access;
   only then does the network phase run with the effective credential.
2. **Generation-checked publication.** `setProvider`/`deleteProvider`/
   `clearProviders` supersede an in-flight refresh; a superseded `publish()`
   returns false and mutates nothing. Publications for one provider are
   serialized.
3. **Refresh no longer rejects.** Provider errors and cancellation come back in
   `ModelsRefreshResult` instead of as a thrown `ModelsError`.
4. **createProvider keeps a static baseline plus a dynamic overlay.** `getModels`
   returns the baseline with dynamic entries merged in by model ID (dynamic wins
   on collision, new IDs append), instead of the old wholesale replacement.
   `refreshModels` on the provider is now derived from a `fetchModels` input.

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
- `modelsImpl` — port the generation and publication machinery
  (`refreshGenerations`, `refreshControllers`, `publicationChains` upstream) with
  Go mechanisms: a per-provider generation counter under the existing mutex, a
  `context.CancelFunc` per in-flight refresh, and serialized publication per
  provider. **The observable contract is what must match** — superseded
  publications return false and mutate nothing — not the JavaScript mechanism.
- `CreateProviderOptions` — `RefreshModels func(ctx) ([]*Model, error)`
  (`ai/provider.go:361`) becomes upstream's `FetchModels(ctx, rc) ([]*Model, error)`;
  `providerImpl` keeps `baselineModels` and `dynamicModels` and merges them in
  `GetModels()` by ID.
- Update the four dynamic bindings in `ai/providers` to the new
  `FetchModels` shape, mechanically — no behavior change beyond what the
  contract forces.

## Out of scope

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
- [ ] `TestPublishReturnsFalseWhenProviderSuperseded` — replacing the provider
      via `SetProvider` mid-refresh makes the in-flight `publish` return false
      and leaves both the store and the provider's model list untouched.
- [ ] `TestRefreshReturnsProviderErrorsWithoutFailing` — one failing provider
      among several yields a non-nil result whose error map has exactly that
      provider's ID, and `Refresh` itself does not return an error.
- [ ] `TestRefreshHonorsProviderFilter` — `ModelsRefreshOptions.Providers`
      restricts the run; unknown and static providers are skipped silently.
- [ ] `TestRefreshCancelledContextReportsAborted` — a cancelled `ctx` yields
      `Aborted: true` and no partial publication.
- [ ] `TestGetModelsMergesDynamicOverlayOverBaseline` — a dynamic model sharing
      an ID with a baseline model replaces it in place; a new ID appends; the
      baseline is never lost after a failed refresh.
- [ ] The four dynamic bindings still list models:
      `GOTMPDIR=$PWD/.gotmp go test ./ai/providers/...` green.
- [ ] `Models.Refresh`'s and `Provider.RefreshModels`' doc comments specify
      failure and concurrency behavior to the bar set by `ai/provider.go:26-41`.
- [ ] CI green: `go test ./... -race -v` — **the race detector is the point
      here**, and its verdict only ever arrives from CI — plus
      `bash upstream/sync_test.sh` and `golangci-lint` v2.12.2.
- [ ] `gofmt -l .` prints nothing; `ai/provider.go` keeps its `// Ports:` header.
- [ ] Conventional Commit, e.g.
      `feat(ai)!: publish model refreshes through a generation-checked store`.

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
  (both edit `ai/provider.go` in the same regions).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR. **Sized L deliberately**: the `Provider` interface change, the `modelsImpl`
refresh rework, and the `createProvider` overlay are one contract — landing any
one alone leaves the interface half-migrated and the dynamic bindings
uncompilable. If it approaches ~1000 lines anyway, the natural cut is
*contract + createProvider overlay* first, *generation/publication concurrency*
second, with the interim `publish` implemented as an unconditional apply and the
gap flagged in the PR body.
