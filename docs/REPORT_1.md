# Issue Review Report 1

## Scope

- Reviewed: `docs/epics/epic-1-repository-hygiene/issues/01-bump-go-directive-to-1-26.md`, `docs/epics/epic-1-repository-hygiene/issues/02-move-module-path-to-kern-ia.md`, `docs/epics/epic-1-repository-hygiene/issues/index.md`
- Reviewed against: `docs/epics/epic-1-repository-hygiene/EPIC_1.md` (the contract), with `docs/epics/index.md`, `docs/planning/SPECS.md`, `docs/planning/CONVENTIONS.md`, `docs/planning/SCOPE.md#milestone-1-repository-hygiene`, `docs/planning/scope/26-module-path-migration.md`, root `CONVENTIONS.md`, `.github/workflows/test.yml`, `.golangci.yml` and the working tree at `59e9c5b` (plus the tree at `318731c`, which issue 02 cites) read for context. `docs/planning/DRIFT.md` does not exist; no `CLAUDE.md`, no `.github/ISSUE_TEMPLATE/`, no `.github/PULL_REQUEST_TEMPLATE.md` in the repo.
- GitHub verification: verified (`kern-ia/kern-link` — issues #127, #128, tracking issue #118, milestone 18)
- External review: `openai/gpt-5.6-luna` (via `opencode run --agent plan`)

## Findings

### P2 — Epic AC 2's single exception is widened by issue 02 to two whole directories

- Location: `docs/epics/epic-1-repository-hygiene/issues/02-move-module-path-to-kern-ia.md:96` (AC grep excludes `':!docs/planning' ':!docs/epics'`), reinforced at `:79-82` ("The planning and epics bundles' historical records … are out of scope")
- Source: `docs/epics/epic-1-repository-hygiene/EPIC_1.md:49-50` — "No reference to `github.com/julienlegoux/kern-link` remains anywhere in the tree, **except in historical CHANGELOG entries for already-released versions**."
- Problem: the epic grants exactly one exception; the issue grants three (CHANGELOG plus the entire `docs/planning/` and `docs/epics/` trees). Measured on the tree at `59e9c5b`, that leaves 23 old-path occurrences alive outside CHANGELOG (`docs/planning/` 12, `docs/epics/` 11) that epic AC 2 as written forbids.
- Impact: epic AC 2 can never be checked off honestly after issue 02 merges. `close-epic` and any later reviewer read a criterion that the shipped work deliberately does not meet, with nothing on record saying the epic was superseded.
- Recommendation: pick one side and make the two files agree. Either extend the epic's AC 2 exception to `docs/planning/` and `docs/epics/` (the issue's reasoning at `:79-82` is sound — those records exist to state the old path), or, if the epic is to stand unchanged, add an explicit "Assumption / deviation from epic AC 2" note in issue 02 naming `EPIC_1.md:49-50` so the widening is a recorded decision rather than a silent one. Note that `EPIC_1.md:28` itself carries the old path, so the epic's AC is already self-contradictory — see Open questions.

### P2 — Epic AC 4 demands a single PR; the epic is split into two

- Location: `docs/epics/epic-1-repository-hygiene/issues/01-bump-go-directive-to-1-26.md:26-31` ("It ships first and alone, ahead of the module-path rename in issue 02") and `…/02-move-module-path-to-kern-ia.md:112-113`
- Source: `docs/epics/epic-1-repository-hygiene/EPIC_1.md:53-54` — "The change lands as a single `refactor!:` commit-scoped PR"; `EPIC_1.md:34` — "Commit as `refactor!: move module path to github.com/kern-ia/kern-link`."
- Problem: the epic asks for one PR carrying both go.mod changes (AC 1 pairs `module github.com/kern-ia/kern-link` **and** `go 1.26` in the same criterion). The issues deliver two PRs with two commit types. Issue 02:112-113 restates AC 4 for itself only, so the criterion silently narrows from "the epic" to "the rename". No issue flags the split as a deviation.
- Impact: the split is well argued (a toolchain bump surfacing a real failure inside a 170-file mechanical diff is genuinely bad), but as recorded it contradicts an explicit acceptance criterion. Separately, epic AC 1's *combined* end state is verified by neither issue: issue 01 asserts `go 1.26` before the rename exists (`01-bump-go-directive-to-1-26.md:57-59`), and issue 02's AC checks only `head -1 go.mod` (`02-move-module-path-to-kern-ia.md:94`) — nothing re-asserts that `go 1.26` survived the rename PR.
- Recommendation: add one sentence to issue 01's Summary naming `EPIC_1.md:53-54` and stating that the epic ships as two PRs, with the `refactor!:` requirement applying to the rename PR. And add to issue 02's acceptance criteria: `grep -c '^go 1\.26$' go.mod` returns `1` — so epic AC 1 has one checkable owner.

### P2 — Issue 01 mandates a `build:` commit type the repo's conventions do not use

- Location: `docs/epics/epic-1-repository-hygiene/issues/01-bump-go-directive-to-1-26.md:72-74` — "The commit follows Conventional Commits **with a scope**, e.g. `build: bump the go directive to 1.26`."
- Source: `docs/planning/CONVENTIONS.md:203-204` — "Types in use: `feat`, `fix`, `refactor`, `docs`, `ci`, `chore`. Scopes name the package or area (`ai`, `apis`, `bedrock`, `epics`, `release`)."
- Problem: `build` is not in the repo's observed type set, and the example given carries no scope at all, contradicting the same bullet's own "with a scope" requirement.
- Impact: this is an acceptance criterion, so the implementer will follow it literally and land the first commit of the program off-convention — in an epic whose entire point is repository hygiene.
- Recommendation: change the example to a type the repo actually uses with a real scope, e.g. `chore(go): bump the go directive to 1.26` or `ci(go): bump the go directive to 1.26`.

### P2 — Issue 02's SPECS.md rewrite is in Scope but has no acceptance criterion

- Location: `docs/epics/epic-1-repository-hygiene/issues/02-move-module-path-to-kern-ia.md:59-62` (Scope: rewrite `docs/planning/SPECS.md:22` for both the module path and `Go **1.25.0**` → `Go **1.26**`, plus `:247`) vs its acceptance criteria at `:95-101`
- Source: same file — the AC grep at `:96` excludes `':!docs/planning'`, and the follow-up AC at `:98-101` only requires surviving `docs/planning/` hits to be "a decision, scope, log or epic record".
- Problem: the two SPECS.md edits the Scope explicitly requires are exempted from every check the issue defines. Issue 01 also deliberately defers its own SPECS.md line to issue 02 (`01-bump-go-directive-to-1-26.md:48-50`), so if issue 02 skips it, nothing anywhere catches a planning doc that still claims Go 1.25.0 and the old module path.
- Impact: a hand-edit inside an excluded directory is exactly the step a `sed`-driven PR forgets, and `docs/planning/SPECS.md` is the file every later epic reads for the stack.
- Recommendation: add an explicit criterion — `git grep -n 'julienlegoux/kern-link' -- docs/planning/SPECS.md` prints nothing, and `docs/planning/SPECS.md:22` reads `github.com/kern-ia/kern-link` at Go **1.26**.

### P3 — Issue 02's reference and file counts do not reproduce from the command it cites

- Location: `docs/epics/epic-1-repository-hygiene/issues/02-move-module-path-to-kern-ia.md:119-121` ("Verified against the tree at `318731c` (`git grep -o 'julienlegoux/kern-link' | wc -l` → **370**")), `:29` ("370 references across 170 files"), `:126` ("`ai/providers` (42 files)"), `:44-45` ("157 `.go` files … 35 provider bindings", "`README.md` (7 refs)")
- Source: the tree itself. At `318731c` the cited command returns **369**, not 370 (366 for the `github.com/`-prefixed form the rename actually targets). `ai/providers` has **44** files carrying the path, not 42. `README.md` has **9** occurrences on 7 matching lines — `README.md:3` and `:4` each carry two — so "7 refs" is a `grep -c` line count while the headline 370 is a `grep -o` occurrence count, mixing two units in one section. (157 `.go` files and 170 total files are both correct.)
- Impact: low on its own, but the numbers are the issue's stated verification basis, and the headline 370/170 counts the CHANGELOG, `docs/planning/` and `docs/epics/` hits the issue itself excludes — in-scope at `59e9c5b` it is ~350 occurrences across 162 files. An implementer sizing the PR from that figure over-estimates the diff.
- Recommendation: re-derive with the exclusions applied and print the exact command, e.g. `git grep -o 'github.com/julienlegoux/kern-link' -- ':!CHANGELOG.md' ':!docs/planning' ':!docs/epics' | wc -l`, and fix `42` → `44`. Counts drift as `docs/` grows, so stating the command matters more than the number.

### P3 — Root `CONVENTIONS.md` line citation is off by one

- Location: `docs/epics/epic-1-repository-hygiene/issues/02-move-module-path-to-kern-ia.md:54` — "**`CONVENTIONS.md:72-77`** (repo root) — the `## Go module` section"
- Source: `CONVENTIONS.md:71` is the `## Go module` heading; `:72` is blank; the bullet runs `:73-77`.
- Problem: the cited range starts on a blank line and omits the heading it names.
- Impact: trivial, but the issue asks the implementer to *replace the section*, and the range given does not contain the section header.
- Recommendation: cite `CONVENTIONS.md:71-77`.

### P3 — `issues/index.md` uses relative links where the bundle rule requires bundle-absolute

- Location: `docs/epics/epic-1-repository-hygiene/issues/index.md:3-4` — `[…](./01-bump-go-directive-to-1-26.md)`, `[…](./02-move-module-path-to-kern-ia.md)`
- Source: `_shared/bundle-interfaces.md`, "The two bundles and their link rules" — within a bundle, links use a bundle-relative absolute path with a leading `/` (`[Epic 1](/epic-1-core-crud/EPIC_1.md)`). `docs/epics/index.md:12-20` follows this; so do the cross-references inside both issue bodies (`01-bump-go-directive-to-1-26.md:27`, `…:90`, `02-move-module-path-to-kern-ia.md:61`, `:145`).
- Problem: the issues index is the only file in the bundle using the `./` form.
- Impact: renders fine today; it is an internal inconsistency an OKF validator would flag.
- Recommendation: rewrite the two bullets as `/epic-1-repository-hygiene/issues/01-bump-go-directive-to-1-26.md` and `…/02-….md`.

### P3 — `depends_on: [1]` does not match `issue: 01`

- Location: `docs/epics/epic-1-repository-hygiene/issues/02-move-module-path-to-kern-ia.md:14` — `depends_on: [1]`
- Source: `docs/epics/epic-1-repository-hygiene/issues/01-bump-go-directive-to-1-26.md:8` — `issue: 01`; `_shared/pipeline-interfaces.md`, "Issue file" — `issue: <nn>` zero-padded, `depends_on: []` holding "other issue numbers in this epic".
- Problem: the two fields express the same identifier in two forms. `01` is a leading-zero scalar and `1` a plain integer; a consumer comparing them without normalisation resolves the dependency to nothing.
- Impact: the human meaning is unambiguous; only automated dependency resolution is at risk.
- Recommendation: pick one representation across the bundle — either `depends_on: ["01"]` matching the `issue` field, or `issue: 1`. Whichever, apply it uniformly across all nine epics, not just this one.

### P3 — Branch naming and status-commit conventions are absent from both bodies

- Location: neither issue body mentions them — `01-bump-go-directive-to-1-26.md:55-74` and `02-move-module-path-to-kern-ia.md:92-115` cover test, lint, `gofmt`, commit type and trailer conventions but stop there.
- Source: `docs/planning/CONVENTIONS.md:216-218` — feature branches are named `issue-<NN>-<slug>`; `docs/planning/CONVENTIONS.md:224-227` — each status transition gets its own `docs(epics): …` commit, separate from the implementation commit.
- Problem: both bodies are otherwise unusually thorough about repo conventions (they correctly cite the `GOTMPDIR` rule, the CI-only `-race` rule, the pinned `golangci-lint v2.12.2`, and the no-tool-signature-trailer rule), which makes these two omissions read as "not required" rather than "assumed".
- Impact: minor — `implement-issue` supplies both — but issue 01 opens the first branch of a nine-epic program, and a wrong branch name there propagates as the pattern everyone copies.
- Recommendation: add one line to each issue's Dependencies or PR-size note: branch `issue-127-bump-go-directive` / `issue-128-move-module-path`, status transitions recorded in separate `docs(epics): …` commits.

## Coverage notes

- **GitHub state is fully consistent with the files.** #127 and #128 exist with titles byte-identical to the `title:` frontmatter of `01-…` and `02-…`, both `OPEN` matching `status: open`, both on milestone 18 "Epic 1: Repository hygiene" matching `EPIC_1.md:11`, both carrying `enhancement` + `upstream-sync` and correctly *not* carrying `epic` (which is reserved for tracking issue #118). Both are native sub-issues of #118 per `repos/kern-ia/kern-link/issues/118/sub_issues`. Milestone 18 shows 3 open issues (#118, #127, #128) and 0 closed — exactly what the local files predict. `resource:` URLs resolve to the right issues; `issues/index.md:3-4` cites the right numbers.
- **Frontmatter is otherwise schema-clean.** Every field `pipeline-interfaces.md` requires is present with the right type in both issues, `resource` appears only because the GitHub issues exist, `gh_pr` is correctly absent (implement-issue owns it), and no unsanctioned extension field was added. `issues/index.md` carries no frontmatter, correct for a non-root index. Cross-bundle links (`../../../planning/CONVENTIONS.md`, `…/scope/19-constraints.md`, `…/scope/26-module-path-migration.md`) all resolve to real files.
- **No invented scope.** Neither issue promises an endpoint, a feature, a test suite or a refactor the epic does not ask for. Issue 02:69-91 restates every one of the epic's out-of-scope items (`EPIC_1.md:38-44`: no `/v2` suffix, no CHANGELOG history rewrite, no semantic change) and adds four further guards — no compatibility shim, no `[Unreleased]` entry, no `develop`→`dev` branch rename, no v0.2.0 tag — each traced to a decision file.
- **Epic scope bullets and AC 3 are covered.** `EPIC_1.md:28-33` (module path, go directive) map cleanly onto issues 02 and 01. `EPIC_1.md:51-52` (CI green: `go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint` pinned v2.12.2) is restated in both issues' ACs and matches the repo exactly — `.github/workflows/test.yml:42` pins `v2.12.2`, `docs/planning/CONVENTIONS.md:233-236` lists the three gates.
- **Ordering and sizing are right.** `depends_on: []` → `depends_on: [1]` is coherent and the go.mod-contention rationale (`01-…:26-31`) holds. `size: S` for a one-line change and `size: M` for ~350 mechanical replacements match `pipeline-interfaces.md`'s bands, and `02-…:157-161` correctly argues that atomicity, not the ~1000-line ceiling, governs the rename — an intermediate split state does not compile.
- **Factual citations that check out** (verified against `59e9c5b` / `318731c`): `go.mod:1` module line, `go.mod:3` `go 1.25.0`, `CHANGELOG.md:20` (historical, kept) and `:63-65` (link refs, updated), `docs/planning/SPECS.md:22` and `:247`, `docs/planning/CONVENTIONS.md:215` (`Merge pull request #N from julienlegoux/<branch>`, correctly excluded as merge history rather than module path), `actions/setup-go@v6` with `go-version-file: go.mod` in both jobs (`test.yml:13,15,33,35`), `.golangci.yml` `version: "2"` / `linters.default: standard`, the "widening the linter set is deliberate future work" claim (`.golangci.yml:3-6`, `docs/planning/CONVENTIONS.md:89-95`), the no-tool-signature-trailer rule (`CONVENTIONS.md:44`), the `GOTMPDIR`-inside-the-repo and CI-only `-race` rules (`docs/planning/CONVENTIONS.md:186-187`), CHANGELOG-at-release-prep (`docs/planning/CONVENTIONS.md:239`), 157 affected `.go` files, and the absence of the module path from `.github/workflows/*`, `go.sum`, `upstream/` and `.golangci.yml`.
- **Body quality is high.** Both issues are action-titled and every section is concrete enough to work from: acceptance criteria are shell commands with expected output rather than prose, the "Relevant files" sections name real paths and line numbers, and issue 02's practical note (`:135-140`) tells the implementer exactly how to run the replacement around the exclusions. The scope reconciliation at `02-…:119-121` — noticing that the epic's 355 is stale and re-counting — is the right instinct even though the new number is also off.

## Open questions

- `EPIC_1.md:28` itself contains `github.com/julienlegoux/kern-link`, and so do the nine epic files and every `docs/planning/scope/*.md` decision. Epic AC 2 as written therefore cannot be satisfied by any implementation, including one that ignores issue 02's exclusions. Whether `docs/epics/` and `docs/planning/` were ever meant to be in scope is a question for the epic, not for these issues — it is recorded here only because finding 1 rests on it.
- Neither issue mentions that `Closes #N` will not auto-close #127/#128, since PRs target `develop` rather than the default branch (`_shared/pipeline-interfaces.md`, "GitHub facts on non-default integration branches"). `implement-issue` handles the explicit close, so this may be a deliberate omission; flagging it only because Epic 1 is the first epic to exercise the path.
