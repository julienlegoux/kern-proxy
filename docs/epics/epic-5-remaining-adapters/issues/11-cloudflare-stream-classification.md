---
type: Issue
title: "cloudflare-stream: classify upstream's dispatch-time base-URL resolution and give it a Go home"
description: "Upstream moved Cloudflare account/gateway placeholder resolution out of auth and into a per-request stream wrapper; decide whether kern-link follows, and record the disposition in PORTING.md."
tags: [epic-5]
timestamp: 2026-08-09T09:45:42Z
epic: 5
issue: 11
slug: cloudflare-stream-classification
size: S
status: open
gh_issue: 169
resource: https://github.com/kern-ia/kern-link/issues/169
depends_on: []
---

# cloudflare-stream: classify upstream's dispatch-time base-URL resolution and give it a Go home

## Summary

[EPIC_5](/epic-5-remaining-adapters/EPIC_5.md) asks for "a one-line
classification" of `cloudflare-stream.ts`. Reading the file makes the case
bigger than one line — and settles two things the scope guessed at:

- **It is `src/providers/cloudflare-stream.ts`, not `src/api/`.** `src/api/`
  holds `cloudflare.ts`, which is 15 lines of base-URL template constants and is
  already dispositioned (`docs/PORTING.md:45`).
- **It is new in range**, and it is one half of a *move*. Upstream deleted
  `resolveCloudflareBaseUrl` from `providers/cloudflare-auth.ts` (which no
  longer returns a `baseUrl` at all) and re-created it in
  `providers/cloudflare-stream.ts`, where `cloudflareStreams(streams)` wraps a
  `ProviderStreams` and materializes `{CLOUDFLARE_ACCOUNT_ID}` /
  `{CLOUDFLARE_GATEWAY_ID}` from `options.env` **at dispatch time**, once per
  request, leaving placeholders intact when the env does not resolve them.

kern-link resolves the same placeholders at **auth** time:
`ai/providers/cloudflare_auth.go:24` `resolveCloudflareBaseURL` is called from
both binding resolvers and returned as `ai.ModelAuth.BaseURL`. So the capability
is present; the *timing* differs, and the observable consequence is narrow: a
model dispatched with a `ProviderEnv` that did not come through the Cloudflare
auth resolver keeps its placeholders in kern-link and gets them substituted
upstream.

The deliverable is a decision, its implementation, and a `docs/PORTING.md` row —
because leaving this file in the "acknowledged but undispositioned" third state
is exactly what [decision 02](../../../planning/scope/02-parity-bar.md)'s
disposition gate exists to eliminate, and what
[decision 16](../../../planning/scope/16-copilot-headers-gap.md) just finished
paying off for the Copilot headers.

**Recommended**: port the dispatch-time resolution as well. It is idempotent
(a fully-resolved URL contains no placeholders, so a second pass is a no-op), it
is ~20 lines, and it makes the Go behavior a superset of both. Keep the
auth-time resolution — removing it would change what `AuthResult.Auth.BaseURL`
reports, which is kern-link's own tested contract. If the implementer instead
concludes the auth-time resolution is sufficient, that is a defensible
deviation — but it must be **written down with that reason**, not left implicit.

## Scope

- Decide, and implement one of:
  - **(recommended) Port it.** Add placeholder resolution on the request path for
    the two Cloudflare bindings, reading `{CLOUDFLARE_ACCOUNT_ID}` /
    `{CLOUDFLARE_GATEWAY_ID}` from the resolved `ProviderEnv`, leaving the URL
    untouched when neither placeholder resolves. Home it beside the existing
    `resolveCloudflareBaseURL` in `ai/providers/cloudflare_auth.go` — reusing
    that function, not writing a second substitution — or in a sibling
    `cloudflare_stream.go` if the wiring needs its own file. `cloudflareStreams`
    wraps `ProviderStreams`; check how kern-link's `ai.Provider` exposes
    `Stream`/`StreamSimple` before deciding whether a wrapper type or a call in
    each binding is the smaller change.
  - **Record it as a deviation.** State that kern-link resolves at auth time and
    why that is structurally sufficient here, naming the case it does not cover.
- **`docs/PORTING.md`** gets a row for `src/providers/cloudflare-stream.ts`
  either way, and the existing `src/api/cloudflare.ts` row (`:45`) is checked
  for staleness — it currently says the templates are "resolved at runtime by
  `resolveCloudflareBaseURL`", which stays true under either outcome but should
  name the new file if one appears.
- `// Ports:` header on whatever new code lands.

## Out of scope

- `providers/cloudflare-auth.ts`'s other changes in range (+68/-35): the
  per-field credential/env merge ("a credential carrying only the API key must
  still pick up the account / gateway id from the environment"), the
  `AbortSignal` threading, and the removed `login` prompts. Those are provider
  **auth** changes. No epic in this program currently claims
  `src/providers/*.ts` updates beyond
  [Epic 6](/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md)'s four new
  bindings — flag it in the PR body so
  [Epic 9](/epic-9-classifier-audit-and-release/EPIC_9.md)'s disposition sweep
  catches it rather than losing it. **Do not fold it in here**; it is a separate
  behavior with its own tests.
- The Cloudflare compat detection in `ai/apis/openaicompletions/compat.go:57-58`
  — unrelated and unchanged.

## Acceptance criteria / Definition of done

- [ ] `docs/PORTING.md` contains a row for `src/providers/cloudflare-stream.ts`
      with a disposition of `ported` or a named deviation, and no upstream file
      under `src/providers/cloudflare*` is left undispositioned.
- [ ] If ported — `TestResolvesCloudflarePlaceholdersAtDispatch`: a model whose
      `BaseURL` is
      `https://gateway.ai.cloudflare.com/v1/{CLOUDFLARE_ACCOUNT_ID}/{CLOUDFLARE_GATEWAY_ID}/openai`
      dispatched with `Env{CLOUDFLARE_ACCOUNT_ID: "account",
      CLOUDFLARE_GATEWAY_ID: "gateway"}` reaches the adapter with
      `https://gateway.ai.cloudflare.com/v1/account/gateway/openai`.
- [ ] If ported — `TestKeepsCloudflarePlaceholdersWhenEnvIsAbsent`: dispatched
      with an empty env, the `BaseURL` is returned **unchanged**, placeholders
      and all (upstream returns the same object identity when nothing
      substitutes; the Go equivalent is simply not mutating it).
- [ ] If ported — `TestCloudflareResolutionIsIdempotent`: an already-resolved
      URL passes through untouched, proving the auth-time and dispatch-time
      passes compose.
- [ ] Both cases of upstream's `test/cloudflare-stream.test.ts` (new, 65 lines)
      are represented — as Go tests if ported, or named in the deviation entry
      as behavior kern-link deliberately does not have.
- [ ] The existing `ai/providers/cloudflare_auth.go` tests still pass unchanged;
      auth-time resolution is not removed.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(providers): resolve cloudflare endpoint placeholders at dispatch time`
      (or `docs: disposition src/providers/cloudflare-stream.ts as a deviation`).

## Relevant files / areas

- `ai/providers/cloudflare_auth.go` (127 lines) — `:24`
  `resolveCloudflareBaseURL`, `:42` `resolveCloudflareResolvedEnv`, `:84`
  `cloudflareWorkersAIAuth`, `:105` `cloudflareAIGatewayAuth`.
- `ai/providers/cloudflare_workers_ai.go`, `ai/providers/cloudflare_ai_gateway.go`
  — the two bindings.
- `ai/provider.go` — `ProviderStreams` / `Stream` / `StreamSimple`, the surface
  upstream's `cloudflareStreams` wraps.
- `docs/PORTING.md:45` — the existing `src/api/cloudflare.ts` row.
- Upstream: `src/providers/cloudflare-stream.ts` (new in range, 28 lines),
  `src/providers/cloudflare-auth.ts` (+68/-35), `src/api/cloudflare.ts`
  (unchanged).
- Upstream test: `test/cloudflare-stream.test.ts` (new, 65 lines).

## Dependencies

- **Blocked by**: None.
- **Blocks**: None.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
