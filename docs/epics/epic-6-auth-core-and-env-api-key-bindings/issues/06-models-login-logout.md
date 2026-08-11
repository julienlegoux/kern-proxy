---
type: Issue
title: "Add Models.Login, Models.Logout, and the provider-id GetAuth overload, and drive pi-ai login through them"
description: "Port the mutating half of upstream's Models auth surface — login persists through the credential store, logout deletes, GetAuth answers for a bare provider id — and make cmd/pi-ai use it instead of calling flows directly."
tags: [epic-6]
timestamp: 2026-08-11T20:00:00Z
epic: 6
issue: 06
slug: models-login-logout
size: M
status: open
gh_issue: 175
resource: https://github.com/kern-ia/kern-link/issues/175
depends_on: ["01", "03", "05"]
---

# Add Models.Login, Models.Logout, and the provider-id GetAuth overload, and drive pi-ai login through them

## Summary

`Models` grew the mutating half of an auth API in range, and it is the half the
CLI should have been using all along. `cmd/pi-ai/oauth.go:72` calls
`selected.Auth.Login(ctx, callbacks)` and persists the credential itself —
before `Models.login` existed, that was the only option. Now the library owns
it.

```ts
login(providerId: string, type: AuthType, interaction: AuthInteraction): Promise<Credential>;
logout(providerId: string, options?): Promise<void>;
getAuth(providerId: string, overrides?): Promise<AuthResult | undefined>;
getAuth(model: Model<Api>, overrides?): Promise<AuthResult | undefined>;
```

Three behaviors to get right, each with its own error string:

- **`login` picks the method by `AuthType`** — `provider.auth.oauth` or
  `provider.auth.apiKey` — and fails before touching the network when it is
  missing: `` `${provider.name} does not support ${type} login` `` (code `auth`),
  or `` `Unknown provider: ${providerId}` `` (code `provider`). Note the first
  uses the provider's **display name**, the second its **id**; that asymmetry is
  upstream's and the tests must pin it.
- **`login` persists through `credentials.modify`**, not by assignment, so a
  login serializes against a concurrent refresh. On store failure:
  `` `Credential store modify failed for ${providerId}` ``. Upstream wraps this
  in an elaborate "did the mutation start before the abort" dance
  (`mutationStarted`, `markMutationStarted`, a `Promise.race`) to answer one
  question: *if the caller cancels while the write is already in flight, do not
  reject — the credential is being written.* In Go that question is answered by
  where you check `ctx.Err()`: check it before entering `Modify`, and let
  `Modify` run to completion once entered. Do not port the promise choreography;
  port the guarantee, and comment it.
- **`getAuth` splits by argument.** The provider-id form resolves and returns.
  The model form additionally merges the **model's static headers** over the
  resolved auth headers. That merge is what
  [Epic 2 issue 09](/epic-2-core-types-and-models-contracts/issues/09-models-request-transforms.md)
  pointed here when it wrote "upstream dropped its `model` parameter and now
  merges static model headers in `getAuth`". Go has `Model.Headers`
  (`ai/model.go:67`) and `MergeProviderHeaders` (`ai/headers.go:38`) already —
  use them; do not write a third merge.

Go has no overloads, so this is two methods. `GetAuth(ctx, model)` keeps its
name and current behavior plus the header merge; the new one is
`GetAuthForProvider(ctx, providerID string, overrides *AuthResolutionOverrides)`.
Name it in the PR body as a deliberate Go shaping of a TS overload, and record
it in `docs/PORTING.md` via
[issue 11](/epic-6-auth-core-and-env-api-key-bindings/issues/11-porting-paths-and-dispositions.md).

## Scope

- `ai/provider.go`, `Models` interface and `modelsImpl`:
  - `Login(ctx context.Context, providerID string, authType AuthType, interaction AuthInteraction) (Credential, error)`.
  - `Logout(ctx context.Context, providerID string) error` — delegates to
    `credentials.Delete`, wrapping failure as
    `Credential store delete failed for %s` (code `auth`).
  - `GetAuthForProvider(ctx context.Context, providerID string, overrides *AuthResolutionOverrides) (*AuthResult, error)`.
  - `GetAuth(ctx, model)` gains an `overrides` parameter and the static-header
    merge. **Confirm whether epic 2 already changed this signature** before
    editing; if it did, extend rather than replace.
- `cmd/pi-ai/oauth.go` — `runLogin` goes through `Models.Login` instead of
  calling `selected.Auth.Login` and persisting by hand (`:72-85`). The CLI keeps
  owning the interactive prompts (`interactiveCallbacks`, `:125`); it stops
  owning persistence. This is the behavior change users can see: a login that
  races a token refresh now serializes instead of clobbering.
- `cmd/pi-ai` gains no new subcommand. `logout` is library surface this issue
  ships; wiring a CLI command for it is not in the epic's scope and upstream's
  `cli.ts` is not in this diff.

## Out of scope

- `Models.CheckAuth` / `GetAvailable` / `Provider.FilterModels` —
  [issue 05](/epic-6-auth-core-and-env-api-key-bindings/issues/05-models-availability.md).
- `ModelsRequestTransforms` and the case-insensitive header merge —
  [Epic 2 issue 09](/epic-2-core-types-and-models-contracts/issues/09-models-request-transforms.md).
  If that landed first, `MergeProviderHeaders` is already case-insensitive and
  this issue inherits it; do not re-implement.
- A `pi-ai logout` command, and any change to `pi-ai list`.
- `CredentialStore.List` — [issue 02](/epic-6-auth-core-and-env-api-key-bindings/issues/02-credential-store-list.md).

## Acceptance criteria / Definition of done

- [ ] `TestLoginPersistsThroughTheCredentialStore` — a successful OAuth login
      leaves the credential readable via `CredentialStore.Read`, and the store's
      `Modify` (not a bare write) is what recorded it.
- [ ] `TestLoginRejectsUnsupportedAuthType` — a provider with OAuth only, asked
      for `AuthTypeAPIKey`, fails with `*ModelsError`, `Code() == "auth"`,
      message exactly `<Provider Name> does not support api_key login` —
      asserting the **display name**, not the id.
- [ ] `TestLoginUnknownProviderIsAnError` — `*ModelsError`, `Code() ==
      "provider"`, message exactly `Unknown provider: <id>` — asserting the
      **id**.
- [ ] `TestLoginCancelledBeforeWriteDoesNotPersist` — a context cancelled before
      the flow returns leaves the store untouched and returns the context's
      error, not a `ModelsError`.
- [ ] `TestLogoutDeletesTheStoredCredential` — after `Logout`, `Read` returns
      `(nil, nil)`; a store failure surfaces as `*ModelsError` with message
      exactly `Credential store delete failed for <id>`.
- [ ] `TestGetAuthForProviderResolvesWithoutAModel` — an ambient env key
      resolves through the provider id alone.
- [ ] `TestGetAuthMergesStaticModelHeaders` — a model carrying
      `Headers{"X-Model": "m"}` over a resolution carrying
      `Headers{"X-Auth": "a"}` yields both, and a model header with the same key
      **wins** over the auth header (upstream merges the model's headers *over*
      the resolved ones).
- [ ] `TestGetAuthForProviderIgnoresModelHeaders` — the provider-id form returns
      the resolution unchanged even when the provider's models declare headers.
- [ ] `cmd/pi-ai` tests still pass, and one asserts the CLI no longer writes to
      the store directly: `TestRunLoginPersistsViaModels` (drive it with a stub
      store that counts `Modify` calls).
- [ ] `ai/provider.go` and `cmd/pi-ai/oauth.go` keep their `// Ports:` headers,
      still describing what each file ports after this change — this issue
      changes what `cmd/pi-ai/oauth.go` does.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(ai): own login and logout in Models rather than in each caller`.

## Relevant files / areas

- `ai/provider.go:44-77` `Models`, `:221-227` `GetAuth`, `:237-296` `applyAuth`.
- `ai/model.go:67` `Model.Headers`; `ai/headers.go:38` `MergeProviderHeaders`.
- `ai/resolve.go:48-53` `AuthResolutionOverrides`, `:206-212` `readCredential`.
- `cmd/pi-ai/oauth.go:44-86` `runLogin`, `:125` `interactiveCallbacks`;
  `cmd/pi-ai/main.go:43-50` the `login` dispatch.
- `ai/auth.go` — `AuthType`, `AuthInteraction` (from issue 01).
- Upstream: `src/models.ts` at `936aff00` — `login`, `logout`, the `getAuth`
  overloads.

## Dependencies

- **Blocked by**: [Issue 01](/epic-6-auth-core-and-env-api-key-bindings/issues/01-auth-contract-surface.md)
  (`AuthType`, `AuthInteraction`),
  [issue 03](/epic-6-auth-core-and-env-api-key-bindings/issues/03-provider-scoped-apikey-resolution.md)
  (`ResolveProviderAuth`'s signature),
  [issue 05](/epic-6-auth-core-and-env-api-key-bindings/issues/05-models-availability.md)
  — both this issue and issue 05 rewrite the `Models` interface and
  `modelsImpl` in `ai/provider.go`; land 05 first and rebase this issue's
  additions onto it rather than running the two concurrently.
- **Blocks**: Nothing in this epic. [Epic 7](/epic-7-four-new-oauth-flows/EPIC_7.md)
  wires four more flows into `cmd/pi-ai login`; landing this first means it
  wires them into `Models.Login` rather than into code about to be replaced.

## PR size note

`M` — ~460 changed lines: three new `Models` methods (`Login`, `Logout`,
`GetAuthForProvider`), the static-header merge added to `GetAuth`, nine named
tests pinning four verbatim error strings, and the `cmd/pi-ai/oauth.go:72-85`
migration off hand-rolled persistence. Also top of the band. Split past ~500,
and the seam is the CLI migration — the library half stands on its own.
