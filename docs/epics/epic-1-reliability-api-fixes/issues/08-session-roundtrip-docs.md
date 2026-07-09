---
type: Issue
title: "Document session serialize/restore round-tripping"
description: "Docs-only: add a serialize/deserialize-a-session example to docs/usage.md and a pointer from the README quick start."
tags: [epic-1]
timestamp: 2026-07-09T01:20:16Z
resource: https://github.com/julienlegoux/kern-proxy/issues/94
epic: 1
issue: 8
slug: session-roundtrip-docs
size: S
status: open
gh_issue: 94
depends_on: []
---

# Document session serialize/restore round-tripping

## Summary

Serializing and restoring a session already works (`ai.Messages`,
`Context.UnmarshalJSON`, `ai/json.go:280-309`) but is undiscoverable — the
adoption review's external consumer had to read the source to find
`ai.Messages`. Docs-only fix.

## Scope

- Add a serialize/deserialize-a-session example to `docs/usage.md`: marshal a conversation's messages, persist, restore into a new `Context`, continue the conversation.
- Add a one-line pointer from the README quick start to that section.
- The example code must compile — verify it against the real API (consider sourcing it from a testable snippet).

## Out of scope

- Any API change to make round-tripping "nicer" — the mechanism works; this is discoverability only.
- The credential-modes docs — that's Epic 2 ([2.2](/epic-2-release-process-hygiene/EPIC_2.md)).

## Acceptance criteria / Definition of done

- `docs/usage.md` contains a working serialize/restore example using `ai.Messages` and `Context.UnmarshalJSON`.
- README quick start links to it.
- Example verified to compile against the current API (note: if issue [07](./07-message-pointer-receivers.md) lands first, the example must use pointer construction).
- `docs/usage.md` stays conformant with the docs OKF bundle conventions.

## Relevant files / areas

- `docs/usage.md`, `README.md`
- `ai/json.go:280-309`, `ai/json_test.go` (reference for the example)

## Dependencies

None hard; soft ordering after [Issue 07](./07-message-pointer-receivers.md) so
the example is written against the final receiver forms. Part of
[Epic 1](/epic-1-reliability-api-fixes/EPIC_1.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.
