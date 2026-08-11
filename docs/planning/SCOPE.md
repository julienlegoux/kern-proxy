---
type: Scope
title: "kern-link — Upstream sync 0.80.3 → 0.84.1"
description: "Bringing the Go port to full parity with @earendil-works/pi-ai v0.84.1, shipped as a breaking v0.2.0 under the kern-ia module path."
tags: [planning, scope]
timestamp: 2026-08-10T09:20:00Z
status: final
---

# kern-link — Upstream sync 0.80.3 → 0.84.1

Scope for the program that brings kern-link from upstream `@earendil-works/pi-ai`
**v0.80.3** to **v0.84.1**, in full parity, released as a breaking **v0.2.0**.

The upstream target is **frozen at `936aff00`**
([decision](/scope/01-sync-target-and-freeze.md)) — commit
`936aff00918de1187f085f123c2812d8f2d67745`, measured 2026-08-09. Every milestone
below is defined against that revision and no other.

The full decision ledger, with the audit facts behind each verdict, is in
[`scope/`](scope/index.md).

## Problem

kern-link exists to be a full-parity Go port of `pi-ai` that tracks upstream over
time. It has fallen four upstream minor versions behind: the pin in
`upstream/UPSTREAM.lock` is `244f1deaf1ae` (v0.80.3), while upstream is at
v0.84.1. The gap measures **232 files, +19531/−21423, across 207 commits** in
`packages/ai` — 29 new source files and 3 deleted.

That gap is not only large, it is *structural*. Upstream changed the core type
surface every adapter compiles against, added two capabilities with no Go
counterpart, reorganised its auth tree, and changed the model-catalog schema. A
port cannot absorb those incrementally by accident; they have to be planned.

GitHub issue **#113** is the umbrella tracking issue for this program and stays
open for its duration ([decision](/scope/17-upstream-lock-and-weekly-job.md)).

## Users

Unchanged by this program: Go developers consuming the library with `go get`,
with future Kern Lx as the driving downstream consumer, and the maintainer as
first user ([decision](/scope/23-target-users.md)). The delivery form is
unchanged — a Go module, no service, no deployed artifact
([decision](/scope/24-delivery-form.md)).

## Goals & success criteria

The program is done when all three gates pass
([decision](/scope/02-parity-bar.md)):

1. **CI green** — `go test ./... -race -v`, `bash upstream/sync_test.sh`, and
   `golangci-lint` (pinned v2.12.2).
2. **Every one of the 232 upstream files in range is dispositioned** — either
   ported, or listed in `docs/PORTING.md` as a deviation with a reason. No file
   left in a third, unaccounted state.
3. **`upstream/UPSTREAM.lock` reads `commit=936aff00…` / `version=0.84.1`** —
   the machine-checkable claim that the sync happened.

### The governing principle

One rule decided during this run governs everything else, and is the reason the
program is as large as it is:

> **A deviation must be justified by structural non-portability, never by cost.**

A deviation is not a postponement — it is the loss of the Go base that the *next*
sync's diff has to apply against. If a file is not ported now, the next sync
reports "this file changed" with nothing to change, converting a one-time cost
into permanent re-derivation against a moved target. Repeated across a few syncs,
`sync.sh` produces noise the maintainer learns to ignore, and the port stops
being a port.

Admissible grounds for a deviation: constructs with no meaning in Go. Not
admissible: "large", "hard to test", "no consumer asked for it". This reversed
three provisional decisions during scoping ([09](/scope/09-new-oauth-flows.md),
[10](/scope/10-new-provider-bindings.md),
[13](/scope/13-pi-messages-adapter.md)) and brought ~3178 lines back into scope.

## Non-goals (out of scope)

([decision](/scope/18-non-goals.md))

1. **Upstream commits after `936aff00`.** The target is frozen; new upstream work
   is the next sync's issue.
2. **Bundler and runtime shims** — `*.lazy.ts`, `bun-oauth.ts` (a Bun entry
   point), `compat/*` (back-compat surface upstream itself treats as deprecated).
   These are the only admissible kind of exclusion.
3. **Compatibility shims for the v0.2.0 breaks.**
4. **New direct dependencies** — a hard constraint; one would reopen a scope
   decision rather than be absorbed by an epic.
5. **Enabling the deferred golangci-lint battery** (revive, gocognit,
   exhaustive). A project-wide standards change, not a sync change.
6. **Coverage tooling or thresholds.**
7. **Refactoring the testbed, the CLI, or `docs/` beyond what the sync forces.**
8. **Idiomatic-Go cleanups of ported code** — upstream's shape outranks Go idiom
   in ported files, and a sync is the worst time to relitigate that.

## Constraints

([decision](/scope/19-constraints.md))

- **Go 1.26** — bumped from 1.25.0 to align with the other Kern packages. CI
  pins from `go-version-file: go.mod`, so the directive carries the workflow.
- **No new direct dependencies.** Raw `net/http` + `ai/internal/sse` everywhere
  except Bedrock (`aws-sdk-go-v2`) and Vertex ADC is a deliberate deviation from
  upstream's SDK delegation, and it erodes one plausible commit at a time under a
  soft rule.
- **No logger.** Non-fatal problems attach as `AssistantMessageDiagnostic`.
- **Tests stay offline and standard-library-only** — `testing` +
  `net/http/httptest`, no `testify`, no build tags, no `t.Parallel()`. Live tests
  gate at runtime with `t.Skip`.
- **Error text stays upstream-verbatim** — `ai/retry.go` and `ai/overflow.go`
  classify failures by matching it.
- **Provenance headers** — `// Ports:` on every ported file; none on original
  code, because absence is meaningful.
- **`develop` is the integration trunk.** Each milestone merges into `develop` as
  it greens; one `develop → main` merge closes the program with the v0.2.0 tag.
- **The race detector's verdict only ever arrives from CI** — no C toolchain
  locally, and `GOTMPDIR` must stay inside the repo. A green local run is not
  sufficient evidence.

## Milestone 1: Repository hygiene

Mechanical, no semantic change, first because it conflicts with every in-flight
branch ([decision](/scope/26-module-path-migration.md)).

- Move the module path from `github.com/julienlegoux/kern-link` to
  `github.com/kern-ia/kern-link` — `go.mod`, every import, README, CHANGELOG,
  `docs/`. **355 references.**
- Bump the `go` directive from 1.25.0 to 1.26.
- No `/v2` suffix: the project is v0.x, so the major-version suffix rule does not
  apply. Consumers pinned to `github.com/julienlegoux/kern-link@v0.1.1` keep
  working — module versions are immutable in proxy.golang.org.
- Commit as `refactor!: move module path to github.com/kern-ia/kern-link`.
- Historical CHANGELOG entries for already-released versions are **not**
  rewritten.

## Milestone 2: Core types and Models contracts

The foundation every other file compiles against
([decision](/scope/04-core-types-migration.md)).

- `types.ts` (+303) and `models.ts` (+685): the `ProviderRequestOptions<TModel>`
  refactor, tiered `ModelCost`/`ModelCostRates`/`ModelCostTier`, `StopReason`
  gaining `pending`/`deferred`, `ThinkingLevel` gaining `max`, `FetchFunction`,
  `SessionAffinityFormat`, `JsonValue`, `BedrockCompat`, the `ModelsPublication`
  / `RefreshModelsContext` / `ModelsRefreshOptions` / `ModelsRequestTransforms`
  contracts, and the removal of the `AuthModel` export.
- New core modules `model-catalog.ts` and `models-store.ts`; their Go mapping is
  an implementer call, but `docs/PORTING.md` records the outcome either way.
- `simple-options` and `lazy` land here, not with the adapters — every adapter
  compiles against them.
- **Deferred tools in full** ([decision](/scope/11-deferred-tools.md)):
  `utils/deferred-tools.ts`, `DeferredHandle`, the deferred fetch/cancel options,
  and the ported test. The type half is already mandatory, so shipping the enum
  values without the behaviour would leave `StopReason` cases the library can
  never emit. Prove the new stop reasons end to end through the `faux` provider.
- **Acceptance criterion, easy to miss:** a manual grep sweep for `StopReason`
  switch sites. `exhaustive` is deliberately absent from `.golangci.yml`, so
  neither compiler nor linter will flag a switch that silently stops being
  exhaustive.

## Milestone 3: Catalog schema and export tooling

`sync.sh` buckets these as "re-run `tools/export-catalog` only" — that label is
wrong for this sync, and treating it as right is the main trap
([decision](/scope/05-catalog-schema-and-tooling.md)).

- **First issue is a tooling spike**: confirm `tools/export-catalog` runs green
  against upstream at `936aff00` at all. Five generator scripts changed upstream
  (`generate-models.ts`, `check-model-data.ts`, `generate-image-models.ts`,
  `model-data.ts`, `models-dev-reasoning-options.ts`). Price this before planning
  the rest of the milestone.
- Then, in order: update `tools/export-catalog` → regenerate all catalog JSON →
  update `ai/catalog` validation → run the catalog tests. Code before data: the
  Go structs that read the catalog ship in milestone 2, and the regenerated tree
  fails validation at load otherwise.
- 41 `*.models.ts` plus `models.generated.ts` and `image-models.generated.ts`.
- **Images surface** rides along ([decision](/scope/14-images-surface.md)):
  `images-models.ts`, the OpenRouter images adapter, and the regenerated image
  catalog — same tooling, so a separate epic would touch it twice. Any change to
  the OpenRouter images adapter gets an offline `httptest` case, because its only
  existing test is an env-gated live smoke.

## Milestone 4: OpenAI-family adapters

These four move together or the shared layer is ported twice
([decision](/scope/06-adapter-updates.md)).

- `openai-completions.ts` (+396) → `ai/apis/openaicompletions`
- `openai-responses.ts` and `openai-responses-shared.ts` (+412) →
  `ai/apis/openairesponses` — the shared core beneath Azure and Codex, whose
  converters `docs/PORTING.md` records as exported for exactly that reason
- `azure-openai-responses.ts` → `ai/apis/azure`
- `openai-codex-responses.ts` (+220) → `ai/apis/codex`
- **Constrained sampling** ([decision](/scope/12-constrained-sampling.md)):
  `api/constrained-sampling.ts` (+148), `GrammarFormat`, `GrammarVariants`,
  `ConstrainedSamplingConfig`. The formats are OpenAI-specific and the request
  builders are being rewritten here anyway. `ConstrainedSamplingConfig` lands on
  `ai.Tool`, **not** in `ai.StreamOptions`: upstream at `936aff00` declares
  `constrainedSampling?: false | ConstrainedSamplingConfig` on the `Tool`
  interface (`packages/ai/src/types.ts:506`), and `StreamOptions` (`:175-219`)
  carries no such field — it is per tool, not per request, so no
  per-adapter-options deviation is needed. Corrected 2026-08-10 against the
  frozen upstream ref; the original text is the one
  [decision 12](/scope/12-constrained-sampling.md) carried before the shape was
  read.

## Milestone 5: Remaining adapters

Mutually independent, so issue-level parallel within the epic
([decision](/scope/06-adapter-updates.md)).

- `anthropic-messages.ts` (+252) → `ai/apis/anthropic`
- `google-generative-ai.ts`, `google-shared.ts`, `google-vertex.ts` →
  `ai/apis/google`, `ai/apis/google/vertex`
- `mistral-conversations.ts` (+423) → `ai/apis/mistral`
- `bedrock-converse-stream.ts` (+146) → `ai/apis/bedrock`
- **The Copilot dynamic-headers gap** ([decision](/scope/16-copilot-headers-gap.md)):
  `src/api/github-copilot-headers.ts` (`X-Initiator`, `Copilot-Vision-Request`),
  unported since the initial port and logged at `docs/PORTING.md:46`. It is
  outside the 0.80.3→0.84.1 delta, but it is the same failure mode the governing
  principle names — an upstream file with no Go base, in files this milestone
  opens anyway. Check whether it changed in range before porting it.
- `cloudflare-stream.ts` needs a one-line classification here: it is Cloudflare
  streaming support, not a provider binding, and probably belongs beside
  `cloudflare_auth.go`.

## Milestone 6: Auth core and env-API-key bindings

([decision](/scope/08-auth-restructure.md),
[decision](/scope/10-new-provider-bindings.md))

- Port the content of `auth/credential-store.ts`, `auth/helpers.ts`,
  `auth/resolve.ts`, `auth/types.ts`, `env-api-keys.ts`, `oauth.ts`.
- **No Go package moves.** Upstream moved `src/utils/oauth/*` to
  `src/auth/oauth/*`; kern-link's `ai/auth/oauth` already sits at that
  destination, a call the port made independently and earlier.
- **Update `docs/PORTING.md`'s upstream paths.** The mapping table is what the
  whole sync procedure navigates by, and leaving it pointing at the deleted
  `src/utils/oauth/*` would break the next sync's ability to resolve `sync.sh`
  output against Go packages.
- Four new env-API-key bindings, each an `envApiKeyAuth` one-liner plus its
  catalog entry: `baseten`, `qwen-token-plan`, `qwen-token-plan-cn`,
  `qwen-token-plan-individual`.

## Milestone 7: Four new OAuth flows

([decision](/scope/09-new-oauth-flows.md))

- `radius` (403), `openrouter` (311), `kimi-coding` (310), `xai` (239) — 1263
  lines, on top of the three flows already ported.
- Upstream ships **1056 lines of tests** with them (`radius-oauth` 129,
  `openrouter-oauth` 322, `kimi-coding-oauth` 270, `xai-oauth` 335), so the
  offline scaffolding is a porting job rather than an invention.
- Each flow gets the `docs/auth.md` terms-of-service treatment `SPECS.md`
  requires: subscription OAuth makes kern-link present as a first-party client,
  which is fine for personal use and a real risk to ship in a product. That
  framing is load-bearing, not boilerplate.
- All four providers declare both an API key and OAuth at `936aff00`, so these
  add convenience over a path that already works — they are ported for base
  continuity, not access.
- Wire into `cmd/pi-ai login`.

## Milestone 8: pi-messages and radius

([decision](/scope/13-pi-messages-adapter.md),
[decision](/scope/10-new-provider-bindings.md))

- `api/pi-messages.ts` (433 + 248 test lines) becomes the **tenth** package under
  `ai/apis/`, with its `ai.Api` constant and registration. It is pi's own
  protocol — a POST of `{model, context, options}` to `<baseUrl>/messages`, SSE
  back — so it maps almost one-to-one onto types kern-link already has.
- `providers/radius.ts` (82) and `radius-config.ts` (96). Radius fetches its
  gateway config at provider setup, a network dependency none of the existing 35
  bindings has; that is the design point to get right.
- Depends on milestone 7's radius OAuth flow.

## Milestone 9: Classifier audit, disposition sweep, and release

([decision](/scope/07-classifier-error-text-sync.md),
[decision](/scope/03-release-and-breaking-strategy.md),
[decision](/scope/17-upstream-lock-and-weekly-job.md))

- **Classifier audit**, after the adapters have moved — these patterns match
  strings the adapters emit, so auditing earlier audits text about to change. The
  acceptance criterion is a table pairing every regex in `ai/retry.go` and
  `ai/overflow.go` with its upstream counterpart at `936aff00`, every difference
  justified. `retry_test.go` and `overflow_test.go` are already table-driven.
- Settle whether upstream's new `utils/provider-retry.ts` supersedes
  `ai/apis/internal/httpretry` or coexists with it. If they diverge, that is a
  `docs/planning/DRIFT.md` entry, not a silent difference.
- **Disposition sweep**: every one of the 232 files ported or recorded as a
  deviation in `docs/PORTING.md`.
- Remove the unused `vX.Y.Z-go.N` tagging convention from `docs/PORTING.md`; it
  contradicts the SemVer line the CHANGELOG maintains and was never used.
- Bump `upstream/UPSTREAM.lock` to `commit=936aff00…` / `version=0.84.1`.
- CHANGELOG entry enumerating every break, then tag **v0.2.0** and merge
  `develop → main`.
- Close issue **#113**.

## Risks & assumptions

([decision](/scope/21-risks-and-assumptions.md))

**Risks**, each owned by the milestone that mitigates it:

1. **`tools/export-catalog` may not run against `936aff00` at all** — five
   generator scripts changed upstream. *Milestone 3's first issue prices this
   before the rest is planned.*
2. **Silent classifier regressions** — retry and overflow behaviour is matched on
   error *text*; a changed upstream string with no Go counterpart produces a
   library that stops retrying something it used to, with every test green.
   *Milestone 9's audit table.*
3. **Non-exhaustive `StopReason` switches** — no compiler or linter error will
   appear. *Milestone 2's manual grep sweep.*
4. **A long-lived integration branch** — nine milestones with nothing releasable
   until the last. *Merge each milestone into `develop` as it greens; `main`
   waits.*
5. **Upstream keeps moving** — `936aff00` will itself be behind by milestone 9.
   *Accepted, not mitigated: the deliberate consequence of freezing the target.
   The weekly job opens the next issue automatically once #113 closes.*
6. **Radius's runtime gateway-config fetch** is a network dependency at provider
   setup that no existing binding has. *Milestone 8 designs for it explicitly
   rather than discovering it.*

**Assumptions**

- `936aff00` is a coherent upstream state, not mid-refactor. Not verified beyond
  it being `origin/main` at a point in time.
- Upstream's suite passes at `936aff00`; the ported tests inherit its
  correctness.
- No downstream consumer is pinned in a way that makes a breaking v0.2.0 costly —
  the maintainer's explicit position is that the package has no real consumers
  and is not ready to be depended on.
