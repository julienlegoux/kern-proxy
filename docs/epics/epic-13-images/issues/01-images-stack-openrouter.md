---
type: Issue
title: "Add the image-generation stack and openrouter-images adapter"
description: "ai/images parallel pipeline plus the openrouter-images adapter — the whole epic in one issue."
tags: [epic-13]
timestamp: 2026-07-07T08:28:03Z
epic: 13
issue: 01
slug: images-stack-openrouter
size: M
status: open
gh_issue: 43
resource: https://github.com/julienlegoux/kern-proxy/issues/43
depends_on: []
blocked_by: ["#37"]
---

# Add the image-generation stack and openrouter-images adapter

## Summary

Port the image-generation stack — the whole of [Epic 13](/epic-13-images/EPIC_13.md) in one issue: a pipeline parallel to (and separate from) text streaming, plus its one adapter, openrouter-images. Image-model catalog entries come from `image-models.generated.ts` via Epic 11's export-catalog JSON.

## Scope

- `ai/images`: image request/result types, the generation entrypoint, auth resolution through Epic 3's core.
- The openrouter-images adapter.
- Catalog integration: image models loaded from the embedded JSON ([Epic 11](/epic-11-catalog-all-providers/EPIC_11.md)).
- TDD gate: port the upstream images tests first (httptest fixtures); goldens where upstream asserts request shapes.
- `// Ports:` headers; flip the matching PORTING.md rows (`src/images*.ts`, `src/api/openrouter-images.ts` → `ai/images`).

## Out of scope

- Text-streaming providers — epics 5–10.
- New image providers beyond upstream parity.

## Acceptance criteria / Definition of done

- Upstream images test ports pass — this closes the epic's acceptance criteria.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- New: `ai/images/` — path follows the plan's layout; no code exists there yet.
- Read-only context: `ai/catalog` (image-model entries, from Epic 11), `ai/auth.go` / `ai/resolve.go`.

## Dependencies

None within this epic. Cross-epic: needs Epic 1 and Epic 3 (done/underway); catalog entries via [Epic 11 issue 01](/epic-11-catalog-all-providers/issues/01-export-catalog-embedded-catalog.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split the stack from the adapter.
