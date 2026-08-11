---
type: Issue
title: "Stream pi-messages over net/http: request, options, error mapping, and the Stream entry points"
description: "Complete ai/apis/pimessages with Stream and StreamSimple — the POST to <baseUrl>/messages, the flat per-adapter options, the response-error type with its diagnostic, and the ported upstream test suite."
tags: [epic-8]
timestamp: 2026-08-11T20:00:00Z
epic: 8
issue: 02
slug: pimessages-stream-entry
size: L
status: open
gh_issue: 188
resource: https://github.com/kern-ia/kern-link/issues/188
depends_on: ["01"]
---

# Stream pi-messages over net/http: request, options, error mapping, and the Stream entry points

## Summary

The transport half of the tenth adapter, on top of
[issue 01](/epic-8-pi-messages-and-radius/issues/01-pimessages-wire-and-converter.md)'s
converter: one POST of `{model, context, options}` to `<baseUrl>/messages`, an
SSE stream back, and the two `ai.StreamFunc`s the provider binding in
[issue 04](/epic-8-pi-messages-and-radius/issues/04-radius-provider-binding.md)
plugs into `ai.StreamFuncs`. Epic 8's acceptance criterion 2 — upstream's 248
test lines ported and passing offline — is completed here: issue 01 ports the
converter cases, this PR ports the transport and error cases.

**The request body is `ai.Context` verbatim.** No `apis.TransformMessages`, no
vendor message shapes: the backend speaks pi's own model, so
`json.Marshal(chat)` *is* the wire form, and `ai/json.go`'s codecs are what
make that true. If a round-trip mismatch shows up, it is a bug in the codecs,
not something to paper over with a local message struct.

Three decisions worth making deliberately rather than discovering:

1. **Cache retention does not default to `short` here.** `ai.EffectiveCacheRetention`
   (`ai/options.go:303`) resolves the zero value to `"short"`, which is right
   for adapters that must send *something*. Upstream's `resolveCacheRetention`
   sends `undefined` unless the caller set it or `PI_CACHE_RETENTION=long` is in
   the environment — the comment says backend defaults apply when unset. Omit
   the key; do not call `EffectiveCacheRetention`.
2. **This adapter does not retry.** Upstream issues a single `fetch` with no
   retry wrapper, so `ai/apis/internal/httpretry` stays out and `MaxRetries` is
   ignored. That is parity, not an oversight — say so in a comment beside the
   request, and [issue 05](/epic-8-pi-messages-and-radius/issues/05-porting-and-docs.md)
   records it.
3. **`StreamSimple` is a pass-through, unlike every other adapter here.**
   Upstream's `streamSimple` forwards `reasoning`, `toolChoice` and `debug`
   into `stream` and does nothing else — no `resolveSimpleOptions`, no
   max-tokens clamping. So do **not** route it through
   `apis.BuildBaseOptions`/`ClampMaxTokensToContext`
   (`ai/apis/simpleopts.go:20-47`). Mirror upstream and pin it with the test
   named below.

Per-adapter options follow the repo's established flat-merge deviation
(`ai/options.go:126-133`, the anthropic and mistral blocks): Go's fixed
`StreamFunc` signature cannot carry a per-api options type, so
`PiMessagesOptions`' three extra fields become flat fields on
`ai.StreamOptions`.

The error type is where the diagnostics contract shows up:
`piMessagesResponseError` carries fields, so it takes a **pointer** receiver
(`docs/planning/CONVENTIONS.md`, "Receivers split by kind"), and it exposes
`Code() string` precisely so `ai.ExtractDiagnosticError` (`ai/diagnostics.go:44`)
lifts the backend's error code into the diagnostic without any special-casing.

## Scope

- `ai/options.go` — add the pi-messages block to `StreamOptions`, in the same
  commented style as the mistral block (`:224-232`):
  `PiMessagesReasoning ThinkingLevel`, `PiMessagesToolChoice string`
  (`auto`/`none`/`required`), `PiMessagesToolChoiceFunction string` (pins one
  named function, overriding `PiMessagesToolChoice` — the same escape valve
  `MistralToolChoiceFunction` uses for a union Go cannot express), and
  `PiMessagesDebug bool`.
- `ai/apis/pimessages/errors.go` (new) — `piMessagesResponseError` with
  `status`, `code`, and the diagnostic details map; `Error()` rendering
  `<status> <statusText>: <message or body>[ (<code>)]` (Go's `resp.Status` is
  already `"401 Unauthorized"`); `Code() string`; body parsing that only treats
  the payload as structured when `error` is a JSON object; and the 8192-char
  truncation with a trailing `…` for the raw body kept in the details.
  Details keys, verbatim from upstream: `version` (1), `provider`, `model`,
  `url`, `status`, `statusText`, `error`, `body` (only when the body did not
  parse), `timestampMs`.
- `ai/apis/pimessages/stream.go` (new) — `Stream` and `StreamSimple`
  (`ai.StreamFunc` / `ai.SimpleStreamFunc`):
  - URL: `strings.TrimRight(model.BaseURL, "/") + "/messages"`, plus
    `?debug=1` when `PiMessagesDebug` is set.
  - Missing API key fails the stream in-band with upstream's exact text
    `No API key provided for provider "<provider>"`.
  - Headers: `authorization: Bearer <key>`, `accept: text/event-stream`,
    `content-type: application/json`, merged with the caller's via
    `ai.MergeProviderHeaders` (`ai/headers.go:38`) so a nil value still
    suppresses a default.
  - Payload: `{model: model.ID, context: chat, options: {temperature,
    maxTokens, reasoning, cacheRetention, sessionId, toolChoice}}` with every
    unset option **absent** from the JSON, not null — upstream's
    `JSON.stringify` drops `undefined`, and the upstream test asserts the exact
    object. Then `OnPayload` may replace it, and `OnResponse` is called with
    status and headers before the body is read (`ai.ProviderResponse`,
    `ai/options.go:41-45`).
  - `resolveCacheRetention`: caller's value if set, else `long` when
    `PI_CACHE_RETENTION` resolves to `long` through a package-local
    `providerEnvValue(name, opts.Env)` helper — the same private copy every
    other adapter keeps (`ai/apis/anthropic/anthropic.go:883`,
    `ai/apis/openaicompletions/openaicompletions.go:440`), else unset.
  - Non-2xx: read the body, fail with `*piMessagesResponseError`, and attach a
    `pi_messages_response_failure` diagnostic to the terminal message.
  - Read loop: `sse.NewReader(resp.Body)` → `decodeEvent` → the converter →
    `stream.Push`, returning on the first `done`/`error`. Reaching EOF without
    a terminal event fails with
    `<provider> stream ended without a terminal event`.
  - A cancelled context ends the stream with `ai.StopReasonAborted` and **no**
    diagnostic (upstream attaches one only when not aborted).
  - Upstream's `response has no body` branch has no Go counterpart —
    `http.Response.Body` is never nil. Note that in a comment rather than
    inventing a check that cannot fire.
- `ai/apis/pimessages/stream_test.go` (new) — the port of
  `test/pi-messages.test.ts` (248 lines) over `httptest`, plus the Go-side
  cases below.

## Out of scope

- The wire union, frame decoding and converter —
  [issue 01](/epic-8-pi-messages-and-radius/issues/01-pimessages-wire-and-converter.md).
- The `radius` binding and its catalog —
  [issues 03](/epic-8-pi-messages-and-radius/issues/03-radius-gateway-config.md)
  and [04](/epic-8-pi-messages-and-radius/issues/04-radius-provider-binding.md).
  Nothing in `ai/providers` changes here; the adapter is reachable through its
  exported `Stream`/`StreamSimple` and that is enough to test it.
- Retry, `httpretry`, and honoring `MaxRetries` — see the Summary. If a
  reviewer asks "why no retry here", the answer belongs in the PR body.
- A live smoke test. Eight adapters carry a `TestLiveSmoke_*`
  (`docs/planning/SPECS.md`, "Testing infrastructure"), but there is no public
  pi-messages endpoint to point one at until a Radius gateway exists.
- `docs/PORTING.md` and `docs/auth.md` —
  [issue 05](/epic-8-pi-messages-and-radius/issues/05-porting-and-docs.md).

## Acceptance criteria / Definition of done

- [ ] `TestStreamPostsTheMessagesRequestAndResolvesTheTerminalMessage` — against
      an `httptest` server serving upstream's event sequence: the recorded
      request path is `/v1/messages`, `authorization` is `Bearer test-key`, the
      caller's `x-custom: 1` header survives the merge, and the decoded body is
      exactly `{"model":"auto","context":<the marshalled ai.Context>,
      "options":{"maxTokens":100,"sessionId":"session-1","toolChoice":"auto"}}`
      — no `temperature`, `reasoning` or `cacheRetention` keys. The resolved
      message has `StopReason == ai.StopReasonToolUse`, the sent `Usage`, and
      `ResponseID == "resp_1"`.
- [ ] `TestStreamAppendsDebugAndReportsResponseHeaders` — with
      `PiMessagesDebug` set, the request path is `/v1/messages?debug=1` and
      `OnResponse` observes status 200 and the server's
      `x-pi-gateway-upstream-provider: anthropic` header.
- [ ] `TestStreamSurfacesBackendErrorResponsesWithDiagnostics` — a 401 with
      `{"error":{"message":"Token expired","code":"unauthorized"}}` ends the
      stream with `StopReason == ai.StopReasonError`, an `ErrorMessage`
      containing `401`, `Token expired` and `unauthorized`, exactly one
      diagnostic of type `pi_messages_response_failure` whose `Details["status"]`
      is 401 and whose `Error.Code` is `unauthorized`.
- [ ] `TestStreamKeepsTheRawBodyInDiagnosticsWhenItDoesNotParse` — a 500 with
      body `not json` puts that text under `Details["body"]` and leaves
      `Details["error"]` unset; a body over 8192 chars is truncated with a
      trailing `…`.
- [ ] `TestStreamPropagatesServerSentErrorEvents` — a `start` then
      `{"type":"error","reason":"error","errorMessage":"Upstream failed"}`
      resolves with that `ErrorMessage`, that usage, and **no**
      `pi_messages_response_failure` diagnostic (the HTTP call succeeded).
- [ ] `TestStreamFailsWithoutAnAPIKey` — with no `APIKey`, the message fails
      with exactly `No API key provided for provider "radius"` and no request
      is made.
- [ ] `TestStreamFailsWhenTheStreamEndsWithoutATerminalEvent` — a body of
      `start`/`text_start`/`text_delta` then EOF resolves with an
      `ErrorMessage` containing `stream ended without a terminal event`.
- [ ] `TestStreamTrimsTrailingSlashesFromTheBaseURL` — a `BaseURL` ending in
      `/v1//` still posts to `/v1/messages`.
- [ ] `TestStreamAppliesOnPayloadReplacement` — an `OnPayload` returning a
      different object sends that object; returning `(nil, nil)` sends the
      original.
- [ ] `TestStreamResolvesCacheRetentionFromTheEnvironment` — `Env` carrying
      `PI_CACHE_RETENTION=long` sends `"cacheRetention":"long"`; unset sends no
      `cacheRetention` key at all (**not** `"short"`); an explicit
      `CacheRetention` on the options wins over both.
- [ ] `TestStreamCancelledContextEndsAborted` — cancelling the context mid-stream
      resolves with `StopReason == ai.StopReasonAborted` and no diagnostic.
- [ ] `TestStreamSimpleForwardsOptionsWithoutClamping` — `StreamSimple` with a
      `MaxTokens` above the model's `MaxTokens`/context window sends it
      unchanged, and forwards `Reasoning` as the payload's `reasoning` — the
      deliberate difference from the other adapters.
- [ ] `TestPiMessagesToolChoiceFunctionPinsOneFunction` —
      `PiMessagesToolChoiceFunction: "read"` sends
      `"toolChoice":{"type":"function","function":{"name":"read"}}` and
      overrides `PiMessagesToolChoice`.
- [ ] Every test is offline, stdlib-only (`testing` + `net/http/httptest`), has
      no `t.Parallel()` and no build tag, and is a discrete named function.
- [ ] `// Ports: packages/ai/src/api/pi-messages.ts` on all three new files
      (`errors.go`, `stream.go`, and `stream_test.go` — it is the port of
      `test/pi-messages.test.ts`'s transport and error cases).
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(apis): stream the pi-messages protocol over net/http`.

## Relevant files / areas

- `ai/apis/pimessages/` — the package from
  [issue 01](/epic-8-pi-messages-and-radius/issues/01-pimessages-wire-and-converter.md).
- `ai/options.go:75-135` `StreamOptions` and its flat-merge comment,
  `:224-232` the mistral block to copy the shape from, `:303`
  `EffectiveCacheRetention` (**not** to be used here), `:321-327`
  `SimpleStreamOptions`, `:41-45` `ProviderResponse`.
- `ai/headers.go:19` `ProviderHeadersToRecord`, `:38` `MergeProviderHeaders`.
- `ai/stream.go:32` `NewStream`, `:41` `Push`, `:60` `End`; `ai/lazy.go`
  `LazyStream` for setup errors surfacing in-band.
- `ai/diagnostics.go:44` `ExtractDiagnosticError` and its `Code() string`
  reflection, `:55` `NewAssistantMessageDiagnostic`.
- `ai/apis/anthropic/anthropic.go:260-300` — the request/pump shape to follow;
  `:883` the `providerEnvValue` private helper every adapter copies.
- `ai/apis/simpleopts.go:20-47` — deliberately **not** used by this adapter's
  `StreamSimple`.
- `ai/json.go` — the codecs that make `json.Marshal(chat)` the wire form.
- Upstream at `936aff00`: `src/api/pi-messages.ts` (`stream`, `streamSimple`,
  `createPiMessagesResponseError`, `formatPiMessagesResponseError`,
  `resolveCacheRetention`, `truncateDiagnosticString`),
  `test/pi-messages.test.ts` (248).

## Dependencies

- **Blocked by**: [Issue 01](/epic-8-pi-messages-and-radius/issues/01-pimessages-wire-and-converter.md).
  Also lands after [Epic 2](/epic-2-core-types-and-models-contracts/EPIC_2.md)
  [issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md),
  which restructures `StreamOptions` into `ProviderRequestOptions` + the
  streaming fields — adding this block first guarantees a conflict in the same
  region of `ai/options.go`.
- **Blocks**: [Issue 04](/epic-8-pi-messages-and-radius/issues/04-radius-provider-binding.md),
  [issue 05](/epic-8-pi-messages-and-radius/issues/05-porting-and-docs.md).

## PR size note

`L` — ~600 changed lines: the transport half of the 433-line
`src/api/pi-messages.ts` across `errors.go` and `stream.go`, the transport and
error cases of the 248-line `test/pi-messages.test.ts`, 13 named tests, and the
four-field pi-messages block on `ai.StreamOptions`. `L` is the ceiling, not the
target: the error type only proves itself through the stream that raises it, and
if it trends past ~1000 the honest cut is `errors.go` plus its diagnostic tests
first, the request and read loop second.
