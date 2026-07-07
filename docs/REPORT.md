# PLAN.md and epic review

Date: 2026-07-07

## Summary

`docs/PLAN.md` and the epic split are broadly correct as a phase plan for the
Go rebuild. The epic scope mostly preserves the plan's dependency order and
records the important layout deviations introduced by the first delivered
phases.

The main corrections needed are documentation/status hygiene, not a rethink of
the roadmap:

- `docs/PLAN.md` still reads partly like the repository is empty and still
  shows some paths that no longer match the current tree.
- Epic links in `docs/epics/index.md` and cross-epic links use root-relative
  paths that are wrong for this repository layout on GitHub.
- Epic 3's "Remaining" list is slightly stale: double-checked OAuth refresh is
  already implemented and covered by a concurrent test.
- The race-test acceptance gate cannot currently be verified on this machine
  because the Windows toolchain lacks `gcc` for cgo.

## Verification performed

- Read `docs/PLAN.md`.
- Read `docs/epics/index.md` and all 14 `EPIC_*.md` files.
- Compared the docs against the current source tree under `ai/`, `upstream/`,
  and `docs/`.
- Checked `docs/PORTING.md`, `upstream/UPSTREAM.lock`, and `upstream/sync.sh`.
- Ran `go test ./...`: passed.
- Ran `go test ./... -race`: blocked. First run failed because cgo was disabled;
  retrying with `CGO_ENABLED=1` failed because `gcc` is not installed in `%PATH%`.

## Current implementation state observed

Implemented packages/files align with Epics 1 and 2, plus most of Epic 3:

- Core domain and utility files exist in `ai/`: types, events, options, models,
  cost, stream, retry, overflow, estimate, hash, headers, diagnostics,
  unicode sanitizing, session resources, auth types/resolution, and provider
  registry.
- `ai/internal/sse` and `ai/internal/partialjson` exist with tests.
- `ai/apis/simpleopts.go` exists with tests.
- `ai/providers/faux` exists with tests.
- `upstream/UPSTREAM.lock` pins upstream `earendil-works/pi` at
  `244f1deaf1ae0fc1a242d9df5cddf457cf3d36a7`, version `0.80.3`.
- `upstream/sync.sh` exists as scaffolding and buckets upstream diffs by area.

Not yet present, matching open epics:

- `ai/apis/anthropic`, `openaicompletions`, `openairesponses`, `google`,
  `mistral`, or `bedrock`.
- `ai/internal/jsonschema`.
- `ai/catalog`.
- `ai/images`.
- `cmd/pi-ai`.
- `tools/export-catalog`.
- Persistent file-backed credential store under an `ai/auth` package or
  equivalent location.
- OAuth flow packages.

## Findings

### 1. PLAN.md is slightly stale as a repository status document

`docs/PLAN.md` says `julienlegoux/kern-proxy` is "empty (README only)". That is
no longer true: Epics 1 and 2 are implemented and Epic 3 is partially
implemented. This is fine if the file is meant to preserve the original plan,
but it is misleading if readers use it as current status.

Recommendation: add a short note near the top that `PLAN.md` is the original
full roadmap and that current implementation status lives in `docs/epics/` and
this report or future status reports.

### 2. PLAN.md path references do not fully match the current layout

The architecture tree in `docs/PLAN.md` shows `PORTING.md` at the repository
root, but the actual file is `docs/PORTING.md`. It also shows planned packages
such as `ai/auth/`, `ai/internal/httpx`, `ai/catalog`, `cmd/pi-ai`, and
`tools/export-catalog` as if they exist. Several are intentionally future work,
but `ai/internal/httpx` is now an explicit layout deviation: retry/overflow/
unicode scrub utilities live in the `ai` root.

Recommendation: mark the tree as target layout and update `PORTING.md` to
`docs/PORTING.md`. For the known `internal/httpx` deviation, either update the
tree or add a footnote pointing to Epic 1's layout note.

### 3. Epic index links are root-relative and likely broken

`docs/epics/index.md` links epics as `/epic-1-foundation/EPIC_1.md`, but the
actual path is `docs/epics/epic-1-foundation/EPIC_1.md`. On GitHub, a leading
slash is repository-root relative, so these links will not resolve to the files.

The same pattern appears in cross-epic links inside individual epic files.

Recommendation: change links in `docs/epics/index.md` to
`epic-1-foundation/EPIC_1.md` style. In individual epic files, use relative
links such as `../epic-1-foundation/EPIC_1.md`.

### 4. Epic 3 remaining work overstates the OAuth refresh gap

Epic 3 lists "Verify/complete double-checked locking on OAuth refresh under
`CredentialStore.Modify`" as remaining. The implementation in `ai/resolve.go`
already performs the optimistic expiry check, locks via `CredentialStore.Modify`,
re-checks expiry under the lock, refreshes once, and persists the rotated
credential. `ai/resolve_test.go` includes `TestResolveExpiredOAuthRefreshesOnceUnderLock`
with concurrent callers.

Recommendation: remove that item from "Remaining" or rewrite it as "Verify
under `go test -race` once the local/toolchain cgo requirement is available."
The true remaining Epic 3 work appears to be the flock-backed file store and
race-mode verification.

### 5. Race-mode acceptance criteria are not currently verifiable here

Epics 1, 2, and 3 all use `go test ./... -race` or race-test wording as an
acceptance gate. On this machine, `go test ./... -race` cannot run:

- Without cgo enabled, Go reports `-race requires cgo`.
- With `CGO_ENABLED=1`, the build fails because `gcc` is not installed.

Recommendation: keep the race gate, but record the local prerequisite in docs:
Windows race verification requires cgo and a C compiler in `%PATH%`. Until then,
only the non-race suite has been verified locally.

### 6. Epic 14 accurately notes sync tooling is scaffolded, not finished

`upstream/UPSTREAM.lock` and `upstream/sync.sh` exist. The script uses
`PORTING.md` in comments, while the actual file is `docs/PORTING.md`; because
this is only human-facing text, it is not a functional bug. The epic correctly
keeps CI automation, completed `PORTING.md`, and end-to-end CLI verification in
future scope.

Recommendation: when Epic 14 is started, update human-facing references from
`PORTING.md` to `docs/PORTING.md` unless the file is intentionally moved back to
the repository root.

## Epic-by-epic verdict

| Epic | Verdict | Notes |
|---|---|---|
| 1 Foundation | Mostly correct | Implemented files and tests exist. Race gate not locally verified. Plan/tree should reflect `internal/httpx` deviation. |
| 2 Faux provider & registry | Correct | Source and tests align with the epic. Non-race suite passes. |
| 3 Auth core | Partly stale | File store remains missing. Double-checked OAuth refresh is already implemented and tested concurrently. Race-mode verification blocked by missing C compiler. |
| 4 Transform & validation | Correct as future work | No `ai/apis/transform.go` or `ai/internal/jsonschema` yet, which matches open status. |
| 5 Anthropic adapter | Correct as future work | No adapter directory yet, matching open status. |
| 6 OpenAI completions | Correct as future work | No adapter directory yet, matching open status. |
| 7 Responses family | Correct as future work | No adapter directory yet, matching open status. |
| 8 Google & Vertex | Correct as future work | No adapter directory yet, matching open status. |
| 9 Mistral | Correct as future work | No adapter directory yet, matching open status. |
| 10 Bedrock | Correct as future work | No adapter directory yet, matching open status. |
| 11 Catalog & providers | Correct as future work | No catalog/export tool/all-provider bindings yet, matching open status. |
| 12 OAuth flows | Correct as future work | No OAuth flow package yet, matching open status. |
| 13 Images | Correct as future work | No image stack yet, matching open status. |
| 14 CLI & sync tooling | Correct with minor path note | Sync scaffolding exists; CLI, CI, and completed porting map remain future work. |

## Recommended doc edits

1. Add a "Current status" note to `docs/PLAN.md` so readers know it is the
   roadmap, not a live implementation inventory.
2. Fix `PORTING.md` references in `docs/PLAN.md`, `docs/epics/epic-14-*`, and
   `upstream/sync.sh` comments to `docs/PORTING.md`, or move the file back to
   the root and update docs consistently.
3. Convert epic links from root-relative to repository-relative Markdown links.
4. Update Epic 3's "Remaining" list to remove the already implemented
   double-checked refresh item.
5. Add a small verification note that `go test ./...` passes as of this review,
   while race-mode verification needs cgo plus a C compiler on Windows.
