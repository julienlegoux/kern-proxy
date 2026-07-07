---
type: Epic
title: "OpenAI completions adapter"
description: "Chat-completions adapter with the ~18-flag vendor compat matrix, 10 thinking-format encodings, prompt caching, and dual-map tool-call correlation."
tags: [epic]
timestamp: 2026-07-07T05:34:54Z
epic: 6
slug: openai-completions
status: open
gh_issue: 7
milestone: 6
resource: https://github.com/julienlegoux/kern-proxy/issues/7
source: docs/PLAN.md#phases-dependency-ordered-tdd-gates
---

# Epic 6: OpenAI completions adapter

## Goal

Port the openai-completions adapter — the compatibility workhorse that serves ~15 vendors through one wire protocol via a compat-flag matrix.

## Scope

- `ai/apis/openaicompletions` over raw `net/http` + own SSE.
- Fidelity-critical mechanics:
  - Compat auto-detection from provider/baseUrl: ~18 flags (`*OpenAICompletionsCompat` on `Model.Compat`; tri-state flags as `*bool`, nil = auto-detect) serving ~15 vendors.
  - 10 thinking-format encodings (10-value thinkingFormat enum).
  - `max_tokens` vs `max_completion_tokens` selection.
  - Anthropic-style `cache_control` replication; session-affinity headers.
  - Tool-call delta correlation by **both index and id** (dual maps).
  - Usage math: `prompt_tokens − cached − cache_write`.
- Partial tool-arg JSON re-parsed on every delta (strict → repair → partial → `{}` from Epic 1); scratch buffers stripped before persistence.

## Out of scope

- The Responses API family (openai-responses, Azure, Codex) — [Epic 7](/epic-7-responses-family/EPIC_7.md).

## Acceptance criteria

- ~10 upstream `openai-completions-*` test ports pass, with golden-request snapshots matching TS-captured goldens.
- Env-gated live smoke with `OPENAI_API_KEY`.

## Dependencies

- [Epic 1](/epic-1-foundation/EPIC_1.md), [Epic 2](/epic-2-faux-provider-registry/EPIC_2.md), [Epic 3](/epic-3-auth-core/EPIC_3.md), [Epic 4](/epic-4-transform-validation/EPIC_4.md).

## Notes

- Size: L. Part of the 45%-effort adapter trio (epics 5–7).
- Project-wide: TDD gate; `// Ports:` headers; PORTING.md mapping discipline.
