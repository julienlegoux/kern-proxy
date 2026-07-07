---
type: Issue
title: "Add the thinking-format encodings"
description: "The 10-value thinkingFormat enum and its per-format request/response encodings for reasoning-capable completions vendors."
tags: [epic-6]
timestamp: 2026-07-07T08:23:01Z
epic: 6
issue: 03
slug: openai-thinking-formats
size: M
status: open
gh_issue: 26
resource: https://github.com/julienlegoux/kern-proxy/issues/26
depends_on: [01]
---

# Add the thinking-format encodings

## Summary

Port the 10 thinking-format encodings (the 10-value `thinkingFormat` enum) that map reasoning content to and from each vendor's wire representation, on top of [Issue 01](./01-openai-completions-adapter-core.md)'s core.

## Scope

- The `thinkingFormat` enum (10 values) and per-format encode (request) / decode (stream delta) logic, matching upstream exactly.
- Interaction with Epic 4's transform: thinking survives same-model replay, downgrades cross-model.
- TDD gate: port the upstream thinking `openai-completions-*` tests first — one fixture-locked case per format.

## Out of scope

- The compat matrix itself — [Issue 02](./02-openai-compat-matrix.md) (formats are selected by model/compat config, but the selection plumbing lands there).

## Acceptance criteria / Definition of done

- All 10 formats covered by passing test ports with goldens for encode and fixtures for decode.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- `ai/apis/openaicompletions/` (from Issue 01), `ai/types.go` (thinking blocks, read-only).

## Dependencies

Blocked by [Issue 01](./01-openai-completions-adapter-core.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.
