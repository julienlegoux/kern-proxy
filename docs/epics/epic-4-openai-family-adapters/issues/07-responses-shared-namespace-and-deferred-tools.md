---
type: Issue
title: "openai-responses shared: tool namespaces and transcript-loaded deferred tools"
description: "Replay ToolCall.namespace when the model can accept it, and re-announce transcript-loaded tools as additional_tools or as a synthetic tool_search call/output pair."
tags: [epic-4]
timestamp: 2026-08-11T15:00:00Z
epic: 4
issue: 07
slug: responses-shared-namespace-and-deferred-tools
size: M
status: open
gh_issue: 153
resource: https://github.com/kern-ia/kern-link/issues/153
depends_on: [6]
---

# openai-responses shared: tool namespaces and transcript-loaded deferred tools

## Summary

The second half of the shared responses converters' 0.84.1 update: how a
transcript that *acquired* tools mid-conversation is replayed.

1. **Namespace replay.** `ai.ToolCall` gains `Namespace`
   ([Epic 2 issue 02](/epic-2-core-types-and-models-contracts/issues/02-message-model-deferred-fields.md)).
   A replayed tool call carries it only when `canReplayNamespace` holds — the
   assistant message came from the *same* model (`isSameModel`, the predicate
   issue 06 introduces), **or** the tool is one of this request's deferred
   tools. Replaying a namespace a different model never issued makes the request
   invalid, so the guard is the feature.
2. **Deferred tool loading.** After a tool result whose `addedToolNames`
   announces tools that `splitDeferredTools` withheld, the converter injects the
   tool definitions into the transcript, in one of two modes:
   - `additional-tools` — a single
     `{"type":"additional_tools","role":"developer","tools":[…]}` item.
   - `tool-search` — a synthetic pair: a `tool_search_call` with
     `call_id = "pi_tool_load_" + shortHash(toolCallId + ":" + names.join(","))`,
     `execution: "client"`, `status: "completed"`, and
     `arguments: {query: <names joined by space>, limit: <count>}`; followed by a
     `tool_search_output` with the same `call_id` and the tool definitions
     converted with `deferLoading: true` (issue 06's flag).
   Each tool is injected **once** per conversion — a `loadedToolNames` set
   guards repeats across multiple tool results.

The mode itself is chosen by each adapter from its compat flags
(`supportsAdditionalTools` → `additional-tools`, else `supportsToolSearch` →
`tool-search`, else off); this PR takes it as a converter option, and issues 09
and 11 wire it.

## Scope

- `ai/apis/openairesponses/messages.go`:
  - `ConvertMessagesOptions` (`:118`) — add `DeferredTools map[string]ai.Tool`,
    `DeferredToolsMode string` (`""`, `"additional-tools"`, `"tool-search"`),
    and `ToolOptions ConvertToolsOptions` (forwarded to `ConvertTools` for the
    injected definitions).
  - `convertAssistantMessage` (`:218`) — carry `Namespace` onto both the
    `function_call` and `custom_tool_call` items, gated on `canReplayNamespace`.
    Emit the key only when the namespace is non-empty (`omitempty`), matching
    upstream's conditional spread.
  - `convertToolResultMessage` (`:284`) / `ConvertMessages` (`:133`) — the
    injection, with the once-only `loadedToolNames` set living in
    `ConvertMessages` (it spans messages, not one message).
  - **Determinism**: the injected tool list must follow the order the names
    appear in `addedToolNames`, not map order. Same trap as
    [Epic 2 issue 11](/epic-2-core-types-and-models-contracts/issues/11-deferred-tools-split.md)
    and issue 05 — an unstable order changes the payload and breaks prompt
    caching.
  - New wire structs for `additional_tools`, `tool_search_call`,
    `tool_search_output`.
  - `pi_tool_load_` id derivation uses `ai.ShortHash` (`ai/hash.go:14`), which
    is documented as hash-compatible with upstream — assert one known
    input→output pair in a test so a future hash change is caught here rather
    than at a provider.
- Port `test/openai-responses-namespace.test.ts` (+224 new) and the deferred-tool
  cases of `test/deferred-tools.test.ts` that exercise the responses transcript
  shape.
- Keep the `// Ports:` header's symbol list current.

## Out of scope

- `ai.SplitDeferredTools` itself —
  [Epic 2 issue 11](/epic-2-core-types-and-models-contracts/issues/11-deferred-tools-split.md).
  This PR consumes the `deferred` map it returns.
- Choosing the mode from compat flags — issues 09 (openai-responses) and 11
  (codex). Azure does not pass a mode at all (issue 10).
- The chat-completions Kimi deferred-tool path — issue 05; a different mechanism
  with a different wire shape.
- Stream-side handling of namespaced tool calls — issue 08.

## Acceptance criteria / Definition of done

- [ ] `TestNamespaceReplayedForSameModel` — an assistant message with matching
      provider/api/model replays its tool call with `"namespace":"…"`.
- [ ] `TestNamespaceDroppedForDifferentModel` — same provider and api, different
      model id, tool not deferred → no `namespace` key.
- [ ] `TestNamespaceReplayedForDeferredTool` — different model, but the tool is
      in `DeferredTools` → the namespace survives.
- [ ] `TestAdditionalToolsInjectedAfterToolResult` — a tool result announcing
      `search` emits an `{"type":"additional_tools","role":"developer"}` item
      immediately after the `function_call_output`, containing exactly that tool.
- [ ] `TestToolSearchPairInjected` — in `tool-search` mode the same transcript
      emits a `tool_search_call` and a `tool_search_output` sharing one
      `pi_tool_load_<hash>` call id, with `arguments.query` the space-joined
      names, `arguments.limit` the count, and every injected tool carrying
      `"defer_loading": true`.
- [ ] `TestDeferredToolInjectedOnlyOnce` — two tool results announcing the same
      tool inject it once.
- [ ] `TestDeferredToolInjectionOrderIsStable` — repeated conversion of the same
      context produces byte-identical output.
- [ ] `TestPiToolLoadIDMatchesUpstreamHash` — one fixed `(toolCallId, names)`
      pair produces the exact id upstream produces (compute it once from
      upstream's `shortHash` and pin it).
- [ ] `docs/PORTING.md`: the `openai-responses-shared.ts` row stays `ported`,
      unchanged — this PR adds symbols to an already-ported file without
      altering its disposition or its parenthetical.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(apis): replay tool namespaces and load deferred tools in responses`.

## Relevant files / areas

- `ai/apis/openairesponses/messages.go:118` `ConvertMessagesOptions`, `:133`
  `ConvertMessages`, `:218` `convertAssistantMessage`, `:284`
  `convertToolResultMessage`.
- `ai/hash.go:14` `ShortHash`.
- `ai/types.go` — `ToolCall.Namespace`, `ToolResultMessage.AddedToolNames`
  (Epic 2 issue 02).
- Upstream: `src/api/openai-responses-shared.ts` (`loadedToolNames`,
  `canReplayNamespace`, the `additional_tools` / `tool_search_*` injection) and
  `test/openai-responses-namespace.test.ts` at `936aff00`.

## Dependencies

- **Blocked by**: [Issue 06](/epic-4-openai-family-adapters/issues/06-responses-shared-grammar-and-tool-results.md);
  [Epic 2 issue 02](/epic-2-core-types-and-models-contracts/issues/02-message-model-deferred-fields.md)
  and [issue 11](/epic-2-core-types-and-models-contracts/issues/11-deferred-tools-split.md).
- **Blocks**: [Issue 09](/epic-4-openai-family-adapters/issues/09-openai-responses-compat-and-wiring.md),
  [Issue 11](/epic-4-openai-family-adapters/issues/11-codex-request-body-and-stop-reasons.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
