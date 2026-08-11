---
type: Issue
title: "openai-responses shared: grammar custom tools and the tool-result output refactor"
description: "Teach the shared responses converters the custom_tool_call / custom_tool_call_output wire shapes, the defer_loading flag, and one unified tool-result output builder."
tags: [epic-4]
timestamp: 2026-08-11T15:00:00Z
epic: 4
issue: 06
slug: responses-shared-grammar-and-tool-results
size: M
status: open
gh_issue: 152
resource: https://github.com/kern-ia/kern-link/issues/152
depends_on: [1]
---

# openai-responses shared: grammar custom tools and the tool-result output refactor

## Summary

`ai/apis/openairesponses/messages.go` is the shared core behind the responses,
Azure, and Codex adapters — `docs/PORTING.md` records its converters as exported
for exactly that reason. This PR brings its **request-side** conversion up to
`936aff00`; the response side is issue 08 and the namespace/deferred-tool half
is issue 07.

Three things change:

1. **Grammar tools in `ConvertTools`** (`messages.go:96`). A grammar-resolved
   tool becomes a flat custom tool —
   `{"type":"custom","name","description","format":{"type":"grammar","syntax","definition"}}`
   — note this is a *different* shape from chat completions' nested
   `custom.format.grammar` (issue 03). Function tools keep `strict`, but only
   when `supportsStrictMode` (default **true** here), and its value now comes
   from `resolveJsonSchemaStrictSampling` falling back to the caller's default.
   A new `deferLoading` option adds `"defer_loading": true` to either shape —
   issue 07 is its only caller, but the flag belongs to the converter.
2. **Tool call replay** (`convertAssistantMessage`, `messages.go:218`). A tool
   call whose name is in the grammar map replays as `custom_tool_call` carrying
   `input` (the raw grammar string, surrogate-sanitized) instead of
   `function_call` carrying JSON `arguments`. The item-id rule widens with it:
   drop the item id when replaying a *non*-grammar call whose item id is not
   `fc_*` (a `ctc_*` custom-tool id would otherwise fail OpenAI's pairing
   validation), on top of the existing different-model rule.
3. **Tool result output** (`convertToolResultMessage`, `messages.go:284`). The
   inline text/image branching becomes one helper returning either a string or a
   content-part array, with the fallbacks made explicit: text if any,
   `"(see attached image)"` when images exist but the model has no image input,
   `"(no tool output)"` when there is nothing. The wrapper item is
   `custom_tool_call_output` when the tool is a grammar tool and
   `function_call_output` otherwise.

## Scope

- `ai/apis/openairesponses/messages.go`:
  - `ConvertToolsOptions` (`:89`) — add `SupportsStrictMode *bool` (nil = true),
    `SupportsOpenAIGrammarTools bool`, `DeferLoading bool`, keeping the existing
    `Strict` default semantics (unset = `false`).
  - `ConvertTools` (`:96`) — the grammar branch, then the function branch with
    the resolved strict value; `strict` is omitted entirely when
    `supportsStrictMode` is false.
  - `ConvertMessagesOptions` (`:118`) — add
    `GrammarToolInputProperties map[string]string` (issue 07 adds the rest).
  - `wireTool` (`:80`), `wireFunctionCall` (`:57`), `wireFunctionCallOutput`
    (`:65`) — add the custom-tool and custom-tool-call/-output variants.
  - `convertAssistantMessage` (`:218`) — the `custom_tool_call` branch and the
    widened item-id drop rule. Keep `isDifferentModel` behavior, but note
    upstream restructured the predicate into `isSameProviderAndApi` /
    `isSameModel` / `isDifferentModel`; port the restructure, since issue 07
    needs `isSameModel`.
  - `convertToolResultMessage` (`:284`) — extract the output builder, add the
    `custom_tool_call_output` wrapper.
  - Grammar resolution errors surface to the caller (each adapter's `run`) as an
    error, not a panic.
- Tests in `ai/apis/openairesponses/messages_test.go` (currently 107 lines) or a
  new focused `grammar_test.go`.
- `docs/PORTING.md` — the `openai-responses-shared.ts` row's parenthetical still
  says "converters exported for the Azure and Codex variants"; extend it if the
  exported surface grows.
- Keep the `// Ports:` header's symbol list current.

## Out of scope

- **Namespace replay, `additional_tools`, `tool_search_call`/`tool_search_output`
  and the deferred-tool machinery** — issue 07, even though they land in the same
  two functions. Split deliberately: this PR is the wire shapes, that one is the
  transcript rewriting.
- **Stream decoding of custom tool calls** — issue 08 (`stream.go`).
- Per-adapter wiring (which compat flag feeds `supportsOpenAIGrammarTools`) —
  issues 09 (responses), 10 (azure), 11 (codex). This PR changes no adapter's
  behavior on its own; the options default to off.

## Acceptance criteria / Definition of done

- [ ] `TestConvertToolsEmitsFlatCustomGrammarTool` — the emitted JSON is
      `{"type":"custom","name":…,"format":{"type":"grammar","syntax":"openai_lark","definition":…}}`
      with no nested `custom` object and no `parameters`.
- [ ] `TestConvertToolsOmitsStrictWhenUnsupported` — `SupportsStrictMode` false
      produces a function tool with **no** `strict` key; the default (unset)
      still emits `"strict": false`.
- [ ] `TestConvertToolsDeferLoading` — `DeferLoading` adds
      `"defer_loading": true` to both the function and custom shapes.
- [ ] `TestReplayGrammarToolCallAsCustomToolCall` — an assistant tool call for a
      grammar tool replays as `{"type":"custom_tool_call","call_id":…,"name":…,"input":"<raw>"}`.
- [ ] `TestReplayDropsNonFunctionItemID` — a non-grammar tool call whose
      composite id carries a `ctc_…` item id replays with no `id` field, while an
      `fc_…` item id survives.
- [ ] `TestToolResultOutputFallbacks` — three cases in one named test each:
      text-only returns the joined string; images with an image-capable model
      return `input_text` + `input_image` parts; images with a text-only model
      return `"(see attached image)"`; empty content returns
      `"(no tool output)"`.
- [ ] `TestGrammarToolResultUsesCustomToolCallOutput` — a tool result for a
      grammar tool wraps in `custom_tool_call_output`, a non-grammar one in
      `function_call_output`.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(apis): convert grammar custom tools in the shared responses core`.

## Relevant files / areas

- `ai/apis/openairesponses/messages.go` (444 lines) — `:57` `wireFunctionCall`,
  `:65` `wireFunctionCallOutput`, `:80` `wireTool`, `:89` `ConvertToolsOptions`,
  `:96` `ConvertTools`, `:118` `ConvertMessagesOptions`, `:133` `ConvertMessages`,
  `:218` `convertAssistantMessage`, `:284` `convertToolResultMessage`, `:331`
  `splitToolCallID`.
- `ai/apis/internal/grammar` — issue 01.
- `ai/apis/openairesponses/messages_test.go` (107 lines).
- `docs/PORTING.md` — the `openai-responses-shared.ts` mapping row.
- Upstream: `src/api/openai-responses-shared.ts` (`convertToolResultOutput`,
  `convertResponsesTools`, the assistant-message branch) at `936aff00`.

## Dependencies

- **Blocked by**: [Issue 01](/epic-4-openai-family-adapters/issues/01-constrained-sampling-core.md).
- **Blocks**: [Issue 07](/epic-4-openai-family-adapters/issues/07-responses-shared-namespace-and-deferred-tools.md),
  [Issue 08](/epic-4-openai-family-adapters/issues/08-responses-shared-stream-decode.md),
  and through them issues 09–12.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
