# Issue Review Report 9

## Scope

- Reviewed: `docs/epics/epic-9-classifier-audit-and-release/issues/01-classifier-parity-audit.md`,
  `02-provider-retry-vs-httpretry.md`, `03-disposition-checker.md`,
  `04-disposition-sweep.md`, `05-upstream-lock-and-changelog.md`,
  `06-cut-v0-2-0-release.md`, and `issues/index.md`.
- Reviewed against: `docs/epics/epic-9-classifier-audit-and-release/EPIC_9.md`
  (the contract), with `docs/planning/SCOPE.md` (the epic's `source`),
  `docs/planning/CONVENTIONS.md`, `docs/planning/SPECS.md`, `docs/PORTING.md`,
  `docs/epics/index.md` and `docs/epics/log.md` read for context. Cited code and
  config (`ai/retry.go`, `ai/overflow.go`, `ai/apis/internal/httpretry/`,
  `CHANGELOG.md`, `upstream/sync.sh`, `upstream/UPSTREAM.lock`,
  `.github/workflows/test.yml`, `docs/index.md`) was opened to check the issues'
  `file:line` claims. No `CLAUDE.md`, `AGENTS.md`, `CONTRIBUTING.md`, issue
  template or PR template exists in this repo.
- GitHub verification: verified (`kern-ia/kern-link`) — issues #192–#197 all
  exist, `OPEN`, titles byte-identical to the local `title:` fields, all on
  milestone 26 "Epic 9: Classifier audit, disposition sweep, and release", and
  all six are native sub-issues of the epic's tracking issue #126. #126 itself is
  `OPEN` with labels `epic`/`upstream-sync`; #113 is `OPEN` as the epic expects.
  Labels on the six are `enhancement` (192, 193, 194, 197) / `documentation`
  (195, 196) — no `epic` label leaked onto a sub-issue.
- External review: `openai/gpt-5.6-terra` via `opencode run --agent plan`
  (two passes: coverage/sizing/ordering, then schema + citations). Its output was
  treated as leads; each is marked below as verified, downgraded, or rejected.

## Findings

### P1 — The disposition checker measures a different file set than the epic's "232 files in range"

- Location: `docs/epics/epic-9-classifier-audit-and-release/issues/03-disposition-checker.md:41-44`
  and `:55`; consumed as the acceptance criterion at
  `docs/epics/epic-9-classifier-audit-and-release/issues/04-disposition-sweep.md:95-100`.
- Source: `docs/epics/epic-9-classifier-audit-and-release/EPIC_9.md:34-36` and
  `:60-61` ("All **232** files in range are dispositioned"), defined in
  `docs/planning/SCOPE.md:28-29` as "**232 files, +19531/−21423, across 207
  commits** in `packages/ai` — 29 new source files and 3 deleted".
- Problem: "in range" in the plan and the epic means *the delta between
  `244f1dea` and `936aff00`* — 232 changed paths, three of which are deletions.
  Issue 03 enumerates the candidate set with
  `git -C "$workdir" ls-tree -r --name-only "$ref" -- packages/ai`
  (`03:43`), which is the **whole tree at the target ref**: it includes hundreds
  of files that never changed in range, and it structurally cannot see the three
  files upstream deleted in range, because they do not exist at `936aff00`. The
  summary line at `03:55` therefore cannot make "the 232 number observable" as it
  claims; it will print the tree size.
- Impact: the epic's second completion gate is decided by a mechanical arbiter
  measuring the wrong population. Deleted-upstream files — exactly the ones whose
  Go counterparts are now orphaned — can never be reported `UNACCOUNTED`, so the
  sweep can exit 0 with them undispositioned. Issue 04's AC at `:98-100`
  half-anticipates this ("if the real count differs, the PR says so"), which
  turns a hard gate into a narrative one.
- Recommendation: derive the candidate set from
  `git -C "$workdir" diff --name-status 244f1deaf1ae0fc1a242d9df5cddf457cf3d36a7..936aff00918de1187f085f123c2812d8f2d67745 -- packages/ai`,
  keeping `D` entries as in-range paths that still need a disposition row, and
  make the summary line print that count so `232` is checkable. If a whole-tree
  sweep is wanted *as well*, state it as a deliberate superset in `03` and drop
  the "232 observable" claim; do not leave two different definitions of "in
  range" in the same epic.

### P2 — Issue 03 builds tooling the epic never asked for, and does not flag it as an assumption

- Location: `docs/epics/epic-9-classifier-audit-and-release/issues/03-disposition-checker.md`
  (whole file — `:35-63` new `upstream/disposition_check.sh` +
  `upstream/disposition_check_test.sh`, `:64-67` a new step in
  `.github/workflows/test.yml`, `:68-69` a new step in `docs/PORTING.md`'s sync
  procedure).
- Source: `EPIC_9.md:34-36` (Scope) and `:60-61` (AC 3) ask only that the 232
  files be "dispositioned in `docs/PORTING.md`"; `EPIC_9.md:44-51` (Out of scope)
  and `:68-69` (AC 8, which names exactly three CI commands) contain nothing about
  a checker, a second offline test script, or a workflow change.
- Problem: a whole issue — a full PR of new shell tooling plus a CI edit plus an
  edit to the documented sync procedure that outlives this epic — is derived from
  the epic rather than requested by it. The issue argues the case well
  (`03:19-31`), but never labels it an assumption, so the next reader cannot tell
  invention from mandate. It also contradicts itself: `03:64-67` wires
  `disposition_check_test.sh` into `test.yml`, while `03:76-79` asserts "the three
  gates `CONVENTIONS.md` names stay the three gates".
- Impact: it will get built. It also inserts a hard `depends_on: [3]` edge into
  issue 04 (`04:14`, `04:136-139`), so an unrequested PR becomes a prerequisite
  for the epic's central deliverable — and the current checker design is the P1
  above.
- Recommendation: either (a) state at the top of `03` that the checker is an
  assumption added beyond `EPIC_9.md`, and reconcile `03:64-67` with `03:76-79`
  (say plainly that the `test` job gains a fourth step while the number of CI
  *checks* is unchanged), or (b) drop issue 03, fold a throwaway verification
  script into issue 04's own PR, and remove `3` from `04`'s `depends_on`.

### P2 — Issue 01's row-count acceptance criterion does not match the code it counts

- Location: `docs/epics/epic-9-classifier-audit-and-release/issues/01-classifier-parity-audit.md:92-96`
  ("the row count is at least `2 + 6 + 6 + 34 + 24 + 3`").
- Source: the six groups the same issue lists at `01:51-59`, i.e.
  `ai/retry.go:16-33`, `ai/retry.go:41-48`, `ai/retry.go:53-97`,
  `ai/overflow.go:9-34`, `ai/overflow.go:39-43`, and `IsContextOverflow`'s three
  modes at `ai/overflow.go:55-87`.
- Problem: counted against the tree, those groups hold **8, 6, 28, 24, 3, 3**
  patterns/modes = 72. The criterion's `2 + 6 + 6 + 34 + 24 + 3` = 75, and no
  group in either file contains 34 or 2 entries. The numbers appear shifted as
  well as wrong.
- Impact: the one acceptance criterion written to be mechanically checkable
  cannot be satisfied honestly — a correct 72-row table fails "at least 75", and
  an implementer padding to 75 produces a table that misrepresents the code. The
  rest of the issue's citations are exact (`ai/retry.go:10-12, 16-33, 35-40,
  41-48, 53-97, 118-132, 145-148` and `ai/overflow.go:9-34, 39-43, 55-87` all
  verified), which makes this one line the outlier.
- Recommendation: replace the arithmetic with a property — "every regex in the
  six groups appears exactly once, and `docs/classifier-parity.md`'s row count
  equals the count produced by enumerating those six vars" — or correct it to
  `8 + 6 + 28 + 24 + 3 + 3`. Related, smaller: `01:57-59` extends the epic's
  "every regex" (`EPIC_9.md:26-30`, `:55-57`) to three non-regex detection modes;
  defensible, but say so as an explicit extension.

### P2 — Issue 04's escape hatch lets a genuinely unported file ship as a recorded "gap"

- Location: `docs/epics/epic-9-classifier-audit-and-release/issues/04-disposition-sweep.md:79-82`
  ("If the sweep finds an upstream file that *should* have been ported and was
  not, it does not get ported here — record it as a row with the gap stated, and
  file a follow-up issue").
- Source: `docs/planning/SCOPE.md:65-78` — "**A deviation must be justified by
  structural non-portability, never by cost.** … A deviation is not a
  postponement … Admissible grounds: constructs with no meaning in Go. Not
  admissible: 'large', 'hard to test'"; carried into `EPIC_9.md:34-36` as
  "ported or recorded as a deviation … No file left in a third, unaccounted
  state".
- Problem: "a row with the gap stated plus a follow-up issue" is precisely the
  postponement the governing principle rules out, and neither issue 05 nor 06 has
  an acceptance criterion that blocks the release on open porting follow-ups
  (`05:83-103`, `06:69-91`). The sweep can therefore exit 0, the lock can claim
  `0.84.1` (`05:85-91`), and v0.2.0 can ship while real files are unported.
- Impact: the program's parity claim — the whole point of gate 2 — becomes
  weaker than the plan promises, and the next sync's diff applies against a Go
  base that is missing files nobody is tracking as missing.
- Recommendation: require every non-`ported` row to state an **admissible**
  ground (a construct with no meaning in Go), and make any "should have been
  ported" finding a release blocker rather than a follow-up: add an AC to issue 05
  or 06 that no open porting follow-up exists at tag time, or say explicitly in
  `04` that the maintainer accepts the deviation from `SCOPE.md:65-78`.

### P3 — Epic AC 2's `docs/planning/DRIFT.md` entry is not written by any issue in this epic

- Location: `docs/epics/epic-9-classifier-audit-and-release/issues/02-provider-retry-vs-httpretry.md:69-75`
  (writes `drift/02-provider-retry-vs-httpretry.md`) and `:79-80` (Out of scope:
  "Writing `docs/planning/DRIFT.md` directly").
- Source: `EPIC_9.md:31-33` and `:58-59` — "that is a `docs/planning/DRIFT.md`
  entry, not a silent difference".
- Problem: read literally, AC 2 is not met by any of the six PRs; it is met only
  once `close-epic` promotes the drift record, and issue 06 explicitly excludes
  that promotion (`06:63-65`).
- Impact: low. The issue is *right* — the drift register is single-writer at
  epic close — and it says so at `02:73-75`. But someone auditing the epic against
  its own ACs after issue 06 merges will find `docs/planning/DRIFT.md` absent (the
  file does not exist yet) and read it as a miss.
- Recommendation: leave issue 02 as written and reword `EPIC_9.md:58-59` to
  "settled in writing — as code, or as a drift record promoted to
  `docs/planning/DRIFT.md` at epic close". (The external pass graded this "High"
  and proposed making issue 02 write the register directly; downgraded — that
  would break the single-writer rule the pipeline depends on.)

### P3 — Issues 01 and 02 both claim the same `httpretry` regression criterion

- Location: `01-classifier-parity-audit.md:115-118` (`IsNonRetryableProviderLimitError`
  "still wins … in both `IsRetryableAssistantError` … and `httpretry.IsRetryable`,
  with a test") versus `02-provider-retry-vs-httpretry.md:110-112`
  (`httpretry.IsRetryable` "still consults `ai.IsNonRetryableProviderLimitError`
  first, with a test proving a quota message returns `false` even at HTTP 429").
- Source: `01:76-78` puts `httpretry` out of scope for issue 01 and hands it to
  issue 02; `01:39-72` (Scope) touches only `ai/retry*.go` and `ai/overflow*.go`.
- Problem: issue 01's AC reaches into a package its own Scope and Out-of-scope
  exclude, and duplicates issue 02's AC nearly verbatim.
- Impact: two PRs may add near-identical tests to
  `ai/apis/internal/httpretry/httpretry_test.go`, and issue 01 can be blocked on
  a file it was told not to touch.
- Recommendation: in `01:115-118` keep only the `IsRetryableAssistantError` half
  and note that the `httpretry` coupling is asserted by issue 02.

### P3 — `docs/classifier-parity.md` lands in the consumer-facing docs bundle

- Location: `01-classifier-parity-audit.md:45-50` (new `docs/classifier-parity.md`)
  and `:69-72` (bullet in `docs/index.md`, entry in `docs/log.md`).
- Source: `docs/planning/mapping/01-planning-bundle-nesting.md:69-74` — the
  accepted verdict keeps `docs/log.md` "a changelog of the consumer-facing
  documentation" and honors "the v0.1.1 decision to keep dev-process artifacts
  out of what ships in the module"; `docs/log.md:5` restates it.
- Problem: a regex-by-regex sync audit is a dev-process artifact, and it is being
  added to the bundle that ships to library consumers, with its own `docs/index.md`
  bullet.
- Impact: low, and there is real counter-precedent — `docs/PORTING.md` is an
  equally dev-facing artifact living in the same bundle and listed at
  `docs/index.md:14`, which is exactly the shape issue 01 follows.
- Recommendation: either accept it explicitly as following the `PORTING.md`
  precedent (one sentence in `01`), or move the audit under `docs/planning/` and
  link it from `docs/PORTING.md:27-28` only.

### P3 — PR size notes are boilerplate and disagree with the declared `size`

- Location: identical text in `01:157-160`, `02:144-147`, `03:128-131`,
  `05:125-128`: "Target ~500 changed lines; if this grows past ~1000, split it
  before opening the PR."
- Source: the issue schema's size bands — `S ≈ under 200`, `M ≈ 200–500`,
  `L ≈ 500–1000`, "L is the ceiling, not the target" — against `01:10`, `02:10`,
  `03:10`, `05:10`, all `size: M`.
- Problem: four `M` issues all "target" the top of the `M` band and quote the `L`
  ceiling as their split threshold, so the note carries no per-issue information.
  Issue 04 (`04:143-150`) and issue 06 (`06:112-116`) show what a real size note
  looks like — 04 argues why it must be one pass, 06 explains its near-zero diff.
- Impact: an implementer gets no signal about which of these PRs is actually
  large. Issue 01 is the one to watch: a ~72-row table plus pattern ports plus
  test cases plus four doc registrations is plausibly over the `M` band.
- Recommendation: give 01, 02, 03 and 05 a one-line, issue-specific estimate in
  the shape of 04's and 06's, and re-check whether issue 01 should be `L`.

### P3 — Link forms: `./` bullets in `issues/index.md`, and bundle-absolute links inside GitHub bodies

- Location: `docs/epics/epic-9-classifier-audit-and-release/issues/index.md:3-8`
  (`](./01-classifier-parity-audit.md)`); GitHub issue bodies, e.g. #194, which
  carry `](/epic-9-classifier-audit-and-release/issues/04-disposition-sweep.md)`.
- Source: the bundle link rule — within a bundle, use a bundle-relative absolute
  path with a leading `/`; across bundles, a plain relative path. The issue bodies
  themselves get this right (`01:77`, `01:80`, `02:82` use the `/epic-9-…` form;
  `01:31`, `02:23`, `04:25` use `../../../planning/…` for cross-bundle links,
  which resolve correctly from `docs/epics/epic-9-…/issues/`).
- Problem: two mirror-image nits. The index bullets use `./NN-…` where the rule
  asks for `/epic-9-classifier-audit-and-release/issues/NN-…`; and the correct
  bundle-absolute links, once copied verbatim into GitHub issue bodies, render as
  `https://github.com/epic-9-…` and 404 for anyone reading the issue on GitHub.
- Impact: cosmetic in the bundle; mildly annoying on GitHub. Both are consistent
  with every sibling epic (`docs/epics/epic-8-pi-messages-and-radius/issues/index.md:3-6`
  uses the same `./` form), so this is a pipeline-wide habit, not an Epic 9 defect.
- Recommendation: no change needed for Epic 9 alone. If it is worth fixing, fix it
  across all nine epics at once, and consider rewriting in-bundle links to full
  `https://github.com/kern-ia/kern-link/blob/develop/docs/…` URLs when pushing
  issue bodies.

### P3 — Issue 05 does not mention the CHANGELOG link-reference footer

- Location: `05-upstream-lock-and-changelog.md:68-69` ("keep the link-reference
  style the file already uses at its foot if it has one") and `:111-112`
  (`CHANGELOG.md:1-30` as the shape to follow).
- Source: `CHANGELOG.md:63-65` — the footer is
  `[Unreleased]: https://github.com/julienlegoux/kern-link/compare/v0.1.1...HEAD`
  and two siblings, all pointing at the **old** `julienlegoux` org, while epic 1
  moves the module path to `github.com/kern-ia/kern-link` (restated at `05:47-49`
  and `06:85-88`).
- Problem: the footer exists (so the conditional "if it has one" resolves to
  yes), and following its style verbatim adds a `[0.2.0]` compare link on the
  stale org. The issue's own Scope highlights the org move as the headline break
  but does not connect it to the footer.
- Impact: small — GitHub redirects the old URLs — but the release entry ends up
  advertising the pre-rename remote.
- Recommendation: add to `05`'s Scope: refresh `CHANGELOG.md:63-65` onto
  `kern-ia/kern-link` and add the `[0.2.0]` compare link in that form.

## Coverage notes

- **Every epic acceptance criterion has an owner.** AC 1 → issue 01; AC 2 →
  issue 02 (via a drift record, see P3 above); AC 3 → issues 03 + 04; AC 4 →
  issue 05 (`05:85-86`, with the exact `grep -c '^commit=…'` form); AC 5 →
  issue 04 (`04:103-105`, `grep -n 'go\.N'`); AC 6 → issues 05 + 06; AC 7 →
  issue 06 (`06:82-84`); AC 8 → repeated as a per-PR criterion in every issue and
  verified on the tagged commit at `06:76-79`. Every Scope bullet of
  `EPIC_9.md:22-42` maps to at least one issue, and nothing in `EPIC_9.md:44-51`
  (Out of scope) is built — no linter battery, no coverage tooling, no compat
  shims, no post-`936aff00` upstream work.
- **Ordering is sound and symmetric.** `depends_on` reads `01:[]`, `02:[1]`,
  `03:[]`, `04:[1,2,3]`, `05:[4]`, `06:[5]`, and every prose "Blocks" claim has a
  matching "Blocked by" on the other side. A developer can work 01 → 06 top to
  bottom without hitting a later prerequisite; 03 is correctly marked parallel
  with 01/02. The cross-epic prerequisite (adapters merged first, per
  `EPIC_9.md:22-25`) is stated in prose at `01:148-152` rather than smuggled into
  `depends_on`, which is right.
- **GitHub state is clean.** All six local `gh_issue`/`resource` values resolve;
  titles match byte-for-byte; states are `open` in both places; all six are native
  sub-issues of #126 with no extras attached; milestone 26 holds 7 open items (six
  issues + the tracking issue) and 0 closed. `docs/epics/log.md:79-84` records a
  creation entry per issue, and `issues/index.md` agrees with each file's
  `size`/`status`/`gh_issue`.
- **Frontmatter is complete and schema-clean** on all six: `type`, `title`,
  `description`, `tags: [epic-9]`, ISO-8601 `timestamp`, `epic`, zero-padded
  `issue`, `slug`, `size`, `status: open`, `gh_issue`, `resource`, `depends_on`.
  No extension fields, no premature `gh_pr`. All six bodies carry exactly the
  seven required sections in the required order; `issues/index.md` correctly
  carries no frontmatter.
- **Repository conventions are genuinely reflected**, not gestured at: the
  `GOTMPDIR=$PWD/.gotmp go test ./...` local command and the CI-only `-race`
  (`CONVENTIONS.md:182-189`) appear in 01, 02, 04 and 05; the three CI gates and
  the pinned `golangci-lint v2.12.2` (`CONVENTIONS.md:230-237`) in all six;
  Conventional Commit examples with real scopes in five; merge-commit-only release
  flow and the `develop → main` shape (`CONVENTIONS.md:213-219`) in 06, including
  the explicit note that this issue has no `issue-<NN>-<slug>` branch; the
  table-driven-test carve-out for `retry_test.go`/`overflow_test.go` and the
  "19 table-driven tests" figure (`CONVENTIONS.md:147-157`) in 01; the ST1005
  rationale for load-bearing error text (`CONVENTIONS.md:108-111`) as issue 01's
  reason not to touch adapter strings; and the "absence of a `// Ports:` header is
  meaningful" rule (`CONVENTIONS.md:63-66`) as the hinge of issue 02.
- **Citation accuracy is high.** Spot-checking every `file:line` in the six
  issues against the tree: `ai/retry.go:10-12/16-33/35-40/41-48/53-97/118-132/
  145-148`, `ai/overflow.go:9-34/39-43/55-87`, `docs/PORTING.md:27-28/38/46/57/
  40-43/19-64/66-109/111-131/128`, `docs/index.md:11-14`, `CHANGELOG.md:1-30`,
  `upstream/sync.sh:22-24`, `upstream/UPSTREAM.lock`, `.github/workflows/test.yml`
  and `.github/workflows/upstream-sync.yml`'s "skip if an issue is already open"
  guard all say what the issues claim. `ai/overflow.go:89-94` for
  `OverflowPatterns` is one line early (the func starts at 90); that is the only
  drift found, and it is not worth a finding.
- **Rejected leads.** (a) The external pass flagged `05:47-49` and `06:85-88` as
  "false" for naming `github.com/kern-ia/kern-link` while the tree still reads
  `github.com/julienlegoux/kern-link` — rejected: that move is epic 1 issue 02
  (#128), so the claim is correctly forward-looking. (b) It flagged issue 03 for
  omitting the local `GOTMPDIR` test line — rejected: issue 03 puts "Any Go code"
  out of scope (`03:83`). (c) It proposed deleting issue 01's non-regex detection
  modes — folded into the P2 above as a labelling point rather than a removal,
  since `IsContextOverflow`'s thresholds are behavior the epic's stated risk
  (`EPIC_9.md:94-97`) plainly covers.

## Open questions

- Is the whole-tree scan in issue 03 a deliberate superset (disposition every
  upstream file that exists, not just the 232 that changed), or an oversight? The
  answer changes the P1 fix from "swap the enumeration" to "keep it and restate
  the 232 claim". Nothing in `EPIC_9.md` or `SCOPE.md` settles it.
- Does the maintainer want issue 03 at all? It is the only issue in the epic with
  no counterpart in `EPIC_9.md`, and dropping it removes one PR, one CI edit and
  one permanent addition to the documented sync procedure — at the cost of making
  AC 3 a judgement call again.
