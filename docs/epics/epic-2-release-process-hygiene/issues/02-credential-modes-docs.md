---
type: Issue
title: "Document credential-mode ToS risk"
description: "Docs-only: add a credential-modes note (API-key vs subscription-OAuth risk) to the README Authentication section and docs/auth.md."
tags: [epic-2]
timestamp: 2026-07-09T01:22:02Z
resource: https://github.com/julienlegoux/kern-proxy/issues/96
epic: 2
issue: 2
slug: credential-modes-docs
size: S
status: open
gh_issue: 96
depends_on: []
---

# Document credential-mode ToS risk

## Summary

The README documents OAuth login (`README.md:95`, Authentication section) with
no terms-of-service caveat. The adoption review flagged that consumers can't
assess the risk of the subscription-OAuth paths without reading upstream
context. Docs-only fix.

## Scope

Add a short "credential modes" note in both places:

- `README.md` Authentication section (line ~95)
- `docs/auth.md` (the auth & credentials doc — keep it conformant with the docs OKF bundle conventions)

Content, per the plan:

- API-key paths carry no ToS risk.
- Subscription-OAuth paths (Claude Pro/Max, ChatGPT Plus/Pro, GitHub Copilot) impersonate first-party clients — inherited upstream behavior, fine for personal use, account-revocation risk if shipped in a product.

## Out of scope

- Any code or behavior change to the OAuth flows themselves.
- The session round-trip docs — that's Epic 1's issue 08 (#94).

## Acceptance criteria / Definition of done

- Both `README.md` Authentication and `docs/auth.md` carry the credential-modes note with the API-key / subscription-OAuth distinction and the account-revocation caveat.
- Wording is consistent between the two places (one can be a condensed pointer to the other).

## Relevant files / areas

- `README.md` (Authentication section, ~line 95)
- `docs/auth.md`

## Dependencies

None within this epic; blocks [Issue 03](./03-tag-v0-1-0.md). Part of
[Epic 2](/epic-2-release-process-hygiene/EPIC_2.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.
