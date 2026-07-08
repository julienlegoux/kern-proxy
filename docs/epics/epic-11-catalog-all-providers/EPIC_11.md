---
type: Epic
title: "Catalog & all provider bindings"
description: "export-catalog tool, go:embed model catalog, ~35 thin provider bindings, and native RefreshModels implementations."
tags: [epic]
timestamp: 2026-07-08T17:00:00Z
epic: 11
slug: catalog-all-providers
status: done
gh_issue: 12
milestone: 11
resource: https://github.com/julienlegoux/kern-proxy/issues/12
source: docs/PLAN.md#phases-dependency-ordered-tdd-gates
---

# Epic 11: Catalog & all provider bindings

## Goal

Ship the model catalog and wire every provider: with all wire-protocol adapters done (epics 5–10), the ~35 providers become thin bindings over them, and the model catalog is embedded from upstream-generated JSON.

## Scope

- `tools/export-catalog`: a ~50-line tsx script run against the pinned upstream checkout that serializes `models.generated.ts` + `image-models.generated.ts` to JSON.
- `ai/catalog`: `go:embed data/*.json`, `BuiltinModel(s)`/`Providers` loaders.
- `ai/providers`: `all.go` + ~35 thin provider bindings over the adapters.
- Native Go `RefreshModels` for providers with live model refresh: OpenRouter, Vercel gateway, NVIDIA, Copilot policy.

## Out of scope

- Porting the ~2100-line `generate-models.ts` — explicitly deferred (plan's catalog strategy). Its value is upstream's hand-maintained fixups; the export-catalog script gives byte-parity with zero Go churn. A native Go generator is an optional far-future phase.

## Acceptance criteria

- Catalog validation test ports pass: `models-runtime`, `providers`, per-provider model tests.
- Catalog test enforces that each model's `Compat` sub-struct matches its `Api` (e.g. `*OpenAICompletionsCompat` only on completions models).

## Dependencies

- All adapter epics: [Epic 5](/epic-5-anthropic-adapter/EPIC_5.md), [Epic 6](/epic-6-openai-completions/EPIC_6.md), [Epic 7](/epic-7-responses-family/EPIC_7.md), [Epic 8](/epic-8-google-vertex/EPIC_8.md), [Epic 9](/epic-9-mistral/EPIC_9.md), [Epic 10](/epic-10-bedrock/EPIC_10.md).

## Notes

- Size: M.
- Catalog sync becomes one command (rerun export-catalog + catalog tests) — this is the linchpin of the standing upstream-sync workflow finished in [Epic 14](/epic-14-cli-sync-tooling/EPIC_14.md).
- Project-wide: TDD gate; `// Ports:` headers; PORTING.md mapping discipline.
