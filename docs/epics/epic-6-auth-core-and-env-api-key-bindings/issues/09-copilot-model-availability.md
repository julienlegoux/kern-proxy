---
type: Issue
title: "Copilot: policy-state model fallback for individual accounts, and a disposition for the unported availability calls"
description: "Port upstream's individual-account fallback to policy.state == enabled when no model reports model_picker_enabled, and settle whether the OAuth-side availability and policy-enable calls stay unported."
tags: [epic-6]
timestamp: 2026-08-09T10:24:00Z
epic: 6
issue: 09
slug: copilot-model-availability
size: M
status: open
gh_issue: 178
resource: https://github.com/kern-ia/kern-link/issues/178
depends_on: [1]
---

# Copilot: policy-state model fallback for individual accounts, and a disposition for the unported availability calls

## Summary

`auth/oauth/github-copilot.ts` changed its model-selection rule in range, and
the reason is in the comment upstream added:

> Some Individual accounts return false for every picker flag despite explicit
> enabled policies. Limit the fallback to that endpoint so other account types
> keep strict picker semantics.

Concretely, `isSelectableCopilotModel` was dissolved into
`parseAvailableCopilotModelIds(raw, allowPolicyFallback)`:

- `capabilities.supports.tool_calls === false` still disqualifies a model
  outright, first;
- `model_picker_enabled === true && policy?.state !== "disabled"` collects the
  **picker** ids;
- `policy?.state === "enabled"` collects a second **policy-enabled** list;
- the result is the picker list — unless it is empty **and**
  `allowPolicyFallback`, in which case it is the policy-enabled list;
- `allowPolicyFallback` is true only when the resolved base URL is exactly
  `https://api.individual.githubcopilot.com`.

kern-link implements the *old* rule, but in a different place. The OAuth flow's
`fetchAvailableGitHubCopilotModelIds` was deliberately **not ported**
(`ai/auth/oauth/copilot.go:9-20` records why); instead
`ai/providers/github_copilot.go:61` `isSelectableCopilotModel` applies the same
rule inside the binding's `RefreshModels`, against the `COPILOT_GITHUB_TOKEN`
env fallback. Its base URL is a package var defaulting to exactly
`https://api.individual.githubcopilot.com` (`:33`) — the endpoint upstream's
fallback is scoped to — so the fallback is *always* on for kern-link's current
default and *must not* be when a test or an enterprise domain points that var
elsewhere.

So this issue does two things, and the second is why it is not an S:

1. **Port the rule** into `ai/providers/github_copilot.go`, gated on the same
   base-URL equality upstream gates on.
2. **Settle the standing deviation.** `ai/auth/oauth/copilot.go`'s header calls
   the unported availability/policy-enable calls "a natural follow-up once this
   flow is consumed (Epic 14)" — a follow-up that never got an epic. Upstream
   just changed that code, which is exactly the situation
   [decision 02](../../../planning/scope/02-parity-bar.md)'s disposition gate
   exists for. Either wire it, or write the deviation down with a revisit
   trigger. `Provider.FilterModels`
   ([issue 05](/epic-6-auth-core-and-env-api-key-bindings/issues/05-models-availability.md))
   is the hook that makes wiring it cheap: it hands a provider its stored
   credential and lets it narrow the catalog, which is precisely what
   `availableModelIds` was for.

**Recommended**: port the rule (1), and take the narrow half of (2) — implement
`FilterModels` on the Copilot binding to honor an `availableModelIds` entry in
the stored OAuth credential's `Extra` when one is present, leaving the
post-login `enableAllGitHubCopilotModels` calls unported with a written
deviation (they mutate account state during login, which is a side effect this
port has never taken). If the implementer disagrees, the deviation entry must
say so with its reason — the one outcome that is not acceptable is leaving it
undispositioned again.

## Scope

- `ai/providers/github_copilot.go`:
  - replace `isSelectableCopilotModel` (`:59-70`) with the two-list parse:
    `toolCalls == false` disqualifies; picker list; policy-enabled list;
    fall back only when the picker list is empty **and** the fallback is
    allowed.
  - `allowPolicyFallback := githubCopilotModelsBaseURL == "https://api.individual.githubcopilot.com"`
    — computed from the same value the request is made against, not from a
    constant, so an enterprise or test base URL keeps strict semantics.
  - keep `refreshGitHubCopilotModels`'s existing behavior of returning the
    static catalog unchanged when there is no token (`:76-79`).
- `ai/auth/oauth/copilot.go` — rewrite the "Not ported" block of the `// Ports:`
  header to state the *current* decision, not Epic 14's intention, and point at
  the `docs/PORTING.md` row.
- If the recommendation is taken: `FilterModels` on the Copilot binding, reading
  `availableModelIds` from `(*ai.OAuthCredential).Extra`
  (`ai/auth.go:52-57` — `Extra` is where per-provider fields already live, and
  the Copilot `enterpriseUrl` already rides there). **A credential with no
  `availableModelIds` entry must return the models unchanged** — that is
  upstream's own back-compat rule for older stored entries.
- `docs/PORTING.md` — a row or an amended row covering
  `fetchAvailableGitHubCopilotModelIds` / `enableAllGitHubCopilotModels`, with
  a disposition and, if deviating, a revisit trigger.

## Out of scope

- The Copilot **device-code login flow** itself. Its diff in range is the
  `OAuthProviderInterface` deletion and signal threading — neither applies.
  `IsSubscription: true` is set in
  [issue 01](/epic-6-auth-core-and-env-api-key-bindings/issues/01-auth-contract-surface.md).
- Copilot **dynamic request headers** — [Epic 5 issue 01](/epic-5-remaining-adapters/issues/01-copilot-dynamic-headers.md)
  (`#159`) owns `github-copilot-headers.ts`. Different file, different concern.
- `providers/github-copilot.ts`'s +13 in range beyond the auth strategy —
  binding changes belong to whichever epic claims them; note anything left over
  in the PR body for [Epic 9](/epic-9-classifier-audit-and-release/EPIC_9.md)'s
  disposition sweep.
- Rewriting `RefreshModels`'s contract — [Epic 2 issue 08](/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md)
  turns it into `FetchModels`. If that landed first, apply this rule inside the
  new shape; do not fight it.

## Acceptance criteria / Definition of done

- [ ] `TestCopilotPickerModelsWinWhenPresent` — a `/models` response mixing
      `model_picker_enabled: true` and policy-enabled-only entries yields only
      the picker entries, even at the individual base URL.
- [ ] `TestCopilotFallsBackToPolicyEnabledForIndividualAccounts` — a response
      where **no** model sets `model_picker_enabled` yields the
      `policy.state == "enabled"` models, at
      `https://api.individual.githubcopilot.com`.
- [ ] `TestCopilotNoFallbackOffTheIndividualEndpoint` — the same response with
      `githubCopilotModelsBaseURL` pointed at an `httptest` server yields **no**
      models. This is the gate; without it the fallback silently applies
      everywhere.
- [ ] `TestCopilotToolCallsFalseDisqualifiesEvenWhenPolicyEnabled` —
      `capabilities.supports.tool_calls: false` plus `policy.state: "enabled"`
      is excluded from both lists.
- [ ] `TestCopilotRefreshWithoutTokenReturnsCatalog` — existing behavior,
      unregressed.
- [ ] If `FilterModels` is implemented:
      `TestCopilotFilterModelsHonorsStoredAvailableModelIds` (a stored
      credential listing two ids narrows the catalog to those two) and
      `TestCopilotFilterModelsPassesThroughWithoutTheField` (a credential
      without the entry returns every model).
- [ ] `docs/PORTING.md` contains no undispositioned upstream symbol from
      `src/auth/oauth/github-copilot.ts`: every unported function is named with
      a reason and a revisit trigger.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `fix(providers): fall back to copilot policy state for individual accounts`.

## Relevant files / areas

- `ai/providers/github_copilot.go:33` the base-URL var, `:38-57` the response
  structs, `:59-70` `isSelectableCopilotModel`, `:76-116`
  `refreshGitHubCopilotModels`, `:118` the binding.
- `ai/auth/oauth/copilot.go:1-20` — the `// Ports:` header recording the
  deviation, `:106-116` the base-URL derivation including the individual
  default.
- `ai/auth.go:48-57` — `OAuthCredential.Extra`.
- `docs/PORTING.md:43` — the `ai/auth/oauth` row.
- Upstream: `src/auth/oauth/github-copilot.ts` at `936aff00`
  (`parseAvailableCopilotModelIds`, `fetchAvailableGitHubCopilotModelIds`,
  `enableAllGitHubCopilotModels`).
- Upstream test: `test/github-copilot-oauth.test.ts` (+201/- in range).

## Dependencies

- **Blocked by**: [Issue 01](/epic-6-auth-core-and-env-api-key-bindings/issues/01-auth-contract-surface.md)
  (the rename touches `ai/auth/oauth/copilot.go`). If `FilterModels` is
  implemented, also [issue 05](/epic-6-auth-core-and-env-api-key-bindings/issues/05-models-availability.md),
  which declares it.
- **Blocks**: Nothing.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
