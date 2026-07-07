---
type: Epic
title: "Bedrock adapter"
description: "AWS Bedrock ConverseStream adapter via aws-sdk-go-v2, with the full AWS auth matrix including bearer-token mode and thinking payloads."
tags: [epic]
timestamp: 2026-07-07T05:34:54Z
epic: 10
slug: bedrock
status: open
gh_issue: 11
milestone: 10
resource: https://github.com/julienlegoux/kern-proxy/issues/11
source: docs/PLAN.md#phases-dependency-ordered-tdd-gates
---

# Epic 10: Bedrock adapter

## Goal

Port the Bedrock adapter using `aws-sdk-go-v2` bedrockruntime (SigV4 + event-stream framing — the one transport where an SDK is used deliberately).

## Scope

- `ai/apis/bedrock` on `aws-sdk-go-v2` bedrockruntime `ConverseStream`.
- Full AWS auth matrix, **including bearer-token auth**; ambient Bedrock AWS auth sentinels from the env-key map (Epic 3).
- Thinking payloads.

## Out of scope

- Provider bindings/catalog entries — [Epic 11](/epic-11-catalog-all-providers/EPIC_11.md).

## Acceptance criteria

- Upstream `bedrock-*` test ports pass with golden-request snapshots.

## Dependencies

- [Epic 1](/epic-1-foundation/EPIC_1.md), [Epic 2](/epic-2-faux-provider-registry/EPIC_2.md), [Epic 3](/epic-3-auth-core/EPIC_3.md), [Epic 4](/epic-4-transform-validation/EPIC_4.md).

## Notes

- Size: M–L.
- **Key risk #3 from the plan:** verify bearer-token auth is expressible in aws-sdk-go-v2 — if not, hand-roll that one auth mode.
- Project-wide: TDD gate; `// Ports:` headers; PORTING.md mapping discipline.
