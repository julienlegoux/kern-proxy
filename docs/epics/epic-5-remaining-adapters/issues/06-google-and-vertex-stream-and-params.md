---
type: Issue
title: "google + vertex: pending and raw stop reasons, toolConfig wiring, and the max thinking level"
description: "Start both Google streams at pending, surface the raw Gemini finish reason in the error text, emit toolConfig from the resolved function-calling mode, and clamp the new max thinking level."
tags: [epic-5]
timestamp: 2026-08-10T04:00:00Z
epic: 5
issue: 06
slug: google-and-vertex-stream-and-params
size: M
status: open
gh_issue: 164
resource: https://github.com/kern-ia/kern-link/issues/164
depends_on: [5]
---

# google + vertex: pending and raw stop reasons, toolConfig wiring, and the max thinking level

## Summary

`google-generative-ai.ts` (+38) and `google-vertex.ts` (+38) changed
**identically** — same four hunks, differing only in the adapter name inside two
error strings. They land in one PR because splitting them would mean writing the
same review twice.

1. **`pending` and `rawStopReason`.** Both streams initialize
   `stopReason: "pending"`, record `output.rawStopReason = candidate.finishReason`
   before mapping, and after the loop:

   ```ts
   if (output.stopReason === "pending") throw new Error("Google stream ended without a finish reason");
   if (output.stopReason === "aborted" || output.stopReason === "error") {
       throw new Error(output.rawStopReason ? `Provider stopped with: ${output.rawStopReason}` : "An unknown error occurred");
   }
   ```

   Vertex's pending message is `"Google Vertex stream ended without a finish
   reason"`. **Both strings are copied verbatim** — `ai/retry.go` and
   `ai/overflow.go` classify by matching error text
   ([CONVENTIONS.md](../../../planning/CONVENTIONS.md)).

2. **`toolConfig` comes from `resolveGoogleFunctionCallingMode`.** The old code
   set `config.toolConfig = undefined` in an else branch; the new code computes
   the mode once (only when `context.tools?.length`) and **omits the key
   entirely** when the resolver returns nothing. Wire issue 05's
   `ResolveFunctionCallingMode` / `SupportsStrictToolSampling(model.ID)` into
   both `buildParams`.

3. **`max` clamps like `xhigh`.** `type ClampedThinkingLevel = Exclude<ThinkingLevel,
   "xhigh" | "max">` in both files — the new `ThinkingMax` level (Epic 2
   [issue 01](/epic-2-core-types-and-models-contracts/issues/01-widen-stopreason-and-thinkinglevel.md))
   must not index the Google budget tables directly.

4. **Custom fetch — a deliberate decision, not a mechanical port.** Upstream
   *rejects* it outright:

   ```ts
   if (options?.fetch && options.fetch !== globalThis.fetch)
       throw new Error("Custom fetch is not supported by the Google Generative AI adapter");
   ```

   because `@google/genai` cannot take one. kern-link has no SDK — it talks raw
   `net/http` — so the constraint does not exist here and `Fetch` is
   *supportable*. **Honor it** (matching epic 4
   [issue 02](/epic-4-openai-family-adapters/issues/02-fetch-and-sampling-params.md),
   which honors `Fetch` for the OpenAI family) rather than porting a rejection
   that only ever encoded an SDK limitation — decided in
   [EPIC_5.md](/epic-5-remaining-adapters/EPIC_5.md)'s `## Notes`. Record it in
   `docs/PORTING.md` as a deviation with that reason: the governing principle is
   that a deviation needs structural grounds, and "upstream's constraint does
   not exist in Go" is exactly that.

## Scope

- `ai/apis/google/google.go`:
  - `run` (`:71`) — `ai.StopReasonPending` initializer; the pending guard and
    the `Provider stopped with: <raw>` error text before the existing
    aborted/error branch (`:145`).
  - `buildParams` (`:249`) — compute the function-calling mode from issue 05's
    resolver; emit `wireToolConfig` (`:209`) only when it resolves. A resolver
    error (strict "require" on an unsupporting model) must reach the caller and
    end the stream in-band.
  - `getThinkingLevel` (`:341`) / `getGoogleBudget` (`:367`) /
    `budgetOverride` (`:387`) — treat `ai.ThinkingMax` exactly as
    `ai.ThinkingXHigh` is treated today.
  - Request path — honor `opts.Fetch` (see the decision above).
- `ai/apis/google/stream.go`:
  - `DecodeStream` (`:61`) — set `output.RawStopReason` from
    `candidate.finishReason` before `MapStopReasonString` (`messages.go:402`).
    `rawCandidate` (`:225`) already decodes the field.
- `ai/apis/google/vertex/vertex.go` — the same four changes at `run` (`:97`),
  `buildParams` (`:263`), `getThinkingLevel` (`:343`) and the request path,
  with `"Google Vertex stream ended without a finish reason"`. Vertex shares
  `ai/apis/google`'s `DecodeStream`, so the `RawStopReason` change lands once.
- Tests, offline `httptest`, in `ai/apis/google/stop_reason_test.go` and
  `ai/apis/google/vertex/vertex_test.go`.
- `// Ports:` headers stay accurate.

## Out of scope

- Everything in `google-shared.ts` — [issue 05](/epic-5-remaining-adapters/issues/05-google-shared-converters.md).
- `retryGoogleRequest`: already satisfied by `httpretry.Do`; issue 05 records
  that and ports the retry test cases.
- ADC / Vertex auth (`ai/apis/google/vertex/auth.go`) — unchanged in range.

## Acceptance criteria / Definition of done

- [ ] `TestGoogleStreamWithoutFinishReasonFails` — an `httptest` stream that
      delivers content but no `finishReason` ends with an `ErrorEvent` whose
      message is exactly `Google stream ended without a finish reason`.
- [ ] `TestVertexStreamWithoutFinishReasonFails` — the same with
      `Google Vertex stream ended without a finish reason`.
- [ ] `TestGoogleRawStopReasonInErrorText` — `finishReason: "SAFETY"` yields
      `RawStopReason == "SAFETY"` and an error message of exactly
      `Provider stopped with: SAFETY`; a mapped-to-error finish reason with no
      raw string still yields `An unknown error occurred`.
- [ ] `TestVertexRawStopReasonInErrorText` — the same against the Vertex
      adapter.
- [ ] `TestGoogleOmitsToolConfigWhenNoModeResolves` — a request with tools, no
      `toolChoice`, and no strict tool carries **no** `toolConfig` key at all
      (assert on the marshalled JSON, not the struct).
- [ ] `TestGoogleSendsValidatedModeForStrictTools` — a strict tool on a Gemini 3
      model sends `toolConfig.functionCallingConfig.mode == "VALIDATED"`; the
      same for Vertex.
- [ ] `TestGoogleClampsMaxThinkingLevel` — `ai.ThinkingMax` produces the same
      request body as `ai.ThinkingXHigh` in both adapters.
- [ ] `TestGoogleHonorsInjectedFetch` — a request with `opts.Fetch` set is
      dispatched through it in both the Google and Vertex adapters, and
      `docs/PORTING.md` records honoring `Fetch` as a deviation with its
      reason.
- [ ] Ported upstream coverage: both cases of
      `test/google-raw-stop-reason.test.ts` come across as discrete Go tests.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `fix(apis): fail the google and vertex streams when they end without a finish reason`.

## Relevant files / areas

- `ai/apis/google/google.go` (452 lines) — `:71` `run`, `:145` the error
  branch, `:209` `wireToolConfig`, `:249` `buildParams`, `:341`
  `getThinkingLevel`, `:367` `getGoogleBudget`.
- `ai/apis/google/stream.go` (245 lines) — `:61` `DecodeStream`, `:225`
  `rawCandidate`.
- `ai/apis/google/vertex/vertex.go` (472 lines) — `:97` `run`, `:235`
  `wireToolConfig`, `:263` `buildParams`, `:343` `getThinkingLevel`.
- `ai/apis/google/messages.go:402` — `MapStopReasonString`.
- Upstream: `src/api/google-generative-ai.ts` and `src/api/google-vertex.ts` at
  `936aff00` — the output initializer, the `finishReason` hunk, the post-loop
  guards, `buildParams`' `toolConfig` block, and `ClampedThinkingLevel`.
- Upstream test: `test/google-raw-stop-reason.test.ts` (new, 106 lines).

## Dependencies

- **Blocked by**: [Issue 05](/epic-5-remaining-adapters/issues/05-google-shared-converters.md)
  (`ResolveFunctionCallingMode`); Epic 2
  [issue 01](/epic-2-core-types-and-models-contracts/issues/01-widen-stopreason-and-thinkinglevel.md)
  (`StopReasonPending`, `ThinkingMax`),
  [issue 02](/epic-2-core-types-and-models-contracts/issues/02-message-model-deferred-fields.md)
  (`RawStopReason`), [issue 13](/epic-2-core-types-and-models-contracts/issues/13-fetch-sampling-and-deferred-options.md)
  (`Fetch`).
- **Blocks**: None.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
