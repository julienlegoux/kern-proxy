---
type: Decision
title: "Pre-existing github-copilot-headers gap"
description: "Whether the known unported Copilot dynamic-headers gap is closed as part of this program."
tags: [decision, scope]
timestamp: 2026-08-09T01:30:23Z
phase: scope
decision: 16
slug: copilot-headers-gap
status: decided
verdict: "Close it inside this program, as an issue in the remaining-adapters milestone. It is outside the 0.80.3->0.84.1 delta, but it is the same failure mode the parity principle names: an upstream file with no Go base, in files this milestone opens anyway."
decided_via: discussion
depends_on: [adapter-updates]
---

# Question

`docs/PORTING.md:46` records one **pre-existing** unported file, predating this
sync:

> `src/api/github-copilot-headers.ts` → none → **not ported** (dynamic
> `X-Initiator`/`Copilot-Vision-Request` headers; open follow-up)

`SPECS.md` repeats it as the project's one "known gap, recorded upstream-side".
It is the only item on the port's follow-up list.

This sync touches `providers/github-copilot.ts`, `github-copilot.models.ts`, the
Copilot OAuth flow, and `test/github-copilot-anthropic.test.ts` — so the Copilot
code path is being opened anyway. The question is whether an old debt gets paid
while the area is open, or stays out because it isn't part of the 0.80.3→0.84.1
delta.

# Options

- **Close it in this program** — the Copilot code is open, the fix is small
  (two dynamic headers), and the port's follow-up list goes to zero.
- **Leave it out** — this program is a version sync; unrelated debt belongs in
  its own change, and mixing them makes the program's scope fuzzy.
- **Close it as a separate change, run after** — keeps this program clean while
  still paying the debt.

# Recommendation

**Close it in this program**, as a single issue inside the adapter epic that
already touches Copilot.

The usual argument against — scope creep — is weak here for a specific reason:
this is not a new idea someone thought of mid-program, it is a *written,
tracked, single-item follow-up list* that the port has carried since the initial
port, sitting in exactly the files being modified. Deferring it again means
opening the same adapter a third time.

It also improves the program's own bookkeeping: [decision 02](/scope/02-parity-bar.md)
asks that every upstream file be dispositioned as ported-or-deviated. Leaving one
file in a third state — "acknowledged debt, neither ported nor accepted as a
deviation" — is the ambiguity this program is otherwise removing.

Note for the epic: check whether `github-copilot-headers.ts` itself changed in
range before porting the 0.80.3 version of it.

# Verdict

**Close it in this program**, as a single issue inside the remaining-adapters
milestone. Decided in discussion, 2026-08-09.

One caveat was put to the maintainer explicitly and accepted: unlike everything
else in this ledger, `src/api/github-copilot-headers.ts` is **not** part of the
0.80.3→0.84.1 delta. It is pre-existing debt, and "out of scope for a version
sync" would have been a defensible verdict.

It is closed anyway because it is the exact failure mode the parity principle
from [decision 09](/scope/09-new-oauth-flows.md) describes: an upstream file
with no Go base, carried since the initial port, sitting in the files this
milestone opens regardless. Leaving it a third time would confirm that the
port's follow-up list is one that never empties.

It also removes an ambiguity from [decision 02](/scope/02-parity-bar.md)'s
disposition gate, which asks every upstream file to be either ported or listed
as a deviation. This file has been in a third state — acknowledged debt, neither
ported nor accepted — which is precisely what the gate exists to eliminate.

Note for the epic: check whether `github-copilot-headers.ts` itself changed
within the range before porting the 0.80.3 version of it.
