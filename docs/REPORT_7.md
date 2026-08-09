# Issue Review Report 7

## Scope
- Reviewed: `docs/epics/epic-7-four-new-oauth-flows/issues/01-xai-device-code-oauth.md`,
  `02-kimi-coding-device-code-oauth.md`, `03-openrouter-pkce-oauth.md`,
  `04-radius-gateway-oauth.md`, `05-cli-login-new-flows.md`,
  `06-auth-docs-and-porting.md`, and `issues/index.md`
- Reviewed against: `docs/epics/epic-7-four-new-oauth-flows/EPIC_7.md` (the contract),
  with context from `docs/planning/SCOPE.md#milestone-7-four-new-oauth-flows`,
  `docs/planning/SPECS.md`, `docs/planning/CONVENTIONS.md`, `docs/epics/index.md`,
  `docs/epics/epic-6-auth-core-and-env-api-key-bindings/` (EPIC_6 + issues 01/04/06/08/11),
  `docs/epics/epic-8-pi-messages-and-radius/EPIC_8.md`, `docs/auth.md`,
  `docs/PORTING.md`, `README.md`, `.github/workflows/test.yml`, and the Go tree
  the issues cite (`ai/auth/oauth`, `ai/providers`, `ai/images`, `cmd/pi-ai`).
  No `CLAUDE.md`, `AGENTS.md`, `CONTRIBUTING.md` or `.github/ISSUE_TEMPLATE/` exists in the repo.
- GitHub verification: verified — milestone 24 (`Epic 7: Four new OAuth flows`),
  tracking issue #124, and sub-issues #181–#186 all exist, are OPEN, carry titles
  identical to the local frontmatter, sit on milestone 24, and are native
  sub-issues of #124 (`repos/kern-ia/kern-link/issues/124/sub_issues`). Labels are
  `enhancement`/`documentation` + `upstream-sync`, with `epic` correctly reserved
  for the tracking issue.
- External review: `openai/gpt-5.6-terra` via `opencode run --agent plan` — one
  coverage/sizing/ordering pass was launched but returned no output before the
  report was written (the host was running many concurrent opencode sessions), so
  every finding below is natively derived and verified line by line against the
  files. Schema and GitHub metadata were verified natively with `gh`.

## Findings

### P2 — Issue 05 sorts the login list, then appends Radius, contradicting its own asserted picker order
- Location: `docs/epics/epic-7-four-new-oauth-flows/issues/05-cli-login-new-flows.md:71-79`
  (derive from the registry "sorted by id", then "**Append** the Radius entry") versus
  `:106-108` (acceptance criterion: the picker offers "exactly `anthropic`,
  `github-copilot`, `kimi-coding`, `openai-codex`, `openrouter`, `radius`, `xai`,
  **in that order**")
- Source: `docs/epics/epic-7-four-new-oauth-flows/EPIC_7.md:52` — acceptance criterion 4,
  "`cmd/pi-ai login` offers each of the four flows and completes them"
- Problem: sorting the six registry-derived entries and then appending Radius produces
  `anthropic, github-copilot, kimi-coding, openai-codex, openrouter, xai, radius`.
  The acceptance criterion asserts the fully alphabetical order, with `radius` sixth.
  The Scope and the AC of the same issue cannot both be satisfied.
- Impact: the implementer writes the code from Scope and the test from the AC, and the
  test fails on the first run — on exactly the ordering assertion the issue introduced
  the sort to make deterministic.
- Recommendation: change the Scope bullet to "append the Radius entry, then sort the
  whole list by id" (one word), or change the AC to assert Radius last. Sorting after
  the append is the better fix: it survives epic 8 removing the special case.

### P2 — `postForm`'s specified signature cannot produce the error strings issues 02 and 04 require
- Location: `issues/01-xai-device-code-oauth.md:75-81` — `postForm(ctx, url string, fields url.Values) (int, map[string]any, error)`,
  "a decoded body on **every** status. A body that is not a JSON object decodes to an
  empty map" — versus `issues/02-kimi-coding-device-code-oauth.md:74-75, :84-85, :89-90`
  (`Kimi Code device authorization failed with status <status>[: <body>]`,
  `Kimi Code device token request failed with status <status>[: <body>]`), and
  `issues/02-...md:121-124` which puts "Widening `postForm`" **out of scope** while in the
  same breath instructing "extend it there in a small, separate commit".
- Source: `EPIC_7.md:29-32` — the four flows and their upstream tests are ported as one
  epic, so the shared plumbing they share should be specified once.
- Problem: the raw response text is unrecoverable from `map[string]any` (a non-JSON 502
  body decodes to an empty map by issue 01's own rule), so the `[: <body>]` suffixes
  issue 02's acceptance criteria pin cannot be produced. Issue 01 defines the helper's
  contract without the field its two declared consumers need, and issue 02 forbids and
  mandates the same edit two lines apart.
- Impact: issue 02's PR must reopen `token.go` — the file issue 01 owns — against an
  Out-of-scope bullet, and issue 04 (`Could not load Radius OAuth config from <gateway>: <status> <body>`,
  `issues/04-radius-gateway-oauth.md:113-115`) hits the same wall.
- Recommendation: in issue 01's Scope, specify `postForm(...) (int, []byte, map[string]any, error)`
  (or a small struct) returning the raw body alongside the decoded map, and delete
  issue 02's contradictory Out-of-scope bullet.

### P2 — The `callbackHost` rename in issue 03 omits `codex.go` from its blast radius and miscounts the callers
- Location: `issues/03-openrouter-pkce-oauth.md:128-129` — "`ai/auth/oauth/anthropic.go` —
  rename `anthropicCallbackHost` to `callbackHost` (mechanical; update its doc comment to
  name **both** callers)"; the same file's Relevant files list (`:206`) names only
  `anthropic.go:44`
- Source: `ai/auth/oauth/codex.go:515` — `StartCallbackServer(anthropicCallbackHost(), codexCallbackPort, …)`;
  the other existing caller is `ai/auth/oauth/anthropic.go:105`
- Problem: the function already has two callers *before* this issue, in two different
  files. Scope names one file, so a PR that follows Scope literally leaves `codex.go`
  referring to a symbol that no longer exists — the package does not compile. "Both
  callers" is also arithmetically wrong for the post-rename state, which has three.
- Impact: a build break discovered at `go build`, plus an unflagged collision: issue 03's
  Dependencies (`:225-226`) sequences only against epic 6 issue 08's rewrite of
  `anthropic.go`, while epic 6 issue 08 rewrites `codex.go` too
  (`docs/epics/epic-6-auth-core-and-env-api-key-bindings/issues/08-anthropic-codex-login-race.md:21-23`).
- Recommendation: list `ai/auth/oauth/codex.go` in Scope and Relevant files, reword to
  "all callers (`anthropic.go:105`, `codex.go:515`, and the new `openrouter.go`)", and
  extend the epic 6 issue 08 sequencing note to both files.

### P2 — Acceptance criterion 5 is only half covered: the four ported test files are never required to carry a `// Ports:` header
- Location: `issues/01-...md:179-180`, `issues/02-...md:167-168`, `issues/03-...md:190-191`,
  `issues/04-...md:220-222` — each demands the header on the flow `.go` file only
  (`xai.go`, `kimicoding.go`, `openrouter.go`, `radius.go`); `issues/06-auth-docs-and-porting.md:127-130`
  greps only for `// Ports: packages/ai/src/auth/oauth`, which by construction cannot
  match a test file
- Source: `EPIC_7.md:54-55` — "`// Ports:` headers on **every ported file**"; and
  `docs/planning/CONVENTIONS.md:50` — "**Every ported file carries a provenance header**"
- Problem: the four new `*_test.go` files are ported files by the epic's own reckoning
  (`EPIC_7.md:29-32` counts 1056 lines of upstream tests), and every existing test file in
  the package already carries one (`ai/auth/oauth/copilot_test.go:3`,
  `devicecode_test.go:3`, plus `anthropic_test.go`, `callback_test.go`, `codex_test.go`,
  `pkce_test.go`, `token_test.go`). No issue asks for them.
- Impact: AC5 cannot be signed off at epic close without a sweep nobody was assigned;
  the next `upstream/sync.sh` run cannot route the four test files back to their Go homes.
- Recommendation: add a criterion to issues 01–04 —
  `// Ports: packages/ai/test/<flow>-oauth.test.ts` on the new `_test.go` file — and widen
  issue 06's grep to `git grep -c "// Ports: packages/ai/test/.*-oauth.test.ts" ai/auth/oauth`.

### P2 — Sizing is internally inconsistent, and every PR-size note contradicts its own `size` field
- Location: `issues/01-...md:10` (`size: M`) with `:216-219`; `issues/02-...md:10` (`M`) with
  `:199-202`; `issues/03-...md:10` (`M`) with `:228-231`; `issues/04-...md:10` (`L`) with
  `:257-260`; `issues/05-...md:10` (`S`) with `:156-159`; `issues/06-...md:10` (`S`) with `:165-168`
- Source: `_shared/pipeline-interfaces.md` — "`size`: S ≈ under 200 changed lines,
  M ≈ 200–500, L ≈ 500–1000"; upstream line counts in `EPIC_7.md:26-32`
- Problem: two separate defects. (a) Issue 03 ports 311 flow + 322 test upstream lines
  (633) *and* writes a bespoke one-shot callback server (`:79-115`, six distinct HTTP
  behaviours) *and* renames a package symbol, yet is graded `M`; issue 04 ports 403 + 129
  (532) and is graded `L`. The smaller job carries the larger grade. Issues 01 (239 + 335
  plus a new shared helper and its test) and 02 (310 + 270) sit in the same position.
  (b) All six issues carry the identical note "Target ~500 changed lines; if this grows
  past ~1000, split it before opening the PR" — which is the M/L boundary, not the S
  ceiling, so it directly contradicts `size: S` on issues 05 and 06 and gives no guidance
  at all on issues 01–03.
- Impact: sizing is what a developer feels first; `M` on issue 03 sets the wrong
  expectation for a PR that will realistically approach the split threshold, and the
  boilerplate note removes the one signal that would have caught it.
- Recommendation: re-grade 01–03 as `L` (or split issue 03's callback server into its own
  step), and make each PR-size note state the band its `size` field claims —
  "~150 changed lines" for the two `S` issues, "~700, split past ~1000" for the `L` ones.
  Issue 04's note is the model: it says where the split seam is (`:259-260`).

### P3 — "epic 12" has no counterpart in this bundle
- Location: `issues/01-xai-device-code-oauth.md:22-24` — "`PollDeviceCodeFlow`, ported in
  epic 12 and already consumed by the Copilot and Codex flows"
- Source: `docs/epics/index.md:12-20` — this bundle contains epics 1–9 only. The number
  comes from the earlier build program, which the code still cites
  (`ai/auth/oauth/devicecode.go:5`, "epic 12, issue 02")
- Problem: a reader inside `docs/epics/` will look for epic 12 and find nothing.
- Impact: traceability only; the symbol exists and the claim is true.
- Recommendation: cite the file (`ai/auth/oauth/devicecode.go`) rather than a number from
  a retired numbering, or qualify it as "the original build program's epic 12".

### P3 — Issue 01 claims epic 6 lands two fields unused; it lands one
- Location: `issues/01-xai-device-code-oauth.md:114-117` — "`Name` … `IsSubscription: true`,
  `LoginLabel: …` — the **two fields** [epic 6 issue 01] lands unused for exactly this"
- Source: `docs/epics/epic-6-auth-core-and-env-api-key-bindings/issues/01-auth-contract-surface.md:60-64`
  — "`isSubscription` is set on exactly three of the flows this port already has … so this
  issue sets it on all three; **the field is not left dead**. `loginLabel` is set only on
  flows that arrive in Epic 7"
- Problem: `IsSubscription` arrives already in use on Anthropic, Copilot and Codex; only
  `LoginLabel` lands unused.
- Impact: minor, but the same sentence is repeated at `issues/05-...md:54-59`, so the
  wrong premise propagates.
- Recommendation: reword to "the `LoginLabel` field epic 6 issue 01 lands unused for
  exactly this".

### P3 — Radius's default gateway constant lands in `cmd/pi-ai` although the epic assigns gateway config to Epic 8
- Location: `issues/05-cli-login-new-flows.md:41-52, :75-78` — register Radius explicitly
  with `defaultRadiusGateway = "https://radius.pi.dev"` declared in `cmd/pi-ai/oauth.go`
- Source: `EPIC_7.md:41-43` — Out of scope: "The `radius` **provider binding** and its
  gateway config, which depend on this epic's radius flow — epic 8"
- Problem: a hardcoded default gateway URL is gateway config, and it lands in this epic.
  The issue argues the case well (`:44-52`: AC4 is otherwise unmeetable, epic 8 is named
  as the removal trigger, and no flag or env var is invented), so this is a reasoned
  assumption rather than drift — but it is not labelled as one.
- Impact: reads as a boundary breach to anyone auditing epic 7 against epic 8.
- Recommendation: add one line to issue 05 explicitly flagging it as an assumption
  against `EPIC_7.md:41-43`, taken because AC4 (`EPIC_7.md:52`) cannot be met without it.

### P3 — Minor citation drift and a case-inconsistent symbol name
- Location: `issues/01-xai-device-code-oauth.md:191` — "`ai/auth/oauth/token.go:24`
  `httpClient`"; the var is at `ai/auth/oauth/token.go:22` (line 24 is `clock`'s doc
  comment). `issues/03-openrouter-pkce-oauth.md:41-42` writes `oauthSuccessHtml` /
  `oauthErrorHtml` where the Go symbols are `oauthSuccessHTML` / `oauthErrorHTML`
  (`ai/auth/oauth/page.go:98, :104`), which the same issue gets right at `:203`.
- Source: `_shared/review-interfaces.md` — citations are the traceability the artifacts
  are graded on
- Problem: two-line offset on one citation, and one symbol given in its TypeScript casing
  in prose.
- Impact: negligible; roughly forty other repo citations across the six issues were
  spot-checked and are exact.
- Recommendation: fix in passing if these issues are edited for anything above.

### P3 — `issues/index.md` uses `./`-relative links where the bundle rule mandates bundle-relative absolute paths
- Location: `docs/epics/epic-7-four-new-oauth-flows/issues/index.md:3-8` — `(./01-xai-device-code-oauth.md)`
- Source: `_shared/bundle-interfaces.md`, link table — "Within a bundle | bundle-relative
  absolute path (leading `/`, relative to the bundle root)"
- Problem: the bullets use `./` instead of `/epic-7-four-new-oauth-flows/issues/…`.
- Impact: none in practice, and it is uniform across the repo — `docs/epics/epic-6-.../issues/index.md:3`
  does the same, while `docs/epics/log.md:68-73` and `docs/epics/index.md:12-20` use the
  mandated form. Recorded for consistency, not as a defect of this epic.
- Recommendation: leave as-is unless the convention is normalised bundle-wide; if it is,
  fix all nine issue indexes in one pass.

## Coverage notes

- **All six epic acceptance criteria are traceable to issues.** AC1 and AC2
  (`EPIC_7.md:48-51`) → issues 01–04, each naming its upstream source file, its upstream
  test file and the exact line counts from `EPIC_7.md:26-32`. AC3 (`:52-53`) → issue 06,
  which does the hard part rather than the easy one: its per-flow category table
  (`issues/06-...md:34-39`) correctly places `xai` and `kimi-coding` in the subscription
  bucket, `openrouter` outside it (the exchange mints a real API key on the user's own
  account), and `radius` in neither — exactly the "not a generic note" the criterion
  demands, and squarely the framing `docs/planning/SPECS.md:198-205` calls load-bearing.
  AC4 (`:53`) → issue 05. AC5 (`:54-55`) → the `// Ports:` criteria in 01–04 plus issue
  06's `docs/PORTING.md` rows (partial — see the P2 above). AC6 (`:56-58`) → every issue,
  matching `.github/workflows/test.yml:20, :26, :40` exactly.
- **Epic scope maps one-to-one and nothing is orphaned.** Four flows, four test files,
  `docs/auth.md`, CLI wiring — no scope bullet lacks an owner and no issue exists without
  a scope bullet behind it.
- **Invented scope is minimal and each instance is argued.** The provider-binding edits
  (`ai/providers/xai.go`, `kimi_coding.go`, `openrouter.go`) are not listed in
  `EPIC_7.md`'s Scope but are load-bearing for AC4 under issue 05's registry-derived
  design, and `EPIC_7.md:20-21` already asserts all four providers declare both methods
  upstream. The `README.md` edits in issue 06 (`:80-83`) are justified by
  `docs/planning/SPECS.md:204` ("`docs/auth.md` and the README **both** say so in those
  terms"). Both are correct calls.
- **Epic boundaries are honoured where it counts.** Issue 04 keeps the radius *provider
  binding*, `radius-config.ts` and the gateway catalog out (`:155-162`, matching
  `EPIC_7.md:41-43` and `docs/epics/epic-8-pi-messages-and-radius/EPIC_8.md:38`), and
  solves the `normalizeRadiusGatewayUrl` import-cycle problem in the only direction Go
  allows — verified: `ai/providers/anthropic.go:14` already imports `ai/auth/oauth`, so
  the reverse edge would indeed cycle.
- **Ordering is sound and the dependency graph is acyclic.** `depends_on` is `[]`, `[1]`,
  `[]`, `[1]`, `[1,2,3,4]`, `[1,2,3,4,5]` — no issue depends on a higher number, `postForm`
  lands in issue 01 before its two consumers, and each issue's prose "Blocks" list matches
  its dependents' "Blocked by" list. Cross-epic dependencies use prose rather than
  `depends_on`, which is correct: the field is scoped to "other issue numbers in this
  epic".
- **Every cross-epic claim was checked and holds.** Epic 6 issues 01, 04, 06, 08 and 11
  all exist and say what epic 7 attributes to them — the `AuthLoginCallbacks → AuthInteraction`
  rename and `IsSubscription`/`LoginLabel` (`epic-6/issues/01-...md:34-40, :80`), the
  five-minute refresh window and 15s cap (`04-...md:24-45`), `Models.Login` taking over
  persistence (`06-...md:79-82`), the always-racing manual-code shape (`08-...md:30-38`),
  and the `PORTING.md` row-43 path migration (`11-...md:24, :35`). `EPIC_6.md:43, :62` and
  `EPIC_8.md:38, :58` reciprocate epic 7's dependency claims exactly.
- **Repo citations are accurate.** Spot-checked and confirmed: `ai/auth/oauth/anthropic.go:36-39`
  (package-var pattern), `:44` (`anthropicCallbackHost`), `:53` (`parseAuthorizationInput`),
  `ai/auth/oauth/copilot.go:233` (`PollDeviceCodeFlow` call site), `:378` (the `Notify`
  shape), `ai/auth.go:52-58` / `:180-207` / `:264-270`, `ai/auth/oauth/callback.go:44` and
  `:54` (the `http://localhost:%d%s` `RedirectURI` issues 03 and 04 both flag),
  `ai/images/builtin.go:16-23`, `cmd/pi-ai/oauth.go:36-42` (the hardcoded three-entry list
  quoted verbatim), `cmd/pi-ai/main.go:43-50`, `docs/PORTING.md:43`, `docs/auth.md:32`,
  `:91`, `:101` ("Three providers support OAuth", the stale claim issue 06 greps for),
  `:117-163`, `README.md:27` and `:125`.
- **Project conventions are reflected, not assumed.** Stdlib-only offline tests, no
  `t.Parallel()`, no build tags, discrete named test functions, `got`/`want`
  (`CONVENTIONS.md:140-166`); the one table-driven exception in issue 04 (`:207-213`) is
  correctly justified as a pure input→output function (`CONVENTIONS.md:152-156`); the
  `kimicoding.go` filename rule (`issues/02-...md:108-110`) matches `CONVENTIONS.md:29-34`;
  the pointer-receiver rule for field-carrying error types (`issues/04-...md:82-84`)
  matches `CONVENTIONS.md:71-73`; `GOTMPDIR=$PWD/.gotmp` matches `CONVENTIONS.md:187`;
  Conventional Commit examples appear on every issue.
- **Bundle and schema hygiene.** All six files carry the full issue frontmatter with no
  extension fields, `status: open` agrees with the live GitHub state, `resource` is
  present only because the issues exist, `issues/index.md` lists all six with matching
  titles/sizes/statuses/numbers, and `docs/epics/log.md:68-73` records all six creations.
  Everything is in English.
- **Issue body quality is high throughout.** Every issue names verbatim error strings,
  exact constants, the seams tests must drive (`deviceCodeSleep`, `clock`, `httpClient`,
  `run(args, stdout, stderr)`), and — unusually — why each behaviour that looks like a bug
  is not one (`issues/03-...md:26-35` on the no-op `Refresh` and `9007199254740991`;
  `issues/01-...md:66-71` and `issues/02-...md:56-59` on the three different expiry
  margins). Issue 04's "Why this is sized L" section (`:91-101`) states the split it
  rejected and why, which is exactly the reasoning a later reviewer would otherwise redo.

## Open questions
- Issue 03 (`:124-127`) justifies touching `ai/images/builtin.go` with "Upstream's test
  asserts both providers expose OAuth and that a single stored credential resolves for
  both". That claim could not be checked here — no upstream checkout at `936aff00` is
  present in the tree. If upstream's `test/openrouter-oauth.test.ts` does not assert it,
  the image-provider edit is invented scope against `EPIC_7.md:26-38` and should be
  dropped or re-flagged as an assumption.
- `EPIC_7.md:53` requires the CLI to "complete" each flow, while issue 05's criteria
  deliberately stop at strategy selection (`:114-117`: "assert the selected entry, not a
  completed network login"). That is the right call for an offline test suite, but it
  leaves "completes them" unverified anywhere in the epic. Worth confirming the epic
  intends completion to be covered by issues 01–04's flow-level tests rather than by the CLI.
