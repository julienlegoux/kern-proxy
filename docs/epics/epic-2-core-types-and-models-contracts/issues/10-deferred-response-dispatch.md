---
type: Issue
title: "Dispatch deferred responses through ProviderStreams, Provider, and Models"
description: "Add the optional fetchDeferred/cancelDeferred capability across the three dispatch layers, with capability detection in CreateProvider and the upstream-verbatim unsupported-provider errors."
tags: [epic-2]
timestamp: 2026-08-09T04:31:17Z
epic: 2
issue: 10
slug: deferred-response-dispatch
size: M
status: open
gh_issue: 138
resource: https://github.com/kern-ia/kern-link/issues/138
depends_on: [1, 2, 5, 8]
---

# Dispatch deferred responses through ProviderStreams, Provider, and Models

## Summary

Deferred responses are the capability the new `"pending"` and `"deferred"` stop
reasons exist *for*
([decision 11](../../../planning/scope/11-deferred-tools.md)): a provider returns
a durable `DeferredHandle` instead of a completed message, and the caller fetches
or cancels it later. Upstream threads an optional capability through all three
dispatch layers:

```ts
interface ProviderStreams {   // and Provider, with the same two members
	fetchDeferred?(model, handle: DeferredHandle, options?: DeferredFetchOptions): AssistantMessageEventStream;
	cancelDeferred?(model, handle: DeferredHandle, options?: DeferredCancelOptions): Promise<void>;
}
interface Models {            // non-optional here — it errors when unsupported
	fetchDeferred(model, handle, options?: ModelsDeferredFetchOptions): Promise<AssistantMessage>;
	cancelDeferred(model, handle, options?: ModelsDeferredCancelOptions): Promise<void>;
}
```

`createProvider` advertises the capability only when at least one of its API
implementations has it, and per-model dispatch errors when the specific
`model.api` does not.

Shipping the enum values without this would leave `StopReason` cases the library
can never emit — the outcome
[decision 11](../../../planning/scope/11-deferred-tools.md) explicitly rejects.

## Scope

- `ai/options.go` — `DeferredFetchOptions` / `DeferredCancelOptions` already
  exist from
  [issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md);
  add the `Models`-level variants carrying `ModelsRequestTransforms` in whatever
  shape [issue 09](/epic-2-core-types-and-models-contracts/issues/09-models-request-transforms.md)
  settled on.
- `ai/options.go:339-343` `ProviderStreams` — Go interfaces have no optional
  members, so the capability is a **second, narrow interface** an implementation
  may also satisfy (e.g. `DeferredStreams` with `FetchDeferred`/`CancelDeferred`),
  detected with a type assertion. Document that mapping in the interface's doc
  comment and in `docs/PORTING.md`; it is the honest Go equivalent of an optional
  method and the same trick `CanRefreshModels` already stands in for.
- `ai/provider.go` — `Provider` gains the same capability pair plus the
  reporting method Go needs (`CanFetchDeferred()` / `CanCancelDeferred()`, in
  the style of `CanRefreshModels()` at `:37`).
- `CreateProvider` (`ai/provider.go:382`) — advertise the capability when any
  entry in `Api`/`ApiByProtocol` supports it; per-request, resolve the
  implementation for `model.Api` and fail with the upstream-verbatim messages:
  - `Provider %s does not support deferred responses for "%s"` (provider id, model api)
  - `Provider %s cannot cancel deferred responses for "%s"`
- `modelsImpl` — `FetchDeferred` runs through `LazyStream` + `applyAuth` and
  resolves to an `*AssistantMessage`; `CancelDeferred` applies auth and
  delegates. Unsupported providers fail with
  `Provider %s does not support deferred responses` as a
  `*ModelsError` with code `provider`. All four strings are copied verbatim —
  error text is load-bearing ([CONVENTIONS.md](../../../planning/CONVENTIONS.md),
  ST1005).

## Out of scope

- **Any adapter implementing the capability.** No `ai/apis/*` package gains
  `FetchDeferred` in this PR; the real implementations (OpenAI Responses,
  Anthropic) belong to epics 4 and 5, and the `faux` proof to
  [issue 12](/epic-2-core-types-and-models-contracts/issues/12-faux-deferred-responses.md).
- **`splitDeferredTools`** — a different capability that shares the word
  "deferred": [issue 11](/epic-2-core-types-and-models-contracts/issues/11-deferred-tools-split.md).
- `lazyApi`'s `LazyApiCapabilities` — not ported (bundler shim), see
  [issue 06](/epic-2-core-types-and-models-contracts/issues/06-simple-options-and-lazy.md).
- `SimpleStreamOptions.deferred` (`boolean | { window?: "15m"|"1h"|"24h" }`) —
  the *request-side* knob that asks a provider to defer. It has no consumer
  until an adapter implements one; port the field here if it costs nothing, but
  it is the adapters' epics that give it meaning. Say which you did in the PR
  body.

## Acceptance criteria / Definition of done

- [ ] `TestProviderAdvertisesDeferredCapability` — a provider built from an API
      implementation supporting deferred responses reports the capability; one
      built from a plain implementation does not.
- [ ] `TestFetchDeferredUnsupportedApiErrors` — for a provider using
      `ApiByProtocol` where only one protocol supports deferral, requesting a
      model on the other protocol yields exactly
      `Provider <id> does not support deferred responses for "<api>"`.
- [ ] `TestModelsFetchDeferredUnsupportedProviderErrors` — the `Models`-level
      error is a `*ModelsError` with `Code() == "provider"` and the verbatim
      message.
- [ ] `TestModelsFetchDeferredAppliesAuth` — the resolved API key and headers
      reach the implementation, through the same `applyAuth` path as `Stream`.
- [ ] `TestModelsFetchDeferredFailsInBand` — a setup failure surfaces as an
      error event through `LazyStream`, never as a panic, matching the
      "failures never escape as panics" rule.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2).
- [ ] `gofmt -l .` prints nothing; `docs/PORTING.md` records the
      optional-method → second-interface mapping.
- [ ] Conventional Commit, e.g.
      `feat(ai): dispatch deferred responses through Provider and Models`.

## Relevant files / areas

- `ai/options.go:329-343` — `StreamFunc`, `SimpleStreamFunc`, `ProviderStreams`,
  `StreamFuncs` adapter (`ai/provider.go:476-489`).
- `ai/provider.go:15-42` (`Provider`), `:47-76` (`Models`), `:346-367`
  (`CreateProviderOptions`), `:442-474` (`apiFor`, `dispatch`).
- `ai/lazy.go:28` `LazyStream` — the in-band failure seam.
- `ai/resolve.go:26-46` — `ModelsError` and its codes (`provider` already
  exists).
- `ai/types.go` — `DeferredHandle` from
  [issue 02](/epic-2-core-types-and-models-contracts/issues/02-message-model-deferred-fields.md).
- Upstream: `src/models.ts` (`fetchDeferred`, `cancelDeferred`, the
  `createProvider` capability block) and `src/types.ts` (`ProviderStreams`) at
  `936aff00`.

## Dependencies

- **Blocked by**: [Issue 01](/epic-2-core-types-and-models-contracts/issues/01-widen-stopreason-and-thinkinglevel.md),
  [Issue 02](/epic-2-core-types-and-models-contracts/issues/02-message-model-deferred-fields.md),
  [Issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md),
  [Issue 08](/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md).
- **Blocks**: [Issue 12](/epic-2-core-types-and-models-contracts/issues/12-faux-deferred-responses.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
