---
type: Issue
title: "Add the Azure responses variant"
description: "Azure OpenAI variant of the responses adapter: endpoint shape, auth headers, and version quirks over the shared core."
tags: [epic-7]
timestamp: 2026-07-08T06:30:00Z
epic: 7
issue: 02
slug: azure-responses-variant
size: S
status: done
gh_issue: 29
gh_pr: 59
resource: https://github.com/julienlegoux/kern-proxy/issues/29
depends_on: [01]
---

# Add the Azure responses variant

## Summary

Port the Azure variant of the Responses adapter as a thin layer over [Issue 01](./01-openai-responses-adapter-core.md)'s shared core: Azure endpoint/deployment URL shape, auth header differences, and any request-shape quirks upstream encodes.

## Scope

- `ai/apis/openairesponses/azure`: Azure-specific base-URL/auth/request shaping over the shared core — no protocol duplication.
- TDD gate: port the upstream Azure responses tests first, goldens via `OnPayload`.
- `// Ports:` headers; flip the matching PORTING.md row.

## Out of scope

- Codex — [Issue 03](./03-codex-adapter.md) / [Issue 04](./04-codex-websocket-transport.md).
- The Azure provider binding/catalog entry — [Epic 11](/epic-11-catalog-all-providers/EPIC_11.md).

## Acceptance criteria / Definition of done

- Azure test ports pass with matching goldens (URL shape, headers, payload).
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- New: `ai/apis/openairesponses/azure/`; shared core from Issue 01 (read-mostly).

## Dependencies

Blocked by [Issue 01](./01-openai-responses-adapter-core.md).

## PR size note

Target ~500 changed lines; this should land well under — it's a thin variant.
