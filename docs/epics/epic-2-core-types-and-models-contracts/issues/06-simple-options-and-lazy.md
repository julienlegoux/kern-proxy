---
type: Issue
title: "Update simple-options and lazy to 0.84.1"
description: "Carry the new request knobs through BuildBaseOptions, export the min-answer-token constant, and make LazyStream forward the inner stream's result."
tags: [epic-2]
timestamp: 2026-08-11T13:10:00Z
epic: 2
issue: 06
slug: simple-options-and-lazy
size: S
status: open
gh_issue: 134
resource: https://github.com/kern-ia/kern-link/issues/134
depends_on: [1, 5]
---

# Update simple-options and lazy to 0.84.1

## Summary

`src/api/simple-options.ts` and `src/api/lazy.ts` are small, but they sit under
every adapter — which is why
[decision 04](../../../planning/scope/04-core-types-migration.md) puts them in
this epic rather than with the adapters. Both changed at `936aff00`:

- `resolveSimpleOptions` now merges `samplingParams` (model defaults under
  per-request overrides) and forwards `fetch` (and `telemetryContext`, which
  kern-link does not port — see
  [issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md)).
- `minOutputTokens` became the exported `MIN_ANSWER_TOKENS = 1024`, documented
  as "tokens always left for the answer when a thinking budget shares the
  response ceiling".
- `lazyStream`'s forwarder now ends the outer stream **with the inner stream's
  result** instead of with nothing.

That last one is the only behavior change of the three, and it is the reason
this is not a pure-mechanics PR: kern-link's `LazyStream` today ends with
`outer.End(nil)` and relies on the inner stream's terminal event having already
completed `outer`. Upstream now propagates the result explicitly.

## Scope

- `ai/apis/simpleopts.go:31` `BuildBaseOptions` — carry `Fetch` through, and
  merge `SamplingParams` as `{...model.SamplingParams, ...options.SamplingParams}`
  (per-request keys win). Produce a fresh map; never mutate `Model.SamplingParams`,
  which is shared catalog state.
- `ai/apis/simpleopts.go:15` — promote `minOutputTokens` to an exported
  `MinAnswerTokens = 1024` with upstream's doc sentence, and use it in
  `AdjustMaxTokensForThinking` (`:70`).
- `ai/lazy.go:28-51` `LazyStream` — end the outer stream with the inner
  stream's result when the inner stream produced one, matching upstream's
  `target.end(hasResult(source) ? await source.result() : undefined)`. The
  existing comment block at `ai/lazy.go:37-46` explains why the pump uses
  `context.Background()`; keep it and extend it rather than replacing it.
- `ai/apis/simpleopts.go:49` `ClampReasoning` is handled by
  [issue 01](/epic-2-core-types-and-models-contracts/issues/01-widen-stopreason-and-thinkinglevel.md);
  do not touch it here.

## Out of scope

- **`lazyApi` and its new `LazyApiCapabilities`.** Upstream's `lazyApi` is a
  bundler code-splitting shim; `docs/PORTING.md` already records it as not
  ported ("every adapter package is imported directly"). The deferred
  capabilities it now advertises are handled natively in
  [issue 10](/epic-2-core-types-and-models-contracts/issues/10-deferred-response-dispatch.md).
  Confirm the non-port still holds and leave the `docs/PORTING.md` row alone if
  it does.
- Adapter-side use of `SamplingParams` on the wire — epics 4 and 5. Only
  OpenAI-compatible adapters apply it upstream.
- Any change to `NewSetupErrorMessage` (`ai/lazy.go:11`).

## Acceptance criteria / Definition of done

- [ ] `TestBuildBaseOptionsMergesSamplingParams` — the sole assertion of
      `SamplingParams` merge precedence in this epic: request-level keys
      override model-level keys, and the model's own map is not mutated
      (assert the model map after the call).
- [ ] `TestBuildBaseOptionsCarriesFetch` — a non-nil `Fetch` on
      `SimpleStreamOptions` reaches the returned `StreamOptions`.
- [ ] `ai.MinAnswerTokens` is exported and used by `AdjustMaxTokensForThinking`;
      `grep -c "1024" ai/apis/simpleopts.go` shows the literal only where the
      constant and the per-level budget table need it.
- [ ] `TestLazyStreamForwardsInnerResult` — a `LazyStream` whose inner stream
      ends with a result resolves `Result(ctx)` to that message; the pre-existing
      behavior for inner streams that terminate via a done/error event stays
      green (`ai/stream_test.go`, `ai/apis/simpleopts_test.go`).
- [ ] `TestLazyStreamSetupErrorStillEmitsErrorEvent` — the setup-failure path
      (`ai/lazy.go:33`) is unchanged.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). The race detector matters here — `LazyStream` owns a goroutine
      and its verdict only ever arrives from CI.
- [ ] `gofmt -l .` prints nothing; both files keep their `// Ports:` headers.
- [ ] Conventional Commit, e.g.
      `feat(apis): carry samplingParams and fetch through simple options`.

## Relevant files / areas

- `ai/apis/simpleopts.go` — `:15` `minOutputTokens`, `:20`
  `ClampMaxTokensToContext`, `:31` `BuildBaseOptions`, `:49` `ClampReasoning`,
  `:70` `AdjustMaxTokensForThinking`.
- `ai/apis/simpleopts_test.go` — existing coverage to extend.
- `ai/lazy.go` (52 lines) — `NewSetupErrorMessage`, `LazyStream`.
- `ai/stream.go:60` `Stream.End(result *AssistantMessage)` — the seam the lazy
  change writes through; `:154` `Result(ctx)`.
- Upstream: `src/api/simple-options.ts` and `src/api/lazy.ts` at `936aff00`.

## Dependencies

- **Blocked by**: [Issue 01](/epic-2-core-types-and-models-contracts/issues/01-widen-stopreason-and-thinkinglevel.md)
  (`ClampReasoning` edits the same function region) and
  [Issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md)
  (`Fetch` and `SamplingParams` must exist first).
- **Blocks**: Nothing inside this epic.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR. Expected here: well under 200.
