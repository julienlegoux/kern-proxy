---
type: Decision
title: "Release and breaking-change strategy"
description: "How the breaking core-type changes reach consumers — a breaking release, or compatibility shims."
tags: [decision, scope]
timestamp: 2026-08-09T01:30:23Z
phase: scope
decision: 03
slug: release-and-breaking-strategy
status: decided
verdict: "Breaking v0.2.0, no compatibility shims and no deprecation window — the package has no real consumers and is not ready. Drop the unused vX.Y.Z-go.N tagging convention from PORTING.md in favour of kern-link's own SemVer. One release at the end of the program, carrying both the type breaks and the module-path move."
decided_via: discussion
depends_on: [parity-bar]
---

# Question

The sync is breaking at the core-type level. From `packages/ai/src/types.ts`
(+303) and `models.ts` (+685):

- `StopReason` gains `"pending"` and `"deferred"` — a consumer `switch` over it
  is no longer exhaustive.
- `ThinkingLevel` gains `"max"`.
- `StreamOptions` is refactored to extend a new `ProviderRequestOptions<TModel>`.
- `ModelCost` becomes tiered (`ModelCostRates`, `ModelCostTier`) — `ai.CalculateCost`
  and every catalog entry change shape.
- `models.ts` drops the `AuthModel` export and adds `ModelsPublication`,
  `RefreshModelsContext`, `ModelsRefreshOptions/Result`, `ModelsRequestTransforms`.

kern-link is at **v0.1.1** and claims SemVer (`CHANGELOG.md`, Keep a Changelog
1.1.0). Under SemVer, pre-1.0 minor bumps may break. `docs/PORTING.md` records
that Go tags mirror upstream releases as `vX.Y.Z-go.N`, which sits awkwardly
with the project's own v0.1.1 line — that tension needs resolving here.

# Options

- **Breaking v0.2.0, no shims** — change the types, document the breaks in
  CHANGELOG.md. Honest and cheap; consumers pin the old tag until ready.
- **Compatibility shims** — keep deprecated aliases for the old shapes alongside
  the new. Kinder to consumers, but permanently doubles the surface and
  contradicts PORTING.md's rejection of upstream's own `compat.ts` /
  `legacy-api-aliases.ts` as "deprecated back-compat shims".
- **Hold at v0.1.x, no release until the whole program lands** — one big release
  at the end; no intermediate consumer value.

# Recommendation

**Breaking v0.2.0, no compatibility shims.** Three reasons, all already on
record in this repo:

1. The project is pre-1.0 and explicitly allows breaking minors.
2. `docs/PORTING.md` already refused to port upstream's own back-compat shims as
   deprecated cruft; adding Go-side ones now would be inconsistent.
3. The one prior breaking change (`564a3fe`, unifying messages on pointer
   receivers) was shipped as a plain breaking change with a `refactor(ai)!:`
   commit — there is precedent and it worked.

Additionally: **drop the `vX.Y.Z-go.N` tagging convention** from PORTING.md, or
demote it to a note. It has never actually been used (current tag is v0.1.1) and
it conflicts with the SemVer line the CHANGELOG maintains. Pick one; the
recommendation is to keep kern-link's own SemVer and record the upstream version
in `UPSTREAM.lock` and the changelog entry instead.

Release once at the end of the program, with the CHANGELOG entry enumerating
every break.

# Verdict

**Breaking v0.2.0, no shims, no deprecation window.** Decided in discussion,
2026-08-09.

The maintainer's reasoning supersedes the recommendation's and is stronger than
it: the package has no known real consumers, is not ready to be depended on, and
anyone who has adopted it early accepted that. That removes not just the shims
(option B) but the deprecation-window question entirely — there is no audience
to give a window to.

Also decided:

- **`vX.Y.Z-go.N` is dropped from `docs/PORTING.md`.** It was never used — the
  only tags are `v0.1.0` and `v0.1.1` — and it contradicted the SemVer line the
  CHANGELOG maintains. The upstream version is recorded in `UPSTREAM.lock` and in
  the changelog entry instead.
- **One release, at the end of the program**, in milestone 7 alongside the lock
  bump ([decision 17](/scope/17-upstream-lock-and-weekly-job.md)).

**Scope enlarged during this discussion.** The maintainer raised that the
project's artifacts should sit under the `kern-ia` organisation rather than a
personal account. Checking established that the repository, its tags and both
GitHub releases have *already* transferred to `kern-ia/kern-link`, and that
`julienlegoux/kern-link` redirects — but that the **Go module path has not
moved**: `go.mod` still declares `github.com/julienlegoux/kern-link`, with 355
references across the tracked tree.

That is the artefact that actually matters, because the module path is what a
consumer types in an import. It is now
[decision 26](/scope/26-module-path-migration.md), and v0.2.0 carries it
together with the type breaks — one breaking release, not two.
