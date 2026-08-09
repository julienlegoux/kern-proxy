---
type: Decision
title: "Constraints"
description: "The externally imposed and self-imposed limits this program must work inside."
tags: [decision, scope]
timestamp: 2026-08-09T01:30:23Z
phase: scope
decision: 19
slug: constraints
status: decided
verdict: "Constraints as listed, with one change: bump the Go directive from 1.25.0 to 1.26 to align with the other Kern packages, landing in milestone 1 beside the module-path move. New direct dependencies are a hard constraint that reopens a scope decision. develop stays the integration trunk with a single develop->main merge at the end, tagged v0.2.0."
decided_via: discussion
depends_on: []
---

# Question

Checklist area 8. For a brownfield sync the constraints are mostly already
decided and recorded — the value of stating them is that they bound what the
epics may propose, and a few are easy to violate accidentally while porting
TypeScript that has no such rules.

# Options

- **Restate the binding constraints from SPECS.md/CONVENTIONS.md, and name any
  new ones this program introduces.**
- **Assume the epics will read the planning docs.** They will — `create-issues`
  and `implement-issue` both read them — but constraints that a *sync* can
  violate specifically are worth surfacing where the scope is decided.

# Recommendation

**Restate, as a short binding list:**

- **Go 1.25.0**, pinned via `go-version-file: go.mod` in CI. No toolchain bump
  as part of this program. *(Superseded by the verdict — see below.)*
- **No new direct dependencies without an explicit decision.** The dependency
  list in `SPECS.md` is eight entries, each justified. Upstream delegates
  transport to vendor SDKs; the Go port speaks raw `net/http` + `ai/internal/sse`
  everywhere except Bedrock and Vertex ADC. A ported file must not quietly pull
  in an SDK to match upstream's shape.
- **No logger.** `CONVENTIONS.md`: "the library imports no logger at all, and new
  code must not introduce one." Non-fatal problems attach as
  `AssistantMessageDiagnostic`.
- **Tests stay offline and standard-library-only.** `testing` +
  `net/http/httptest`; no `testify`; live tests gate at runtime with `t.Skip`,
  never a build tag; no `t.Parallel()`.
- **Error text stays upstream-verbatim** — load-bearing for the classifiers
  ([07](/scope/07-classifier-error-text-sync.md)).
- **Provenance headers** — every ported file carries `// Ports: <upstream path>`
  after the `package` clause; new code with no upstream counterpart carries none,
  because absence is meaningful.
- **Two-level branch flow** — feature branches → PR → `develop` → PR → `main`,
  merge commits, no rebasing or squashing onto integration branches. Feature
  branches named `issue-<NN>-<slug>`.
- **CI gate** — `go test ./... -race -v`, `bash upstream/sync_test.sh`, and
  `golangci-lint` (pinned v2.12.2) all green before merge. Locally `-race` is
  unavailable (no C toolchain) and `GOTMPDIR` must point inside the repo, so
  **the race detector's verdict only ever arrives from CI** — epics must not
  assume a local green run is sufficient.

No deadline and no budget constraint apply; this is unpaid single-maintainer
work, which is itself a constraint worth stating, because it argues for epics
that are independently shippable rather than a long-lived integration branch.

# Verdict

**The list as written, with one substantive change and two clarifications.**
Decided in discussion, 2026-08-09.

## Changed: Go 1.25.0 → 1.26

The recommendation said "no toolchain bump as part of this program". Reversed:
the other Kern packages are on 1.26, and kern-link should align.

Verified while deciding — the change is one line and carries itself:

- The local toolchain is already `go1.26.3`.
- `.github/workflows/test.yml` pins CI with `go-version-file: go.mod` at both
  job sites, so editing the `go` directive moves the language version **and** CI
  together, with no workflow edit.

Lands in **milestone 1**, beside the module-path move
([decision 26](/scope/26-module-path-migration.md)): both are mechanical
`go.mod` changes that would conflict with every in-flight branch if done later.

## Clarified: new dependencies are a hard constraint

A new direct dependency **reopens a scope decision**; it is not something an
implementer settles and records in `DRIFT.md`.

The reasoning is architectural, not procedural. Raw `net/http` +
`ai/internal/sse` everywhere except Bedrock and Vertex ADC is a deliberate
deviation from upstream, which delegates transport to vendor SDKs. Under a soft
rule, an implementer porting an OAuth flow or the `pi-messages` adapter would
reasonably reach for a library to match the upstream shape, and the architecture
would erode one plausible commit at a time. `DRIFT.md` records a *discovered
blocker*; it is not a channel for ratifying an architectural choice.

This matters more now than when the constraint was drafted, because
[decisions 09](/scope/09-new-oauth-flows.md),
[10](/scope/10-new-provider-bindings.md) and
[13](/scope/13-pi-messages-adapter.md) brought four OAuth flows, the radius
provider and a tenth wire adapter into scope.

## Clarified: integration branch over nine milestones

`develop` stays the integration trunk. Each milestone merges into `develop` as
it greens; **one** `develop → main` merge closes the program, carrying the
v0.2.0 tag ([decision 03](/scope/03-release-and-breaking-strategy.md)).

The invariant kept is "`main` is the last release" — which is how the repository
already behaves, and which matters for a library that must not present untagged
breaking code on its default branch. The objection that the final PR will be
enormous is cosmetic: it is a merge commit over work already reviewed milestone
by milestone.
