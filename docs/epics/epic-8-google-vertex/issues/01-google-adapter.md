---
type: Issue
title: "Add the Google (Gemini) adapter"
description: "google-shared converters, thinking signatures, and the raw HTTP + SSE transport for the Gemini API."
tags: [epic-8]
timestamp: 2026-07-08T07:30:00Z
epic: 8
issue: 01
slug: google-adapter
size: L
status: in-progress
gh_issue: 32
resource: https://github.com/julienlegoux/kern-proxy/issues/32
depends_on: []
blocked_by: ["#18", "#19"]
---

# Add the Google (Gemini) adapter

## Summary

Port the Google (Gemini) adapter ([Epic 8](/epic-8-google-vertex/EPIC_8.md)): the google-shared request/response converters, thinking-signature handling, and streaming over raw `net/http` + the hand-rolled SSE parser. The Vertex variant layers its auth and endpoint differences on top in [Issue 02](./02-vertex-variant-adc.md).

## Scope

- New `ai/apis/google` package: google-shared converters (contents, tools, generation config), thinking signatures, streaming decode to unified events.
- API-key auth path via the resolved `ModelAuth`.
- Shared code factored so the Vertex variant reuses the converters unchanged.
- TDD gate: port the upstream `google-*` tests first; goldens via `OnPayload` match TS-captured goldens.
- `// Ports:` headers; flip the matching PORTING.md rows.

## Out of scope

- Vertex endpoints and ADC auth — [Issue 02](./02-vertex-variant-adc.md).
- Provider bindings/catalog entries — [Epic 11](/epic-11-catalog-all-providers/EPIC_11.md).

## Acceptance criteria / Definition of done

- `google-*` test ports pass against httptest fixtures with matching goldens; thinking signatures round-trip.
- Env-gated live smoke where upstream has one.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- New: `ai/apis/google/` — path follows the plan's layout; no adapter code exists yet.
- Read-only context: `ai/internal/sse`, `ai/events.go`, `ai/stream.go`, `ai/providers/faux`.

## Dependencies

Blocks [Issue 02](./02-vertex-variant-adc.md). Epic-level: needs epics 1–4.

## PR size note

Sized L: converters + stream decoder + verbatim fixture ports form one unit. Fixtures/goldens excluded from the budget; if hand-written code approaches ~1000 lines, split converters from transport.
