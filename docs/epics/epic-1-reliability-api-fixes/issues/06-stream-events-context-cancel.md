---
type: Issue
title: "Fix the Stream.Events() goroutine leak with a context-taking signature"
description: "Change Events() to Events(ctx context.Context) so the pump goroutine exits when the consumer stops early; update all in-repo consumers and record the TS-contract deviation in PORTING.md."
tags: [epic-1]
timestamp: 2026-07-09T13:00:00Z
resource: https://github.com/julienlegoux/kern-proxy/issues/92
epic: 1
issue: 6
slug: stream-events-context-cancel
size: M
status: done
gh_issue: 92
gh_pr: 103
depends_on: []
---

# Fix the Stream.Events() goroutine leak with a context-taking signature

## Summary

`ai/stream.go:93-106`: the `Events()` pump goroutine sends on an unbuffered
channel with no cancel path. A consumer that breaks out of the range early
strands the goroutine forever once the next event arrives. Pre-v1 with no
external consumers, a clean signature break beats an `EventsCtx` variant.

## Scope

- Change `func (s *Stream) Events() <-chan Event` to `Events(ctx context.Context) <-chan Event`; the pump selects on `ctx.Done()` around the channel send (and unblocks from `s.cond.Wait()` — mind the sync.Cond interaction when cancelling).
- Update all in-repo consumers (~12 call sites: `ai/stream_test.go`, `ai/lazy.go`, `ai/apis/bedrock/bedrock.go`, adapter tests, `ai/providers/faux/faux_test.go`, testbed if applicable) to pass a context — `context.Background()` where full-drain behavior is intended.
- Update the code examples in `README.md` and `docs/usage.md` that show `.Events()`.
- Record the deviation from the TS `EventStream` contract in `docs/PORTING.md`.

## Out of scope

- The unbounded event queue on the `Result`-only path — documented contract mirroring the TS `EventStream` ("Push never blocks"); at most a godoc sentence noting the memory behavior. Do not "fix" it while in this file.
- Retry work (issues 01–05) — independent.

## Acceptance criteria / Definition of done

- A test breaks out of the event loop early and asserts the pump goroutine exits (`goleak` or a done-signal).
- Existing full-drain consumers behave unchanged with `context.Background()`.
- `docs/PORTING.md` notes the signature deviation from upstream's `EventStream`.
- `go test -race ./...` clean.

## Relevant files / areas

- `ai/stream.go` (`Events`, `next`, the pump goroutine), `ai/stream_test.go`
- `ai/lazy.go`, `ai/apis/bedrock/bedrock.go` (in-repo consumers)
- `README.md`, `docs/usage.md`, `docs/PORTING.md`

## Dependencies

None within this epic (touches different files than the retry work). Part of
[Epic 1](/epic-1-reliability-api-fixes/EPIC_1.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.
