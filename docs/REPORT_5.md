# Issue Review Report 5

## Scope

- Reviewed: `docs/epics/epic-5-remaining-adapters/issues/01-copilot-dynamic-headers.md` through `11-cloudflare-stream-classification.md` (11 issue files) plus `docs/epics/epic-5-remaining-adapters/issues/index.md`.
- Reviewed against: `docs/epics/epic-5-remaining-adapters/EPIC_5.md` (the contract), with `docs/planning/SCOPE.md#milestone-5-remaining-adapters` (the epic's `source`), `docs/planning/SPECS.md`, `docs/planning/CONVENTIONS.md`, `docs/epics/index.md`, `docs/PORTING.md` and the repo at `develop` read for context. `docs/planning/DRIFT.md` does not exist yet; no `CLAUDE.md`, `AGENTS.md`, `CONTRIBUTING.md`, `.github/ISSUE_TEMPLATE/` or `.github/PULL_REQUEST_TEMPLATE.md` exist in this repo (only `.github/workflows/`), consistent with CONVENTIONS.md:228-229.
- GitHub verification: **verified**. Milestone 22 ("Epic 5: Remaining adapters") and tracking issue #122 exist and are OPEN; issues #159–#169 all exist, are OPEN, sit on milestone 22, carry `enhancement,upstream-sync`, and all eleven are native sub-issues of #122. Every title matches the local frontmatter and the `issues/index.md` bullet exactly.
- External review: `openai/gpt-5.6-sol` via `opencode --agent plan`, batched by area. The coverage pass and the ordering/sizing pass returned usable output (the ordering pass after one retry — the first run aborted mid-read). The schema/GitHub-metadata pass returned nothing within the timeout; that area was reviewed natively instead and is recorded as a verification gap on the external pass only, not on the review. All external output was treated as leads and re-verified against the files before entering this report.

## Findings

### P1 — Seven of eleven issues are blocked by an Epic 4 issue the epic says they do not need

- Location: `docs/epics/epic-5-remaining-adapters/issues/03-anthropic-strict-tools-and-signed-thinking.md:157-163`, `05-google-shared-converters.md:159-163`, `07-mistral-stop-reasons-and-strict-tools.md:155-161`, `09-bedrock-stop-reasons-strict-tools-and-claude-5.md:170-178`; transitively `04-anthropic-deferred-tools.md:189-190`, `06-google-and-vertex-stream-and-params.md:156-157`, `08-mistral-wire-and-header-parity.md:193-194`.
- Source: `docs/epics/epic-5-remaining-adapters/EPIC_5.md:69-70` — "Independent of [Epic 4](/epic-4-openai-family-adapters/EPIC_4.md); the two can run concurrently once epic 2 has merged"; `docs/epics/index.md:12-16` records no edge either.
- Problem: four issues declare Epic 4 issue 01 (#147, which creates `ai/apis/internal/grammar` and `ResolveJSONSchemaStrictSampling`) as a hard blocker, and three more inherit it through the intra-epic edges 03→04, 05→06, 07→08. Issue 03:160-163 states the contradiction outright: "The epic file says epic 5 is 'independent of Epic 4'; that is true of the wire work and false of this one function." The epic's `## Dependencies` section still says otherwise, and `depends_on` cannot carry the edge (pipeline-interfaces.md scopes it to "other issue numbers in this epic"), so nothing mechanical catches it.
- Impact: anyone scheduling from the epic — the stated reason these adapters were grouped is that they "parallelise at issue level" (`EPIC_5.md:19-22`) — starts epic 5 alongside epic 4 and stalls on 7 of 11 issues. Only two issues (01, 11) and the two purely-behavioural ones (02, 10) can actually start.
- Recommendation: reconcile in one place. Either add Epic 4 issue 01 to `EPIC_5.md:65-70` (and to the `docs/epics/index.md` line for epic 5), or move `ai/apis/internal/grammar` into Epic 2, where every consumer of it already depends. The issue bodies are correct as written; the epic is the side that is wrong, and leaving the two disagreeing is what makes the schedule unreliable.

### P2 — Epic acceptance criterion 3 can be closed without a Go home

- Location: `issues/11-cloudflare-stream-classification.md:72-73` ("**Record it as a deviation.**") and its acceptance criteria at `:99-101`, `:102-117` (the three implementation tests are all prefixed "If ported").
- Source: `EPIC_5.md:57` — "3. `cloudflare-stream.ts` has a recorded classification **and a Go home**."
- Problem: the issue's only unconditional criterion is a `docs/PORTING.md` row. On the deviation branch nothing lands in Go, so #169 can be closed green while AC 3 is unmet. The issue recommends porting (`:51-57`) but never requires it.
- Impact: an epic acceptance criterion becomes unverifiable at close, which is exactly the failure the criterion list exists to prevent. It also has a downstream cost: Epic 6 issue 03 (#172, `docs/epics/epic-6-auth-core-and-env-api-key-bindings/issues/03-provider-scoped-apikey-resolution.md:36-50`) states that if #169 "dispositioned `cloudflare-stream.ts` as a deviation", #172 must carry the dispatch-time substitution itself or Cloudflare requests ship with literal `{CLOUDFLARE_ACCOUNT_ID}` in the URL — "a regression, not a deviation".
- Recommendation: make the port the required outcome in issue 11's `## Scope` and drop the "If ported —" prefixes, or take the deviation branch back to the epic and amend AC 3. Do not leave the choice inside the issue.

### P2 — `## PR size note` is identical boilerplate that contradicts the declared `size`

- Location: `01:154-157`, `02:183-186`, `03:169-172`, `05:166-169`, `06:165-168`, `07:165-168`, `08:197-200`, `09:181-184`, `10:190-193`, `11:147-150` — all ten read verbatim "Target ~500 changed lines; if this grows past ~1000, split it before opening the PR." (`04:197-207` is the only one that reasons.)
- Source: `_shared/pipeline-interfaces.md` — "`size`: S ≈ under 200 changed lines, M ≈ 200–500, L ≈ 500–1000. **L is the ceiling, not the target.**"; `review-issues/SKILL.md` Step 2 — "PR size concrete enough to act on".
- Problem: for the two `size: S` issues (`01:10`, `11:10`) the note names a target 2.5× the band's ceiling; for the eight `size: M` issues it names the top of M as the target and the top of L as the split trigger. The section carries no per-issue information at all.
- Impact: the field that is supposed to keep PRs reviewable instead licenses every M issue to drift into L, and each S issue contradicts its own frontmatter inside the same file. Sizing is the first thing a developer feels.
- Recommendation: rewrite each note against its own band — S: "under ~200 changed lines; split past 200", M: "~200–500; split past 500", L: keep issue 04's shape, which states *why* it is whole and names the honest cut if it overruns.

### P2 — Issue 01's `size: S` does not match its own scope

- Location: `issues/01-copilot-dynamic-headers.md:10` (`size: S`), against its scope at `:56-91` and acceptance criteria at `:105-130`.
- Source: `_shared/pipeline-interfaces.md` size bands (S ≈ under 200 changed lines).
- Problem: the issue creates a new package with three exported functions, wires three separate adapter packages, and requires three unit tests, `httptest` assertions in three packages, a caller-header-precedence test, plus edits to `docs/PORTING.md` and `docs/planning/SPECS.md`. That is 300–500 changed lines, not under 200 — and the issue's own PR size note (`:154-157`) already says "~500", contradicting the frontmatter inside one file.
- Impact: an S label sets a review expectation the PR breaks, and `issues/index.md:3` propagates the wrong size to the index.
- Recommendation: re-label `size: M` in `01:10` and in the `index.md:3` bullet.

### P2 — Epic acceptance criterion 5's `docs/PORTING.md` half is dropped by seven issues

- Location: `02:109`, `03:96-97`, `04:110`, `05:100`, `07:94`, `09:94`, `10:107` — each closes its `## Scope` with only "`// Ports:` headers stay accurate", and none of their acceptance-criteria checklists mentions `docs/PORTING.md`.
- Source: `EPIC_5.md:60-61` — "5. Every file touched carries a `// Ports:` header **and is dispositioned in `docs/PORTING.md`**."
- Problem: only issues 01 (`:87-91`, `:124-125`), 06 (`:128-130`), 08 (`:107-109`) and 11 (`:74-78`, `:99-101`) carry a PORTING.md deliverable, and each of those does so for a *deviation* it introduces. The disposition sweep the criterion asks for — confirm the row for each touched upstream file still describes what the Go code does — has no owner in the other seven.
- Impact: half an acceptance criterion silently becomes Epic 9's problem (`EPIC_5.md:44-45` explicitly defers only the *classifier* audit there, not this). It is the criterion class the review skill flags as "impossible to verify afterwards".
- Recommendation: add one acceptance-criterion line to each of the seven — e.g. "`docs/PORTING.md`'s row for `src/api/anthropic-messages.ts` still describes the Go code after this change" — even where the expected outcome is "unchanged".

### P2 — Five acceptance criteria pass whichever way the implementer decides

- Location: `06:128-130` ("`TestGoogleHonorsInjectedFetch` (or `TestGoogleRejectsCustomFetch`, matching whichever way the decision above went)"), `08:145-149` (60s timeout adopted **or** a PORTING deviation), `09:88-91` ("leave a comment saying so, or match upstream's two-lookup shape"), `02:85-88` ("decide, implement one way, and assert it in a test"), `11:72-73`/`:99-117` (port **or** deviate).
- Source: `EPIC_5.md:88-89` — "The governing principle: a deviation must be justified by structural non-portability, never by cost"; `EPIC_5.md:52-53` AC 1 — adapters "match upstream at `936aff00`".
- Problem: each of these criteria is satisfied by either outcome, so the checkbox proves only that something was written down. The issues do demand a written reason (06:70-72 is explicit and even recommends honoring `Fetch`), but nothing in the criterion tests that the reason is *structural* rather than convenient.
- Impact: five wire-visible behaviors are settled by whoever picks the issue up. The custom-fetch one is the sharpest: epic 4 issue 02 honors `Fetch` for the OpenAI family, so a "port the rejection" outcome in issue 06 leaves the library with two different `Fetch` contracts across adapters.
- Recommendation: decide these four or five in the epic (a `## Notes` line each is enough) and collapse each acceptance criterion to the single expected behavior. Issue 06's already has the right answer written next to the question.

### P3 — Two issues claim nobody owns work that Epic 6 owns

- Location: `02:116-121` — "`providers/anthropic.ts` (+49: `ANTHROPIC_AUTH_TOKEN` as a bearer header) … **No epic in this program currently owns it**"; `11:86-90` — "No epic in this program currently claims `src/providers/*.ts` updates beyond [Epic 6]'s four new bindings".
- Source: `docs/epics/epic-6-auth-core-and-env-api-key-bindings/issues/07-anthropic-auth-token.md:1-17` (#176) is exactly the `ANTHROPIC_AUTH_TOKEN` bearer-header work; `.../issues/03-provider-scoped-apikey-resolution.md:55`, `:80`, `:148-155` (#172) is exactly the `cloudflare-auth.ts` per-field credential/env merge.
- Problem: epic 5's issues were written before epic 6's (`9e751c3` vs `6c6a391`), so both claims were true when written and are false now. Relatedly, `11:118-119` requires that "auth-time resolution is not removed", which #172 explicitly removes (`03-provider-scoped-apikey-resolution.md:36-50`); epic 6 records the conflict from its side, epic 5 does not.
- Impact: the implementer raises PR-body flags for Epic 9's sweep about work that is already an open, numbered issue, and may re-do it. The 11↔#172 contract conflict is the one with teeth.
- Recommendation: replace `02:116-121` with a pointer to #176, `11:86-90` with a pointer to #172, and add to issue 11 the same coordination note #172 carries.

### P3 — Issue 01 asks for an edit to a finalized planning doc the epic did not ask for

- Location: `01:89-90` — "remove the matching 'known gap' line from `docs/planning/SPECS.md` if one is still there" (the line is `docs/planning/SPECS.md:340`).
- Source: `EPIC_5.md:54-56` AC 2 names only `docs/PORTING.md:46`.
- Problem: a small piece of invented scope, and it puts a `docs/planning/` edit inside an implementation PR. `_shared/bundle-interfaces.md` keeps `docs/planning/` as a separate bundle maintained by the planning skills, which epics "reference, never copy".
- Impact: minor, but it is the kind of edit that races with whatever else is rewriting SPECS.md, and it is not what the epic promised.
- Recommendation: keep the `docs/PORTING.md:46` row change (that is AC 2) and either drop the SPECS.md edit or add it to the epic explicitly so it is tracked.

### P3 — Two out-of-scope boundaries are crossed deliberately but never amended in the epic

- Location: `01:85-86` and `01:120-121` require changes and tests in `ai/apis/openaicompletions` and `ai/apis/openairesponses`; `08:80-83` requires that "the classifier patterns must be checked in the same PR" if the Mistral error text changes.
- Source: `EPIC_5.md:43` — "The OpenAI-family adapters — epic 4 owns those"; `EPIC_5.md:44-45` — "The classifier audit of the error strings these adapters emit; epic 9 audits after the adapters have moved".
- Problem: both crossings are justified — `01:52-54` records that Epic 4 issue 09 declines the Copilot headers and points here, `01:97-100` confines the touch to the header builder, and 08's classifier check is the minimum needed not to break `ai/retry.go` — but the epic's Out of scope still reads as an absolute.
- Impact: a reader auditing the epic against its issues sees two violations and has to re-derive why they are fine. That is the cost the Out of scope section exists to avoid.
- Recommendation: qualify `EPIC_5.md:43` with "except the Copilot header builders, which issue 01 owns end to end" and `EPIC_5.md:44-45` with "except where this epic's own change to an error string forces a classifier check in the same PR".

### P3 — Three stale or off-by-one code citations

- Location: `01:134` cites `ai/apis/anthropic/anthropic.go:1193` for `buildHeaders` (it is at `:1192`); `10:165-168` cites `ai/apis/bedrock/clientauth.go:123`, `:132`, `:147` for `hasAmbientConfiguredProfile`, the bearer token and the credentials assignment (they are at `:124`, `:133`, `:148`); `01:138-139` names `ai/apis/internal/grammar` as one of "the two existing precedents for a shared, consumer-invisible adapter helper package".
- Source: the repo at `develop` — `ai/apis/internal/` contains only `httpretry`; `grammar` is created by Epic 4 issue 01 (`04-openai-family-adapters/issues/01-constrained-sampling-core.md`).
- Problem: the line offsets are trivial, but "existing precedent" sends an implementer looking for a package that does not exist yet, and it understates the Epic 4 dependency this report's P1 finding is about.
- Impact: minutes, not correctness. Worth noting because every other citation in these eleven files checked out — file line counts (`anthropic.go` 1234, `messages.go` 411, `mistral.go` 368, `clientauth.go` 211, `thinking.go` 179, `cloudflare_auth.go` 127) and symbol offsets (`anthropic.go:207` `run`, `:361` `decodeEvents`, `:625` `rawContentBlockStart`, `:679` `mapStopReason`, `google.go:71/:249/:341`, `mistral/messages.go:97`, `bedrock/clientauth.go:95/:117/:172`) are all exact.
- Recommendation: ±1 corrections, and reword `01:138-139` as "the package Epic 4 issue 01 creates".

### P3 — Branch and status-commit conventions are absent from the bodies

- Location: every issue closes its acceptance criteria with a Conventional Commit example (`01:129-130`, `02:156-157`, …) and nothing else about how the work lands.
- Source: `docs/planning/CONVENTIONS.md:216-224` — feature branches named `issue-<NN>-<slug>`, merged by PR into `develop` with merge commits (no rebase, no squash), and each status transition getting its own `docs(epics): …` commit.
- Problem: the commit-message half of the convention is reflected everywhere; the branch, target-branch and status-commit half is nowhere.
- Impact: none when `implement-issue` drives the work, since it supplies these. It bites only if an issue is worked by hand.
- Recommendation: one line in the epic's `## Notes` covering all eleven, rather than eleven duplicated lines.

## Coverage notes

- **Epic coverage is complete.** All six `## Scope` bullets and all six numbered acceptance criteria have named owners: anthropic (`EPIC_5.md:28`) → issues 02/03/04; google + shared + vertex (`:29-30`) → 05/06; mistral (`:31`) → 07/08; bedrock (`:32`) → 09/10; the Copilot gap (`:33-36`) → 01; `cloudflare-stream.ts` (`:37-39`) → 11. The splits track the upstream diff sizes the epic quotes (+252 / +82+38+38 / +423 / +146) and each half is independently observable.
- **The two checks the epic explicitly demanded were performed, not deferred.** `01:28-32` reports `git diff 244f1dea..936aff00 -- packages/ai/src/api/github-copilot-headers.ts` empty, answering `EPIC_5.md:36`; `11:25-34` corrects the epic's own guess — the file is `src/providers/cloudflare-stream.ts`, not `src/api/`, and it is one half of a move out of `cloudflare-auth.ts`.
- **The "no new direct dependencies" rule (`EPIC_5.md:82-85`) is honored actively.** Four issues explain why an upstream SDK-wrapper port is a no-op in Go rather than adding a dependency: `02:56-62` (`retryProviderRequest` vs `httpretry`), `05:63-71` (`retryGoogleRequest`), `07:101-105` and `08:117-127` (Mistral's hand-rolled transport), `09:100-106` (an `aws-sdk-go-v2` bump would be a DRIFT.md conversation, not a silent `go get`).
- **Error-text-verbatim is called out wherever error text changes** — `02:33-37`, `06:37-40`, `07:49-51`, `09:29-33`, `10:64-67` — each citing CONVENTIONS.md and the `ai/retry.go` / `ai/overflow.go` coupling, with the exact strings quoted in the acceptance criteria so they can be asserted byte for byte.
- **Test conventions match CONVENTIONS.md:140-189 throughout**: discrete named `TestSpecificBehavior` functions rather than tables, `httptest` and offline everywhere, stdlib only, one focused `_test.go` per concern, and the table-driven exception used exactly once (`04:145-151`) for a pure input→output classifier — which is precisely the case CONVENTIONS.md reserves it for. Every issue repeats the local `GOTMPDIR=$PWD/.gotmp go test ./...` form and the three CI gates with the pinned `golangci-lint v2.12.2`.
- **Intra-epic ordering is sound.** The only edges are 04→03, 06→05, 08→07; each is declared in both `depends_on` and the prose `## Dependencies`, no issue depends on a higher-numbered sibling, and the shared-file rebase hazards that are *not* logical dependencies are named as such (`02:180-181`, `10:113-114`, `01:150-152`).
- **Schema and index conform.** All eleven files carry exactly the thirteen permitted fields (`type`, `title`, `description`, `tags: [epic-5]`, `timestamp`, `epic`, `issue`, `slug`, `size`, `status`, `gh_issue`, `resource`, `depends_on`) with no extensions; `resource` is present as required for `status: open`; all seven required body sections appear in order in every file; `issues/index.md` carries no frontmatter, lists all eleven in order, and its sizes, statuses and issue numbers agree with the frontmatter and with GitHub. Cross-bundle links use plain relative paths (`../../../planning/…`) and in-bundle links use the leading-slash form (`/epic-4-openai-family-adapters/…`), as `bundle-interfaces.md` requires. Everything is in English.
- **GitHub linkage is clean** — see Scope. Sub-issue linkage on #122 is complete for all eleven, so the tracking issue's progress bar is honest.

## Open questions

- Issues 09 and 10 are both `size: M` but each changes four to five production files and ports 12+ upstream test cases; the external pass flagged both as plausibly L. If either overruns, the natural cut in 09 is (stop reasons) vs (Claude 5 matrix + strict tools) and in 10 is (credentials) vs (diagnostic). Worth pre-deciding rather than discovering at review time.
- `08:74-83` asks whether the Mistral error text should move to upstream's now that upstream is raw HTTP too. That is the one decision in this epic whose blast radius leaves the adapter — `ai/retry.go` and `ai/overflow.go` match against the current text. It may deserve its own decision rather than living inside an issue.
- Would moving `ai/apis/internal/grammar` from Epic 4 issue 01 into Epic 2 be cheaper than carrying the cross-epic edge in the P1 finding? Four epic-5 issues and most of epic 4 need it; Epic 2 is already the "core contracts" epic that both depend on.
