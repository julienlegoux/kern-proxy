---
type: Epic
title: "Reliability & API fixes"
description: "Make the documented API surface real: honor MaxRetries/MaxRetryDelay in every adapter, fix the Stream.Events goroutine leak, unify message receivers, and document session round-tripping."
tags: [epic]
timestamp: 2026-07-09T01:16:00Z
resource: https://github.com/julienlegoux/kern-proxy/issues/85
epic: 1
slug: reliability-api-fixes
status: open
gh_issue: 85
milestone: 16
source: docs/PLAN.md#epic-1--reliability--api-fixes
---

# Epic 1: Reliability & API fixes

## Goal

An external "expert dev evaluating for adoption" review found that kern-proxy's
documented API surface is partly dead: retry options are accepted but ignored by
almost every adapter, and the streaming contract has a goroutine-leak sharp edge.
This epic makes kern-proxy honest about what it documents — retries that actually
work, a leak-free streaming contract, and consistent, discoverable API surface.

## Scope

### 1.1 Honor `MaxRetries`/`MaxRetryDelay` in all adapters

**Problem.** `StreamOptions.MaxRetries`/`MaxRetryDelay` (`ai/options.go:104`)
are documented but only the Codex adapter reads them
(`doRequestWithRetry`, `ai/apis/codex/codex.go:276-356`). Every other adapter
accepts the options and silently makes exactly one attempt. `ai/retry.go` is
classifiers only (`IsRetryableAssistantError`) — the loop is missing. Upstream
got retries for free from vendor SDKs; the Go port speaks raw `net/http`.

**Approach.** Generalize Codex's HTTP-request-level retry loop into a shared
helper (new `ai/apis/internal/httpretry` package, or the shared `ai/apis` root
alongside `simpleopts.go`/`transform.go`). Retry the POST until 2xx *before*
streaming begins — this matches upstream/vendor-SDK semantics and avoids the
partial-emission problem: a wrapper around the dispatched `*ai.Stream`
(`ai/provider.go:449-462`) cannot transparently retry once events have flowed
to the consumer.

The helper carries over Codex's behaviors:

* exponential backoff (`exponentialDelay`, base 1s, `var` so tests can shrink it)
* `Retry-After` / `retry-after-ms` honoring, clamped via `opts.EffectiveMaxRetryDelay()`
* non-retryable quota/billing short-circuit
* context cancellation aborts the backoff sleep

Unify the codex-local regex patterns with the `ai/retry.go` patterns where
feasible instead of keeping two copies.

**Adoption.** Each raw-HTTP adapter (anthropic, openaicompletions,
openairesponses, google, mistral, azure, …) switches its request path to the
helper. Exceptions:

* **bedrock** uses the AWS SDK — map `MaxRetries` onto the SDK retryer config instead.
* **codex** keeps its deliberate default of 0 (`ai/apis/codex/codex.go:49`) but migrates to the shared helper.

Pick and document a default for the rest — upstream-equivalent is 2, per the
`ai/options.go` doc comment.

### 1.2 Fix the `Stream.Events()` goroutine leak

**Problem.** `ai/stream.go:93-106`: the pump goroutine sends on an unbuffered
channel with no cancel path. A consumer that breaks out of the range early
strands the goroutine forever once the next event arrives.

**Approach.** Change the signature to `Events(ctx context.Context) <-chan Event`
with the pump selecting on `ctx.Done()`. Pre-v1 with no external consumers, a
clean break beats an `EventsCtx` variant. Record the deviation from the TS
`EventStream` contract in the porting map (`docs/PORTING.md`). Update all
in-repo consumers (testbed, examples, tests).

### 1.3 Message receiver consistency

**Problem.** `ai/types.go:195-244`: `UserMessage` and `ToolResultMessage`
implement `Message` by value while `AssistantMessage` uses a pointer receiver.

**Approach.** Unify on pointer receivers for all three (`AssistantMessage` is
mutated during streaming and its `MarshalJSON` is already on the pointer).
Pre-v1 breaking change, mechanical; update construction sites and tests.

### 1.4 Session round-trip documentation

**Problem.** Serializing and restoring a session works (`ai.Messages`,
`Context.UnmarshalJSON`, `ai/json.go:280-309`) but is undiscoverable — an
external consumer had to read the source to find `ai.Messages`.

**Approach.** Docs-only: add a serialize/deserialize-a-session example to the
usage guide (`docs/usage.md`) and a pointer from the README quick start.

## Out of scope

The plan does not state Epic-1-specific exclusions; see Notes for the
project-wide non-goals that bound this epic.

## Acceptance criteria

For 1.1 (retries):

- Per-adapter `httptest` tests: 429/500-then-success succeeds within `MaxRetries`; exhausted retries surface the in-band error event.
- `Retry-After` honored; `MaxRetryDelay` clamps it.
- Quota/billing errors are not retried.
- Cancelling the context aborts a pending backoff sleep promptly.

For 1.2 (stream leak):

- Test that breaks out of the event loop early and asserts the pump goroutine exits (`goleak` or a done-signal).
- Existing full-drain consumers behave unchanged with `context.Background()`.

For 1.3 and 1.4 the plan states approaches but no explicit criteria; done means
all three message types use pointer receivers with construction sites and tests
updated (1.3), and the session round-trip example exists in `docs/usage.md`
with a README quick-start pointer (1.4).

## Dependencies

None — this epic goes first. [Epic 2](/epic-2-release-process-hygiene/EPIC_2.md)
must land after it (its `v0.1.0` tag depends on this epic's fixes).

## Notes

Project-wide non-goals from the plan (not specific to this epic — do not
resurrect these when splitting issues):

* **Unbounded event queue on the `Result`-only path** — documented contract mirroring the TS `EventStream` ("Push never blocks"); at most a godoc sentence noting the memory behavior. Directly adjacent to 1.2's stream work — do not "fix" it while touching `ai/stream.go`.
* **Restructuring `StreamOptions`, `map[string]*string` headers, int64-millisecond timestamps** — deliberate port-fidelity decisions; changing them forks the API away from upstream and makes the weekly upstream-diff job noisier. Relevant to 1.1/1.3, which touch `StreamOptions` consumers and `ai/types.go`.
* **Maturity concerns (stars, bus factor, usage history)** — not addressable by code.

Context from the review: overall verdict was positive (clean build, race-clean
behavioral tests, good credential hygiene, strong upstream discipline); this
epic addresses the one documented-but-dead option and the one streaming sharp
edge it identified.
