---
type: Epic
title: "Mistral adapter"
description: "mistral-conversations adapter over raw HTTP."
tags: [epic]
timestamp: 2026-07-08T12:23:06Z
epic: 9
slug: mistral
status: done
gh_issue: 10
milestone: 9
resource: https://github.com/julienlegoux/kern-proxy/issues/10
source: docs/PLAN.md#phases-dependency-ordered-tdd-gates
---

# Epic 9: Mistral adapter

## Goal

Port the mistral-conversations adapter — the simplest of the wire protocols, over raw HTTP.

## Scope

- `ai/apis/mistral`: mistral-conversations protocol, raw `net/http` + own SSE.

## Out of scope

- Provider bindings/catalog entries — [Epic 11](/epic-11-catalog-all-providers/EPIC_11.md).

## Acceptance criteria

- Upstream `mistral-*` test ports pass with golden-request snapshots.

## Dependencies

- [Epic 1](/epic-1-foundation/EPIC_1.md), [Epic 2](/epic-2-faux-provider-registry/EPIC_2.md), [Epic 3](/epic-3-auth-core/EPIC_3.md), [Epic 4](/epic-4-transform-validation/EPIC_4.md).

## Notes

- Size: S–M — deliberately kept as its own epic (one epic per plan phase, per project decision), even though it is on the small side.
- Project-wide: TDD gate; `// Ports:` headers; PORTING.md mapping discipline.
