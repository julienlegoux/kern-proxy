---
type: Decision
title: "Catalog schema migration and export tooling"
description: "How the embedded model catalog and its TypeScript export tool are brought to the 0.84.1 schema."
tags: [decision, scope]
timestamp: 2026-08-09T01:30:23Z
phase: scope
decision: 05
slug: catalog-schema-and-tooling
status: decided
verdict: "Regenerate the catalog and migrate its schema in one epic, sequenced after the core types. First issue verifies tools/export-catalog still runs against the frozen SHA before the rest of the milestone is planned."
decided_via: triage
depends_on: [core-types-migration]
---

# Question

The range touches 41 `src/providers/*.models.ts` files plus
`src/models.generated.ts` and `src/image-models.generated.ts`. `upstream/sync.sh`
buckets these as "re-run `tools/export-catalog` only" — but that bucket label is
wrong for this sync, and treating it as right is the main trap here.

Three things break the "just regenerate" assumption:

1. The **catalog schema changed**: `ModelCost` is now tiered
   (`ModelCostRates`/`ModelCostTier`), and `ThinkingLevel` gains `"max"`. The
   embedded JSON shape changes, and `ai/catalog` validates the tree at load.
2. The **generator scripts changed**: `scripts/generate-models.ts`,
   `check-model-data.ts`, `generate-image-models.ts`, `model-data.ts`,
   `models-dev-reasoning-options.ts`. `tools/export-catalog/export-catalog.ts`
   runs against a pinned upstream checkout and may no longer match its inputs.
3. Providers **appear and disappear**: `baseten`, `qwen-token-plan{,-cn,-individual}`
   are new catalog files (see [decision 10](/scope/10-new-provider-bindings.md)).

SPECS.md records 35 catalog files under `ai/catalog/data/models/`, embedded with
`go:embed data` and validated at load.

# Options

- **Regenerate + migrate the schema in one epic** — update `tools/export-catalog`
  to the 0.84.1 shape, regenerate all catalog JSON, update `ai/catalog`'s
  validation and the Go structs that read it.
- **Hand-patch the JSON** — no tooling change; fast now, but abandons the
  generated-catalog property that makes every future sync cheap. Strongly against.
- **Freeze the catalog at the old schema, port code only** — leaves `ai/cost.go`
  unable to price tiered models and blocks [decision 04](/scope/04-core-types-migration.md).

# Recommendation

**Regenerate plus schema migration, as its own epic, sequenced immediately after
the core-types epic.** Sequencing matters and is easy to get wrong: the Go
structs that *read* the catalog live in `ai` (decision 04), so the schema change
must land in code before the regenerated data can validate. Within the epic the
order is: update `tools/export-catalog` → regenerate → update `ai/catalog`
validation → run the catalog tests.

Also fold in a check the audit surfaced: verify `tools/export-catalog` still
runs against upstream at `936aff00` at all, before assuming it does. Five
generator scripts changed upstream; if the export tool reads their output shape,
it needs updating first, and that is a real risk worth its own acceptance
criterion rather than a surprise mid-epic.

# Verdict

**Regenerate plus schema migration, own epic, immediately after core types.**
Accepted as recommended.

Order within the epic is load-bearing and is part of the verdict: update
`tools/export-catalog` → regenerate → update `ai/catalog` validation → run the
catalog tests. The Go structs that read the catalog live in `ai` and land in
milestone 1, so the code must precede the data or the regenerated tree fails
validation at load.

The first issue of this epic is a **tooling spike**: confirm
`tools/export-catalog` runs green against upstream at `936aff00` at all. Five
generator scripts changed upstream, and if the export tool depends on their
output shape it needs repair before anything else in the milestone can be
estimated. This is risk 1 in
[decision 21](/scope/21-risks-and-assumptions.md), and pricing it first is the
mitigation.

The images surface rides along in this epic
([decision 14](/scope/14-images-surface.md)) because it shares the same
generator tooling.
