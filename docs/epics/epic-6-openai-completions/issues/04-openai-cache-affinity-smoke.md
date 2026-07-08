---
type: Issue
title: "Add prompt-cache replication, session-affinity headers, and the live smoke"
description: "Anthropic-style cache_control replication for completions vendors, session-affinity headers, and the env-gated OPENAI_API_KEY smoke test."
tags: [epic-6]
timestamp: 2026-07-08T09:00:00Z
epic: 6
issue: 04
slug: openai-cache-affinity-smoke
size: S
status: done
gh_issue: 27
gh_pr: 57
resource: https://github.com/julienlegoux/kern-proxy/issues/27
depends_on: [01]
---

# Add prompt-cache replication, session-affinity headers, and the live smoke

## Summary

Finish [Epic 6](/epic-6-openai-completions/EPIC_6.md): replicate anthropic-style `cache_control` for the completions vendors that support it, emit session-affinity headers, and add the env-gated live smoke — closing out the ~10 upstream `openai-completions-*` suites.

## Scope

- Anthropic-style `cache_control` replication in completions requests, per upstream rules.
- Session-affinity headers.
- Env-gated live smoke mirroring upstream `skipIf`: runs only with `OPENAI_API_KEY` set.
- TDD gate: port the remaining upstream `openai-completions-*` tests first.

## Out of scope

- Usage math — landed with [Issue 01](./01-openai-completions-adapter-core.md); compat/thinking — [Issue 02](./02-openai-compat-matrix.md) / [Issue 03](./03-openai-thinking-formats.md).

## Acceptance criteria / Definition of done

- Cache/affinity test ports pass with goldens; live smoke streams with `OPENAI_API_KEY`, skips without.
- All ~10 upstream `openai-completions-*` suites now pass, closing the epic's acceptance criteria.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- `ai/apis/openaicompletions/` (from Issue 01), `ai/headers.go` (read-only reuse).

## Dependencies

Blocked by [Issue 01](./01-openai-completions-adapter-core.md).

## PR size note

Target ~500 changed lines; this should land well under — it's focused wiring plus test ports.
