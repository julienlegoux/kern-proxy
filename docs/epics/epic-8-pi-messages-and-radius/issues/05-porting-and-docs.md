---
type: Issue
title: "Disposition pi-messages and radius in docs/PORTING.md, and document the tenth adapter"
description: "Add the mapping rows and deviations for pi-messages.ts, pi-messages.lazy.ts, radius.ts and radius-config.ts, give radius its auth-doc entries, and update every place that still counts nine adapters and 35 providers."
tags: [epic-8]
timestamp: 2026-08-11T20:00:00Z
epic: 8
issue: 05
slug: porting-and-docs
size: S
status: open
gh_issue: 191
resource: https://github.com/kern-ia/kern-link/issues/191
depends_on: ["01", "02", "03", "04"]
---

# Disposition pi-messages and radius in docs/PORTING.md, and document the tenth adapter

## Summary

Epic 8's acceptance criterion 4 — every ported file dispositioned in
`docs/PORTING.md` — and the reader-facing half of criteria 1 and 3. This is one
PR rather than four table edits fighting over the same file, the same split
epics 6 and 7 use
([epic 7 issue 06](/epic-7-four-new-oauth-flows/issues/06-auth-docs-and-porting.md)).

Radius is also the first provider whose credential mode does not fit the
existing story in `docs/auth.md`. The three OAuth flows documented there are
consumer subscriptions with a real account-revocation risk (`docs/auth.md:131-161`,
framing SPECS calls load-bearing). A Radius gateway credential is not that: it
is a first-party credential for a backend the user runs or subscribes to
directly, which is why upstream sets neither `isSubscription` nor a login label
on it. Say that explicitly rather than letting radius inherit the subscription
warning by proximity.

The counts to fix are real claims in prose, not decoration:
`docs/architecture.md:22-23` lists the nine adapter packages by name,
`ai/doc.go:2` and `README.md:7` say "35 providers". Epic 6 issue 10 also moves
the provider number; take the count as it stands when this PR opens rather than
assuming 36.

## Scope

- `docs/PORTING.md` — file-mapping rows, in the table's existing style:
  - `src/api/pi-messages.ts` → `ai/apis/pimessages` — ported. Note in the row
    what differs: no retry wrapper (upstream issues a single `fetch`, so
    `MaxRetries` is ignored by this adapter alone), no `apis.TransformMessages`
    (the backend speaks pi's own model), and `StreamSimple` as a pass-through
    that does not clamp max tokens.
  - `src/api/pi-messages.lazy.ts` → covered by the existing
    `src/api/*.lazy.ts` "not ported" row; extend that row only if it does not
    already generalize.
  - `src/providers/radius.ts` → `ai/providers/radius.go` — ported; mention that
    it is the only binding taking options, and the only purely dynamic one with
    no static catalog entry (upstream `all.ts:50-52`).
  - `src/providers/radius-config.ts` → `ai/providers/radius_config.go` —
    ported, with `normalizeRadiusGatewayUrl` living in `ai/auth/oauth` instead
    (the import cycle recorded by
    [epic 7 issue 04](/epic-7-four-new-oauth-flows/issues/04-radius-gateway-oauth.md)).
  - `test/pi-messages.test.ts` → `ai/apis/pimessages/stream_test.go` +
    `events_test.go`, if the table tracks test files; otherwise skip rather
    than inventing a column.
  - Update the `src/providers/*.ts` bindings row's "~35 bindings" count.
- `docs/auth.md` — a `radius` row in the env-var table (`:34-49`) for
  `RADIUS_API_KEY`; and in the OAuth section (`:91-130`), radius alongside the
  flows epic 7 added, with the one-paragraph distinction above and a note that
  the gateway is configurable per binding
  (`RadiusProvider(&RadiusProviderOptions{Gateway: …})`). Coordinate with
  [epic 7 issue 06](/epic-7-four-new-oauth-flows/issues/06-auth-docs-and-porting.md):
  if it already added a radius OAuth entry, extend it rather than writing a
  second one.
- `docs/architecture.md:22-23` — add `pimessages` to the adapter list.
- `ai/doc.go`, `README.md` — the provider count, and a mention of pi-messages
  where the adapter set is enumerated.
- `docs/planning/SPECS.md` — the package map (`ai/apis/{…}`) and the "nine wire
  adapters" line. SPECS is a planning-bundle doc describing the code as mapped;
  updating it here keeps the next epic from planning against a stale package
  map. Touch only the lines this epic invalidates.
- `docs/planning/index.md:13` — the bundle-root bullet that copies
  `SPECS.md:4`'s description verbatim, including "across 35 providers … nine
  wire adapters over net/http"; update alongside `SPECS.md`.
- `docs/usage.md:213` — "catalog (~35 providers)". This epic's `## Out of
  scope` excludes only Radius examples in `docs/usage.md`, not the provider
  count, so the count still updates here.

## Out of scope

- `CHANGELOG.md` and the upstream lock bump —
  [epic 9](/epic-9-classifier-audit-and-release/EPIC_9.md) owns the release.
- The full 232-file disposition sweep — also epic 9. This PR dispositions
  **only** this epic's four upstream files.
- `docs/usage.md` examples against a Radius gateway. There is no public
  endpoint to write a runnable example against; a fabricated one would be worse
  than none.
- Any code change. If writing a row turns up a behavior the code does not have,
  file it rather than fixing it here.
- `docs/planning/DRIFT.md` — the register is written at epic close by
  `close-epic`, from the drift records issues leave behind, not by an issue.

## Acceptance criteria / Definition of done

- [ ] `grep -c "pi-messages\|radius" docs/PORTING.md` returns at least 4, and
      every one of upstream's four files in this epic's range
      (`src/api/pi-messages.ts`, `src/api/pi-messages.lazy.ts`,
      `src/providers/radius.ts`, `src/providers/radius-config.ts`) appears in
      the mapping table exactly once, each with a status of `ported` or
      `not ported` plus a reason.
- [ ] `docs/auth.md`'s env table has a `radius` row naming `RADIUS_API_KEY`,
      and the value matches the binding's `auth.EnvAPIKeyAuth` list in
      `ai/providers/radius.go` — no divergence between doc and code.
- [ ] `docs/auth.md` states, in its own paragraph, that a Radius gateway
      credential is not a consumer subscription and does not carry the
      revocation risk described at `:131-161`.
- [ ] `docs/architecture.md`'s adapter diagram lists ten packages including
      `pimessages`.
- [ ] No file this issue owns still claims nine wire adapters or 35 providers:
      `grep -rn "nine wire adapters\|35 providers" README.md ai/doc.go
      docs/architecture.md docs/auth.md docs/usage.md docs/planning/SPECS.md
      docs/planning/index.md` returns nothing (the provider count matches
      `len(providers.Providers())`). `docs/planning/scope/` and `docs/epics/`
      describe the repo as it was when the plan was written; they are
      historical and exempt from this sweep.
- [ ] `docs/planning/SPECS.md`'s package map and adapter-count line match the
      code after this epic.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2) — a docs PR still has to be green.
- [ ] Conventional Commit, e.g.
      `docs: disposition pi-messages and radius in the porting map`.

## Relevant files / areas

- `docs/PORTING.md:21-64` — the file-mapping table, including the
  `src/api/*.lazy.ts` row (`:38`), the `src/auth/helpers.ts` row (`:42`) that
  already records `lazyOAuth`'s absence, and the `src/providers/*.ts` row
  (`:57`) with its "~35 bindings" count (`:48` is `anthropic-messages`, `:59`
  is `env-api-keys.ts`).
- `docs/auth.md:34-49` the env-var table, `:76` the codex-only note, `:91-130`
  the OAuth CLI section, `:131-161` the subscription-risk section.
- `docs/architecture.md:22-23` — the adapter list.
- `ai/doc.go:2`, `README.md:7` — the provider count.
- `docs/planning/SPECS.md`, "Architecture" — the package map and the "nine wire
  adapters" line.
- Upstream at `936aff00`: `src/api/pi-messages.ts`, `src/api/pi-messages.lazy.ts`,
  `src/providers/radius.ts`, `src/providers/radius-config.ts`.

## Dependencies

- **Blocked by**: [Issues 01](/epic-8-pi-messages-and-radius/issues/01-pimessages-wire-and-converter.md),
  [02](/epic-8-pi-messages-and-radius/issues/02-pimessages-stream-entry.md),
  [03](/epic-8-pi-messages-and-radius/issues/03-radius-gateway-config.md),
  [04](/epic-8-pi-messages-and-radius/issues/04-radius-provider-binding.md) —
  the rows describe files that must exist.
- **Blocks**: [Epic 9](/epic-9-classifier-audit-and-release/EPIC_9.md)'s
  disposition sweep, which expects this epic's files already accounted for.

## PR size note

`S` — ~150 changed lines: it touches eight files, but only five
`docs/PORTING.md` rows and one `docs/auth.md` paragraph plus a `RADIUS_API_KEY`
row are new prose; the rest are one- or two-line count and enumeration fixes
(`docs/architecture.md:22-23`, `ai/doc.go:2`, `README.md:7`,
`docs/planning/SPECS.md`, `docs/planning/index.md:13`, `docs/usage.md:213`).
Split past ~200.
