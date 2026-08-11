---
type: Issue
title: "Bind baseten and the three qwen-token-plan providers"
description: "Add the four new env-API-key provider bindings upstream shipped in range, each one EnvAPIKeyAuth over the openai-completions adapter, and register them in ai/providers/all.go."
tags: [epic-6]
timestamp: 2026-08-11T13:05:00Z
epic: 6
issue: 10
slug: env-api-key-bindings
size: S
status: open
gh_issue: 179
resource: https://github.com/kern-ia/kern-link/issues/179
depends_on: []
---

# Bind baseten and the three qwen-token-plan providers

## Summary

Four of upstream's new provider files in range are plain env-API-key bindings —
15 lines each, all four over `openai-completions`, no OAuth, no dynamic refresh:

| Provider id | Name | Base URL | Env var |
|---|---|---|---|
| `baseten` | Baseten | `https://inference.baseten.co/v1` | `BASETEN_API_KEY` |
| `qwen-token-plan` | Qwen Token Plan | `https://token-plan.ap-southeast-1.maas.aliyuncs.com/compatible-mode/v1` | `QWEN_TOKEN_PLAN_API_KEY` |
| `qwen-token-plan-cn` | Qwen Token Plan CN | `https://token-plan.cn-beijing.maas.aliyuncs.com/compatible-mode/v1` | `QWEN_TOKEN_PLAN_CN_API_KEY` |
| `qwen-token-plan-individual` | Qwen Token Plan Individual | `https://token-plan.ap-southeast-1.maas.aliyuncs.com/compatible-mode/v1` | `QWEN_TOKEN_PLAN_API_KEY` |

Two details that look like typos and are not — copy them exactly:

- `qwen-token-plan-individual` shares **`QWEN_TOKEN_PLAN_API_KEY`** with
  `qwen-token-plan`, not a variable of its own. `env-api-keys.ts`'s map agrees.
- `qwen-token-plan-individual` shares the **`ap-southeast-1` base URL** with
  `qwen-token-plan`. Only the `-cn` variant differs.

Each binding's `envApiKeyAuth` display name is
`"<Provider Name> API key"` — `"Baseten API key"`, `"Qwen Token Plan CN API
key"`, and so on. That string is what a login prompt shows, so it is not
cosmetic.

**This issue does not create catalog data.**
[Epic 3 issue 04](/epic-3-catalog-schema-and-export-tooling/issues/04-regenerate-model-catalog.md)
(`#144`) regenerates the embedded catalog "35 to 39 provider files" — those four
are these four. `catalog.BuiltinModels(id)` returns **nil** for a provider with
no data file, per its own doc comment (`ai/catalog/catalog.go:84-89`), so a
binding landing first compiles and registers but serves nothing. That is a real
ordering constraint, not a nice-to-have: land `#144` first, or accept that these
four providers are empty until it does and say so in the PR body — either way,
`EPIC_6.md`'s acceptance criterion 4 ("each has a catalog entry") is checked by
this issue's own `TestNewBindingsServeCatalogModels` below, not left to Epic 3
to prove.

## Scope

- Four new files under `ai/providers`, one per binding, named after the provider
  id with hyphens as underscores — `baseten.go`, `qwen_token_plan.go`,
  `qwen_token_plan_cn.go`, `qwen_token_plan_individual.go` — matching the
  existing 35 (`CONVENTIONS.md`, "Naming & file layout"; see
  `xiaomi_token_plan_cn.go` for the closest precedent, in both naming and
  shape).
- Each is `ai.CreateProvider` with `ID`, `Name`, `BaseURL`,
  `Auth: ai.ProviderAuth{APIKey: auth.EnvAPIKeyAuth(...)}`,
  `Models: catalog.BuiltinModels(id)`, and
  `Api: ai.StreamFuncs{StreamFunc: openaicompletions.Stream, StreamSimpleFunc: openaicompletions.StreamSimple}`.
  Copy the exact shape from an existing openai-completions binding rather than
  re-deriving it.
- Each carries a `// Ports: packages/ai/src/providers/<file>.ts` header.
- `ai/providers/all.go` — four entries in `Providers()`, in the list's existing
  alphabetical-by-constructor order.

## Out of scope

- Catalog data — [Epic 3 issue 04](/epic-3-catalog-schema-and-export-tooling/issues/04-regenerate-model-catalog.md).
- `radius` and its `RADIUS_API_KEY` entry — [Epic 8](/epic-8-pi-messages-and-radius/EPIC_8.md),
  because it needs both the radius OAuth flow ([Epic 7](/epic-7-four-new-oauth-flows/EPIC_7.md))
  and a runtime gateway-config fetch ([decision 10](../../../planning/scope/10-new-provider-bindings.md)).
- `cloudflare-stream.ts` ([Epic 5 issue 11](/epic-5-remaining-adapters/issues/11-cloudflare-stream-classification.md))
  and `openrouter-images.ts` ([decision 14](../../../planning/scope/14-images-surface.md))
  — the two remaining new `providers/*.ts` files, neither of which is a binding.
- Any change to `auth.EnvAPIKeyAuth` itself —
  [issue 07](/epic-6-auth-core-and-env-api-key-bindings/issues/07-anthropic-auth-token.md)
  makes its one change (propagating a stored credential's `Env`).
- **The four rows these bindings need in `docs/auth.md`'s env-key table
  (`:34-59`)** — [issue 11](/epic-6-auth-core-and-env-api-key-bindings/issues/11-porting-paths-and-dispositions.md)
  (#180) owns that table and lands last, so all of this epic's env changes are
  written up once rather than four times. Named here because the table is
  otherwise the kind of documentation that quietly stays wrong: this issue is
  what makes it wrong.

## Acceptance criteria / Definition of done

- [ ] `TestProvidersIncludesTheNewEnvAPIKeyBindings` — `providers.Providers()`
      contains all four ids exactly once each, and the total count is 39.
- [ ] `TestNewBindingsResolveTheirEnvVars` — for each of the four, an
      `AuthContext` supplying only that provider's variable resolves to an
      `AuthResult` whose `Auth.APIKey` is the value and whose `Source` is the
      variable name; with the variable unset, resolution returns `(nil, nil)`.
- [ ] `TestQwenIndividualSharesTheTokenPlanEnvVar` — `qwen-token-plan-individual`
      resolves from `QWEN_TOKEN_PLAN_API_KEY` and **not** from
      `QWEN_TOKEN_PLAN_INDIVIDUAL_API_KEY`, `got`/`want`.
- [ ] `TestNewBindingsBaseURLs` — the four base URLs match the table above
      exactly, including `qwen-token-plan-individual` sharing
      `ap-southeast-1` with `qwen-token-plan`.
- [ ] Each binding's `Auth().OAuth` is nil and `Auth().APIKey` is non-nil.
- [ ] `TestNewBindingsServeCatalogModels` — each of the four ids returns a
      non-empty `catalog.BuiltinModels(id)`. If
      [Epic 3 issue 04](/epic-3-catalog-schema-and-export-tooling/issues/04-regenerate-model-catalog.md)
      (`#144`) has not merged yet, skip the test with `t.Skip` naming `#144`
      and say so in the PR body — do not delete or weaken the assertion.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(providers): bind baseten and the three qwen-token-plan providers`.

## Relevant files / areas

- `ai/providers/xiaomi_token_plan_cn.go` — the closest existing precedent
  (env-key auth over openai-completions, hyphenated id).
- `ai/providers/all.go:22-...` — `Providers()`.
- `ai/auth/helpers.go:17` — `EnvAPIKeyAuth`.
- `ai/catalog/catalog.go:86-89` — `BuiltinModels`, and its behavior for an
  unknown provider id.
- `ai/providers/providers_test.go` — where the registry-level assertions live.
- Upstream: `src/providers/baseten.ts`, `qwen-token-plan.ts`,
  `qwen-token-plan-cn.ts`, `qwen-token-plan-individual.ts` (15 lines each) and
  `src/env-api-keys.ts`'s map additions, at `936aff00`.

## Dependencies

- **Blocked by**: None inside this epic.
- **Ordering**: land after [Epic 3 issue 04](/epic-3-catalog-schema-and-export-tooling/issues/04-regenerate-model-catalog.md)
  (`#144`) if you want these providers to serve models on merge.
- **Blocks**: Nothing.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
