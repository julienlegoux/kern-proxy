---
type: Issue
title: "Honor MaxRetries and MaxRetryDelay in the OpenRouter images adapter"
description: "Port upstream's retryProviderRequest wrapping of the images request by making the shared httpretry loop reachable from ai/images, with offline httptest coverage."
tags: [epic-3]
timestamp: 2026-08-09T04:52:00Z
epic: 3
issue: 06
slug: images-adapter-retry
size: M
status: open
gh_issue: 146
resource: https://github.com/kern-ia/kern-link/issues/146
depends_on: []
---

# Honor MaxRetries and MaxRetryDelay in the OpenRouter images adapter

## Summary

Upstream's `api/openrouter-images.ts` at `936aff00` stops delegating retries to
the OpenAI SDK (`maxRetries: 0`) and wraps the request in
`retryProviderRequest(...)` with `{maxRetries, maxRetryDelayMs, signal}`.

kern-link's images adapter never retried at all: `ai/images/openrouter.go:239`
builds a bare `&http.Client{}` and calls `client.Do(req)` once.
`Options.MaxRetries` and `Options.MaxRetryDelay` (`ai/images/types.go:98-102`)
are declared, documented, and **read nowhere** — grep for either name under
`ai/images` returns only their declarations. Every text adapter honors them
through `ai/apis/internal/httpretry`; the images adapter is the outlier.

Two things stand in the way of just calling the shared loop, both worth naming
up front because they shape the PR:

1. **Package visibility.** `ai/apis/internal/httpretry` is importable only from
   within the tree rooted at `ai/apis/`. `ai/images` is outside it, so the
   import is a compile error, not a style preference. Moving the package to
   `ai/internal/httpretry` makes it reachable from every package under `ai/`
   including `ai/images`, with no import cycle: it imports `ai`, and `ai` does
   not import it.
2. **Config coupling.** `httpretry.Config.Opts` is a `*ai.StreamOptions`
   (`httpretry.go:60-76`), used for `MaxRetries`, `MaxRetryDelay`, `Timeout`
   and `OnResponse`. `ai/images.Options` is a separate type with the same four
   concerns. Widen `Config` to carry those four explicitly and have the text
   adapters pass them from their `StreamOptions`, rather than teaching a
   shared helper about two option types.

**A stale premise, corrected.**
[Decision 14](../../../planning/scope/14-images-surface.md) and
[SPECS.md](../../../planning/SPECS.md) both state that `ai/images` has "exactly
one live smoke test", which is where the epic's acceptance criterion 5 comes
from. That is no longer true: `ai/images/openrouter_test.go` already carries
five offline `httptest` tests (`:18`, `:80`, `:108`, `:122`, `:142`) plus
`TestGenerateImagesOpenRouter_LiveSmoke` (`:171`). The criterion still stands
and this PR still satisfies it — offline coverage for every adapter change is
the house rule either way — but plan the work against the tests that exist, and
correct the claim in SPECS.md as part of this PR.

## Scope

- Move `ai/apis/internal/httpretry` → `ai/internal/httpretry`, updating the
  package doc comment (which currently says "every raw-`net/http` adapter under
  `ai/apis`") and all importers: `ai/apis/anthropic`, `ai/apis/azure`,
  `ai/apis/bedrock`, `ai/apis/codex`, and their retry tests.
- Widen `httpretry.Config` to take `MaxRetries *int`, `MaxRetryDelay
  *time.Duration`, `Timeout time.Duration` and an `OnResponse` callback
  directly, replacing the `Opts *ai.StreamOptions` field. Keep `Model` for the
  callback. Text adapters pass the same values they pass today; behavior for
  them must not change.
- Rewrite `generateImagesOpenRouter`'s transport (`ai/images/openrouter.go:204-291`)
  to build an `httpretry.Request` and call `httpretry.Do`, mapping
  `Options.MaxRetries` / `Options.MaxRetryDelay` / `Options.Timeout` onto the
  new `Config` and moving the `OnResponse` callback into it.
- Preserve every existing behavior of the adapter: the
  `"No API key for provider: %s"` text, `OnPayload` replacement, the
  `abortAwareErrorResult` context-cancelled → `StopReasonAborted` mapping, the
  non-2xx error text from `openRouterImagesHTTPStatusError`, and the data-URL
  parsing of returned images.
- Add offline `httptest` coverage for the new retry path (below).

## Out of scope

- **Whether upstream's `utils/provider-retry.ts` supersedes
  `ai/apis/internal/httpretry`.** That is
  [Epic 9](/epic-9-classifier-audit-and-release/EPIC_9.md)'s explicit question
  ([decision 07](../../../planning/scope/07-classifier-error-text-sync.md)).
  This PR moves and parameterizes the existing helper; it does not re-derive it
  from upstream.
- The **clamp-vs-throw** difference on a server-requested delay above the cap:
  kern-link's `capRetryDelay` (`httpretry.go:296-303`) clamps, upstream's
  `validateServerRetryDelayMs` throws. Pre-existing, applies to every adapter,
  and belongs to the same epic 9 reconciliation. Do not change it here.
- `fetch` injection into the images client — epic 2
  [issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md)
  owns `ProviderRequestOptions`, which is where upstream's new `fetch` option
  comes from.
- OpenRouter images **OAuth** — [Epic 7](/epic-7-four-new-oauth-flows/EPIC_7.md).
- The `getAuth(providerId | model, overrides)` overload and
  `AuthResolutionOverrides` — [Epic 6](/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md).
- Catalog regeneration —
  [issues 04](/epic-3-catalog-schema-and-export-tooling/issues/04-regenerate-model-catalog.md)
  and [05](/epic-3-catalog-schema-and-export-tooling/issues/05-regenerate-image-catalog.md).

## Acceptance criteria / Definition of done

- [ ] `TestGenerateImagesOpenRouter_RetriesTransientStatusUpToMaxRetries` — an
      `httptest` server answering 429, 500, then 200 returns a successful
      `AssistantImages` with `MaxRetries: 3`, and the handler saw exactly three
      requests.
- [ ] `TestGenerateImagesOpenRouter_StopsAtMaxRetriesAndReturnsErrorResult` —
      with `MaxRetries: 1` and a server that always answers 500, the result is
      `StopReasonError`, the message carries the body text, and the handler saw
      exactly two requests.
- [ ] `TestGenerateImagesOpenRouter_HonorsRetryAfterOn429` — a `Retry-After`
      header is used instead of the exponential backoff (shrink
      `httpretry.BaseDelay` in the test; it is a `var` for exactly this,
      `httpretry.go:36-40`).
- [ ] `TestGenerateImagesOpenRouter_CapsServerRequestedDelayAtMaxRetryDelay` —
      a large `Retry-After` with a small `Options.MaxRetryDelay` waits the cap,
      not the requested delay.
- [ ] `TestGenerateImagesOpenRouter_DoesNotRetryNonRetryableStatus` — a 400 is
      returned immediately, with one request seen.
- [ ] `TestGenerateImagesOpenRouter_AbortDuringRetryBackoffReturnsAborted` — a
      context cancelled between attempts yields `StopReasonAborted`, not
      `StopReasonError`.
- [ ] All five existing offline tests in `ai/images/openrouter_test.go` still
      pass unchanged, and `TestGenerateImagesOpenRouter_LiveSmoke` still skips
      without `OPENROUTER_API_KEY`.
- [ ] The four text adapters' retry tests (`ai/apis/anthropic/anthropic_retry_test.go`,
      `ai/apis/azure/azure_retry_test.go`, `ai/apis/bedrock/retry_test.go`,
      and the codex retry tests) pass unchanged apart from the import path — the
      move and the `Config` widening are behavior-preserving for them.
- [ ] No test sleeps on a real backoff: every retry test shrinks
      `httpretry.BaseDelay` and restores it with `t.Cleanup`.
- [ ] `docs/planning/SPECS.md` — the "Testing infrastructure" claim about
      `ai/images/openrouter_test.go` being a live-only smoke is corrected, and
      the `ai/apis/internal/httpretry` row in the package map is updated to the
      new path.
- [ ] `docs/PORTING.md` — the mapping row for the images adapter reflects the
      retry behavior; `ai/internal/httpretry/httpretry.go` still carries **no**
      `// Ports:` header (it is original code, and its absence is meaningful per
      [CONVENTIONS.md](../../../planning/CONVENTIONS.md)).
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2).
- [ ] `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(images): honor MaxRetries and MaxRetryDelay in the openrouter adapter`.

## Relevant files / areas

- `ai/images/openrouter.go:204-291` `generateImagesOpenRouter` — `:239` the bare
  `&http.Client{}`, `:243` the single `client.Do`, `:249-255` the `OnResponse`
  callback, `:257-259` the non-2xx path; `:298-310`
  `abortAwareErrorResult` / `openRouterImagesHTTPStatusError`.
- `ai/images/types.go:96-102` — `Timeout`, `MaxRetries`, `MaxRetryDelay`, all
  currently unread.
- `ai/images/openrouter_test.go:18-170` — the five existing offline tests to
  keep green; `:171-201` the live smoke.
- `ai/apis/internal/httpretry/httpretry.go` — `:1-13` package doc, `:36-45`
  `DefaultMaxRetries` / `BaseDelay`, `:49-76` `Request` / `Config`, `:89-168`
  `Do`, `:249-303` `IsRetryable` / `RetryAfterDelay` / `capRetryDelay`.
- Importers to update: `ai/apis/anthropic/anthropic.go:24`,
  `ai/apis/azure/azure.go:22`, `ai/apis/bedrock/client.go:27`,
  `ai/apis/codex/codex.go:34`, plus the retry tests beside each.
- `docs/planning/SPECS.md` (package map, "Testing infrastructure"),
  `docs/PORTING.md`.
- Upstream at `936aff00`: `packages/ai/src/api/openrouter-images.ts:56-82`
  (the `retryProviderRequest` wrapping), `packages/ai/src/utils/provider-retry.ts`.

## Dependencies

- **Blocked by**: None. Independent of issues 01–05 — it touches no catalog data
  and no export tooling. In practice it will rebase over whichever of them lands
  first only where both touch `ai/images`.
- **Blocks**: Nothing. Note for [Epic 9](/epic-9-classifier-audit-and-release/EPIC_9.md):
  the `provider-retry.ts` reconciliation it owns will be against
  `ai/internal/httpretry` after this PR, not `ai/apis/internal/httpretry`.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR. The package move is mechanical but touches ten files — if the `Config`
widening turns out to change text-adapter behavior anywhere, split that half out
rather than growing this PR.
