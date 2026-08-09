---
type: Issue
title: "Port utils/deferred-tools.ts as the deferred-tool split, with the upstream test"
description: "Add SplitDeferredTools, which partitions a context's tools into immediate and transcript-loaded sets, and port upstream's test for it."
tags: [epic-2]
timestamp: 2026-08-09T04:31:17Z
epic: 2
issue: 11
slug: deferred-tools-split
size: M
status: open
gh_issue: 139
resource: https://github.com/kern-ia/kern-link/issues/139
depends_on: [2]
---

# Port utils/deferred-tools.ts as the deferred-tool split, with the upstream test

## Summary

`src/utils/deferred-tools.ts` is a pure function that partitions a request's
tools into those sent up front and those a provider loads later from the
transcript:

```ts
export function splitDeferredTools(
	context: Context, enabled: boolean, normalizeName = (n: string) => n,
): { immediate: Tool[]; deferred: Map<string, Tool> }
```

It walks the messages, collecting tool names actually *used* by assistant
`toolCall` blocks and names *announced* by
`ToolResultMessage.addedToolNames`; an announced-but-unused name becomes
deferred, everything else immediate. Tools are deduplicated by normalized name
first, so a normalizer that collapses two spellings keeps only the last.

Despite the shared word, this is a different capability from deferred
*responses* ([issue 10](/epic-2-core-types-and-models-contracts/issues/10-deferred-response-dispatch.md)):
this one is about tool definitions, that one about durable response handles.
Both land in this epic because
[decision 11](../../../planning/scope/11-deferred-tools.md) says to port the
capability in full rather than shipping inert types.

## Scope

- New `ai/deferredtools.go` (or `ai/internal/…` if the sweep shows no consumer
  outside `ai/apis` — an implementer call, recorded in `docs/PORTING.md`) with a
  `// Ports: packages/ai/src/utils/deferred-tools.ts` header and
  `SplitDeferredTools(chat Context, enabled bool, normalizeName func(string) string) (immediate []Tool, deferred map[string]Tool)`.
  - A nil `normalizeName` behaves as identity (upstream's default parameter).
  - When `enabled` is false, every unique tool is immediate and `deferred` is
    empty — not nil-vs-empty ambiguity; pick one, document it, test it.
  - Deduplication happens **before** the enabled check, and last-write-wins by
    normalized name, matching upstream's `uniqueTools.set(...)` loop.
  - Iteration order: upstream returns `immediate` in `Map` insertion order.
    Go maps do not preserve insertion order, so the dedup must keep an explicit
    order slice for `immediate` to be deterministic. **This is the one place a
    naive port silently diverges** — a non-deterministic tool order changes the
    request payload and breaks prompt caching.
- Port `test/deferred-tools.test.ts` (21 KB upstream). Translate the cases that
  exercise `splitDeferredTools` itself into discrete named Go tests; cases that
  exercise an adapter's deferred-tool serialization belong to epics 4 and 5 —
  list those in the PR body so they are not lost.

## Out of scope

- **`Compat.DeferredToolsMode` (`"kimi"`)** — the serialization mode flag ships
  in [issue 04](/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md),
  and the serialization itself in
  [Epic 4](/epic-4-openai-family-adapters/EPIC_4.md).
- Anthropic `tool_reference` blocks and the `SupportsToolReferences` compat flag
  — [Epic 5](/epic-5-remaining-adapters/EPIC_5.md).
- OpenAI Responses `additional_tools` / tool search — epic 4.
- Any adapter calling `SplitDeferredTools`. This PR ships the function and its
  tests; the callers are epics 4 and 5, and the PR body says so.

## Acceptance criteria / Definition of done

- [ ] `TestSplitDeferredToolsDisabledReturnsAllImmediate` — with `enabled`
      false, every unique tool is immediate regardless of `AddedToolNames`.
- [ ] `TestSplitDeferredToolsDefersAnnouncedUnusedTools` — a tool named in a
      `ToolResultMessage.AddedToolNames` and never called lands in `deferred`.
- [ ] `TestSplitDeferredToolsKeepsUsedToolsImmediate` — a tool that is both
      announced and later called by an assistant `ToolCall` stays immediate.
- [ ] `TestSplitDeferredToolsDeduplicatesByNormalizedName` — two tools whose
      names normalize identically collapse to one, the last one winning.
- [ ] `TestSplitDeferredToolsPreservesImmediateOrder` — for a fixed input, the
      `immediate` slice is in first-declaration order and is byte-identical
      across 100 runs (guard against map-iteration nondeterminism explicitly).
- [ ] `TestSplitDeferredToolsNilNormalizerIsIdentity`.
- [ ] The PR body lists every upstream `deferred-tools.test.ts` case, marked
      *ported here* or *belongs to epic N because …*.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2).
- [ ] `gofmt -l .` prints nothing; tests are stdlib-only, discrete named
      functions, no `t.Parallel()`
      ([CONVENTIONS.md](../../../planning/CONVENTIONS.md), Testing).
- [ ] Conventional Commit, e.g. `feat(ai): split deferred tools out of a context`.

## Relevant files / areas

- `ai/types.go:229-254` — `ToolResultMessage` (gaining `AddedToolNames` in
  [issue 02](/epic-2-core-types-and-models-contracts/issues/02-message-model-deferred-fields.md)),
  `Tool`, `Context`.
- `ai/types.go:56-70` — `ContentPart` / `AssistantContentPart`, how a `ToolCall`
  block is recognized when walking assistant content.
- `ai/apis/transform.go` — existing tool-name normalization precedent; check
  whether a normalizer already exists before writing a second one.
- Upstream:
  ```bash
  gh api "repos/earendil-works/pi/contents/packages/ai/src/utils/deferred-tools.ts?ref=936aff00918de1187f085f123c2812d8f2d67745" -H "Accept: application/vnd.github.raw"
  gh api "repos/earendil-works/pi/contents/packages/ai/test/deferred-tools.test.ts?ref=936aff00918de1187f085f123c2812d8f2d67745" -H "Accept: application/vnd.github.raw"
  ```

## Dependencies

- **Blocked by**: [Issue 02](/epic-2-core-types-and-models-contracts/issues/02-message-model-deferred-fields.md)
  — `AddedToolNames` is this function's entire input.
- **Blocks**: Nothing inside this epic; epics 4 and 5 call it.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR. The function is ~60 lines; the ported test is the bulk.
