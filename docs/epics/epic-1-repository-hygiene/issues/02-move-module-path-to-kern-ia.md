---
type: Issue
title: "Move the module path to github.com/kern-ia/kern-link"
description: "Rename the Go module path off the personal account onto the kern-ia organisation across go.mod, every import, README, CHANGELOG links and docs — one mechanical breaking PR."
tags: [epic-1]
timestamp: 2026-08-11T18:15:00Z
epic: 1
issue: 02
slug: move-module-path-to-kern-ia
size: M
status: open
gh_issue: 128
resource: https://github.com/kern-ia/kern-link/issues/128
depends_on: [1]
---

# Move the module path to github.com/kern-ia/kern-link

## Summary

The repository, its tags and its releases already live at `kern-ia`; only the Go
module path still says `julienlegoux`
([decision 26](../../../planning/scope/26-module-path-migration.md)). The module
path is the artefact that matters most, because it is what a consumer writes in
an import statement — the repository URL is only how the toolchain fetches it.

This is a pure mechanical rename with no semantic change, and it runs before any
other epic in the program opens a branch: **370 references across 170 files**
touch nearly every file in the repo, so run late it collides with six milestones
of ported code, once per branch.

No `/v2` suffix: the project is v0.x, so Go's major-version suffix rule does not
apply. Consumers pinned to `github.com/julienlegoux/kern-link@v0.1.1` keep
working — module versions are immutable in `proxy.golang.org` — so nothing
breaks today; only new versions live under the new path.

## Scope

Replace `github.com/julienlegoux/kern-link` with `github.com/kern-ia/kern-link`
in:

- **`go.mod:1`** — the `module` line.
- **157 `.go` files** — every self-import. Highest-density files:
  `ai/providers/*.go` (35 provider bindings, 4–7 refs each),
  `ai/apis/**/*.go`, `cmd/pi-ai/example/main.go`.
- **`README.md`** (7 lines match `grep -c 'julienlegoux/kern-link' README.md`;
  9 occurrences by `grep -o`, since the two badge lines each carry it twice) —
  the Test workflow badge URL, the pkg.go.dev badge and link, the `go get`
  line, the two example imports, the `go run …/cmd/pi-ai login` line.
- **`docs/usage.md`** (8 refs) and **`docs/auth.md`** (3 refs) — `go get`, the
  package table, example imports, the `go run …/cmd/pi-ai` invocations.
- **`CHANGELOG.md:63-65`** — the three link-reference URLs at the bottom
  (`[Unreleased]`, `[0.1.1]`, `[0.1.0]`). These are navigation to a repo that has
  already moved, not history.
- **`CONVENTIONS.md:71-77`** (repo root) — the `## Go module` section currently
  reads "Current path: `github.com/julienlegoux/kern-link` … do not rename
  unilaterally here". Replace it with the new path and a one-line note that the
  move was decided at org level and executed in this epic; leaving the old
  instruction standing tells the next reader the opposite of what the repo does.
- **`docs/planning/SPECS.md:22` and `:247`** — the stack line (rewrite the module
  path *and* `Go **1.25.0**` → `Go **1.26**`, closing the line
  [issue 01](/epic-1-repository-hygiene/issues/01-bump-go-directive-to-1-26.md)
  deliberately left alone) and the `go get` line in the delivery section.

Commit as `refactor!: move module path to github.com/kern-ia/kern-link`, per the
`!`-marks-breaking convention.

## Out of scope

- **A `/v2` module suffix**, or any compatibility shim / type alias package for
  the old path. The redirect and the immutable proxy cover existing consumers.
- **Historical CHANGELOG entries for already-released versions.**
  `CHANGELOG.md:20` — the 0.1.1 entry describing the `kern-proxy` →
  `github.com/julienlegoux/kern-link` rename — is **not** rewritten. It describes
  what was published under the old path, and that remains true.
- **A new `## [Unreleased]` CHANGELOG entry for this change.** `CHANGELOG.md` is
  updated as part of release preparation, not per-PR
  ([CONVENTIONS.md, Review](../../../planning/CONVENTIONS.md)); the v0.2.0 notes
  are Epic 9's job.
- **The planning and epics bundles' historical records** — `docs/planning/SCOPE.md`,
  `docs/planning/scope/*.md`, `docs/planning/log.md`, `docs/epics/**`. They state
  the old path as the thing being migrated; rewriting them erases the decision
  they exist to record.
- **`docs/planning/CONVENTIONS.md:215`** — `Merge pull request #N from
  julienlegoux/<branch>` describes real merge-commit history, not the module path.
- **Any semantic change riding along with the rename** — no refactors, no
  reordered imports beyond what `gofmt` does, no drive-by fixes.
- **Renaming the `develop` integration branch to `dev`**, flagged as a to-fix in
  the root `CONVENTIONS.md`. `develop` stays the integration trunk for this whole
  program ([decision 19](../../../planning/scope/19-constraints.md)).
- **Tagging or publishing v0.2.0.** That closes the program, in Epic 9.

## Acceptance criteria / Definition of done

- [ ] `head -1 go.mod` reads `module github.com/kern-ia/kern-link`.
- [ ] `grep -c '^go 1\.26$' go.mod` returns `1` — the toolchain bump from
      [issue 01](/epic-1-repository-hygiene/issues/01-bump-go-directive-to-1-26.md)
      survived the 170-file rename.
- [ ] No unintended reference survives:
      `git grep -l 'julienlegoux/kern-link' -- ':!CHANGELOG.md' ':!docs/planning' ':!docs/epics'`
      prints nothing.
- [ ] The surviving references are exactly the intended ones —
      `git grep -n 'julienlegoux/kern-link' -- CHANGELOG.md` returns only line 20
      (the 0.1.1 historical entry), and every remaining `docs/planning/` and
      `docs/epics/` hit is a decision, scope, log or epic record.
- [ ] `docs/planning/SPECS.md` carries both fixes:
      `git grep -n 'julienlegoux/kern-link' -- docs/planning/SPECS.md` prints
      nothing, and `docs/planning/SPECS.md:22` reads
      `github.com/kern-ia/kern-link` at Go **1.26**.
- [ ] `go build ./...` passes and `GOTMPDIR=$PWD/.gotmp go test ./...` passes
      locally (`-race` is CI-only — no local C toolchain).
- [ ] `gofmt -l .` prints nothing.
- [ ] CI green on the PR: the `test` job (`go test ./... -race -v` and
      `bash upstream/sync_test.sh`) and the `lint` job (`golangci-lint` pinned
      `v2.12.2`). **The race detector's verdict only ever arrives from CI.**
- [ ] `go.sum` is unchanged — the rename touches no dependency:
      `git diff --stat go.sum` is empty.
- [ ] No test file was edited to accommodate the rename beyond its import block —
      a changed assertion means something semantic moved, which this PR forbids.
- [ ] The change lands as a single `refactor!:`-scoped PR merged into `develop`
      **before any other epic in this program opens a branch**, with the semver
      impact (breaking, part of v0.2.0) stated in the PR body. No tool-signature
      trailer in the commit message.

## Relevant files / areas

Verified against the tree at `318731c` (`git grep -o 'julienlegoux/kern-link' | wc -l`
→ **369** total occurrences, **366** for the `github.com/julienlegoux/kern-link`
form the rename actually targets; up from the 355 counted when the scope was
written — `docs/` keeps growing independently of this migration, so re-run the
command above rather than trusting either number):

- `go.mod:1` — the module line.
- 157 `.go` files, densest in `ai/providers` (44 files), `ai/apis/bedrock` (14),
  `ai/images` (9), `ai/apis/openaicompletions` (9), `ai/apis/mistral` (9), then
  the remaining `ai/apis/*` adapters, `ai/auth`, `ai/auth/oauth`, `ai/catalog`,
  `ai/internal/*` and `cmd/pi-ai/*`. No `.go` file under `testbed/` or `tools/`
  references the module path.
- `README.md` (7), `docs/usage.md` (8), `docs/auth.md` (3).
- `CHANGELOG.md` (4 — one kept, three updated), `CONVENTIONS.md` (root),
  `docs/planning/SPECS.md` (2).
- Untouched: `.github/workflows/*` (no module path in either workflow),
  `go.sum`, `upstream/`, `.golangci.yml`.

Practical note: a tree-wide `sed` over the tracked file list is the intended
mechanism, but the exclusions above are not expressible as a single pattern —
run it over `git grep -l`'s output minus `CHANGELOG.md`, `docs/planning/`,
`docs/epics/`, then hand-edit the six intended lines in the excluded files. The
path gets *shorter*, and Go import blocks are not column-aligned, so `gofmt`
output should not move.

## Dependencies

- **Blocked by**:
  [Issue 01](/epic-1-repository-hygiene/issues/01-bump-go-directive-to-1-26.md) —
  both edit `go.mod`, and landing the toolchain bump first keeps this diff purely
  mechanical.
- **Blocks**: every other epic in the program (2 through 9). Their branches must
  open after this merges into `develop`.

## PR size note

`M` — ~370 modified lines across ~170 files (366 occurrences of the
`github.com/julienlegoux/kern-link` form at `318731c`), entirely
search-and-replace. Split past ~500.

**This one cannot be split further**, and the band does not license splitting
it: an intermediate state where `go.mod` declares the new path while imports
still name the old one does not compile, and vice versa. Atomicity is a build
constraint, not a preference. If the diff exceeds expectations, that means
something non-mechanical crept in — remove it rather than splitting the rename.
