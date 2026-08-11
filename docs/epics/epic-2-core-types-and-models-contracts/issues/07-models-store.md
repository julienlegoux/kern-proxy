---
type: Issue
title: "Port models-store.ts as ai.ModelsStore, and map both new core modules in PORTING.md"
description: "Add the persistent per-provider model-catalog store interface with its in-memory implementation, and record the Go mapping chosen for models-store.ts and model-catalog.ts."
tags: [epic-2]
timestamp: 2026-08-11T13:10:00Z
epic: 2
issue: 07
slug: models-store
size: S
status: open
gh_issue: 135
resource: https://github.com/kern-ia/kern-link/issues/135
depends_on: []
---

# Port models-store.ts as ai.ModelsStore, and map both new core modules in PORTING.md

## Summary

Upstream added two core modules with no Go counterpart. Both are small, and both
need a `docs/PORTING.md` row whatever the outcome — that is acceptance criterion
4 of [Epic 2](/epic-2-core-types-and-models-contracts/EPIC_2.md), from
[decision 04](../../../planning/scope/04-core-types-migration.md).

**`src/models-store.ts`** is a real contract: persistent model catalogs keyed by
provider ID, with an in-memory default. It is what the new refresh pipeline
(`RefreshModelsContext.stored`, `publish({persist})`) reads and writes, so it
must land before
[issue 08](/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md).

```ts
export interface ModelsStoreEntry {
	models: readonly Model<Api>[];
	lastModified?: number;   // from the remote catalog's Last-Modified header
	checkedAt?: number;      // last completed remote check
	etag?: string;           // stored verbatim, quotes included, echoed as If-None-Match
}
export interface ModelsStore {
	read(providerId, options?): Promise<ModelsStoreEntry | undefined>;
	write(providerId, entry, options?): Promise<void>;
	delete(providerId, options?): Promise<void>;
}
export class InMemoryModelsStore implements ModelsStore { … }   // structuredClone on read and write
```

**`src/model-catalog.ts`** is the opposite: 27 lines of TypeScript type-level
machinery (`ModelGroups`, mapped `ModelCatalog<…>`, and `flattenModelCatalog`,
whose runtime body is `Object.assign({}, ...Object.values(groups))`). It exists
to give per-provider `*.models.ts` files literal-typed model IDs. Go has no
equivalent need — the catalog is decoded JSON validated at load
(`ai/catalog`) — so the expected outcome is a **not-ported** row with that
reasoning. Verify that reading before writing it down.

## Scope

- New `ai/modelsstore.go` (file naming follows the repo's lowercase,
  no-separator convention — `credentialstore.go`, `sessionresources.go`) with a
  `// Ports: packages/ai/src/models-store.ts` header:
  - `ModelsStoreEntry` — `Models []*Model`, `LastModified int64`,
    `CheckedAt int64`, `ETag string`, with upstream's JSON keys and upstream's
    comments on what each means (the `etag` verbatim-with-quotes rule is
    load-bearing for `If-None-Match`).
  - `ModelsStore` interface — `Read`/`Write`/`Delete`, each taking
    `context.Context` as its first parameter (the established
    `AbortSignal` → `context.Context` deviation; `CredentialStore` at
    `ai/auth.go:146` is the in-repo precedent to match).
  - `InMemoryModelsStore` + `NewInMemoryModelsStore()`, mirroring
    `InMemoryCredentialStore` (`ai/credentialstore.go`) including its
    per-provider locking style.
  - Upstream `structuredClone`s on both read and write so callers cannot mutate
    stored state through a returned slice. Do the equivalent deep copy in Go and
    test it — this is the one subtle behavior in the file.
- `ai/provider.go:87-91` `CreateModelsOptions` gains
  `ModelsStore ModelsStore`, defaulting to `NewInMemoryModelsStore()` in
  `CreateModels` (`ai/provider.go:103`), matching upstream's constructor.
- `docs/PORTING.md` — add a mapping row for **each** of
  `src/models-store.ts` and `src/model-catalog.ts`.

## Out of scope

- **Using the store.** Nothing reads or writes it until
  [issue 08](/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md)
  ports the refresh pipeline. This PR ships the contract plus the wiring point,
  and its PR body says so.
- A file-backed `ModelsStore` implementation. Upstream ships only the in-memory
  one at this revision; adding a persistent one is new capability, not parity.
- Any change to `ai/catalog` — the embedded build-time catalog is a different
  thing from this runtime store, and epic 3 owns it.
- **Deep-copying fields this PR doesn't add.**
  `TestInMemoryModelsStoreReadReturnsCopy` proves the copy is deep for
  everything `ModelsStoreEntry` and `Model` carry as of this PR.
  [Issue 03](/epic-2-core-types-and-models-contracts/issues/03-tiered-model-cost.md)
  and [issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md)
  each add a field to `Model` afterward (`Cost.Tiers`, `SamplingParams`) that
  the copy must also clone; extending the copy and this test when they land is
  their criterion, not this one's.

## Acceptance criteria / Definition of done

- [ ] `ai.ModelsStore`, `ai.ModelsStoreEntry`, `ai.InMemoryModelsStore` exist
      with upstream's field semantics and JSON keys.
- [ ] `TestInMemoryModelsStoreReadReturnsCopy` — mutating the slice or a model
      returned by `Read` does not change what a subsequent `Read` returns.
- [ ] `TestInMemoryModelsStoreWriteCopiesEntry` — mutating the caller's entry
      after `Write` does not change stored state.
- [ ] `TestInMemoryModelsStoreDeleteRemovesEntry` — `Read` after `Delete`
      returns no entry and no error.
- [ ] `TestCreateModelsDefaultsModelsStore` — `CreateModels(nil)` yields a
      collection with a usable in-memory store, and an explicitly supplied store
      is used instead.
- [ ] Each store method honors a cancelled `context.Context` (upstream's
      `signal?.throwIfAborted()`), covered by a test.
- [ ] `docs/PORTING.md` contains a row for `src/models-store.ts` and a row for
      `src/model-catalog.ts`, the latter stating the not-ported reasoning if
      that is the conclusion.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2).
- [ ] `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g. `feat(ai): add the ModelsStore contract`.

## Relevant files / areas

- `ai/credentialstore.go` (72 lines) — the shape to mirror: in-memory map,
  per-provider mutex, `context.Context` first parameter.
- `ai/auth.go:146` — `CredentialStore`, the sibling interface.
- `ai/provider.go:87-116` — `CreateModelsOptions`, `CreateModels`, `modelsImpl`.
- `docs/PORTING.md` — "File mapping" table and "Intentional deviations".
- Upstream, both small enough to read whole:
  ```bash
  gh api "repos/earendil-works/pi/contents/packages/ai/src/models-store.ts?ref=936aff00918de1187f085f123c2812d8f2d67745" -H "Accept: application/vnd.github.raw"
  gh api "repos/earendil-works/pi/contents/packages/ai/src/model-catalog.ts?ref=936aff00918de1187f085f123c2812d8f2d67745" -H "Accept: application/vnd.github.raw"
  ```

## Dependencies

- **Blocked by**: None.
- **Blocks**: [Issue 08](/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md)
  — the refresh pipeline reads and publishes through this store.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR. Expected here: ~150 including tests.
