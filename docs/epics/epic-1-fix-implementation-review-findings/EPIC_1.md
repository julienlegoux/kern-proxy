---
type: Epic
title: "Fix IMPLEMENTATION_REVIEW findings"
description: "Fix the two functional bugs (provider OAuth wiring, Google /v1beta path doubling) and close the // Ports: header traceability gap found in docs/IMPLEMENTATION_REVIEW.md."
tags: [epic]
timestamp: 2026-07-08T19:42:30Z
epic: 1
slug: fix-implementation-review-findings
status: open
gh_issue: 76
milestone: 15
resource: https://github.com/julienlegoux/kern-proxy/issues/76
source: docs/PLAN_fix.md
---

# Epic 1: Fix IMPLEMENTATION_REVIEW findings

## Goal
`docs/IMPLEMENTATION_REVIEW.md` is a static review of the Go port against the
(now-archived) plan. All three findings were verified against current code and are
factually accurate: two are real functional bugs and one is a traceability gap. This
epic closes all three, restoring correctness (OAuth logins and Google requests both
work end-to-end) and completing the upstream-source traceability that the repo
mandates.

## Scope
Three fixes, expected to be roughly one PR each. TDD (red → green) applies to the two
behavior changes; the header pass is comment-only.

- **Fix 1 — Wire provider OAuth bindings to the real strategies (CRITICAL).**
  `ai/auth/oauth` fully implements `AnthropicOAuth`, `CopilotOAuth`, and `CodexOAuth`,
  and `cmd/pi-ai login` uses them, but the bindings `ai.ResolveProviderAuth`
  (`ai/resolve.go:88-111`) consults are stale:
  - `ai/providers/openai_codex.go` and `ai/providers/github_copilot.go` return
    `errCodexOAuthPending` / `errCopilotOAuthPending` ("not implemented yet (Epic 12)")
    from Login/Refresh/ToAuth.
  - `ai/providers/anthropic.go` has **no** OAuth binding — and because a stored
    `*OAuthCredential` with `auth.OAuth == nil` falls through to `resolve.go:110`
    returning `nil, nil`, a saved Anthropic login also **blocks the ambient API-key
    fallback**.

  Changes: import `ai/auth/oauth` into `anthropic.go` and add
  `OAuth: oauth.AnthropicOAuth` (keep the existing `APIKey` env strategy and its
  `ANTHROPIC_OAUTH_TOKEN` precedence); replace the stub closures in `openai_codex.go`
  with `oauth.CodexOAuth` and delete `errCodexOAuthPending` + now-unused
  `context`/`errors` imports; replace the stub closures in `github_copilot.go` with
  `oauth.CopilotOAuth` and delete `errCopilotOAuthPending` (keep the
  `COPILOT_GITHUB_TOKEN` `APIKey` fallback and `RefreshModels` logic untouched). Update
  the three header comments that currently say OAuth is "deferred to Epic 12". No
  import cycle: `ai/auth/oauth` imports `ai` only; `providers` already imports
  `ai/auth`.

  Tests (update first): rework
  `TestOpenAICodexProviderAdvertisesOAuthPendingEpic12` (~`providers_test.go:306`) to
  assert `provider.Auth().OAuth == oauth.CodexOAuth` and
  `Name == "OpenAI (ChatGPT Plus/Pro)"` (do **not** call the real `Login` — network);
  rework `TestGitHubCopilotProviderAdvertisesOAuthPendingEpic12`
  (~`github_copilot_test.go:25`) to assert `oauth.CopilotOAuth`; add an Anthropic case
  asserting `oauth.AnthropicOAuth`; leave
  `TestAnthropicAuthResolvesOAuthTokenPrecedence` (~line 216) as-is. Add a regression
  test (in `providers_test.go` or `ai/resolve_test.go`): store an `*ai.OAuthCredential`
  for `"anthropic"` and assert `ResolveProviderAuth` returns
  `AuthResult{Source: "OAuth"}` with `Auth.APIKey == <access token>`, not the current
  `nil, nil`.

- **Fix 2 — Stop doubling the Google `/v1beta` path segment (CRITICAL).**
  `ai/catalog/data/models/google.json:7` and `ai/providers/google.go:18` both set base
  URL `https://generativelanguage.googleapis.com/v1beta`, while
  `ai/apis/google/google.go:438-444` `requestURL` appends `/v1beta/models/...` again →
  `.../v1beta/v1beta/models/...` → 404. `live_smoke_test.go:34` only passes via a
  per-request `model.BaseURL` override workaround.

  Approach (chosen): normalize in `requestURL`. In the non-vertex google package
  `model.BaseURL` is read *only* at `google.go:439` inside `requestURL`; nothing else
  consumes it (Vertex has its own separate `customHost` logic, untouched). After
  `base := strings.TrimRight(model.BaseURL, "/")`, strip a trailing `/v1beta`
  (`base = strings.TrimSuffix(base, "/v1beta")`) before the
  `fmt.Sprintf(".../v1beta/models/%s...")`; add a short comment. Remove the
  `model.BaseURL` override workaround and comment at
  `cmd/pi-ai/toolcallexample/live_smoke_test.go:34-41` and use the catalog model as-is.

  Tests (add first): a `requestURL` case in `ai/apis/google/google_test.go` asserting a
  base URL of `https://generativelanguage.googleapis.com/v1beta` yields a single
  `/v1beta/models/<id>:streamGenerateContent?alt=sse` segment; plus a regression test
  through the real catalog
  (`providers.Models(nil).GetModel("google", ...)` → build request URL → assert no
  `/v1beta/v1beta`), placed where it can import `providers` without an import cycle into
  `ai/apis/google` (e.g. under `cmd/pi-ai/toolcallexample` or a provider-level test).

- **Fix 3 — Add `// Ports:` headers to the 16 non-test source files (no SHA).**
  Add a top-of-file `// Ports: packages/ai/src/…` comment (upstream TS source only —
  **no `@ <sha>`**) to each, deriving the path from `docs/archived/PORTING.md` rows and
  sibling files in the same package that already carry headers:
  - Bedrock (7): `awsconvert.go`, `errors.go`, `helpers.go`, `messages.go`,
    `stream.go`, `thinking.go`, `wire.go` → `src/api/bedrock*.ts` /
    `src/api/bedrock-convert-messages.ts`.
  - Codex (1): `websocket.go` → codex websocket portion of `src/api/codex*.ts`.
  - Google (1): `stream.go` → stream-decode portion of `src/api/google*.ts`.
  - Mistral (2): `messages.go`, `stream.go` → `src/api/mistral-conversations.ts`.
  - `ai/internal/partialjson/partial.go` → `src/utils/json-parse.ts` + npm
    `partial-json`.
  - `ai/json.go` → its upstream JSON util (confirm from file body / PORTING.md).
  - `ai/providers/refreshmodels.go` → `refreshModels` provider machinery (confirm from
    PORTING.md).
  - `cmd/pi-ai/example/main.go`, `cmd/pi-ai/toolcallexample/roundtrip.go` — likely
    original Go demo harnesses; if no upstream counterpart, add an explicit
    `// Ports: none — original <purpose>` note rather than inventing a mapping.

  The 56 `_test.go` files are out of scope. Headers are comment-only; no test changes.

## Out of scope
- The 56 `_test.go` files (explicitly excluded from the `// Ports:` header pass).
- The `@ <sha>` suffix on `// Ports:` headers — no file has ever carried it; naming the
  upstream file is enough.
- Vertex's `customHost` logic in the google package (untouched by Fix 2).

## Acceptance criteria
- `go test ./...` all green.
- `go build ./...` clean (no unused-import errors after removing stub
  `errors`/`context` imports).
- `go vet ./...` clean.
- Grep confirms no non-test `.go` file under `ai`/`cmd` lacks a `// Ports:` header.
- Regression tests prove: a stored Anthropic OAuth credential resolves with
  `Source: "OAuth"`; a Google catalog model produces a single-`/v1beta` URL.
- Live smoke tests still pass **without** the removed `model.BaseURL` workaround (run if
  creds available; otherwise the unit-level regression test stands in).

## Dependencies
None — self-contained fixes against current code.

## Notes
- **Project-wide constraint (applies to all three fixes):** the repo mandates TDD
  (red → green) and `// Ports:` headers naming the upstream source. For each behavior
  change, update/add the test first. This is a repo-wide rule, not specific to this
  epic.
- Fixes 1 and 2 are the two verified functional bugs (both CRITICAL); Fix 3 is the
  traceability gap.
- Source review lives in `docs/IMPLEMENTATION_REVIEW.md`; the plan this epic was split
  from is `docs/PLAN_fix.md`. Path/line references above were current as of that plan
  and should be re-verified at implementation time.
- The Google `/v1beta` doubling is also recorded in the project memory as a
  previously-known unfixed bug; Fix 2 closes it.
