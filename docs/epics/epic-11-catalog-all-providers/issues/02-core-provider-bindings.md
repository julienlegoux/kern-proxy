---
type: Issue
title: "Add all.go and the core provider bindings"
description: "ai/providers scaffolding plus the bindings for the first-party adapters: anthropic, openai (completions + responses), azure, codex, google, vertex, mistral, bedrock."
tags: [epic-11]
timestamp: 2026-07-08T14:30:00Z
epic: 11
issue: 02
slug: core-provider-bindings
size: M
status: pr-open
gh_issue: 38
gh_pr: 68
resource: https://github.com/julienlegoux/kern-proxy/issues/38
depends_on: [01]
blocked_by: ["#6", "#7", "#8", "#9", "#10", "#11"]
---

# Add all.go and the core provider bindings

## Summary

Wire the first tranche of providers for [Epic 11](/epic-11-catalog-all-providers/EPIC_11.md): the `ai/providers` package with `all.go` registration plus thin bindings for every provider that fronts a first-party adapter — anthropic, openai (completions and responses), azure, codex, google, vertex, mistral, bedrock. The remaining ~compat vendors follow in [Issue 03](./03-vendor-bindings-refreshmodels.md).

## Scope

- `ai/providers`: binding scaffolding alongside the existing `faux` provider — `all.go` and the binding pattern (provider ID, auth strategy from Epic 3 types, adapter selection, catalog model wiring).
- Bindings for the core providers listed above, each a thin declaration over its adapter — no protocol logic in bindings.
- TDD gate: port the upstream `providers` tests covering these bindings first.
- `// Ports:` headers; flip the matching PORTING.md rows.

## Out of scope

- The ~25 remaining compat-vendor bindings and `RefreshModels` — [Issue 03](./03-vendor-bindings-refreshmodels.md).
- Catalog loaders — [Issue 01](./01-export-catalog-embedded-catalog.md).

## Acceptance criteria / Definition of done

- `providers` test ports pass for the core bindings; each registers, resolves auth, and selects the right adapter/models.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- New files in the existing `ai/providers/` package (which currently holds only `faux/`).
- Read-only context: `ai/catalog` (from Issue 01), the adapter packages from epics 5–10, `ai/provider.go` (registry).

## Dependencies

Blocked by [Issue 01](./01-export-catalog-embedded-catalog.md). Blocks [Issue 03](./03-vendor-bindings-refreshmodels.md). Epic-level: the bindings need their adapters (epics 5–10) merged.

## PR size note

Target ~500 changed lines; bindings are thin and mechanical — if the batch grows past ~1000, split by adapter family.
