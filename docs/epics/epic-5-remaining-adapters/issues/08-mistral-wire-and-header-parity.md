---
type: Issue
title: "mistral: request-shape and header parity with the SDK-free upstream client"
description: "Send prefix and tool-call index on assistant replay, make x-affinity suppression case-insensitive across model and request headers, apply a default request timeout, and stop assuming streamed content parts are non-null."
tags: [epic-5]
timestamp: 2026-08-11T20:00:00Z
epic: 5
issue: 08
slug: mistral-wire-and-header-parity
size: M
status: open
gh_issue: 166
resource: https://github.com/kern-ia/kern-link/issues/166
depends_on: ["07"]
---

# mistral: request-shape and header parity with the SDK-free upstream client

## Summary

When upstream dropped `@mistralai/mistralai` it had to spell out, by hand, every
field the SDK had been serializing for it — and in doing so it pinned down four
behaviors that were previously implicit. kern-link has always talked raw HTTP to
Mistral, so it never inherited them. This PR closes that gap.

1. **Assistant replay gains two fields.** `toChatMessages` now emits
   `{role: "assistant", prefix: false}` on every assistant message, and every
   replayed tool call carries `index: 0`:

   ```ts
   toolCalls.push({ id: block.id, type: "function", function: { name, arguments }, index: 0 });
   const assistantMessage: MistralChatMessage = { role: "assistant", prefix: false };
   ```

   Both are literal constants upstream, not computed — `index` is `0` even for
   the second tool call in a message. Port them as written; if that looks wrong,
   it is upstream's wire contract and "what does upstream do" outranks what
   looks right ([CONVENTIONS.md](../../../planning/CONVENTIONS.md)).

2. **`x-affinity` suppression is case-insensitive and checks the *overrides*,
   not the merged map.**

   ```ts
   const hasExplicitAffinity =
       hasMistralHeaderOverride(model.headers, "x-affinity") || hasMistralHeaderOverride(options?.headers, "x-affinity");
   if (shouldUsePromptCaching(options) && !hasExplicitAffinity) headers.set("x-affinity", options.sessionId);
   ```

   `hasMistralHeaderOverride` lowercases each key before comparing. kern-link
   checks `merged["x-affinity"]` with an exact-key lookup
   (`mistral.go:299`), so a caller who sets `X-Affinity` today gets the
   auto-injected header **as well** — two affinity headers, and the wrong one may
   win. Upstream also builds on a `Headers` object, whose `set` is
   case-insensitive, so `model.headers` and `options.headers` collapse onto one
   another by name; kern-link's `map[string]string` does not.

3. **A default request timeout.** Upstream applies
   `AbortSignal.timeout(options?.timeoutMs ?? 60_000)` for the whole request,
   including the SSE read. `httpretry` takes `Opts.Timeout` and applies it only
   until response headers arrive (`httpretry.go:172`, deliberate and
   documented), and there is no default when `Timeout` is zero. **Apply a 60s
   default in the Mistral adapter** — decided in
   [EPIC_5.md](/epic-5-remaining-adapters/EPIC_5.md)'s `## Notes` — so a hung
   Mistral SSE connection no longer blocks forever, which is the failure
   upstream's timeout exists to bound. If `httpretry`'s header-only timeout
   scope means the SSE-read portion needs its own bound to match upstream's
   whole-request semantics, add it here rather than silently covering only the
   header wait.

4. **Streamed content parts are nullable.** Upstream added `?? []` / `?? ""`
   around `item.thinking` and `item.text` when it stopped trusting SDK types.
   Go's zero values already make a missing field harmless — verify that
   `decodeContentItems` (`stream.go:240`) does not panic or emit a spurious
   empty delta on `{"type":"thinking"}` with no `thinking` array, and lock it
   with a test. If it is already safe, the test is the deliverable.

Also in this PR: **error shape**. Upstream's `MistralHttpError` carries
`statusCode` and the raw body, and `formatMistralError` renders
`Mistral API error (<status>): <body>`. kern-link's `statusError`
(`mistral.go:203`) renders `"<status> <compacted-body>"` instead, with a doc
comment saying the SDK-shape probing has no raw-HTTP equivalent — **that
rationale is now stale**, because upstream is raw HTTP too. Re-check whether the
Go text should move to upstream's, remembering that `ai/retry.go` and
`ai/overflow.go` match against it: if it changes, the classifier patterns must
be checked in the same PR, and if it stays, `docs/PORTING.md` gets a deviation
entry that replaces the stale reason.

## Scope

- `ai/apis/mistral/messages.go`:
  - `wireMessage` (`:21`) — add `Prefix *bool \`json:"prefix,omitempty"\`` or a
    plain `bool` with `json:"prefix"`; upstream always emits `prefix: false` on
    assistant messages and never on the others, so the encoding must be able to
    say "false, present" for assistants and "absent" elsewhere.
  - `wireToolCall` (`:29`) — add `Index int \`json:"index"\``.
  - `assistantMessage` (`:170`) — set both.
- `ai/apis/mistral/mistral.go`:
  - `buildHeaders` (`:280`) — case-insensitive `x-affinity` detection over
    `model.Headers` and `opts.Headers` *before* merging, and case-insensitive
    override of `model.Headers` against the defaults (upstream's `Headers.set`
    semantics). `ai.MergeProviderHeaders` (`ai/headers.go:38`) already has
    null-deletion for `opts.Headers`; check whether Epic 2
    [issue 09](/epic-2-core-types-and-models-contracts/issues/09-models-request-transforms.md)'s
    case-insensitive header merging supersedes part of this — if it does, use it
    rather than writing a second implementation.
  - `run` (`:98`) — apply the 60s default timeout from point 3.
- `ai/apis/mistral/stream.go` — `decodeContentItems` (`:240`) nil-safety
  verification.
- Tests, offline `httptest`, in `ai/apis/mistral/transport_test.go`.
- `docs/PORTING.md` — refresh the `mistral-conversations.ts` row: upstream is no
  longer SDK-based, so "ported (raw REST; no Go Mistral SDK)" now understates
  the alignment. Record whichever error-text deviation survives point 4 below;
  the 60s timeout is a straight port, not a deviation.
- `// Ports:` headers stay accurate — in particular `statusError`'s comment if
  its rationale changes.

## Out of scope

- Stop reasons and strict tools — [issue 07](/epic-5-remaining-adapters/issues/07-mistral-stop-reasons-and-strict-tools.md),
  which lands first.
- Porting `toMistralWirePayload`'s camelCase→snake_case remapping table as code.
  It exists because upstream's payload builder still speaks the SDK's camelCase
  names internally; kern-link's `wireRequest` (`mistral.go:218`) declares
  snake_case struct tags directly. **Use the table as a checklist** — confirm
  every one of its 13 payload keys, 2 message keys and 6 content-chunk keys is
  either already correct in the Go structs or genuinely unused — and report the
  result in the PR body. Do not reimplement the remapping.
- Rewriting SSE framing. `findMistralEventBoundary`'s eight-way boundary regex
  is upstream's hand-rolled parser catching up to what `ai/internal/sse` already
  does; confirm `ai/internal/sse` handles `\r\n\r\n`, `\r\r`, `\n\n` and the
  mixed forms, and say so.

## Acceptance criteria / Definition of done

- [ ] `TestAssistantReplaySendsPrefixFalse` — a replayed assistant message
      serializes with `"prefix": false`; user, system and tool messages carry no
      `prefix` key.
- [ ] `TestAssistantToolCallsCarryIndex` — every replayed tool call serializes
      with `"index": 0`, including the second one in the same message.
- [ ] `TestExplicitAffinityHeaderSuppressesAutoAffinity` — with prompt caching
      active and `opts.Headers["X-Affinity"]` set, the request carries exactly
      one affinity header and it is the caller's value; same for
      `model.Headers["X-AFFINITY"]`.
- [ ] `TestAutoAffinityStillAppliedWithoutOverride` — the existing behavior is
      unchanged when no caller header is present.
- [ ] `TestModelHeaderOverridesDefaultCaseInsensitively` — a
      `model.Headers["Content-Type"]` replaces the default `content-type`
      rather than adding a second entry.
- [ ] `TestRequestTimeoutBoundsAStalledStream` — an `httptest` server that
      accepts the request and then never writes ends the stream with an error
      rather than hanging, within the 60s default.
- [ ] `TestStreamThinkingPartWithoutTextIsHarmless` — an SSE chunk with
      `{"type":"thinking"}` and no `thinking` array, and one with
      `{"type":"text"}` and no `text`, produce no panic and no spurious delta
      event.
- [ ] Ported upstream coverage: the applicable cases of
      `test/mistral-http-transport.test.ts` (new, 427 lines) come across —
      "serializes assistant thinking, tool calls, and tool results for replay",
      "honors case-insensitive header overrides and explicit affinity
      suppression", "applies the request timeout while waiting for an SSE
      chunk", "aborts while waiting for an SSE chunk", "parses SSE and UTF-8
      sequences split across transport chunks", "preserves HTTP status and
      response bodies in errors". The two that only exercise upstream's
      camelCase→snake_case remapping ("serializes SDK-style payloads to the
      Mistral wire format") have no Go analogue — list them in the PR body as
      deliberately not ported, with the reason.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `fix(apis): honor caller-supplied x-affinity headers in the mistral adapter`.

## Relevant files / areas

- `ai/apis/mistral/mistral.go` — `:98` `run`, `:146` the request URL and
  `httpretry.Do` call, `:203` `statusError`, `:218` `wireRequest`, `:271`
  `shouldUsePromptCaching`, `:280` `buildHeaders`, `:299` the exact-key
  `x-affinity` check.
- `ai/apis/mistral/messages.go` — `:21` `wireMessage`, `:29` `wireToolCall`,
  `:170` `assistantMessage`.
- `ai/apis/mistral/stream.go:240` — `decodeContentItems`.
- `ai/headers.go:38` — `ai.MergeProviderHeaders`.
- `ai/apis/internal/httpretry/httpretry.go:172` — `doWithHeaderTimeout` and its
  documented header-only timeout scope.
- `ai/internal/sse` — the shared SSE framer.
- `ai/retry.go`, `ai/overflow.go` — classifiers matching the error text.
- Upstream: `src/api/mistral-conversations.ts` at `936aff00` —
  `requestMistralStream`, `buildMistralHeaders`, `hasMistralHeaderOverride`,
  `toMistralWirePayload`, `toChatMessages`, `MistralHttpError`,
  `readMistralEvents`.
- Upstream test: `test/mistral-http-transport.test.ts` (new, 427 lines).

## Dependencies

- **Blocked by**: [Issue 07](/epic-5-remaining-adapters/issues/07-mistral-stop-reasons-and-strict-tools.md)
  (same three files).
- **Blocks**: None.

## PR size note

`M` — ~450 changed lines: four wire and header behaviors plus the `statusError`
text re-check, spread over `mistral.go`, `messages.go` and `stream.go`, with
seven named tests and six ported cases from the new 427-line
`mistral-http-transport.test.ts`. This sits at the top of M rather than in L
because two of the four behaviors are verification-only. Split past ~500, and
the seam is the message-shape half (`prefix`, tool-call `index`) versus the
transport half (case-insensitive `x-affinity`, the 60s timeout, nil-safety).
