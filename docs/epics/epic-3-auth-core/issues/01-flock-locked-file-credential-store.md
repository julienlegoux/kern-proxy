---
type: Issue
title: "Add flock-locked file credential store"
description: "Persistent CredentialStore backed by ~/.pi/agent/auth.json with 0600/0700 modes and a cross-process file lock."
tags: [epic-3]
timestamp: 2026-07-07T22:00:00Z
epic: 3
issue: 01
slug: flock-locked-file-credential-store
size: M
status: done
gh_issue: 16
gh_pr: 46
resource: https://github.com/julienlegoux/kern-proxy/issues/16
depends_on: []
---

# Add flock-locked file credential store

## Summary

Implement the persistent credential store that completes [Epic 3: Auth core](/epic-3-auth-core/EPIC_3.md)'s storage layer: an `ai.CredentialStore` backed by `~/.pi/agent/auth.json`, guarded by an OS-level file lock so OAuth token rotations are safe across concurrent processes. The in-memory store, credential types, and resolve precedence already shipped in commit `61ce7d6`; this is the on-disk counterpart that login flows (Epic 12) and real provider adapters will persist through.

## Scope

- New `ai/auth` package (per the PORTING.md mapping for `src/auth/helpers.ts` / the persistent auth.json store — the credential *types* stay in the `ai` root, see the epic's layout note).
- A file-backed store implementing `ai.CredentialStore` (`Read`, `Modify`, `Delete`) at `~/.pi/agent/auth.json`:
  - JSON object keyed by provider ID; values decoded via `ai.UnmarshalCredential`.
  - File mode `0600`, directory mode `0700` (created on first write).
  - `Modify` is an atomic read-modify-write under an exclusive file lock — flock on Unix, `LockFileEx` on Windows (e.g. via `github.com/gofrs/flock` or `golang.org/x/sys`), giving the cross-process mutual exclusion the `CredentialStore.Modify` contract promises.
  - Missing file reads as an empty store; `Read` returns `(nil, nil)` for missing entries, matching the interface's error semantics.
- TDD gate: port the upstream `oauth-auth` persistent-store tests first, plus a cross-goroutine (and ideally cross-process) locking test.
- `// Ports:` header on each new file referencing the upstream sources; flip the PORTING.md row for the persistent auth.json store from `phase 3` to `ported`.

## Out of scope

- Interactive OAuth login flows (PKCE, device code) — [Epic 12: OAuth flows](/epic-12-oauth-flows/EPIC_12.md).
- Any changes to the resolve precedence, env-key map, or in-memory store — already delivered.
- CI / `-race` execution — [Issue 02](./02-run-race-tests-in-ci.md).

## Acceptance criteria / Definition of done

- Ports of the upstream `oauth-auth` store tests pass with the file store substituted.
- Concurrent `Modify` calls (multiple goroutines, and a child-process test where feasible) never lose or double-apply a rotation.
- On Unix, a fresh store creates `auth.json` with mode `0600` inside a `0700` directory (skip the mode assertions on Windows).
- `go test ./...` green; new files carry `// Ports:` headers; PORTING.md updated.

## Relevant files / areas

- New: `ai/auth/` (store + tests). No code exists there yet — the path follows the PORTING.md mapping, not existing files.
- Read-only context: `ai/auth.go` (`CredentialStore` contract, `UnmarshalCredential`), `ai/credentialstore.go` (in-memory reference implementation), `ai/resolve.go` (how `Modify` is used for locked refresh).

## Dependencies

None within this epic. Blocks [Epic 12: OAuth flows](/epic-12-oauth-flows/EPIC_12.md) (login flows persist through this store).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.
