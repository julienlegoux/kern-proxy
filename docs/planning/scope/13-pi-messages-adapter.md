---
type: Decision
title: "pi-messages wire adapter"
description: "Whether upstream's new pi-messages protocol becomes a tenth adapter package in this sync."
tags: [decision, scope]
timestamp: 2026-08-09T01:30:23Z
phase: scope
decision: 13
slug: pi-messages-adapter
status: decided
verdict: "Port it as a tenth ai/apis package, in the same milestone as the radius provider and the radius OAuth flow. Still locked to radius — but locked to porting it, not to deferring it."
decided_via: discussion
depends_on: [new-provider-bindings, adapter-updates]
---

# Question

Upstream added `src/api/pi-messages.ts` (+433) — a new wire protocol that would
become a tenth package under `ai/apis/`. Its own doc comment describes it:

> Streams pi's own message protocol directly to a backend: the request is a
> single POST of `{ model, context, options }` to `<baseUrl>/messages`, the
> response is an SSE stream of serialized assistant-message events plus a
> terminal `done`/`error` event. **This is the wire protocol spoken by the Radius
> gateway**, but any backend implementing it can be used, e.g. via a models.json
> custom provider with `"api": "pi-messages"`.

Grepping upstream at `936aff00`, its only in-tree consumers are
`providers/radius.ts`, `providers/radius-config.ts`, `auth/oauth/radius.ts`,
`compat.ts` (which kern-link does not port), and the index/type registrations.

So `pi-messages` and `radius` are one unit, not two independent items.

# Options

- **Port it** — adds a tenth adapter package plus its `ai.Api` constant and
  registration; only useful once `radius` exists or a consumer runs their own
  pi-messages backend.
- **Defer with radius** — one coherent deferral, recorded together in
  `docs/PORTING.md`.
- **Port the adapter but not radius** — leaves a working protocol with no
  built-in provider that speaks it. Defensible only if a consumer intends to
  point it at a self-hosted backend.

# Recommendation

**Defer, as one unit with `radius`.** Keep the two verdicts locked together: the
adapter's only built-in consumer is the provider that
[decision 10](/scope/10-new-provider-bindings.md) recommends deferring, and
porting a 433-line wire adapter that nothing in the module can reach is surface
without function.

One caveat that makes this reversible cheaply, and is worth writing into the
deviation entry: this protocol is *pi's own*, so it is the one adapter whose
shapes map almost one-to-one onto types kern-link already has
(`AssistantMessage`, the event protocol, `StopReason`). If a downstream consumer
— future Kern Lx included — ever wants to talk to a self-hosted pi-messages
backend, this is the cheapest adapter in the whole set to add later, and it does
not get harder by waiting.

Revisit trigger for the deviation: a consumer needs Radius, or needs to point
kern-link at a self-hosted pi-messages endpoint.

# Verdict

**Port it.** Decided in discussion, 2026-08-09 — **reversing the recommendation
above.**

The recommendation's coupling logic was right and is kept: `pi-messages` and
`radius` are one unit, because the adapter's only in-tree consumer is that
provider. What flipped is the direction — under the full-parity principle from
[decision 09](/scope/09-new-oauth-flows.md), they are locked together *in scope*
rather than locked together *out of it*.

The recommendation's own closing observation now cuts the other way. It noted
this is the cheapest adapter in the set to port, because the protocol is pi's
own and maps almost one-to-one onto types kern-link already has
(`AssistantMessage`, the event protocol, `StopReason`) — and that upstream ships
`test/pi-messages.test.ts` (248 lines) with it. An adapter that is both the
cheapest to port and structurally the closest to the existing domain model is
the weakest possible candidate for a deviation entry.

It becomes the tenth package under `ai/apis/`, with its `ai.Api` constant and
registration, in the same milestone as `radius` and the radius OAuth flow.
