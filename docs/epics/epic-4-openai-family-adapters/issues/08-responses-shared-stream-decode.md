---
type: Issue
title: "openai-responses shared: custom tool-call streaming, reasoning backfill, and stop-reason mapping"
description: "Decode custom_tool_call input deltas into JSON tool-call deltas, backfill encrypted reasoning signatures at completion, split incomplete responses by reason, and account cache-write tokens."
tags: [epic-4]
timestamp: 2026-08-11T15:00:00Z
epic: 4
issue: 08
slug: responses-shared-stream-decode
size: L
status: open
gh_issue: 154
resource: https://github.com/kern-ia/kern-link/issues/154
depends_on: [6]
---

# openai-responses shared: custom tool-call streaming, reasoning backfill, and stop-reason mapping

## Summary

The response half of the shared responses core (`ai/apis/openairesponses/stream.go`,
518 lines). Five behaviors change and they are genuinely entangled — all live
inside the single event loop and its finalizer — which is why this is the epic's
one **L**.

1. **Custom tool-call streaming.** New events
   `response.custom_tool_call_input.delta` / `.done`, and a `custom_tool_call`
   output item. The block carries a grammar input buffer; each raw increment is
   turned into a JSON delta (`{"prop":"` … `"}`) via issue 01's
   `AppendInputJSONDelta`, and the block's arguments are rewritten to
   `{property: input}`. The input property comes from the request-side grammar
   map, defaulting to `"input"` when the tool is unknown.
2. **Reasoning signature backfill.** Encrypted reasoning content sometimes only
   arrives on the terminal `response.completed` payload. The decoder now indexes
   thinking blocks by reasoning-item id during the stream and, at completion,
   re-serializes each block's stored signature with the late
   `encrypted_content` merged in — skipping blocks that already have one.
3. **Stop reason.** `mapStopReason` becomes `(status, incompleteReason) →
   (StopReason, errorMessage)`. `incomplete` now maps to `length` **only** for
   `max_output_tokens`; any other reason becomes `error` with
   `"Response incomplete: <reason>"` (or `"Response incomplete without a
   provider reason"`). `RawStopReason` is set to `status` or
   `"<status>.<reason>"`.
4. **Message phase.** An output item of type `message` with
   `phase == "final_answer"` sets the stop reason to `stop` as it arrives — the
   signal a response is answering rather than still working.
5. **Cache-write tokens.** `input_tokens_details.cache_write_tokens` is now read,
   reported as `cacheWrite`, and **subtracted from `input`** alongside
   `cached_tokens`, floored at zero.

Callers additionally start the message at `StopReason` `pending` and treat a
stream that ends still `pending` as an error; that entry-point half is in issues
09 (responses), 10 (azure), 11 (codex) since each adapter owns its own `run`.

## Scope

- `ai/apis/openairesponses/stream.go`:
  - `ServiceTierOptions` (`:28`) — rename or extend to carry
    `GrammarToolInputProperties map[string]string` (upstream's
    `OpenAIResponsesStreamOptions` is the same struct gaining a field; keep the
    Go name if renaming ripples too far, and say which you chose in the PR body).
  - `responsesSlot` (`:58`) — add the grammar input buffer to the tool-call slot.
  - `DecodeStream` (`:70`) — handle `custom_tool_call` items and the two new
    input events; keep the existing `function_call_arguments.delta` path
    untouched.
  - `finalizeResponse` (`:288`) — the backfill, the usage arithmetic, the new
    stop-reason mapping, `RawStopReason`.
  - `mapStopReason` (`:361`) — the new signature and the `incomplete` split. It
    already returns `(ai.StopReason, string)` in Go; extend it with the reason
    parameter rather than inventing a second function.
  - `rawItem` (`:462`) / `rawResponseTokenDetails` (`:473`) /
    `rawIncompleteDetails` (`:491`) — decode `phase`, `encrypted_content`,
    `cache_write_tokens`, `incomplete_details.reason`.
  - `responseFailedError` (`:263`) — set `RawStopReason` from
    `event.response.status` on `response.failed`, matching upstream.
- Scratch state (`partialJson`, the grammar buffer) must never reach a persisted
  message — the adapters clear it in their error paths; make sure the decoder
  does not leak it into `output.Content` on the success path either.
- Port `test/openai-responses-terminal-event.test.ts` (+135) and the reasoning /
  usage cases of `test/openai-responses-reasoning-replay-e2e.test.ts` and
  `test/openai-responses-partial-json-cleanup.test.ts`.
- Keep the `// Ports:` header's symbol list current.

## Out of scope

- Per-adapter entry-point changes (`pending` start state, the
  `"…stream ended without a stop reason"` error, `errorMessage` propagation) —
  issues 09, 10, 11.
- Request-side grammar conversion — issue 06.
- Namespace and deferred-tool transcript injection — issue 07.
- Auditing `ai/retry.go` / `ai/overflow.go` against the new
  `"Response incomplete: …"` text —
  [Epic 9](/epic-9-classifier-audit-and-release/EPIC_9.md).

## Acceptance criteria / Definition of done

- [ ] `TestCustomToolCallInputStreamsAsJSONDeltas` — an SSE fixture delivering
      `response.custom_tool_call_input.delta` three times then `.done` produces
      `toolcall_delta` events concatenating to `{"query":"abc"}` and a final tool
      call whose arguments decode to that object.
- [ ] `TestCustomToolCallUnknownToolDefaultsToInputProperty` — a custom tool call
      whose name is absent from the grammar map uses `"input"` as the property.
- [ ] `TestReasoningSignatureBackfilledFromCompletion` — a thinking block whose
      streamed signature had no `encrypted_content` ends up with the value
      carried on the terminal `response.completed` item of the same id, and a
      block that already had one is left untouched.
- [ ] `TestIncompleteMaxOutputTokensMapsToLength` — `status: "incomplete"` with
      `incomplete_details.reason = "max_output_tokens"` yields `StopReason`
      `length`, no error message, and `RawStopReason ==
      "incomplete.max_output_tokens"`.
- [ ] `TestIncompleteOtherReasonMapsToError` — reason `content_filter` yields
      `StopReason` `error` and the exact message
      `Response incomplete: content_filter`.
- [ ] `TestIncompleteWithoutReasonMapsToError` — no reason yields
      `Response incomplete without a provider reason` and `RawStopReason ==
      "incomplete"`.
- [ ] `TestFinalAnswerPhaseSetsStopReasonStop` — a `message` item with
      `phase: "final_answer"` sets `stop` even before the terminal event.
- [ ] `TestCacheWriteTokensSubtractedFromInput` — usage
      `{input_tokens: 100, cached: 30, cache_write: 20}` yields
      `input: 50, cacheRead: 30, cacheWrite: 20`; an over-counting payload
      (`cached + cache_write > input_tokens`) floors `input` at 0 rather than
      going negative.
- [ ] `docs/PORTING.md`: the `openai-responses-shared.ts` row stays `ported`,
      unchanged — this PR adds symbols to an already-ported file without
      altering its disposition or its parenthetical.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(apis): decode custom tool calls and split incomplete stop reasons`.

## Relevant files / areas

- `ai/apis/openairesponses/stream.go` (518 lines) — `:28` `ServiceTierOptions`,
  `:47-68` the slot types, `:70` `DecodeStream`, `:263` `responseFailedError`,
  `:288` `finalizeResponse`, `:361` `mapStopReason`, `:451-509` the raw wire
  structs.
- `ai/apis/openairesponses/stream_test.go` (378 lines).
- `ai/apis/internal/grammar` — issue 01's buffer.
- Upstream: `src/api/openai-responses-shared.ts` `processResponsesStream`,
  `backfillReasoningSignatures`, `mapStopReason` at `936aff00`.

## Dependencies

- **Blocked by**: [Issue 06](/epic-4-openai-family-adapters/issues/06-responses-shared-grammar-and-tool-results.md)
  (the grammar option plumbing);
  [Epic 2 issue 01](/epic-2-core-types-and-models-contracts/issues/01-widen-stopreason-and-thinkinglevel.md)
  (`pending`) and
  [issue 02](/epic-2-core-types-and-models-contracts/issues/02-message-model-deferred-fields.md)
  (`RawStopReason`).
- **Blocks**: [Issue 09](/epic-4-openai-family-adapters/issues/09-openai-responses-compat-and-wiring.md),
  [Issue 10](/epic-4-openai-family-adapters/issues/10-azure-responses-wiring.md),
  [Issue 11](/epic-4-openai-family-adapters/issues/11-codex-request-body-and-stop-reasons.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR. **Sized L deliberately**: the five behaviors share one event loop and one
finalizer, and landing them separately would mean three PRs each rewriting the
same forty lines. If it does approach 1000, the clean cut is
custom-tool-call streaming in one PR and the finalizer changes (backfill, usage,
stop reason) in another.
