---
type: Issue
title: "mistral: pending and raw stop reasons with provider-stopped error text, and strict tool sampling"
description: "Stop silently mapping unknown Mistral finish reasons to stop, start the stream at pending, record the raw finish reason, and resolve per-tool strict JSON-schema sampling."
tags: [epic-5]
timestamp: 2026-08-11T18:15:00Z
epic: 5
issue: 07
slug: mistral-stop-reasons-and-strict-tools
size: M
status: open
gh_issue: 165
resource: https://github.com/kern-ia/kern-link/issues/165
depends_on: []
---

# mistral: pending and raw stop reasons with provider-stopped error text, and strict tool sampling

## Summary

`mistral-conversations.ts` is the epic's largest upstream diff (+423), but most
of it is upstream **catching up to kern-link**: it dropped
`@mistralai/mistralai` and hand-rolled raw HTTP + SSE + snake_case wire
serialization, which is what `ai/apis/mistral` has always done (`PORTING.md`:
"ported (raw REST; no Go Mistral SDK)"). The 300-odd lines of
`requestMistralStream` / `readMistralEvents` / `toMistralWirePayload` have no Go
counterpart to write. What is left is behavioral, and splits into this issue and
[issue 08](/epic-5-remaining-adapters/issues/08-mistral-wire-and-header-parity.md).

This issue covers the two behavioral changes:

1. **Stop-reason mapping stops lying.** `mapChatStopReason` was
   `string → StopReason` with `default: return "stop"`. It is now
   `string → {stopReason, errorMessage?}`:

   | reason | before | after |
   |---|---|---|
   | `stop` | stop | stop |
   | `length`, `model_length` | length | length |
   | `tool_calls` | toolUse | toolUse |
   | `error` | error | error + `"Provider stopped with: error"` |
   | *anything else* | **stop** | **error** + `"Provider stopped with: <reason>"` |
   | `null` | stop | stop |

   That default flip is the substantive bug fix: an unrecognized finish reason
   currently reports a clean completion. `output.rawStopReason` records the
   literal string, the stream starts at `"pending"`, and a stream that ends
   without a finish reason throws
   `"Mistral stream ended without a finish reason"`. **All error strings are
   copied verbatim** — `ai/retry.go` and `ai/overflow.go` classify by matching
   them ([CONVENTIONS.md](../../../planning/CONVENTIONS.md)).

2. **Strict tool sampling.** `toFunctionTools` hardcodes `strict: false` today.
   Upstream now resolves it per tool:

   ```ts
   const strict = resolveJsonSchemaStrictSampling(tool, true);
   // …
   strict: strict ?? false
   ```

   Note the literal `true` for `supportsStrictMode` — Mistral is assumed to
   support strict mode unconditionally, so there is no compat flag to thread
   and `strict: "require"` never throws here. Keep the `?? false` fallback: the
   field stays present and non-optional on the wire.

## Scope

- `ai/apis/mistral/messages.go`:
  - `mapChatStopReason` (`:97`) — return `(ai.StopReason, string)` (or a small
    struct), with the new default branch and the two `Provider stopped with: …`
    strings. An empty/absent reason keeps mapping to `stop`.
  - `toFunctionTools` (`:55`) — resolve strict via
    `grammar.ResolveJSONSchemaStrictSampling(tool, true)` (epic 4
    [issue 01](/epic-4-openai-family-adapters/issues/01-constrained-sampling-core.md)),
    defaulting to false when it resolves to "unspecified".
    `wireToolFunction.Strict` (`:50`) is already a non-omitempty bool, which
    matches upstream's always-present field.
- `ai/apis/mistral/stream.go`:
  - `DecodeStream` (`:47`) — set `output.RawStopReason` from the raw
    `finish_reason` before mapping, and set `output.ErrorMessage` from the
    mapper's second return. `rawChoice` (`:327`) already decodes
    `finish_reason`.
- `ai/apis/mistral/mistral.go`:
  - `run` (`:98`) — initialize `output.StopReason = ai.StopReasonPending`
    (currently `ai.StopReasonStop`, `:104`), and add the pending guard before
    the existing aborted/error branch (`:174`) with the exact message
    `Mistral stream ended without a finish reason`. The existing branch already
    prefers `output.ErrorMessage` over `"An unknown error occurred"`, matching
    upstream — leave it.
- Tests, offline `httptest`, in `ai/apis/mistral/stop_reason_test.go`;
  strict-tool assertions alongside the existing `mistral_test.go` wire
  coverage.
- `// Ports:` headers stay accurate.

## Out of scope

- Request/wire shape and headers (`prefix`, tool-call `index`, `x-affinity`
  override precedence, the request timeout, nil-safe thinking parts) —
  [issue 08](/epic-5-remaining-adapters/issues/08-mistral-wire-and-header-parity.md).
- Porting `requestMistralStream`, `readMistralEvents`, `toMistralWirePayload`,
  `MistralHttpError` or `findMistralEventBoundary` as code: kern-link already
  has all of it (`mistral.go:146` raw POST through `httpretry.Do`,
  `ai/internal/sse` for framing, snake_case struct tags in `wireRequest`
  `:218`). Say so in the PR body; the *tests* those functions brought with them
  are issue 08's business.

## Acceptance criteria / Definition of done

- [ ] `TestUnknownFinishReasonIsProviderError` — an `httptest` SSE stream
      finishing with `finish_reason: "content_filter"` ends with an
      `ErrorEvent`, `StopReason` `error`, `RawStopReason == "content_filter"`,
      and `ErrorMessage == "Provider stopped with: content_filter"` (exact
      string). It must **not** produce a `DoneEvent`.
- [ ] `TestErrorFinishReasonCarriesDescriptiveMessage` — `finish_reason:
      "error"` yields `ErrorMessage == "Provider stopped with: error"`.
- [ ] `TestKnownFinishReasonsUnchanged` — `stop`, `length`, `model_length`,
      `tool_calls` map exactly as before and set `RawStopReason` to the literal
      string while producing a `DoneEvent`.
- [ ] `TestMistralStreamWithoutFinishReasonFails` — a stream delivering content
      and `[DONE]` but no `finish_reason` ends with an `ErrorEvent` whose
      message is exactly `Mistral stream ended without a finish reason`.
- [ ] `TestToFunctionToolsResolvesStrictPerTool` — a tool with
      `ConstrainedSampling` `{type: "json_schema", strict: "prefer"}`
      serializes with `"strict": true`; a tool with no config serializes with
      `"strict": false`, and the key is present in both cases.
- [ ] Ported upstream coverage: all three cases of
      `test/mistral-raw-stop-reason.test.ts` and the strict case of
      `test/mistral-tool-schema.test.ts` (+3 in range) come across as discrete
      Go tests.
- [ ] `docs/PORTING.md`'s row for `src/api/mistral-conversations.ts` still
      describes the Go code after this change (unchanged: already
      `ported (raw REST; no Go Mistral SDK)`).
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `fix(apis): stop mapping unknown mistral finish reasons to a clean stop`.

## Relevant files / areas

- `ai/apis/mistral/messages.go` (276 lines) — `:41` `wireTool`, `:46`
  `wireToolFunction`, `:55` `toFunctionTools`, `:97` `mapChatStopReason`.
- `ai/apis/mistral/stream.go` (337 lines) — `:47` `DecodeStream`, `:327`
  `rawChoice`.
- `ai/apis/mistral/mistral.go` (368 lines) — `:98` `run`, `:104` the output
  initializer, `:174` the aborted/error branch, `:218` `wireRequest`.
- `ai/apis/internal/grammar` — epic 4 issue 01's package.
- `ai/retry.go`, `ai/overflow.go` — the classifiers that match this error text.
- Upstream: `src/api/mistral-conversations.ts` at `936aff00` —
  `mapChatStopReason`, `toFunctionTools`, `consumeChatStream`'s
  `choice.finish_reason` hunk, and the post-loop guards in `stream`.
- Upstream tests: `test/mistral-raw-stop-reason.test.ts` (new, 61 lines),
  `test/mistral-tool-schema.test.ts` (+3).

## Dependencies

- **Blocked by**: Epic 4
  [issue 01](/epic-4-openai-family-adapters/issues/01-constrained-sampling-core.md)
  (cross-epic — `ResolveJSONSchemaStrictSampling`); Epic 2
  [issue 01](/epic-2-core-types-and-models-contracts/issues/01-widen-stopreason-and-thinkinglevel.md)
  (`StopReasonPending`) and
  [issue 02](/epic-2-core-types-and-models-contracts/issues/02-message-model-deferred-fields.md)
  (`RawStopReason`).
- **Blocks**: [Issue 08](/epic-5-remaining-adapters/issues/08-mistral-wire-and-header-parity.md)
  touches the same three files; land this one first.

## PR size note

`M` — ~300 changed lines: the epic's largest upstream diff (+423) is misleading
here, since roughly 300 of those lines are upstream hand-rolling transport
kern-link already has. The Go work is `mapChatStopReason`'s new return shape,
`toFunctionTools`' per-tool strict resolution, two decoder hunks and the pending
guard, plus five named tests and four ported upstream cases. Split past ~500,
and the seam is stop reasons versus strict tools.
