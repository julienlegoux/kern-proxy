---
type: Issue
title: "Add the export-catalog tool and embedded model catalog"
description: "tools/export-catalog tsx script serializing upstream generated models to JSON, plus ai/catalog with go:embed loaders and catalog validation tests."
tags: [epic-11]
timestamp: 2026-07-07T08:27:44Z
epic: 11
issue: 01
slug: export-catalog-embedded-catalog
size: M
status: open
gh_issue: 37
resource: https://github.com/julienlegoux/kern-proxy/issues/37
depends_on: []
---

# Add the export-catalog tool and embedded model catalog

## Summary

Ship the model catalog for [Epic 11](/epic-11-catalog-all-providers/EPIC_11.md): a ~50-line tsx script that serializes upstream's `models.generated.ts` + `image-models.generated.ts` to JSON from the pinned checkout, and an `ai/catalog` package that embeds that JSON and exposes the loaders. This is the linchpin of the standing upstream-sync workflow — catalog sync becomes "rerun the script + run the tests".

## Scope

- `tools/export-catalog`: tsx script run against the pinned upstream checkout (`upstream/UPSTREAM.lock`), emitting `ai/catalog/data/*.json` with byte-stable output.
- `ai/catalog`: `go:embed data/*.json`, `BuiltinModel(s)` / `Providers` loaders.
- Catalog validation tests: port `models-runtime` and the per-provider model tests; enforce that each model's `Compat` sub-struct matches its `Api` (e.g. `*OpenAICompletionsCompat` only on completions models).
- Explicitly **not** porting the ~2100-line `generate-models.ts` — the plan defers it; the export script gives byte-parity with zero Go churn.
- `// Ports:` headers; flip the matching PORTING.md rows.

## Out of scope

- Provider bindings — [Issue 02](./02-core-provider-bindings.md) / [Issue 03](./03-vendor-bindings-refreshmodels.md).
- A native Go model generator — deferred per the plan's catalog strategy.

## Acceptance criteria / Definition of done

- `models-runtime` and per-provider catalog test ports pass; the Compat/Api consistency check is enforced by a test.
- Re-running export-catalog against the pinned SHA is a no-op diff.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- New: `tools/export-catalog/`, `ai/catalog/` — paths follow the plan's layout.
- Read-only context: `upstream/UPSTREAM.lock`, `ai/model.go`, `ai/provider.go`.

## Dependencies

Blocks [Issue 02](./02-core-provider-bindings.md). Epic-level: needs the adapter epics only for full binding coverage, not for the catalog itself.

## PR size note

Target ~500 changed lines of hand-written code; the generated JSON doesn't count against the budget.
