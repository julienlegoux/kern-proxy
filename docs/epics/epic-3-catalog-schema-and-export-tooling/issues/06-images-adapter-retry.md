---
type: Issue
title: "Honor MaxRetries and MaxRetryDelay in the OpenRouter images adapter"
description: "Port upstream's retryProviderRequest wrapping of the images request by reaching the shared httpretry loop from ai/images through a thin ai/apis shim, with offline httptest coverage."
tags: [epic-3]
timestamp: 2026-08-11T18:15:00Z
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
   import is a compile error, not a style preference. **The package does not
   move.** [Epic 0](/epic-0-plan-remediation/EPIC_0.md) decided that, and
   [EPIC_3.md](/epic-3-catalog-schema-and-export-tooling/EPIC_3.md)'s `## Notes`
   records it: relocating a shared internal package while epics 4 and 5 rewrite
   its four consumers buys nothing that waiting does not, and
   [CONVENTIONS.md](../../../planning/CONVENTIONS.md) (`:22-26`) names
   `ai/apis/internal/httpretry` as part of the decided package layout. Reach the
   loop instead through a **thin exported shim in the existing `ai/apis`
   package** (`package apis`, already at `ai/apis/simpleopts.go` /
   `transform.go`): `ai/apis` is rooted at `ai/apis`, so it may import the
   internal package, and `ai/images` may import `ai/apis`. No cycle — `ai/apis`
   imports `ai` and does not import `ai/images`, and `ai` does not import either.
2. **Config coupling.** `httpretry.Config.Opts` is a `*ai.StreamOptions`
   (`httpretry.go:60-76`), used for `MaxRetries`, `MaxRetryDelay`, `Timeout`
   and `OnResponse` — and `Config.Model` is an `*ai.Model`, where the images
   callback wants an `*images.Model`. The shim absorbs the mismatch: it takes
   the four knobs explicitly plus a model-free
   `OnResponse func(ctx, ai.ProviderResponse) error` (the images adapter closes
   over its own model), and builds the `httpretry.Config` behind them.
   `httpretry` itself needs no change; if the shim cannot be written without one,
   that change must be **additive**, and every text-adapter call site and retry
   test must stay byte-identical.

**A stale premise, corrected — and routed through drift, not through an edit.**
[Decision 14](../../../planning/scope/14-images-surface.md) states that
`ai/images` has "exactly one live smoke test" (`:11`, `:54-57`, `:70-72`), which
is where the epic's acceptance criterion 5 comes from. That is no longer true:
`ai/images/openrouter_test.go` carries **six offline tests** — four driving an
`httptest` server (`:18`, `:80`, `:122`, `:142`) and two needing no server
(`:108`, `:165`) — beside `TestGenerateImagesOpenRouter_LiveSmoke` (`:171`).
The criterion still stands and this PR still satisfies it — offline coverage for
every adapter change is the house rule either way — but plan the work against
the tests that exist, and record the stale decision text as a **drift record**
(below) instead of editing `docs/planning/` inside this PR.

`docs/planning/SPECS.md:301` is **not** part of that correction: it says only
that eight adapters plus `ai/images/openrouter_test.go` carry a gated live
smoke, which is true and stays as it is.

## Scope

- Add the shim to the existing `ai/apis` package — one new file, e.g.
  `ai/apis/retry.go` — exporting a `Do` that takes an `httpretry.Request`-shaped
  request plus `MaxRetries *int`, `MaxRetryDelay *time.Duration`,
  `Timeout time.Duration`, a `ParseError` hook and a model-free
  `OnResponse func(ctx context.Context, response ai.ProviderResponse) error`, and
  forwards to `httpretry.Do`. **`ai/apis/internal/httpretry` does not move and
  its importers are not touched**: `ai/apis/anthropic`, `ai/apis/azure`,
  `ai/apis/bedrock` and `ai/apis/codex` keep importing it directly, and their
  retry tests are untouched by this PR.
- Rewrite `generateImagesOpenRouter`'s transport (`ai/images/openrouter.go:204-291`)
  to call that shim instead of `client.Do`, mapping `Options.MaxRetries` /
  `Options.MaxRetryDelay` / `Options.Timeout` onto it and moving the `OnResponse`
  callback into it.
- Preserve every existing behavior of the adapter: the
  `"No API key for provider: %s"` text, `OnPayload` replacement, the
  `abortAwareErrorResult` context-cancelled → `StopReasonAborted` mapping, the
  non-2xx error text from `openRouterImagesHTTPStatusError`, and the data-URL
  parsing of returned images.
- Add offline `httptest` coverage for the new retry path (below).
- Write a **drift record** at
  `docs/epics/epic-3-catalog-schema-and-export-tooling/drift/06-images-live-smoke-claim.md`
  (format in the pipeline interfaces) for decision 14's "exactly one live smoke
  test" claim: what was decided, what `ai/images/openrouter_test.go` actually
  holds with the command that shows it, and a revisit trigger. `close-epic`
  promotes it to `docs/planning/DRIFT.md`, where the user dispositions it. Do
  **not** edit `docs/planning/` in this PR.

## Out of scope

- **Relocating `ai/apis/internal/httpretry`.** Decided against for this program
  by [Epic 0](/epic-0-plan-remediation/EPIC_0.md) and recorded in
  [EPIC_3.md](/epic-3-catalog-schema-and-export-tooling/EPIC_3.md)'s `## Notes`.
  This PR adds a shim beside the package; it does not move it, does not rename
  it, and changes no text-adapter import.
- **Any edit to `docs/planning/SPECS.md` or `docs/planning/CONVENTIONS.md`.**
  `CONVENTIONS.md:22-26` keeps naming `ai/apis/internal/httpretry` because that
  is still where the package lives, and `SPECS.md:73`'s package-map row and
  `:301`'s smoke-test sentence both stay true. Decision 14's stale claim goes in
  the drift record, not into a planning-doc diff.
- **Whether upstream's `utils/provider-retry.ts` supersedes
  `ai/apis/internal/httpretry`.** That is
  [Epic 9](/epic-9-classifier-audit-and-release/EPIC_9.md)'s explicit question
  ([decision 07](../../../planning/scope/07-classifier-error-text-sync.md)).
  This PR wraps the existing helper; it does not re-derive it from upstream.
- The **clamp-vs-throw** difference on a server-requested delay above the cap:
  kern-link's `capRetryDelay` (`httpretry.go:296-302`) clamps, upstream's
  `validateServerRetryDelayMs` throws. Pre-existing, applies to every adapter,
  and belongs to the same epic 9 reconciliation. Do not change it here.
- `fetch` injection into the images client —
  [issue 07](/epic-3-catalog-schema-and-export-tooling/issues/07-images-options-base-and-auth-overrides.md)
  (#218), which lands after this one and injects `Options.Fetch` into the retry
  loop this PR introduces.
  [Epic 2 issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md)
  declares `ProviderRequestOptions` and `FetchFunction`; it does not touch
  `ai/images`.
- OpenRouter images **OAuth** — [Epic 7](/epic-7-four-new-oauth-flows/EPIC_7.md).
- The `getAuth(providerId | model, overrides)` overload and
  `AuthResolutionOverrides` on the images `Models` —
  [issue 07](/epic-3-catalog-schema-and-export-tooling/issues/07-images-options-base-and-auth-overrides.md)
  (#218). [Epic 6](/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md) owns
  the chat-side auth restructure and disclaims `ai/images`.
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
- [ ] All six existing offline tests in `ai/images/openrouter_test.go` still
      pass unchanged, and `TestGenerateImagesOpenRouter_LiveSmoke` still skips
      without `OPENROUTER_API_KEY`.
- [ ] `git diff --name-only` lists no file under `ai/apis/anthropic`,
      `ai/apis/azure`, `ai/apis/bedrock`, `ai/apis/codex` or
      `ai/apis/internal/httpretry`, and
      `git grep -n 'ai/apis/internal/httpretry' -- '*.go'` still resolves for all
      four text adapters — the shim is additive, so their retry tests are not
      merely still green, they are untouched.
- [ ] No test sleeps on a real backoff: every retry test shrinks
      `httpretry.BaseDelay` and restores it with `t.Cleanup`.
- [ ] `git diff --name-only` lists nothing under `docs/planning/`; the drift
      record at
      `docs/epics/epic-3-catalog-schema-and-export-tooling/drift/06-images-live-smoke-claim.md`
      exists, has valid frontmatter, names decision 14's line, and has its
      **Disposition** and **Revisit when** fields filled with a concrete trigger.
- [ ] `docs/PORTING.md` — the mapping row for the images adapter reflects the
      retry behavior; `ai/apis/internal/httpretry/httpretry.go` still carries
      **no** `// Ports:` header (it is original code, and its absence is
      meaningful per
      [CONVENTIONS.md](../../../planning/CONVENTIONS.md)), and the new
      `ai/apis` shim file takes none either, for the same reason.
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
- `ai/images/openrouter_test.go:18-170` — the six existing offline tests to keep
  green (`:18`, `:80`, `:108`, `:122`, `:142`, `:165`); `:171-201` the live
  smoke.
- `ai/apis/internal/httpretry/httpretry.go` (302 lines) — `:1-13` package doc,
  `:36-45` `DefaultMaxRetries` / `BaseDelay`, `:49-76` `Request` / `Config`,
  `:89-168` `Do`, `:249-302` `IsRetryable` / `RetryAfterDelay` /
  `capRetryDelay`. **Read and wrapped, not edited.**
- `ai/apis/simpleopts.go`, `ai/apis/transform.go` — the existing `package apis`
  the shim joins; it imports `ai` only, so `ai/images` can import it without a
  cycle.
- Read-only: `docs/planning/CONVENTIONS.md:22-26` (the decided home of
  `ai/apis/internal/httpretry`), `docs/planning/SPECS.md:73` (package-map row)
  and `:301` (the gated-smoke sentence),
  `docs/planning/scope/14-images-surface.md:11`, `:54-57`, `:70-72` (the stale
  "exactly one live smoke test" claim the drift record names).
- `docs/PORTING.md` — the images-adapter mapping row.
- Upstream at `936aff00`: `packages/ai/src/api/openrouter-images.ts:56-82`
  (the `retryProviderRequest` wrapping), `packages/ai/src/utils/provider-retry.ts`.

## Dependencies

- **Blocked by**: None. Independent of issues 01–05 — it touches no catalog data
  and no export tooling. In practice it will rebase over whichever of them lands
  first only where both touch `ai/images`.
- **Blocks**: [Issue 07](/epic-3-catalog-schema-and-export-tooling/issues/07-images-options-base-and-auth-overrides.md)
  (#218) — it injects `Options.Fetch` into the retry loop this PR introduces, so
  it rebases on this one. Note for
  [Epic 9](/epic-9-classifier-audit-and-release/EPIC_9.md):
  the `provider-retry.ts` reconciliation it owns is against
  `ai/apis/internal/httpretry`, at that path, before and after this PR.

## PR size note

`M` — ~400 changed lines across three files plus the drift record: the new
`ai/apis` retry shim, the rewrite of `generateImagesOpenRouter`'s transport
(`ai/images/openrouter.go:204-291`), and six new offline `httptest` retry tests
in `ai/images/openrouter_test.go`. Split past ~500, and the seam is the shim —
land `ai/apis/retry.go` with its own coverage first. If it turns out to need a
change inside `ai/apis/internal/httpretry`, that change lands additively and is
proved by the four text adapters' retry tests staying untouched.
