---
type: Epic
title: "Four new OAuth flows"
description: "Port the radius, openrouter, kimi-coding and xai OAuth flows with their upstream tests, their terms-of-service documentation, and CLI login wiring."
tags: [epic]
timestamp: 2026-08-11T20:10:00Z
epic: 7
slug: four-new-oauth-flows
status: open
gh_issue: 124
milestone: 24
resource: https://github.com/kern-ia/kern-link/issues/124
source: docs/planning/SCOPE.md#milestone-7-four-new-oauth-flows
---

# Epic 7: Four new OAuth flows

## Goal

Four OAuth flows upstream added that kern-link has no Go base for. They are
ported for **base continuity**, not access: all four providers declare both an
API key and OAuth at `936aff00`, so these add convenience over a path that
already works. Skipping them would leave the next sync reporting "these files
changed" with nothing to change them against.

## Scope

- `radius` (403 lines), `openrouter` (311), `kimi-coding` (310), `xai` (239) —
  **1263 lines**, on top of the three flows already ported.
- The upstream tests that ship with them — **1056 lines**: `radius-oauth` 129,
  `openrouter-oauth` 322, `kimi-coding-oauth` 270, `xai-oauth` 335. The offline
  scaffolding is therefore a porting job rather than an invention.
- **`docs/auth.md` terms-of-service treatment for each flow**, as `SPECS.md`
  requires: subscription OAuth makes kern-link present as a first-party client,
  which is fine for personal use and a real risk to ship in a product. That
  framing is load-bearing, not boilerplate.
- Wire each flow into `cmd/pi-ai login`.

## Out of scope

- The auth core these flows sit on — epic 6.
- The `radius` **provider binding** and its gateway config, which depend on this
  epic's radius flow — epic 8. Exception:
  [issue 05](/epic-7-four-new-oauth-flows/issues/05-cli-login-new-flows.md)
  declares a single hardcoded `defaultRadiusGateway` constant in
  `cmd/pi-ai/oauth.go` — not a configuration surface — so this epic's own
  acceptance criterion 4 can be met without epic 8; the comment there names
  epic 8 as its removal trigger.
- Compatibility shims for the v0.2.0 breaks.

## Acceptance criteria

1. All four flows are ported and match upstream at `936aff00`.
2. The four upstream test files are ported and pass offline — no live-gated-only
   coverage for the flows themselves.
3. `docs/auth.md` carries the terms-of-service framing for each of the four
   flows, in the form `SPECS.md` requires — not a generic note.
4. `cmd/pi-ai login` offers each of the four flows and completes them.
5. `// Ports:` headers on every ported file; each file dispositioned in
   `docs/PORTING.md`.
6. CI green: `go test ./... -race -v`, `bash upstream/sync_test.sh`,
   `golangci-lint` v2.12.2.

## Dependencies

- [Epic 6: Auth core and env-API-key bindings](/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md)

Blocks [Epic 8: pi-messages and radius](/epic-8-pi-messages-and-radius/EPIC_8.md),
which needs the radius flow.

## Context

- [Technical specs](../../planning/SPECS.md)
- [Conventions](../../planning/CONVENTIONS.md)
- [Upstream sync scope](../../planning/SCOPE.md)
- [Decision 09 — new OAuth flows](../../planning/scope/09-new-oauth-flows.md)

## Notes

- This epic is one of three whose provisional "defer it" verdict was **reversed**
  during scoping by the governing principle: **a deviation must be justified by
  structural non-portability, never by cost.** Cost was the only argument
  against porting these.
- Project-wide, not this epic's own boundary: no new direct dependencies, no
  logger, tests offline and stdlib-only (`testing` + `net/http/httptest`, no
  `testify`, no build tags, no `t.Parallel()`); live tests gate at runtime with
  `t.Skip`.
- `## Out of scope`'s radius provider-binding bullet carries one narrow
  exception, added by [Epic 0 issue 10](/epic-0-plan-remediation/issues/10-repair-the-epic-7-issue-set.md):
  [issue 05](/epic-7-four-new-oauth-flows/issues/05-cli-login-new-flows.md)
  declares a single hardcoded gateway constant so acceptance criterion 4 is
  meetable without epic 8.
- Upstream target frozen at `936aff00`.
- **Branch, base-branch and status-commit conventions**
  (`docs/planning/CONVENTIONS.md:213-224`). Feature branches are named
  `issue-<NN>-<slug>` (`:216-218`). PRs target `develop` and merge with a merge
  commit — no rebase, no squash (`:213-219`). Each status transition gets its own
  `docs(epics): …` commit, separate from the implementation commit (`:221-224`).
  `Closes #N` will not auto-close the issue, because PRs merge into `develop`
  rather than the repo's default branch — the explicit close at reconcile is the
  normal route, not a fallback.
