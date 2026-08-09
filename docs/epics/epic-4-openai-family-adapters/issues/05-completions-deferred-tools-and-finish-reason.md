---
type: Issue
title: "openai-completions: Kimi deferred tools, finish-reason inference, and item-unique tool call ids"
description: "Withhold transcript-loaded tools and re-announce them in a Kimi system message, infer the stop reason when a provider omits finish_reason, record rawStopReason, and stop collapsing distinct tool calls onto one id."
tags: [epic-4]
timestamp: 2026-08-09T05:17:46Z
epic: 4
issue: 05
slug: completions-deferred-tools-and-finish-reason
size: M
status: open
gh_issue: 151
resource: https://github.com/kern-ia/kern-link/issues/151
depends_on: [3]
---

# openai-completions: Kimi deferred tools, finish-reason inference, and item-unique tool call ids

## Summary

Four changes to the chat-completions request/response contract, all in
`ai/apis/openaicompletions`:

1. **Deferred tools, Kimi flavor.** When `Compat.DeferredToolsMode == "kimi"`,
   tools whose names were *announced* by a `ToolResultMessage.addedToolNames`
   are withheld from the top-level `tools` array and re-announced mid-transcript
   as a system message carrying only a `tools` key and **no `content` field**
   ("Kimi accepts a system message with tools but omits the standard content
   field"). Upstream computes the withheld set with a local
   `getDeferredToolNames(messages)` walk, not with `splitDeferredTools`.
2. **Finish-reason inference.** With `Compat.SupportsFinishReason` false, a
   stream that ends without `finish_reason` no longer errors: the stop reason
   becomes `toolUse` if any content block is a tool call, else `stop`. With the
   flag true (the default), the missing-`finish_reason` error stays — and a stop
   reason still sitting at `pending` at the end is now an error too.
3. **`rawStopReason`.** The provider's literal `finish_reason` string is
   recorded on the assistant message alongside the mapped `StopReason`.
   [Epic 2 issue 02](/epic-2-core-types-and-models-contracts/issues/02-message-model-deferred-fields.md)
   ships the field and hands adapter-side population to this epic.
4. **Item-unique tool call ids.** `normalizeToolCallID`
   (`openaicompletions.go:744`) currently takes `callId` from a `callId|itemId`
   composite and drops the rest — so two tool calls in one turn that share a
   `call_id` collapse onto one id, which Chat Completions rejects. Upstream now
   joins both halves (`callId_itemId`) when the result fits in 40 chars, and
   otherwise falls back to `callIdPrefix_<8-char shortHash(fullId)>`.

Also here, because it is a one-line type widening on the same options struct:
`toolChoice` accepts the full OpenAI tool-choice union rather than the four
hand-listed shapes.

## Scope

- `ai/apis/openaicompletions/openaicompletions.go`:
  - `buildParams` (`:340`) — compute the withheld set when
    `compat.DeferredToolsMode == "kimi"`, filter `chat.Tools` before
    `convertTools`, and keep the existing `zaiToolStream` behavior intact for
    the filtered list.
  - New unexported helpers mirroring upstream: `deferredToolNames(messages)`
    (union of every `ToolResultMessage.AddedToolNames`) and
    `toolsByName(tools, names)` (dedup by name, **deterministic order** — a Go
    map here silently reorders the request payload and breaks prompt caching;
    keep an explicit order slice, the same trap
    [Epic 2 issue 11](/epic-2-core-types-and-models-contracts/issues/11-deferred-tools-split.md)
    calls out).
  - `convertMessages` (`:767`) — after the tool-result run, append the Kimi
    system message when the mode is set and there are deferred tools; it carries
    `role: "system"` and `tools`, no `content`.
  - `normalizeToolCallID` (`:744`) — the combined-id + `ShortHash` fallback,
    using `ai.ShortHash` (`ai/hash.go:14`) truncated to 8 chars. Keep the
    existing non-composite branches (`openai` 40-char truncation, others)
    untouched.
  - `run` (`:72`) — the finish-reason inference and the `pending` guard, in
    upstream's order: infer first, then the `error` check, then the
    missing-`finish_reason` error.
  - `decodeEvents` (`:1045`) — set `output.RawStopReason` from the raw
    `finish_reason` before mapping it.
  - The `toolChoice` option's type widening on `ai.StreamOptions`' completions
    field (or wherever the flat per-adapter field lives), keeping the wire
    encoding permissive.
- Tests: port `test/openai-completions-raw-stop-reason.test.ts` (+79) and the
  relevant halves of `test/openai-completions-tool-choice.test.ts` (+218) and
  `test/deferred-tools.test.ts`; offline `httptest` only.

## Out of scope

- `ai.SplitDeferredTools` — [Epic 2 issue 11](/epic-2-core-types-and-models-contracts/issues/11-deferred-tools-split.md)
  ports it, and the **responses** adapters use it (issue 07). Chat completions
  deliberately does not: upstream uses a local, simpler walk here. Do not
  "unify" them.
- Grammar/custom tools — issue 03.
- The `RawStopReason` field declaration and its codec — Epic 2 issue 02.
- Auditing `ai/retry.go` / `ai/overflow.go` against the strings this adapter now
  emits — [Epic 9](/epic-9-classifier-audit-and-release/EPIC_9.md) owns that,
  deliberately after the text stops moving.

## Acceptance criteria / Definition of done

- [ ] `TestKimiDeferredToolsWithheldFromTopLevelTools` — with
      `DeferredToolsMode: "kimi"` and a transcript whose tool result announces
      `search`, the request's top-level `tools` omits `search` while still
      carrying the others.
- [ ] `TestKimiDeferredToolsAnnouncedInSystemMessage` — the same request contains
      a `{"role":"system","tools":[…]}` message positioned after the announcing
      tool result, with no `content` key in the marshalled JSON.
- [ ] `TestKimiDeferredToolOrderIsDeterministic` — the same context built twice
      produces byte-identical `tools` arrays (run it enough times to catch map
      ordering, e.g. 20 iterations).
- [ ] `TestDeferredToolsIgnoredWithoutKimiMode` — with the mode unset, nothing is
      withheld and no system message is added.
- [ ] `TestInferStopReasonWhenFinishReasonUnsupported` — `SupportsFinishReason:
      false` plus a stream ending with a tool call yields `StopReason` `toolUse`;
      the same stream with text only yields `stop`; neither errors.
- [ ] `TestMissingFinishReasonStillErrorsWhenSupported` — the default compat
      keeps upstream's `Stream ended without finish_reason` error, verbatim.
- [ ] `TestRawStopReasonRecordsProviderString` — a stream finishing with
      `finish_reason: "length"` leaves `RawStopReason == "length"` and
      `StopReason == ai.StopReasonLength`.
- [ ] `TestNormalizeToolCallIDKeepsItemUniqueness` — two tool calls sharing
      `call_id` but differing in `item_id` normalize to **different** ids; a
      composite whose joined form exceeds 40 chars normalizes to
      `<prefix>_<8 hex>` of total length ≤ 40 and is stable across runs.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(apis): support kimi deferred tools and finish-reason inference`.

## Relevant files / areas

- `ai/apis/openaicompletions/openaicompletions.go:72` `run`, `:340`
  `buildParams`, `:744` `normalizeToolCallID`, `:767` `convertMessages`, `:1045`
  `decodeEvents`, `:1327` `rawChoice`.
- `ai/hash.go:14` `ShortHash`.
- `ai/apis/openaicompletions/openaicompletions_test.go` (506 lines) and
  `compat_wiring_test.go` (234) — existing coverage.
- Upstream: `src/api/openai-completions.ts` (`getDeferredToolNames`,
  `getToolsByName`, `KimiToolSystemMessageParam`, the id-normalizer, the
  finish-reason block) at `936aff00`.

## Dependencies

- **Blocked by**: [Issue 03](/epic-4-openai-family-adapters/issues/03-completions-grammar-custom-tools.md)
  (shares `convertTools` / `buildParams`);
  [Epic 2 issue 02](/epic-2-core-types-and-models-contracts/issues/02-message-model-deferred-fields.md)
  for `RawStopReason` and `AddedToolNames`;
  [Epic 2 issue 04](/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md)
  for `SupportsFinishReason` / `DeferredToolsMode`.
- **Blocks**: None.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
