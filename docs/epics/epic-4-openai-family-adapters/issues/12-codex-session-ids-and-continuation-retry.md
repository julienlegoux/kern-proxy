---
type: Issue
title: "openai-codex-responses: cache-derived session ids, UUIDv7 request ids, and the missing-continuation retry"
description: "Derive one clamped cache session id and use it everywhere, port utils/uuid.ts as the request-id source, retry once on previous_response_not_found, and record the connection-cache changes with no Go counterpart."
tags: [epic-4]
timestamp: 2026-08-09T05:17:46Z
epic: 4
issue: 12
slug: codex-session-ids-and-continuation-retry
size: M
status: open
gh_issue: 158
resource: https://github.com/kern-ia/kern-link/issues/158
depends_on: [11]
---

# openai-codex-responses: cache-derived session ids, UUIDv7 request ids, and the missing-continuation retry

## Summary

Codex's session handling is reworked upstream around one derived value:

```
cacheSessionId = cacheRetention === "none" ? undefined : options.sessionId
codexSessionId = clampOpenAIPromptCacheKey(cacheSessionId)
```

`codexSessionId` becomes the `prompt_cache_key` and the WebSocket request id;
`cacheSessionId` keys the SSE headers, the WebSocket fallback flag, the failure
recorder and the debug stats. Today the Go adapter uses the raw
`opts.SessionID` for several of those and clamps only at the
`prompt_cache_key`, so a caller asking for `cacheRetention: "none"` still leaks a
session id into transport-level bookkeeping.

Two things ride along:

- **`uuidv7`.** Upstream deleted `createCodexRequestId()` in favor of a new
  `src/utils/uuid.ts` exporting `uuidv7()`, re-exported from `src/index.ts` — so
  it is public API, not an internal helper. It is the WebSocket request id when
  no session id exists. **No epic currently owns `utils/uuid.ts`**; it lands
  here because Codex is its only in-repo consumer at `936aff00`. If
  [Epic 8](/epic-8-pi-messages-and-radius/EPIC_8.md) needs it too, it will find
  it already ported.
- **`previous_response_not_found`.** When the server rejects a continuation
  because the previous response is gone, the WebSocket path retries the request
  **once** (a dedicated one-shot flag, separate from the existing
  connection-limit retry) before falling back.

## Scope

- New `ai/uuid.go` in package `ai` with a
  `// Ports: packages/ai/src/utils/uuid.ts` header and
  `func UUIDv7() string` — time-ordered, monotonic within a millisecond via the
  same sequence-counter scheme upstream uses. Upstream's module-level
  `lastTimestamp`/`sequence` state is shared mutable state; in Go it must be
  guarded (a `sync.Mutex` or atomics) because adapters generate ids from
  concurrent streams and CI runs `-race`. Use `crypto/rand`; there is no need to
  port upstream's `Math.random` fallback (Go's `crypto/rand.Read` does not fail
  silently) — record that as a one-line deviation in `docs/PORTING.md`.
- `ai/apis/codex/codex.go`:
  - `run` (`:75`) — derive `cacheSessionID` and `codexSessionID` once, and use
    them for the request body's cache key, the SSE headers (`buildHeaders`,
    `:360`), the WebSocket request id (`codexSessionID` else `ai.UUIDv7()`), the
    fallback check (`codexWebSocketFallbackActive`, `websocket.go:119`), the
    failure recorder (`codexRecordWebSocketFailure`, `:164`) and the SSE-fallback
    recorder (`:151`).
  - `attemptCodexWebSocketOrFallback` (`:196`) — the one-shot
    `previous_response_not_found` retry, ordered **before** the existing
    connection-limit retry, and skipped when the context is already cancelled.
  - A predicate mirroring upstream's `isPreviousResponseNotFoundError`, matching
    the `previous_response_not_found` code on the existing `*codexAPIError`
    (`websocket.go:65`) — the same shape as `isWebSocketConnectionLimitReached`
    (`:89`).
- `ai/apis/codex/params.go:66` — `prompt_cache_key` takes the derived value
  rather than clamping `options.sessionId` inline.
- Port the session/transport cases of `test/openai-codex-stream.test.ts` and
  `test/uuid.test.ts` (+50).
- `docs/PORTING.md` — add the `src/utils/uuid.ts` → `ai/uuid.go` row, and extend
  the Codex row's parenthetical with the two upstream changes that have **no Go
  counterpart** (below), so the next sync does not read their absence as an
  oversight.

## Out of scope, recorded rather than ported

- **The account-keyed connection cache.** Upstream changed
  `websocketSessionCache` from `Map<sessionId, entry>` to
  `Map<sessionId, Map<accountId, entry>>` and reworked expiry accordingly.
  kern-link never ported the connection cache at all — `websocket.go:15-25`
  documents the scope explicitly and each request opens its own connection — so
  there is nothing to key. Record it; write no code.
- **`custom_tool_call_output` filtering in cached continuation bodies.**
  Upstream's `processWebSocketStream` now filters that item type alongside
  `function_call_output` when building a delta-continuation body. That body only
  exists on the cached path, which the port does not have. Same treatment:
  record, do not port.
- `validateRetryDelayMs` — see
  [issue 11](/epic-4-openai-family-adapters/issues/11-codex-request-body-and-stop-reasons.md);
  it is [Epic 9](/epic-9-classifier-audit-and-release/EPIC_9.md)'s call.

## Acceptance criteria / Definition of done

- [ ] `TestUUIDv7IsTimeOrderedAndUnique` — 1000 sequential ids are strictly
      increasing as strings and all distinct; version and variant nibbles are
      `7` and `0b10`.
- [ ] `TestUUIDv7IsRaceFree` — concurrent generation from several goroutines
      produces no duplicates (this test is the reason the state is guarded; CI
      runs `-race`).
- [ ] `TestCodexCacheRetentionNoneDropsSessionID` — with
      `CacheRetention: "none"` and a session id set, the request carries no
      `prompt_cache_key`, and the WebSocket request id is a generated UUIDv7
      rather than the session id.
- [ ] `TestCodexPromptCacheKeyIsClamped` — a session id longer than the 64-char
      limit is clamped identically wherever it is used (assert the body's
      `prompt_cache_key` and the transport request id agree).
- [ ] `TestCodexRetriesOnPreviousResponseNotFound` — a WebSocket attempt failing
      with `previous_response_not_found` is retried exactly once and then
      succeeds; a second identical failure is **not** retried again and falls
      through to the existing fallback path.
- [ ] `TestCodexPreviousResponseNotFoundNotRetriedWhenCancelled` — a cancelled
      context short-circuits the retry.
- [ ] `docs/PORTING.md` carries the `ai/uuid.go` mapping row, the `crypto/rand`
      deviation, and the two no-Go-counterpart notes.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(apis): derive codex session ids from cache retention and add uuidv7`.

## Relevant files / areas

- `ai/apis/codex/codex.go:75` `run`, `:196` `attemptCodexWebSocketOrFallback`,
  `:360` `buildHeaders`.
- `ai/apis/codex/websocket.go` (563 lines) — `:15-25` the connection-cache scope
  note, `:65` `codexAPIError`, `:89` `isWebSocketConnectionLimitReached`, `:119`
  `codexWebSocketFallbackActive`, `:151`/`:164` the recorders.
- `ai/apis/codex/params.go:66` `buildRequestBody`, `:192`
  `clampOpenAIPromptCacheKey`.
- `ai/apis/codex/websocket_test.go` (393), `stream_test.go` (675).
- New `ai/uuid.go`; `docs/PORTING.md`.
- Upstream: `src/api/openai-codex-responses.ts`, `src/utils/uuid.ts`,
  `test/uuid.test.ts` at `936aff00`.

## Dependencies

- **Blocked by**: [Issue 11](/epic-4-openai-family-adapters/issues/11-codex-request-body-and-stop-reasons.md)
  (same `run` and `buildRequestBody`).
- **Blocks**: None.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
