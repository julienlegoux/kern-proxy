---
type: Issue
title: "Prove the pending and deferred stop reasons end to end through the faux provider"
description: "Port faux's deferred scripting, pending partial snapshots, and deferred fetch/cancel counters, so the new stop reasons are emitted and observed by a test rather than merely declared."
tags: [epic-2]
timestamp: 2026-08-11T18:15:00Z
epic: 2
issue: 12
slug: faux-deferred-responses
size: M
status: open
gh_issue: 140
resource: https://github.com/kern-ia/kern-link/issues/140
depends_on: [1, 2, 10]
---

# Prove the pending and deferred stop reasons end to end through the faux provider

## Summary

This issue is [Epic 2](/epic-2-core-types-and-models-contracts/EPIC_2.md)'s
acceptance criterion 2 in code form: *the new stop reasons are actually emitted
and observed by a test, not merely declared*.
[Decision 11](../../../planning/scope/11-deferred-tools.md) names `ai/providers/faux`
as the proving ground because it is "the executable specification of the event
contract", and upstream extended its own `faux.ts` in exactly this direction at
`936aff00`:

- `fauxResponse(...)` takes a `deferred?: DeferredHandle` and the scripted
  message carries it;
- streaming partial snapshots now start at `stopReason: "pending"` instead of
  inheriting the final reason, and a scripted response still marked `"pending"`
  when the stream ends is a programming error
  (`throw new Error("Faux response ended without a stop reason")`);
- `FauxProviderState` grows from `{ callCount }` to
  `{ callCount, deferredFetchCount, cancelledDeferred: DeferredHandle[] }`;
- provider options gain
  `deferred?: { pendingFetches?: number; pollAfterMs?: number }` — the number of
  fetches that return the original handle before the scripted response is ready.

## Scope

- `ai/providers/faux/faux.go` — port the above:
  - a `Deferred *ai.DeferredHandle` field on the scripted-response builder;
  - a `createDeferredMessage` equivalent producing an `AssistantMessage` with
    empty content, `StopReasonDeferred`, and the handle;
  - partial snapshots emitted during streaming carry `StopReasonPending`;
  - the "ended without a stop reason" guard, with upstream's message verbatim;
  - `FauxProviderState` gaining `DeferredFetchCount` and `CancelledDeferred`;
  - provider options gaining the `deferred` block (`PendingFetches`,
    `PollAfterMs`);
  - `FetchDeferred` / `CancelDeferred` implementations satisfying the capability
    interface from
    [issue 10](/epic-2-core-types-and-models-contracts/issues/10-deferred-response-dispatch.md):
    the first `PendingFetches` fetches return the same handle again, the next
    returns the scripted response; cancel records the handle.
- Tests in `ai/providers/faux/faux_test.go` (and/or `ai/` where the assertion is
  about the `Models` layer) proving the round trip:
  a `StreamSimple` that ends `deferred` with a handle → `Models.FetchDeferred`
  returning pending once → a second `FetchDeferred` returning the completed
  message → `Models.CancelDeferred` recording the handle.
- Upstream also changed `FauxResponseFactory`'s options parameter from
  `StreamOptions` to `SimpleStreamOptions`; carry that if the Go signature has
  the same shape.

## Out of scope

- Real adapter implementations of deferred responses — epics 4 and 5.
- Extending the `testbed/` web UI. It is gitignored and outside the module
  ([SPECS.md](../../../planning/SPECS.md), "Interfaces & integrations").
- `splitDeferredTools` — [issue 11](/epic-2-core-types-and-models-contracts/issues/11-deferred-tools-split.md).

## Acceptance criteria / Definition of done

- [ ] `TestFauxEmitsDeferredStopReasonWithHandle` — a scripted deferred response
      ends the stream with `StopReason` `deferred` and a non-nil
      `AssistantMessage.Deferred` whose `Provider`/`ModelId`/`Api` match the
      model.
- [ ] `TestFauxPartialSnapshotsCarryPendingStopReason` — every non-terminal
      event's `Partial` has `StopReasonPending`, and the terminal message does
      not.
- [ ] `TestFauxFetchDeferredReturnsHandleUntilReady` — with
      `PendingFetches: 1`, the first `Models.FetchDeferred` resolves to a
      message still marked `deferred` carrying the same handle, and the second
      resolves to the scripted completion with a terminal stop reason.
- [ ] `TestFauxCancelDeferredRecordsHandle` — `Models.CancelDeferred` appends to
      `state.CancelledDeferred`.
- [ ] `TestFauxResponseWithoutStopReasonFails` — a scripted response left
      `pending` surfaces the verbatim error `Faux response ended without a stop
      reason` in band (as an error event), not as a panic.
- [ ] `TestFauxDeferredFetchCountIncrements` — `state.DeferredFetchCount`
      tracks fetches.
- [ ] The epic's acceptance criterion 2 is quotable from this PR: the PR body
      names the tests that emit **and** observe `pending` and `deferred`.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2).
- [ ] `gofmt -l .` prints nothing; `ai/providers/faux/faux.go` keeps its
      `// Ports:` header; tests stay stdlib-only with no `t.Parallel()`.
- [ ] Conventional Commit, e.g.
      `feat(faux): script deferred responses and pending partials`.

## Relevant files / areas

- `ai/providers/faux/faux.go` and `ai/providers/faux/faux_test.go` — the only
  two files in the package.
- `ai/events.go:124-137` — `DoneEvent` / `ErrorEvent`; the partial-snapshot
  contract lives in the `Text*`/`Thinking*`/`ToolCall*` triples above it.
- `ai/types.go` — `DeferredHandle`, `StopReasonPending`, `StopReasonDeferred`.
- `ai/provider.go` — `Models.FetchDeferred` / `CancelDeferred` from issue 10.
- `ai/example_stream_test.go`, `ai/example_session_test.go` — package examples
  run against `faux`; keep them green.
- Upstream:
  ```bash
  gh api "repos/earendil-works/pi/contents/packages/ai/src/providers/faux.ts?ref=936aff00918de1187f085f123c2812d8f2d67745" -H "Accept: application/vnd.github.raw"
  ```

## Dependencies

- **Blocked by**: [Issue 01](/epic-2-core-types-and-models-contracts/issues/01-widen-stopreason-and-thinkinglevel.md),
  [Issue 02](/epic-2-core-types-and-models-contracts/issues/02-message-model-deferred-fields.md),
  [Issue 10](/epic-2-core-types-and-models-contracts/issues/10-deferred-response-dispatch.md).
- **Blocks**: Nothing — it is the epic's last issue, and the evidence for
  acceptance criterion 2.

## PR size note

`M` — ~350 changed lines, confined to the two-file `ai/providers/faux` package:
the scripted-response builder gains a handle, partial snapshots switch to
`StopReasonPending`, `FauxProviderState` grows two fields, the provider options
gain a `deferred` block, and `FetchDeferred`/`CancelDeferred` arrive alongside
six named tests proving the round trip. Split past ~500; the seam is the faux
port itself versus the `Models`-layer round-trip assertions, which can follow.
