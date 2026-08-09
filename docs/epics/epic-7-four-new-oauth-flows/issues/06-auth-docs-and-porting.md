---
type: Issue
title: "Document the four new flows in docs/auth.md and disposition their files in docs/PORTING.md"
description: "Extend docs/auth.md's OAuth table and its terms-of-service section with a per-flow treatment for radius, openrouter, kimi-coding and xai, and give each ported file its row in docs/PORTING.md."
tags: [epic-7]
timestamp: 2026-08-09T13:40:00Z
epic: 7
issue: 06
slug: auth-docs-and-porting
size: S
status: open
gh_issue: 186
resource: https://github.com/kern-ia/kern-link/issues/186
depends_on: [1, 2, 3, 4, 5]
---

# Document the four new flows in docs/auth.md and disposition their files in docs/PORTING.md

## Summary

Two of [EPIC_7](/epic-7-four-new-oauth-flows/EPIC_7.md)'s six acceptance
criteria are documentation, and neither is boilerplate.

**Criterion 3, `docs/auth.md`.** `SPECS.md` records the terms-of-service split
as a deliberate position, not a disclaimer: an API key is billed under a
developer agreement written for programmatic access, while subscription OAuth
hands over a *first-party client's* credential that kern-link then presents
itself as — "fine for personal use, and a real risk to ship in a product".
`docs/auth.md:117-163` says that today about `anthropic`, `openai-codex` and
`github-copilot`. Four flows arrive, and **they do not all land in the same
category**. Getting the categories right is the work here; a blanket warning
copied across all four would be as wrong as no warning at all.

| Flow | Category | Why |
|---|---|---|
| `xai` | **subscription OAuth** | `isSubscription: true`; the credential is a SuperGrok / X Premium subscription's, obtained through the Grok CLI's own OAuth client id (`b1a00492-073a-47ea-816f-4c329264a828`) with a `grok-cli:access` scope. Same category as `anthropic` — including the first-party client id, which the existing section's third bullet already covers. |
| `kimi-coding` | **subscription OAuth** | `isSubscription: true`; a Kimi Code subscription, through Kimi Code's client id (`17e5f671-d194-4dfb-9706-5516cb48c098`). |
| `openrouter` | **API key** | Not marked `isSubscription`, and rightly so: the flow's whole point is that the code exchange **mints a permanent API key on the user's own OpenRouter account**. It is a nicer way to obtain the same credential `OPENROUTER_API_KEY` carries, and it carries the same (absent) risk. Say that plainly — a reader who sees "OAuth" and assumes risk will avoid the safest login in the list. |
| `radius` | **neither, and say so** | Not marked `isSubscription`. A Radius gateway credential comes from the gateway you point at — often your own deployment — so the risk is whatever that operator's terms say, and kern-link is a legitimate client of a pi gateway rather than an impersonated one. One honest paragraph beats forcing it into one of the two existing buckets. |

Verify each of those claims against the code as landed rather than against
this table: read `IsSubscription` on each strategy and the client id constant
in each flow file. If one disagrees, the code is the fact and this table is
the thing to fix.

**Criterion 5, `docs/PORTING.md`.** The mapping table is what
`upstream/sync.sh` output is resolved against, so an unlisted file is a file
the next sync cannot route. This issue dispositions the four flow files, their
four test files, and the *partial* port of `src/providers/radius-config.ts` —
`NormalizeGatewayURL` lands in `ai/auth/oauth/radius.go`
([issue 04](/epic-7-four-new-oauth-flows/issues/04-radius-gateway-oauth.md))
while the rest of that file is
[Epic 8](/epic-8-pi-messages-and-radius/EPIC_8.md)'s. A row that claims the
whole file is ported would make epic 8 look done before it starts; say which
symbol, and name epic 8 for the remainder.

**Land after [epic 6 issue 11](/epic-6-auth-core-and-env-api-key-bindings/issues/11-porting-paths-and-dispositions.md).**
That issue rewrites row 43 (`src/utils/oauth/*` → `ai/auth/oauth`) to
upstream's new `src/auth/oauth/*` paths. Writing new rows against the old
paths first guarantees a conflict and, worse, a table that is half-migrated.

## Scope

- `docs/auth.md`:
  - `# OAuth logins — the pi-ai CLI` (`:91`): "Three providers support OAuth"
    becomes seven, and the table gains a row per new flow in the same
    three-column shape (`Provider` / `Flow` / `What you do`) — device code for
    `xai` and `kimi-coding`, PKCE on an ephemeral loopback port for
    `openrouter`, and browser-or-device-code against a gateway for `radius`.
    Mention `KIMI_CODE_OAUTH_HOST` where it belongs (the Kimi row or the env
    table), since it is user-facing configuration.
  - `# Credential modes and terms-of-service risk` (`:117`): keep the two-mode
    framing, extend `## Subscription OAuth` with `xai` and `kimi-coding`
    (including their first-party client ids in the existing bullet list), and
    add short treatments for `openrouter` and `radius` that place them
    *outside* the subscription bucket, per the Summary's table. The existing
    "use API keys if you ship it" conclusion stands.
  - `# Env API keys` (`:32`) — check the `openrouter`, `xai` and `kimi-coding`
    rows still read correctly now that each has a second way in.
- `README.md:27` and `:125` — both enumerate the OAuth flows ("Claude Pro/Max,
  ChatGPT Plus/Pro, GitHub Copilot"). `SPECS.md` says the README carries this
  framing too, so update both lines rather than letting the README contradict
  `docs/auth.md`.
- `docs/PORTING.md` — rows for `src/auth/oauth/radius.ts`, `openrouter.ts`,
  `kimi-coding.ts`, `xai.ts` → their Go files; the four upstream test files;
  and the partial `radius-config.ts` row described above. Each row states
  *why* in the disposition column, matching the table's existing habit. Record
  the two Go-shaping decisions a reader would otherwise reverse-engineer:
  `createRadiusOAuth` → the exported `RadiusOAuth` factory, and
  `NormalizeGatewayURL` living in `ai/auth/oauth` because `ai/providers`
  already imports it and the reverse edge would be an import cycle.

## Out of scope

- `src/auth/oauth/load.ts`, `src/bun-oauth.ts`, `src/compat/extension-oauth-types.ts`,
  `src/utils/abort.ts` — [epic 6 issue 11](/epic-6-auth-core-and-env-api-key-bindings/issues/11-porting-paths-and-dispositions.md)
  owns those dispositions.
- `src/providers/radius.ts` and the rest of `radius-config.ts` —
  [Epic 8](/epic-8-pi-messages-and-radius/EPIC_8.md).
- The whole-repo disposition sweep — [Epic 9](/epic-9-classifier-audit-and-release/EPIC_9.md).
- `upstream/UPSTREAM.lock`. The pin advances in epic 9; the whole program is
  still building against `936aff00`.
- `CHANGELOG.md`. Release preparation is epic 9's, not per-PR
  (`CONVENTIONS.md`, "Review").
- Any code change. If documenting a flow reveals a behaviour that is wrong,
  file it rather than fixing it here — a docs PR that also changes behaviour
  is two PRs.

## Acceptance criteria / Definition of done

- [ ] `docs/auth.md`'s OAuth table lists all seven providers, and every
      `Provider` cell matches an id `pi-ai login <id>` accepts after
      [issue 05](/epic-7-four-new-oauth-flows/issues/05-cli-login-new-flows.md)
      (check them against `cmd/pi-ai`'s test for the picker list, not from
      memory).
- [ ] The terms-of-service section names `xai` and `kimi-coding` as
      subscription OAuth **with their client ids**, and explicitly places
      `openrouter` (mints a real API key on the user's account) and `radius`
      (a gateway credential, risk set by the gateway operator) outside that
      category.
- [ ] Each of those four claims is verified against the landed code —
      `IsSubscription` on the strategy and the client-id constant in the flow
      file — and the PR body quotes the four `IsSubscription` lines as
      evidence.
- [ ] `README.md` no longer enumerates only three OAuth flows in either
      `:27` or `:125`.
- [ ] `grep -c "src/auth/oauth" docs/PORTING.md` shows a row for each of the
      four flow files, and `git grep -l "// Ports: packages/ai/src/auth/oauth"
      ai/auth/oauth` lists all four new Go files — the header and the table
      agreeing is the point.
- [ ] The `radius-config.ts` row names `normalizeRadiusGatewayUrl` as the
      only symbol ported and names epic 8 for the remainder.
- [ ] No stale claim survives: `grep -n "Three providers support OAuth"
      docs/auth.md` returns nothing.
- [ ] `bash upstream/sync_test.sh` passes and CI is green
      (`go test ./... -race -v`, `golangci-lint` v2.12.2) — a docs-only PR
      still gates on the same checks.
- [ ] Conventional Commit, e.g.
      `docs: document the four new oauth flows and their terms-of-service risk`.

## Relevant files / areas

- `docs/auth.md:91-116` (the CLI OAuth table), `:117-163` (credential modes and
  terms-of-service risk), `:32-77` (env API keys).
- `README.md:27`, `:125`.
- `docs/PORTING.md:43` (the `ai/auth/oauth` row, rewritten by epic 6 issue 11),
  and the "Intentional deviations" section below the table.
- The four new flow files from issues 01–04, for the `IsSubscription` and
  client-id verification.
- `docs/planning/SPECS.md` — "Terms-of-service split, documented deliberately",
  the requirement this criterion comes from.
- Upstream at `936aff00`: `src/auth/oauth/{radius,openrouter,kimi-coding,xai}.ts`
  and `test/{radius,openrouter,kimi-coding,xai}-oauth.test.ts`.

## Dependencies

- **Blocked by**: issues [01](/epic-7-four-new-oauth-flows/issues/01-xai-device-code-oauth.md)–[04](/epic-7-four-new-oauth-flows/issues/04-radius-gateway-oauth.md)
  (the flows must exist to be documented and verified against) and
  [issue 05](/epic-7-four-new-oauth-flows/issues/05-cli-login-new-flows.md)
  (the CLI table describes what `pi-ai login` actually offers). Also
  [epic 6 issue 11](/epic-6-auth-core-and-env-api-key-bindings/issues/11-porting-paths-and-dispositions.md),
  which migrates the `PORTING.md` rows these new ones sit beside.
- **Blocks**: Nothing. **Land last in this epic.**

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
