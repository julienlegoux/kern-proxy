---
type: Issue
title: "Generation-check ModelsPublication so a superseded refresh returns false and mutates nothing"
description: "Port the generation-counter and per-provider publication serialization behind Models.Refresh's publish(), plus in-flight cancellation, so setProvider/deleteProvider/clearProviders correctly supersede a running refresh."
tags: [epic-2]
timestamp: 2026-08-11T18:15:00Z
epic: 2
issue: 14
slug: refresh-generation-and-publication
size: M
status: open
gh_issue: 226
resource: https://github.com/kern-ia/kern-link/issues/226
depends_on: [8]
---

# Generation-check ModelsPublication so a superseded refresh returns false and mutates nothing

## Summary

Numbered 14 rather than inserted after the existing 08: this issue is one half
of a pre-split
([Epic 0 issue 05](/epic-0-plan-remediation/issues/05-pre-split-epic-2-issues-05-and-08.md))
of what was originally Epic 2 issue 08. That issue's own PR size note conceded
its ceiling and named this seam before either PR opened: "the natural cut is
*contract + createProvider overlay* first, *generation/publication concurrency*
second." [Issue 08](/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md)
ships the contract types and a `publish()` with no supersede detection; this
issue makes `publish()` mean what upstream's does:

> **Generation-checked publication.** `setProvider`/`deleteProvider`/
> `clearProviders` supersede an in-flight refresh; a superseded `publish()`
> returns false and mutates nothing. Publications for one provider are
> serialized.

Shipping `Models.Refresh` without this is a real behavioral gap, not a
formality: without it, a refresh racing a `SetProvider` call can persist a
catalog for a provider that no longer exists, or clobber a newer one — exactly
the guarantee `TestPublishReturnsFalseWhenProviderSuperseded` exists to pin
down.

## Scope

- `modelsImpl` — port the generation and publication machinery
  (`refreshGenerations`, `refreshControllers`, `publicationChains` upstream)
  with Go mechanisms: a per-provider generation counter under the existing
  mutex, a `context.CancelFunc` per in-flight refresh, and serialized
  publication per provider. **The observable contract is what must match** —
  superseded publications return false and mutate nothing — not the
  JavaScript mechanism.
- Wire `SetProvider`, `DeleteProvider`, and `ClearProviders` to bump the
  affected provider's generation and cancel its in-flight refresh context,
  so a `publish()` call already in flight observes the bump and returns
  `false` without touching the store or the provider's model list.
- Cancellation: a cancelled `ctx` passed to `Refresh` yields
  `ModelsRefreshResult.Aborted: true` and no partial publication — this is
  `RefreshModelsContext.signal`'s Go equivalent, and it is the second half of
  what `publish()` must check alongside the generation.

## Out of scope

- **Everything the contract itself needs to exist first** — `ModelsPublication`,
  `RefreshModelsContext`, `ModelsRefreshOptions`, `ModelsRefreshResult`, the
  two-phase refresh, and the `createProvider` overlay are
  [issue 08](/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md),
  which this issue depends on and hardens.
- Everything issue 08's own `## Out of scope` already excludes (auth-dependent
  `Models` methods, credential resolution inside refresh, ETag conditional
  fetching, `ModelsRequestTransforms`) — unchanged by this issue.

## Acceptance criteria / Definition of done

- [ ] `TestPublishReturnsFalseWhenProviderSuperseded` — replacing the provider
      via `SetProvider` mid-refresh makes the in-flight `publish` return false
      and leaves both the store and the provider's model list untouched.
- [ ] `TestRefreshCancelledContextReportsAborted` — a cancelled `ctx` yields
      `Aborted: true` and no partial publication.
- [ ] `TestPublicationsSerializedPerProvider` — two concurrent refreshes for the
      same provider do not interleave their `publish()` calls (assert via a
      publish that records call order, or `-race` catching an unsynchronized
      write if the serialization is missing).
- [ ] CI green: `go test ./... -race -v` — the race detector is the point of
      this issue's tests specifically — plus `bash upstream/sync_test.sh` and
      `golangci-lint` v2.12.2.
- [ ] `gofmt -l .` prints nothing; `ai/provider.go` keeps its `// Ports:`
      header.
- [ ] Conventional Commit, e.g.
      `feat(ai): generation-check model refresh publication`.

## Relevant files / areas

- `ai/provider.go` — `modelsImpl` (`:93-344`), `SetProvider` / `DeleteProvider`
  / `ClearProviders` (registry mutation entry points), `Refresh` (`:188`) and
  the `publish()` this issue makes generation-aware.
- `ai/provider_test.go` — existing registry and refresh coverage; the seam for
  the concurrency tests above.
- Upstream: `src/models.ts` at `936aff00` (`refreshGenerations`,
  `refreshControllers`, `publicationChains`, `publishProviderModels`).

## Dependencies

- **Blocked by**: [Issue 08](/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md)
  — hardens the `publish()` contract it ships.
- **Blocks**: Nothing inside this epic.

## PR size note

`M` — ~250 changed lines: a per-provider generation counter, a
`context.CancelFunc` map, and serialized publication per provider, all under
`modelsImpl`'s existing mutex, plus the `SetProvider`/`DeleteProvider`/
`ClearProviders` wiring and three `-race`-sensitive concurrency tests. Real
concurrency work, but no new public types beyond what issue 08 already declared.
Split past ~500; the seam is cancellation reporting (`Aborted: true`) versus
generation checking.
