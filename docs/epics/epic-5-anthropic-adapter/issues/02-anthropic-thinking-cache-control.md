---
type: Issue
title: "Add adaptive thinking, cache_control, and 1h cache-write cost to the Anthropic adapter"
description: "Thinking-budget handling, cache_control placement, and the 1h cache-write 2x-input billing rule in calculateCost."
tags: [epic-5]
timestamp: 2026-07-07T08:21:52Z
epic: 5
issue: 02
slug: anthropic-thinking-cache-control
size: M
status: open
gh_issue: 21
resource: https://github.com/julienlegoux/kern-proxy/issues/21
depends_on: [01]
---

# Add adaptive thinking, cache_control, and 1h cache-write cost to the Anthropic adapter

## Summary

Layer the thinking and prompt-caching mechanics onto the adapter core from [Issue 01](./01-anthropic-adapter-core.md): adaptive thinking configuration, `cache_control` breakpoint placement, and the billing rule that 1h cache writes cost 2× input in `calculateCost` (the `anthropic-cache-write-1h-cost` table from Epic 1 already encodes the rates).

## Scope

- Adaptive thinking: thinking budget/effort mapping into the request; thinking block decode (signatures preserved for same-model replay via Epic 4's transform).
- `cache_control` placement rules matching upstream exactly.
- Cost integration: **1h cache-write billed at 2× input** in the cost calculation.
- TDD gate: port the upstream thinking/cache `anthropic-*` tests first, goldens via `OnPayload`.

## Out of scope

- OAuth impersonation headers — [Issue 03](./03-anthropic-oauth-impersonation.md).
- Retry/overflow wiring — [Issue 04](./04-anthropic-retry-overflow-smoke.md).

## Acceptance criteria / Definition of done

- Thinking and cache-related `anthropic-*` test ports pass; golden requests show correct `thinking` and `cache_control` payloads.
- Cost assertions cover the 1h cache-write 2× input rule.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- `ai/apis/anthropic/` (from Issue 01), `ai/cost.go` / `ai/cost_test.go` (existing cost tables).

## Dependencies

Blocked by [Issue 01](./01-anthropic-adapter-core.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.
