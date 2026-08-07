# CONVENTIONS.md — kern-link

Local authority for this repo, as announced by the org-wide
[CONTRIBUTING.md](https://github.com/kern-ia/.github/blob/main/CONTRIBUTING.md). The rules
shared by all `kern-ia` repos are restated below; the "Specifics" sections cover what belongs
only to `kern-link`.

`kern-link` is currently the only repo in the org that already runs a real GitHub PR flow
(51 PRs merged so far) — it serves as the reference for the other repos on this specific
point, not the other way around.

## Language

Code, identifiers, and comments are written in English — no exceptions. This applies to
source files, docstrings, commit diffs, and test names. Internal documentation such as this
file, `README.md`, or `CLAUDE.md` stays in whatever language the team works in day to day.

> Note: this repo ports `@earendil-works/pi-ai`, whose error text is copied verbatim (see the
> `staticcheck`/`ST1005` exception below). That upstream text stays as-is even where its
> phrasing differs from house style — it is not a comment, and rewriting it silently changes
> behavior tests depend on.

## Branches

- `main`: stable branch, always deployable. Protected — no direct pushes.
- Integration branch: **`dev`** — the common `kern-ia` standard (see every other
  `CONVENTIONS.md` in the org).

> **To fix on this repo**: the integration branch is currently called `develop`, the only
> repo in the org in that case. Rename `develop` → `dev` to align (low-risk operation: rename
> the branch, update its protection and the base of open PRs) — not an open question, just a
> correction to make, deliberately and not silently.

- Working branches: `feature/<slug>`, `fix/<slug>`, `chore/<slug>`, `docs/<slug>`,
  `release/<version>` (already the real usage here: `repo-polish`, `rename-kern-link`...).
- Any change to `main` or `develop` goes through a Pull Request — already respected.
- Merge: standard merge commit via the GitHub button (`Merge pull request #N from
  <owner>/<branch>`) — pattern already in place, to document as the reference for the other
  repos that currently do local merges invisible on GitHub.

## Commits

Conventional Commits: `feat:`, `fix:`, `chore:`, `docs:`, `chore(release):`... No tool
signature (`Co-Authored-By`, `Claude-Session`, or equivalent trailer) in commit messages —
the git author is enough. Past commits in this repo still carry them; don't add new ones,
no need to rewrite history.

## Pull Requests

- One subject per PR, linked to the issue or RFC it resolves.
- PR template inherited from `kern-ia/.github`.
- States the semver impact.
- No real personal data.

## Style and lint

`.golangci.yml` — `version: 2`, `linters.default: standard`, deliberately minimal (the
comment at the top of the file says so: widening the set is deliberate future work, not an
oversight). Two documented narrowings (`errcheck` on `io.Closer.Close`/`fmt.Fprint*`,
`staticcheck` without `ST1005`) — each carries its justification inline, to keep as a model
for documenting any future lint exception anywhere in the org.
`max-issues-per-linter: 0`, `max-same-issues: 0`.

## Tests / CI

`.github/workflows/test.yml`: `test` job (`go test ./... -race -v` + an offline test of the
upstream sync script) and `lint` job (`golangci-lint`). `.github/workflows/upstream-sync.yml`
handles the weekly sync with the ported upstream project — a specificity of this repo (a port
of `@earendil-works/pi-ai`), not a pattern to generalize elsewhere in the org.

## Go module

- Current path: `github.com/julienlegoux/kern-link` — consistent with the GitHub account of
  the repo's main author, but diverges from the `github.com/kern-ia/...` path one would
  expect for a repo hosted under the organization. Same decision to make at the org level as
  for `kern-ui`/`kern-orch`/`kern-anon` (see the global report) — do not rename unilaterally
  here, it's a module consumed downstream.

## Release / CHANGELOG

> **Gap with org policy**: `kern-ia/.github`'s `CONTRIBUTING.md` explicitly says "there is no
> `CHANGELOG.md`," with release notes living in the annotated tag. `kern-link` nonetheless
> maintains a real, up-to-date `CHANGELOG.md`, used consistently (v0.1.0, v0.1.1 releases
> documented). Two possible outcomes: either `kern-link` stays a documented exception
> (explicitly noted in the org `CONTRIBUTING.md`), or the org generalizes this pattern to all
> repos. To decide, not to leave unaddressed.

## Documentation

- `README.md`, `LICENSE`, `NOTICE` at the root — `NOTICE` is specific to this repo
  (attribution for the ported upstream project).
- No `CLAUDE.md` today, unlike `kern-ui`/`kern-orch`/`kern-anon` — to create if Claude Code
  sessions start working on this repo regularly.

## Security / privacy

See the org-inherited `SECURITY.md`. Particular attention here: `kern-link` receives
ephemeral credentials per call (never persisted credentials in the repo).
