---
type: Decision
title: "Auth package restructure"
description: "Whether to mirror upstream's move of utils/oauth into auth/oauth, and port the auth-core changes."
tags: [decision, scope]
timestamp: 2026-08-09T01:30:23Z
phase: scope
decision: 08
slug: auth-restructure
status: decided
verdict: "Port auth content only — no Go package moves, since ai/auth/oauth already matches upstream's destination. Update docs/PORTING.md's upstream paths to the new src/auth/oauth/* locations. Exclude bun-oauth.ts and compat/extension-oauth-types.ts as deviations."
decided_via: triage
depends_on: [core-types-migration]
---

# Question

Upstream reorganized its auth code in range. `src/utils/oauth/index.ts`,
`load.ts`, and `types.ts` were **deleted**; `anthropic.ts`, `github-copilot.ts`,
and `openai-codex.ts` **moved** from `src/utils/oauth/` to `src/auth/oauth/`
with edits. `src/auth/oauth/load.ts` is new. Alongside that:
`auth/credential-store.ts`, `auth/helpers.ts`, `auth/resolve.ts`,
`auth/types.ts`, `src/env-api-keys.ts`, and `src/oauth.ts` all changed, and
`src/bun-oauth.ts` and `src/compat/extension-oauth-types.ts` are new.

kern-link's Go layout **already** looks like upstream's destination, not its
source: `ai/auth/oauth` exists today, and `docs/PORTING.md` maps
`src/utils/oauth/*` + `src/oauth.ts` → `ai/auth/oauth`. The port made this
structural call independently, before upstream did.

So the structural half of this change is a no-op for Go. What remains is the
*content* of the moved files, the auth-core changes, and PORTING.md's mapping
table pointing at upstream paths that no longer exist.

# Options

- **Port content only; update PORTING.md's paths** — no Go package moves,
  because the Go tree is already right. Update the mapping table's left column
  to the new upstream paths so the next sync's `sync.sh` output resolves.
- **Restructure to mirror upstream literally** — pointless churn; it would move
  Go files that are already where upstream just moved its own.
- **Defer the auth-core changes** — leaves `resolve.ts`/`credential-store.ts`
  behavior at 0.80.3 while the new OAuth flows
  ([09](/scope/09-new-oauth-flows.md)) expect the new contracts.

# Recommendation

**Port content only, and update `docs/PORTING.md`'s upstream paths.** The
mapping table is the artifact that makes every future sync navigable; leaving it
pointing at `src/utils/oauth/*` after upstream deleted that directory would
silently break the one document the sync procedure depends on.

Two exclusions to record as deviations rather than work:

- **`src/bun-oauth.ts`** — a Bun-runtime entry point. No Go equivalent, same
  category as the `*.lazy.ts` bundler shims PORTING.md already excludes.
- **`src/compat/extension-oauth-types.ts`** — lives under upstream's `compat/`,
  and PORTING.md already declines to port `src/compat.ts` as deprecated
  back-compat surface. Worth a one-line check that this is the same category
  before excluding it, not an assumption.

# Verdict

**Port content only; update `docs/PORTING.md`'s paths.** Accepted as
recommended; lands in milestone 5.

The structural half of upstream's change is a no-op for Go: kern-link's
`ai/auth/oauth` already sits where upstream just moved its own code, a call the
port made independently and earlier. What remains is the content of the moved
files plus the `auth/*` core changes.

Updating the mapping table's upstream paths is not bookkeeping — it is the
artifact the whole sync procedure navigates by, and leaving it pointing at the
deleted `src/utils/oauth/*` would break the next sync's ability to resolve
`sync.sh` output against Go packages.

Two exclusions recorded as deviations:

- **`src/bun-oauth.ts`** — a Bun-runtime entry point; same category as the
  `*.lazy.ts` bundler shims PORTING.md already excludes.
- **`src/compat/extension-oauth-types.ts`** — under upstream's `compat/`, which
  PORTING.md already declines as deprecated back-compat surface. The epic should
  spend one line confirming it is the same category rather than assuming it.
