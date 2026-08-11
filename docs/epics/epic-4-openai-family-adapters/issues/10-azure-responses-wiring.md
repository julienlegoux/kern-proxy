---
type: Issue
title: "azure-openai-responses: grammar tools, strict resolution, and the pending stop reason"
description: "Wire Azure onto the updated shared responses core — grammar tool properties, compat-driven strict mode, the pending start state and real error messages — and port the reasoning-replay coverage."
tags: [epic-4]
timestamp: 2026-08-11T15:00:00Z
epic: 4
issue: 10
slug: azure-responses-wiring
size: S
status: open
gh_issue: 156
resource: https://github.com/kern-ia/kern-link/issues/156
depends_on: [6, 8]
---

# azure-openai-responses: grammar tools, strict resolution, and the pending stop reason

## Summary

Azure is the smallest of the four (+52 upstream) because it inherits almost
everything from the shared core. What is left is wiring plus one behavior change:

- Build the grammar input-property map from
  `model.compat.supportsOpenAIGrammarTools` (default false) and pass it to both
  `ConvertMessages` and `DecodeStream`.
- Pass `supportsStrictMode` (default **true** for Azure) and
  `supportsOpenAIGrammarTools` into `ConvertTools`, replacing today's
  no-options call.
- Start the assistant message at `StopReason` `pending`; a stream that ends
  still `pending` fails with `Azure OpenAI Responses stream ended without a stop
  reason`; the failure path surfaces `output.errorMessage` instead of the
  generic `An unknown error occurred`.
- Accept `max` as a reasoning effort.
- Clear the grammar scratch state alongside the existing `partialJson`/`index`
  cleanup in the error path.

Azure deliberately does **not** get deferred tools: upstream passes no
`deferredToolsMode` here. Do not add one.

`test/azure-openai-responses-reasoning-replay.test.ts` (+136) is new upstream
and is the main coverage this PR carries across — it exercises issue 08's
reasoning-signature backfill through the Azure entry point.

## Scope

- `ai/apis/azure/azure.go`:
  - `run` (`:74`) — grammar map construction, `pending` start state, the
    `pending` guard, `errorMessage` propagation, scratch cleanup.
  - `buildParams` (`:257`) — `ConvertMessages` and `ConvertTools` options.
  - `applyReasoning` (`:318`) / `mappedThinkingLevel` (`:358`) — accept `max`.
- `SamplingParams` merging is issue 02's; do not duplicate it here.
- Port `test/azure-openai-responses-reasoning-replay.test.ts` and the Azure
  cases of `test/azure-openai-base-url.test.ts` that changed (+30) — check
  whether the base-URL change is behavioral or test-only before touching
  `baseurl.go`, and say which in the PR body.
- Keep the `// Ports:` header's symbol list current.

## Out of scope

- Deferred tools / `additional_tools` / tool search — not wired for Azure
  upstream.
- Azure endpoint and deployment resolution (`baseurl.go`) unless the upstream
  base-URL test proves a behavior change.
- The shared core's own changes — issues 06 and 08.

## Acceptance criteria / Definition of done

- [ ] `TestAzureGrammarToolsSentWhenCompatEnabled` — a grammar tool serializes as
      the flat `custom` responses tool; with the flag off it serializes as a
      function tool.
- [ ] `TestAzureStrictDefaultsToSupported` — a `json_schema` constrained tool
      emits `"strict": true` under Azure's default compat.
- [ ] `TestAzureStreamEndingPendingErrors` — an SSE stream with no terminal
      response event fails with `Azure OpenAI Responses stream ended without a
      stop reason`, verbatim.
- [ ] `TestAzureErrorMessagePropagated` — a failed response surfaces the
      provider's message.
- [ ] `TestAzureReasoningSignatureBackfill` — the ported reasoning-replay case:
      a stream whose encrypted reasoning content arrives only on the terminal
      event produces a thinking block whose signature carries it.
- [ ] `TestAzureAcceptsMaxReasoningEffort` — `max` maps through
      `thinkingLevelMap` (or passes through unmapped) without erroring.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(apis): wire azure onto the 0.84.1 shared responses core`.

## Relevant files / areas

- `ai/apis/azure/azure.go` (390 lines) — `:74` `run`, `:227` `wireRequest`,
  `:257` `buildParams`, `:318` `applyReasoning`, `:373` `buildHeaders`.
- `ai/apis/azure/azure_test.go` (243), `stream_test.go` (221),
  `azure_retry_test.go` (235), `baseurl.go` (170), `baseurl_test.go` (194).
- `ai/apis/openairesponses/messages.go`, `stream.go` — the shared core.
- Upstream: `src/api/azure-openai-responses.ts` and
  `test/azure-openai-responses-reasoning-replay.test.ts` at `936aff00`.

## Dependencies

- **Blocked by**: issues [06](/epic-4-openai-family-adapters/issues/06-responses-shared-grammar-and-tool-results.md)
  and [08](/epic-4-openai-family-adapters/issues/08-responses-shared-stream-decode.md);
  [Epic 2 issue 04](/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md)
  (`supportsOpenAIGrammarTools` / `supportsStrictMode`, read from
  `model.compat` at `:24-29`). Cross-epic, so it is stated here rather than in
  `depends_on`, which holds intra-epic numbers only.
- **Blocks**: None.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
