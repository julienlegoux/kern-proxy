# Issue Review Report 8

## Scope
- Reviewed: `docs/epics/epic-8-pi-messages-and-radius/issues/01-pimessages-wire-and-converter.md`,
  `02-pimessages-stream-entry.md`, `03-radius-gateway-config.md`,
  `04-radius-provider-binding.md`, `05-porting-and-docs.md`, and
  `docs/epics/epic-8-pi-messages-and-radius/issues/index.md`
- Reviewed against: `docs/epics/epic-8-pi-messages-and-radius/EPIC_8.md` (the contract),
  with `docs/planning/SCOPE.md#milestone-8-pi-messages-and-radius` (the source plan),
  `docs/planning/SPECS.md`, `docs/planning/CONVENTIONS.md`, `docs/epics/index.md`,
  `docs/epics/epic-7-four-new-oauth-flows/EPIC_7.md` and its issues 04–06, and
  `docs/epics/epic-2-core-types-and-models-contracts/issues/05` and `08`.
  Schema judged against `pipeline-interfaces.md` / `bundle-interfaces.md`.
  No `docs/planning/DRIFT.md` exists yet; no `CLAUDE.md`, `AGENTS.md`,
  `CONTRIBUTING.md` or `.github/ISSUE_TEMPLATE/` present in the repo.
- GitHub verification: verified (`gh`, `kern-ia/kern-link`) — issues #187–#191 all
  exist and are OPEN, titles byte-identical to the local `title:` fields, all five
  on milestone 25 "Epic 8: pi-messages and radius", and all five returned by
  `gh api repos/kern-ia/kern-link/issues/125/sub_issues` as native sub-issues of the
  epic tracking issue #125. Milestone 25 is open with 6 open / 0 closed (the five
  issues plus the tracking issue). Labels: `enhancement` on #187–#190,
  `documentation` on #191; `epic` correctly appears only on #125.
- External review: `openai/gpt-5.6-terra` via `opencode run --agent plan` (second
  attempt; the first pass exited without producing an answer). Its output was treated
  as leads and verified against the files — four leads were rejected as unfounded
  (see Coverage notes), the rest are reflected below in corrected form.

## Findings

### P1 — Ported test files are exempted from the `// Ports:` header the epic requires on *every* ported file
- Location: `docs/epics/epic-8-pi-messages-and-radius/issues/01-pimessages-wire-and-converter.md:173-174`
  ("`// Ports: …` after the `package` clause on every new **non-test** file"),
  `02-pimessages-stream-entry.md:196` ("on both new files" — `errors.go` and
  `stream.go` only, excluding `stream_test.go`), `03-radius-gateway-config.md:151`,
  `04-radius-provider-binding.md:159-160`
- Source: `EPIC_8.md:51-52` acceptance criterion 4 — "`// Ports:` headers on every
  ported file"; `docs/planning/CONVENTIONS.md:50-52` — "Every ported file carries a
  provenance header"; repo practice: 33 `_test.go` files already carry one, e.g.
  `ai/apis/anthropic/anthropic_test.go:3` (`// Ports: packages/ai/test/anthropic-sse-parsing.test.ts`)
  and `ai/catalog/compat_test.go:3`.
- Problem: issue 01 narrows the epic's "every ported file" to "non-test file", and
  issues 02–04 name only the non-test files. The single most explicitly-ported
  artifact in this epic is a test file — `02:119-121` says `stream_test.go` is "the
  port of `test/pi-messages.test.ts` (248 lines)" — and it would ship with no
  provenance header. `ai/providers/radius_config_test.go` (`03:95`) is original code
  and correctly needs none, but `stream_test.go` and issue 01's converter tests are
  ports.
- Impact: acceptance criterion 4 is contradicted rather than covered, and criterion 4
  is the one epic 9's disposition sweep
  (`docs/epics/epic-9-classifier-audit-and-release/issues/04-disposition-sweep.md`)
  consumes. `CONVENTIONS.md:63-66` makes header *absence* meaningful — it marks
  original code — so an unheadered `stream_test.go` actively misreports provenance.
- Recommendation: in issue 01 replace "every new **non-test** file" with "every new
  file that ports upstream code, tests included"; in issue 02 name
  `stream_test.go` alongside `errors.go`/`stream.go` with
  `// Ports: packages/ai/test/pi-messages.test.ts`; leave issue 03's original test
  file explicitly headerless with a one-line note saying why.

### P2 — `size:` frontmatter is contradicted by an identical boilerplate PR-size note in all five issues
- Location: `01:10` (`size: M`) vs `01:211-214`; `02:10` (`size: M`) vs `02:237-241`;
  `03:10` (`size: M`) vs `03:189-192`; `04:10` (`size: M`) vs `04:203-206`;
  `05:11` (`size: S`) vs `05:147-150` — all five carry the same sentence, "Target ~500
  changed lines; if this grows past ~1000, split it before opening the PR."
- Source: `pipeline-interfaces.md`, "Issue file" field notes — "S ≈ under 200 changed
  lines, M ≈ 200–500, L ≈ 500–1000. L is the ceiling, not the target";
  `EPIC_8.md:26` and `SCOPE.md:264-270` size the work at 433 + 248 upstream lines for
  pi-messages and 82 + 96 for radius.
- Problem: "~500 target / ~1000 ceiling" describes an **L**. On issue 05 it
  contradicts `size: S` by a factor of more than two. On issues 01 and 02 it sits at
  the very top of M before any Go expansion: 433 TS lines split over two issues plus a
  248-line test port plus 12 (01) and 13 (02) named Go test functions will not land
  inside 500 changed lines.
- Impact: `size` is what a supervisor and an implementer plan from. Two issues that
  are silently L, and a docs issue advertised as S but licensed to ~500 lines, make
  the epic's remaining-work estimate wrong and invite exactly the oversized PR the
  sizing rule exists to prevent.
- Recommendation: write a real per-issue note. Relabel 01 and 02 `size: L` (or act on
  02's own suggested seam at `02:240-241` — `errors.go` + diagnostics tests first, the
  request/read loop second — and keep both halves M); keep 03 and 04 at M with a
  ~300-line target; give 05 a target under 200 lines to match `size: S`.

### P2 — Issue 05's count-sweep acceptance criterion greps frozen planning artifacts and can never pass
- Location: `05-porting-and-docs.md:110-113` — "`grep -rn "nine wire adapters\|35
  providers" docs/ README.md ai/doc.go` returns nothing stale"
- Source: `EPIC_8.md:44-45` (AC 1, the tenth adapter) and `05:42-79`, whose Scope names
  only `docs/PORTING.md`, `docs/auth.md`, `docs/architecture.md`, `ai/doc.go`,
  `README.md` and `docs/planning/SPECS.md`.
- Problem: run today, that grep also hits `docs/planning/scope/06-adapter-updates.md:18`
  and `:43`, `docs/planning/scope/10-new-provider-bindings.md:30` — frozen scoping
  decision records that describe the repo *as it was when the plan was written* and
  must not be rewritten — plus the issue file's own `05:4` and `05:111`. The criterion
  is therefore unsatisfiable as written, and read literally it points the implementer
  at artifacts they must leave alone.
- Impact: a binary AC that cannot go green either blocks the PR or trains the
  implementer to ignore its own acceptance criteria; worse, an obedient implementer
  edits the decision ledger.
- Recommendation: scope the grep to the files the issue actually owns, e.g.
  `grep -rn "nine wire adapters\|35 providers" README.md ai/doc.go docs/architecture.md
  docs/auth.md docs/usage.md docs/planning/SPECS.md docs/planning/index.md`, and add
  one line saying `docs/planning/scope/` and `docs/epics/` are historical and exempt.

### P2 — Issue 05's Scope misses two files that carry the same stale counts
- Location: `05-porting-and-docs.md:72-78` (Scope: `docs/architecture.md`, `ai/doc.go`,
  `README.md`, `docs/planning/SPECS.md`) and `05:36-40`
- Source: `bundle-interfaces.md`, reserved files — "Index bullets are **mechanical**
  … tracking the target file's frontmatter"; the live text is
  `docs/planning/index.md:13`, which copies `docs/planning/SPECS.md:4`'s description
  verbatim ("across 35 providers … nine wire adapters over net/http"), and
  `docs/usage.md:213` ("catalog (~35 providers)").
- Problem: issue 05 updates `SPECS.md`'s description-bearing lines but never mentions
  `docs/planning/index.md`, whose bullet is a copy of that description, nor
  `docs/usage.md`. Its own Out of scope (`05:86-88`) excludes only *Radius examples*
  in `docs/usage.md`, not the provider count.
- Impact: the planning bundle's index would disagree with the doc it indexes — the
  exact drift `bundle-interfaces.md` forbids — and two reader-facing files keep
  claiming 35 providers and nine adapters after the epic ships ten and 36+.
- Recommendation: add `docs/planning/index.md:13` and `docs/usage.md:213` to issue
  05's Scope, and to the AC at `05:110-113` once its grep is narrowed.

### P3 — Every `docs/PORTING.md` line citation in issue 05 is wrong
- Location: `05-porting-and-docs.md:124-127` — "`docs/PORTING.md:20-60` — the
  file-mapping table, including the `src/api/*.lazy.ts` row (`:39`), the
  `src/auth/helpers.ts` row (`:48`) … and the `src/providers/*.ts` row (`:59`)"
- Source: `docs/PORTING.md` — the table runs `:21-64`; `src/api/*.lazy.ts` is at
  `:38`, `src/auth/helpers.ts` at `:42`, `src/providers/*.ts` (with its "~35 bindings"
  count) at `:57`. `:48` is the `anthropic-messages` row and `:59` is `env-api-keys.ts`.
- Problem: three of three row citations point at the wrong rows, and the stated table
  extent is short by four lines.
- Impact: cosmetic — the rows are named as well as numbered, so the implementer finds
  them anyway — but it is the only place in this epic where the file:line discipline
  the rest of the bodies keep breaks down.
- Recommendation: re-derive the four citations (`21-64`, `:38`, `:42`, `:57`).

### P3 — "fetched at provider setup" in the epic becomes "fetched inside a gated refresh" in the issues, unacknowledged
- Location: `03-radius-gateway-config.md:21-26` (the fetch lives in
  `loadRadiusGatewayConfig`, called from the refresh) and
  `04-radius-provider-binding.md:36-53` (phase 3: "`context.allowNetwork` false … The
  network call is gated, not unconditional") with `04:130-131`
  (`TestRadiusListsNoModelsBeforeARefresh` — construction touches no network)
- Source: `EPIC_8.md:31-33` — "Radius fetches its gateway config **at provider
  setup**, a network dependency none of the existing 35 bindings has"; identically at
  `SCOPE.md:268-270` and in the risk register at `SCOPE.md:317-319`.
- Problem: the issues place the fetch in the `RefreshModels` pipeline behind
  `AllowNetwork`, so nothing happens at `RadiusProvider(nil)` construction. This is
  almost certainly the *right* reading of upstream and of epic 2's refresh contract,
  but no issue says so, and epic 8's acceptance criterion 3 (`EPIC_8.md:50-52`) is
  worded around "the network call at setup".
- Impact: a reviewer checking AC 3 against `04`'s tests can reasonably conclude the
  epic's central risk was not met, when it was met somewhere else.
- Recommendation: add one line to issue 04's Summary stating that "provider setup" in
  the epic means the refresh pipeline, and that construction is deliberately
  network-free — with the pointer to `EPIC_8.md:31-33`.

### P3 — Acceptance criterion 2 ("248 test lines ported") is claimed by issue 02 but half-executed in issue 01
- Location: `02-pimessages-stream-entry.md:26-27` — "Epic 8's acceptance criterion 2 —
  upstream's 248 test lines ported and passing offline — is settled here" — against
  `01-pimessages-wire-and-converter.md:129-134`, which ports "the upstream test's
  event sequence", and `01:200-201`, which cites `test/pi-messages.test.ts (248)` as
  issue 01's own upstream source.
- Source: `EPIC_8.md:47` acceptance criterion 2.
- Problem: one AC is executed across two PRs but declared closed by one of them.
  Neither issue says which cases of the 248 lines belong to which PR.
- Impact: nobody reviewing PR 02 can verify the claim, and a case that fell between
  the two PRs would be invisible.
- Recommendation: in issue 02 change "is settled here" to "is completed here — issue
  01 ports the converter cases, this PR ports the transport and error cases", and add
  the matching half-sentence to issue 01.

### P3 — `events.go` departs from CONVENTIONS' named adapter file layout without the note issue 03 gives its own departure
- Location: `01-pimessages-wire-and-converter.md:96-97` — "`ai/apis/pimessages/events.go`
  (new) — the converter"
- Source: `docs/planning/CONVENTIONS.md:37-40` — "adapters put wire structs in
  `wire.go`, streaming in `stream.go`, message translation in `messages.go`, errors in
  `errors.go`". No adapter package in the repo has an `events.go`
  (`find ai -name "events*.go"` returns only the core `ai/events.go`).
- Problem: the converter is translation, which the convention assigns to
  `messages.go`. The file name may well be the better one here — the unit converts
  events, not messages — but issue 01 does not flag it, whereas issue 03 flags its
  own (smaller) layout departure explicitly at `03:63-68`.
- Impact: minor; a reviewer citing CONVENTIONS at PR time has nothing in the issue to
  point at.
- Recommendation: either use `messages.go`, or add one sentence to issue 01's Scope
  noting the departure and its reason, in the shape issue 03 already uses.

### P3 — Issue 04 leaves a stop-work branch for a contract question its dependency already answers
- Location: `04-radius-provider-binding.md:56-64` — "**One thing to verify before
  writing code**: that issue describes `FetchModels(ctx, rc) ([]*Model, error)` … If
  the landed contract exposes `rc.Publish`, use it … If it does not … stop, say so in
  the PR body, and record it as a drift record"
- Source: `docs/epics/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md:30-35`
  — `RefreshModelsContext` carries `publish(publication) => Promise<boolean>` — and
  `:70-72` and `:87`, which add `RefreshModelsContext` to `ai` and pass it into
  `FetchModels(ctx, rc)`.
- Problem: the question is answerable today from the artifact issue 04 depends on:
  `rc` carries `Publish`, so phase 1 publishes through `rc.Publish` and the fetched
  list is the return value. Issue 04 neither cites those lines nor resolves the
  branch.
- Impact: the highest-value issue in the epic opens with a conditional stop, and
  four of its acceptance criteria (`04:132-146`) presuppose the answer.
- Recommendation: replace the hedge with a citation to epic 2 issue 08:30-35 and :87,
  keeping only a one-line "if the landed signature differs, report it" fallback.

## Coverage notes

- **All five epic acceptance criteria are mapped.** AC 1 (`EPIC_8.md:44-46`) → issue
  01's `ApiPiMessages` const in `ai/types.go:10-19` plus issue 04's `StreamFuncs`
  wiring — and `ApiGoogleVertex` appears in exactly one non-test file, so there is no
  second registration site the issues could have missed. AC 2 → issues 01 + 02 (see
  the P3 above). AC 3 (`:50-52`) → issue 03's `httptest` fetch tests including the
  503, malformed-payload and cancellation failure paths (`03:118-138`) and issue 04's
  `TestRadiusRefreshKeepsLastKnownModelsWhenTheGatewayFails` (`04:147-149`). AC 4 →
  issue 05 (with the header caveat above). AC 5 → the identical CI line in all five
  issues, matching `CONVENTIONS.md:236` (`golangci-lint` pinned `v2.12.2`) and
  `SPECS.md:264` (`go test ./... -race -v`, then `bash upstream/sync_test.sh`).
- **Ordering is sound.** `depends_on` is `[]`, `[1]`, `[1]`, `[2,3]`, `[1,2,3,4]` —
  acyclic, monotone, and workable 01→05 top to bottom. It correctly carries only
  in-epic numbers, per `pipeline-interfaces.md` ("other issue numbers in this epic");
  the external pass flagged the absence of epic 2 / epic 7 numbers as a defect, which
  the schema does not support. Cross-epic blockers are instead carried in prose under
  `## Dependencies` in every issue.
- **Cross-epic dependency claims line up in both directions.** `EPIC_7.md:63` declares
  it blocks epic 8; epic 7 issue 04 promises `NormalizeGatewayURL` in
  `ai/auth/oauth/radius.go` (`:53-55`, `:111`) and explicitly defers the
  `gatewayConfig` credential field to epic 8 (`:157-161`), which is exactly what issue
  03 picks up at `:76-81`; epic 7 issue 05 declares `defaultRadiusGateway` in
  `cmd/pi-ai/oauth.go` (`:75-78`) naming epic 8 as its removal trigger, and issue 03
  (`:53-58`) tells the implementer to check for it rather than declaring a second
  constant — the one nuance is that issue 03 says to look in `ai/auth/oauth` whereas
  epic 7 put it in `cmd/pi-ai`, which the "declare it here and say so" fallback
  covers. Epic 9 issue 04 (`:87`) explicitly cedes `SPECS.md`'s package map to epic 8
  issue 05, so that Scope item is coordinated, not invented — the external pass's
  "invented docs scope" lead was rejected on that basis, and editing `SPECS.md` from
  an issue has precedent in epics 1 and 3.
- **Body quality is unusually high, and the citations hold up.** Roughly 25 `file:line`
  references into the Go tree were spot-checked and every one landed on what it
  claimed: `ai/events.go:99-105` (`ToolCallStartEvent` with no id/name field),
  `ai/types.go:73-80`, `:139-148`, `:211-215` (the `Clone` contract),
  `ai/catalog/compat.go:20-37` and `:52`, `ai/providers/providers_test.go:94` and
  `:110-112` (`wantProviderCount = 35` and the "has no models" assertion issue 04 must
  amend), `ai/internal/partialjson/partial.go:24`, `ai/options.go:41-45`, `:126-133`,
  `:224-232`, `:303`, `ai/headers.go:19`/`:38`, `ai/stream.go:32`/`:41`/`:60`,
  `ai/apis/simpleopts.go:20-47`, `ai/providers/refreshmodels.go:26`, `ai/auth.go:48-56`,
  `ai/provider.go:29-37`/`:346-367`/`:476-489`, `docs/auth.md:34-49`/`:91`/`:131`,
  `docs/architecture.md:22-23`, `ai/doc.go:2`, `README.md:7`. Only `docs/PORTING.md`
  is off (P3 above). Each issue also names the *trap* rather than the task —
  `encoding/json` zero-filling a dropped `contextWindow` (`03:29-37`),
  `new URL("/v1/config", gateway)` being an absolute-path join (`03:39-43`), sparse
  `contentIndex` growth (`01:49-52`), `Partial: output.Clone()` (`01:55-60`) — which
  is what makes these bodies actionable.
- **Repo conventions are reflected, not assumed.** Offline stdlib-only tests with no
  `t.Parallel()` and no build tags (`CONVENTIONS.md:142`, `:159`), discrete named test
  functions with the single table exception used exactly where `:147-152` permits it
  (`03:147-150`), pointer receivers for field-carrying error types (`:68-73` →
  `02:63-65`), upstream-verbatim error text (`:108-111` → `02:91-92`, `03:88-92`),
  `GOTMPDIR=$PWD/.gotmp` locally with `-race` CI-only (`:187`), Conventional Commits
  with scopes (`:193`), no logger and no new dependencies (`:127`, `EPIC_8.md:78-79`),
  and the deliberate non-uses (`EffectiveCacheRetention`, `httpretry`,
  `apis.BuildBaseOptions`) each argued rather than merely omitted.
- **Schema and index are clean.** Every issue carries the full frontmatter set from
  `pipeline-interfaces.md` with no extension fields; `resource` is present on all five
  and consistent with `status: open`; `issues/index.md` carries no frontmatter (correct
  for a non-root index) and its five mechanical bullets match every file's title,
  size, status and `gh_issue`. Cross-bundle links use plain relative paths
  (`../../../planning/...`) and in-bundle links use the leading-slash form, per
  `bundle-interfaces.md`.
- **Rejected leads.** Besides the two above: `ai/providers/radius_config.go`'s
  underscore was flagged as a `CONVENTIONS.md:29-35` violation, but
  `ai/providers/cloudflare_auth.go` is the existing precedent for a non-binding
  support file in that package, and issue 03 already flags the one-file-per-binding
  departure at `:63-68`; and issue 02's edit to `StreamOptions` was flagged as
  conflicting with epic 2 issue 05's restructure, but that issue (`:49-51`) keeps
  `StreamOptions` and embeds the new base in it, so the per-adapter block still
  belongs where issue 02 puts it.

## Open questions
- Is acceptance criterion 2's "248 lines" (`EPIC_8.md:47`) a provenance statement
  ("the upstream test file is ported in full") or a literal line budget? The issues
  read it the first way, which is the sensible reading, but the split across two PRs
  makes the difference matter for whoever signs the criterion off.
