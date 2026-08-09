---
type: Conventions
title: "kern-link — Conventions"
description: "How kern-link's code is actually written — upstream-provenance headers, receiver rules, typed errors whose text is load-bearing, stdlib-only tests, and Conventional Commits onto a develop integration branch."
tags: [planning, conventions]
timestamp: 2026-08-08T23:07:30Z
status: final
mapped_commit: b70cba3293f998348d60080a05954bbb2eb27785
mapped_at: 2026-07-29T04:36:27Z
---

# kern-link — Conventions

Descriptive, read off the code at `b70cba3`. One rule shapes all the others:
kern-link is a **port** that tracks upstream over time, so "what does upstream
do" outranks "what is idiomatic Go" wherever the two conflict — and where the
port deviates, the deviation is written down rather than absorbed.

## Naming & file layout

**Package layout is by role, then by wire protocol.** `ai` is the domain core at
the module's second level (not the repo root — the root holds no Go files).
Adapters get one package each under `ai/apis/<protocol>`, provider bindings one
file each under `ai/providers`, and anything not part of the public contract goes
under an `internal/` package (`ai/internal/sse`, `ai/internal/partialjson`,
`ai/internal/jsonschema`, `ai/apis/internal/httpretry`) so it cannot be imported
by consumers.

**File names are lowercase, no separators** — `authcontext.go`,
`credentialstore.go`, `sessionresources.go`, `openaicompletions.go`. Underscores
appear only where a name would otherwise be unreadable, and then only in
`ai/providers` bindings, which are named after the provider ID with hyphens
turned into underscores: `amazon_bedrock.go` for `amazon-bedrock`,
`xiaomi_token_plan_ams.go` for `xiaomi-token-plan-ams`. One file per binding, no
grouping.

**Files split by concern within a package, not by type.** `ai/types.go` declares
the message model while `ai/json.go` holds every `MarshalJSON`/`UnmarshalJSON`
for it; adapters put wire structs in `wire.go`, streaming in `stream.go`, message
translation in `messages.go`, errors in `errors.go`. Test files sit beside their
subject with the `_test.go` suffix, and a focused concern gets its own test file
(`anthropic_error_mapping_test.go`, `messages_cache_test.go`,
`thinking_replay_test.go`) rather than being appended to the package's main one.

**Identifiers keep upstream's spelling.** `staticcheck`'s ST1003 is disabled
precisely so `Api`, `ProviderId`-style names, and `ai.ApiGoogleGenerativeAI`
survive unrenamed. Do not "fix" `Api` to `API` in ported identifiers; new code
that has no upstream counterpart may use Go initialisms.

**Every ported file carries a provenance header** — a `// Ports:` comment naming
the upstream source, placed immediately after the `package` clause (or after the
`// Command …` doc comment in `main` packages):

```go
package ai

// Ports: packages/ai/src/models.ts (Provider, Models, createModels,
// createProvider)
```

157 files carry one. The header names specific upstream symbols when the mapping
is partial, and larger ones explain what was ported when and why (see
`ai/providers/all.go`). **New code with no upstream counterpart carries no
header** — the only two non-test files without one,
`ai/apis/internal/httpretry/httpretry.go` and `ai/doc.go`, are both new. Absence
of the header is therefore meaningful: it marks original code.

**Receivers split by kind, deliberately.** Messages use **pointer** receivers,
unified across all three implementations in `564a3fe` as an explicit breaking
change and locked in by `ai/message_receivers_test.go`. Events use **value**
receivers. Small self-describing error types use value receivers when they are
string-backed (`func (e streamError) Error() string`, `bedrockError`) and pointer
receivers when they carry fields (`*ModelsError`, `*codexAPIError`).

**Doc comments are contracts, not descriptions.** Interface methods in
`ai/provider.go` specify failure behavior in prose that the implementations
honor — "Must not fail", "Concurrent calls share one in-flight fetch", "On
failure the model list stays at its last-known state and a later call retries",
"Returns (nil, nil) when the provider is unknown or unconfigured". Write new
interface methods to that bar. `ai/doc.go` is the package overview, using
`[Symbol]` doc links and `#` subheadings for pkg.go.dev.

## Code style, lint & format

`gofmt` via the `formatters` block in `.golangci.yml`; no `goimports`, no
`gofumpt`. Tabs, LF line endings enforced for `.go`/`.md`/`.yml`/`.sh` through
`.gitattributes`.

**Linting is deliberately minimal and deliberately loud.** `.golangci.yml` uses
`default: standard` — errcheck, govet, ineffassign, staticcheck, unused — and
nothing more. The config's own comment records that enabling an opinionated
battery (revive, gocognit, exhaustive) is acknowledged future work and that "the
first pass is about having a linter at all". **No linter is disabled wholesale
and no path is excluded.** `max-issues-per-linter: 0` and `max-same-issues: 0`
override golangci's defaults, which hid repeats and made a green run "a statement
about the first three occurrences rather than about the codebase".

Three narrowings exist, each scoped to one check or one function, and **each
carries its reason inline**. Follow that pattern: a new exclusion needs a written
justification next to it.

- `errcheck` excludes `(io.Closer).Close` (23 `defer resp.Body.Close()` sites
  with no possible recovery) and `fmt.Fprint`/`Fprintf`/`Fprintln` (CLI
  diagnostics — if the write fails, the terminal is gone).
- `staticcheck` spells out its check list as `all` minus ST1000, ST1003, ST1016,
  ST1020, ST1021, ST1022, ST1005 — spelled out because listing `checks` replaces
  the default set rather than extending it.
- **ST1005 is the important one.** Error strings stay sentence-cased and
  upstream-verbatim ("Request was aborted", "No API key for provider: %s")
  because `ai/retry.go` and `ai/overflow.go` classify failures by matching that
  exact text and tests assert it. Rewriting error text is a behavior change.

**Error handling.** Wrap with `fmt.Errorf` and `%w` (68 wrap sites) so `errors.Is`
and `errors.As` work through the chain; `errors.New` for sentinels. The domain
error is `*ModelsError` — a `ModelsErrorCode` (`model_source`,
`model_validation`, `provider`, `stream`, `auth`, `oauth`), a message, and a
cause — constructed with `NewModelsError`, with `Error()` rendering
`"message: cause"`, `Unwrap()` returning the cause, and a `Code() string` method
that `ExtractDiagnosticError` reflects on to populate diagnostics. Adapter
packages define their own small unexported error types rather than reaching for
the domain type.

**Failures never escape as panics, and never escape silently.** Stream setup runs
inside `ai.LazyStream` so configuration errors arrive as in-band error events;
provider failures end the stream with `StopReason` `error`/`aborted`. Non-fatal
problems attach to the message as `AssistantMessageDiagnostic` rather than being
logged — **the library imports no logger at all**, and new code must not
introduce one. `errors.Join` appears once, for multi-resource cleanup in
`ai/sessionresources.go`.

**Comments explain decisions, not mechanics.** The codebase's distinctive habit
is a multi-line comment above a tricky declaration explaining why it is shaped
that way and what breaks otherwise — `retry.go`'s warning that status-code
patterns are text patterns valid only against strings known to *start* with a
status code (because a 400 rejecting `max_tokens: 15000` contains "500"),
`google_live_smoke_test.go`'s note on why no upstream test was ported verbatim,
`test.yml`'s note on why `-v` is set. Match that density where the reasoning is
non-obvious; leave obvious code uncommented.

## Testing

**Standard library only.** `testing` plus `net/http/httptest`; `testify` appears
zero times and no assertion or mocking library is used. Assert by hand and report
with `t.Fatalf`/`t.Errorf` (1822 sites) in `got`/`want` phrasing:
`t.Errorf("auth.json mode = %o, want 600", perm)`.

**Discrete named test functions are the dominant style**, not table-driven tests.
102 test files contain only 28 `t.Run` calls between them. Each behavior gets its
own top-level `func TestSomeSpecificBehavior(t *testing.T)` with a name that
states the claim — `TestEstimateSkipsFailedAssistantTurns`,
`TestResolveExpiredOAuthRefreshesOnceUnderLock`,
`TestLiveSmoke_GoogleGeminiStreamsPlainText`. Table-driven tests are reserved for
matrix-shaped input→output checks over pure functions, and that is where all 19
of them are: classifiers and parsers (`retry_test.go`, `overflow_test.go`,
`json_test.go`, `jsonschema_test.go`, `partialjson_test.go`, `compat_test.go`,
`params_test.go`, `baseurl_test.go`). Behavioral and wire tests use discrete
functions.

**No `t.Parallel()` anywhere** and no `//go:build` tags anywhere. Tests are
co-located with their subject.

**Everything runs offline.** Three tools make that true: the `faux` in-process
provider for the full event/tool/abort/prompt-cache contract, `httptest` servers
for vendor endpoints (32 files), and injectable seams for time and I/O —
`ai/resolve.go` exposes `var authClock = func() int64 { … }` for tests to
override, and `cmd/pi-ai`'s `run(args, stdout, stderr) int` takes its writers as
parameters so the CLI is testable without a process.

**Live tests gate at runtime with `t.Skip`**, never with a build tag. Read the
env var, skip with a message naming it:

```go
apiKey := os.Getenv("GEMINI_API_KEY")
if apiKey == "" {
    t.Skip("GEMINI_API_KEY not set; skipping live Gemini smoke test")
}
```

Name them `TestLiveSmoke_*` where they are dedicated smoke tests. CI sets no
provider secrets, so they always skip there.

**Test commands** ([decision](/mapping/02-canonical-test-command.md)):

| | Command | Notes |
|---|---|---|
| **CI gate** | `go test ./... -race -v` | Authoritative — must be green before merge. `-v` is intentional so each test is visible by name in the run log. Followed by `bash upstream/sync_test.sh`. |
| **Local** | `go test ./...` with `GOTMPDIR` set to the repo's `.gotmp/` | `-race` is CI-only: it needs a C toolchain that is not installed locally. `GOTMPDIR` must stay inside the repo because Application Control blocks test binaries built into the system temp directory. |

No coverage threshold or reporting is configured.

## Commits & branches

**Conventional Commits with scopes**, consistently, with `!` marking breaking
changes:

```
feat(apis): honor MaxRetries in the google, vertex, and mistral adapters
fix(ai): make Stream.Events cancellable to stop leaking its pump goroutine
refactor(ai)!: unify Message implementations on pointer receivers
ci: run golangci-lint, and fix everything its first pass flagged
docs: document the terms-of-service risk of each credential mode
chore(release): prepare v0.1.1 changelog
```

Types in use: `feat`, `fix`, `refactor`, `docs`, `ci`, `chore`. Scopes name the
package or area (`ai`, `apis`, `bedrock`, `epics`, `release`). Subjects are
lowercase after the colon, imperative, and describe the *effect* rather than the
edit — `"make Stream.Events cancellable to stop leaking its pump goroutine"`, not
`"add ctx param"`. A handful of older commits use a bare imperative subject with
no type (`11fd3c7 Remove Go Report Card badge from README`); the prefixed form is
current practice.

**Two-level branch flow, merge commits throughout.** Feature branches merge by PR
into `develop`; `develop` merges by PR into `main`. History is full of
`Merge pull request #N from julienlegoux/<branch>` — **no rebasing or squashing
onto the integration branches**. Feature branches are named
`issue-<NN>-<slug>` when they implement a tracked issue
(`issue-92-stream-events-context`, `issue-95-golangci-lint-ci`) and a bare
kebab-case topic otherwise (`repo-polish`, `docs-cleanup`, `rename-kern-link`).

**Issue status is tracked in commits.** When a planning bundle is present, each
transition gets its own `docs(epics): …` commit recording the state change and
the PR number — `docs(epics): mark issue 05 done (PR #102 merged)`. That commit
is separate from the implementation commit.

## Review

No `CODEOWNERS`, no PR template, no `CONTRIBUTING.md`, and no branch-protection
configuration visible in the repository — this is a single-maintainer project.
What the process does enforce, from `.github/workflows/test.yml` on every push and
pull request:

1. `go test ./... -race -v` green.
2. `bash upstream/sync_test.sh` green — the offline test of `sync.sh`'s
   diff-detection logic.
3. `golangci-lint` (pinned `v2.12.2`) green.

Work still flows through pull requests rather than direct pushes, and PRs are
merged with a merge commit. `CHANGELOG.md` is updated as part of release
preparation, not per-PR.

**Unknowns:** whether the three CI jobs are configured as *required* checks
(not visible in-repo), and whether review approval is required before merge.
