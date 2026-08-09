---
type: Epic
title: "Catalog schema and export tooling"
description: "Price and update tools/export-catalog against the frozen upstream, regenerate the model and image catalogs, and re-validate them — including the images surface that rides on the same tooling."
tags: [epic]
timestamp: 2026-08-09T03:55:05Z
epic: 3
slug: catalog-schema-and-export-tooling
status: open
gh_issue: 120
milestone: 20
resource: https://github.com/kern-ia/kern-link/issues/120
source: docs/planning/SCOPE.md#milestone-3-catalog-schema-and-export-tooling
---

# Epic 3: Catalog schema and export tooling

## Goal

`sync.sh` buckets these files as "re-run `tools/export-catalog` only". That
label is wrong for this sync, and treating it as right is the main trap of the
whole program: five upstream generator scripts changed
(`generate-models.ts`, `check-model-data.ts`, `generate-image-models.ts`,
`model-data.ts`, `models-dev-reasoning-options.ts`), so the exporter may not run
against `936aff00` at all.

## Scope

- **First issue is a tooling spike**: confirm `tools/export-catalog` runs green
  against upstream at `936aff00`. Price this before planning the rest of the
  epic.
- Then, strictly in this order: update `tools/export-catalog` → regenerate all
  catalog JSON → update `ai/catalog` validation → run the catalog tests. Code
  before data — the Go structs that read the catalog ship in epic 2, and the
  regenerated tree fails validation at load otherwise.
- 41 `*.models.ts` plus `models.generated.ts` and `image-models.generated.ts`.
- **Images surface rides along**: `images-models.ts`, the OpenRouter images
  adapter, and the regenerated image catalog. Same tooling, so a separate epic
  would touch it twice.

## Out of scope

- The core Go structs that read the catalog — epic 2 ships those.
- Provider bindings and OAuth flows whose catalog entries other epics add
  (epics 6, 7, 8 each carry their own entries).
- Coverage tooling or thresholds.

## Acceptance criteria

1. The spike's finding is recorded before the rest of the epic is planned:
   whether `tools/export-catalog` runs against `936aff00` as-is, and what it
   costs if not.
2. `tools/export-catalog` runs green against upstream at `936aff00`.
3. All catalog JSON is regenerated from that revision, and `ai/catalog`
   validation accepts it at load.
4. The catalog tests pass, including the image catalog.
5. **Any change to the OpenRouter images adapter gets an offline `httptest`
   case** — its only existing test is an env-gated live smoke, so a live-only
   test proves nothing in CI.
6. CI green: `go test ./... -race -v`, `bash upstream/sync_test.sh`,
   `golangci-lint` v2.12.2.

## Dependencies

- [Epic 2: Core types and Models contracts](/epic-2-core-types-and-models-contracts/EPIC_2.md)
  — the catalog-reading structs must exist before the regenerated tree lands.

## Context

- [Technical specs](../../planning/SPECS.md)
- [Conventions](../../planning/CONVENTIONS.md)
- [Upstream sync scope](../../planning/SCOPE.md)
- [Decision 05 — catalog schema and tooling](../../planning/scope/05-catalog-schema-and-tooling.md)
- [Decision 14 — images surface](../../planning/scope/14-images-surface.md)
- [Decision 21 — risks and assumptions](../../planning/scope/21-risks-and-assumptions.md)

## Notes

- Risk owned here, and the reason the spike is issue one: **`tools/export-catalog`
  may not run against `936aff00` at all.** The spike prices it before the rest
  is planned.
- Project-wide, not this epic's own boundary: no new direct dependencies, tests
  offline and stdlib-only, `// Ports:` headers on ported files, `develop` as the
  integration trunk, and the race detector's verdict only from CI.
- The upstream target is frozen at `936aff00`; regeneration is against that
  revision and no other.
