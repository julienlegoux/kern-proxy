---
type: Issue
title: "anthropic: strict tool schemas and signature-only thinking blocks"
description: "Resolve per-tool strict JSON-schema sampling into the Anthropic tool wire shape behind SupportsStrictTools, and stop dropping an empty thinking block that carries a signature."
tags: [epic-5]
timestamp: 2026-08-10T04:00:00Z
epic: 5
issue: 03
slug: anthropic-strict-tools-and-signed-thinking
size: M
status: open
gh_issue: 161
resource: https://github.com/kern-ia/kern-link/issues/161
depends_on: []
---

# anthropic: strict tool schemas and signature-only thinking blocks

## Summary

The request-converter half of the Anthropic sync, minus deferred tools
(issue 04). Two independent changes that both live in the message/tool
converters:

1. **Strict tool schemas.** `convertTools` now calls
   `resolveJsonSchemaStrictSampling(tool, supportsStrictTools)` per tool. When
   it resolves to `true` the tool gains a top-level `"strict": true` **and** its
   `input_schema` becomes the tool's *full* JSON schema merged with the legacy
   three-key shape:

   ```ts
   const legacyInputSchema = { type: "object", properties: schema.properties ?? {}, required: schema.required ?? [] };
   const inputSchema = strict === true
       ? { ...(tool.parameters as Record<string, unknown>), ...legacyInputSchema }
       : legacyInputSchema;
   ```

   Note the spread order: the legacy keys **win** over the full schema's own
   `type`/`properties`/`required`. Everything else in the tool's schema
   (`$defs`, `additionalProperties`, `$schema`, …) survives only in the strict
   branch. The non-strict branch is byte-identical to today's behavior.
   `supportsStrictTools` is the new `Compat` flag from Epic 2
   [issue 04](/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md),
   defaulting to false.

2. **Signature-only thinking blocks survive.** Upstream's
   `convertMessages` assistant-block branch changed from

   ```ts
   if (block.thinking.trim().length === 0) continue;
   ```

   to keeping the block when it is empty **but carries a signature**:

   ```ts
   const hasThinkingSignature = !!thinkingSignature && thinkingSignature.trim().length > 0;
   if (block.thinking.trim().length === 0 && !hasThinkingSignature) continue;
   ```

   The signature then has to be echoed back to Anthropic; dropping it breaks
   the reasoning chain. The Go port drops it today
   (`anthropic.go:1093`). The sibling Google change is issue 05 — upstream
   explicitly calls the two "the same rule".

## Scope

- `ai/apis/anthropic/anthropic.go`:
  - `wireTool` (`:733`) — add `Strict bool \`json:"strict,omitempty"\``.
  - `wireInputSchema` (`:741`) is a fixed three-field struct and **cannot carry
    the strict branch's merged schema**. Either widen it to
    `json.RawMessage` / `map[string]any` built by the converter, or give
    `wireTool.InputSchema` an `any` type with the legacy struct used in the
    non-strict path. Pick one, and make sure the non-strict wire bytes do not
    change (that is what the existing tests assert).
  - `convertTools` (`:932`) — resolve strict per tool via
    `grammar.ResolveJSONSchemaStrictSampling` (epic 4
    [issue 01](/epic-4-openai-family-adapters/issues/01-constrained-sampling-core.md)'s
    `ai/apis/internal/grammar`), gated on the resolved `SupportsStrictTools`
    compat value. **`resolveJsonSchemaStrictSampling` throws when
    `strict: "require"` meets a provider that cannot do it** — that failure must
    surface as an in-band error event through the existing `ai.LazyStream` path,
    never a panic ([CONVENTIONS.md](../../../planning/CONVENTIONS.md), "Failures
    never escape as panics"). `convertTools` currently returns a value with no
    error channel; changing its signature is expected.
  - A resolved-compat helper beside the existing
    `supportsEagerToolInputStreaming` (`:925`) / `allowEmptySignature` (`:921`)
    for `supportsStrictTools`, default false.
  - `convertAssistantBlocks` (`:1075`), `ai.ThinkingContent` branch (`:1086`) —
    keep the block when `Thinking` is blank but `ThinkingSignature` is not, and
    emit `{"type":"thinking","thinking":"","signature":<sig>}`. The existing
    `AllowEmptySignature` downgrade path (blank signature → plain text, or a
    preserved empty-signature block on marked models) is unchanged; only the
    early `continue` moves.
- Tests, offline, in `ai/apis/anthropic/strict_tools_test.go` and alongside the
  existing `thinking_replay_test.go`-style coverage.
- `// Ports:` headers stay accurate; extend the symbol list where new upstream
  symbols arrived.

## Out of scope

- Deferred tools, `defer_loading`, and `tool_reference` blocks — issue 04, even
  though it also rewrites `convertTools`' signature. This PR lands first; issue
  04 rebases onto it.
- Grammar/custom tools. Anthropic has **no** grammar tool path upstream —
  `anthropic-messages.ts` imports only `resolveJsonSchemaStrictSampling` from
  `constrained-sampling.ts`. Do not port the grammar half here.
- The stream-decoding changes — issue 02.
- The Google sibling of the signature rule — issue 05.

## Acceptance criteria / Definition of done

- [ ] `TestConvertToolsOmitsStrictWhenUnsupported` — a tool with
      `ConstrainedSampling.Strict == "prefer"` against a model whose
      `SupportsStrictTools` is false or unset serializes with **no** `strict`
      key and the legacy three-key `input_schema`, byte-identical to the
      pre-change output.
- [ ] `TestConvertToolsSendsFullSchemaForStrictTools` — the same tool against
      `SupportsStrictTools: true` serializes with `"strict": true` and an
      `input_schema` that retains a `$defs` key from the tool's parameters while
      `type`/`properties`/`required` still come from the legacy shape.
- [ ] `TestConvertToolsRequireStrictOnUnsupportedProviderErrors` — a tool with
      `Strict: "require"` against `SupportsStrictTools: false` ends the stream
      with an `ErrorEvent` (no panic, no silent downgrade).
- [ ] `TestReplaySignedEmptyThinkingBlock` — an assistant message whose only
      thinking block has `Thinking == ""` and a non-empty `ThinkingSignature`
      round-trips as a `thinking` block carrying that signature.
- [ ] `TestReplayUnsignedEmptyThinkingBlockStillDropped` — blank thinking, blank
      signature → the block is dropped, exactly as today.
- [ ] Ported upstream coverage: the strict case of
      `test/anthropic-eager-tool-input-compat.test.ts` ("only sends the full
      input schema for strict JSON-schema tools") comes across as a discrete Go
      test.
- [ ] `docs/PORTING.md`'s row for `src/api/anthropic-messages.ts` still
      describes the Go code after this change (unchanged: already `ported`).
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(apis): send strict tool schemas and replay signed empty thinking blocks in anthropic`.

## Relevant files / areas

- `ai/apis/anthropic/anthropic.go` — `:733` `wireTool`, `:741`
  `wireInputSchema`, `:807` the `convertTools` call in `buildParams`, `:932`
  `convertTools`, `:1075` `convertAssistantBlocks`, `:1086` the thinking branch.
- `ai/apis/internal/grammar` — epic 4
  [issue 01](/epic-4-openai-family-adapters/issues/01-constrained-sampling-core.md)'s
  package; `ResolveJSONSchemaStrictSampling` is the only symbol needed here.
- `ai/model.go` — `Compat.SupportsStrictTools` (Epic 2
  [issue 04](/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md)).
- `ai/apis/anthropic/anthropic_test.go`,
  `ai/apis/anthropic/anthropic_error_mapping_test.go` — existing wire assertions
  that must keep passing unchanged in the non-strict path.
- Upstream: `src/api/anthropic-messages.ts` at `936aff00`, `convertTools` and
  the `thinking` branch of `convertMessages`.

## Dependencies

- **Blocked by**: Epic 4
  [issue 01](/epic-4-openai-family-adapters/issues/01-constrained-sampling-core.md)
  — a **cross-epic** dependency. `resolveJsonSchemaStrictSampling` ports there,
  and this epic's strict-tool issues (03, 05, 07, 09) all call it. The epic file
  says epic 5 is "independent of Epic 4"; that is true of the wire work and
  false of this one function. If epic 4 issue 01 has not merged, do **not**
  fork a second copy — wait, or land the package alone and let epic 4 rebase.
  Epic 2 [issue 04](/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md)
  for `SupportsStrictTools`.
- **Blocks**: [Issue 04](/epic-5-remaining-adapters/issues/04-anthropic-deferred-tools.md)
  (same `convertTools` signature).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
