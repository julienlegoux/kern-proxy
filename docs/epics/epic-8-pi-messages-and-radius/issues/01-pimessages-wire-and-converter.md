---
type: Issue
title: "Open the pi-messages package with its wire event union and the converter onto ai.Event"
description: "Add ai.ApiPiMessages and the ai/apis/pimessages package's decode half: the serialized pi-messages event union, its SSE frame decoding, and the converter that assembles an ai.AssistantMessage partial and emits unified events."
tags: [epic-8]
timestamp: 2026-08-09T14:45:00Z
epic: 8
issue: 01
slug: pimessages-wire-and-converter
size: M
status: open
gh_issue: 187
resource: https://github.com/kern-ia/kern-link/issues/187
depends_on: []
---

# Open the pi-messages package with its wire event union and the converter onto ai.Event

## Summary

pi-messages is the tenth wire protocol, and the only one that is *pi's own*:
the backend sends serialized assistant-message events, so the adapter's job is
decode-and-relabel rather than translation. That is what makes it the cheapest
adapter in the set ([decision 13](../../../planning/scope/13-pi-messages-adapter.md)),
and it is why this issue takes the decode half on its own — it is pure,
offline-testable, and needs no HTTP at all.

This PR lands: the `ai.ApiPiMessages` constant, the package, the wire event
union with its SSE frame decoding, and the converter that turns each wire event
into an `ai.Event` while maintaining the `*ai.AssistantMessage` partial. Issue
02 puts a request and a response body in front of it.

**The correspondence is close but not exact, and the gaps are the work.**
Upstream's `PiMessagesEvent` union is *nearly* kern-link's `ai.Event` union, so
the temptation is to decode straight into `ai.Event`. Four places where that
breaks:

1. **`toolcall_start` carries `id` and `toolName` on the wire**; kern-link's
   `ai.ToolCallStartEvent` (`ai/events.go:99-105`) carries only `ContentIndex`
   and `Partial`. The id and name go into the `ai.ToolCall` block on the
   partial, not onto the event.
2. **`text_end`/`thinking_end` carry `contentSignature`**; kern-link's
   `TextEndEvent`/`ThinkingEndEvent` do not. It belongs on the content block —
   `TextContent.TextSignature` (`ai/types.go:75-78`),
   `ThinkingContent.ThinkingSignature` plus `Redacted` (`:94-98`).
3. **`done`/`error` carry `usage`, `responseId` and `rewrite`**, which land on
   the terminal `*ai.AssistantMessage`, not on the event struct
   (`DoneEvent`/`ErrorEvent` carry only `Reason` + the message).
4. **`contentIndex` is an array index upstream assigns sparsely.** Go slices
   do not grow by assignment. Grow `partial.Content` to `contentIndex+1`,
   filling any gap with a zero `ai.TextContent`, and comment why — a backend
   that skips an index must not panic the consumer's process.

Two more shapes to get right:

- **Partial snapshots are clones, one per event.** Every adapter in the repo
  pushes `Partial: output.Clone()` (`ai/apis/anthropic/anthropic.go:275`,
  `:421`, `:477`); the doc comment on `ai.AssistantMessage.Clone`
  (`ai/types.go:211-215`) says why. Upstream hands out the same mutable object
  every time and gets away with it because JS is single-threaded; Go does not.
- **Tool arguments stream through `partialjson.ParsePartial`**
  (`ai/internal/partialjson/partial.go:24`), accumulating raw JSON per content
  index exactly like upstream's `toolJson` map, so a consumer reading
  `arguments` mid-stream sees a growing object rather than nothing. Replace the
  `Arguments` map rather than mutating it — `Clone`'s doc comment requires it.

The wire `usage` object is the unified `ai.Usage` already
(`ai/types.go:139-148`, `cost` included), so it decodes directly with
`encoding/json`; do not hand-map its fields.

## Scope

- `ai/types.go` — add `ApiPiMessages Api = "pi-messages"` to the known-protocol
  const block (`:10-19`), ported from upstream `src/types.ts:27`.
- `ai/catalog/compat.go` — **no change, deliberately.** Upstream's
  `Model<"pi-messages">` has no compat interface, so a pi-messages catalog
  entry must leave `compat` unset; `validateCompatForApi` (`:52`) already
  rejects a non-nil compat for an api missing from `compatFieldsByApi`. Prove
  it with a test rather than adding a map entry.
- `ai/apis/pimessages/pimessages.go` (new) — package doc comment in the style
  of `ai/apis/anthropic/anthropic.go:1-8`, followed by
  `// Ports: packages/ai/src/api/pi-messages.ts`, plus the package constants
  (`messagesPath = "/messages"`, the `[DONE]` sentinel).
- `ai/apis/pimessages/wire.go` (new) — the serialized event union as one
  decode struct discriminated on `type`, with `rewrite` as
  `piRewriteImpact{PolicyID, PolicyVersion, Changed, TokenCountChange,
  MessageCountChange, SystemPromptChanged}`, `usage` as `ai.Usage`, and the
  `toolCall` payload of `toolcall_end` as `ai.ToolCall`. Plus
  `decodeEvent(sse.Event) (piEvent, bool, error)`: skips frames with no data
  and the `[DONE]` sentinel (the `bool`), and reports malformed JSON as an
  error.
  - Read frames with `ai/internal/sse`'s `Reader` (`NewReader`/`Next`) — it
    already does CR/LF/CRLF normalization and multi-line `data:` joining, which
    is upstream's hand-rolled `\n\n` splitter's whole job. Do **not** port a
    second frame splitter into this package.
- `ai/apis/pimessages/events.go` (new) — the converter: a struct holding the
  `*ai.AssistantMessage` partial and the per-index raw tool JSON, with a
  `convert(piEvent) (ai.Event, error)` method. Initial partial: `Api` from
  `model.Api`, `Provider`, `Model` from the model, empty `ai.Usage`,
  `StopReason: ai.StopReasonPending`, `Timestamp: time.Now().UnixMilli()`.
- `ai/apis/pimessages/wire_test.go`, `events_test.go` (new) — the criteria
  below. No HTTP: feed frames from a `strings.Reader`, feed events to the
  converter directly.
- `ai/catalog/compat_test.go` — one case for the pi-messages rule above.

## Out of scope

- **Everything with a socket in it** — request building, headers, the API-key
  check, response error mapping, the read loop, `Stream`/`StreamSimple`:
  [issue 02](/epic-8-pi-messages-and-radius/issues/02-pimessages-stream-entry.md).
  This PR exports no `ai.StreamFunc`.
- `apis.TransformMessages`. pi-messages sends `context` verbatim — the backend
  speaks pi's model, so there is nothing to normalize. Adding a transform here
  would be a behavior change no upstream line asks for.
- Any catalog data file for `radius` or another pi-messages provider. Radius
  is purely dynamic and has no static catalog entry (upstream
  `providers/all.ts:50-52` says so explicitly).
- `docs/PORTING.md` rows —
  [issue 05](/epic-8-pi-messages-and-radius/issues/05-porting-and-docs.md).
  This PR writes the `// Ports:` headers on the files it creates; the mapping
  table is one file four PRs would otherwise fight over.

## Acceptance criteria / Definition of done

- [ ] `TestConverterStartsThePartialAsPending` — before any event, the partial
      has `StopReason == ai.StopReasonPending`, `Api == ai.ApiPiMessages`, and
      the model's provider and id; the `start` event's `Partial` shows the
      same.
- [ ] `TestConverterAssemblesTextThinkingAndToolCalls` — the upstream test's
      event sequence (`text_start`, two `text_delta`, `text_end`,
      `toolcall_start`, two `toolcall_delta` splitting `{"path":` /
      `"a.txt"}`, `toolcall_end`) yields final content
      `[TextContent{Text: "Hello"}, ToolCall{ID: "call_1", Name: "read",
      Arguments: {"path": "a.txt"}}]`.
- [ ] `TestConverterCarriesContentSignatures` — `text_end` with
      `contentSignature` sets `TextContent.TextSignature`; `thinking_end` with
      `contentSignature` and `redacted: true` sets `ThinkingSignature` and
      `Redacted`.
- [ ] `TestConverterParsesToolArgumentsWhileStreaming` — after the first
      `toolcall_delta` carrying only `{"path":`, the partial's `ToolCall.Arguments`
      is already a non-nil map (`partialjson.ParsePartial`'s output), not nil
      and not an error.
- [ ] `TestConverterSnapshotsThePartialPerEvent` — the `Partial` on an early
      event still shows the content as it was when that event was emitted after
      later events have mutated the live partial (i.e. it is a `Clone`).
- [ ] `TestConverterGrowsContentForSparseIndexes` — a `text_start` at
      `contentIndex: 2` with nothing at 0 or 1 does not panic and leaves two
      zero-value text blocks ahead of it.
- [ ] `TestConverterDoneCarriesUsageAndResponseID` — a `done` with
      `reason: "toolUse"`, a full `usage` (including `cost`) and
      `responseId: "resp_1"` yields an `ai.DoneEvent` whose `Message` carries
      all three, and `Usage` decodes field-for-field without hand-mapping.
- [ ] `TestConverterErrorEventCarriesErrorMessage` — an `error` event with
      `reason: "error"` and `errorMessage: "Upstream failed"` yields an
      `ai.ErrorEvent` whose message has that `ErrorMessage`, that `StopReason`,
      and the event's `usage`.
- [ ] `TestConverterAppendsTheRewriteDiagnostic` — a terminal event carrying
      `rewrite` appends exactly one `ai.AssistantMessageDiagnostic` of type
      `pi_messages_rewrite` whose `Details` carries all six rewrite fields; a
      terminal event without `rewrite` appends none.
- [ ] `TestDecodeEventSkipsTheDoneSentinelAndDatalessFrames` — a `data: [DONE]`
      frame and a comment-only frame both decode to "no event, no error";
      `data: {` reports an error.
- [ ] `TestDecodeEventHandlesCRLFFrames` — a stream using `\r\n` line endings
      decodes the same events as one using `\n` (the property
      `ai/internal/sse` is there to provide).
- [ ] `TestPiMessagesModelsAcceptNoCompatBlock` in `ai/catalog` — a
      `*ai.Compat` on an `ai.ApiPiMessages` model is rejected by
      `validateCompatForApi`, and a nil one is accepted.
- [ ] Tests are offline, stdlib-only (`testing`, `strings`), no `t.Parallel()`,
      no build tags, discrete named functions
      (`docs/planning/CONVENTIONS.md`, "Testing").
- [ ] `// Ports: packages/ai/src/api/pi-messages.ts` after the `package` clause
      on every new non-test file.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(apis): decode the pi-messages event protocol`.

## Relevant files / areas

- `ai/types.go:10-19` — the `Api` const block this extends; `:75-98`
  `TextContent`/`ThinkingContent`; `:114-119` `ToolCall`; `:139-148` `Usage`;
  `:195-207` `AssistantMessage`; `:211` `Clone` and its contract.
- `ai/events.go:13-25` the event-type constants, `:40-140` the twelve event
  structs (note `ToolCallStartEvent` has no id/name field, `TextEndEvent` no
  signature field).
- `ai/diagnostics.go:19-25` `AssistantMessageDiagnostic`, `:55`
  `NewAssistantMessageDiagnostic`.
- `ai/internal/sse/sse.go:15-24` `Event`, `:39` `NewReader`, `:46` `Next`.
- `ai/internal/partialjson/partial.go:24` `ParsePartial`.
- `ai/apis/anthropic/anthropic.go:400-530` — the closest existing
  content-block assembly loop, and the `Partial: output.Clone()` convention.
- `ai/catalog/compat.go:20-37` `compatFieldsByApi`, `:52`
  `validateCompatForApi`.
- Upstream at `936aff00`: `src/api/pi-messages.ts` (433 —
  `PiMessagesEvent`, `createEventConverter`, `readPiMessagesEvents`,
  `parsePiMessagesEvent`, `appendRewriteDiagnostic`), `src/types.ts:27` and
  `:249` (the `Api` union member and its options mapping),
  `test/pi-messages.test.ts` (248).

## Dependencies

- **Blocked by**: [Epic 2](/epic-2-core-types-and-models-contracts/EPIC_2.md)
  [issue 01](/epic-2-core-types-and-models-contracts/issues/01-widen-stopreason-and-thinkinglevel.md)
  — `ai.StopReasonPending` does not exist yet (`ai/types.go:36-42`), and the
  partial's initial stop reason is `pending` in every upstream test assertion.
- **Blocks**: [Issue 02](/epic-8-pi-messages-and-radius/issues/02-pimessages-stream-entry.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening
the PR.
