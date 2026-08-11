---
type: Issue
title: "Add Models.CheckAuth, Models.GetAvailable, and Provider.FilterModels"
description: "Port the availability half of upstream's Models auth surface: a side-effect-free per-provider auth check, the models a configured provider can actually serve, and the credential-scoped filter that narrows them."
tags: [epic-6]
timestamp: 2026-08-11T20:00:00Z
epic: 6
issue: 05
slug: models-availability
size: M
status: open
gh_issue: 174
resource: https://github.com/kern-ia/kern-link/issues/174
depends_on: ["01", "03"]
---

# Add Models.CheckAuth, Models.GetAvailable, and Provider.FilterModels

## Summary

[Epic 2 issue 08](/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md)
explicitly deferred `Provider.filterModels`, `Models.getAvailable` and
`Models.checkAuth` to this epic, because they depend on `AuthCheck` from
`src/auth/types.ts`. Those types now exist
([issue 01](/epic-6-auth-core-and-env-api-key-bindings/issues/01-auth-contract-surface.md)),
so this issue lands the availability half of the surface. The login/logout half
is [issue 06](/epic-6-auth-core-and-env-api-key-bindings/issues/06-models-login-logout.md).

Three pieces, one PR, because each is useless without the others:

```ts
checkAuth(providerId: string, options?): Promise<AuthCheck | undefined>;
getAvailable(providerId?: string, options?): Promise<readonly Model<Api>[]>;
// on Provider:
filterModels?(models, credential): readonly Model<TApi>[];
```

The point of `checkAuth` is that it is **cheaper and safer than resolving**. Its
private helper `checkProviderAuth` short-circuits three ways before it ever
resolves:

1. a stored **OAuth** credential → `{source: "OAuth", type: "oauth"}` with no
   refresh, no network, no lock;
2. an `ApiKeyAuth.check` hook, if the provider declares one → its result,
   wrapped on failure as `ModelsError("auth", "API key auth check failed for
   provider " + provider.id)`;
3. otherwise fall back to a full `resolveProviderAuth`, reporting
   `{source: resolution.source, type: "api_key"}`.

Step 1 is the behavioral difference that matters: today the only way to ask "is
this provider configured" in the Go port is `GetAuth`, which for a stored OAuth
credential takes the credential-store lock and may fire a refresh. A status
screen listing 39 providers must not do that.

`getAvailable` is `checkAuth` applied across providers, keeping only those that
check out, then narrowing each provider's catalog through `filterModels`. Its
doc comment states the invariant to preserve: *"`getModels()` remains the
complete synchronous catalog; `Models.getAvailable()` applies this filter after
confirming that provider auth is configured."* `GetModels` must not change.

No built-in provider implements `filterModels` at `936aff00` — the hook exists
for credential-scoped catalogs, and GitHub Copilot is its natural first consumer
(see [issue 09](/epic-6-auth-core-and-env-api-key-bindings/issues/09-copilot-model-availability.md),
which decides whether kern-link takes it). Land the hook here; leave every
binding's implementation nil.

## Scope

- `ai/provider.go`:
  - `Provider` interface (`:15-42`) gains
    `FilterModels(models []*Model, credential Credential) []*Model`. Go
    interfaces have no optional members, so the contract is **"return `models`
    unchanged when there is no policy"**, implemented once in `providerImpl` and
    documented as such — not a nil-returning stub. A caller must be able to use
    the result without a nil check.
  - `CreateProviderOptions` gains
    `FilterModels func(models []*Model, credential Credential) []*Model`, nil by
    default.
  - `Models` interface (`:47-77`) gains:
    - `CheckAuth(ctx context.Context, providerID string) (*AuthCheck, error)` —
      `(nil, nil)` for an unknown provider or one that is not configured,
      matching `GetAuth`'s existing "(nil, nil) when the provider is unknown or
      unconfigured" contract.
    - `GetAvailable(ctx context.Context, providerID string) ([]*Model, error)` —
      empty `providerID` means all providers, matching `GetModels`'s existing
      convention rather than inventing a second one.
  - `modelsImpl`: `checkProviderAuth` per the three-step order above, plus the
    two public methods.
- Concurrency: upstream runs the per-provider checks with `Promise.all`. Go's
  equivalent is an `errgroup`-free `sync.WaitGroup` fan-out (**no new direct
  dependencies** — `CONVENTIONS.md`), or a sequential loop if the fan-out costs
  more than it saves. Whichever you pick, one provider's failure must fail
  `GetAvailable` — upstream uses `Promise.all`, not `allSettled`, and that is
  deliberate: an unreadable credential store is not "this provider has no
  models".
- Error text, verbatim: `API key auth check failed for provider %s`,
  `Credential store read failed for %s` (the latter already exists in
  `ai/resolve.go:206-212` — reuse `readCredential`, do not write a second one).

## Out of scope

- `Models.Login` / `Logout` / the provider-id `GetAuth` overload —
  [issue 06](/epic-6-auth-core-and-env-api-key-bindings/issues/06-models-login-logout.md).
- Any binding's `FilterModels` implementation, Copilot included —
  [issue 09](/epic-6-auth-core-and-env-api-key-bindings/issues/09-copilot-model-availability.md).
- `ApiKeyAuth.Check` implementations — the hook is declared in
  [issue 01](/epic-6-auth-core-and-env-api-key-bindings/issues/01-auth-contract-surface.md)
  and upstream ships none; this issue only *calls* it.
- `resolveRefreshCredential`, upstream's refresh-path credential resolution.
  [Epic 2 issue 08](/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md)
  already ruled that `ai/resolve.go` covers it.
- `CredentialStore.List` — [issue 02](/epic-6-auth-core-and-env-api-key-bindings/issues/02-credential-store-list.md).
  `GetAvailable` reads per provider; it must not be rewritten onto `List`.

## Acceptance criteria / Definition of done

- [ ] `TestCheckAuthOnStoredOAuthDoesNotRefresh` — a stored OAuth credential
      already **inside** the refresh window returns
      `AuthCheck{Source: "OAuth", Type: "oauth"}` with the flow's `Refresh`
      func recording zero calls. This is the whole point of the method.
- [ ] `TestCheckAuthUsesTheCheckHookWhenPresent` — a provider declaring
      `APIKeyAuth.Check` returns that hook's `AuthCheck` and never calls
      `Resolve`.
- [ ] `TestCheckAuthFallsBackToResolution` — no hook, no stored credential, an
      ambient env var set: `AuthCheck{Source: "<ENV_VAR>", Type: "api_key"}`.
- [ ] `TestCheckAuthUnknownProviderIsNotAnError` — `(nil, nil)`, `got`/`want`.
- [ ] `TestCheckHookFailureIsAModelsError` — a hook returning an error surfaces
      as `*ModelsError`, `Code() == "auth"`, message exactly
      `API key auth check failed for provider <id>`.
- [ ] `TestGetAvailableOmitsUnconfiguredProviders` — two registered providers,
      one configured: exactly the configured provider's models come back, and
      `GetModels("")` still returns both providers' catalogs unchanged.
- [ ] `TestGetAvailableAppliesFilterModels` — a provider whose `FilterModels`
      drops all but one model yields that one model, and the filter receives the
      stored credential it was resolved against.
- [ ] `TestFilterModelsDefaultsToIdentity` — a provider constructed without the
      option returns its models unchanged through `Provider.FilterModels`, no
      nil.
- [ ] `TestGetAvailableForOneProvider` — passing a provider id restricts the
      result to that provider; passing `""` covers all.
- [ ] Every `checkAuth`/`getAvailable` case in upstream's `test/oauth-auth.test.ts`
      (+67/- in range) is represented by a Go test in this PR or explicitly
      dispositioned in the PR body. This file's types/`Check` cases belong to
      [issue 01](/epic-6-auth-core-and-env-api-key-bindings/issues/01-auth-contract-surface.md)
      and its expiry-window cases to
      [issue 04](/epic-6-auth-core-and-env-api-key-bindings/issues/04-oauth-refresh-window.md) —
      do not duplicate either here.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] `ai/provider.go` keeps its `// Ports:` header; the new interface methods
      carry contract-style doc comments (`CONVENTIONS.md`: "Doc comments are
      contracts, not descriptions").
- [ ] Conventional Commit, e.g.
      `feat(ai): report configured providers and their available models`.

## Relevant files / areas

- `ai/provider.go:15-42` `Provider`, `:44-77` `Models`, `:148` `GetProviders`,
  `:221` `GetAuth`, and `CreateProviderOptions`.
- `ai/resolve.go:206-212` `readCredential` — reuse it.
- `ai/auth.go` — `AuthCheck`, `AuthType` (from issue 01), `Credential`.
- `ai/providers/faux` — the in-process provider these tests should lean on
  (`CONVENTIONS.md`: "Everything runs offline").
- Upstream: `src/models.ts` at `936aff00` — `checkProviderAuth`, `checkAuth`,
  `getAvailable`, `Provider.filterModels`.
- Upstream test: `test/oauth-auth.test.ts` (+67/- in range).

## Dependencies

- **Blocked by**: [Issue 01](/epic-6-auth-core-and-env-api-key-bindings/issues/01-auth-contract-surface.md)
  (`AuthCheck`, `AuthType`, `APIKeyAuth.Check`),
  [issue 03](/epic-6-auth-core-and-env-api-key-bindings/issues/03-provider-scoped-apikey-resolution.md)
  (`ResolveProviderAuth`'s signature).
- **Blocks**: [Issue 06](/epic-6-auth-core-and-env-api-key-bindings/issues/06-models-login-logout.md)
  — both issues rewrite the `Models` interface and `modelsImpl` in
  `ai/provider.go`; land this issue first and have issue 06 rebase onto it.
  [Issue 09](/epic-6-auth-core-and-env-api-key-bindings/issues/09-copilot-model-availability.md)
  needs `FilterModels` to exist before it can decide whether Copilot implements it.
- Also touches `ai/provider.go`, which
  [Epic 2 issues 05/08/09](/epic-2-core-types-and-models-contracts/issues/index.md)
  rewrite heavily. Rebase onto epic 2's integration state rather than the other
  way round.

## PR size note

`M` — ~470 changed lines: three surfaces land together (`Provider.FilterModels`
plus its `CreateProviderOptions` field, `Models.CheckAuth`,
`Models.GetAvailable`), with `checkProviderAuth`'s three-step short-circuit and
the per-provider fan-out, against nine named tests driven through
`ai/providers/faux`. That is the top of the band. Split past ~500, and the seam
is `FilterModels` plus its identity default, which can land before the two
`Models` methods that consume it.
