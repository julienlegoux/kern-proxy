---
type: Epic
title: "Anthropic adapter"
description: "Full anthropic-messages adapter including OAuth Claude Code impersonation, adaptive thinking, cache_control, and retry/overflow integration."
tags: [epic]
timestamp: 2026-07-08T06:00:00Z
epic: 5
slug: anthropic-adapter
status: done
gh_issue: 6
milestone: 5
resource: https://github.com/julienlegoux/kern-proxy/issues/6
source: docs/PLAN.md#phases-dependency-ordered-tdd-gates
---

# Epic 5: Anthropic adapter

## Goal

Port the full anthropic-messages adapter — the first real provider adapter, and one of the three (with epics 6–7) that carry ~45% of the total effort.

## Scope

- Full anthropic-messages adapter (`ai/apis/anthropic`) over raw `net/http` + the hand-rolled SSE parser.
- Fidelity-critical mechanics (from upstream exploration):
  - Usage seeded at `message_start` and only non-null-overwritten at `message_delta`.
  - Stream "ended before message_stop" → retryable.
  - OAuth **Claude Code impersonation** mode: identity system block, beta headers, `user-agent: claude-cli/…`, tool-name remap.
  - Adaptive thinking; `cache_control` placement.
  - **1h cache-write billed at 2× input** in `calculateCost`.
- Retry/overflow classifier integration (from Epic 1's foundation utilities).

## Out of scope

- The Anthropic OAuth *login* flow (PKCE :53692) — [Epic 12: OAuth flows](/epic-12-oauth-flows/EPIC_12.md). This epic consumes stored credentials.
- Provider bindings/catalog entries — [Epic 11](/epic-11-catalog-all-providers/EPIC_11.md).

## Acceptance criteria

- ~12 upstream `anthropic-*` test ports pass, with SSE fixtures copied verbatim and golden-request snapshots (via `OnPayload`) matching TS-captured goldens.
- Env-gated live smoke test (mirroring upstream `skipIf`) passes with `ANTHROPIC_API_KEY` set.

## Dependencies

- [Epic 1](/epic-1-foundation/EPIC_1.md), [Epic 2](/epic-2-faux-provider-registry/EPIC_2.md), [Epic 3](/epic-3-auth-core/EPIC_3.md), [Epic 4](/epic-4-transform-validation/EPIC_4.md).

## Notes

- Size: L. Plan weighting: epics 5–7 together ≈ 45% of total effort.
- Key risk #5 from the plan: SSE edge cases feed the retry classifier — must be bit-exact (fixtures exist).
- Project-wide: TDD gate; `// Ports:` headers; PORTING.md mapping discipline; retry/overflow/scrub utilities live in the `ai` root, not `ai/internal/httpx` (layout deviation recorded in Epic 1).
