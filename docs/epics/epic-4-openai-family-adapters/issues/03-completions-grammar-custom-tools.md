---
type: Issue
title: "openai-completions: emit and stream grammar custom tools"
description: "Send grammar-constrained tools as type:\"custom\" chat-completions tools, replay them on the request side, and reassemble their streamed raw input into JSON tool-call deltas."
tags: [epic-4]
timestamp: 2026-08-11T20:00:00Z
epic: 4
issue: 03
slug: completions-grammar-custom-tools
size: M
status: open
gh_issue: 149
resource: https://github.com/kern-ia/kern-link/issues/149
depends_on: ["01"]
---

# openai-completions: emit and stream grammar custom tools

## Summary

With `ai/apis/internal/grammar` in place (issue 01), the chat-completions
adapter learns the OpenAI *custom tool* wire shape. Three surfaces change, all
in `ai/apis/openaicompletions`:

1. **Tool definitions.** `convertTools` (`openaicompletions.go:702`) emits
   `{"type":"custom","custom":{"name","description","format":{"type":"grammar","grammar":{"syntax","definition"}}}}`
   for a tool that resolves to a grammar, and otherwise keeps the existing
   `{"type":"function", …}` shape — with `strict` now taking the value
   `resolveJsonSchemaStrictSampling` returns (`strict ?? false`) instead of a
   hardcoded `false`, still gated on `supportsStrictMode !== false`.
2. **Request-side replay.** `convertMessages`
   (`openaicompletions.go:767`/`convertAssistantMessage:850`) replays a prior
   grammar tool call as `{"id","type":"custom","custom":{"name","input"}}`,
   where `input` is `getGrammarToolInput(name, arguments, inputProperty)` run
   through surrogate sanitization — not `JSON.stringify(arguments)`.
3. **Streaming.** A `custom` tool-call delta carries raw grammar text, not JSON.
   The adapter buffers it and *synthesizes* JSON deltas so downstream consumers
   see an ordinary `toolcall_delta` stream:
   `appendGrammarToolInputJsonDelta(buffer, property, nextInput, close)` emits
   `{"property":"` first, then the escaped increment, then `"}` on close, and the
   block's `arguments` is rewritten to `{property: nextInput}` each time.

The `customInput` scratch state is a streaming buffer and must never be
persisted: upstream deletes it alongside `partialArgs` and `index` in the error
path.

## Scope

- `ai/apis/openaicompletions/openaicompletions.go`:
  - In `run` (`:72`), build the name→inputProperty map once via
    `grammar.InputProperties(chat.Tools, compat.SupportsOpenAIGrammarTools)` and
    thread it into `buildParams`, `convertMessages`, and the stream decoder. A
    resolution error must surface as an in-band error event through the existing
    `ai.LazyStream` path, never a panic
    ([CONVENTIONS.md](../../../planning/CONVENTIONS.md), "Failures never escape
    as panics").
  - `wireTool` (`:265`) and `wireFunction` (`:283`) — add the `custom` variant.
    A tool is either `function` or `custom`; model it so an invalid combination
    is unrepresentable or, if that costs too much, assert it in a test.
  - `wireToolCall` (`:327`) / `wireToolCallFunction` (`:333`) — add the
    `custom: {name, input}` variant for assistant-message replay.
  - `convertTools` (`:702`) — grammar branch first, then the strict resolution
    for the function branch.
  - `convertAssistantMessage` (`:850`) — the custom-tool-call replay, with
    surrogate sanitization via the existing `ai.SanitizeUnicode` path already
    used elsewhere in the file.
  - `decodeEvents` (`:1045`) and `toolCallBlock` (`:1032`) — carry a
    `customInput *struct{ Property string; Buffer grammar.InputJSONBuffer }` on
    the block; on a `custom` delta, append the raw input and push the
    synthesized JSON delta; on tool-call end, close the buffer and push the
    trailing `"}`.
  - `rawToolCallDelta` (`:1275`) — decode `custom: {name, input}` beside
    `function`, and resolve a delta's tool name from either
    (`toolCall.function?.name ?? toolCall.custom?.name`).
  - Error path — clear the `customInput` scratch state on the assembled blocks,
    beside the existing `partialArgs`/`index` cleanup.
- Tests, offline `httptest`, in
  `ai/apis/openaicompletions/grammar_test.go` (new focused file, per
  [CONVENTIONS.md](../../../planning/CONVENTIONS.md)'s one-concern-per-test-file
  habit).
- `// Ports:` headers on every touched file stay accurate; extend the symbol
  lists where new upstream symbols arrived.

## Out of scope

- The `openai-responses` custom-tool path — issues 06 and 08. The two wire
  shapes differ (`custom_tool_call` vs `type:"custom"` tool calls); do not try
  to share code across the packages in this PR.
- `deferredToolsMode` / Kimi tool serialization and the finish-reason inference —
  issue 05, even though both touch `buildParams`.
- Thinking-token budgets and chat-template values — issue 04.

## Acceptance criteria / Definition of done

- [ ] `TestConvertToolsEmitsCustomGrammarTool` — a tool with
      `constrainedSampling.type == "grammar"` and a lark variant, against a model
      whose compat sets `SupportsOpenAIGrammarTools: true`, serializes to
      `{"type":"custom","custom":{"name":…,"format":{"type":"grammar","grammar":{"syntax":"openai_lark","definition":…}}}}`
      with no `function` key.
- [ ] `TestConvertToolsFallsBackToFunctionWhenGrammarUnsupported` — the same tool
      with the flag false serializes as a `function` tool.
- [ ] `TestConvertToolsStrictFromConstrainedSampling` — a `json_schema` config
      with `strict: "prefer"` against `supportsStrictMode` true emits
      `"strict": true`; a tool with no config still emits `"strict": false`.
- [ ] `TestReplayGrammarToolCallSendsCustomInput` — an assistant message holding
      a prior grammar tool call replays as
      `{"type":"custom","custom":{"name":…,"input":"<raw>"}}`, and the raw input
      is the value of the tool's single required property, not JSON.
- [ ] `TestStreamCustomToolCallSynthesizesJSONDeltas` — an `httptest` SSE stream
      delivering `custom.input` in three chunks produces `toolcall_delta` events
      whose concatenation is exactly `{"query":"<chunk1><chunk2><chunk3>"}`, and
      the final `ToolCall.Arguments` decodes to that object.
- [ ] `TestStreamCustomToolCallInputIsNotPersisted` — after a stream (success and
      error path both), the assistant message's tool-call block carries no
      grammar scratch state.
- [ ] Ported upstream coverage: the grammar cases of
      `test/openai-completions-tool-choice.test.ts` and
      `test/constrained-sampling.test.ts` that exercise the completions wire
      shape come across as discrete Go tests.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(apis): send and stream grammar custom tools in openai-completions`.

## Relevant files / areas

- `ai/apis/openaicompletions/openaicompletions.go` (1388 lines) — `:265` wire
  tool types, `:702` `convertTools`, `:767` `convertMessages`, `:850`
  `convertAssistantMessage`, `:1032` `toolCallBlock`, `:1045` `decodeEvents`,
  `:1275` `rawToolCallDelta`.
- `ai/apis/openaicompletions/compat.go:189` — where the resolved compat struct
  gains `SupportsOpenAIGrammarTools` / `SupportsStrictMode`
  ([Epic 2 issue 04](/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md)).
- `ai/apis/internal/grammar` — issue 01's package.
- Upstream: `src/api/openai-completions.ts` at `936aff00`, hunks around
  `convertTools`, `convertMessages`, and the streaming tool-call block.

## Dependencies

- **Blocked by**: [Issue 01](/epic-4-openai-family-adapters/issues/01-constrained-sampling-core.md);
  [Epic 2 issue 04](/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md)
  (`SupportsOpenAIGrammarTools`, called at `run` via
  `grammar.InputProperties(chat.Tools, compat.SupportsOpenAIGrammarTools)`).
  Cross-epic, so it is stated here rather than in `depends_on`, which holds
  intra-epic numbers only.
- **Blocks**: [Issue 05](/epic-4-openai-family-adapters/issues/05-completions-deferred-tools-and-finish-reason.md)
  (same `buildParams`/`convertTools` call sites).

## PR size note

`M` — ~480 changed lines: eight call sites inside the 1388-line
`openaicompletions.go` (`wireTool`, `wireToolCall`, `convertTools`,
`convertAssistantMessage`, `toolCallBlock`, `decodeEvents`, `rawToolCallDelta`
and the error path) plus a new `grammar_test.go` holding six named tests and
the ported grammar cases of two upstream suites. It sits near the top of `M`;
split past ~500, and the seam is the request-side wire shapes apart from the
streamed JSON-delta synthesis.
