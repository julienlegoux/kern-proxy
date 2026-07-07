---
type: Issue
title: "Add the cross-provider message transform"
description: "transform-messages normalizer: same-model thinking replay, cross-model downgrade, tool-call id remap, orphan toolResult synthesis, plus unicode-scrub verification."
tags: [epic-4]
timestamp: 2026-07-08T00:05:00Z
epic: 4
issue: 02
slug: cross-provider-transform
size: M
status: pr-open
gh_issue: 19
gh_pr: 49
resource: https://github.com/julienlegoux/kern-proxy/issues/19
depends_on: []
---

# Add the cross-provider message transform

## Summary

Port the `transform-messages` normalizer that makes conversations portable between models and providers — the layer every adapter epic (5–10) replays messages through. Also wires the existing Phase 1 unicode scrub (`ai/sanitize.go`) into the pipeline and locks it against the upstream `unicode-surrogate` test port (verify, don't rewrite).

## Scope

- `ai/apis/transform.go` with upstream `transform-messages` semantics:
  - Same-model replay keeps thinking blocks + signatures; cross-model downgrades thinking→text and remaps tool-call ids.
  - Synthesizes `"No result provided"` toolResults for orphaned tool calls.
  - Skips errored/aborted assistant turns; drops `stopReason:"aborted"` turns from replay.
- Tool-call id normalization per upstream `tool-call-id-normalization`.
- Lax message-content handling per upstream `lax-message-content`.
- Unicode scrub integration: verify `ai/sanitize.go` against the ported `unicode-surrogate` tests.
- TDD gate: port `transform-messages-*`, `tool-call-id-normalization`, `lax-message-content`, and `unicode-surrogate` tests first.
- `// Ports:` headers; flip the matching PORTING.md rows.

## Out of scope

- JSON-schema validation/coercion — [Issue 01](./01-jsonschema-validation-coercion.md).
- Per-provider request/response conversion — adapter epics 5–10.

## Acceptance criteria / Definition of done

- Ported `transform-messages-*`, `tool-call-id-normalization`, `lax-message-content`, and `unicode-surrogate` suites pass.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- New: `ai/apis/transform.go` (+ tests) — a new file in the existing `ai/apis` package (which currently holds only `simpleopts.go`).
- Read-only context: `ai/types.go` (message/thinking/tool types), `ai/sanitize.go` (existing scrub), upstream `transform-messages` sources.

## Dependencies

None within this epic — independent of [Issue 01](./01-jsonschema-validation-coercion.md). Blocks all adapter epics (5–10) via [Epic 4](/epic-4-transform-validation/EPIC_4.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.
