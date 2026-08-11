---
type: Issue
title: "openai-completions: baseten thinking format, thinking_token_budget, and qwen effort mapping"
description: "Add the baseten chat_template_args branch, the vLLM thinking_token_budget cap, and thinkingLevelMap resolution on the qwen and baseten reasoning-effort paths."
tags: [epic-4]
timestamp: 2026-08-11T18:15:00Z
epic: 4
issue: 04
slug: completions-thinking-formats
size: M
status: open
gh_issue: 150
resource: https://github.com/kern-ia/kern-link/issues/150
depends_on: []
---

# openai-completions: baseten thinking format, thinking_token_budget, and qwen effort mapping

## Summary

Three independent additions to the chat-completions thinking ladder, grouped
because they all live in `applyThinkingFormat` /
`buildChatTemplateKwargs` (`openaicompletions.go:531,652`) and would otherwise
conflict line-for-line:

1. **`baseten` thinking format** — a new `ThinkingFormat` value. It sends
   `chat_template_args` (a *different* request key from `chat_template_kwargs`),
   built from the new `Compat.ChatTemplateArgs` map by the same resolver, plus
   `reasoning_effort` resolved through `model.thinkingLevelMap`. Its distinctive
   detail: when no effort is requested it still consults
   `thinkingLevelMap.off`, so a model can force an explicit "off" effort string.
2. **`thinking_token_budget`** — vLLM's top-level cap, gated on
   `Compat.SupportsThinkingTokenBudget` and independent of thinking format
   ("the same server can serve zai, qwen or chat-template models"). Budgets
   default to `minimal: 1024, low: 2048, medium: 8192, high: 16384`, overridable
   by `StreamOptions.OpenAIThinkingBudgets`, then clamped to
   `ceiling - MIN_ANSWER_TOKENS` where the ceiling is `max_tokens`, else
   `max_completion_tokens`, else `model.maxTokens`. A budget of `0` or less is
   not sent. Upstream's comment explains why the clamp exists — reasoning and
   answer share `max_tokens`, so an uncapped budget "recreates the bug it
   prevents". Carry that comment across; it is exactly the decision-explaining
   comment style [CONVENTIONS.md](../../../planning/CONVENTIONS.md) asks for.
3. **qwen effort mapping** — the `qwen` branch now also sends
   `reasoning_effort`, mapped through `thinkingLevelMap`, when
   `supportsReasoningEffort` is set.

`buildChatTemplateKwargs` is generalized to `buildChatTemplateValues(model,
options, values)` taking the map as a parameter, so the same resolver serves
both `chat_template_kwargs` and `chat_template_args`.

## Scope

- `ai/apis/openaicompletions/openaicompletions.go`:
  - `applyThinkingFormat` (`:531`) — add the `baseten` branch after
    `chat-template`; extend the `qwen` branch with the mapped
    `reasoning_effort`.
  - `buildChatTemplateKwargs` (`:652`) → take the value map as a parameter
    (rename to match upstream's `buildChatTemplateValues`, since
    [CONVENTIONS.md](../../../planning/CONVENTIONS.md) keeps upstream spellings).
    `resolveChatTemplateKwargValue` (`:671`) is unchanged.
  - `wireRequest` (`:216`) — add `chat_template_args` and
    `thinking_token_budget`.
  - `buildParams` (`:340`) — the `thinking_token_budget` block, placed where
    upstream places it (after the thinking-format ladder, before the OpenRouter
    routing block).
  - `MIN_ANSWER_TOKENS` — port upstream's constant with its value and its
    comment; do not invent a different name.
- The caller-side override is `StreamOptions.OpenAIThinkingBudgets
  *ai.ThinkingBudgets`, declared by
  [Epic 2 issue 13](/epic-2-core-types-and-models-contracts/issues/13-fetch-sampling-and-deferred-options.md)
  and mirroring `BedrockThinkingBudgets` (`ai/options.go:276-278`). The
  `ai.ThinkingBudgets` **type** exists today, but the only field of that type on
  the caller path is `SimpleStreamOptions.ThinkingBudgets` (`:325-326`), which
  `buildParams(model, chat, opts *ai.StreamOptions)`
  (`ai/apis/openaicompletions/openaicompletions.go:340`) never sees — read
  `OpenAIThinkingBudgets`, not that one. The defaults above are the adapter's,
  applied per level with the caller's map overlaid field by field (upstream
  spreads `...options.thinkingBudgets` over the defaults at
  `packages/ai/src/api/openai-completions.ts:858`, so an override of one level
  leaves the others at their default — a Go struct of pointers or an explicit
  per-field merge, not a wholesale replace).
- Port `test/openai-completions-thinking-token-budget.test.ts` (+124) and the
  baseten cases of `test/baseten-models.test.ts` that exercise the wire shape
  (the provider *binding* itself is
  [Epic 6](/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md)'s).
- Keep the `// Ports:` header's symbol list current.

## Out of scope

- The `Compat.ChatTemplateArgs`, `ThinkingFormatBaseten`, and
  `SupportsThinkingTokenBudget` fields themselves —
  [Epic 2 issue 04](/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md)
  ships them. This PR reads them.
- The `baseten` provider binding and its catalog entry — Epic 6 / Epic 3.
- The `max` thinking level's own widening —
  [Epic 2 issue 01](/epic-2-core-types-and-models-contracts/issues/01-widen-stopreason-and-thinkinglevel.md).
  This PR must, however, not break on it: whatever `clampReasoning` equivalent
  the ladder uses has to have a defined answer for `max`.
- Grammar tools (issue 03), deferred tools and finish-reason inference
  (issue 05).

## Acceptance criteria / Definition of done

- [ ] `TestBasetenThinkingFormatSendsChatTemplateArgs` — a model with
      `ThinkingFormat: "baseten"` and `ChatTemplateArgs` set produces a body with
      `chat_template_args` populated and **no** `chat_template_kwargs`.
- [ ] `TestBasetenReasoningEffortUsesThinkingLevelMapOff` — with no requested
      effort and `thinkingLevelMap.off = "none"`, the body carries
      `"reasoning_effort":"none"`; with `thinkingLevelMap` absent and no effort
      requested, the key is absent.
- [ ] `TestThinkingTokenBudgetClampsToMaxTokensMinusAnswerFloor` — with
      `max_tokens` set just above `MIN_ANSWER_TOKENS`, the emitted
      `thinking_token_budget` equals `max_tokens - MIN_ANSWER_TOKENS`, not the
      per-level default.
- [ ] `TestThinkingTokenBudgetOmittedWhenNonPositive` — a ceiling at or below
      `MIN_ANSWER_TOKENS` emits no `thinking_token_budget` key at all.
- [ ] `TestThinkingTokenBudgetHonorsPerLevelOverride` — overriding only `high`
      via `StreamOptions.OpenAIThinkingBudgets` leaves `low` at its default in a second
      request.
- [ ] `TestThinkingTokenBudgetIndependentOfThinkingFormat` — the budget is sent
      for a `qwen`-format model as well as a `chat-template` one, given the
      compat flag.
- [ ] `TestQwenSendsMappedReasoningEffort` — `thinkingLevelMap` translates the
      requested level, and the key is omitted when `supportsReasoningEffort` is
      false.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(apis): add the baseten thinking format and thinking_token_budget`.

## Relevant files / areas

- `ai/apis/openaicompletions/openaicompletions.go:216` `wireRequest`, `:340`
  `buildParams`, `:531` `applyThinkingFormat`, `:614-651` the
  `mappedThinking*` helpers, `:652` `buildChatTemplateKwargs`, `:671`
  `resolveChatTemplateKwargValue`.
- `ai/apis/openaicompletions/thinking_test.go` (420 lines) and
  `compat_wiring_test.go` (234) — the existing coverage this extends.
- `ai/options.go:276-278` — `BedrockThinkingBudgets`, the precedent
  `OpenAIThinkingBudgets` copies; `:325-326` — `SimpleStreamOptions.ThinkingBudgets`,
  the field this adapter must **not** read.
- `ai/model.go` — `ThinkingFormat` values and the compat struct.
- Upstream: `src/api/openai-completions.ts` `buildParams` /
  `buildChatTemplateValues` at `936aff00`;
  `test/openai-completions-thinking-token-budget.test.ts`.

## Dependencies

- **Blocked by**: [Epic 2 issue 04](/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md).
- **Blocked by**: [Epic 2 issue 13](/epic-2-core-types-and-models-contracts/issues/13-fetch-sampling-and-deferred-options.md)
  (`OpenAIThinkingBudgets` on `ai.StreamOptions`). Cross-epic, so `depends_on`
  cannot carry it — it holds intra-epic numbers only.
- **Blocks**: None.

## PR size note

`M` — ~380 changed lines: three additions to the thinking ladder (the `baseten`
format, `thinking_token_budget` with its clamp and `MIN_ANSWER_TOKENS`, the qwen
effort mapping), the `buildChatTemplateValues` generalization, and seven named
tests carrying the +124-line upstream thinking-token-budget suite into the
existing 420-line `thinking_test.go`. Split past ~500; the budget block is the
standalone half, since it is gated independently of thinking format.
