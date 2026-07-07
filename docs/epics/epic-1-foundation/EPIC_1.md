---
type: Epic
title: "Foundation"
description: "Domain core with zero deps: unified types, events, options, cost, stream, plus internal SSE, partial-JSON, and HTTP utilities."
tags: [epic]
timestamp: 2026-07-07T05:34:54Z
epic: 1
slug: foundation
status: done
gh_issue: 2
milestone: 1
resource: https://github.com/julienlegoux/kern-proxy/issues/2
source: docs/PLAN.md#phases-dependency-ordered-tdd-gates
---

# Epic 1: Foundation

## Goal

Establish the zero-dependency domain core of the Go rebuild of `@earendil-works/pi-ai`: the unified message/event model, stream contract, options, and cost tracking that every provider adapter builds on, plus the internal SSE, partial-JSON, and HTTP-quirk utilities.

## Scope

- `ai` package: types, events, options, cost, stream.
- `ai/internal/sse`: hand-rolled SSE parser handling CR/LF/CRLF and early-EOF detection.
- `ai/internal/partialjson`: strict → repair → partial → `{}` cascade, written from scratch (no Go equivalent exists).
- HTTP utilities (planned as `ai/internal/httpx`): retry classifier, overflow regexes, error-body normalize, proxy, zstd, unicode scrub.
- Key Go mapping decisions locked here:
  - Unions → sealed interfaces with one struct per variant (`ContentPart`, `Message`, `Event`) and custom `MarshalJSON`/`UnmarshalJSON` discriminating on `"type"` so wire JSON stays byte-identical to TS.
  - EventStream → `Stream`: buffered channel behind `Events() <-chan Event` plus `Result(ctx) *AssistantMessage`; errors stay in-band events (`error{reason: aborted|error}`), never Go errors after invocation; `Complete = Stream().Result()`.
  - AbortSignal → `context.Context` as first param everywhere; ctx cancellation → `stopReason:"aborted"`.
  - Options as plain structs (`*StreamOptions`, `SimpleStreamOptions` embedding it), not functional options, mirroring TS shape.
  - `Tool.parameters` → `json.RawMessage`.

## Out of scope

- Provider adapters, auth, catalog, transform — later epics.
- Upstream's explicitly deprecated shims (`compat.ts` global API, `legacy-api-aliases.ts`) — skipped project-wide, recorded as deviations in PORTING.md.

## Acceptance criteria

- Union JSON goldens match the TS wire format byte-for-byte.
- SSE and partial-JSON fixture suites (fixtures copied verbatim from upstream `test/data`) pass.
- Cost tables pass (`tokens`, `anthropic-cache-write-1h-cost`).
- `go test ./... -race` green with no network.

## Dependencies

None — this is the root of the dependency graph. Blocks every other epic.

## Notes

- **Status: DONE.** Delivered in commit `5cdf6be` ("Phase 1: domain core — unified types, events, stream, cost, classifiers"). This epic exists for the record; its GitHub issue is created closed.
- **Layout deviation from the plan:** `ai/internal/httpx` was not created as a package — retry classifier (`ai/retry.go`), overflow regexes (`ai/overflow.go`), and unicode scrub (`ai/sanitize.go`) live in the `ai` root instead. Later epics should reference the actual layout.
- Key risk #1 from the plan (`ai/internal/partialjson` cascade, no Go equivalent) was addressed here and is fixture-locked.
- Project-wide (not specific to this epic): TDD gate — port upstream tests first (red) → implement (green) → golden-request snapshots via `OnPayload` vs TS-captured goldens; every ported Go file carries `// Ports: packages/ai/src/… @ <sha>`; PORTING.md holds the mapping + deviations; `upstream/UPSTREAM.lock` pins the upstream SHA (v0.80.3 snapshot).
