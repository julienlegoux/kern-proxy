---
type: Issue
title: "Cut v0.2.0: merge develop into main, tag the release, and close #113"
description: "The program's closing act — the develop-to-main release PR, the v0.2.0 tag and GitHub release on a green commit, and the umbrella tracking issue closed."
tags: [epic-9]
timestamp: 2026-08-11T20:00:00Z
epic: 9
issue: 06
slug: cut-v0-2-0-release
size: S
status: open
gh_issue: 197
resource: https://github.com/kern-ia/kern-link/issues/197
depends_on: ["05"]
---

# Cut v0.2.0: merge develop into main, tag the release, and close #113

## Summary

Epic 9's acceptance criteria 6 and 7, and the end of the nine-epic program.
Every other issue in this epic produces a PR into `develop`; this one produces
the **release PR** — `develop → main` — plus the tag, the GitHub release, and
the close of the umbrella issue **#113**.

That shape is deliberate and worth stating, because it does not fit the usual
feature-branch flow: there is no `issue-<NN>-<slug>` branch and no code change.
The work is the merge, the tag, and the verification that the commit being
tagged is green.

Closing #113 has a designed consequence: the weekly upstream-sync job stays
quiet only while an `upstream-sync`-labeled issue is open, so its next Monday
run will open a fresh issue for everything upstream landed after `936aff00`.
That is the intended outcome of freezing the target
([decision 17](../../../planning/scope/17-upstream-lock-and-weekly-job.md)),
not a failure — say so in the closing comment so the next reader does not treat
it as one.

## Scope

- Confirm `develop` is the complete program: every epic's milestone closed or
  its remaining issues explicitly dispositioned, and
  `gh pr list --state open --base develop` empty of program work.
- Confirm no open GitHub issue is a should-have-been-ported follow-up left by
  [issue 04](/epic-9-classifier-audit-and-release/issues/04-disposition-sweep.md)'s
  disposition sweep. `SCOPE.md:65-78` rules out postponement as a deviation
  ground by name; a `docs/PORTING.md` row honestly recording "should be
  ported, not yet done" is allowed to exist, but the follow-up it points at
  may not still be open when this tag is cut.
- Verify the three gates on the tip of `develop` before merging —
  `go test ./... -race -v`, `bash upstream/sync_test.sh`, and `golangci-lint`
  v2.12.2 — via the CI run on the commit, not locally (`-race` needs a C
  toolchain that is not installed locally, and CI's verdict is the
  authoritative one).
- Open the release PR `develop → main`, titled for the release, its body
  pointing at the `CHANGELOG.md` `## [0.2.0]` section. Merge with a **merge
  commit** — no squash, no rebase onto an integration branch.
- Tag `v0.2.0` on the resulting merge commit on `main` and push the tag.
- Create the GitHub release for `v0.2.0`, notes drawn from the changelog
  section, matching the shape of the existing `v0.1.0` / `v0.1.1` releases.
- Close **#113** with a comment linking the release and the epic issues, and
  naming the weekly job's next run as the expected follow-on.

## Out of scope

- Any code or docs change. If CI is red on `develop`, this issue **stops** and
  the fix lands as its own PR into `develop` first — a release PR that also
  fixes something is a release cut from an untested tree.
- Closing the epic 9 milestone, promoting drift records into
  `docs/planning/DRIFT.md`, and cleaning up worktrees and merged branches —
  `close-epic` owns all of that, after this issue.
- Announcing the release anywhere outside GitHub.
- Pausing, editing, or re-running `.github/workflows/upstream-sync.yml`.

## Acceptance criteria / Definition of done

- [ ] No open GitHub issue is a should-have-been-ported follow-up from issue
      04's disposition sweep — checked against the issues that PR opened, not
      by memory. If one is still open, this issue stops and waits rather than
      tagging with a known-postponed deviation.
- [ ] A `develop → main` PR exists and is merged with a merge commit:
      `git log main --merges -1 --format=%s` names it, and `main`'s history
      shows no squash or rebase of `develop`'s commits.
- [ ] `git tag --points-at $(git rev-parse main)` includes `v0.2.0`, and the tag
      is pushed (`git ls-remote --tags origin v0.2.0` returns it).
- [ ] CI is green on the tagged commit — all three checks
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2) — verified with
      `gh run list --commit $(git rev-parse main) --json name,conclusion`.
- [ ] A GitHub release `v0.2.0` exists, marked as the latest release, with notes
      covering the breaks (`gh release view v0.2.0`).
- [ ] `gh issue view 113 --json state` reports `CLOSED`, with a closing comment
      that links the release and states that the weekly job will open the next
      sync issue on its next run.
- [ ] `main` carries the module path `github.com/kern-ia/kern-link` (epic 1), so
      `go install github.com/kern-ia/kern-link/cmd/pi-ai@v0.2.0` resolves once
      the module proxy has indexed the tag — check it, and if the proxy lags,
      note that rather than treating it as a failure.
- [ ] `upstream/UPSTREAM.lock` on `main` reads `936aff00…` / `0.84.1` — the
      third of the program's three completion gates, verified on the released
      branch rather than only on `develop`.

## Relevant files / areas

- No files change. The artifacts are git and GitHub state: the merge commit on
  `main`, the `v0.2.0` tag, the release, and issue #113.
- `CHANGELOG.md` `## [0.2.0]` — the source for the release notes.
- `upstream/UPSTREAM.lock` — verified, not edited.
- `.github/workflows/test.yml` — the three gates; `.github/workflows/upstream-sync.yml`
  — the job that wakes up when #113 closes.
- `docs/planning/CONVENTIONS.md`, "Commits & branches" — merge commits
  throughout, no rebasing or squashing onto the integration branches.

## Dependencies

- **Blocked by**: [Issue 05](/epic-9-classifier-audit-and-release/issues/05-upstream-lock-and-changelog.md)
  — the tag is cut from a `develop` that already carries the lock bump and the
  changelog entry. Transitively blocked by every issue in every epic of the
  program.
- **Blocks**: Nothing in this epic. `close-epic` runs after it.

## PR size note

`S` — near-zero diff by design: the PR is a merge of `develop` into `main`, not
a change, and `## Relevant files / areas` names no file at all. The reviewable
unit is the release itself: the changelog section, the green CI run, and the
tag. Split past ~200 — which cannot happen without a code change this issue
forbids outright.
