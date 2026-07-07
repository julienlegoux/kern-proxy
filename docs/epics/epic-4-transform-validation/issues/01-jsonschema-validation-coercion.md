---
type: Issue
title: "Add ai/internal/jsonschema with the ported coercion pass"
description: "Tool-parameter validation via santhosh-tekuri/jsonschema/v6 plus a hand-port of upstream's coercion pass, fixture-locked against validation.test.ts."
tags: [epic-4]
timestamp: 2026-07-07T22:14:30Z
epic: 4
issue: 01
slug: jsonschema-validation-coercion
size: M
status: pr-open
gh_issue: 18
gh_pr: 48
resource: https://github.com/julienlegoux/kern-proxy/issues/18
depends_on: []
---

# Add ai/internal/jsonschema with the ported coercion pass

## Summary

Build the tool-schema validation layer of [Epic 4: Transform & validation](/epic-4-transform-validation/EPIC_4.md): wrap `santhosh-tekuri/jsonschema/v6` and hand-port upstream's coercion pass so tool arguments validate and coerce exactly like the TS library. This is the plan's key risk #4 — the coercion pass is custom code, so it must be fixture-locked against upstream's `validation.test.ts` rather than approximated.

## Scope

- New `ai/internal/jsonschema` package:
  - Compile/validate `Tool.parameters` (which stays a `json.RawMessage` document end to end) via `santhosh-tekuri/jsonschema/v6`.
  - Hand-port of upstream's coercion pass applied before/around validation, matching `validation.test.ts` semantics exactly.
- TDD gate: port the upstream `validation` tests first; every coercion case in `validation.test.ts` becomes a Go test case.
- `// Ports:` headers referencing the upstream sources; flip the matching PORTING.md rows.

## Out of scope

- The cross-provider message transform — [Issue 02](./02-cross-provider-transform.md).
- Per-provider request/response conversion — adapter epics 5–10.

## Acceptance criteria / Definition of done

- Ported `validation` test suite passes; coercion behavior is locked case-by-case against upstream's `validation.test.ts` (plan risk #4).
- `Tool.parameters` remains `json.RawMessage` at the API boundary — no premature struct decoding.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- New: `ai/internal/jsonschema/` — no code exists there yet; the path follows the plan's layout, not verified files.
- Read-only context: `ai/types.go` (Tool definition), upstream `packages/ai/src` validation sources and `validation.test.ts`.

## Dependencies

None within this epic — independent of [Issue 02](./02-cross-provider-transform.md). Blocks the adapter epics' tool-call handling (epics 5–10) via [Epic 4](/epic-4-transform-validation/EPIC_4.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.
