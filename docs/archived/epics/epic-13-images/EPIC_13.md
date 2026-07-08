---
type: Epic
title: "Images"
description: "Parallel image-generation stack with the openrouter-images adapter."
tags: [epic]
timestamp: 2026-07-08T17:13:16Z
epic: 13
slug: images
status: done
gh_issue: 14
milestone: 13
resource: https://github.com/julienlegoux/kern-proxy/issues/14
source: docs/PLAN.md#phases-dependency-ordered-tdd-gates
---

# Epic 13: Images

## Goal

Port the image-generation stack — a parallel (separate from text streaming) pipeline with its own adapter.

## Scope

- `ai/images`: the parallel image-gen stack.
- The openrouter-images adapter.
- Image-model catalog entries come from `image-models.generated.ts` via the export-catalog JSON ([Epic 11](/epic-11-catalog-all-providers/EPIC_11.md)).

## Out of scope

- Text-streaming providers — earlier epics.

## Acceptance criteria

- Upstream images test ports pass.

## Dependencies

- [Epic 1](/epic-1-foundation/EPIC_1.md), [Epic 3](/epic-3-auth-core/EPIC_3.md); catalog integration via [Epic 11](/epic-11-catalog-all-providers/EPIC_11.md).

## Notes

- Size: S — deliberately kept as its own epic (one epic per plan phase, per project decision).
- Project-wide: TDD gate; `// Ports:` headers; PORTING.md mapping discipline.
