---
type: Issue
title: "Add the remaining vendor bindings and RefreshModels implementations"
description: "The ~25 compat-vendor bindings over the completions adapter, plus native RefreshModels for OpenRouter, Vercel gateway, NVIDIA, and Copilot policy."
tags: [epic-11]
timestamp: 2026-07-08T16:10:00Z
epic: 11
issue: 03
slug: vendor-bindings-refreshmodels
size: M
status: in-progress
gh_issue: 39
resource: https://github.com/julienlegoux/kern-proxy/issues/39
depends_on: [02]
---

# Add the remaining vendor bindings and RefreshModels implementations

## Summary

Finish [Epic 11](/epic-11-catalog-all-providers/EPIC_11.md): the remaining ~25 vendor bindings (mostly thin declarations over the openai-completions adapter and its compat matrix), bringing the total to ~35, plus the native Go `RefreshModels` implementations for the providers with live model refresh: OpenRouter, Vercel gateway, NVIDIA, and Copilot policy.

## Scope

- Remaining vendor bindings registered in `all.go`, following the pattern from [Issue 02](./02-core-provider-bindings.md).
- Native `RefreshModels` for OpenRouter, Vercel gateway, NVIDIA, Copilot policy (live model listing endpoints, decoded into catalog model shapes).
- TDD gate: port the remaining upstream `providers` and per-provider model tests first; `RefreshModels` tested against httptest fixtures.
- `// Ports:` headers; flip the matching PORTING.md rows.

## Out of scope

- New vendors beyond upstream parity.

## Acceptance criteria / Definition of done

- All catalog validation test ports pass across the full ~35-provider set — this closes the epic's acceptance criteria.
- `RefreshModels` fixtures round-trip for all four providers.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- `ai/providers/` (from Issue 02), `ai/catalog` (from Issue 01).

## Dependencies

Blocked by [Issue 02](./02-core-provider-bindings.md).

## PR size note

Target ~500 changed lines; bindings are mechanical — if the batch plus RefreshModels grows past ~1000, split RefreshModels into its own PR.
