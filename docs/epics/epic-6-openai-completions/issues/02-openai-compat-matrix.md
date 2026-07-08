---
type: Issue
title: "Add the vendor compat auto-detection matrix"
description: "~18 tri-state compat flags auto-detected from provider/baseUrl serving ~15 vendors, plus max_tokens vs max_completion_tokens selection."
tags: [epic-6]
timestamp: 2026-07-08T05:40:00Z
epic: 6
issue: 02
slug: openai-compat-matrix
size: M
status: done
gh_issue: 25
gh_pr: 55
resource: https://github.com/julienlegoux/kern-proxy/issues/25
depends_on: [01]
---

# Add the vendor compat auto-detection matrix

## Summary

Port the compat-flag matrix that lets one wire protocol serve ~15 vendors: ~18 flags on `Model.Compat` (`*OpenAICompletionsCompat`), tri-state as `*bool` where `nil` means auto-detect from provider/baseUrl, layered onto [Issue 01](./01-openai-completions-adapter-core.md)'s core.

## Scope

- `*OpenAICompletionsCompat` struct (~18 flags, tri-state `*bool`, nil = auto-detect) and the provider/baseUrl auto-detection rules, matching upstream vendor-by-vendor.
- `max_tokens` vs `max_completion_tokens` selection.
- TDD gate: port the upstream compat `openai-completions-*` tests first — every vendor detection case fixture-locked.

## Out of scope

- Thinking formats — [Issue 03](./03-openai-thinking-formats.md); caching/affinity — [Issue 04](./04-openai-cache-affinity-smoke.md).
- Catalog enforcement that `Compat` matches `Api` — [Epic 11](/epic-11-catalog-all-providers/EPIC_11.md).

## Acceptance criteria / Definition of done

- Compat test ports pass; explicit flag values override auto-detection; goldens show correct request shapes per vendor.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- `ai/apis/openaicompletions/` (from Issue 01), `ai/model.go` (`Model.Compat` wiring).

## Dependencies

Blocked by [Issue 01](./01-openai-completions-adapter-core.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.
