# Log

## 2026-08-18

* **Creation**: [DRIFT.md](DRIFT.md), the drift register, at the close of
  [Epic 1](../epics/epic-1-repository-hygiene/EPIC_1.md). First entry promoted:
  the module-path rename's grep criterion excluded the planning and epic bundles
  but not `docs/REPORT_*.md`, whose occurrences of the old path are all
  quotations of those same bundles. Disposition `accepted` — both acceptance
  criteria were amended to add the exclusion rather than the reports rewritten.

## 2026-08-09

* **Creation**: scoped the upstream sync 0.80.3 → 0.84.1 into [SCOPE.md](SCOPE.md)
  over a 26-decision [ledger](scope/index.md) — 22 decided, 4 N/A. Entered through
  `define-change`, which stopped at its size guard: the measured diff is 232 files,
  +19531/−21423 over 207 commits, well past one epic. The target is frozen at
  upstream `936aff00`, and the program cuts into nine milestones.
* **Decision**: the run's governing principle, set by the maintainer and reversing
  three provisional verdicts ([09](scope/09-new-oauth-flows.md),
  [10](scope/10-new-provider-bindings.md), [13](scope/13-pi-messages-adapter.md)):
  a deviation must be justified by *structural non-portability*, never by cost —
  because a deviation is not a postponement but the loss of the Go base the next
  sync's diff must apply against. That brought ~3178 lines (four OAuth flows, the
  radius provider, the pi-messages adapter) back into scope and tightened the
  parity bar in [02](scope/02-parity-bar.md).
* **Note**: two facts surfaced during deep-dive that the intake had not
  anticipated. The repository, its tags and both releases have already transferred
  to `kern-ia/kern-link`, but the Go module path had not — 355 references still
  read `github.com/julienlegoux/kern-link` ([26](scope/26-module-path-migration.md)).
  And the other Kern packages are on Go 1.26 while this one declares 1.25.0
  ([19](scope/19-constraints.md)). Both are mechanical `go.mod` changes that would
  conflict with every in-flight branch, so they pair as milestone 1.

## 2026-08-08

* **Creation**: mapped kern-link at `b70cba3` — wrote [SPECS](SPECS.md) and
  [CONVENTIONS](CONVENTIONS.md) from the manifests, folder tree, source, tests,
  CI workflows, and git history. The sweep produced facts almost everywhere;
  two genuine ambiguities went to the [mapping ledger](mapping/index.md) and
  were both accepted at triage: the planning bundle nests as a sibling OKF root
  under `docs/` rather than folding into the consumer bundle
  ([01](mapping/01-planning-bundle-nesting.md)), and the canonical test command
  is two-tier — `-race` as the CI gate, a `GOTMPDIR`-scoped plain run locally
  ([02](mapping/02-canonical-test-command.md)).
* **Note**: recorded Unknowns rather than guesses — no deployment artifacts (the
  module is consumed with `go get`), no observability wiring in the library by
  design, no coverage configuration, and no visible branch-protection or review
  requirements.
