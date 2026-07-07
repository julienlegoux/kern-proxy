---
type: Epic
title: "Transform & validation"
description: "Cross-provider message normalizer, JSON-schema validation with the ported coercion pass, and unicode scrubbing."
tags: [epic]
timestamp: 2026-07-07T22:50:06Z
epic: 4
slug: transform-validation
status: done
gh_issue: 5
milestone: 4
resource: https://github.com/julienlegoux/kern-proxy/issues/5
source: docs/PLAN.md#phases-dependency-ordered-tdd-gates
---

# Epic 4: Transform & validation

## Goal

Build the cross-provider normalizer and tool-schema validation that make conversations portable between models and providers — the layer all six adapter epics (5–10) depend on for correct message replay.

## Scope

- `ai/apis/transform.go` — cross-provider normalizer (`transform-messages` semantics):
  - Same-model replay keeps thinking blocks + signatures; cross-model downgrades thinking→text and remaps tool-call ids.
  - Synthesizes `"No result provided"` toolResults for orphaned tool calls.
  - Skips errored/aborted assistant turns; drops `stopReason:"aborted"` turns from replay.
- `ai/internal/jsonschema`: `santhosh-tekuri/jsonschema/v6` plus a hand-port of upstream's coercion pass (`Tool.parameters` stays a `json.RawMessage` document).
- Unicode scrub integration (surrogate handling).

## Out of scope

- Per-provider request/response conversion — that lives in each adapter epic.

## Acceptance criteria

- Ports of upstream `transform-messages-*`, `validation`, `lax-message-content`, `tool-call-id-normalization`, and `unicode-surrogate` tests pass.
- Coercion pass is locked against upstream's `validation.test.ts` (plan risk #4).

## Dependencies

- [Epic 1: Foundation](/epic-1-foundation/EPIC_1.md), [Epic 2: Faux provider & registry](/epic-2-faux-provider-registry/EPIC_2.md). Blocks all adapter epics (5–10).

## Notes

- A unicode scrub already exists from Phase 1 (`ai/sanitize.go`) — this epic wires/verifies it against the `unicode-surrogate` test port rather than rewriting it.
- Key risk #4 from the plan: the JSON-schema coercion pass is custom — fixture-lock it.
- Project-wide: TDD gate (port upstream tests first → implement → goldens via `OnPayload`); `// Ports:` headers; PORTING.md mapping discipline.
