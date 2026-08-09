# Issue Review Report 6

## Scope

- Reviewed: `docs/epics/epic-6-auth-core-and-env-api-key-bindings/issues/01-auth-contract-surface.md`
  through `11-porting-paths-and-dispositions.md` (11 files) plus
  `docs/epics/epic-6-auth-core-and-env-api-key-bindings/issues/index.md`.
- Reviewed against: `docs/epics/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md`
  (the contract), with `docs/planning/SCOPE.md` (§ *Milestone 6*, the epic's
  `source`), `docs/planning/SPECS.md`, `docs/planning/CONVENTIONS.md`,
  `docs/PORTING.md`, `docs/auth.md`, `docs/epics/index.md` and the referenced
  Epic 2/3/5/7/8/9 issue files read for context. `docs/planning/DRIFT.md` does
  not exist yet; no repo `CLAUDE.md`, `CONTRIBUTING.md`, issue template or PR
  template is present (`CONVENTIONS.md:228` records their absence).
- GitHub verification: **verified**. Milestone 23 ("Epic 6: Auth core and
  env-API-key bindings") is open with 12 open issues. Issues #170–#180 all
  exist, are `OPEN`, carry milestone 23, and their titles match both file
  frontmatter and `issues/index.md` byte for byte. All eleven are native
  sub-issues of tracking issue #123 (`gh api
  repos/kern-ia/kern-link/issues/123/sub_issues` returns exactly those eleven,
  in order). Labels: `enhancement` + `upstream-sync` on #170–#179,
  `documentation` + `upstream-sync` on #180; the `epic` label is correctly
  reserved for #123. Cross-epic GitHub numbers cited in the bodies (#144, #159,
  #169) resolve to the files claimed.
- External review: `openai/gpt-5.6-sol` via `opencode --agent plan`. The
  coverage pass and the ordering/dependency/sizing pass both returned usable
  output and their leads were verified against the files before entering this
  report. The schema/body-quality pass returned nothing on two attempts (the
  external session aborted on a skill load, then produced an empty result), so
  schema, links and conventions were reviewed natively — including on-disk
  resolution of all 28 distinct link targets and spot-verification of ~30
  `file:line` citations against the Go tree.

## Findings

### P1 — Four issues build upstream files the epic never names, and issue 08 contradicts the epic's Out of scope

- Location: `issues/08-anthropic-codex-login-race.md:21-22` and `:63-87`;
  `issues/09-copilot-model-availability.md:52-75`;
  `issues/05-models-availability.md:29-66`;
  `issues/06-models-login-logout.md:68-86`.
- Source: `EPIC_6.md:27-28` (Scope: the six files) and `EPIC_6.md:40-42` (Out of
  scope: "**No Go package moves.** … Mirroring the move would be churn with no
  content behind it").
- Problem: the epic's Scope names exactly six upstream files
  (`auth/credential-store.ts`, `auth/helpers.ts`, `auth/resolve.ts`,
  `auth/types.ts`, `env-api-keys.ts`, `oauth.ts`). Issues 08 and 09 port content
  from three files not on that list — `src/auth/oauth/anthropic.ts`,
  `src/auth/oauth/openai-codex.ts`, `src/auth/oauth/github-copilot.ts` — and
  issue 08 opens by asserting the opposite of `EPIC_6.md:41-42`: "Upstream moved
  `utils/oauth/anthropic.ts` and `utils/oauth/openai-codex.ts` to `auth/oauth/`
  **with edits**" (`08:21-22`), then deletes the optional manual-code branch,
  `redirectUriForExchange` and the text-prompt fallback from both flows
  (`08:65-74`). Issues 05 and 06 build the whole `src/models.ts` auth surface
  (`CheckAuth`, `GetAvailable`, `Provider.FilterModels`, `Login`, `Logout`,
  `GetAuthForProvider`) and rewire `cmd/pi-ai/oauth.go`; `models.ts` is not on
  the epic's list either. That is four of eleven issues, all sized `M`.
- Impact: roughly a third of the epic's PRs land work no epic acceptance
  criterion verifies, so the epic can read "complete" with the OAuth-flow
  simplification half-done, and nothing catches it at close. A reader
  reconciling `EPIC_6.md:41-42` against issue 08 must decide which document is
  wrong. (Issues 05/06 are at least traceable: Epic 2 issue 08 explicitly
  deferred `filterModels`/`getAvailable`/`checkAuth`/`login` here at
  `docs/epics/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md:96-106`
  — the defect is that the epic file never absorbed the hand-off.)
- Recommendation: amend `EPIC_6.md`. Add `src/models.ts` and the three
  `src/auth/oauth/*.ts` files to `## Scope`, add matching acceptance criteria
  ("the anthropic and codex login flows always race the manual-code prompt";
  "the `Models` auth surface Epic 2 deferred here is complete"), and rewrite
  `EPIC_6.md:41-42` to say what it means: the *directory move* is a no-op, the
  *content edits inside the moved files* are in scope. If the user instead wants
  those files out, issues 08 and 09 move to Epic 7 along with issue 01's
  `IsSubscription` sweep (`01:81-83`).

### P2 — Acceptance criterion 4's "catalog entry" half has no owner in this epic

- Location: `issues/10-env-api-key-bindings.md:43-50` and `:70-72` ("**This
  issue does not create catalog data.**" / Out of scope: "Catalog data — Epic 3
  issue 04").
- Source: `EPIC_6.md:52-53` (AC 4: "The four env-API-key bindings resolve
  credentials from their environment variables **and each has a catalog
  entry**"), restating `docs/planning/SCOPE.md:238-240`.
- Problem: issue 10 is the only issue touching the four bindings and it
  explicitly excludes the catalog half, delegating to
  `docs/epics/epic-3-catalog-schema-and-export-tooling/issues/04-regenerate-model-catalog.md`
  (#144). Issue 10's acceptance criteria (`10:85-97`) check registration, env
  resolution and base URLs only.
- Impact: at epic close, criterion 4 cannot be ticked from Epic 6's own work. If
  #144 slips or drops the four providers, four registered providers serve zero
  models and no Epic 6 check notices.
- Recommendation: add one acceptance criterion to issue 10 —
  `TestNewBindingsServeCatalogModels`: each of the four ids returns a non-empty
  `catalog.BuiltinModels(id)`, skipped-and-named in the PR body if #144 has not
  merged — or move criterion 4's catalog clause into `EPIC_6.md`'s
  `## Dependencies` on Epic 3 so the epic stops claiming it.

### P2 — `depends_on` under-declares two conflicts the bodies do declare

- Location: `issues/06-models-login-logout.md:14` (`depends_on: [1, 3]`) against
  `:155-156` ("Conflicts with issue 05 in `ai/provider.go`. Sequence them; do
  not run both in parallel"); `issues/11-porting-paths-and-dispositions.md:14`
  (`depends_on: []`) against `:76` ("**Land this last.**") and `:132-134`.
- Source: the issue schema — `depends_on` is the machine-readable ordering
  field, and `implement-epic` picks unblocked issues from it, not from prose.
- Problem: issues 05 and 06 both rewrite the `Models` interface and `modelsImpl`
  in `ai/provider.go` (`05:79-88`, `06:70-77`); a supervisor reading
  `depends_on` sees both unblocked once 01 and 03 land and runs them
  concurrently. Issue 11 reconciles `docs/PORTING.md` after issues 03 and 09
  amend it (`03:134-136`, `09:98-100`) but declares no dependency at all, so it
  is eligible to run first.
- Impact: a guaranteed merge conflict in `ai/provider.go`, and a `PORTING.md`
  reconciliation that reconciles nothing because it ran before the amendments.
- Recommendation: set `depends_on: [1, 3, 5]` on issue 06 and
  `depends_on: [3, 9]` on issue 11 (see the P3 finding below — issue 07 does not
  in fact edit `PORTING.md`). Both are backwards references, so numeric order
  still holds.

### P2 — Issue 03 removes an acceptance criterion Epic 5 issue 11 (#169) promises to preserve

- Location: `issues/03-provider-scoped-apikey-resolution.md:36-53` and `:85`
  ("`AuthResult.Auth.BaseURL` is no longer set by these resolvers").
- Source: `docs/epics/epic-5-remaining-adapters/issues/11-cloudflare-stream-classification.md:118-119`
  — acceptance criterion: "The existing `ai/providers/cloudflare_auth.go` tests
  still pass unchanged; auth-time resolution is not removed."
- Problem: Epic 5 precedes Epic 6 in `docs/epics/index.md:16-17`, so #169 merges
  first with that criterion satisfied, and #172 then falsifies it. Issue 03 sees
  the collision and documents a resolution procedure (`03:38-54`), but only in
  its own body — #169 still carries a criterion it cannot keep, and nothing on
  the Epic 5 side records that.
- Impact: whoever implements or reviews #169 treats "auth-time resolution is not
  removed" as a contract and may build the dispatch-time path so that it
  *depends* on the auth-time one, which issue 03 then has to unpick.
- Recommendation: amend `epic-5-…/issues/11-…md:118-119` to "auth-time
  resolution stays until Epic 6 issue 03 (#172) removes it; the dispatch-time
  path must not depend on it", and post the same sentence as a comment on #169.
  Issue 03 already names the reciprocal action (`03:43-45`); make it mutual.

### P2 — Acceptance criterion 5 ("ported upstream tests come across") is unenforced, and three issues share one upstream test file with no owner

- Location: `issues/01-auth-contract-surface.md:141`,
  `issues/04-oauth-refresh-window.md:147`, `issues/05-models-availability.md:160`
  — all three list `test/oauth-auth.test.ts (+67/- in range)` under *Relevant
  files* with no matching acceptance criterion. Same pattern in
  `issues/07-anthropic-auth-token.md:149-150` (`test/anthropic-auth-token.test.ts`,
  new, 187 lines) and `issues/09-copilot-model-availability.md:163`
  (`test/github-copilot-oauth.test.ts` +201).
- Source: `EPIC_6.md:54` (AC 5: "Ported upstream tests come across; new coverage
  is offline and stdlib-only").
- Problem: only issue 08 turns this into a requirement ("The tests that pin the
  old shape … get rewritten to the new one, not deleted wholesale … read those
  diffs", `08:84-87`). Elsewhere the upstream test file is a pointer and the
  acceptance criteria list freshly invented Go tests instead. Where one upstream
  file is named by three issues, each can reasonably assume another covers it.
- Impact: upstream cases with no Go equivalent are silently dropped — precisely
  the parity claim `docs/planning/SCOPE.md`'s governing principle exists to
  protect. Criterion 5 becomes unverifiable at close.
- Recommendation: add one acceptance criterion per issue naming its upstream
  test file and requiring each case to be represented or explicitly
  dispositioned — and split `test/oauth-auth.test.ts` explicitly (types/`Check`
  cases → 01, expiry-window cases → 04, `checkAuth`/`getAvailable` cases → 05)
  so no case is everyone's and nobody's.

### P2 — `docs/auth.md` goes stale and issue 11 routes the fix to an epic that will not make it

- Location: `issues/11-porting-paths-and-dispositions.md:90-91` ("Out of scope:
  Rewriting `docs/auth.md`. Its terms-of-service treatment is per-flow and
  belongs with the flows (Epic 7)").
- Source: `docs/auth.md:36` — the env-key precedence row
  `| anthropic | ANTHROPIC_OAUTH_TOKEN, ANTHROPIC_API_KEY |` — and
  `docs/auth.md:22-24` — "A stored credential *owns* the provider — there is no
  silent env fallback behind it."
- Problem: issue 07 puts `ANTHROPIC_AUTH_TOKEN` first in that precedence list
  and adds a bearer-header credential mode with no api key at all (`07:38-44`,
  `07:66-79`); issue 03 introduces a per-field credential→env fallback for
  Cloudflare, contradicting `docs/auth.md:22-24` (`03:56-64`); issue 10 adds four
  providers to a table that lists every binding (`10:24-29`). Epic 7 issue 06
  covers `docs/auth.md`'s *terms-of-service* sections for the four new flows only
  (`docs/epics/epic-7-four-new-oauth-flows/issues/06-auth-docs-and-porting.md:24-41`)
  — it will not touch the env-key table or the resolution-order prose.
- Impact: the repo's user-facing auth guide documents a resolution order and an
  env-var table this epic makes wrong, with no issue in any epic owning the fix.
- Recommendation: bring `docs/auth.md`'s env-key table and resolution-order
  section into issue 11's scope (it is already the epic's documentation issue and
  already lands last), or add the row edit as an acceptance criterion on issues
  07 and 10 individually.

### P2 — Issue 08 is sized `M` for work that reads as `L`

- Location: `issues/08-anthropic-codex-login-race.md:10` (`size: M`) against its
  own scope at `:65-87` and `:140`.
- Source: issue schema — `S` ≈ under 200 changed lines, `M` ≈ 200–500,
  `L` ≈ 500–1000.
- Problem: the issue rewrites the login race in two files, deletes a branch, a
  helper and an options field from each, adds context-cancellation of the
  callback wait, updates two `// Ports:` headers, and rewrites the two test files
  that pin the old shape — `anthropic_test.go` (415 lines) and `codex_test.go`
  (738 lines), "rewritten to the new one, not deleted wholesale" (`08:84-87`).
  Even a partial rewrite of 1153 lines of tests plus both flows exceeds 500
  changed lines. Issues 05 (`05:67-98`, nine tests) and 06 (`06:68-86`, eight
  tests plus a CLI migration) are also at the top of the `M` band; issue 02 is
  the reverse case — an interface change plus two implementations plus five test
  groups across three production files is doubtful as `S` (`02:51-69`, `:81-104`).
- Impact: sizing is what a developer plans against; an `M` that lands at ~900
  lines is the PR that stops getting reviewed properly, and issue 08 is the
  epic's most behavior-changing PR.
- Recommendation: mark issue 08 `L`, or split it into `08a` (anthropic) and
  `08b` (codex) — the two files share no code and the issue already calls them
  "the same three edits". Re-check 02 against the `S` band.

### P2 — The `## PR size note` is identical boilerplate in all eleven issues, and contradicts the size bands

- Location: every issue file's final section, verbatim: "Target ~500 changed
  lines; if this grows past ~1000, split it before opening the PR" —
  `01:155-156`, `02:128-129`, `03:172-173`, `04:157-158`, `05:177-178`,
  `06:160-161`, `07:160-161`, `08:155-156`, `09:175-176`, `10:126-127`,
  `11:139-140`.
- Source: the schema's size bands and the `size` frontmatter each file already
  sets — `S` on issues 02, 10 and 11 (`02:10`, `10:10`, `11:10`).
- Problem: a section byte-identical across `S` and `M` issues carries no
  information. For the three `S` issues it states a target 2.5× the band's
  ceiling; for the eight `M` issues it licenses growth to 1000 lines, which is
  the `L` band. Issue 11 is a documentation-only PR against a 131-line file.
- Impact: the section meant to keep PRs reviewable says the same thing whether
  the PR is a four-line table edit or a two-flow rewrite, so developers read past
  it — and the `S` issues get explicit permission to triple their intended size.
- Recommendation: make each note concrete and consistent with its own `size`
  (issue 11: "~40 changed lines in `docs/PORTING.md`, no Go code"; issue 02:
  "~150 lines across three files plus tests"), and cap the `M` issues' split
  threshold at ~500 rather than ~1000.

### P3 — Acceptance criterion 2 ("no Go package moved or renamed") is verified nowhere

- Location: no issue file contains a check for it; the nearest statement is
  `issues/08-anthropic-codex-login-race.md:21-24` ("The move is a no-op for this
  port").
- Source: `EPIC_6.md:49` (acceptance criterion 2).
- Problem: a negative criterion with no mechanical check holds only until
  someone, reading upstream's new layout, "tidies" `ai/auth/oauth`. Issues 08 and
  09 are exactly the PRs where that temptation arises.
- Impact: low — easy to eyeball at close — but this is the criterion the epic
  presents as the point of the whole exercise (`EPIC_6.md:20-23`).
- Recommendation: add to issue 11 (which lands last and already runs
  `grep`-shaped checks): `git diff --name-status <epic-base>..HEAD -- ai/auth/`
  shows no `R` (rename) entries.

### P3 — `// Ports:` header maintenance is a DoD item in six issues and absent from four that edit ported files

- Location: required by `01:120-125`, `05:144-146`, `07:133-135`, `08:80-83` and
  `:124`, `09:89-91`, `10:66`. Absent from `issues/02-credential-store-list.md`
  (edits `ai/auth.go`, `ai/credentialstore.go`, `ai/auth/filestore.go`),
  `issues/03-provider-scoped-apikey-resolution.md` (edits `ai/auth.go`,
  `ai/resolve.go`, `ai/provider.go`, `ai/providers/cloudflare_auth.go`),
  `issues/04-oauth-refresh-window.md` (`ai/resolve.go`) and
  `issues/06-models-login-logout.md` (`ai/provider.go`, `cmd/pi-ai/oauth.go`).
- Source: `docs/planning/CONVENTIONS.md:50-66` ("Every ported file carries a
  provenance header … Absence of the header is therefore meaningful") and
  `EPIC_6.md:73-75`.
- Problem: issue 03 changes what `ai/resolve.go` ports (`resolveProviderAuth` no
  longer takes a model) and issue 06 changes what `cmd/pi-ai/oauth.go` does, but
  neither requires the header to still describe the file afterwards.
- Impact: headers drift from the code they claim to map — the input
  `docs/PORTING.md` and the next sync are read against.
- Recommendation: add the same one-line DoD item ("the file keeps its `// Ports:`
  header and it still describes the file") to issues 02, 03, 04 and 06.

### P3 — Issue 11 names issue 07 as amending `docs/PORTING.md`; issue 07 explicitly does not

- Location: `issues/11-porting-paths-and-dispositions.md:74-76` ("issues 03, 07
  and 09 each touch it").
- Source: `issues/07-anthropic-auth-token.md:59-62` ("`docs/PORTING.md:59`
  describes `src/env-api-keys.ts` as ported … the row itself is issue 11's") and
  its Scope at `:64-87`, which contains no `docs/PORTING.md` edit.
- Problem: issue 11 will look for a row change from issue 07 that never happened,
  and — worse — may assume the `src/env-api-keys.ts` row was already handled.
- Impact: the `env-api-keys.ts` row (`docs/PORTING.md:59`) is the one row whose
  claim ("distributed: each binding declares its own env-var precedence list")
  issue 07 makes newly load-bearing; if both issues think the other owns it, it
  goes unedited.
- Recommendation: change `11:74-76` to "issues 03 and 09 each touch it" and add
  the `env-api-keys.ts` row rewrite explicitly to issue 11's own scope list.

### P3 — Three small internal inconsistencies

- `issues/01-auth-contract-surface.md:23-24` says "the four issues that consume
  it (05, 06, 08, 09)" while `:146-150` lists five blocked issues, including 03.
  Make the Summary say five, or say "four consumers plus issue 03, which
  rewrites the same declaration".
- `issues/10-env-api-key-bindings.md:47-48` — "`catalog.BuiltinModels(id)`
  returns an empty slice for a provider with no data file
  (`ai/catalog/catalog.go:86-89`)". The function returns `byProvider[provider]`,
  which is **nil**, and its own doc comment says "or nil if provider is unknown"
  (`ai/catalog/catalog.go:84-89`). Behaviourally equivalent for `range`, but a
  test written to the stated claim would fail. Say "nil".
- `issues/01-auth-contract-surface.md:43` reaches `docs/PORTING.md` as
  `../../../../docs/PORTING.md` while its siblings reach `docs/planning/` as
  `../../../planning/…` (`08:24`, `09:61`, `10:75`, `11:26`). Both resolve on
  disk; use `../../../PORTING.md` for consistency.

## Coverage notes

- **GitHub state is exactly right.** Eleven issues, eleven native sub-issues of
  #123, one milestone, titles matching frontmatter and index bullets character
  for character, `status: open` + `gh_issue` + `resource` consistent with the
  lifecycle in every file, `epic` label on #123 only. This is the part that
  usually breaks and it does not.
- **Frontmatter conforms** in all eleven files: `type`, `title`, `description`,
  `tags: [epic-6]`, ISO-8601 `timestamp`, `epic`, `issue`, `slug`, `size`,
  `status`, `gh_issue`, `resource`, `depends_on`, and no extension fields. Body
  headings are present, correctly named and in order in all eleven.
- **Every link resolves.** All 22 distinct bundle-relative targets
  (`/epic-N-…/…`) and all 6 cross-bundle relative targets were checked on disk;
  none is dangling, and the two link forms are used in the right places.
  `issues/index.md` carries no frontmatter and has one mechanical bullet per
  issue in numeric order, in the same form every other epic uses. (The external
  pass flagged the `/epic-N-…` links as "not filesystem-resolvable"; that is a
  misreading of the bundle-relative link rule and is not a finding.)
- **`file:line` citations into the Go tree are accurate**, sampled across ~30 of
  them: `ai/auth.go:52/146/218/232/240`;
  `ai/resolve.go:48/56/67/141/188/206`; `ai/provider.go:15/47/148/221/239`;
  `ai/auth/helpers.go:17/30` and `:31-36` (the stored-credential branch,
  exactly); `ai/auth/filestore.go:59/76/111/123`;
  `ai/providers/cloudflare_auth.go:24/42`; `ai/auth/oauth/anthropic.go:134`,
  `codex.go:533/542`; `ai/apis/anthropic/anthropic.go:331/1215`. Issue 03's claim
  of "13 `Resolve: func` sites" is exactly right; `docs/PORTING.md` rows
  40/42/43/45/59 are the rows the issues say they are; issue 01's "all 35
  existing bindings" and issue 10's "the total count is 39" both check out
  against `ai/providers/all.go`.
- **Ordering is sound as written**: every `depends_on` points backwards
  (`03→[1]`, `04→[3]`, `05→[1,3]`, `06→[1,3]`, `07→[3]`, `08→[1]`, `09→[1]`;
  01, 02, 10, 11 free), there is no cycle, and a developer working 01→11 in
  numeric order never hits a later prerequisite. The gaps are the *undeclared*
  conflicts above, which bite only under parallel execution.
- **Project conventions are genuinely reflected, not name-dropped.** Every issue
  carries the local test command with `GOTMPDIR=$PWD/.gotmp`, the three CI gates
  and `golangci-lint v2.12.2` exactly as `CONVENTIONS.md:182-189` and `:230-236`
  define them; Conventional Commit examples use the right types, scopes and `!`;
  error strings are treated as load-bearing with byte-identical text mandated
  (`03:73-77`, `04:79-82`, `05:96-98`, `06:34-51`) per the ST1005 rule at
  `CONVENTIONS.md:108-111`; tests are named `TestSomeSpecificBehavior`, asserted
  `got`/`want`, driven through `faux` and `httptest` (`05:156-157`,
  `07:126-129`). Issue 05's "`errgroup`-free `sync.WaitGroup` fan-out"
  (`05:89-92`) correctly honours the no-new-dependencies rule —
  `golang.org/x/sync` is indeed absent from `go.mod`. Issue 04's insistence on
  overriding `authClock` rather than sleeping (`04:58-61`) matches
  `CONVENTIONS.md:162-167`.
- **Acceptance criterion 3 is fully owned** by issue 11, with a mechanical check
  (`grep -n "utils/oauth" docs/PORTING.md`) and a `git diff --name-only` sweep
  forcing every in-range auth path into the table (`11:95-102`). Criterion 6 (CI
  green) appears in all eleven issues.
- **Criterion 1's `oauth.ts` clause is answered, not dropped**: `11:40` records
  that `src/oauth.ts` is 10 lines of type-only re-export at `936aff00`, so "port
  its content" correctly resolves to a mapping-row fix.
- **Out-of-scope discipline is good**: no issue builds anything the epic
  excludes. The four new OAuth flows stay in Epic 7 (`08:100`, `10:74`), radius
  stays in Epic 8 (`07:93-96`, `10:73-75`), and the cross-epic dependency claims
  match `docs/epics/index.md` and `EPIC_7.md:61` in both directions.
- **The bodies are unusually actionable.** Each explains *why* upstream changed,
  names the exact Go seam, and lists named test functions with the assertion each
  must make. Issue 04's `TestDefaultWindowDoesNotRejectShortLivedRefresh` —
  "without this test the previous one over-constrains" (`04:121-123`) — and issue
  09's refusal to accept "undispositioned again" as an outcome (`09:68-75`) are
  the kind of specificity that survives contact with an implementer.

## Open questions

- Does `src/auth/helpers.ts` have any in-range delta beyond the one line issue 07
  claims (`env: credential.env` on the stored-credential branch, `07:70-74`)? It
  is one of the epic's six named files and no issue asserts the rest of it is
  unchanged. A one-line confirmation in issue 07's PR body would close AC 1.
- Issue 09 leaves the `FilterModels`-on-Copilot decision to the implementer
  (`09:68-75`), which changes both the PR's size (S vs M) and whether it depends
  on issue 05. If the recommendation is binding, say so in the Scope and set
  `depends_on: [1, 5]`; if it is genuinely open, the epic should expect either
  outcome at close.
- `EPIC_6.md:41-42`'s "no content behind it" and `08:21-22`'s "with edits" cannot
  both be true of the same upstream commit. Which reflects the audit that
  produced `docs/planning/scope/08-auth-restructure.md`?
