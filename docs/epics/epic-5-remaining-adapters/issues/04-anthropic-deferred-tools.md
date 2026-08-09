---
type: Issue
title: "anthropic: deferred tools via defer_loading and tool_reference blocks"
description: "Split the tool list with SplitDeferredTools, send withheld tools with defer_loading, and load them at their tool-result markers as tool_reference blocks with displaced sibling content."
tags: [epic-5]
timestamp: 2026-08-09T09:45:42Z
epic: 5
issue: 04
slug: anthropic-deferred-tools
size: L
status: open
gh_issue: 162
resource: https://github.com/kern-ia/kern-link/issues/162
depends_on: [3]
---

# anthropic: deferred tools via defer_loading and tool_reference blocks

## Summary

Anthropic's half of the deferred-tools capability. Epic 2
[issue 11](/epic-2-core-types-and-models-contracts/issues/11-deferred-tools-split.md)
ships `SplitDeferredTools` with no callers and explicitly assigns
"Anthropic `tool_reference` blocks and the `SupportsToolReferences` compat flag"
to this epic. This is that issue.

The mechanism: tools a transcript *announces* but has not yet *used* are sent
with `defer_loading: true`, and the model is told to load one at the exact
tool-result message that announced it, via a `tool_reference` content block.

Four pieces:

1. **The split.** `buildParams` transforms messages **first** (upstream hoisted
   `transformMessages` out of `convertMessages` for exactly this reason), then
   calls `splitDeferredTools({...context, messages: transformedMessages},
   compat.supportsToolReferences, normalizeToolName)`, where `normalizeToolName`
   is `toClaudeCodeName` under OAuth and identity otherwise. **If every tool
   ends up deferred, they all become immediate** — Anthropic rejects a request
   whose entire tool list is deferred:

   ```ts
   if (immediateTools.length === 0 && deferredTools.length > 0) { immediateTools = deferredTools; deferredTools = []; }
   ```

2. **Two `convertTools` passes.** Immediate tools keep the `cache_control`
   breakpoint; deferred tools are appended after them with
   `deferLoading = true` (→ `"defer_loading": true`) and **no** cache control.
   Order matters: the cache breakpoint sits on the last *immediate* tool, not
   the last tool overall.

3. **`tool_reference` blocks in tool results.** For each `toolResult` message,
   every name in `msg.addedToolNames` that is deferred **and not already
   loaded** becomes `{type: "tool_reference", tool_name: <normalized>}`. A
   result that emits references replaces its own content with the reference
   array — *Anthropic rejects tool references mixed with ordinary tool-result
   content* — and the displaced content is re-appended as **sibling content
   after every `tool_result` block** in the batched user message:

   ```ts
   params.push({ role: "user", content: [...toolResults, ...siblingContent] });
   ```

   `loadedToolNames` is per-conversion and dedupes across the whole message
   list, so a name announced twice loads once.

4. **`supportsToolReferences` and its conditional default.** Epic 2
   [issue 04](/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md)
   ships the *field* and deliberately leaves the model-by-model defaulting here.
   Upstream's `defaultSupportsToolReferences` is:

   ```ts
   if (model.provider !== "anthropic" || model.id.includes("haiku")) return false;
   const version = model.id.match(/^claude-(?:opus|sonnet|fable)-(\d+)(?:-(\d+))?(?:-|$)/);
   if (!version) return false;
   const major = Number(version[1]);
   const minor = version[2] && version[2].length < 8 ? Number(version[2]) : 0;
   return major > 4 || (major === 4 && minor >= 5);
   ```

   The `version[2].length < 8` guard exists because `claude-opus-4-20250514`
   would otherwise parse its **date** as the minor version. Port the regex and
   that guard exactly; it is the kind of detail a "cleaner" rewrite silently
   drops.

## Scope

- `ai/apis/anthropic/anthropic.go`:
  - `buildParams` (`:768`) — hoist `apis.TransformMessages` out of
    `convertMessages` (`:984`, currently calls it internally) so the split and
    the conversion see the same transformed list. Then split, apply the
    all-deferred fallback, build `deferredToolNames` as a set of **normalized**
    names, and pass both into the converters.
  - `convertMessages` (`:984`) — takes the already-transformed messages plus
    `deferredToolNames` and `normalizeToolName`; stops calling
    `TransformMessages` itself.
  - `convertTools` (`:932`) — gains a `deferLoading bool` parameter emitting
    `"defer_loading": true`, and is called twice from `buildParams`. `wireTool`
    (`:733`) gains `DeferLoading bool \`json:"defer_loading,omitempty"\``.
  - `toolResultBlock` (`:1142`) and the batching loop (`:1012-1030`) — the
    reference/sibling split. Upstream restructured the loop so the current
    message is handled *inside* the look-ahead (`let j = i` rather than
    pushing the first result then starting at `i + 1`); mirror that, since the
    sibling-content accumulation depends on it.
  - A `supportsToolReferences(model)` helper beside `allowEmptySignature`
    (`:921`), returning `Compat.SupportsToolReferences` when set and the ported
    `defaultSupportsToolReferences` otherwise.
- `ai.ToolResultMessage.AddedToolNames` must be readable here (Epic 2
  [issue 02](/epic-2-core-types-and-models-contracts/issues/02-message-model-deferred-fields.md)).
- Tests, offline, in `ai/apis/anthropic/deferred_tools_test.go`.
- `// Ports:` headers stay accurate.

## Out of scope

- The `SplitDeferredTools` function itself and its own unit tests — Epic 2
  [issue 11](/epic-2-core-types-and-models-contracts/issues/11-deferred-tools-split.md).
- Kimi deferred-tool serialization and OpenAI `additional_tools` / tool search —
  [Epic 4](/epic-4-openai-family-adapters/EPIC_4.md) issues 05 and 07. The wire
  shapes are unrelated; do not try to share code across the packages.
- Strict tools — issue 03, which this PR rebases onto.
- Deferred *responses* (`StopReason` `deferred`, `DeferredHandle`) — a different
  capability that happens to share a word; Epic 2 issues 10 and 12.

## Acceptance criteria / Definition of done

- [ ] `TestDeferredToolsSentWithDeferLoading` — a context whose transcript
      announces a tool it never used, against a model with
      `SupportsToolReferences: true`, serializes that tool after the immediate
      ones with `"defer_loading": true`, and the `cache_control` breakpoint sits
      on the last **immediate** tool.
- [ ] `TestToolReferenceEmittedAtItsMarker` — the tool-result message carrying
      the `AddedToolNames` entry serializes as
      `{"type":"tool_result","tool_use_id":…,"content":[{"type":"tool_reference","tool_name":…}]}`.
- [ ] `TestToolReferenceDisplacesResultContentToSibling` — that result's
      original text arrives as a sibling block **after** every `tool_result` in
      the same user message, not inside it.
- [ ] `TestToolReferenceLoadsOnce` — a name announced by two separate tool
      results yields exactly one `tool_reference`.
- [ ] `TestAllDeferredToolsFallBackToImmediate` — when every tool is deferred,
      none carries `defer_loading` and the request is a plain tool list.
- [ ] `TestToolReferencesDisabledUsesNormalToolList` — `SupportsToolReferences:
      false` produces today's exact wire bytes.
- [ ] `TestOAuthNamesNormalizedBeforeDeferralCheck` — under an OAuth token, a
      marker naming the pre-canonicalization tool name still matches the
      `toClaudeCodeName`-normalized active tool.
- [ ] `TestDefaultSupportsToolReferences` — table-driven over model ids (this is
      a pure input→output classifier, the one shape
      [CONVENTIONS.md](../../../planning/CONVENTIONS.md) reserves tables for):
      `claude-sonnet-4-5-…` true, `claude-opus-4-20250514` **false** (the date
      must not read as minor 20250514), `claude-opus-4-1-…` false,
      `claude-haiku-4-5-…` false, `claude-fable-5-…` true, a non-`anthropic`
      provider false, an unmatched id false.
- [ ] Ported upstream coverage: the Anthropic cases of
      `test/deferred-tools.test.ts` (lines 188–343 at `936aff00` — "loads an
      Anthropic tool at its tool-result marker", "preserves tool output as
      sibling content after emitting references", "does not resurrect a marked
      tool missing from Context.tools", "keeps a tool immediate when it was used
      before its marker", "normalizes OAuth names before checking prior tool
      usage", "matches OAuth-canonicalized markers to active tools",
      "deduplicates active tools after OAuth canonicalization", "uses the normal
      tool list when Anthropic tool references are unsupported", "keeps one
      immediate Anthropic tool when every current tool is marked", "supports
      explicit Anthropic compatibility overrides") come across as discrete Go
      tests.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(apis): load deferred anthropic tools through tool_reference blocks`.

## Relevant files / areas

- `ai/apis/anthropic/anthropic.go` — `:768` `buildParams`, `:807` the
  `convertTools` call, `:932` `convertTools`, `:984` `convertMessages`,
  `:1012-1030` the tool-result batching loop, `:1142` `toolResultBlock`, `:733`
  `wireTool`, `:88` `toClaudeCodeName`.
- `ai/apis/transform.go` — `apis.TransformMessages`, which moves out of
  `convertMessages`.
- The `SplitDeferredTools` symbol from Epic 2
  [issue 11](/epic-2-core-types-and-models-contracts/issues/11-deferred-tools-split.md)
  (its final package is that issue's call — check before importing).
- `ai/model.go` — `Compat.SupportsToolReferences`.
- Upstream: `src/api/anthropic-messages.ts` at `936aff00` —
  `defaultSupportsToolReferences`, `buildParams`, `convertToolResult`,
  `convertMessages`, `convertTools`.
- Upstream test: `test/deferred-tools.test.ts:188-343`.

## Dependencies

- **Blocked by**: [Issue 03](/epic-5-remaining-adapters/issues/03-anthropic-strict-tools-and-signed-thinking.md)
  (same `convertTools` signature and `wireTool` struct); Epic 2
  [issue 11](/epic-2-core-types-and-models-contracts/issues/11-deferred-tools-split.md)
  (`SplitDeferredTools`), [issue 02](/epic-2-core-types-and-models-contracts/issues/02-message-model-deferred-fields.md)
  (`AddedToolNames`), [issue 04](/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md)
  (`SupportsToolReferences`).
- **Blocks**: None.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR. Sized **L** because the four pieces are one wire contract — the split, the
two-pass tool emission, the reference blocks and the sibling displacement are
mutually unobservable if landed separately, and a half-ported version sends
Anthropic a request it rejects. Splitting it would mean shipping a knowingly
broken intermediate state, which is why it stays whole; if it starts trending
past ~1000 lines, the honest cut is to land the split + `defer_loading` emission
first (with `SupportsToolReferences` forced false so nothing changes on the
wire) and the reference blocks second.
