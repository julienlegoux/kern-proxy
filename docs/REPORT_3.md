# Issue Review Report 3

## Scope
- Reviewed: `docs/epics/epic-3-catalog-schema-and-export-tooling/issues/*.md` (6 issue
  files: `01-export-catalog-spike.md`, `02-export-catalog-json-input.md`,
  `03-catalog-validation-0-84-1.md`, `04-regenerate-model-catalog.md`,
  `05-regenerate-image-catalog.md`, `06-images-adapter-retry.md`) plus
  `issues/index.md`
- Reviewed against: `docs/epics/epic-3-catalog-schema-and-export-tooling/EPIC_3.md`
  (the contract), with `docs/planning/SCOPE.md` (Milestone 3, Out of scope, Risks),
  `docs/planning/SPECS.md`, `docs/planning/CONVENTIONS.md`,
  `docs/planning/scope/14-images-surface.md`, `.../10-new-provider-bindings.md`,
  `docs/epics/index.md`, and the neighbouring epic 2 / 6 / 7 issue bundles read for
  cross-epic ownership. No `docs/planning/DRIFT.md` exists yet; no
  `.github/ISSUE_TEMPLATE/` or `PULL_REQUEST_TEMPLATE.md` exists (only
  `.github/workflows`), so no repo issue/PR template applies.
- GitHub verification: verified - milestone 20 ("Epic 3: Catalog schema and export
  tooling") exists; tracking issue #120 is OPEN, on milestone 20, labelled
  `epic` + `upstream-sync`; issues #141-#146 all exist, all OPEN, all on milestone
  20, all six titles byte-match their local `title:` frontmatter, and all six are
  native sub-issues of #120 (`gh api repos/kern-ia/kern-link/issues/120/sub_issues`
  returns exactly 141-146 in order); labels follow the repo scheme (`documentation`
  on the docs-only spike, `enhancement` on the other five, `epic` on none of them).
  No discrepancies found.
- External review: `openai/gpt-5.6-terra` via `opencode run --agent plan`. The first
  pass returned no analysis (the external session tried to load the skill's shared
  interface files from outside its `--dir` and stopped after auto-rejection); the
  mandated retry returned a usable pass, whose leads were each re-verified against
  the files before entering this report. Leads 9 (cross-epic `depends_on`) and 11
  (provenance headers) were rejected on verification and are not reported.

## Findings

### P1 - The `ImagesOptions` to `ProviderRequestOptions` reshape is deferred in a circle and has no owner
- Location: `docs/epics/epic-3-catalog-schema-and-export-tooling/issues/05-regenerate-image-catalog.md:76-77`
  and `.../issues/06-images-adapter-retry.md:93-96`
- Source: `EPIC_3.md:37-39` (the images surface, incl. `images-models.ts`, rides along
  in this epic); `docs/planning/scope/14-images-surface.md` verdict ("The forced part -
  `ai/images` compiling against the new `ProviderRequestOptions<ImagesModel<ImagesApi>>`
  - happens regardless")
- Problem: both epic-3 issues that touch `ai/images` push the `ImagesOptions` reshape
  and the `fetch` override to epic 2 issue 05. That issue pushes it straight back:
  `docs/epics/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md:91-94`
  reads "**`ai/images`.** Upstream reshaped `ImagesOptions` onto the same base; Epic 3
  owns the images surface (decision 14). Touch it only if the refactor breaks
  compilation." `ai/images.Options` (`ai/images/types.go:96-102`) is a standalone Go
  struct that does not embed `ai.StreamOptions`, so epic 2's refactor will *not* break
  its compilation - the conditional never fires and nobody builds the reshape.
- Impact: decision 14's only *forced* item silently lands nowhere. Epic 3 closes with
  its images-surface scope bullet unfulfilled and no issue, drift record, or open
  question saying so.
- Recommendation: give it one owner. Either add an epic-3 issue (natural home: beside
  issue 06, which already opens `ai/images/openrouter.go`'s transport) covering
  `ImagesOptions` on the new base plus the `fetch` override, or amend epic 2 issue 05's
  out-of-scope to claim it unconditionally and have epic-3 issue 05 point at that
  specific epic-2 issue number.

### P1 - Issue 06 relocates `ai/apis/internal/httpretry` repo-wide: invented scope that contradicts CONVENTIONS.md and collides with epics 4 and 5
- Location: `.../issues/06-images-adapter-retry.md:61-69` (the move plus the `Config`
  widening), `:129-132` (four text adapters' retry tests must still pass),
  `:135-138` (edit `docs/planning/SPECS.md`)
- Source: `EPIC_3.md:32-39` (scope: export tooling, catalog JSON, `ai/catalog`
  validation, images surface) and `EPIC_3.md:56-59` (AC 5 asks only that *any change to
  the OpenRouter images adapter* gets an offline `httptest` case);
  `docs/planning/CONVENTIONS.md:22-26`, which names `ai/apis/internal/httpretry` as part
  of the decided package layout; `docs/planning/SPECS.md:73` (package-map row)
- Problem: the epic authorises changing the OpenRouter images adapter. It does not
  authorise moving a shared internal package and rewriting the call sites in
  `ai/apis/anthropic`, `ai/apis/azure`, `ai/apis/bedrock` and `ai/apis/codex` - packages
  that Epic 4 (azure, codex) and Epic 5 (anthropic, bedrock) rewrite wholesale. Nothing
  in issue 06, `EPIC_3.md`, or epics 4/5 declares that collision. The move also
  contradicts a decided convention, and issue 06's remediation (`:135-138`) edits
  `SPECS.md` only - `CONVENTIONS.md:22-26` carries the same now-stale path and is not
  mentioned anywhere in the issue.
- Impact: a catalog epic ships a cross-cutting refactor of the retry layer at the exact
  moment two other epics are rewriting the same four adapter packages, guaranteeing
  rebase pain, and leaves `CONVENTIONS.md` asserting a package path that no longer
  exists.
- Recommendation: either (a) keep the move but make it explicit - add it to
  `EPIC_3.md`'s `## Scope`, add `CONVENTIONS.md:22-26` to issue 06's
  `## Relevant files`, and add a coordination note naming Epic 4 and Epic 5; or
  (b) scope issue 06 to the images adapter alone by exposing the retry loop through a
  thin exported shim, deferring the relocation to Epic 9, which already owns the
  `provider-retry.ts` reconciliation (`06:83-87`).

### P2 - `images-models.ts`'s `getAuth` reshape is named in the epic's scope but owned by no issue in any epic
- Location: `.../issues/05-regenerate-image-catalog.md:70-75` ("That is the auth
  restructure (decision 08, Epic 6); `ai/images/provider.go:213-218` keeps its current
  signature here.")
- Source: `EPIC_3.md:37-39` - `images-models.ts` is listed in the epic's scope;
  `docs/planning/scope/14-images-surface.md` "In range: `src/images-models.ts` changed"
- Problem: the deferral target does not accept it. `docs/planning/scope/08-auth-restructure.md`
  contains no mention of images at all; `docs/epics/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md`
  contains no mention of images; and epic-6 issue 10 explicitly hands the images files
  *back* - `docs/epics/epic-6-auth-core-and-env-api-key-bindings/issues/10-env-api-key-bindings.md:76-78`
  lists `openrouter-images.ts` under out-of-scope, citing decision 14 (i.e. epic 3). The
  only epic-6 contact with `ai/images` is a mechanical `Resolve` literal edit in test
  files (`.../issues/03-provider-scoped-apikey-resolution.md:94,152`), which is not the
  `getAuth(providerId | model, overrides)` overload.
- Impact: an epic-scope bullet has no owning issue anywhere in the program; a reader
  following the deferral chain lands on an epic that disclaims it.
- Recommendation: verify against upstream at `936aff00` whether the `getAuth` overload
  has any Go-visible consequence for `ai/images/provider.go`. If it does, add an issue
  here or an explicit line in an epic-6 issue claiming it; if it does not, say so in
  `EPIC_3.md`'s `## Out of scope` rather than pointing at a decision that never mentions
  images. (Note: the sibling deferral of OpenRouter images **OAuth** to Epic 7 *is*
  honoured - `docs/epics/epic-7-four-new-oauth-flows/issues/03-openrouter-pkce-oauth.md:124,212`
  covers `ai/images/builtin.go`. Only the `getAuth` reshape is orphaned.)

### P2 - Issues 02 and 06 mandate editing `docs/planning/SPECS.md` inside implementation PRs, bypassing the drift mechanism issue 01 uses correctly
- Location: `.../issues/02-export-catalog-json-input.md:78-81`;
  `.../issues/06-images-adapter-retry.md:135-138`
- Source: `_shared/pipeline-interfaces.md` Drift register - a drift record
  (`docs/epics/epic-<n>-<slug>/drift/<nn>-<slug>.md`) is written by `implement-issue`
  during the run, `docs/planning/DRIFT.md` is written once by `close-epic`, and
  dispositions ("an `accepted` one also means SPECS/CONVENTIONS should be edited") are
  the user's call at close. Issue 01 follows exactly this at
  `.../issues/01-export-catalog-spike.md:75-81`.
- Problem: the same epic contains both patterns. Issue 01 records a finding as a drift
  record for triage; issues 02 and 06 make un-triaged edits to a planning doc a
  merge-blocking acceptance criterion. `SPECS.md` is the planning bundle's source of
  truth and there is no `docs/planning/DRIFT.md` yet to reconcile against.
- Impact: the planning bundle mutates from two PRs with no register entry, so
  `close-epic` has nothing to promote and a later reader cannot tell which SPECS lines
  were decided and which were quietly overwritten mid-epic.
- Recommendation: convert both to drift records under
  `docs/epics/epic-3-catalog-schema-and-export-tooling/drift/`, matching issue 01's
  pattern, and leave the `SPECS.md`/`CONVENTIONS.md` edit to the disposition sweep at
  epic close. If a same-PR edit really is wanted, say so explicitly in `EPIC_3.md` so
  all three issues agree.

### P2 - Issues 04 and 05 apply opposite sizing rules to the same generated-data situation
- Location: `.../issues/04-regenerate-model-catalog.md:10` (`size: L`) with
  `:133-141`; `.../issues/05-regenerate-image-catalog.md:10` (`size: S`) with
  `:130-134`
- Source: `_shared/pipeline-interfaces.md` field notes - "`size`: S ~ under 200 changed
  lines, M ~ 200-500, L ~ 500-1000. L is the ceiling, not the target."
- Problem: issue 04 declares L *because* the generated JSON is huge, then argues at
  `:138-139` that "the `~500` target applies to hand-written lines only - those should
  stay well under 200 here". Issue 05 makes the identical argument at `:132-134` and
  declares S. Verified against the repo: `ai/catalog/data/images/openrouter.json` is 712
  lines with 35 entries today and grows to ~850, so issue 05's real diff is ~800+
  changed lines plus a new test file - S by the hand-written rule, L by the raw rule.
  One of the two is wrong whichever rule is chosen.
- Impact: `size` is the field a supervisor uses to sequence and to decide what to split;
  two issues in the same epic mean different things by it.
- Recommendation: pick one rule and state it once in `EPIC_3.md`'s `## Notes` - e.g.
  "generated catalog output is excluded from the size band" - then re-label issue 04 to
  S/M or issue 05 to L accordingly, and update `issues/index.md:6-7`.

### P2 - Issue 04 has no acceptance criterion keeping the image catalog out of its diff, though one tool run writes both trees
- Location: `.../issues/04-regenerate-model-catalog.md:64-65` (out of scope: "The image
  catalog - issue 05") and its acceptance list `:76-103`, which contains no diff guard
- Source: `.../issues/05-regenerate-image-catalog.md:44-48` - "the tool writes both
  trees in one invocation; committing them as two PRs is a review convenience, not two
  runs"; and the discipline issue 02 already demonstrates at
  `.../issues/02-export-catalog-json-input.md:110` ("`git diff --stat ai/catalog/data`
  is empty in this PR")
- Problem: neither issue 04 nor issue 05 declares an order between them (both are
  `depends_on: [2, 3]`, neither blocks the other), yet both are produced by a single
  tool invocation. Whichever lands first will have an unstaged sibling tree sitting in
  the working copy, and issue 04 has no criterion catching an accidental
  `ai/catalog/data/images/` change.
- Impact: the image catalog can ride into issue 04's PR unreviewed, or be regenerated
  twice from two different network fetches, defeating the "same run" premise.
- Recommendation: add to issue 04's acceptance criteria "`git diff --stat
  ai/catalog/data/images` is empty in this PR", and either add `depends_on: [4]` to
  issue 05 or state in both that the pair share one generator run whose date/SHA is
  recorded identically in both PR bodies.

### P3 - All six PR size notes are identical boilerplate that contradicts the declared `size`
- Location: `.../issues/01-export-catalog-spike.md:153-156` (`size: S`),
  `02:144-147` (M), `03:177-180` (M), `04:133-141` (L), `05:130-134` (S),
  `06:180-185` (M) - every one opens "Target ~500 changed lines; if this grows past
  ~1000, split it before opening the PR."
- Source: `_shared/pipeline-interfaces.md` size bands (S < 200, M 200-500, L 500-1000)
- Problem: an S issue is told to target 500 lines and an M issue is licensed up to 1000.
  The note carries no information about the specific issue except where an author added
  a second sentence (01, 04, 05, 06 do; 02 and 03 do not).
- Impact: the section that is supposed to make sizing actionable instead tells the
  implementer to ignore the band.
- Recommendation: restate each note against its own band - S "split past ~200", M "split
  past ~500", L "already at the ceiling; do not grow" - keeping the per-issue second
  sentence where it exists.

### P3 - The images strict-decode test is owned twice
- Location: `.../issues/03-catalog-validation-0-84-1.md:88-90` and `:119-122`
  (`TestCatalogRejectsUnknownKeys` over "every embedded `data/**/*.json`", extended to
  the image catalog via `catalog.ImagesData`) vs
  `.../issues/05-regenerate-image-catalog.md:90-93`
  (`TestCatalogModelsHasNoUnknownKeys` over the same embedded image bytes)
- Source: `EPIC_3.md:55-56` (AC 3/4 - one validation surface, one catalog test suite)
- Problem: two issues each promise a `DisallowUnknownFields` sweep over
  `ai/catalog/data/images/openrouter.json`. Issue 05 calls it "the same guard issue 03
  adds on the model side", which reads as reuse but is written as a new named test.
- Impact: duplicate coverage, and a likely merge conflict since both land in
  `ai/catalog` / `ai/images` test files.
- Recommendation: leave the raw-bytes strict decode entirely in issue 03 and reword
  issue 05's criterion to "reuses issue 03's helper; adds no second sweep", or drop it
  from issue 03's scope and let issue 05 own the images side alone.

### P3 - Issue 03 inverts the epic's stated order, correctly, but the epic is left contradicting its own issues
- Location: `.../issues/03-catalog-validation-0-84-1.md:41-48` and its
  `depends_on: [1]` with `Blocks: Issue 04`
- Source: `EPIC_3.md:32-35` - "strictly in this order: update `tools/export-catalog` ->
  regenerate all catalog JSON -> update `ai/catalog` validation -> run the catalog
  tests" (and identically `docs/planning/SCOPE.md:175-178`)
- Problem: the issues sequence validation *before* regeneration. Issue 03's reasoning is
  right - the epic's own "code before data" justification (`EPIC_3.md:34-35`) demands it,
  and landing validation after the data means merging a red PR. But `EPIC_3.md:32-35`
  still says "strictly in this order", so the epic and its issues disagree in writing.
- Impact: cosmetic today, but a later reader comparing epic to issues will read the
  issue set as non-conformant to its source.
- Recommendation: amend `EPIC_3.md:32-35` (and, at the next SCOPE touch,
  `SCOPE.md:175-178`) to "update `tools/export-catalog` -> update `ai/catalog`
  validation -> regenerate all catalog JSON -> run the catalog tests", keeping the
  "code before data" sentence that already justifies it.

### P3 - Issue 01 makes rewriting its four successor issues part of its own definition of done
- Location: `.../issues/01-export-catalog-spike.md:82-83` ("Where the finding
  contradicts issues 02-05 as drafted, **edit those issue files** (and their GitHub
  bodies)") and the matching criterion at `:118-119`
- Source: `_shared/pipeline-interfaces.md` Issue status lifecycle - `implement-issue`
  writes status, `timestamp`, the `issues/index.md` bullet and the GitHub mirror for
  *its own* issue; nothing in the contract has one issue rewrite a sibling's body
- Problem: the instruction is well-intentioned (the spike's whole point is that 02-05
  may be planned on false premises, and 02 already concedes "the record wins" at
  `02:45-48`), but it is unbounded: no criterion says the edited issues' `timestamp`s
  are refreshed, that `issues/index.md` titles stay in sync with rewritten GitHub
  titles, or what happens if the finding invalidates an issue entirely.
- Impact: an implementer either skips the step or performs an untracked plan rewrite in
  a PR labelled `documentation`.
- Recommendation: narrow it to "record every correction in the drift record and post it
  on #120; open a follow-up to re-plan issues 02-05 if the finding invalidates them",
  and if in-place edits are kept, add criteria for refreshed `timestamp`s and a
  reconciled `issues/index.md`.

### P3 - Issue 02 invents a configuration surface the epic never asked for
- Location: `.../issues/02-export-catalog-json-input.md:56-58` - "overridable with an
  explicit `MODEL_CATALOG_DIR` env var or a positional argument"
- Source: `EPIC_3.md:32-34` - "update `tools/export-catalog`", nothing about a new
  configuration surface; `docs/planning/SCOPE.md:96` puts "refactoring the testbed, the
  CLI, or `docs/` beyond what the sync forces" out of scope program-wide
- Problem: the default (`${UPSTREAM_CLONE_DIR}/.artifacts/model-catalog`) is already
  derived from the existing `UPSTREAM_CLONE_DIR` seam (`tools/export-catalog/export-catalog.ts:23`,
  verified). The env var *and* the positional argument are two extra ways in that
  nothing in the epic requires, each needing documentation in `docs/PORTING.md`.
- Impact: small, but it is exactly the "it will get built" case - two new interface
  surfaces on a tool the epic only asked to keep working.
- Recommendation: keep the derived default and the actionable error message (both
  clearly justified at `:63-65`); drop the env var and the positional argument, or flag
  them in the issue as an explicit assumption for the user to accept.

### P3 - Issue 03's invariant battery exceeds what the epic's acceptance criteria ask for
- Location: `.../issues/03-catalog-validation-0-84-1.md:71-90` (strict-decode sweep,
  tier-threshold ordering, thinking-level key validity, `Api`-is-known,
  provider-matches-filename, id-uniqueness) and the seven new tests at `:109-129`
- Source: `EPIC_3.md:54-56` - AC 3 asks that "`ai/catalog` validation accepts it at
  load"; AC 4 that "the catalog tests pass, including the image catalog". Neither asks
  for new invariants beyond the regenerated tree loading.
- Problem: the `compatFieldsByApi` expansion at `:52-70` is squarely required (verified:
  `ai/catalog/compat.go:20-38` does list exactly 18 / 3 / 7 fields, as the issue
  claims). The six added invariant sweeps are defensible hardening - the "fails open"
  argument at `:34-39` is real - but they are not epic-authorised work, and they sit on
  the critical path since issue 03 blocks issue 04.
- Impact: an optional hardening battery gates the epic's required regeneration.
- Recommendation: either add a line to `EPIC_3.md`'s `## Scope` authorising the
  strict-decode guard (it directly serves the "silent capability drop" risk the epic's
  Notes are about), or split the six invariant sweeps into a follow-up issue that does
  not block issue 04, leaving issue 03 with the compat-map work its blocker status
  actually needs.

### P3 - The epic's "41 `*.models.ts`" figure is never reconciled with the issues' 39 provider files
- Location: `.../issues/02-export-catalog-json-input.md:25` ("re-exports 39
  `providers/<id>.models.ts` wrappers"); `.../issues/04-regenerate-model-catalog.md:28-30`
  and `:76-79` (35 -> 39, four named additions)
- Source: `EPIC_3.md:36` - "41 `*.models.ts` plus `models.generated.ts` and
  `image-models.generated.ts`" (identically `docs/planning/SCOPE.md:179`)
- Problem: the epic counts 41, the issues consistently count 39, and nothing explains
  the gap (changed-files-in-diff vs `MODELS` keys is the likely reason, but no artifact
  says so). Verified locally: `ai/catalog/data/models/` holds 35 files today, so the
  35 -> 39 arithmetic in issue 04 is right; only the epic's 41 is unaccounted for.
- Impact: traceability only - but it is the one number a reviewer will use to check the
  regeneration is complete.
- Recommendation: add one clause to `EPIC_3.md:36` or to issue 04's Summary explaining
  the 41-vs-39 difference (e.g. "41 files touched in the upstream diff; 39 providers in
  `MODELS` at `936aff00`").

## Coverage notes
- **All six epic acceptance criteria have an owning issue**, verified line by line:
  AC 1 -> issue 01 (`01:102-119`); AC 2 -> issue 02 (`02:102-106`); AC 3 -> issues 03-05
  (`03:109-130`, `04:76-85`, `05:82-95`); AC 4 -> issues 04/05 (`04:82-92`,
  `05:99-102`); AC 5 -> issue 06's six `httptest` criteria (`06:106-128`); AC 6 -> the
  CI triad (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
  v2.12.2) is restated in every one of the six issues, matching `EPIC_3.md:60-61`.
- **The epic's ordering intent is honoured in `depends_on`**: `01:[]` -> `02:[1]`,
  `03:[1]` -> `04:[2,3]`, `05:[2,3]`, `06:[]`. No issue depends on a later number; a
  developer can work 01->06 top to bottom. Issue 06's independence is stated explicitly
  (`06:173-175`) rather than left implied.
- **Cross-epic prerequisites are handled correctly** given the schema: `depends_on`
  carries only intra-epic numbers per `pipeline-interfaces.md`, and issue 03 states the
  epic-2 prerequisite in prose with links to the three specific epic-2 issues
  (`03:163-172`), backed by `EPIC_3.md:63-66`'s declared Epic 2 dependency.
- **Factual accuracy of the issue bodies is unusually high.** Spot-checked against the
  repo: `ai/catalog/compat.go:20-38` really does list 18 / 3 / 7 fields; `ai/images`
  really does declare `MaxRetries`/`MaxRetryDelay` unread (`ai/images/types.go:96-102`)
  and build a bare `&http.Client{}` at `ai/images/openrouter.go:239`;
  `ai/catalog/data/images/openrouter.json` really is 712 lines / 35 entries;
  `ai/catalog/data/models/` really holds 35 files; `ai/providers` really has 35 binding
  files (38 `.go` files less `all.go`, `cloudflare_auth.go`, `refreshmodels.go`);
  `tools/export-catalog/export-catalog.ts` really is 58 lines. Issue 06's "stale
  premise" correction (`06:48-57`) is right - `ai/images/openrouter_test.go` carries
  five offline `httptest` tests plus the live smoke, contradicting
  `docs/planning/SPECS.md:301` and decision 14's "exactly one live smoke test".
- **Boundaries against neighbouring epics are drawn deliberately and mostly correctly**:
  the four new catalog-only providers stay unbound per decision 10 (`04:59-63`), the
  `UPSTREAM.lock` bump stays with Epic 9 (`01:93-95`, `02:96`, `04:71-72`), the
  `provider-retry.ts` reconciliation and the clamp-vs-throw difference stay with Epic 9
  (`06:83-92`), and the OpenRouter images OAuth deferral to Epic 7 is genuinely honoured
  there.
- **Bundle and schema conformance is clean**: all six files carry `type: Issue`,
  `tags: [epic-3]`, `epic: 3`, zero-padded `issue`, `slug`, `size`, `status: open`,
  `gh_issue`, `resource`, `depends_on`, and a refreshed `timestamp`; all seven body
  headings are present in every file; `issues/index.md` carries no frontmatter (correct
  for a non-root index), and its six bullets are mechanical and match the sibling epics'
  format exactly.
- **Repo conventions are reflected in the bodies**: discrete named test functions,
  stdlib-only, no `t.Parallel()`, `got`/`want` phrasing (`03:140-141`, `05:103`),
  `gofmt -l .` (`03:134`, `06:146`), the `// Ports:` provenance rule applied in both
  directions (`03:134-139` for a ported file, `06:139-142` for deliberately original
  code - matching `docs/planning/SPECS.md:335-338`), the local `GOTMPDIR=$PWD/.gotmp`
  workaround, and a Conventional Commit example per issue.

## Open questions
- Does the `getAuth(providerId | model, overrides)` overload in upstream's
  `images-models.ts` have any Go-visible consequence for `ai/images/provider.go`, or is
  it a TypeScript-only ergonomics change? The answer decides whether the P2 above needs
  an issue or just a line in `EPIC_3.md`'s `## Out of scope`.
- Issue 02's entire design rests on seven unverified claims about upstream at
  `936aff00` (`01:30-62`). Issue 01 is explicitly the verification step and issue 02
  concedes "the record wins" (`02:45-48`), so this is handled by design - but if the
  spike disproves claims 1-3, issue 02's scope, size and acceptance criteria are void
  and the re-planning path is only sketched (see the P3 on `01:82-83`).
