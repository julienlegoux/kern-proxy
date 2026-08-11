---
type: Issue
title: "Add CredentialStore.List and CredentialInfo across the in-memory and file stores"
description: "Port credential-store.ts's new list() enumeration — non-secret credential metadata — onto the CredentialStore interface and both implementations."
tags: [epic-6]
timestamp: 2026-08-11T13:05:00Z
epic: 6
issue: 02
slug: credential-store-list
size: S
status: open
gh_issue: 171
resource: https://github.com/kern-ia/kern-link/issues/171
depends_on: []
---

# Add CredentialStore.List and CredentialInfo across the in-memory and file stores

## Summary

`src/auth/credential-store.ts` gained one method in range, and it is the only
part of that file's +60/-19 that is not an `AbortSignal` concern Go's
`context.Context` already covers:

```ts
/** Non-secret credential metadata for account/status enumeration. */
export interface CredentialInfo { providerId: string; type: Credential["type"] }

/**
 * List stored credential metadata without resolving or exposing secrets.
 * Implementations must not execute configured API-key commands while listing.
 */
list(options?: AuthOperationOptions): Promise<readonly CredentialInfo[]>;
```

The doc comment is the contract: **no secrets, no side effects**. `List` exists
so a status UI can answer "which providers am I logged in to" without touching
`Models.GetAuth`, which resolves — and may refresh a token or run a command.

The rest of upstream's diff to that file is the `enqueue` chain rewrite
(`raceWithAbortSignal`, deleting the chain entry once its tail settles). Go's
`InMemoryCredentialStore` serializes with a `sync.Mutex` per provider id
(`ai/credentialstore.go:28-37`) rather than a promise chain, so there is nothing
to port there — the leak upstream's rewrite fixes (an unbounded `chains` map)
does apply to Go's `locks` map, but the two stores are keyed by a bounded set of
provider ids, so it is not a leak in practice. Note it, do not fix it.

This issue owns **every** `CredentialStore` edit in the epic so the interface
and both implementations change in one compiling PR.

## Scope

- `ai/auth.go` — `type CredentialInfo struct { ProviderID string; Type
  CredentialType }`, and a `List(ctx context.Context) ([]CredentialInfo,
  error)` method on the `CredentialStore` interface (`:146-159`), carrying
  upstream's contract verbatim in the doc comment: metadata only, no secrets,
  and no execution of configured API-key commands.
- `ai/credentialstore.go` — `InMemoryCredentialStore.List`. Snapshot under the
  same `s.mu` the other reads use.
- `ai/auth/filestore.go` — `FileCredentialStore.List`. Read via the existing
  `load()` + shared-lock path (`:59` `withLock`, `:76` `load`), decoding only
  each entry's `"type"` tag; **do not** call `decodeEntry` (`:111`) — a full
  decode materializes the secret, which is precisely what the contract forbids.
  An unknown or missing `type` tag on a stored entry must not fail the whole
  listing; skip that entry.
- **Ordering is part of the contract or it is not** — upstream returns map
  iteration order, which is insertion order in JS and deliberately randomized in
  Go. Sort by `ProviderID` in both implementations and say why in a comment,
  so the tests are not flaky and callers get something stable to render.

## Out of scope

- Any `Models`-level surface built on `List`. Upstream's `models.ts` does not
  call it at `936aff00`; it is a store-level capability for apps.
- The `enqueue`/`raceWithAbortSignal` rewrite — see above.
- Every other `ai/auth.go` type change —
  [issue 01](/epic-6-auth-core-and-env-api-key-bindings/issues/01-auth-contract-surface.md).
  This PR touches `ai/auth.go` only to add `CredentialInfo` and the interface
  method.

## Acceptance criteria / Definition of done

- [ ] `TestInMemoryListReturnsProviderIdsAndTypes` — a store holding an api-key
      credential for `"openai"` and an OAuth credential for `"anthropic"` lists
      exactly two entries, sorted `anthropic`, `openai`, with
      `Type` `oauth` and `api_key` respectively.
- [ ] `TestFileStoreListDoesNotDecodeSecrets` — write an `auth.json` whose
      entry carries a `key` field, list it, and assert the returned
      `CredentialInfo` has no field able to carry it (a compile-time property —
      assert instead that `List` succeeds against an entry whose *other* fields
      are malformed JSON-typed values, proving it never decoded past `type`).
- [ ] `TestFileStoreListSkipsUnknownCredentialTypes` — an entry with
      `"type":"totally-unknown"` is skipped and the remaining entries still
      list, with no error.
- [ ] `TestListOnEmptyStoreReturnsEmptyNotNilError` — both stores return a
      zero-length slice and a nil error, never an error, for an empty store (and
      for a `FileCredentialStore` whose file does not exist yet — the same case
      `Read` already handles at `ai/auth/filestore.go:123`).
- [ ] `auth.json`'s file mode is untouched by a `List` call: extend or mirror
      the existing mode assertion (`t.Errorf("auth.json mode = %o, want 600",
      perm)`) so listing cannot create the file.
- [ ] `ai/auth.go`, `ai/credentialstore.go` and `ai/auth/filestore.go` keep
      their `// Ports:` headers, still describing what each file ports after
      this change.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(ai)!: add CredentialStore.List for secret-free credential enumeration`.

## Relevant files / areas

- `ai/auth.go:138-159` — the `CredentialStore` interface and its error
  semantics doc comment.
- `ai/credentialstore.go:39-79` — `Read`/`Modify`/`Delete`, the locking pattern
  `List` must respect.
- `ai/auth/filestore.go:59` `withLock`, `:76` `load`, `:111` `decodeEntry`,
  `:123` `Read`.
- Upstream: `packages/ai/src/auth/credential-store.ts` at `936aff00`;
  `src/auth/types.ts`'s `CredentialInfo` and the `list()` doc comment.

## Dependencies

- **Blocked by**: None. It is deliberately independent of
  [issue 01](/epic-6-auth-core-and-env-api-key-bindings/issues/01-auth-contract-surface.md)
  — different types, different files — so the two can land in either order.
- **Blocks**: Nothing.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
