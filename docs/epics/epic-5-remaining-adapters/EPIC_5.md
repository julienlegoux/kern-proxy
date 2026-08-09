---
type: Epic
title: "Remaining adapters"
description: "Sync the Anthropic, Google/Vertex, Mistral and Bedrock adapters, close the long-standing Copilot dynamic-headers gap, and classify cloudflare-stream."
tags: [epic]
timestamp: 2026-08-09T03:55:05Z
epic: 5
slug: remaining-adapters
status: open
gh_issue: 122
milestone: 22
resource: https://github.com/kern-ia/kern-link/issues/122
source: docs/planning/SCOPE.md#milestone-5-remaining-adapters
---

# Epic 5: Remaining adapters

## Goal

The adapters outside the OpenAI family. They are mutually independent, so the
work parallelises at issue level within the epic — but each one compiles against
the same epic-2 core, which is why they are grouped rather than scattered.

## Scope

- `anthropic-messages.ts` (+252) → `ai/apis/anthropic`
- `google-generative-ai.ts`, `google-shared.ts`, `google-vertex.ts` →
  `ai/apis/google`, `ai/apis/google/vertex`
- `mistral-conversations.ts` (+423) → `ai/apis/mistral`
- `bedrock-converse-stream.ts` (+146) → `ai/apis/bedrock`
- **The Copilot dynamic-headers gap**: `src/api/github-copilot-headers.ts`
  (`X-Initiator`, `Copilot-Vision-Request`), unported since the initial port and
  logged at `docs/PORTING.md:46`. It sits outside the 0.80.3→0.84.1 delta, but
  it is the same failure mode the governing principle names — an upstream file
  with no Go base — in files this epic opens anyway. **Check whether it changed
  in range before porting it.**
- `cloudflare-stream.ts` needs a one-line classification here: it is Cloudflare
  streaming support, not a provider binding, and probably belongs beside
  `cloudflare_auth.go`.

## Out of scope

- The OpenAI-family adapters — epic 4 owns those.
- The classifier audit of the error strings these adapters emit; epic 9 audits
  after the adapters have moved.
- `pi-messages`, which is a new tenth adapter package rather than an update —
  epic 8.
- Idiomatic-Go cleanups of ported code.

## Acceptance criteria

1. Anthropic, Google, Vertex, Mistral and Bedrock match upstream at `936aff00`
   and compile against the epic-2 core.
2. The Copilot dynamic-headers file is ported (or, if it proves structurally
   non-portable, recorded as a deviation with the reason), and
   `docs/PORTING.md:46`'s entry is updated to say so.
3. `cloudflare-stream.ts` has a recorded classification and a Go home.
4. Ported upstream tests come across with the code; new coverage is offline
   `httptest`.
5. Every file touched carries a `// Ports:` header and is dispositioned in
   `docs/PORTING.md`.
6. CI green: `go test ./... -race -v`, `bash upstream/sync_test.sh`,
   `golangci-lint` v2.12.2.

## Dependencies

- [Epic 2: Core types and Models contracts](/epic-2-core-types-and-models-contracts/EPIC_2.md)

Independent of [Epic 4](/epic-4-openai-family-adapters/EPIC_4.md); the two can
run concurrently once epic 2 has merged.

## Context

- [Technical specs](../../planning/SPECS.md)
- [Conventions](../../planning/CONVENTIONS.md)
- [Upstream sync scope](../../planning/SCOPE.md)
- [Decision 06 — adapter updates](../../planning/scope/06-adapter-updates.md)
- [Decision 16 — Copilot headers gap](../../planning/scope/16-copilot-headers-gap.md)

## Notes

- Project-wide, not this epic's own boundary: **no new direct dependencies** —
  raw `net/http` + `ai/internal/sse` everywhere except Bedrock
  (`aws-sdk-go-v2`) and Vertex ADC, a deliberate deviation from upstream's SDK
  delegation that erodes one plausible commit at a time under a soft rule.
- **Error text stays upstream-verbatim**; `ai/retry.go` and `ai/overflow.go`
  classify failures by matching it.
- The governing principle: a deviation must be justified by structural
  non-portability, never by cost.
- Upstream target frozen at `936aff00`.
