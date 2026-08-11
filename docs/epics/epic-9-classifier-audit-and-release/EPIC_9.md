---
type: Epic
title: "Classifier audit, disposition sweep, and release"
description: "Audit the retry and overflow classifiers against upstream, dispose of all 232 files in range, bump the upstream lock, and ship v0.2.0."
tags: [epic]
timestamp: 2026-08-11T20:10:00Z
epic: 9
slug: classifier-audit-and-release
status: open
gh_issue: 126
milestone: 26
resource: https://github.com/kern-ia/kern-link/issues/126
source: docs/planning/SCOPE.md#milestone-9-classifier-audit-disposition-sweep-and-release
---

# Epic 9: Classifier audit, disposition sweep, and release

## Goal

The closing epic. It does the one audit that can only be done after the adapters
have moved, proves the parity claim file by file, and turns the whole program
into a released **v0.2.0** with an upstream lock that says so.

## Scope

- **Classifier audit**, after the adapters have moved — these patterns match
  strings the adapters emit, so auditing earlier audits text about to change.
  The deliverable is a table pairing every regex in `ai/retry.go` and
  `ai/overflow.go` with its upstream counterpart at `936aff00`, every difference
  justified. `retry_test.go` and `overflow_test.go` are already table-driven.
- Settle whether upstream's new `utils/provider-retry.ts` supersedes
  `ai/apis/internal/httpretry` or coexists with it. Settled in writing either
  way — as code where the behaviors converge, or as a drift record promoted to
  `docs/planning/DRIFT.md` at epic close where they diverge deliberately.
- **Disposition sweep**: every one of the **232** files in range is either
  ported or recorded as a deviation in `docs/PORTING.md`. No file left in a
  third, unaccounted state.
- **Disposition checker** (`upstream/disposition_check.sh` and its offline
  test, wired into `.github/workflows/test.yml`): a mechanical arbiter for the
  disposition-sweep criterion above, added because a 232-row table cannot be
  verified complete by inspection. An assumption beyond this epic's original
  scope, authorized here by Epic 0's plan-remediation triage
  (`docs/epics/epic-0-plan-remediation/EPIC_0.md`, `## Notes`), which decided
  to keep it rather than leave the completion gate a judgement call.
- Remove the unused `vX.Y.Z-go.N` tagging convention from `docs/PORTING.md`; it
  contradicts the SemVer line the CHANGELOG maintains and was never used.
- Bump `upstream/UPSTREAM.lock` to `commit=936aff00…` / `version=0.84.1`.
- CHANGELOG entry enumerating every break, then tag **v0.2.0** and merge
  `develop → main`.
- Close issue **#113**.

## Out of scope

- Upstream commits after `936aff00` — the target is frozen; new upstream work is
  the next sync's issue.
- Compatibility shims for the v0.2.0 breaks.
- Enabling the deferred `golangci-lint` battery (revive, gocognit, exhaustive) —
  a project-wide standards change, not a sync change.
- Coverage tooling or thresholds.

## Acceptance criteria

1. The classifier audit table exists, pairing every regex in `ai/retry.go` and
   `ai/overflow.go` with its upstream counterpart at `936aff00`, with every
   difference justified.
2. The `provider-retry.ts` vs `ai/apis/internal/httpretry` question is settled
   in writing — as code, or as a drift record promoted to
   `docs/planning/DRIFT.md` at epic close.
3. All **232** files in range are dispositioned in `docs/PORTING.md`: ported, or
   a deviation with a reason.
4. `upstream/UPSTREAM.lock` reads `commit=936aff00918de1187f085f123c2812d8f2d67745`
   / `version=0.84.1`.
5. The `vX.Y.Z-go.N` convention is gone from `docs/PORTING.md`.
6. CHANGELOG enumerates every break; **v0.2.0** is tagged and `develop` is merged
   into `main`.
7. Issue **#113** is closed.
8. CI green at the tag: `go test ./... -race -v`, `bash upstream/sync_test.sh`,
   `golangci-lint` v2.12.2.

## Dependencies

- [Epic 3: Catalog schema and export tooling](/epic-3-catalog-schema-and-export-tooling/EPIC_3.md)
- [Epic 4: OpenAI-family adapters](/epic-4-openai-family-adapters/EPIC_4.md)
- [Epic 5: Remaining adapters](/epic-5-remaining-adapters/EPIC_5.md)
- [Epic 8: pi-messages and radius](/epic-8-pi-messages-and-radius/EPIC_8.md)

Transitively, every epic in the program.

## Context

- [Technical specs](../../planning/SPECS.md)
- [Conventions](../../planning/CONVENTIONS.md)
- [Upstream sync scope](../../planning/SCOPE.md)
- [Decision 07 — classifier error-text sync](../../planning/scope/07-classifier-error-text-sync.md)
- [Decision 03 — release and breaking strategy](../../planning/scope/03-release-and-breaking-strategy.md)
- [Decision 17 — upstream lock and weekly job](../../planning/scope/17-upstream-lock-and-weekly-job.md)
- [Decision 02 — parity bar](../../planning/scope/02-parity-bar.md)

## Notes

- The program's three completion gates are all proved here: CI green, all 232
  files dispositioned, and the lock reading `936aff00` / `0.84.1`.
- Risk owned here: **silent classifier regressions.** Retry and overflow
  behaviour is matched on error *text*; a changed upstream string with no Go
  counterpart produces a library that stops retrying something it used to, with
  every test green. The audit table is the mitigation.
- Accepted and deliberately unmitigated: `936aff00` will itself be behind by the
  time this epic lands. That is the consequence of freezing the target; the
  weekly job opens the next sync's issue automatically once #113 closes.
- Project-wide: `develop` is the integration trunk and one `develop → main`
  merge closes the program; the race detector's verdict only ever arrives from
  CI.
- **Amended by [epic-0 issue 12](/epic-0-plan-remediation/issues/12-repair-the-epic-9-issue-set.md).**
  The disposition-checker's candidate set moved from a whole-tree enumeration
  to the in-range diff (`issue 03`); its assumption-beyond-this-epic status is
  now stated and authorized above; the classifier row-count criterion was
  corrected from an unreachable 75 to the property it should have stated
  (`issue 01`); a should-have-been-ported finding is now a release blocker
  rather than an open follow-up (`issues 04`, `06`); AC 2's wording above no
  longer implies this epic writes `docs/planning/DRIFT.md` directly; the
  `httpretry.IsRetryable` guarantee is asserted by exactly one issue now
  (`issue 02`); `docs/classifier-parity.md`'s placement in the consumer-facing
  `docs/` bundle is confirmed explicit (`issue 01`); and `issue 05`'s
  `CHANGELOG.md` scope now names the missing `[0.2.0]` footer link (not an
  org-link refresh — Epic 1 issue 02 already owns that, a premise the epic-0
  issue got wrong; see its PR body).
- **Branch, base-branch and status-commit conventions**
  (`docs/planning/CONVENTIONS.md:213-224`). Feature branches are named
  `issue-<NN>-<slug>` (`:216-218`). PRs target `develop` and merge with a merge
  commit — no rebase, no squash (`:213-219`). Each status transition gets its own
  `docs(epics): …` commit, separate from the implementation commit (`:221-224`).
  `Closes #N` will not auto-close the issue, because PRs merge into `develop`
  rather than the repo's default branch — the explicit close at reconcile is the
  normal route, not a fallback.
