# Issue Review Report 4

## Scope

- Reviewed: `docs/epics/epic-4-openai-family-adapters/issues/01-constrained-sampling-core.md` through `12-codex-session-ids-and-continuation-retry.md` (12 files) plus `docs/epics/epic-4-openai-family-adapters/issues/index.md`.
- Reviewed against: `docs/epics/epic-4-openai-family-adapters/EPIC_4.md` (the contract), with context from `docs/planning/SCOPE.md#milestone-4-openai-family-adapters`, `docs/planning/SPECS.md`, `docs/planning/CONVENTIONS.md`, `docs/planning/scope/06-adapter-updates.md`, `docs/planning/scope/12-constrained-sampling.md`, `docs/epics/index.md`, `docs/epics/log.md`, `docs/PORTING.md`, and the Go tree the issues cite (`ai/`, `ai/apis/*`). No `CLAUDE.md`, `AGENTS.md`, `CONTRIBUTING.md`, PR template or issue template exists in this repo; `.github/` holds only `workflows/`. There is no `docs/planning/DRIFT.md`.
- GitHub verification: **verified** - `gh` authenticated for `kern-ia/kern-link`. Milestone 21 ("Epic 4: OpenAI-family adapters") exists and holds exactly 13 open issues (#121 tracking + #147-#158). All twelve `gh_issue`/`resource` values resolve, are `OPEN`, carry milestone 21 and labels `enhancement`,`upstream-sync`, and all twelve are native sub-issues of #121 (`gh api repos/kern-ia/kern-link/issues/121/sub_issues`). One title discrepancy - see P3 below.
- External review: `openai/gpt-5.6-sol` via `opencode run --agent plan`, run in two batches (epic coverage vs. acceptance criteria; ordering, dependencies and sizing). A third batch (frontmatter schema and GitHub metadata) produced no output within ~20 minutes and was abandoned; that area was reviewed **natively** instead - frontmatter keys, section headings, index bullets, link forms and all GitHub state were checked directly. External output was treated as leads and every finding below was re-verified against the files.

## Findings

### P1 - Three issues wire an OpenAI `tool_choice` option that does not exist and that no issue creates

- Location: `docs/epics/epic-4-openai-family-adapters/issues/05-completions-deferred-tools-and-finish-reason.md:47-49` and `:77-79`; `.../09-openai-responses-compat-and-wiring.md:45`, `:63`, `:106`; `.../11-codex-request-body-and-stop-reasons.md:30-31`, `:47-48`, `:94-95`.
- Source: `ai/options.go:196-269` - `StreamOptions` carries only per-adapter tool-choice fields (`GoogleToolChoice:202`, `MistralToolChoice:227`, `BedrockToolChoice:265`). There is **no** OpenAI-family tool-choice field, and grepping `tool_choice|ToolChoice` across `ai/apis/openaicompletions`, `ai/apis/openairesponses` and `ai/apis/azure` returns nothing outside `ai/apis/codex/params.go:47,102`, where it is a hardcoded `"auto"`.
- Problem: issue 05 calls it "a one-line type widening on the same options struct: `toolChoice` accepts the full OpenAI tool-choice union rather than the four hand-listed shapes" (`05:47-49`) and then hedges the location - "on `ai.StreamOptions`' completions field (**or wherever the flat per-adapter field lives**)" (`05:77-79`). Nothing lives there. Issue 09 asks for `tool_choice` passthrough into `wireRequest` (`09:63`) with `TestToolChoicePassedThrough` (`09:106`), and issue 11 asks for `options.toolChoice ?? "auto"` (`11:30-31`, `11:94-95`). No Epic 2 issue adds the field either - grepping `ToolChoice` across `docs/epics/epic-2-*/issues/` is empty.
- Impact: the first of the three issues to be worked has to invent a new **public** `ai.StreamOptions` field (a core-types change Epic 2 owns), and the other two will each find it named differently or not at all. Three PRs, three different guesses at a released API surface, in the epic immediately before Epic 9 cuts v0.2.0.
- Recommendation: name the field once and assign it. Either add `OpenAIToolChoice` (following the `GoogleToolChoice`/`MistralToolChoice` precedent at `ai/options.go:200-232`) to Epic 2 issue 05 `05-provider-request-options.md` and make issues 05/09/11 depend on it, or declare it in epic-4 issue 05's `## Scope` with its exact Go name and type and have issues 09 and 11 list issue 05 in `depends_on`.

### P1 - Issue 04's thinking-budget criteria reference `StreamOptions.ThinkingBudgets`, which does not exist

- Location: `docs/epics/epic-4-openai-family-adapters/issues/04-completions-thinking-formats.md:68-73` ("`ai.ThinkingBudgets` already exists (`ai/options.go:325-326`)") and `:109-111` (`TestThinkingTokenBudgetHonorsPerLevelOverride` - "overriding only `high` via `StreamOptions.ThinkingBudgets`").
- Source: `ai/options.go:321-326` - `ThinkingBudgets` is a field of **`SimpleStreamOptions`**, not `StreamOptions`. The only budget field on `StreamOptions` is `BedrockThinkingBudgets` (`ai/options.go:276-278`), which exists precisely because `ai/apis/bedrock/bedrock.go:93-127` has to stash the simple-options value onto `StreamOptions` to reach the low-level `Stream`.
- Problem: the work the issue describes lands in `buildParams(model, chat, opts *ai.StreamOptions)` (`ai/apis/openaicompletions/openaicompletions.go:340`), which never sees `SimpleStreamOptions`. As written, four of the seven acceptance criteria (`04:103-114`) cannot be satisfied without a new carrier field on `StreamOptions`, which the issue neither scopes nor attributes to Epic 2 (its only stated blocker is Epic 2 issue 04, `04:138-140`).
- Impact: the implementer discovers the gap mid-PR and either invents another public options field or silently drops the per-level override criterion.
- Recommendation: add a bullet to `04:51-78` naming the carrier explicitly - e.g. `OpenAIThinkingBudgets` on `StreamOptions` mirroring `BedrockThinkingBudgets` - or, if Epic 2 should own it, add it to Epic 2 issue 05 and list that as a blocker. Fix the AC wording at `04:110` to name the real field.

### P2 - Issue 01 reverses the epic's acceptance criterion 3, and the epic was left unamended

- Location: `docs/epics/epic-4-openai-family-adapters/issues/01-constrained-sampling-core.md:43-51` ("At `936aff00` the field is `Tool.constrainedSampling?` ... So it goes on `ai.Tool` and **no deviation is needed at all**; `StreamOptions` is not touched") and `:55-69`.
- Source: `EPIC_4.md:35-36` ("`ConstrainedSamplingConfig` lands in `ai.StreamOptions`, continuing the established per-adapter-options deviation") and `EPIC_4.md:52-53` (acceptance criterion 3: "Constrained sampling is ported, with `ConstrainedSamplingConfig` reachable through `ai.StreamOptions`"). Same text in `docs/planning/scope/12-constrained-sampling.md` (Verdict section) and `docs/planning/SCOPE.md:200-201`.
- Problem: the correction is well-argued and properly flagged as an assumption (it instructs the implementer to record it in the PR body and `docs/PORTING.md`, `01:51`, `01:96-98`), so this is not invented scope. But the epic's criterion 3 is left literally unsatisfiable: after issue 01 lands, `ai.StreamOptions` is untouched and criterion 3 reads as failed. `close-epic` and `review-implementation` both grade against the epic's acceptance criteria.
- Impact: the epic can never be closed green against its own text, and the next reader cannot tell whether the deviation was decided or drifted.
- Recommendation: user's call to arbitrate, then amend `EPIC_4.md:35-36` and `:52-53` (and ideally `docs/planning/scope/12-constrained-sampling.md`) to say `ai.Tool`. Note that this also makes issue 01 a change to package `ai`'s public `Tool` struct (`ai/types.go:243-247`), i.e. core-type work inside an adapter epic - worth a sentence in the epic body either way.

### P2 - Issues 03, 10 and 11 omit their cross-epic blockers from `## Dependencies`

- Location: `.../03-completions-grammar-custom-tools.md:139-143` (lists only issue 01) while `03:51-52` calls `compat.SupportsOpenAIGrammarTools` and `03:132-134` names Epic 2 issue 04 only under *Relevant files*; `.../10-azure-responses-wiring.md:100-104` (lists only issues 06 and 08) while `10:24-29` reads `supportsOpenAIGrammarTools` / `supportsStrictMode`; `.../11-codex-request-body-and-stop-reasons.md:124-129` (lists only issues 06/07/08) while `11:26-29` uses `SplitDeferredTools` and `11:32-35` populates `EndTurn`.
- Source: the sibling issues do it correctly - `01:159-164`, `02:131-138`, `04:138-141`, `05:142-147`, `07:131-135`, `08:140-147`, `09:134-140` all name their Epic 2 blockers explicitly. Epic 2 issue 04 (`/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md`), issue 02 and issue 11 are the owners.
- Problem: `depends_on` only holds intra-epic numbers, so `## Dependencies` prose is the only place a cross-epic blocker can be recorded. Three issues drop it.
- Impact: a supervisor picking "the next unblocked issue" from `depends_on` plus the Dependencies section starts issue 03 or issue 10 against a core that does not yet compile, and only finds out at `go build`.
- Recommendation: add "**Blocked by**: Epic 2 issue 04" to `03:139` and `10:100`; add "Epic 2 issue 02 (`EndTurn`) and issue 11 (`SplitDeferredTools`)" to `11:124`.

### P2 - Sizing: issue 09 and issue 11 are under-declared; issue 10 contradicts its own size

- Location: `.../09-openai-responses-compat-and-wiring.md:10` (`size: M`) against `09:22-23` and `09:68-69` (ports `test/openai-responses-compat.test.ts`, **+472** lines) plus eight adapter surfaces at `09:54-67`; `.../11-codex-request-body-and-stop-reasons.md:10` (`size: M`) against `11:58-61` ("`test/openai-codex-stream.test.ts` (+778 upstream - this PR takes the half about request shape and stream termination)") plus changes to two production files and both transports (`11:44-57`); `.../10-azure-responses-wiring.md:10` (`size: S`) against `10:106-109` ("Target ~500 changed lines").
- Source: `pipeline-interfaces.md` size bands - S is under 200, M is 200-500, L is 500-1000 - and `EPIC_4.md:54-55` (acceptance criterion 4: ported upstream tests "come across with the code").
- Problem: issue 09's own PR-size note already anticipates the overflow and offers to "land the adapter change with the highest-value cases and open a follow-up for the remainder" (`09:145-148`), which is criterion 4 being negotiated away at planning time rather than a size being declared honestly. Issue 10 is declared S and then told to target ~500 lines - 2.5x its own band; its real content (upstream +52 implementation, +136 reasoning-replay test, +30 base-URL test, per `10:21-22`, `10:41-43`, `10:53-56`) puts it in M.
- Impact: sizing is the finding the implementer feels first; two of the three end-of-chain issues will blow their band, and the escape hatch at `09:145-148` will be used.
- Recommendation: re-declare issue 09 and issue 11 as `L` (and give each the same "here is the clean cut if it approaches 1000" sentence issue 08 has at `08:152-158`); re-declare issue 10 as `M` or rewrite its size note to match `S`.

### P2 - Epic acceptance criterion 5 (provenance + `docs/PORTING.md`) is absent from issues 05, 07 and 08

- Location: `.../05-completions-deferred-tools-and-finish-reason.md:51-82` (`## Scope`) and `:96-127` (AC); `.../07-responses-shared-namespace-and-deferred-tools.md:50-77` and `:90-115`; `.../08-responses-shared-stream-decode.md:55-81` and `:94-125` - none mentions a `// Ports:` header or `docs/PORTING.md`.
- Source: `EPIC_4.md:56-57` - "Every file touched carries a `// Ports:` provenance header, and its disposition is reflected in `docs/PORTING.md`" - and `docs/planning/CONVENTIONS.md` ("Every ported file carries a provenance header ... The header names specific upstream symbols when the mapping is partial").
- Problem: nine of twelve issues carry the requirement (`01:96-98,137-138`, `03:81-82`, `04:78`, `06:77-79`, `09:70-71`, `10:57`, `11:87-90`, `12:78-81,121-122`); three do not, and all three add new upstream symbols to already-ported files (`openaicompletions.go`, `messages.go`, `stream.go`) whose `// Ports:` symbol lists therefore go stale.
- Impact: `upstream/sync.sh` and the Epic 9 disposition sweep navigate by those headers and by `docs/PORTING.md`; a stale symbol list makes the next sync read ported code as unported.
- Recommendation: add the one-line "keep the `// Ports:` header's symbol list current" bullet (the wording already used at `04:78` and `10:57`) to the `## Scope` of issues 05, 07 and 08, and an AC checkbox where a `PORTING.md` row actually changes.

### P2 - Issue 12 adds a new public `ai` API (`ai/uuid.go`, `ai.UUIDv7`) the epic never scoped

- Location: `.../12-codex-session-ids-and-continuation-retry.md:37-43` ("**No epic currently owns `utils/uuid.ts`**; it lands here because Codex is its only in-repo consumer") and `:51-59` (new `ai/uuid.go` in package `ai` with `func UUIDv7() string`).
- Source: `EPIC_4.md:24-36` - the epic's `## Scope` names four adapter sources plus `api/constrained-sampling.ts`, nothing else. Grepping `uuid` across `docs/planning/SCOPE.md` and `docs/planning/scope/` returns nothing: no scope decision covers `src/utils/uuid.ts` anywhere in the program.
- Problem: the orphan is honestly flagged, which is the right behaviour, but the resolution was taken unilaterally inside an issue. It puts a new **exported** symbol in package `ai` (the core Epic 2 owns) from an adapter epic, and the file is upstream-public API (re-exported from `src/index.ts`, `12:39-40`).
- Impact: a public symbol enters v0.2.0 without an epic or a decision record behind it; Epic 8's issues cannot know it will already exist.
- Recommendation: user's call - either accept it here and add a one-line bullet to `EPIC_4.md:32-36` ("plus `src/utils/uuid.ts` to `ai/uuid.go`, orphaned by the split"), or move it to Epic 2 and make issue 12 depend on it. Do not leave the ownership recorded only inside issue 12's Summary.

### P3 - Ten GitHub issue titles drop the colon present in the local files

- Location: `03:3` `title: "openai-completions: emit and stream grammar custom tools"` versus `gh issue view 149 --json title`, which returns `openai-completions emit and stream grammar custom tools`. Same for #150, #151, #152, #153, #154, #155, #156, #157, #158. #147 and #148 match exactly (their titles contain no colon).
- Source: `pipeline-interfaces.md` - the issue file's `title` and its `gh_issue` describe one artifact; `issues/index.md:3-14` reproduces the local titles with the colon.
- Problem: local files, `issues/index.md` and `docs/epics/log.md:34-45` all use one wording; GitHub uses another for ten of twelve.
- Impact: cosmetic, but a title search from either side misses, and later reconciliation cannot match on title.
- Recommendation: `gh issue edit <n> --title "<exact frontmatter title>"` for #149-#158, or accept the GitHub form and normalise the local titles. Worth a look at whichever step stripped the colon, since Epic 5's issues show the same shape.

### P3 - Issue 12 renames a required section heading

- Location: `.../12-codex-session-ids-and-continuation-retry.md:83` - `## Out of scope, recorded rather than ported`.
- Source: `pipeline-interfaces.md`, issue body template - the heading is `## Out of scope`. All eleven siblings use it verbatim (`01:100`, `02:77`, `03:84`, `04:80`, `05:84`, `06:81`, `07:79`, `08:83`, `09:73`, `10:59`, `11:63`).
- Problem: the qualifier is genuinely useful content, but it belongs in the section body, not the heading.
- Impact: any consumer matching headings exactly reads issue 12 as missing its Out of scope section.
- Recommendation: restore `## Out of scope` at `12:83` and move "recorded rather than ported" into a lead sentence beneath it.

### P3 - `issues/index.md` links are `./file.md`, not bundle-relative absolute

- Location: `.../issues/index.md:3-14` - links of the form `(./01-constrained-sampling-core.md)`.
- Source: `bundle-interfaces.md`, "The two bundles and their link rules" - within a bundle, links use a bundle-relative absolute path with a leading `/` (`/epic-4-openai-family-adapters/issues/01-constrained-sampling-core.md`). The issue bodies themselves get this right (e.g. `03:141`, `08:142-150`).
- Problem: the index uses a third form.
- Impact: none inside a renderer that resolves relative paths; it breaks the bundle's own convention and diverges from how the issue bodies link.
- Recommendation: low priority, and it is **program-wide** - epics 2, 3 and 5 index files use the same `./` form. Fix all nine together or leave all nine, but do not fix epic 4 alone.

### P3 - The `## PR size note` is identical boilerplate in eleven of twelve issues

- Location: `01:166-169`, `02:140-143`, `03:145-148`, `04:143-146`, `05:150-153`, `06:141-144`, `07:139-142`, `10:106-109`, `11:131-134`, `12:149-152` - all read "Target ~500 changed lines; if this grows past ~1000, split it before opening the PR." Only `08:152-158` and, partially, `09:143-148` add issue-specific reasoning.
- Source: the review contract asks that `PR size` be "concrete enough to act on".
- Problem: "~500" is the top of the M band for every M issue and 2.5x the band for the one S issue, so the note conveys no information beyond the `size:` field it should be explaining.
- Impact: minor; it removes the one place an implementer could learn where a split would be clean.
- Recommendation: follow issue 08's pattern - one sentence naming the natural cut line - at least for issues 05, 07, 09 and 11.

### P3 - Issue 12 attributes the WebSocket recorders to the wrong file in `## Scope`

- Location: `.../12-codex-session-ids-and-continuation-retry.md:60-67` - under the bullet `ai/apis/codex/codex.go:`, "the failure recorder (`codexRecordWebSocketFailure`, `:164`) and the SSE-fallback recorder (`:151`)".
- Source: `ai/apis/codex/websocket.go:164` is `func codexRecordWebSocketFailure`; `ai/apis/codex/codex.go:164` is a bare `return`. Its call site is `codex.go:228`. The issue's own `## Relevant files` gets this right (`12:133-135` lists them under `websocket.go`).
- Problem: bare `:NNN` references nested under a `codex.go:` bullet read as `codex.go` line numbers.
- Impact: two wrong jumps for the implementer; every other line reference in this epic checked out (see Coverage notes).
- Recommendation: qualify the two references as `websocket.go:151` / `websocket.go:164` at `12:65-67`.

## Coverage notes

- **GitHub state is clean.** Milestone 21 exists with the exact title `Epic 4: OpenAI-family adapters` and holds precisely #121 + #147-#158 - no strays, no orphans. Every `gh_issue`/`resource` pair in the twelve files resolves to the right number and URL, every issue is `OPEN` (matching `status: open`), every issue carries milestone 21 and the labels `enhancement`,`upstream-sync` - and correctly **not** `epic`, which #121 alone carries. All twelve are registered as native sub-issues of #121, so the tracking issue's progress bar is honest.
- **Frontmatter is schema-clean across all twelve.** `type`, `title`, `description`, `tags: [epic-4]`, `timestamp`, `epic: 4`, zero-padded `issue`, `slug`, `size`, `status`, `gh_issue`, `resource`, `depends_on` - no missing field, no extra field, no `gh_pr` leaked in early. Every `slug` matches its filename and every `title` matches its `# H1` and its `issues/index.md` bullet.
- **All seven body sections are present, in order, in all twelve** (only issue 12's third heading is renamed, above).
- **The dependency graph is sound.** 01 to 03 to 05; 01 to 06 to 07 and 08; 06/07/08 to 09; 06/08 to 10; 06/07/08 to 11 to 12: acyclic, no forward reference, and frontmatter `depends_on` agrees with every `## Dependencies` prose statement on intra-epic edges. Serial ordering deliberately separates the pairs that would otherwise collide on the same function (03/05 on `buildParams` + `convertTools`, 06/07 on `ConvertMessages`, 11/12 on Codex `run`).
- **Line references into the Go tree are unusually accurate.** A ~60-reference sample across `openaicompletions.go`, `openairesponses/messages.go` and `stream.go`, `openairesponses.go`, `azure.go`, `codex/codex.go`, `params.go` and `websocket.go`, `ai/types.go:243-247`, `ai/hash.go:14` and `ai/options.go:325-326` landed on the named declaration in every case but the two noted above. Stated file line counts (444/518/513/1388/390/388/216/563) are exact.
- **Acceptance criteria are concrete and testable.** Named Go test functions with the asserted wire shape and, where it matters, the verbatim upstream error text (`05:114`, `09:112`, `10:74-76`, `11:99-101`) - which is exactly the `CONVENTIONS.md` rule that error text is load-bearing for `ai/retry.go`.
- **Conventions are reflected, not assumed.** Offline `httptest` only, discrete named tests rather than table-driven, `GOTMPDIR=$PWD/.gotmp` for the local run, the three CI gates (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint` v2.12.2), `gofmt -l .`, a Conventional Commit example per issue, no-logger / no-panic (`03:53-56`), determinism traps called out where map ordering would break prompt caching (`01:89-92`, `05:60-64`, `07:64-68`), and `02:52-55` correctly *forbids* a `// Ports:` header on new code. Epic acceptance criterion 6 is carried by all twelve.
- **Epic boundaries are respected.** The classifier audit is deferred to Epic 9 in six places (`02:83-87`, `05:92-94`, `08:90-92`, `09:86`, `11:67-74`, `12:96-98`); the Copilot dynamic-headers gap is explicitly refused and pointed at Epic 5 (`09:75-81`), which does have an issue for it (`epic-5-remaining-adapters/issues/01-copilot-dynamic-headers.md`); Azure is correctly denied deferred tools (`10:38-40`, `07:83-84`); the two Codex upstream changes with no Go counterpart are recorded rather than ported (`12:85-95`), which is the right call against `websocket.go:15-25`. Every cross-epic link target checked (epic-2 issues 01/02/04/05/06/11, epic-3 issue 06, the epic-5/6/8/9 EPIC files, `scope/16-copilot-headers-gap.md`) exists.
- **`docs/epics/log.md`** carries a `**Creation**` entry for the epic and all twelve issues with their GitHub numbers (`log.md:8`, `:34-45`).
- Deliberate, well-argued deviations that are **not** findings: issue 02's `httpretry` change reaching the Epic 5 adapters "for free" is called out as such (`02:28-36`, `02:79-80`); issue 11 leaving Codex's retry-delay clamp where upstream now throws is routed to Epic 9 with reasoning (`11:67-74`); issue 06 porting upstream's `isSameProviderAndApi`/`isSameModel` restructure because issue 07 needs it (`06:68-70`).

## Open questions

- **Criterion 3 needs an owner's ruling.** `EPIC_4.md:52-53` and `docs/planning/scope/12-constrained-sampling.md` both say `ai.StreamOptions`; issue 01 reads upstream `936aff00` as `Tool.constrainedSampling`. Only the upstream source settles it, and the frozen checkout is not vendored in this repo (`upstream/` holds `UPSTREAM.lock` and `sync.sh` only), so this review could not verify the upstream shape independently. If issue 01 is right, the epic and the scope decision both need amending; if not, issue 01 does.
- **Does `src/utils/uuid.ts` belong to Epic 4 at all?** Issue 12 places it here by process of elimination (`12:40-43`). If Epic 8's `pi-messages` work also needs it, Epic 2 is the more natural home for a public `ai` symbol.
- The third external batch (schema/GitHub metadata) never returned; that area was covered natively and directly against `gh`, so no gap remains - but a second independent reading of the frontmatter did not happen.
