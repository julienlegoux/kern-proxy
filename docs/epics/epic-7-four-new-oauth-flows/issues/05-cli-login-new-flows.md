---
type: Issue
title: "Offer every registered OAuth provider in pi-ai login, using each flow's login label"
description: "Replace cmd/pi-ai's hardcoded three-entry login list with one derived from the provider registry's OAuth strategies, add Radius against its default gateway, and show LoginLabel where a flow sets one."
tags: [epic-7]
timestamp: 2026-08-11T18:15:00Z
epic: 7
issue: 05
slug: cli-login-new-flows
size: S
status: open
gh_issue: 185
resource: https://github.com/kern-ia/kern-link/issues/185
depends_on: [1, 2, 3, 4]
---

# Offer every registered OAuth provider in pi-ai login, using each flow's login label

## Summary

`cmd/pi-ai/oauth.go:36-42` hardcodes the three login targets:

```go
func oauthProviders() []oauthProviderEntry {
	return []oauthProviderEntry{
		{ID: "anthropic", Auth: oauth.AnthropicOAuth},
		{ID: "github-copilot", Auth: oauth.CopilotOAuth},
		{ID: "openai-codex", Auth: oauth.CodexOAuth},
	}
}
```

Issues 01–03 attach OAuth to three more bindings, so that list is now the only
thing standing between a user and `pi-ai login xai`. Derive it instead: walk
the registry's providers, keep those whose `ai.ProviderAuth.OAuth` is non-nil,
and order them deterministically (by provider id — the CLI's output is
asserted by tests and a map-iteration order would flake). Six of the seven
targets then come for free, and the seventh binding that declares OAuth in a
later epic needs no CLI change at all.

**Radius is the exception, and it is a deliberate one.** Its strategy is a
factory over a gateway URL, and its provider binding is
[Epic 8](/epic-8-pi-messages-and-radius/EPIC_8.md)'s — there is nothing in the
registry to derive it from. Register it explicitly with upstream's
`DEFAULT_RADIUS_GATEWAY` (`https://radius.pi.dev`) so
[EPIC_7](/epic-7-four-new-oauth-flows/EPIC_7.md)'s acceptance criterion 4
("`cmd/pi-ai login` offers each of the four flows and completes them") is met
without epic 8. Mark the special case with a comment naming epic 8 as its
removal trigger: once `ai/providers/radius.go` exists and declares OAuth, the
registry supplies it and this branch should go. Do not invent a gateway flag
or environment variable to go with it — upstream has neither, and a CLI
surface this epic invents is a surface epic 8 has to keep.

**This is an assumption against `EPIC_7.md:41-43`**, which puts the `radius`
provider binding "and its gateway config" in epic 8. `defaultRadiusGateway`
is a single hardcoded constant, not a gateway configuration surface, and
`EPIC_7.md:53` (acceptance criterion 4) cannot be met without it — there is no
other way to offer a working Radius login from this epic alone. Taken because
the alternative is failing an acceptance criterion this epic owns; recorded
here so an auditor does not have to re-derive why it is fine.

`LoginLabel` is what the picker shows when a flow sets one
(`Sign in with SuperGrok or X Premium`, `Sign in with Kimi Code`,
`Sign in with OpenRouter`), falling back to `Name` for the flows that do not
(Anthropic, Copilot, Codex, Radius). That is the field's entire purpose —
[epic 6 issue 01](/epic-6-auth-core-and-env-api-key-bindings/issues/01-auth-contract-surface.md)
lands it explicitly unused and points here.

By the time this lands,
[epic 6 issue 06](/epic-6-auth-core-and-env-api-key-bindings/issues/06-models-login-logout.md)
has moved `runLogin`'s persistence into `Models.Login`. Wire the new targets
through whatever that issue left in place; if this issue somehow lands first,
say so in the PR body rather than reintroducing the hand-rolled
`store.Modify` call it deleted.

## Scope

- `cmd/pi-ai/oauth.go`:
  - Replace `oauthProviders()`'s literal list with derivation from the
    provider registry (`ai/providers`' `Models()`/provider list — use whatever
    accessor `runLogin`'s caller already has rather than constructing a second
    registry), filtered on `Auth.OAuth != nil`.
  - Append the Radius entry from `oauth.RadiusOAuth(oauth.RadiusOAuthOptions{
    Name: "Radius", Gateway: defaultRadiusGateway})`, with
    `defaultRadiusGateway = "https://radius.pi.dev"` declared here and a
    comment naming epic 8 as its removal trigger. **Then sort the whole list
    by id** — appending Radius before sorting, rather than after, is what
    makes the fix survive epic 8 removing the special case: Radius lands in
    alphabetical position instead of always last.
  - Picker labels: `LoginLabel` when non-empty, else `Name`. The **id** stays
    what `pi-ai login <id>` accepts and what keys the credential store.
  - The unknown-id error keeps its current shape and now lists what is
    actually available.
- `cmd/pi-ai/main.go` — only if the registry is not already reachable from
  `runLogin`'s call site. Prefer threading what exists to adding a parameter.
- `cmd/pi-ai/oauth_test.go` — extend the existing CLI tests.

## Out of scope

- A `pi-ai logout` command. `docs/auth.md` already records that there is none;
  [epic 6 issue 06](/epic-6-auth-core-and-env-api-key-bindings/issues/06-models-login-logout.md)
  ships `Models.Logout` as library surface and explicitly does not wire a
  command.
- `pi-ai list` and `pi-ai help` output.
- Interactive prompt rendering (`interactiveCallbacks`). The new flows use
  prompt and event types the CLI already renders — `select`, `manual_code`,
  `device_code`, `auth_url`, `progress`. If one of them renders badly, fix it
  here in a line or two; if it needs a new prompt *type*, that is a defect in
  the flow issue, not new CLI surface.
- The `ai.AuthEventInfo` event added by
  [epic 6 issue 01](/epic-6-auth-core-and-env-api-key-bindings/issues/01-auth-contract-surface.md).
  None of these four flows emits it.
- `docs/auth.md`'s CLI section — [issue 06](/epic-7-four-new-oauth-flows/issues/06-auth-docs-and-porting.md).

## Acceptance criteria / Definition of done

- [ ] `TestLoginListsEveryRegisteredOAuthProvider` — the interactive picker
      offers exactly `anthropic`, `github-copilot`, `kimi-coding`,
      `openai-codex`, `openrouter`, `radius`, `xai`, in that order, driving
      `runLogin` with a stubbed stdin/stdout (the `run(args, stdout, stderr)`
      seam `CONVENTIONS.md` names).
- [ ] `TestLoginPickerPrefersTheLoginLabel` — the `xai` row reads
      `Sign in with SuperGrok or X Premium` and the `anthropic` row reads its
      `Name`, proving both branches.
- [ ] `TestLoginAcceptsEachNewProviderId` — `pi-ai login xai`,
      `login kimi-coding`, `login openrouter` and `login radius` each select
      the matching strategy without prompting (assert the selected entry, not
      a completed network login).
- [ ] `TestLoginRadiusUsesTheDefaultGateway` — the Radius entry's strategy is
      built against `https://radius.pi.dev`.
- [ ] `TestLoginUnknownProviderListsAvailableIds` — `pi-ai login nope` fails
      with a message naming the available ids.
- [ ] The picker order is asserted, not incidental — a second run produces the
      same list (this is the map-iteration regression the sort exists for).
- [ ] Tests are offline, stdlib-only, no `t.Parallel()`, no build tags,
      discrete named functions. No test performs a real OAuth login.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(cli): offer every registered oauth provider in pi-ai login`.

## Relevant files / areas

- `cmd/pi-ai/oauth.go:22-42` `oauthProviderEntry` + `oauthProviders`, `:52-86`
  `runLogin`, `:88-120` `resolveLoginProvider`, `:125` `interactiveCallbacks`.
- `cmd/pi-ai/main.go:43-50` — the `login` dispatch and where the store is
  constructed.
- `cmd/pi-ai/oauth_test.go` — the existing CLI login tests to extend.
- `ai/providers/all.go` — the binding list the derivation walks.
- `ai/auth.go:264-270` `OAuthAuth` (`Name`, `LoginLabel` after epic 6 issue 01).
- Upstream at `936aff00`: `src/cli.ts`'s login selector, and
  `src/providers/radius-config.ts`'s `DEFAULT_RADIUS_GATEWAY`.

## Dependencies

- **Blocked by**: issues [01](/epic-7-four-new-oauth-flows/issues/01-xai-device-code-oauth.md),
  [02](/epic-7-four-new-oauth-flows/issues/02-kimi-coding-device-code-oauth.md),
  [03](/epic-7-four-new-oauth-flows/issues/03-openrouter-pkce-oauth.md),
  [04](/epic-7-four-new-oauth-flows/issues/04-radius-gateway-oauth.md) — all
  four strategies must exist; [epic 6](/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md)
  issues 01 (`LoginLabel`) and 06 (`Models.Login` owning persistence).
- **Blocks**: Nothing. [Issue 06](/epic-7-four-new-oauth-flows/issues/06-auth-docs-and-porting.md)
  documents the CLI table this issue makes true, so land it first if both are
  ready.

## PR size note

`S` — ~170 changed lines: `oauthProviders()` (`cmd/pi-ai/oauth.go:36-42`)
becomes a registry walk plus the Radius special case, a sort and a
`LoginLabel`-else-`Name` fallback — about 40 lines of source — and the rest is
five named tests extending the existing `cmd/pi-ai/oauth_test.go` harness
rather than building one. Split past ~200.
