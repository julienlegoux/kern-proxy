---
type: Epic
title: "Google & Vertex adapters"
description: "google-shared converters, thinking signatures, raw HTTP transport, and Vertex ADC auth via golang.org/x/oauth2/google."
tags: [epic]
timestamp: 2026-07-07T05:34:54Z
epic: 8
slug: google-vertex
status: open
gh_issue: 9
milestone: 8
resource: https://github.com/julienlegoux/kern-proxy/issues/9
source: docs/PLAN.md#phases-dependency-ordered-tdd-gates
---

# Epic 8: Google & Vertex adapters

## Goal

Port the Google (Gemini) adapter and its Vertex variant, including the shared converters and Vertex's Application Default Credentials auth path.

## Scope

- `ai/apis/google` (+ `vertex/`): google-shared converters, thinking signatures, raw HTTP + own SSE.
- Vertex ADC via `golang.org/x/oauth2/google`; the ambient **Vertex ADC sentinel** from the auth env-key map (Epic 3).

## Out of scope

- Provider bindings/catalog entries — [Epic 11](/epic-11-catalog-all-providers/EPIC_11.md).

## Acceptance criteria

- Upstream `google-*` test ports pass with golden-request snapshots.
- Env-gated live smoke where applicable.

## Dependencies

- [Epic 1](/epic-1-foundation/EPIC_1.md), [Epic 2](/epic-2-faux-provider-registry/EPIC_2.md), [Epic 3](/epic-3-auth-core/EPIC_3.md), [Epic 4](/epic-4-transform-validation/EPIC_4.md).

## Notes

- Size: M–L.
- Project-wide: TDD gate; `// Ports:` headers; PORTING.md mapping discipline.
