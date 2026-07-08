# Fix Plan — findings in docs/IMPLEMENTATION_REVIEW.md

## Context

`docs/IMPLEMENTATION_REVIEW.md` is a static review of the Go port against the
(now-archived) plan. All three findings were verified against current code and
are factually accurate. Two are real functional bugs; one is a traceability
gap. Decisions:

- **Fix the two functional bugs** (OAuth wiring, Google `/v1beta` doubling).
- **Add `// Ports:` headers to the 16 non-test source files** that lack them —
  **without** the `@ <sha>` suffix (no file has ever carried it; naming the
  upstream file is enough). The 56 `_test.go` files are out of scope.

The repo mandates TDD (red → green) and `// Ports:` headers naming the upstream
source. For each behavior change, update/add the test first.

---

## Fix 1 — Wire provider OAuth bindings to the real strategies (CRITICAL)

**Problem.** `ai/auth/oauth` fully implements `AnthropicOAuth`, `CopilotOAuth`,
`CodexOAuth`, and `cmd/pi-ai login` uses them and saves credentials keyed by
provider id. But the bindings that `ai.ResolveProviderAuth`
(`ai/resolve.go:88-111`) consults are stale:
- `ai/providers/openai_codex.go` and `ai/providers/github_copilot.go` return
  `errCodexOAuthPending` / `errCopilotOAuthPending` ("not implemented yet
  (Epic 12)") from Login/Refresh/ToAuth.
- `ai/providers/anthropic.go` has **no** OAuth binding — and because a stored
  `*OAuthCredential` with `auth.OAuth == nil` falls through to `resolve.go:110`
  returning `nil, nil`, a saved Anthropic login also **blocks the ambient
  API-key fallback**.

No import cycle: `ai/auth/oauth` imports `ai` only; `providers` already imports
`ai/auth`.

**Changes:**

- `ai/providers/anthropic.go` — import `ai/auth/oauth`; add
  `OAuth: oauth.AnthropicOAuth` to the `ai.ProviderAuth` literal (keep the
  existing `APIKey` env strategy and its `ANTHROPIC_OAUTH_TOKEN` precedence).
  Update the header comment (currently says OAuth is "deferred to Epic 12").
- `ai/providers/openai_codex.go` — replace the three stub closures with
  `oauth.CodexOAuth`; delete `errCodexOAuthPending` and now-unused
  `context`/`errors` imports. Update the header comment.
- `ai/providers/github_copilot.go` — replace the three stub closures in the
  `OAuth` field with `oauth.CopilotOAuth`; delete `errCopilotOAuthPending`.
  Keep the `COPILOT_GITHUB_TOKEN` `APIKey` fallback and `RefreshModels` logic
  untouched. Update the header comment.

**Tests (update first, red → green):**

- `ai/providers/providers_test.go` —
  `TestOpenAICodexProviderAdvertisesOAuthPendingEpic12` (~line 306): rework to
  assert `provider.Auth().OAuth == oauth.CodexOAuth` and
  `Name == "OpenAI (ChatGPT Plus/Pro)"`. Do **not** call the real `Login`
  (network). Leave `TestAnthropicAuthResolvesOAuthTokenPrecedence` (~line 216)
  as-is — it tests env-token precedence, unaffected.
- `ai/providers/github_copilot_test.go` —
  `TestGitHubCopilotProviderAdvertisesOAuthPendingEpic12` (~line 25): assert
  `provider.Auth().OAuth == oauth.CopilotOAuth`.
- Add an Anthropic case: `AnthropicProvider().Auth().OAuth == oauth.AnthropicOAuth`.
- **Regression test for the actual bug** (`providers_test.go` or
  `ai/resolve_test.go`): store an `*ai.OAuthCredential` for `"anthropic"` and
  assert `ResolveProviderAuth` returns `AuthResult{Source: "OAuth"}` with
  `Auth.APIKey == <access token>`, instead of the current `nil, nil`.

---

## Fix 2 — Stop doubling the Google `/v1beta` path segment (CRITICAL)

**Problem.** `ai/catalog/data/models/google.json:7` and
`ai/providers/google.go:18` both set base URL
`https://generativelanguage.googleapis.com/v1beta`, while
`ai/apis/google/google.go:438-444` `requestURL` appends `/v1beta/models/...`
again → `.../v1beta/v1beta/models/...` → 404. `live_smoke_test.go:34` only
passes via a per-request `model.BaseURL` override workaround.

**Approach (chosen): normalize in `requestURL`.** Verified safe — in the
non-vertex google package `model.BaseURL` is read *only* at `google.go:439`
inside `requestURL`; nothing else consumes it. (Vertex has its own separate
`customHost` logic, untouched.) Trimming a trailing `/v1beta` there is robust
whether the base URL carries the segment or not, touches one file + its test,
and edits no catalog data.

**Changes:**

- `ai/apis/google/google.go` `requestURL` — after
  `base := strings.TrimRight(model.BaseURL, "/")`, strip a trailing `/v1beta`
  (`base = strings.TrimSuffix(base, "/v1beta")`) before the
  `fmt.Sprintf(".../v1beta/models/%s...")`. Add a short comment.
- `cmd/pi-ai/toolcallexample/live_smoke_test.go:34-41` — remove the
  `model.BaseURL` override workaround and its comment; use the catalog model
  as-is.

**Tests (add first, red → green):**

- `ai/apis/google/google_test.go` — a `requestURL` case asserting a base URL of
  `https://generativelanguage.googleapis.com/v1beta` yields a single
  `/v1beta/models/<id>:streamGenerateContent?alt=sse` segment.
- Regression test through the real catalog:
  `providers.Models(nil).GetModel("google", ...)` → build request URL → assert
  no `/v1beta/v1beta`. Place where it can import `providers` without an import
  cycle into `ai/apis/google` (e.g. under `cmd/pi-ai/toolcallexample` or a
  provider-level test).

---

## Fix 3 — Add `// Ports:` headers to the 16 non-test source files (no SHA)

Add a top-of-file `// Ports: packages/ai/src/…` comment (upstream TS source
only — **no `@ <sha>`**) to each. Derive the correct path from
`docs/archived/PORTING.md` rows and sibling files in the same package that
already carry headers.

- **Bedrock (7)** — `awsconvert.go`, `errors.go`, `helpers.go`, `messages.go`,
  `stream.go`, `thinking.go`, `wire.go` → `src/api/bedrock*.ts` /
  `src/api/bedrock-convert-messages.ts`.
- **Codex (1)** — `websocket.go` → codex websocket portion of `src/api/codex*.ts`.
- **Google (1)** — `stream.go` → stream-decode portion of `src/api/google*.ts`.
- **Mistral (2)** — `messages.go`, `stream.go` → `src/api/mistral-conversations.ts`.
- **`ai/internal/partialjson/partial.go`** → `src/utils/json-parse.ts` + npm
  `partial-json`.
- **`ai/json.go`** → its upstream JSON util (confirm from file body / PORTING.md).
- **`ai/providers/refreshmodels.go`** → `refreshModels` provider machinery
  (confirm from PORTING.md).
- **`cmd/pi-ai/example/main.go`, `cmd/pi-ai/toolcallexample/roundtrip.go`** —
  likely original Go demo harnesses. If no upstream counterpart, add an
  explicit `// Ports: none — original <purpose>` note rather than inventing a
  mapping.

No test changes; headers are comment-only.

---

## Verification

1. `go test ./...` — all green.
2. `go build ./...` — clean (no unused-import errors after removing stub
   `errors`/`context` imports).
3. `go vet ./...` — clean.
4. Grep: no non-test `.go` file under `ai`/`cmd` lacks a `// Ports:` header.
5. New regression tests prove: stored Anthropic OAuth credential resolves with
   `Source: "OAuth"`; Google catalog model produces a single-`/v1beta` URL.
6. Live smoke tests still pass **without** the removed workaround (run if creds
   available; otherwise rely on the unit-level regression test).
