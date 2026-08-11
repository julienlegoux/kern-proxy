---
type: Issue
title: "openai-responses: compat resolution, session affinity, explicit prompt cache, and tool wiring"
description: "Resolve the five new responses compat flags with OpenRouter session-affinity detection, wire grammar and deferred tools into the request, and add explicit prompt-cache mode, tool_choice and the xai reasoning include."
tags: [epic-4]
timestamp: 2026-08-11T15:00:00Z
epic: 4
issue: 09
slug: openai-responses-compat-and-wiring
size: M
status: open
gh_issue: 155
resource: https://github.com/kern-ia/kern-link/issues/155
depends_on: [6, 7, 8]
---

# openai-responses: compat resolution, session affinity, explicit prompt cache, and tool wiring

## Summary

The `openai-responses` adapter itself, now that the shared core (issues 06–08)
can do the work. Upstream deleted `test/openai-responses-copilot-provider.test.ts`
(-291) and replaced it with `test/openai-responses-compat.test.ts` (+472),
which is the shape of this change: provider sniffing turns into resolved compat.

- **`getCompat`** gains `supportsStrictMode` (default **false** here, unlike the
  shared converter's true), `supportsOpenAIGrammarTools`, `supportsAdditionalTools`,
  `supportsToolSearch`, `supportsExplicitPromptCacheMode`, and replaces
  `sendSessionIdHeader` with `sessionAffinityFormat`, defaulted by
  `detectSessionAffinityFormat`: `"openrouter"` when the provider is `openrouter`
  or the base URL contains `openrouter.ai`, else `"openai"`.
- **Session headers** follow the format: `openrouter` sends only
  `x-session-id`; `openai` sends `session_id` **and** `x-client-request-id`; any
  third value sends only `x-client-request-id`. (Today the Go adapter sends
  `session_id` conditionally plus `x-client-request-id` unconditionally — the new
  rule is not a superset of that.)
- **Deferred tools** — the mode is `additional-tools` when
  `supportsAdditionalTools`, else `tool-search` when `supportsToolSearch`, else
  off; `SplitDeferredTools(chat, mode != "")` partitions the tools, only
  `immediate` goes in `tools`, and `deferred` is handed to the converter
  (issue 07).
- **Explicit prompt cache** — `cacheRetention == "none"` plus
  `supportsExplicitPromptCacheMode` sends
  `prompt_cache_options: {mode: "explicit"}`.
- Smaller riders: `tool_choice` passthrough from
  `ai.StreamOptions.OpenAIToolChoice` (declared as `any` by
  [Epic 2 issue 13](/epic-2-core-types-and-models-contracts/issues/13-fetch-sampling-and-deferred-options.md);
  upstream types it `ResponseCreateParamsStreaming["tool_choice"]`,
  `packages/ai/src/api/openai-responses.ts:96` at `936aff00`);
  `reasoningEffort` accepts `max`;
  `include: ["reasoning.encrypted_content"]` for the `xai` provider; the message
  starts at `StopReason` `pending`, a stream ending still `pending` errors with
  `OpenAI Responses stream ended without a stop reason`, and the
  aborted/error path now surfaces `output.errorMessage` instead of the generic
  `An unknown error occurred`.

## Scope

- `ai/apis/openairesponses/openairesponses.go`:
  - `resolvedCompat` (`:224`) / `getCompat` (`:230`) — the five new flags and
    `SessionAffinityFormat`, with `detectSessionAffinityFormat` as the default.
    Keep the existing defaults for `supportsDeveloperRole` and
    `supportsLongCacheRetention`.
  - `buildHeaders` (`:479`) — the three-way session-affinity branch.
  - `buildParams` (`:326`) — grammar map construction, the deferred-tool split
    and mode, `ConvertTools` options, `tool_choice` from
    `opts.OpenAIToolChoice`, `prompt_cache_options`, and the `xai` include.
  - `wireRequest` (`:309`) — `tool_choice` as a free-form value (the wire field
    takes whatever `OpenAIToolChoice` holds, unmodified),
    `prompt_cache_options`, `include`.
  - `applyReasoning` (`:401`) / `mappedThinkingLevel` (`:443`) — accept `max`.
  - `run` (`:74`) — initialize `output.StopReason` to `ai.StopReasonPending`,
    add the `pending` guard, and propagate `output.ErrorMessage` in the failure
    path. Error text upstream-verbatim.
- Port `test/openai-responses-compat.test.ts` (+472) — this is the bulk of the
  test work; translate to discrete named Go tests, offline `httptest`.
- `docs/PORTING.md` — if provider sniffing moved into catalog-driven compat, the
  `openai-responses.ts` row's parenthetical should say so.
- Keep the `// Ports:` header's symbol list current.

## Out of scope

- **The Copilot dynamic-headers gap** (`buildCopilotDynamicHeaders`,
  `hasCopilotVisionInput`, imported by this file upstream). It predates the
  0.80.3→0.84.1 delta and
  [decision 16](../../../planning/scope/16-copilot-headers-gap.md) assigns it to
  [Epic 5](/epic-5-remaining-adapters/EPIC_5.md). Do not port it here; if the
  work turns out to be blocked without it, say so in the PR body rather than
  quietly pulling it in.
- The compat *fields* — [Epic 2 issue 04](/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md).
- Catalog values that set the new flags per provider —
  [Epic 3](/epic-3-catalog-schema-and-export-tooling/EPIC_3.md).
- Azure and Codex — issues 10 and 11, even though they consume the same core.
- Swapping `httpretry` for upstream's `retryProviderRequest` — Epic 9.

## Acceptance criteria / Definition of done

- [ ] `TestSessionAffinityOpenRouterSendsOnlySessionHeader` — an
      `openrouter.ai` base URL sends `x-session-id` and neither `session_id` nor
      `x-client-request-id`.
- [ ] `TestSessionAffinityOpenAISendsBothHeaders` — the default sends
      `session_id` and `x-client-request-id`.
- [ ] `TestSessionAffinityExplicitCompatOverridesDetection` — a catalog value of
      `"openai"` on an `openrouter.ai` base URL wins over detection.
- [ ] `TestExplicitPromptCacheModeSentOnlyWhenRetentionNone` — the
      `prompt_cache_options` key appears only with both the flag and
      `cacheRetention: "none"`.
- [ ] `TestDeferredToolsSplitPlacesOnlyImmediateToolsInRequest` — with
      `supportsAdditionalTools`, an announced-but-unused tool is absent from
      `tools` and present as an `additional_tools` transcript item.
- [ ] `TestToolSearchModeSelectedWhenOnlyToolSearchSupported` — the mode falls
      through to `tool-search` correctly, and to off when neither flag is set
      (all tools immediate, no injection).
- [ ] `TestToolChoicePassedThrough` — an arbitrary value set on
      `ai.StreamOptions.OpenAIToolChoice` (a bare string *and* a nested object)
      round-trips into the body's `tool_choice` unchanged; unset omits the key.
- [ ] `TestXaiIncludesEncryptedReasoning` — provider `xai` with a reasoning model
      sends `"include":["reasoning.encrypted_content"]`; another provider does
      not.
- [ ] `TestStreamEndingPendingErrors` — an SSE stream with no terminal response
      event fails with `OpenAI Responses stream ended without a stop reason`.
- [ ] `TestErrorMessagePropagatedFromOutput` — a failed response surfaces the
      provider's message rather than `An unknown error occurred`.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(apis): resolve the 0.84.1 openai-responses compat surface`.

## Relevant files / areas

- `ai/apis/openairesponses/openairesponses.go` (513 lines) — `:74` `run`, `:224`
  `resolvedCompat`, `:230` `getCompat`, `:270` `getPromptCacheRetention`, `:309`
  `wireRequest`, `:326` `buildParams`, `:401` `applyReasoning`, `:479`
  `buildHeaders`.
- `ai/apis/openairesponses/openairesponses_test.go` (357 lines),
  `openairesponses_retry_test.go` (219).
- `ai/apis/openairesponses/messages.go`, `stream.go` — issues 06–08's work.
- Upstream: `src/api/openai-responses.ts` and
  `test/openai-responses-compat.test.ts` at `936aff00`; the deleted
  `test/openai-responses-copilot-provider.test.ts` for what it replaced.

## Dependencies

- **Blocked by**: issues [06](/epic-4-openai-family-adapters/issues/06-responses-shared-grammar-and-tool-results.md),
  [07](/epic-4-openai-family-adapters/issues/07-responses-shared-namespace-and-deferred-tools.md),
  [08](/epic-4-openai-family-adapters/issues/08-responses-shared-stream-decode.md);
  [Epic 2 issue 04](/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md)
  and [issue 11](/epic-2-core-types-and-models-contracts/issues/11-deferred-tools-split.md);
  [Epic 2 issue 13](/epic-2-core-types-and-models-contracts/issues/13-fetch-sampling-and-deferred-options.md)
  (`OpenAIToolChoice` on `ai.StreamOptions`). The Epic 2 edges live in prose
  because `depends_on` holds intra-epic numbers only.
- **Blocks**: None.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR. The likely overflow is the ported compat test suite — if it alone pushes
past the ceiling, land the adapter change with the highest-value cases and open
a follow-up for the remainder rather than trimming coverage silently.
