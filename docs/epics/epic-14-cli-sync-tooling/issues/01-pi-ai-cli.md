---
type: Issue
title: "Add the pi-ai CLI (login, list, help)"
description: "cmd/pi-ai with login/list/help plus the end-to-end example streaming a tool-call round-trip through faux."
tags: [epic-14]
timestamp: 2026-07-07T08:28:06Z
epic: 14
issue: 01
slug: pi-ai-cli
size: M
status: open
gh_issue: 44
resource: https://github.com/julienlegoux/kern-proxy/issues/44
depends_on: []
blocked_by: ["#12", "#13"]
---

# Add the pi-ai CLI (login, list, help)

## Summary

Build the user-facing surface of [Epic 14](/epic-14-cli-sync-tooling/EPIC_14.md): the `cmd/pi-ai` CLI with `login`, `list`, and `help`, plus the small end-to-end example program that streams a tool-call round-trip through the faux provider (and a real provider when a key is present) — the epic's e2e acceptance criterion.

## Scope

- `cmd/pi-ai`: `login` (drives the Epic 12 flows through `AuthLoginCallbacks`, persisting via the Epic 3 file store), `list` (catalog + provider listing from Epic 11), `help`.
- The e2e example: stream a tool-call round-trip through `ai/providers/faux`; env-gated variant against a real provider.
- Ports `src/cli.ts` per the PORTING.md mapping (`cmd/pi-ai`, phase 14).
- `// Ports:` headers; flip the PORTING.md row.

## Out of scope

- Sync tooling and PORTING.md completion — [Issue 02](./02-upstream-sync-porting-md.md).
- New CLI features beyond upstream parity.

## Acceptance criteria / Definition of done

- `pi-ai list` prints the catalog; `pi-ai login` completes a flow end to end (manual check); `pi-ai help` documents both.
- The e2e example streams a faux tool-call round-trip in tests; env-gated real-provider run works with a key set.
- `go test ./...` green; `// Ports:` header present; PORTING.md updated.

## Relevant files / areas

- New: `cmd/pi-ai/` — path follows the plan's layout.
- Read-only context: `ai/catalog`, `ai/providers` (Epic 11), `ai/auth/oauth` (Epic 12), `ai/providers/faux`.

## Dependencies

None within this epic, but cross-epic: `login` needs [Epic 12](/epic-12-oauth-flows/EPIC_12.md), `list` needs [Epic 11](/epic-11-catalog-all-providers/EPIC_11.md), the e2e needs the adapters (epics 5–10).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.
