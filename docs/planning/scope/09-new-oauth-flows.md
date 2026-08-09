---
type: Decision
title: "Four new OAuth flows"
description: "Whether radius, openrouter, kimi-coding, and xai OAuth flows are in scope for this sync."
tags: [decision, scope]
timestamp: 2026-08-09T01:30:23Z
phase: scope
decision: 09
slug: new-oauth-flows
status: decided
verdict: "Port all four (radius, openrouter, kimi-coding, xai) in their own milestone. Deferring them would forfeit the Go base that the next sync's diff needs to apply against, converting a one-time cost into permanent re-derivation."
decided_via: discussion
depends_on: [auth-restructure]
---

# Question

Upstream added four OAuth flows in range, on top of the three kern-link ports
today (Anthropic PKCE, GitHub Copilot device code, Codex dual):

| Upstream file | Δ |
|---|---|
| `src/auth/oauth/radius.ts` | +403 |
| `src/auth/oauth/openrouter.ts` | +311 |
| `src/auth/oauth/kimi-coding.ts` | +310 |
| `src/auth/oauth/xai.ts` | +239 |

That is ~1263 lines of new auth code, each flow needing Go porting, tests, and
wiring into `cmd/pi-ai login`.

Two things make these more expensive than their line count suggests. First,
`SPECS.md` records a deliberate terms-of-service position: subscription OAuth
makes kern-link present itself as a first-party client, which is "fine for
personal use, and a real risk to ship in a product". Each new flow extends that
exposure and needs the same documentation treatment in `docs/auth.md`. Second,
OAuth flows are the hardest thing in this repo to test offline — the existing
ones needed injectable clocks and local callback servers.

# Options

- **All four in scope** — full parity on the auth surface.
- **None in scope** — record all four as deviations; the providers still work by
  API key where they support one.
- **Subset by usability** — port the flows for providers whose bindings are
  already in the port and that have no API-key path; defer the rest.

# Recommendation

**Defer all four out of this program**, recorded as explicit deviations in
`docs/PORTING.md` with a revisit trigger.

The reasoning is a cost/benefit the audit makes concrete: ~1263 lines plus the
hardest test scaffolding in the repo, for flows that mostly serve providers
either not yet bound (`radius`) or already reachable by API key (`openrouter`,
`xai`). Set against a program that is already carrying a breaking core migration
and nine adapter updates, this is the cleanest thing to cut without weakening
the parity bar in [decision 02](/scope/02-parity-bar.md) — because that bar
counts *dispositioned*, not *ported*.

If any single one is kept, `openrouter` is the strongest candidate: the
`openrouter` binding is already in the port and is one of the four dynamic
providers.

This decision pairs with [decision 10](/scope/10-new-provider-bindings.md) — a
binding without its OAuth flow is still useful if the provider takes an API key,
so check that the two verdicts stay consistent.

# Verdict

**Port all four.** Decided in discussion, 2026-08-09 — **reversing the
recommendation above.**

The maintainer's argument: the project's reason for existing is being a
full-parity port, and dropping half of what upstream adds on the very first sync
makes the debt unrecoverable. On examination the mechanism is sharper than
"debt", and sharper than the revisit-trigger mitigation this ledger originally
proposed:

> A deviation is not a postponement, it is the **loss of a base**. If
> `radius.ts` is not ported now, the next sync's diff reports "`radius.ts`
> changed" with no Go file to apply that change to. The cost is not deferred, it
> is converted into permanent re-derivation against a target that has since
> moved. Repeated across a few syncs, `sync.sh` produces noise the maintainer
> learns to ignore, and the port stops being a port.

Two facts measured during the discussion argue the same way:

- The cost is bounded: 1263 lines of flow source, plus 1056 lines of **upstream
  tests that come with them** (`radius-oauth` 129, `openrouter-oauth` 322,
  `kimi-coding-oauth` 270, `xai-oauth` 335).
- Those tests weaken this doc's own strongest objection. The Question section
  argued OAuth is the hardest thing here to test offline — true when the port had
  to invent the scaffolding, much less true when upstream ships 1056 lines of it
  to port.

Verified while deciding: all four providers declare **both** `apiKey:
envApiKeyAuth(...)` and `oauth: lazyOAuth(...)` at `936aff00`, so the flows are a
convenience layer over an API-key path that already works. That fact was the
original recommendation's main support — it survives, but it is now outweighed.

Lands in its own milestone, alongside the `docs/auth.md` treatment each flow
needs for the terms-of-service exposure `SPECS.md` records.
