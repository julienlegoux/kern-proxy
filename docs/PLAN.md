---
type: Plan
title: Post-review improvement plan
description: Improvement plan from the external adoption review — honor MaxRetries in all adapters, fix the Stream.Events goroutine leak, API polish, lint CI, credential-risk docs, and a v0.1.0 tag.
tags: [plan, retries, streaming, ci, release]
timestamp: "2026-07-09"
---

# Post-review improvement plan

## Context

An external "expert dev evaluating for adoption" review of kern-proxy raised a
set of findings; each was verified against the code before landing here. The
review's overall verdict was positive (clean build, race-clean behavioral
tests, good credential hygiene, strong upstream discipline), but it identified
one documented-but-dead option, one streaming sharp edge, and several
adoption-readiness gaps.

Goal of this plan: make kern-proxy honest about its documented API surface and
adoption-ready — retries that actually work, a leak-free streaming contract, a
linted CI, documented credential risk, and a tagged release.

This document is the input for the split-epics → create-issues →
implement-issue workflow. Epics below are ordered; 2.3 (tag) must land last.

## Epic 1 — Reliability & API fixes

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

**Acceptance.**

* Per-adapter `httptest` tests: 429/500-then-success succeeds within `MaxRetries`; exhausted retries surface the in-band error event.
* `Retry-After` honored; `MaxRetryDelay` clamps it.
* Quota/billing errors are not retried.
* Cancelling the context aborts a pending backoff sleep promptly.

### 1.2 Fix the `Stream.Events()` goroutine leak

**Problem.** `ai/stream.go:93-106`: the pump goroutine sends on an unbuffered
channel with no cancel path. A consumer that breaks out of the range early
strands the goroutine forever once the next event arrives.

**Approach.** Change the signature to `Events(ctx context.Context) <-chan Event`
with the pump selecting on `ctx.Done()`. Pre-v1 with no external consumers, a
clean break beats an `EventsCtx` variant. Record the deviation from the TS
`EventStream` contract in the [porting map](/PORTING.md). Update all in-repo
consumers (testbed, examples, tests).

**Acceptance.**

* Test that breaks out of the event loop early and asserts the pump goroutine exits (`goleak` or a done-signal).
* Existing full-drain consumers behave unchanged with `context.Background()`.

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
[usage guide](/usage.md) and a pointer from the README quick start.

## Epic 2 — Release & process hygiene

### 2.1 Add a linter to CI

No linter exists (no `.golangci.yml`, no lint step in
`.github/workflows/test.yml`). Add golangci-lint with a minimal config and a
lint job in `test.yml` (toolchain from `go.mod`, Go 1.25). Fix or explicitly
ignore whatever the first run flags — no blanket exclusions.

### 2.2 Document credential-mode risk

README documents OAuth login (`README.md`, Authentication section) with no
terms-of-service caveat. Add a short "credential modes" note to the README
Authentication section and [auth & credentials](/auth.md):

* API-key paths carry no ToS risk.
* Subscription-OAuth paths (Claude Pro/Max, ChatGPT Plus/Pro, GitHub Copilot) impersonate first-party clients — inherited upstream behavior, fine for personal use, account-revocation risk if shipped in a product.

### 2.3 Tag `v0.1.0`

No tags exist; `go get` currently pins a pseudo-version of `main`. Tag
`v0.1.0` with a minimal release note **after** the other items in this plan
land.

## Non-goals

Raised by the review, deliberately not planned — do not resurrect these when
splitting issues:

* **Unbounded event queue on the `Result`-only path** — documented contract mirroring the TS `EventStream` ("Push never blocks"); at most a godoc sentence noting the memory behavior.
* **Restructuring `StreamOptions`, `map[string]*string` headers, int64-millisecond timestamps** — deliberate port-fidelity decisions; changing them forks the API away from upstream and makes the weekly upstream-diff job noisier.
* **Maturity concerns (stars, bus factor, usage history)** — not addressable by code; mitigated over time and by 2.1/2.3.
