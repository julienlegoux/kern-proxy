---
type: Issue
title: "Add the full AWS auth matrix including bearer-token mode"
description: "All Bedrock auth modes — profiles, explicit keys, ambient sentinels, and bearer-token auth (plan risk #3: verify it's expressible in aws-sdk-go-v2)."
tags: [epic-10]
timestamp: 2026-07-07T08:25:39Z
epic: 10
issue: 02
slug: bedrock-aws-auth-matrix
size: M
status: open
gh_issue: 36
resource: https://github.com/julienlegoux/kern-proxy/issues/36
depends_on: [01]
---

# Add the full AWS auth matrix including bearer-token mode

## Summary

Complete [Epic 10](/epic-10-bedrock/EPIC_10.md) by porting the full AWS auth matrix onto [Issue 01](./01-bedrock-adapter.md)'s adapter: explicit keys, profiles, the ambient Bedrock AWS sentinels from Epic 3's env-key map, and **bearer-token auth**. This carries **plan risk #3**: first verify bearer-token auth is expressible in `aws-sdk-go-v2` — if it is not, hand-roll that one auth mode.

## Scope

- Wire every upstream Bedrock auth mode into the adapter's client construction: explicit access key/secret/session, profile, region resolution, and bearer token.
- Risk #3 spike (do this first): confirm `aws-sdk-go-v2` can express bearer-token auth for bedrockruntime; document the finding in the PR and hand-roll the HTTP auth for that mode only if the SDK cannot.
- Consume the ambient Bedrock AWS sentinels from the env-key map (`ai/resolve.go` side is done — this issue consumes the resolved values).
- TDD gate: port the upstream Bedrock auth tests first — one case per matrix cell.

## Out of scope

- Converse encode/decode — [Issue 01](./01-bedrock-adapter.md).
- Changes to the env-key map itself — Epic 3, already delivered.

## Acceptance criteria / Definition of done

- Auth-matrix test ports pass, covering every mode including bearer token.
- Risk #3 outcome recorded (SDK-native or hand-rolled, and why).
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- `ai/apis/bedrock/` (from Issue 01); `ai/resolve.go` / `ai/resolve_test.go` (ambient sentinel behavior, read-only).

## Dependencies

Blocked by [Issue 01](./01-bedrock-adapter.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.
