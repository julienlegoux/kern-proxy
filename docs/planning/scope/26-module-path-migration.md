---
type: Decision
title: "Module path migration to kern-ia"
description: "Moving the Go module path off the personal account onto the kern-ia organisation, and when it lands."
tags: [decision, scope]
timestamp: 2026-08-09T01:30:23Z
phase: scope
decision: 26
slug: module-path-migration
status: decided
verdict: "Change the module path to github.com/kern-ia/kern-link as the program's first milestone, inside the sync program rather than as a separate change. One mechanical PR, no /v2 suffix needed, shipped in the same breaking v0.2.0 as the type changes."
decided_via: discussion
depends_on: [release-and-breaking-strategy]
---

# Question

Surfaced during [decision 03](/scope/03-release-and-breaking-strategy.md)'s
deep-dive, not from the checklist — this decision is numbered last but sequences
**first**.

The repository has moved to the `kern-ia` organisation. Verified at
2026-08-09:

- `git remote -v` → `https://github.com/kern-ia/kern-link.git`
- `gh release list` → v0.1.0 and v0.1.1 both present under `kern-ia/kern-link`
- `git ls-remote --tags origin` → both tags present
- `gh api repos/julienlegoux/kern-link` → resolves to `kern-ia/kern-link`, i.e.
  GitHub's transfer redirect is in place

So the repository, its tags and its releases have already moved. **The Go module
path has not.** `go.mod` line 1 still reads
`module github.com/julienlegoux/kern-link`, and `git grep -o
'julienlegoux/kern-link' | wc -l` counts **355** occurrences across the tracked
tree — the import block of nearly every Go file, plus `README.md` (7) and
`CHANGELOG.md` (4).

The module path is the artefact that matters most, because it is what a consumer
writes in an import statement; the repository URL is only how the toolchain
fetches it.

# Options

- **Migrate as the program's first milestone** — mechanical rename before any
  porting starts.
- **Migrate as a separate change, outside this program** — same work, tracked
  independently, run before `split-epics`.
- **Migrate late, with the release** — bundles it with the version bump.
- **Leave the path on the personal account** — the redirect works, so nothing
  breaks today; but the canonical import path keeps contradicting where the
  project actually lives.

# Recommendation

**Migrate as the program's first milestone.** Recommended over the alternatives
on sequencing grounds: 355 references touch nearly every file in the repo, so
this rename conflicts with *every* in-flight branch. Run first, it is a pure
mechanical rename with no semantic risk and the deck is clear. Run late, it
collides with six milestones of ported code.

Two facts that make it cheaper than its size suggests:

- **No `/v2` suffix is needed.** Go's major-version suffix rule starts at v2;
  the project is at v0.x, so the new path is simply
  `github.com/kern-ia/kern-link`.
- **Existing consumers do not break.** `github.com/julienlegoux/kern-link@v0.1.1`
  stays resolvable permanently through proxy.golang.org, where module versions
  are immutable. A pinned consumer keeps working; only *new* versions live under
  the new path.

# Verdict

**First milestone, inside the sync program.** Decided in discussion, 2026-08-09.

The maintainer chose to track it as a milestone of this program rather than
split it out as a separate change ("Non, fais une milestone"), so it becomes
**Milestone 1** and the six milestones from
[decision 20](/scope/20-milestones-phasing.md) shift to 2–7. Sequential
numbering from 1 is used rather than a "Milestone 0", because `split-epics` cuts
epics from the `## Milestone N:` headings and a zero-indexed one would read as an
off-by-one to every later reader.

Shape of the work, as one epic and essentially one PR:

- `go.mod` module line, then every import across the tree.
- `README.md` and `CHANGELOG.md` references.
- `docs/` references, including the `go get` line.
- Commit as `refactor!: move module path to github.com/kern-ia/kern-link`,
  following the `!`-marks-breaking convention in `CONVENTIONS.md`.
- Acceptance: `go build ./...`, `go test ./...` and `golangci-lint` green, and
  `git grep julienlegoux` returns only intentional historical mentions
  (CHANGELOG entries for already-released versions, which must **not** be
  rewritten).

It ships in the same breaking **v0.2.0** as the type changes
([decision 03](/scope/03-release-and-breaking-strategy.md)) — one breaking
release for consumers to absorb, not two.
