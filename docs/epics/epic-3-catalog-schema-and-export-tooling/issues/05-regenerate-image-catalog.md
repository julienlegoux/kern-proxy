---
type: Issue
title: "Regenerate the image catalog from upstream 936aff00 and cover its decode path"
description: "Refresh ai/catalog/data/images/openrouter.json to the 42 image models upstream declares at the frozen revision, and prove ai/images decodes every entry."
tags: [epic-3]
timestamp: 2026-08-11T20:00:00Z
epic: 3
issue: 05
slug: regenerate-image-catalog
size: S
status: open
gh_issue: 145
resource: https://github.com/kern-ia/kern-link/issues/145
depends_on: ["02", "03"]
---

# Regenerate the image catalog from upstream 936aff00 and cover its decode path

## Summary

The images surface rides along in this epic because it shares the export tool
([decision 14](../../../planning/scope/14-images-surface.md)). Its catalog half
is small and separable from its adapter half, so it ships on its own.

`src/image-models.generated.ts` moves from 35 to 42 models between `244f1dea`
and `936aff00`, all under the single `openrouter` image provider. Added:
`krea/krea-2-large`, `krea/krea-2-medium`, `krea/krea-2-medium-turbo`,
`microsoft/mai-image-2.5-pro`, `openrouter/auto-beta`, `qwen/qwen-image-3`,
`qwen/qwen-image-3-pro`. Nothing was removed and the `ImagesModel` interface
itself is unchanged at `936aff00` (`src/types.ts:825-830`), so
`ai/images.Model` (`ai/images/types.go:23-33`) needs no field changes.

Unlike the model catalog, this half of the export tool was never broken:
`image-models.generated.ts` is still committed upstream with its models inline
and imports only types.

The decode path is the part worth testing. `images.CatalogModels`
(`ai/images/catalog.go:17-27`) **panics** on a decode failure, and today's only
coverage of the embedded bytes is `ai/catalog/images_test.go`, which asserts
the file is non-empty — it never decodes it.

## Scope

- Regenerate `ai/catalog/data/images/openrouter.json` against upstream
  `936aff00` with the same tool run as
  [issue 04](/epic-3-catalog-schema-and-export-tooling/issues/04-regenerate-model-catalog.md)
  (the tool writes both trees in one invocation; committing them as two PRs is
  a review convenience, not two runs). **The pair shares one generator run**, so
  its upstream SHA, its date and the upstream generator command are recorded
  *identically* in both PR bodies — two different dates would mean two fetches of
  live third-party data and an inconsistent pair. Neither PR waits on the other;
  each commits only its own half.
- Extend `ai/catalog/images_test.go` and/or `ai/images/catalog_test.go` so the
  embedded image catalog is actually decoded and checked: every entry has a
  non-empty `ID`/`Name`/`BaseURL`, `Provider == "openrouter"`,
  `Api == "openrouter-images"`, `Output` non-empty, and ids are unique.
- Assert the count and the presence of the seven added ids, so a future
  regeneration that silently loses models fails.
- Confirm `images.BuiltinProviders()` / `images.BuiltinModels(nil)` still build
  from the regenerated catalog (`ai/images/builtin.go:16-30` feeds
  `CatalogModels("openrouter")` straight into the provider).

## Out of scope

- **The OpenRouter images adapter's behavior** — retry, `httptest` coverage, and
  everything upstream's `api/openrouter-images.ts` diff carries is
  [issue 06](/epic-3-catalog-schema-and-export-tooling/issues/06-images-adapter-retry.md),
  which owns the epic's acceptance criterion 5.
- **OpenRouter images OAuth.** `providers/openrouter-images.ts` at `936aff00`
  adds a `lazyOAuth`/`loadOpenRouterOAuth` arm beside the env API key; that is
  [Epic 7](/epic-7-four-new-oauth-flows/EPIC_7.md) per
  [decision 09](../../../planning/scope/09-new-oauth-flows.md). `ai/images`
  keeps env-key-only auth after this PR.
- **The `getAuth` reshape.** Upstream's `images-models.ts` now overloads
  `getAuth(providerId | model, overrides)` on a new `AuthResolutionOverrides`
  and drops the model argument from `resolveProviderAuth`. The images half of it
  is [issue 07](/epic-3-catalog-schema-and-export-tooling/issues/07-images-options-base-and-auth-overrides.md)
  (#218) — **not** [Epic 6](/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md),
  whose epic file never mentions `ai/images` and whose
  [issue 10](/epic-6-auth-core-and-env-api-key-bindings/issues/10-env-api-key-bindings.md)
  hands the images files back to this epic by name. (Issue 07 does take a
  cross-epic edge on Epic 6 issues 03 and 06 for the shared shapes; that edge is
  recorded there.) `ai/images/provider.go:213-219` keeps its current signature
  here.
- `ImagesOptions extends ProviderRequestOptions` and the `fetch` override —
  [issue 07](/epic-3-catalog-schema-and-export-tooling/issues/07-images-options-base-and-auth-overrides.md)
  (#218), which consumes the `ProviderRequestOptions` base
  [Epic 2 issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md)
  declares.
- The model catalog (issue 04).

## Acceptance criteria / Definition of done

- [ ] `ai/catalog/data/images/openrouter.json` holds 42 entries and includes
      `krea/krea-2-large`, `krea/krea-2-medium`, `krea/krea-2-medium-turbo`,
      `microsoft/mai-image-2.5-pro`, `openrouter/auto-beta`,
      `qwen/qwen-image-3`, `qwen/qwen-image-3-pro`.
- [ ] `TestCatalogModelsDecodesEveryEmbeddedImageModel` — `CatalogModels("openrouter")`
      returns 42 models, each with non-empty `ID`, `Name`, `BaseURL`, `Provider`
      `"openrouter"`, `Api` `"openrouter-images"`, and a non-empty `Output`;
      ids are unique. It must fail on a missing field, not just on a panic.
- [ ] The strict decode over the embedded image bytes **reuses
      [issue 03](/epic-3-catalog-schema-and-export-tooling/issues/03-catalog-validation-0-84-1.md)'s
      helper and adds no second sweep.** Issue 03 owns
      `TestCatalogRejectsUnknownKeys` over every embedded `data/**/*.json`,
      images included; this PR proves the regenerated bytes still pass it and
      declares no new `DisallowUnknownFields` test of its own.
- [ ] `TestBuiltinProvidersServeRegeneratedCatalog` — `images.BuiltinModels(nil)`
      resolves one of the newly added ids through `GetModel("openrouter", …)`.
- [ ] `git diff --stat ai/catalog/data/models` is empty in this PR — the mirror
      of the guard
      [issue 04](/epic-3-catalog-schema-and-export-tooling/issues/04-regenerate-model-catalog.md)
      carries, since the shared invocation writes both trees.
- [ ] The PR body records the upstream SHA, the generator command and the date
      of the run, matching issue 04's PR body exactly.
- [ ] No change to `ai/images/types.go`'s `Model` struct is needed; if one turns
      out to be, say so in the PR body rather than sneaking a type change into a
      data PR.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./ai/catalog/... ./ai/images/...` passes;
      then `GOTMPDIR=$PWD/.gotmp go test ./...`; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2).
- [ ] Discrete named test functions, standard library only, no `t.Parallel()`.
- [ ] Conventional Commit, e.g.
      `feat(catalog): regenerate the image catalog from upstream 936aff00`.

## Relevant files / areas

- `ai/catalog/data/images/openrouter.json` — 35 entries today, 42 after.
- `ai/catalog/images.go:22-49` `loadImages()`, `:61-67` `ImagesData` (raw bytes
  by design — `:1-8` explains why `ai/catalog` must not import `ai/images`).
- `ai/catalog/images_test.go` — currently asserts only non-emptiness.
- `ai/images/catalog.go:17-27` `CatalogModels`, which panics on a decode error;
  `:31-38` `CatalogModel`.
- `ai/images/types.go:23-44` — `Model` and `SupportsOutput`.
- `ai/images/builtin.go:16-40` — `OpenRouterProvider`, `BuiltinProviders`,
  `BuiltinModels`.
- `ai/images/catalog_test.go`, `builtin_test.go` — existing shapes to extend.
- Upstream at `936aff00`: `packages/ai/src/image-models.generated.ts` (639
  lines, 42 models), `packages/ai/src/types.ts:825-830` (`ImagesModel`).

## Dependencies

- **Blocked by**: [Issue 02](/epic-3-catalog-schema-and-export-tooling/issues/02-export-catalog-json-input.md)
  (the tool run that produces both trees) and
  [Issue 03](/epic-3-catalog-schema-and-export-tooling/issues/03-catalog-validation-0-84-1.md)
  (the strict-decode helper this reuses on the images side).
- **Blocks**: Nothing.

## PR size note

`S` — ~80 hand-written lines: one test file covering the decode path
(`TestCatalogModelsDecodesEveryEmbeddedImageModel` plus the builtin-provider
case), reusing issue 03's strict-decode helper rather than adding a sweep. The
regenerated `ai/catalog/data/images/openrouter.json` is generated — 712 lines
today, ~850 after — and does not count toward the band
([Epic 0](/epic-0-plan-remediation/EPIC_0.md), `## Notes`), which is the same
rule that now re-labels issue 04 to `S`. Split past ~200 hand-written lines.
