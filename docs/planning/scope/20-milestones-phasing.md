---
type: Decision
title: "Milestones and phasing"
description: "How the sync splits into independently shippable milestones — the headings split-epics cuts on."
tags: [decision, scope]
timestamp: 2026-08-09T01:30:23Z
phase: scope
decision: 20
slug: milestones-phasing
status: decided
verdict: "Nine milestones: repository hygiene; core types + deferred tools; catalog + images; OpenAI-family adapters + constrained sampling; remaining adapters + copilot headers; auth core + env-key bindings; four OAuth flows; pi-messages + radius; classifier audit + disposition + release."
decided_via: discussion
depends_on: [core-types-migration, catalog-schema-and-tooling, adapter-updates, classifier-error-text-sync, auth-restructure, new-provider-bindings, deferred-tools, constrained-sampling, images-surface, copilot-headers-gap]
---

# Question

Checklist area 10, and the load-bearing decision of this whole run: `split-epics`
detects epic boundaries from `## Milestone N:` headings in `SCOPE.md`, so this
verdict *is* the epic list.

The constraints on the cut come from the audit's dependency structure:

- Nothing compiles against 0.84.1 shapes until the core types move, so that is
  unavoidably first.
- The Go structs that read the catalog live in `ai`, so the catalog schema
  migration follows the core types, not the reverse.
- `openai-responses-shared` is the shared core beneath the Azure and Codex
  adapters, so those four move together or the shared layer is ported twice.
- The classifiers match strings the adapters emit, so they are audited after the
  adapters move.
- The lock, CHANGELOG, and release are a single terminal act.

# Options

- **Six milestones, dependency-ordered** (below).
- **Fewer, larger milestones** — e.g. "core + catalog", "all adapters",
  "release". Each would exceed what `create-issues` can cut into PR-sized issues
  without overload; the adapter one alone is >2000 upstream lines.
- **One milestone per upstream area** (~9, mirroring `sync.sh`'s buckets) — finer
  than needed, and it would split the OpenAI family across milestones, which the
  shared-core dependency forbids.

# Recommendation

**Nine milestones.** *(Revised 2026-08-09 from an original six: milestone 1 is
new per [decision 26](/scope/26-module-path-migration.md) and
[19](/scope/19-constraints.md), and milestones 7–8 are new because decisions
[09](/scope/09-new-oauth-flows.md), [10](/scope/10-new-provider-bindings.md) and
[13](/scope/13-pi-messages-adapter.md) reversed to full porting.)*

1. **Repository hygiene** — move the module path to
   `github.com/kern-ia/kern-link` (355 references) and bump the `go` directive
   from 1.25.0 to 1.26. Mechanical, no semantic change, first because it
   conflicts with every in-flight branch
   ([26](/scope/26-module-path-migration.md), [19](/scope/19-constraints.md)).
2. **Core types and Models contracts** — `types.ts`/`models.ts`, the
   `ProviderRequestOptions` refactor, tiered `ModelCost`, new `StopReason` and
   `ThinkingLevel` values, `model-catalog.ts`/`models-store.ts`,
   `simple-options`, `lazy`, and **deferred tools** in full
   ([04](/scope/04-core-types-migration.md), [11](/scope/11-deferred-tools.md)).
3. **Catalog schema and export tooling** — update `tools/export-catalog`,
   regenerate all catalog JSON to the tiered-cost schema, update `ai/catalog`
   validation, and bring the **images surface** along
   ([05](/scope/05-catalog-schema-and-tooling.md), [14](/scope/14-images-surface.md)).
4. **OpenAI-family adapters** — `openaicompletions`, `openairesponses`, `azure`,
   `codex`, plus **constrained sampling**
   ([06](/scope/06-adapter-updates.md), [12](/scope/12-constrained-sampling.md)).
5. **Remaining adapters** — `anthropic`, `google` + `vertex`, `mistral`,
   `bedrock`, plus the **Copilot dynamic-headers gap**
   ([06](/scope/06-adapter-updates.md), [16](/scope/16-copilot-headers-gap.md)).
6. **Auth core and env-API-key bindings** — the `auth/*` content changes,
   `env-api-keys`, `oauth.ts`, PORTING.md path updates, and `baseten` +
   `qwen-token-plan` ×3
   ([08](/scope/08-auth-restructure.md), [10](/scope/10-new-provider-bindings.md)).
7. **Four new OAuth flows** — `radius`, `openrouter`, `kimi-coding`, `xai`
   (1263 lines plus 1056 lines of upstream tests), each with its `docs/auth.md`
   terms-of-service treatment ([09](/scope/09-new-oauth-flows.md)).
8. **pi-messages and radius** — the tenth `ai/apis` package (433 lines + 248 of
   tests), the `radius` binding (82) and `radius-config` (96). Needs milestone 7's
   radius OAuth flow ([13](/scope/13-pi-messages-adapter.md),
   [10](/scope/10-new-provider-bindings.md)).
9. **Classifier audit, disposition sweep, and release** — the retry/overflow
   diff-level audit, the PORTING.md disposition of all 232 files, the
   `UPSTREAM.lock` bump, the CHANGELOG entry, and the v0.2.0 tag
   ([07](/scope/07-classifier-error-text-sync.md), [02](/scope/02-parity-bar.md),
   [03](/scope/03-release-and-breaking-strategy.md),
   [17](/scope/17-upstream-lock-and-weekly-job.md)).

Ordering constraints, tightest first: 1 before everything (it rewrites every
import). 2 before 3 (the Go structs that read the catalog ship in 2). 7 before 8
(radius needs its OAuth flow). 9 strictly last. Milestones 4, 5, 6 and 7 are
mutually independent once 2 and 3 land, so they may run in any order or overlap.

One honest caveat: only milestone 9 is "shippable" in the release sense — the
breaking changes mean there is nothing to tag until the end. What 1–8 each
deliver is a green `develop` with a coherent slice ported, which is the practical
unit for a single maintainer.

# Verdict

**Nine milestones, as revised above.** Accepted at triage 2026-08-09 as a
six-milestone cut, then **refreshed after its inputs landed** — which is why
this doc was held open through the deep-dives instead of being finalized on the
triage accept. It is the ledger's most consequential document, because
`split-epics` turns these headings into the epic list verbatim.

Three changes between the accepted cut and this verdict:

- **New milestone 1 (repository hygiene).** The module-path move
  ([26](/scope/26-module-path-migration.md)) and the Go 1.26 bump
  ([19](/scope/19-constraints.md)) both surfaced during deep-dive. They pair
  naturally — two mechanical `go.mod` changes — and go first because the rename
  touches 355 references and would conflict with every in-flight branch. Numbered
  from 1 rather than added as a "Milestone 0", which would read as an off-by-one
  to `split-epics` and to every later reader.
- **New milestones 7 and 8.** The full-parity reversal
  ([09](/scope/09-new-oauth-flows.md), [10](/scope/10-new-provider-bindings.md),
  [13](/scope/13-pi-messages-adapter.md)) brought ~3178 lines back into scope.
  They are two milestones rather than one because 8 depends on 7: the `radius`
  binding needs its OAuth flow before it can work.
- **Unchanged contents** for the original six, now numbered 2–6 and 9.

The caveat carried over from the accepted version still holds and is worth
keeping in `SCOPE.md`: only the last milestone is shippable in the release
sense.
