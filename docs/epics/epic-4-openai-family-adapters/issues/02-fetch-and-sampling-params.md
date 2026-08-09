---
type: Issue
title: "Honor Fetch and SamplingParams in the four OpenAI-family adapters"
description: "Make the injectable HTTP doer actually carry requests through httpretry, and merge arbitrary samplingParams into each adapter's request body last."
tags: [epic-4]
timestamp: 2026-08-09T05:17:46Z
epic: 4
issue: 02
slug: fetch-and-sampling-params
size: M
status: open
gh_issue: 148
resource: https://github.com/kern-ia/kern-link/issues/148
depends_on: []
---

# Honor Fetch and SamplingParams in the four OpenAI-family adapters

## Summary

[Epic 2 issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md)
adds `Fetch` and `SamplingParams` to the options structs and explicitly hands
the wire behavior to this epic: *"Honoring `Fetch` and `SamplingParams` on the
wire is Epic 4 and Epic 5."* This PR does the epic-4 half.

Both are one-line changes upstream and neither is one line in Go:

- **`fetch`** is threaded into every `new OpenAI({fetch})` client. All four Go
  adapters instead call `httpretry.Do(ctx, httpretry.Request{…},
  httpretry.Config{Opts: opts, …})` (`openaicompletions.go:123`,
  `openairesponses.go:129`, `azure.go`, `codex.go:277`), so the honest place to
  honor an injected doer is `httpretry` itself — which is exactly where
  [Epic 2 issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md)
  said it would plausibly land. Doing it there gives the epic-5 adapters the
  same behavior for free; that is a feature, but state it in the PR body rather
  than letting it look like scope creep.
- **`samplingParams`** is `Object.assign(params, options.samplingParams)`,
  applied **last so custom keys override the named request fields**. Go's
  request bodies are typed structs (`wireRequest` in each adapter), so
  overriding a named field from a `map[string]any` needs a deliberate
  serialization seam, not a struct field.

## Scope

- **Fetch** — in `httpretry` (at whatever path
  [Epic 3 issue 06](/epic-3-catalog-schema-and-export-tooling/issues/06-images-adapter-retry.md)
  left it: `ai/internal/httpretry` if that landed, `ai/apis/internal/httpretry`
  otherwise):
  - Use `Config.Opts.Fetch` as the doer when non-nil, falling back to the
    current default client when nil. Retry, timeout, and error classification
    behavior must be identical either way — the override replaces the transport,
    not the loop.
  - The file is new code with no upstream counterpart: **do not add a `// Ports:`
    header** ([CONVENTIONS.md](../../../planning/CONVENTIONS.md), provenance
    headers mark ported code, and absence is meaningful).
- **Fetch, WebSocket exclusion** — upstream states `fetch` does not affect
  WebSocket transports. Codex's WebSocket path (`ai/apis/codex/websocket.go`)
  therefore ignores it. Add a one-line comment saying so at the dial site, so
  the next reader does not file it as a bug.
- **SamplingParams** — merge `opts.SamplingParams` into the request body of all
  four adapters, last, with per-request keys overriding named fields:
  - `ai/apis/openaicompletions/openaicompletions.go:340` `buildParams`
  - `ai/apis/openairesponses/openairesponses.go:326` `buildParams`
  - `ai/apis/azure/azure.go:257` `buildParams`
  - `ai/apis/codex/params.go:66` `buildRequestBody`
  - Mechanism is an implementer call, but it must survive the override
    requirement. The two workable shapes are (a) a `MarshalJSON` on each
    `wireRequest` that marshals the struct, unmarshals to
    `map[string]json.RawMessage`, and overlays the sampling keys; or (b) a
    shared helper in `ai/apis` (beside `transform.go` / `simpleopts.go`) doing
    the same so it is written once. Prefer (b); if you take (a), say why.
  - `Model.SamplingParams` vs `StreamOptions.SamplingParams` precedence is
    merged upstream in `simple-options.ts` and owned by
    [Epic 2 issue 06](/epic-2-core-types-and-models-contracts/issues/06-simple-options-and-lazy.md).
    This PR consumes the already-merged map; do not re-implement the precedence.

## Out of scope

- The five non-OpenAI-family adapters' `samplingParams` merge — Epic 5. (The
  `httpretry` Fetch change does reach them; that is unavoidable and intended.)
- `ai/images` — [Epic 3](/epic-3-catalog-schema-and-export-tooling/EPIC_3.md)
  owns that surface.
- Changing retry policy, delay clamping, or error classification. Upstream's new
  `utils/provider-retry.ts` overlaps `httpretry`, and whether it supersedes it is
  [Epic 9](/epic-9-classifier-audit-and-release/EPIC_9.md)'s call
  ([Epic 3 issue 06](/epic-3-catalog-schema-and-export-tooling/issues/06-images-adapter-retry.md)
  recorded the question). Touch only the doer.

## Acceptance criteria / Definition of done

- [ ] `TestFetchOverrideCarriesRequest` (in `httpretry`) — a `Fetch` that records
      the request and returns a canned 200 is the only transport used; the
      default client is never called.
- [ ] `TestFetchOverrideStillRetries` — a `Fetch` returning 429 then 200 is
      invoked twice and the call succeeds (shrink `httpretry.BaseDelay` in the
      test and restore it with `t.Cleanup`, as `codex/stream_test.go:532` does).
- [ ] `TestFetchNilFallsBackToDefaultClient` — nil `Fetch` still reaches an
      `httptest` server.
- [ ] One test per adapter, offline `httptest`, asserting a request built with
      `SamplingParams: map[string]any{"top_p": 0.3, "temperature": 0.9}` puts
      both keys in the decoded body **and** that `temperature` is `0.9` even when
      `StreamOptions.Temperature` was set to something else — the override
      direction is the whole point:
      `TestOpenAICompletionsSamplingParamsOverrideNamedFields`,
      `TestOpenAIResponsesSamplingParamsOverrideNamedFields`,
      `TestAzureSamplingParamsOverrideNamedFields`,
      `TestCodexSamplingParamsOverrideNamedFields`.
- [ ] Codex's WebSocket dial site carries the comment explaining that `Fetch` is
      SSE-only, matching upstream.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(apis): honor Fetch and samplingParams in the openai-family adapters`.

## Relevant files / areas

- `ai/apis/internal/httpretry/httpretry.go` — `Do`, `Config` (`:60-76` per
  [Epic 3 issue 06](/epic-3-catalog-schema-and-export-tooling/issues/06-images-adapter-retry.md));
  may already be `ai/internal/httpretry` by the time this lands.
- `ai/apis/openaicompletions/openaicompletions.go:123,340`
- `ai/apis/openairesponses/openairesponses.go:129,326`
- `ai/apis/azure/azure.go:257`
- `ai/apis/codex/codex.go:277`, `ai/apis/codex/params.go:66`,
  `ai/apis/codex/websocket.go`
- `ai/apis/transform.go`, `ai/apis/simpleopts.go` — where a shared merge helper
  would sit.
- Upstream: `test/fetch-option.test.ts` (+178) and the `samplingParams` tails of
  each adapter's `buildParams` at `936aff00`.

## Dependencies

- **Blocked by**: [Epic 2 issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md)
  (the `Fetch` and `SamplingParams` fields) and
  [Epic 2 issue 06](/epic-2-core-types-and-models-contracts/issues/06-simple-options-and-lazy.md)
  (the model/request merge).
- **Blocks**: None, but land it early — it touches all four `buildParams`
  functions the rest of the epic rewrites.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
