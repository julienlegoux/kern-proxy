---
type: Issue
title: "anthropic: pending and raw stop reasons, prefilled content blocks, and nullable message_delta usage"
description: "Start the Anthropic stream at StopReason pending, record the provider's literal stop_reason, honor text/thinking prefilled on content_block_start, and stop treating a usage-less message_delta as zeroes."
tags: [epic-5]
timestamp: 2026-08-10T04:00:00Z
epic: 5
issue: 02
slug: anthropic-stream-lifecycle
size: M
status: open
gh_issue: 160
resource: https://github.com/kern-ia/kern-link/issues/160
depends_on: []
---

# anthropic: pending and raw stop reasons, prefilled content blocks, and nullable message_delta usage

## Summary

The stream-decoding half of the Anthropic sync. Five behavior changes, all in
the SSE path, all independently observable:

1. **`stopReason` starts at `"pending"`, not `"stop"`.** Upstream initializes
   `output.stopReason = "pending"` and, after the loop, throws
   `"Anthropic stream ended without a stop reason"` if it is still pending. Today
   a truncated stream that never delivers `message_delta` silently reports a
   clean `stop` — the bug this change exists to kill.
2. **`rawStopReason`.** The provider's literal `stop_reason` string is recorded
   on the message before mapping, so a consumer can see `"refusal"` or
   `"sensitive"` even though both map to `error`.
3. **`sensitive` gets an error message.** `mapStopReason("sensitive")` now
   returns `{stopReason: "error", errorMessage: "Provider stopped with: sensitive"}`
   instead of a bare `error`. **This string is load-bearing** —
   [CONVENTIONS.md](../../../planning/CONVENTIONS.md) records that `ai/retry.go`
   and `ai/overflow.go` classify by matching error text — so it is copied
   verbatim, not paraphrased.
4. **`content_block_start` may carry content.** `event.content_block.text`,
   `.thinking` and `.signature` are now seeded into the new block instead of
   being discarded in favor of `""`. Proxies that deliver a whole block in one
   event currently lose it entirely.
5. **`message_delta` without `usage` is a no-op.** The whole usage-accumulation
   body is wrapped in `if (event.usage)`. Today a proxy that omits `usage` from
   `message_delta` zeroes the input/output/cache counters that `message_start`
   already established.

`fetch` injection also lands here: upstream threads `options?.fetch` into
`createClient`. kern-link has no client object — the Go adapter talks raw HTTP
through `ai/apis/internal/httpretry` — so this is about honoring
`ProviderRequestOptions.Fetch` (Epic 2
[issue 13](/epic-2-core-types-and-models-contracts/issues/13-fetch-sampling-and-deferred-options.md))
on the anthropic request path, the same way epic 4
[issue 02](/epic-4-openai-family-adapters/issues/02-fetch-and-sampling-params.md)
does for the OpenAI family.

Upstream's `retryProviderRequest` wrapper (`maxRetries` moved off the SDK's own
retry and onto the shared helper) needs **no port**: `ai/apis/anthropic` already
routes its request through `httpretry.Do` with `DefaultMaxRetries`
(`anthropic.go:259`). Say so in the PR body — and note that whether
`utils/provider-retry.ts` supersedes `httpretry` is
[Epic 9](/epic-9-classifier-audit-and-release/EPIC_9.md)'s question, not this
PR's.

## Scope

- `ai/apis/anthropic/anthropic.go`:
  - `run` (`:207`) — initialize `output.StopReason = ai.StopReasonPending`
    (Epic 2 [issue 01](/epic-2-core-types-and-models-contracts/issues/01-widen-stopreason-and-thinkinglevel.md)),
    and after `decodeEvents` returns, before the existing
    aborted/error check (`:283`), fail with
    `errors.New("Anthropic stream ended without a stop reason")` when the reason
    is still pending.
  - `decodeEvents` (`:361`), `message_delta` branch (`:533`) — set
    `output.RawStopReason` from the raw `stop_reason` string before mapping
    (Epic 2 [issue 02](/epic-2-core-types-and-models-contracts/issues/02-message-model-deferred-fields.md)
    declares the field).
  - `mapStopReason` (`:679`) — `"sensitive"` returns the error message
    `Provider stopped with: sensitive`, byte for byte.
  - `rawContentBlockStart` (`:625`) — add `Text`, `Thinking` and `Signature`
    to the embedded `ContentBlock` struct.
  - `decodeEvents` `content_block_start` branch (`:411`) — seed
    `ai.TextContent{Text: …}` and
    `ai.ThinkingContent{Thinking: …, ThinkingSignature: …}` from those fields
    instead of the current zero values. The `redacted_thinking` and `tool_use`
    branches are unchanged. **Emit the matching `TextDeltaEvent` /
    `ThinkingDeltaEvent` for a non-empty prefill**, so it reaches streaming
    consumers the same way every other content source in this decoder does —
    seeded, then delivered as a delta before it lands in the final message.
    Decided in [EPIC_5.md](/epic-5-remaining-adapters/EPIC_5.md)'s `## Notes`.
  - `applyUsageDelta` (`:582`) — it already only overwrites non-null fields, but
    the caller (`:547`) invokes it unconditionally and then recomputes
    `TotalTokens`. Make a `message_delta` carrying **no** `usage` object at all
    a complete no-op, including the `TotalTokens` recomputation. `rawUsage`
    must be reachable as a pointer (or presence-tracked) for that distinction to
    exist.
  - Request path — honor `opts.Fetch` when set, defaulting to the current client
    when nil. Follow whatever seam epic 4
    [issue 02](/epic-4-openai-family-adapters/issues/02-fetch-and-sampling-params.md)
    establishes in `httpretry` rather than inventing a second one; if that issue
    has not merged, add the seam here and say so in the PR body.
  - The `mapThinkingLevelToEffort` doc comment (`:172-174`) — upstream rewrote
    its note to "effort `max` is available on all adaptive-thinking Claude
    models, while native `xhigh` is only available on Opus 4.7/4.8, Sonnet 5,
    and Fable 5." The function **body** is unchanged upstream and already
    matches; update the comment only.
- Tests, offline `httptest`, in
  `ai/apis/anthropic/anthropic_sse_parsing_test.go` (new focused file, per
  [CONVENTIONS.md](../../../planning/CONVENTIONS.md)'s one-concern-per-test-file
  habit).
- `// Ports:` headers stay accurate.

## Out of scope

- Strict tool schemas and signature-only thinking blocks — issue 03.
- Deferred tools / `tool_reference` — issue 04.
- Copilot dynamic headers — issue 01.
- `providers/anthropic.ts` (+49 in range: `ANTHROPIC_AUTH_TOKEN` as a bearer
  header). That is a provider binding, not this adapter; upstream's new
  `test/anthropic-auth-token.test.ts` covers it.
  [Epic 6](/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md) issue 07
  (#176) owns this work — the `ANTHROPIC_AUTH_TOKEN` bearer-header binding is
  exactly that issue's scope. Do not fold it in here.
- The `retryProviderRequest` vs `httpretry` question — Epic 9.

## Acceptance criteria / Definition of done

- [ ] `TestStreamEndingWithoutStopReasonFails` — an `httptest` SSE stream that
      delivers `message_start`, a text block, and `message_stop` but **no**
      `message_delta` ends with an `ErrorEvent` whose message is exactly
      `Anthropic stream ended without a stop reason`, not a `DoneEvent`.
- [ ] `TestRawStopReasonRecordsProviderString` — a stream finishing with
      `stop_reason: "refusal"` leaves `RawStopReason == "refusal"` while
      `StopReason` is the mapped value.
- [ ] `TestSensitiveStopReasonCarriesDescriptiveError` — `stop_reason:
      "sensitive"` produces `StopReason` `error` and `ErrorMessage ==
      "Provider stopped with: sensitive"` (exact string).
- [ ] `TestContentBlockStartPreservesPrefilledText` — a `content_block_start`
      whose `content_block` is `{"type":"text","text":"hello"}` followed
      immediately by `content_block_stop` yields a text block of `"hello"`
      **and** a `TextDeltaEvent` carrying `"hello"` is emitted before the block
      closes.
- [ ] `TestContentBlockStartPreservesPrefilledThinking` — the same for
      `{"type":"thinking","thinking":"…","signature":"sig"}`, with the signature
      landing on `ThinkingSignature` **and** a matching `ThinkingDeltaEvent`
      emitted.
- [ ] `TestMessageDeltaWithoutUsageLeavesUsageUntouched` — after a
      `message_start` establishing `input_tokens: 100`, a `message_delta`
      carrying only `delta.stop_reason` leaves `Usage.Input == 100` and
      `Usage.TotalTokens` unchanged.
- [ ] `TestAnthropicHonorsInjectedFetch` — a request with `opts.Fetch` set is
      dispatched through it, and the nil case still uses the default client.
- [ ] Ported upstream coverage: all six cases of
      `test/anthropic-sse-parsing.test.ts` (new at `936aff00`) come across as
      discrete Go tests, including "repairs malformed SSE JSON" and "ignores
      unknown SSE events after message_stop" — those two assert behavior the Go
      port already has, so they are regression locks, not new features.
- [ ] `docs/PORTING.md`'s row for `src/api/anthropic-messages.ts` still
      describes the Go code after this change (unchanged: already `ported`).
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `fix(apis): fail the anthropic stream when it ends without a stop reason`.

## Relevant files / areas

- `ai/apis/anthropic/anthropic.go` (1234 lines) — `:207` `run`, `:361`
  `decodeEvents`, `:411` `content_block_start`, `:533` `message_delta`, `:582`
  `applyUsageDelta`, `:625` `rawContentBlockStart`, `:655` `rawMessageDelta`,
  `:679` `mapStopReason`, `:172` the effort-mapping doc comment.
- `ai/apis/internal/httpretry/httpretry.go` — where `Fetch` injection has to
  land for every raw-HTTP adapter.
- `ai/types.go` — `StopReasonPending`, `AssistantMessage.RawStopReason`
  (Epic 2 issues 01 and 02).
- Upstream: `src/api/anthropic-messages.ts` at `936aff00` — hunks at the output
  initializer, `content_block_start`, `message_delta`, the post-loop guards, and
  `mapStopReason`.
- Upstream test: `test/anthropic-sse-parsing.test.ts` (new, 177 lines).

## Dependencies

- **Blocked by**: Epic 2 [issue 01](/epic-2-core-types-and-models-contracts/issues/01-widen-stopreason-and-thinkinglevel.md)
  (`StopReasonPending`), [issue 02](/epic-2-core-types-and-models-contracts/issues/02-message-model-deferred-fields.md)
  (`RawStopReason`), [issue 13](/epic-2-core-types-and-models-contracts/issues/13-fetch-sampling-and-deferred-options.md)
  (`Fetch`).
- **Blocks**: None. Issues 03 and 04 touch the request-building half of the same
  file; merge order between them is a rebase concern, not a logical dependency.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
