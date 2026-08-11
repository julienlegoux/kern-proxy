---
type: Issue
title: "Dispatch deferred responses through ProviderStreams, Provider, and Models"
description: "Add the optional fetchDeferred/cancelDeferred capability across the three dispatch layers, with capability detection in CreateProvider and the upstream-verbatim unsupported-provider errors."
tags: [epic-2]
timestamp: 2026-08-11T20:00:00Z
epic: 2
issue: 10
slug: deferred-response-dispatch
size: M
status: open
gh_issue: 138
resource: https://github.com/kern-ia/kern-link/issues/138
depends_on: ["01", "02", "05", "08", "09", "13"]
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

- `ai/options.go` — **`SimpleStreamOptions.Deferred`, the request-side knob.**
  Upstream declares `deferred?: boolean | { window?: "15m" | "1h" | "24h" }` on
  `SimpleStreamOptions` (`packages/ai/src/types.ts:307` at `936aff00`). Without
  it a caller can *fetch* and *cancel* a deferred response but has no supported
  way to *ask* for one — the asymmetry
  [decision 11](../../../planning/scope/11-deferred-tools.md) exists to prevent,
  which is why it belongs in this PR rather than in an adapter epic. Go form:
  `Deferred *DeferredRequest` on `ai.SimpleStreamOptions` (`ai/options.go:321-327`)
  with `type DeferredRequest struct { Window string }` — nil means unset, a
  non-nil zero value is upstream's bare `true`, and `Window` carries
  `"15m"`/`"1h"`/`"24h"` (empty = provider default). That collapses upstream's
  union losslessly; record the collapse in the field's doc comment.
  `apis.BuildBaseOptions` (`ai/apis/simpleopts.go:31`) returns a `StreamOptions`
  and therefore cannot carry it — `Deferred` stays on the simple layer, read by
  `StreamSimple` implementations, exactly as upstream's only reader
  (`providers/faux.ts`) reads it off `SimpleStreamOptions`.
- `ai/options.go` — `DeferredFetchOptions` / `DeferredCancelOptions` already
  exist from
  [issue 13](/epic-2-core-types-and-models-contracts/issues/13-fetch-sampling-and-deferred-options.md);
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

- **Any adapter implementing the capability — in this PR or anywhere in this
  program.** No `ai/apis/*` package gains `FetchDeferred`, and no later epic
  adds one: `git grep -rniE 'FetchDeferred|CancelDeferred|deferred response'
  docs/epics/` matches nothing outside this epic. That is deliberate, not an
  omission, and it is parity rather than a shortfall — **upstream has no adapter
  implementation either.** At `936aff00` the only things under `packages/ai/src`
  that carry `fetchDeferred`/`cancelDeferred` are `types.ts` (the optional
  interface members), `models.ts` (the dispatch this issue ports),
  `api/lazy.ts` (a shim that forwards) and `providers/faux.ts`. So the
  capability ships **proved by `faux` only**
  ([issue 12](/epic-2-core-types-and-models-contracts/issues/12-faux-deferred-responses.md)),
  which is exactly upstream's own coverage. Epics 4 and 5 cover deferred
  *tools*, a different capability distinguished in the next bullet; they were
  never a home for this one. `EPIC_2.md`'s `## Out of scope` records the same
  gap at epic level. When a provider ships a real deferred-response API, the
  adapter implementation is new work with a new issue, not a debt this program
  left behind.
- **`splitDeferredTools`** — a different capability that shares the word
  "deferred": [issue 11](/epic-2-core-types-and-models-contracts/issues/11-deferred-tools-split.md).
- `lazyApi`'s `LazyApiCapabilities` — not ported (bundler shim), see
  [issue 06](/epic-2-core-types-and-models-contracts/issues/06-simple-options-and-lazy.md).
- Adapter-side *behavior* for `SimpleStreamOptions.Deferred`. The field itself
  is in `## Scope` above; what a real provider does with it is out of range
  here, and `faux` is what proves the round trip
  ([issue 12](/epic-2-core-types-and-models-contracts/issues/12-faux-deferred-responses.md)).

## Acceptance criteria / Definition of done

- [ ] `TestSimpleStreamOptionsDeferredRoundTrips` — `&ai.SimpleStreamOptions{Deferred: &ai.DeferredRequest{Window: "1h"}}`
      reaches a `StreamSimple` implementation with both the non-nil handle and
      the window intact; a nil `Deferred` arrives nil; and
      `apis.BuildBaseOptions` on the same value returns a `StreamOptions`
      unaffected by it (the field is deliberately simple-layer only).
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

- `ai/options.go:319-327` `SimpleStreamOptions` (which gains `Deferred`),
  `:329-343` `StreamFunc`, `SimpleStreamFunc`, `ProviderStreams`,
  `StreamFuncs` adapter (`ai/provider.go:476-489`).
- `ai/apis/simpleopts.go:31` `BuildBaseOptions` — the `SimpleStreamOptions` →
  `StreamOptions` narrowing that `Deferred` deliberately does not cross.
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
  [Issue 08](/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md),
  [Issue 09](/epic-2-core-types-and-models-contracts/issues/09-models-request-transforms.md)
  — the `Models`-level deferred option variants carry `ModelsRequestTransforms`
  in the shape issue 09 settles on — and
  [Issue 13](/epic-2-core-types-and-models-contracts/issues/13-fetch-sampling-and-deferred-options.md),
  which declares `DeferredFetchOptions` / `DeferredCancelOptions`.
- **Blocks**: [Issue 12](/epic-2-core-types-and-models-contracts/issues/12-faux-deferred-responses.md).

## PR size note

`M` — ~450 changed lines, near the top of the band: the capability threads
through three dispatch layers (`ProviderStreams`, `Provider`, `modelsImpl`),
`SimpleStreamOptions` gains `Deferred`, `CreateProvider` grows capability
detection, four upstream-verbatim error strings are copied, and six named tests
plus a `docs/PORTING.md` row follow. Split past ~500, and the seam is
`SimpleStreamOptions.Deferred` — the request-side knob is separable from the
fetch/cancel dispatch.
