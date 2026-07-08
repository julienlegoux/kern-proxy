---
type: Issue
title: "Add the mistral-conversations adapter"
description: "The mistral-conversations protocol over raw HTTP + SSE — the simplest wire protocol, one issue for the whole epic."
tags: [epic-9]
timestamp: 2026-07-08T12:23:06Z
epic: 9
issue: 01
slug: mistral-adapter
size: M
status: done
gh_issue: 34
gh_pr: 64
resource: https://github.com/julienlegoux/kern-proxy/issues/34
depends_on: []
blocked_by: ["#18", "#19"]
---

# Add the mistral-conversations adapter

## Summary

Port the mistral-conversations adapter — the simplest of the wire protocols and the whole of [Epic 9](/epic-9-mistral/EPIC_9.md) in a single issue: request building and streaming decode over raw `net/http` + the hand-rolled SSE parser.

## Scope

- New `ai/apis/mistral` package: mistral-conversations request building (entries, tools) and streaming decode to unified events.
- TDD gate: port the upstream `mistral-*` tests first; goldens via `OnPayload` match TS-captured goldens.
- `// Ports:` headers; flip the matching PORTING.md rows.

## Out of scope

- Provider binding/catalog entry — [Epic 11](/epic-11-catalog-all-providers/EPIC_11.md).

## Acceptance criteria / Definition of done

- `mistral-*` test ports pass against httptest fixtures with matching goldens — this closes the epic's acceptance criteria.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- New: `ai/apis/mistral/` — path follows the plan's layout; no adapter code exists yet.
- Read-only context: `ai/internal/sse`, `ai/events.go`, `ai/stream.go`, `ai/providers/faux`.

## Dependencies

None within this epic. Epic-level: needs epics 1–4.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.
