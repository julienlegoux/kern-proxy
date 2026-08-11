---
type: Issue
title: "Adopt the six orphaned scope items, or record each as out of scope with a reason"
description: "Give an owning issue to the six pieces of work every epic defers to another epic that disclaims them — or an explicit, reasoned out-of-scope line."
tags: [epic-0]
timestamp: 2026-08-11T20:00:00Z
epic: 0
issue: 02
slug: adopt-the-orphaned-scope-items
size: M
status: done
gh_issue: 200
gh_pr: 219
resource: https://github.com/kern-ia/kern-link/issues/200
depends_on: ["01"]
---

# Adopt the six orphaned scope items, or record each as out of scope with a reason

## Summary

Six pieces of work in this program are deferred to somewhere that does not accept
them. Two are deferred in a circle between epics; three point at an epic whose own
issues disclaim them; one has no owner anywhere. Each one is a scope bullet that will
close green with nothing behind it, and none of them is visible from any single epic —
you only see the gap by following the deferral to the other side and finding it
refused.

This issue closes each one the same way: an owning issue named by number, or a line in
an epic's `## Out of scope` with the reason it is not being done. "Later" and "another
epic" are not outcomes.

Findings repaired: REPORT_2 P2 ×2, REPORT_3 P1 + P2, REPORT_4 P2, REPORT_6 P2. Epic 0
scope item 3; acceptance criterion 3.

## Scope

**1. `ImagesOptions` → `ProviderRequestOptions` (REPORT_3 P1).** Deferred in a circle:
`epic-3 issue 05:76-77` and `epic-3 issue 06:93-96` push it to Epic 2 issue 05, which
pushes it back at `epic-2 issue 05:91-94` ("Touch it only if the refactor breaks
compilation"). It never breaks compilation — `ai/images.Options`
(`ai/images/types.go:96-102`) is a standalone struct that does not embed
`ai.StreamOptions` — so the conditional never fires. This is the only *forced* item of
`docs/planning/scope/14-images-surface.md`. Resolve by adding a new Epic 3 issue
covering `ImagesOptions` on the new base plus the `fetch` override (natural home:
beside issue 06, which already opens `ai/images/openrouter.go`'s transport), and point
`epic-3 issue 05:76-77` at it by number. Rewrite `epic-2 issue 05:91-94` to disclaim it
unconditionally rather than conditionally.

**2. `images-models.ts`'s `getAuth` reshape (REPORT_3 P2).** `EPIC_3.md:37-39` puts it
in scope; `epic-3 issue 05:70-75` defers it to the auth restructure (decision 08,
Epic 6); `docs/planning/scope/08-auth-restructure.md` never mentions images, `EPIC_6.md`
never mentions images, and `epic-6 issue 10:76-78` hands `openrouter-images.ts` back to
Epic 3 by name. **Verify against upstream at `936aff00` first**: if the
`getAuth(providerId | model, overrides)` overload has a Go-visible consequence for
`ai/images/provider.go:213-218`, add it to the new Epic 3 issue from item 1; if it is
TypeScript-only ergonomics, say exactly that in `EPIC_3.md`'s `## Out of scope`, citing
the upstream evidence.

**3. Deferred-response adapter work (REPORT_2 P2).** `epic-2 issue 10:79-82` sends
`FetchDeferred`/`CancelDeferred` implementations to "epics 4 and 5".
`git grep -rniE 'FetchDeferred|CancelDeferred|deferred response' docs/epics/` matches
nothing outside Epic 2 — Epics 4 and 5 cover deferred *tools*, a different capability
that issue 10 itself distinguishes at `:83-84`. Reword `:79-82` to record the gap
honestly: the `Provider`/`Models` deferred-response capability ships proved by `faux`
(Epic 2 issue 12) only, with no real adapter implementation in this program, and say
why that is acceptable — or name the issue that will implement one.

**4. `SimpleStreamOptions.deferred` (REPORT_2 P2).** `epic-2 issue 10:87-91` makes it a
conditionally-in-scope out-of-scope bullet ("port the field here if it costs nothing")
with no fallback owner. Without it, a caller can *fetch* and *cancel* a deferred
response but has no supported way to *request* one — the asymmetry decision 11 exists
to prevent. Move it into issue 10's `## Scope` with a round-trip acceptance criterion
(it is a struct field and a codec case), or name the owning issue.

**5. `src/utils/uuid.ts` (REPORT_4 P2).** `epic-4 issue 12:37-43` places a new
**exported** `ai.UUIDv7` in package `ai` by process of elimination, from an adapter
epic, with no scope decision behind it —
`git grep -n uuid docs/planning/SCOPE.md docs/planning/scope/` is empty. Either add a
one-line bullet to `EPIC_4.md:32-36` accepting it there ("plus `src/utils/uuid.ts` to
`ai/uuid.go`, orphaned by the split"), or move it to an Epic 2 issue and give Epic 4
issue 12 a cross-epic blocker line. Do not leave the ownership recorded only inside
issue 12's Summary.

**6. `docs/auth.md`'s env-key table and resolution-order prose (REPORT_6 P2).**
`epic-6 issue 11:90-91` routes it to Epic 7, and `epic-7 issue 06:24-41` covers only
the *terms-of-service* sections for the four new flows — it will not touch the table.
Meanwhile `epic-6 issue 07:38-44`, `:66-79` puts `ANTHROPIC_AUTH_TOKEN` first in the
precedence list `docs/auth.md:36` states, `epic-6 issue 03:56-64` introduces a per-field
credential→env fallback that contradicts `docs/auth.md:22-24`, and `epic-6 issue 10:24-29`
adds four providers to that table. Bring the table and the resolution-order section
into `epic-6 issue 11`'s `## Scope` (it is already the epic's documentation issue and
already lands last), and delete the Epic 7 pointer at `11:90-91`.

## Out of scope

- **Implementing any of the six.** This issue assigns ownership; the assigned issues do
  the work in their own epics, later.
- **The Epic 4 tool-choice and thinking-budget fields, and `ConstrainedSamplingConfig`**
  — [issue 01](/epic-0-plan-remediation/issues/01-epic-4-core-type-contracts.md) settles
  those and lands first.
- **Renumbering Epic 3's issues.** The new issue from item 1 is appended as Epic 3
  issue 07 with `depends_on: [6]`; issues 01–06 keep their numbers, their slugs and
  their GitHub issues.
- **Epic 6's other ownership gaps** (acceptance criterion 4's catalog half, the
  `test/oauth-auth.test.ts` split, the `depends_on` conflicts) —
  [issue 09](/epic-0-plan-remediation/issues/09-amend-epic-6-and-close-its-gaps.md).
- **`docs/planning/` edits.** Item 2 may *cite* `docs/planning/scope/08-auth-restructure.md`
  and `.../14-images-surface.md`; it does not edit them. The only authorized planning
  edit in this epic is issue 01's.

## Acceptance criteria / Definition of done

- [ ] All six items are resolved, each one either naming an owning issue **by number**
      or appearing in an epic's `## Out of scope` with a stated reason. A grep for the
      six subjects across `docs/epics/` finds no deferral pointing at an epic whose own
      files disclaim it.
- [ ] The new Epic 3 issue exists as
      `docs/epics/epic-3-catalog-schema-and-export-tooling/issues/07-<slug>.md` with
      full frontmatter, a GitHub issue on milestone 20, native sub-issue linkage to
      #120, a bullet in `issues/index.md`, and a `docs/epics/log.md` creation entry.
- [ ] `epic-2 issue 05:91-94` disclaims `ai/images` unconditionally, and `epic-3 issue
      05` points at the new issue by number — the circle is broken in both directions.
- [ ] Items 2 and 5's resolutions each cite upstream evidence at `936aff00` (the file
      and symbol read), recorded in the PR body.
- [ ] `epic-2 issue 10` no longer contains a conditional out-of-scope bullet: the
      `deferred` request field is either in its `## Scope` with a criterion, or owned
      elsewhere by number.
- [ ] `epic-6 issue 11`'s `## Scope` names `docs/auth.md`'s env-key table
      (`docs/auth.md:36`) and its resolution-order prose (`:22-24`), and its
      `## Out of scope` no longer routes them to Epic 7.
- [ ] `timestamp` refreshed on every edited file; every edited issue's GitHub body
      updated to match; `issues/index.md` of each touched epic still agrees with its
      files' frontmatter.
- [ ] `git diff --name-only` lists only files under `docs/`.
- [ ] CI green (the three gates are unaffected by a docs-only change).
- [ ] Branch `issue-<NN>-adopt-the-orphaned-scope-items`, PR targets `develop`, merge
      commit. Commit e.g. `docs(epics): give the six orphaned scope items owners`.

## Relevant files / areas

- `docs/epics/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md:91-94`
  and `.../10-deferred-response-dispatch.md:79-82`, `:83-84`, `:87-91`.
- `docs/epics/epic-3-catalog-schema-and-export-tooling/EPIC_3.md:37-39`,
  `.../issues/05-regenerate-image-catalog.md:70-77`,
  `.../issues/06-images-adapter-retry.md:93-96`, and the new `issues/07-*.md`.
- `docs/epics/epic-4-openai-family-adapters/EPIC_4.md:32-36` and
  `.../issues/12-codex-session-ids-and-continuation-retry.md:37-43`, `:51-59`.
- `docs/epics/epic-6-auth-core-and-env-api-key-bindings/issues/11-porting-paths-and-dispositions.md:90-91`,
  with `.../issues/03-provider-scoped-apikey-resolution.md:56-64`,
  `.../issues/07-anthropic-auth-token.md:38-44`, `:66-79`,
  `.../issues/10-env-api-key-bindings.md:24-29`, `:76-78`.
- `docs/epics/epic-7-four-new-oauth-flows/issues/06-auth-docs-and-porting.md:24-41` —
  read to confirm it does not cover the table.
- Read-only: `ai/images/types.go:96-102`, `ai/images/provider.go:213-218`,
  `ai/images/openrouter.go:239`, `docs/auth.md:22-24`, `:36`,
  `docs/planning/scope/14-images-surface.md`, `docs/planning/scope/08-auth-restructure.md`.

## Dependencies

- **Blocked by**: [Issue 01](/epic-0-plan-remediation/issues/01-epic-4-core-type-contracts.md)
  — both edit `EPIC_4.md` and Epic 2 issue 05, and issue 01 settles the field names
  item 5's resolution has to sit beside.
- **Blocks**: [Issue 04](/epic-0-plan-remediation/issues/04-repair-the-epic-2-issue-set.md)
  and [issue 05](/epic-0-plan-remediation/issues/05-pre-split-epic-2-issues-05-and-08.md)
  (both re-edit Epic 2 issues 05 and 10),
  [issue 06](/epic-0-plan-remediation/issues/06-rein-in-epic-3-invented-scope.md) (the
  new Epic 3 issue changes that epic's issue set),
  [issue 07](/epic-0-plan-remediation/issues/07-repair-the-epic-4-issue-set.md) and
  [issue 09](/epic-0-plan-remediation/issues/09-amend-epic-6-and-close-its-gaps.md).
- **External**: upstream at `936aff00` for items 2 and 5.

## PR size note

`M` — ~200–450 changed lines: a new ~120-line Epic 3 issue file plus edits across eight
existing files. Split past ~500. The clean cut if it overruns is (the new Epic 3 issue
+ items 1–2) and (items 3–6, which are all reworded bullets).
