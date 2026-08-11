---
type: Issue
title: "Add ModelsRequestTransforms, case-insensitive header merging, and drop the AuthModel export"
description: "Let Models-level callers rewrite fully assembled headers before dispatch, fix header override to be case-insensitive, and remove the AuthModel export upstream deleted."
tags: [epic-2]
timestamp: 2026-08-11T20:00:00Z
epic: 2
issue: 09
slug: models-request-transforms
size: M
status: open
gh_issue: 137
resource: https://github.com/kern-ia/kern-link/issues/137
depends_on: ["05", "08"]
---

# Add ModelsRequestTransforms, case-insensitive header merging, and drop the AuthModel export

## Summary

Upstream added a `Models`-only option mixin that runs after auth resolution and
before provider dispatch:

```ts
export interface ModelsRequestTransforms {
	transformHeaders?: (headers: ProviderHeaders) => ProviderHeaders | Promise<ProviderHeaders>;
}
export type ModelsApiStreamOptions<TApi extends Api> = ApiStreamOptions<TApi> & ModelsRequestTransforms;
export type ModelsSimpleStreamOptions = SimpleStreamOptions & ModelsRequestTransforms;
```

It is deliberately not a `StreamOptions` field: the transform is stripped before
the options reach the adapter (`const { transformHeaders: _t, ...providerOptions } = options`),
so an adapter can never see it. `applyAuth` changed in three ways at the same
time, and they are one PR because they are one function:

1. `transformHeaders` runs last, on the fully merged header map;
2. header merging became **case-insensitive** — a caller's `Authorization`
   now replaces a provider default `authorization` instead of both surviving;
3. an unresolvable provider now fails with
   `Provider is not configured: ${model.provider}` instead of silently
   dispatching with no auth.

Separately, `models.ts` stopped re-exporting `AuthModel`
(`export { type AuthModel, ModelsError, … }` → `export { ModelsError, … }`).

## Scope

- `ai/options.go` (or `ai/provider.go`, wherever it reads better) — add
  `ModelsRequestTransforms` with `TransformHeaders func(ctx context.Context, headers ProviderHeaders) (ProviderHeaders, error)`.
  Go's fixed `StreamFunc` signature has no room for an options union, so the
  Go-shaped equivalent of upstream's `ModelsApiStreamOptions` /
  `ModelsSimpleStreamOptions` intersections is an implementer call: a separate
  `ModelsStreamOptions` wrapper, or a field on `StreamOptions` that `applyAuth`
  strips before dispatch. **Whichever you choose, an adapter must not be able to
  observe it** — that is the contract, and the choice goes in `docs/PORTING.md`.
- `ai/provider.go:239-297` `applyAuth`:
  - extract a `mergeHeaders(base, override ProviderHeaders) ProviderHeaders`
    helper that removes any existing key matching case-insensitively before
    setting the override's key (upstream's exact semantics: the *override's*
    spelling survives);
  - run `TransformHeaders` after that merge, on a non-nil map;
  - fail with `NewModelsError(ModelsErrorAuth, fmt.Sprintf("Provider is not configured: %s", model.Provider), nil)`
    when resolution yields nothing. **The string is verbatim from upstream** —
    error text is load-bearing here (ST1005 is disabled for exactly this reason,
    [CONVENTIONS.md](../../../planning/CONVENTIONS.md)).
- Wire the transform through `Stream`, `Complete`, `StreamSimple`,
  `CompleteSimple` (`ai/provider.go:299-344`).
- **Remove the `AuthModel` export.** Confirm what `AuthModel` is in the Go port
  (`ai/auth.go:19` declares `ModelAuth`; establish whether that is the same
  symbol under the port's naming before deleting anything) and remove it only if
  it is genuinely upstream's deleted `AuthModel`. If it turns out to be
  load-bearing in Go where it was not in TS, keep it and record the divergence
  in `docs/PORTING.md` rather than breaking a consumer for a cosmetic parity
  win.

## Out of scope

- `Models.checkAuth` / `getAvailable` / `login` / `logout` and the
  `getAuth(providerId, …)` overload — [Epic 6](/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md),
  for the reason given in
  [issue 08](/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md).
- The `resolveProviderAuth` signature change (upstream dropped its `model`
  parameter and now merges static model headers in `getAuth`) — that is
  `src/auth/resolve.ts`, epic 6's file. Keep `ai/resolve.go`'s current signature
  and merge headers where kern-link already does.
- Adapter-side header handling — epics 4 and 5.

## Acceptance criteria / Definition of done

- [ ] `TestApplyAuthTransformHeadersRunsLast` — a transform sees the merged
      provider + auth + caller headers, and its return value is what reaches the
      adapter.
- [ ] `TestApplyAuthStripsTransformBeforeDispatch` — the adapter's received
      options carry no transform (assert through a `faux`-backed provider or a
      stub `ProviderStreams`).
- [ ] `TestMergeHeadersOverridesCaseInsensitively` — base `{"authorization":"a"}`
      plus override `{"Authorization":"b"}` yields exactly one entry, keyed
      `Authorization`, valued `b`.
- [ ] `TestApplyAuthFailsWhenProviderUnconfigured` — the resulting in-band error
      message is exactly `Provider is not configured: <id>`, asserted as a
      string (`got`/`want`), because retry/overflow classification matches error
      text.
- [ ] `TestTransformHeadersErrorSurfacesInBand` — a transform returning an error
      terminates the stream with an error event, not a returned Go error
      (`ai.LazyStream` guarantees this; assert it).
- [ ] `grep -rn "AuthModel" --include=*.go .` returns nothing, **or**
      `docs/PORTING.md` records why the Go symbol stays.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2).
- [ ] `gofmt -l .` prints nothing; `ai/provider.go` keeps its `// Ports:` header.
- [ ] Conventional Commit, e.g.
      `feat(ai): let Models callers transform request headers before dispatch`.

## Relevant files / areas

- `ai/provider.go:237-297` — `applyAuth`, including today's header and env merge
  (`:275-294`); `:299-344` — the `Stream`/`Complete`/`StreamSimple`/
  `CompleteSimple` quartet.
- `ai/options.go:36` — `ProviderHeaders` (`map[string]*string`, nil suppresses a
  default header — the transform must preserve that convention).
- `ai/resolve.go:13-46` — `ModelsErrorCode`, `ModelsError`, `NewModelsError`.
- `ai/auth.go:19` — `ModelAuth`, the candidate for the `AuthModel` removal.
- `ai/headers.go` — existing header helpers; reuse rather than duplicate.
- Upstream: `src/models.ts` at `936aff00` (`mergeHeaders`, `applyAuth`, the
  `Models*Options` aliases).

## Dependencies

- **Blocked by**: [Issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md),
  [Issue 08](/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md)
  (both rewrite regions of `ai/provider.go` this PR edits).
- **Blocks**: [Issue 10](/epic-2-core-types-and-models-contracts/issues/10-deferred-response-dispatch.md)
  — issue 10's `Models`-level deferred option variants carry
  `ModelsRequestTransforms` in whatever shape this issue settles on.

## PR size note

`M` — ~300 changed lines: `applyAuth` (`ai/provider.go:239-297`) changes in
three ways at once, the case-insensitive `mergeHeaders` helper is extracted out
of it, the transform threads through the `Stream`/`Complete`/`StreamSimple`/
`CompleteSimple` quartet, and five named tests plus the `AuthModel` removal
decision follow. Split past ~500; the seam is the `AuthModel` removal, the only
part of this PR not welded to `applyAuth`.
