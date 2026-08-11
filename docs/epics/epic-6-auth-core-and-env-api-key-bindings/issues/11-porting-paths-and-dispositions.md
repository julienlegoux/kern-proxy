---
type: Issue
title: "Point docs/PORTING.md at upstream's src/auth/* paths and disposition the new auth files"
description: "Rewrite the mapping table's deleted src/utils/oauth/* entries to upstream's current locations, give every new or changed auth file in range a disposition, and bring docs/auth.md's env-key table and resolution-order section back in line with what this epic changed."
tags: [epic-6]
timestamp: 2026-08-11T12:30:00Z
epic: 6
issue: 11
slug: porting-paths-and-dispositions
size: S
status: open
gh_issue: 180
resource: https://github.com/kern-ia/kern-link/issues/180
depends_on: []
---

# Point docs/PORTING.md at upstream's src/auth/* paths and disposition the new auth files

## Summary

`docs/PORTING.md`'s mapping table is what the whole sync procedure navigates by:
`upstream/sync.sh` prints changed upstream paths bucketed by area, and the table
is how a maintainer turns those paths into Go packages. Upstream deleted
`src/utils/oauth/` in range. Left alone, the table's row 43 points at a
directory that no longer exists, and the next sync's output stops resolving —
which is why [decision 08](../../../planning/scope/08-auth-restructure.md) calls
this "not bookkeeping" and
[EPIC_6](/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md) makes it
acceptance criterion 3.

Rows to fix, and the new files that need a first disposition:

| Upstream path at `936aff00` | Status |
|---|---|
| `src/utils/oauth/*` → `src/auth/oauth/*` | **moved** — row 43's left column is stale |
| `src/utils/oauth/index.ts`, `load.ts`, `types.ts` | **deleted** upstream; `types.ts`'s content folded into `src/auth/types.ts` (row 40 names it as a second source and must drop it) |
| `src/auth/oauth/load.ts` (new, 68) | needs a row — bundler-opaque dynamic-import registry, same category as `lazyOAuth` at row 42 |
| `src/bun-oauth.ts` (new, 21) | needs a row — Bun-runtime entry point, same category as the `*.lazy.ts` shims at row 38 |
| `src/compat/extension-oauth-types.ts` (new, 45) | needs a row — [decision 08](../../../planning/scope/08-auth-restructure.md) asks for **one line of verification** that it is the same category as `src/compat.ts`, already declined under "Intentional deviations", rather than an assumption |
| `src/oauth.ts` (now 10 lines, type-only) | row 43 pairs it with `ai/auth/oauth`; at `936aff00` it re-exports **only types** from `compat/extension-oauth-types.ts`, so the pairing is wrong |
| `src/utils/abort.ts` (new, 50) | needs a row — `AbortSignal` ergonomics; the "Cancellation" deviation covers it, but "covered by an existing deviation" is itself a disposition and must be written |

`sync.sh`'s own auth bucket regex still lists the dead paths:

```
bucket "Auth (ai/auth)" 'packages/ai/src/(auth/|oauth\.ts|env-api-keys\.ts|utils/oauth/|utils/provider-env\.ts)' ''
```

`src/auth/` already matches everything that moved, so the bucket is not broken —
but `utils/oauth/` can never match again. Leave it or drop it; if you drop it,
`upstream/sync_test.sh` must still pass, and that test is a CI gate.

## Scope

- `docs/PORTING.md` — rewrite rows 40, 42, 43 and the `src/env-api-keys.ts` row
  (59) against upstream's `936aff00` paths, and add rows for the four new files
  above. Every row keeps the table's existing three-column shape and its habit
  of stating *why* in the disposition column, not just *whether*.
- **Verify, do not assume**, that `src/compat/extension-oauth-types.ts` is the
  same category as `src/compat.ts`: read it at `936aff00`, and record what it
  actually contains in the disposition. If it turns out to carry a live
  contract rather than deprecated back-compat surface, say so and flag it — the
  point of the check is that it can come back "no".
- Record the Go-shaping decisions this epic made that a reader would otherwise
  have to reverse-engineer:
  - `GetAuthForProvider` as the Go form of upstream's `getAuth` overload
    ([issue 06](/epic-6-auth-core-and-env-api-key-bindings/issues/06-models-login-logout.md));
  - `Provider.FilterModels` as a required method returning its input unchanged,
    where upstream has an optional one
    ([issue 05](/epic-6-auth-core-and-env-api-key-bindings/issues/05-models-availability.md));
  - `AuthOperationOptions` and `ProviderAuthInteraction` folded into the
    existing `ctx` deviation
    ([issue 01](/epic-6-auth-core-and-env-api-key-bindings/issues/01-auth-contract-surface.md)).
- **`docs/auth.md`'s env-key table (`:34-59`) and its resolution-order section
  (`:15-30`).** Three of this epic's issues change what those two sections
  state, and nothing else in the program updates them —
  [Epic 7 issue 06](/epic-7-four-new-oauth-flows/issues/06-auth-docs-and-porting.md)
  covers the *terms-of-service* sections for the four new flows and says so in
  its own `## Out of scope`. So they land here, with the epic's other
  documentation, and last:
  - `ANTHROPIC_AUTH_TOKEN` goes **first** in the `anthropic` row's precedence
    list (`docs/auth.md:36`, today `ANTHROPIC_OAUTH_TOKEN, ANTHROPIC_API_KEY`),
    per [issue 07](/epic-6-auth-core-and-env-api-key-bindings/issues/07-anthropic-auth-token.md).
    Note in the row that it resolves as an `Authorization: Bearer` header
    rather than an api key — the table's "Env var(s), in precedence order"
    heading does not say that on its own.
  - Rows for the four providers
    [issue 10](/epic-6-auth-core-and-env-api-key-bindings/issues/10-env-api-key-bindings.md)
    binds: `baseten` (`BASETEN_API_KEY`), `qwen-token-plan`,
    `qwen-token-plan-cn` and `qwen-token-plan-individual`. Copy the env-var
    names from the landed bindings, not from this list.
  - The resolution-order section's line 3 claim — "A stored credential *owns*
    the provider — there is no silent env fallback behind it"
    (`docs/auth.md:22-24`) — is contradicted by the per-field
    credential→env merge
    [issue 03](/epic-6-auth-core-and-env-api-key-bindings/issues/03-provider-scoped-apikey-resolution.md)
    introduces for the Cloudflare resolvers. Rewrite it to state the rule that
    actually holds after this epic: a stored credential owns the provider's
    *key*, and a credential that does not carry a given configuration field
    falls through to the environment for that field. Name the fields it applies
    to; do not turn a narrow, deliberate exception into a general one.
- Check the file for rows this epic's other issues already amended (issues 03,
  07 and 09 each touch it) and reconcile rather than duplicate. **Land this
  last.**

## Out of scope

- The four new OAuth flows' rows — [Epic 7](/epic-7-four-new-oauth-flows/EPIC_7.md)
  dispositions its own files (its acceptance criterion 5).
- `src/providers/radius.ts` / `radius-config.ts` — [Epic 8](/epic-8-pi-messages-and-radius/EPIC_8.md).
- The whole-repo disposition sweep — [Epic 9](/epic-9-classifier-audit-and-release/EPIC_9.md).
  This issue covers the auth surface only; anything it notices outside that,
  it flags rather than fixes.
- `upstream/UPSTREAM.lock`'s pinned commit. Advancing the pin is
  [decision 17](../../../planning/scope/17-upstream-lock-and-weekly-job.md)'s,
  in epic 9. Do not touch it here — the whole program is still building against
  `936aff00`.
- `docs/auth.md`'s **OAuth** sections — the CLI OAuth table (`:91-116`) and the
  credential-modes / terms-of-service treatment (`:117-163`). Those are per-flow
  and belong with the flows:
  [Epic 7 issue 06](/epic-7-four-new-oauth-flows/issues/06-auth-docs-and-porting.md)
  (#186). This issue owns the env-key table and the resolution-order section
  only; the two halves of the file are edited by two issues in two epics, so
  keep to yours.

## Acceptance criteria / Definition of done

- [ ] `grep -n "utils/oauth" docs/PORTING.md` returns nothing, or returns only a
      line that explicitly records the move (`src/utils/oauth/* → src/auth/oauth/*`)
      as history.
- [ ] Every upstream auth path that `git diff 244f1dea..936aff00 --name-only --
      'packages/ai/src/auth/' 'packages/ai/src/oauth.ts'
      'packages/ai/src/env-api-keys.ts' 'packages/ai/src/bun-oauth.ts'
      'packages/ai/src/compat/' 'packages/ai/src/utils/abort.ts'` reports
      appears in `docs/PORTING.md` — as a mapped row or a named deviation.
      Paste that command's output into the PR body next to the table diff.
- [ ] The `src/compat/extension-oauth-types.ts` disposition names what the file
      contains, not just which category it was assigned to.
- [ ] Each deviation row carries a reason; each *deferred* deviation also
      carries a revisit trigger (a version or an event, never "later" —
      `../_shared` drift conventions and
      [decision 02](../../../planning/scope/02-parity-bar.md)).
- [ ] `docs/auth.md`'s `anthropic` row names `ANTHROPIC_AUTH_TOKEN` first and
      says it resolves as a bearer header:
      `grep -n 'ANTHROPIC_AUTH_TOKEN' docs/auth.md` returns a line inside the
      env-key table.
- [ ] The env-key table has a row for each provider issue 10 bound —
      `for p in baseten qwen-token-plan qwen-token-plan-cn
      qwen-token-plan-individual; do grep -qn "\`$p\`" docs/auth.md || echo
      "MISSING $p"; done` prints nothing — and each env var matches the landed
      binding, not this issue's text.
- [ ] `grep -n 'no silent env fallback' docs/auth.md` returns nothing, or
      returns a line that scopes the claim to the fields it still holds for.
      The resolution-order section and
      [issue 03](/epic-6-auth-core-and-env-api-key-bindings/issues/03-provider-scoped-apikey-resolution.md)'s
      per-field merge must not state opposite rules.
- [ ] `bash upstream/sync_test.sh` passes, whether or not the auth bucket regex
      was touched.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2).
- [ ] Conventional Commit, e.g.
      `docs: point PORTING.md at upstream's restructured auth paths`.

## Relevant files / areas

- `docs/auth.md:15-30` the resolution-order section (`:22-24` the
  stored-credential claim), `:34-59` the env-key table (`:36` the `anthropic`
  row).
- `docs/PORTING.md:40` (auth core), `:41` (filestore), `:42` (helpers), `:43`
  (`src/utils/oauth/*` + `src/oauth.ts`), `:59` (`src/env-api-keys.ts`), and the
  "Intentional deviations" section below the table.
- `upstream/sync.sh` — the `bucket "Auth (ai/auth)"` regex.
- `upstream/sync_test.sh` — the CI gate over `sync.sh`'s diff-detection logic.
- Upstream at `936aff00`: `src/auth/oauth/load.ts`, `src/bun-oauth.ts`,
  `src/compat/extension-oauth-types.ts`, `src/oauth.ts`, `src/utils/abort.ts`.
- [Decision 08](../../../planning/scope/08-auth-restructure.md) — the two
  exclusions it asks to be recorded as deviations.

## Dependencies

- **Blocked by**: None mechanically, but it should land **last** in the epic —
  issues 03, 07 and 09 each amend `docs/PORTING.md`, and this issue reconciles
  the file as a whole. The `docs/auth.md` half sharpens that: it documents what
  issues 03, 07 and 10 landed, so all three must be merged before it is written.
- **Blocks**: Nothing.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
