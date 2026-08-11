---
type: Issue
title: "openai-codex-responses: grammar and deferred tools in the request body, end_turn and stop-reason guards"
description: "Wire Codex onto the updated shared responses core, add tool_choice, and record end_turn plus the pending/error stop-reason guards on every transport."
tags: [epic-4]
timestamp: 2026-08-11T18:15:00Z
epic: 4
issue: 11
slug: codex-request-body-and-stop-reasons
size: L
status: open
gh_issue: 157
resource: https://github.com/kern-ia/kern-link/issues/157
depends_on: [6, 7, 8]
---

# openai-codex-responses: grammar and deferred tools in the request body, end_turn and stop-reason guards

## Summary

Codex's request builder catches up with the shared core, and its stream-end
contract tightens.

- **`buildRequestBody`** resolves `supportsStrictMode` (default **true**),
  `supportsOpenAIGrammarTools` (default false) and a deferred-tools mode
  (`additional-tools` → `tool-search` → off) from `model.compat`, splits tools
  with `SplitDeferredTools`, sends only the immediate ones, and passes
  `{strict: null, supportsStrictMode, supportsOpenAIGrammarTools}` to the
  converters. `strict: null` is Codex's existing default and stays.
- **`tool_choice`** becomes `opts.OpenAIToolChoice` falling back to `"auto"`,
  instead of a hardcoded `"auto"` (`params.go:102`). The field is
  `ai.StreamOptions.OpenAIToolChoice`, declared as `any` by
  [Epic 2 issue 13](/epic-2-core-types-and-models-contracts/issues/13-fetch-sampling-and-deferred-options.md);
  upstream's Codex options narrow it to `"auto" | "none" | "required"`
  (`packages/ai/src/api/openai-codex-responses.ts:91`, used at `:569` as
  `options?.toolChoice ?? "auto"`), but the Go field is shared with completions
  and responses, so Codex simply passes through whatever it is given.
- **`end_turn`** — `response.done` / `.completed` / `.incomplete` events carry a
  boolean `end_turn`, which is recorded on the assistant message.
  [Epic 2 issue 02](/epic-2-core-types-and-models-contracts/issues/02-message-model-deferred-fields.md)
  ships the field and hands adapter population here.
- **Stop-reason guards** — upstream folds the repeated end-of-stream checks into
  `assertSuccessfulOutput`: `pending` → `Codex stream ended without a stop
  reason`; `error`/`aborted` → `output.errorMessage` or `An unknown error
  occurred`. It runs on **both** the WebSocket and SSE paths, which is the point
  of extracting it — the two paths previously drifted.

## Scope

- `ai/apis/codex/params.go`:
  - `buildRequestBody` (`:66`) — the grammar map, the strict/grammar converter
    options, the deferred-tool split and mode, and `tool_choice` from
    `opts.OpenAIToolChoice` (currently hardcoded `"auto"` at `:102`).
  - `wireRequest` (`:38`) — widen its `tool_choice` member (`params.go:47`,
    today `string` with `json:"tool_choice,omitempty"`) to a free-form value, so
    any shape `OpenAIToolChoice` carries survives to the wire.
- `ai/apis/codex/codex.go`:
  - `run` (`:75`) — start `output.StopReason` at `ai.StopReasonPending`, build
    the grammar map once and thread it into the shared converters and decoder.
  - `finishCodexStream` (`:240`) — replace the inline end-of-stream checks with
    a single `assertSuccessfulOutput`-equivalent helper used by the WebSocket and
    SSE paths alike; error text upstream-verbatim.
  - Event mapping — record `end_turn` when the terminal event carries a boolean,
    and `RawStopReason` from the response status (the shared decoder does this
    for the responses adapters; Codex's own event mapper needs the same).
- Port the request-body and stop-reason cases of
  `test/openai-codex-stream.test.ts` (+778 upstream — this PR takes the half
  about request shape and stream termination; issue 12 takes the session/
  transport half).
- Keep the `// Ports:` header's symbol list current.

## Out of scope

- Session-id derivation, prompt-cache-key clamping, `uuidv7` request ids and the
  `previous_response_not_found` retry — issue 12.
- **`validateRetryDelayMs`.** Upstream turned its delay *clamp* into a thrown
  `Server requested Xs retry delay (max: Ys)` error. kern-link retries through
  the shared `httpretry`, whose `capRetryDelay` clamps for all nine adapters, so
  flipping it here would change behavior well outside Codex. That comparison is
  already logged as
  [Epic 9](/epic-9-classifier-audit-and-release/EPIC_9.md)'s call
  ([Epic 3 issue 06](/epic-3-catalog-schema-and-export-tooling/issues/06-images-adapter-retry.md)
  recorded it) — leave it, and note in the PR body that Codex still clamps.
- The WebSocket connection cache and its account-keyed rework — kern-link never
  ported the cache (`docs/PORTING.md`: "upstream's cross-request connection
  cache is not [ported] — each request opens its own connection", and
  `websocket.go:15-25` documents the scope). Upstream's change to it has no Go
  counterpart; issue 12 records that rather than porting it.

## Acceptance criteria / Definition of done

- [ ] `TestCodexSendsGrammarCustomTools` — a grammar tool with
      `SupportsOpenAIGrammarTools` serializes as the flat responses `custom` tool
      inside the Codex body.
- [ ] `TestCodexKeepsStrictNullDefault` — a non-constrained tool still serializes
      with Codex's existing `strict` treatment; the port's documented
      `"strict": false` deviation (`docs/PORTING.md`, "Codex tool `strict` flag")
      is either preserved or, if this PR changes it, the deviation entry is
      updated in the same commit.
- [ ] `TestCodexDeferredToolsWithheldFromRequest` — with
      `SupportsAdditionalTools`, an announced-but-unused tool is absent from
      `tools` and appears as an `additional_tools` transcript item.
- [ ] `TestCodexToolChoiceOverridesAuto` — an explicit
      `ai.StreamOptions.OpenAIToolChoice` replaces `"auto"` in the body; unset
      (nil) still sends `"auto"`.
- [ ] `TestCodexRecordsEndTurn` — a terminal event with `"end_turn": false`
      leaves `EndTurn` false on the message and `true` sets it; an event without
      the key leaves it unset.
- [ ] `TestCodexStreamEndingPendingErrors` — both transports (SSE and the
      WebSocket path's fallback-free case) fail with
      `Codex stream ended without a stop reason`, verbatim.
- [ ] `TestCodexErrorMessagePropagated` — an errored stream surfaces the
      provider's message rather than `An unknown error occurred`.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(apis): wire codex onto the 0.84.1 shared responses core`.

## Relevant files / areas

- `ai/apis/codex/params.go` (216 lines) — `:38` `wireRequest`, `:66`
  `buildRequestBody`, `:131` `applyReasoning`, `:192`
  `clampOpenAIPromptCacheKey`.
- `ai/apis/codex/codex.go` (388) — `:75` `run`, `:196`
  `attemptCodexWebSocketOrFallback`, `:240` `finishCodexStream`.
- `ai/apis/codex/stream_test.go` (675), `params_test.go` (230).
- `ai/apis/openairesponses/messages.go`, `stream.go` — the shared core.
- `docs/PORTING.md` — the Codex row and the `strict` deviation.
- Upstream: `src/api/openai-codex-responses.ts` (`buildRequestBody`,
  `assertSuccessfulOutput`, `mapCodexEvents`) and
  `test/openai-codex-stream.test.ts` at `936aff00`.

## Dependencies

- **Blocked by**: issues [06](/epic-4-openai-family-adapters/issues/06-responses-shared-grammar-and-tool-results.md),
  [07](/epic-4-openai-family-adapters/issues/07-responses-shared-namespace-and-deferred-tools.md),
  [08](/epic-4-openai-family-adapters/issues/08-responses-shared-stream-decode.md);
  [Epic 2 issue 02](/epic-2-core-types-and-models-contracts/issues/02-message-model-deferred-fields.md)
  (`EndTurn`), [issue 13](/epic-2-core-types-and-models-contracts/issues/13-fetch-sampling-and-deferred-options.md)
  (`OpenAIToolChoice` on `ai.StreamOptions`), and
  [issue 11](/epic-2-core-types-and-models-contracts/issues/11-deferred-tools-split.md)
  (`SplitDeferredTools`) — cross-epic, so stated here rather than in
  `depends_on`, which holds intra-epic numbers only.
- **Blocks**: [Issue 12](/epic-4-openai-family-adapters/issues/12-codex-session-ids-and-continuation-retry.md).

## PR size note

`L` — ~700 changed lines: this PR takes the request-shape and stream-end half
of the +778-line upstream `openai-codex-stream.test.ts`, and every termination
case must be proven on both the SSE and the WebSocket transport, so each named
test needs two harnesses. Raised from `M` on that duplication. `L` is the
ceiling, not the target: `assertSuccessfulOutput` exists because the two
transports drifted apart, so they must land together; past ~1000 the honest cut
is the `buildRequestBody` tool wiring apart from the stop-reason guards.
