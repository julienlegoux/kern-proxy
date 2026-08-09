---
type: Decision
title: "Deferred tools capability"
description: "Whether upstream's new deferred-tool-execution capability is ported in this sync."
tags: [decision, scope]
timestamp: 2026-08-09T01:30:23Z
phase: scope
decision: 11
slug: deferred-tools
status: decided
verdict: "Port fully, in the core-types milestone. The type half is already mandatory, so shipping the enum values without the behavior would leave StopReason cases that are unreachable for undocumented reasons. Prove the new stop reasons end to end through the faux provider."
decided_via: triage
depends_on: [core-types-migration]
---

# Question

Upstream added a deferred-tools capability with no Go counterpart:
`src/utils/deferred-tools.ts`, a `DeferredHandle` type, `DeferredFetchOptions` /
`DeferredCancelOptions` extending the new `ProviderRequestOptions`, and
`test/deferred-tools.test.ts`.

Critically, this capability is **entangled with the core types** rather than
bolted on: `StopReason` gains `"pending"` and `"deferred"` precisely to express
it, and `models.ts` gains `ModelsDeferredFetchOptions` /
`ModelsDeferredCancelOptions`. Those type changes are already in scope under
[decision 04](/scope/04-core-types-migration.md), because the adapters won't
compile without them.

So the real question is not "do we take the types" — we do — but whether the
*behavior* behind them is implemented or left as inert surface.

# Options

- **Port fully** — types plus `deferred-tools.go` plus the adapter-side handling
  plus the ported test.
- **Types only, behavior deferred** — the enum values and option structs exist
  (they must), but nothing produces or consumes them; documented as a deviation.
- **Neither** — not available: the type changes are load-bearing for the rest of
  the sync.

# Recommendation

**Port fully, in the same epic as the core types.**

The usual reason to defer a new capability is that it is separable. This one is
not: half of it is already mandatory, and shipping `StopReason: "deferred"` as a
value the library can emit-but-never-emits is a worse outcome than either
alternative — a consumer switching on `StopReason` gets a case that is
unreachable for reasons nothing documents.

Porting it is also cheaper than its novelty suggests: `utils/deferred-tools.ts`
is a utility module, upstream ships a test for it, and kern-link's `faux`
provider (described in its own docs as "the executable specification of the
event contract") is the natural place to prove the new stop reasons flow through
the event protocol end to end.

If this is instead deferred, the deviation entry must be explicit that the
`"pending"`/`"deferred"` stop reasons are present-but-unreachable, so a consumer
reading the type isn't misled.

# Verdict

**Port fully, in the same epic as the core types.** Accepted as recommended.

The deciding argument is that this capability is not separable: `StopReason`'s
new `"pending"` and `"deferred"` values and the `ModelsDeferred*Options` types
are already mandatory under [decision 04](/scope/04-core-types-migration.md),
because the rest of the sync will not compile without them. Taking the types and
skipping the behavior would ship a public enum with cases the library can never
emit — strictly worse for a consumer than either porting or not.

`ai/providers/faux` is the natural proving ground: it is described in its own
docs as "the executable specification of the event contract", so the new stop
reasons flowing through the event protocol end to end belongs there.
