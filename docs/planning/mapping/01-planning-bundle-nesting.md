---
type: Decision
title: "Planning bundle nesting under docs/"
description: "docs/ is already an OKF bundle root — does docs/planning/ get its own root index.md and log.md, or fold into the existing ones?"
tags: [decision, mapping]
timestamp: 2026-07-29T04:36:27Z
phase: mapping
decision: 01
slug: planning-bundle-nesting
status: decided
verdict: "A — nested sibling bundle: docs/planning/ carries its own index.md and log.md, pointed at from docs/index.md"
decided_via: triage
depends_on: []
---

# Question

`docs/index.md` already declares `okf_version: "0.1"` and `docs/log.md` already
exists — the repository's public documentation *is* an OKF bundle whose root is
`docs/`. The map-codebase deliverables belong at `docs/planning/SPECS.md` and
`docs/planning/CONVENTIONS.md`, and the OKF format wants a bundle root to carry
`index.md` + `log.md`.

Writing those two files at `docs/planning/` produces a second bundle root nested
inside the first. Which layout does this repo adopt?

# Options

**A — Nested sibling bundle (skill default).** `docs/planning/index.md` carries
`okf_version: "0.1"`, `docs/planning/log.md` records mapping/planning history.
`docs/index.md` gains one line pointing at the planning bundle.

- Planning history stays separate from consumer-facing documentation history —
  `docs/log.md` keeps reading as a changelog of the *docs*, not of the process.
- Every downstream skill (`define-change`, `create-issues`, `implement-issue`)
  finds exactly the paths it hardcodes, with no special-casing.
- Cost: two `okf_version` roots in one tree. OKF does not forbid this, but a
  validator run from `docs/` sees a nested root.

**B — Fold into the existing bundle.** No `docs/planning/index.md` or
`docs/planning/log.md`; `SPECS.md`/`CONVENTIONS.md` are listed in
`docs/index.md` and their history appends to `docs/log.md`.

- One bundle root, unambiguously valid.
- Cost: process artifacts (decision ledgers, epic tracking, per-issue status
  churn) start writing into the bundle the *library's users* read. The repo
  deliberately removed dev-process docs from the shipped module in v0.1.1
  ("Dev-process planning documents no longer ship in the module"), which is
  evidence against mixing the two.

**C — Planning bundle outside docs/.** e.g. `planning/` at the repo root.

- Cleanest separation, no nesting at all.
- Cost: every downstream skill hardcodes `docs/planning/`, so this breaks the
  whole brownfield chain. Not viable without patching four skills.

# Recommendation

**A — nested sibling bundle.** The v0.1.1 changelog entry establishes an
explicit project intent to keep dev-process documents out of the consumer docs
surface; option B reverses that decision six weeks later. Nesting costs only a
second `okf_version` declaration, and it keeps `docs/log.md` honest as a
documentation changelog while `docs/planning/log.md` carries planning history.
Add one pointer line in `docs/index.md` so the planning bundle is discoverable
rather than hidden.

# Verdict

**A — nested sibling bundle.** `docs/planning/index.md` declares
`okf_version: "0.1"` and `docs/planning/log.md` carries planning history;
`docs/index.md` gains one pointer line so the bundle is discoverable. Two OKF
roots in one tree is the accepted cost — it keeps `docs/log.md` a changelog of
the consumer-facing documentation, and honors the v0.1.1 decision to keep
dev-process artifacts out of what ships in the module.

Accepted at triage, 2026-08-08.
