---
type: Decision
title: "Seven new provider bindings"
description: "Which of upstream's new provider bindings this sync adds to ai/providers."
tags: [decision, scope]
timestamp: 2026-08-09T01:30:23Z
phase: scope
decision: 10
slug: new-provider-bindings
status: decided
verdict: "Port all of them, radius included. The four env-API-key bindings ride with the auth milestone; radius (82 lines) plus radius-config (96) lands with pi-messages and the radius OAuth flow, since it needs both."
decided_via: discussion
depends_on: [catalog-schema-and-tooling, new-oauth-flows]
---

# Question

Upstream added these provider-side files in range:

| Upstream file | Auth mode (read from the source at `936aff00`) |
|---|---|
| `providers/baseten.ts` | `envApiKeyAuth("Baseten API key", ["BASETEN_API_KEY"])` |
| `providers/qwen-token-plan.ts` | `envApiKeyAuth(…, ["QWEN_TOKEN_PLAN_API_KEY"])` |
| `providers/qwen-token-plan-cn.ts` | env API key |
| `providers/qwen-token-plan-individual.ts` | env API key |
| `providers/radius.ts` + `radius-config.ts` | env API key **and** OAuth, plus a runtime gateway-config fetch |
| `providers/cloudflare-stream.ts` | not a binding — Cloudflare streaming support code |
| `providers/openrouter-images.ts` | images surface (see [decision 14](/scope/14-images-surface.md)) |

kern-link binds 35 providers today, one file each under `ai/providers`, named
after the provider ID with hyphens as underscores (`CONVENTIONS.md`). A plain
env-API-key binding is a small, highly patterned file plus its catalog entry.

The cost here is very unevenly distributed, which is the whole point of
separating this decision from [09](/scope/09-new-oauth-flows.md).

# Options

- **All of them** — full binding parity.
- **The four cheap env-API-key bindings only** (`baseten`, `qwen-token-plan` ×3),
  deferring `radius`; handle `cloudflare-stream` and `openrouter-images` under
  their own decisions.
- **None** — record all as deviations. Leaves the catalog carrying models the
  registry cannot serve, which is worse than either alternative.

# Recommendation

**Take the four env-API-key bindings; defer `radius`.**

`baseten` and the three `qwen-token-plan` variants are each an `envApiKeyAuth`
one-liner plus a catalog entry — they follow the existing 35-binding pattern
exactly, and their catalog files come along free with
[decision 05](/scope/05-catalog-schema-and-tooling.md)'s regeneration. Excluding
them would cost more in explanation than including them costs in code.

`radius` is a different animal: OAuth (+403 in `auth/oauth/radius.ts`), a
separate `radius-config.ts`, and a **runtime gateway-config fetch** — the source
calls `loadRadiusGatewayConfig(gateway, apiKey, signal)` during provider setup,
which is a network dependency none of the existing 35 bindings has. Defer it
with [decision 09](/scope/09-new-oauth-flows.md)'s radius OAuth flow, as one
coherent unit, and record it in `docs/PORTING.md`.

`cloudflare-stream.ts` needs a one-line look during the adapter epic to classify
it — it is Cloudflare streaming support, not a binding, and probably belongs
with the existing `cloudflare_auth.go`.

# Verdict

**Port all of them, `radius` included.** Decided in discussion, 2026-08-09 —
**partially reversing the recommendation above**, which proposed deferring
`radius`.

The four env-API-key bindings (`baseten`, `qwen-token-plan` ×3) are unchanged
from the recommendation: small, patterned, catalog entries arrive free with
[decision 05](/scope/05-catalog-schema-and-tooling.md). They ride with the auth
milestone.

`radius` is now in scope under the full-parity principle established in
[decision 09](/scope/09-new-oauth-flows.md): a binding left unported is a base
the next sync's diff cannot be applied to. Measured, it is also smaller than its
dependency graph suggested — `providers/radius.ts` is **82 lines** and
`radius-config.ts` is **96**. What made it look expensive was not its own size
but its two prerequisites, both of which are now in scope anyway: the radius
OAuth flow (decision 09) and the `pi-messages` adapter
([decision 13](/scope/13-pi-messages-adapter.md)).

So `radius` lands **with** those two, not before them — the three form one
coherent unit and one milestone.

The genuinely open item is unchanged: `cloudflare-stream.ts` is not a binding and
needs a one-line classification during the adapter work; it most likely belongs
beside the existing `cloudflare_auth.go`.
