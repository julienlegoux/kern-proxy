---
type: Issue
title: "google-shared: Gemini 3 tool-call ids, signature-bearing empty blocks, and the VALIDATED function-calling mode"
description: "Require tool-call ids on Gemini 3+, stop dropping empty text and thinking parts that carry a thought signature, and resolve the function-calling mode from strict tool sampling."
tags: [epic-5]
timestamp: 2026-08-10T04:00:00Z
epic: 5
issue: 05
slug: google-shared-converters
size: M
status: open
gh_issue: 163
resource: https://github.com/kern-ia/kern-link/issues/163
depends_on: []
---

# google-shared: Gemini 3 tool-call ids, signature-bearing empty blocks, and the VALIDATED function-calling mode

## Summary

`google-shared.ts` (+82) is the converter layer both Google adapters build on,
so it moves first and issue 06 wires the results into the two `stream`
functions. Three changes:

1. **`requiresToolCallId` now covers Gemini 3+.** It was
   `modelId.startsWith("claude-") || modelId.startsWith("gpt-oss-")`; it now
   also returns true when `getGeminiMajorVersion(modelId) >= 3`. Gemini 3 needs
   explicit ids in function calls and responses.

2. **Empty parts that carry a thought signature are kept.** This is the change
   with a written failure mode attached — upstream's comment:

   > Gemini can attach the signature to a part whose visible text is empty and
   > requires it echoed back; dropping it breaks the reasoning chain and the
   > model intermittently ends mid-task turns with a thought-only STOP (empty
   > completion, no tool call).

   Three distinct rules replace two unconditional skips:
   - **text**: skip only when blank **and** it has no resolved thought
     signature.
   - **thinking, same provider+model**: same rule — and note the signature must
     be resolved *before* the emptiness check, which is a reordering, not just a
     condition change.
   - **thinking, cross provider/model**: the signature is unusable there, so
     blank blocks stay dropped.

   The Anthropic sibling of this rule is
   [issue 03](/epic-5-remaining-adapters/issues/03-anthropic-strict-tools-and-signed-thinking.md);
   upstream's comment calls them "the same rule".

3. **Strict tool sampling picks the function-calling mode.** Two new exports:

   ```ts
   supportsGoogleStrictToolSampling(modelId)  // gemini major version >= 3
   resolveGoogleFunctionCallingMode(tools, toolChoice, supportsStrictMode)
   //   toolChoice "none" | "any"  → mapToolChoice(toolChoice)          (explicit wins)
   //   any tool resolves strict   → VALIDATED
   //   otherwise                  → toolChoice ? mapToolChoice(toolChoice) : undefined
   ```

   `mapToolChoice` is unchanged and still exported.

`retryGoogleRequest` needs **no port**. Upstream added it to wrap the
`@google/genai` SDK call in the shared retry policy, including a shim that
bolts a missing `headers` property onto the SDK's `ApiError`. kern-link has no
SDK: `ai/apis/google/google.go:120` and `vertex.go` already dispatch through
`httpretry.Do` with `DefaultMaxRetries`, which is what the wrapper exists to
achieve. Record that in the PR body and port
`test/google-shared-retry.test.ts`'s three cases as Go tests against
`httpretry` behavior — the retry semantics are now asserted for Google either
way.

## Scope

- `ai/apis/google/messages.go`:
  - `RequiresToolCallID` (`:88`) — add the `geminiMajorVersion(modelID) >= 3`
    clause. `geminiMajorVersion` (`:94`) already exists and returns the sentinel
    this needs; confirm its "not a Gemini model" return value distinguishes
    correctly from major version 0.
  - `ConvertMessages` (`:128`) — the text and thinking branches. Resolve the
    thought signature first (`resolveThoughtSignature`, `:78`), then apply the
    three rules above.
  - New `SupportsStrictToolSampling(modelID string) bool` beside
    `MapToolChoice` (`:381`).
  - New `ResolveFunctionCallingMode(tools []ai.Tool, toolChoice string,
    supportsStrictMode bool) (string, bool)` — the bool (or a `""` sentinel;
    pick one and document it) carries upstream's `undefined`, which means
    "omit `toolConfig` entirely". That distinction is the whole point of the
    change: the old code set `config.toolConfig = undefined` explicitly, the new
    code omits the key.
  - Strict resolution goes through `grammar.ResolveJSONSchemaStrictSampling`
    (epic 4 [issue 01](/epic-4-openai-family-adapters/issues/01-constrained-sampling-core.md)).
    It **throws** on `strict: "require"` against an unsupporting provider; that
    must surface as an in-band error event, never a panic
    ([CONVENTIONS.md](../../../planning/CONVENTIONS.md)). Since
    `ResolveFunctionCallingMode` is called from `buildParams` (issue 06), give
    it an error return rather than swallowing.
- Tests, offline, in `ai/apis/google/messages_test.go` and a new
  `ai/apis/google/signed_empty_blocks_test.go`.
- `// Ports:` headers stay accurate.

## Out of scope

- Wiring `ResolveFunctionCallingMode` into `buildParams` for either adapter, and
  everything stream-side (`pending`, `rawStopReason`, the `max` thinking clamp,
  the custom-fetch decision) — [issue 06](/epic-5-remaining-adapters/issues/06-google-and-vertex-stream-and-params.md).
- `ConvertTools` itself: `test/google-shared-convert-tools.test.ts` changed
  (+18/-…) but the diff of `convertTools` in `google-shared.ts` is empty in
  range — read the test diff before assuming otherwise, and if it turns out to
  assert something the Go port does not do, say so in the PR body rather than
  quietly widening scope.

## Acceptance criteria / Definition of done

- [ ] `TestRequiresToolCallIDForGemini3` — `gemini-3-pro-…` and
      `gemini-3.1-flash-…` require ids; `gemini-2.5-pro` does not; the existing
      `claude-` / `gpt-oss-` cases still do.
- [ ] `TestKeepsSignedEmptyThinkingBlock` — an empty thinking block with a valid
      thought signature, same provider and model, converts to a part carrying
      `thoughtSignature`.
- [ ] `TestKeepsSignedEmptyTextBlock` — the same for an empty text block.
- [ ] `TestDropsUnsignedEmptyBlocks` — blank text and blank thinking with no
      signature are still dropped.
- [ ] `TestDropsSignedEmptyBlocksFromAnotherModel` — a signed empty block whose
      provider/model differs is dropped (the signature is unusable).
- [ ] `TestResolveFunctionCallingModeExplicitChoiceWins` — `toolChoice` `"none"`
      and `"any"` map through `MapToolChoice` even when a tool resolves strict.
- [ ] `TestResolveFunctionCallingModeValidatedForStrictTools` — a strict tool on
      a Gemini 3 model with no `toolChoice` yields `VALIDATED`.
- [ ] `TestResolveFunctionCallingModeOmittedWhenNothingApplies` — no strict
      tool and no `toolChoice` yields the "omit the key" signal, and issue 06's
      `buildParams` must then emit no `toolConfig` at all.
- [ ] Ported upstream coverage: all four cases of
      `test/google-shared-signed-empty-blocks.test.ts` and the three cases of
      `test/google-shared-retry.test.ts` come across as discrete Go tests.
- [ ] `docs/PORTING.md`'s row for `src/api/google-shared.ts` still describes
      the Go code after this change (unchanged: already `ported`).
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `fix(apis): keep signature-bearing empty google parts so the reasoning chain survives`.

## Relevant files / areas

- `ai/apis/google/messages.go` (411 lines) — `:78` `resolveThoughtSignature`,
  `:88` `RequiresToolCallID`, `:94` `geminiMajorVersion`, `:128`
  `ConvertMessages`, `:357` `ConvertTools`, `:381` `MapToolChoice`.
- `ai/apis/google/google.go:120` and `ai/apis/google/vertex/vertex.go` — the
  existing `httpretry.Do` dispatch that makes `retryGoogleRequest` a no-op port.
- `ai/apis/internal/grammar` — epic 4 issue 01's package.
- Upstream: `src/api/google-shared.ts` at `936aff00` — `requiresToolCallId`,
  `convertMessages`, `supportsGoogleStrictToolSampling`,
  `resolveGoogleFunctionCallingMode`, `retryGoogleRequest`.
- Upstream tests: `test/google-shared-signed-empty-blocks.test.ts` (new, 117
  lines), `test/google-shared-retry.test.ts` (new, 40 lines),
  `test/google-shared-convert-tools.test.ts` (+18).

## Dependencies

- **Blocked by**: Epic 4
  [issue 01](/epic-4-openai-family-adapters/issues/01-constrained-sampling-core.md)
  (cross-epic — `ResolveJSONSchemaStrictSampling`; see
  [issue 03](/epic-5-remaining-adapters/issues/03-anthropic-strict-tools-and-signed-thinking.md)'s
  note on why epic 5 is not fully independent of epic 4).
- **Blocks**: [Issue 06](/epic-5-remaining-adapters/issues/06-google-and-vertex-stream-and-params.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
