---
type: Issue
title: "Add the Bedrock ConverseStream adapter"
description: "Bedrock adapter on aws-sdk-go-v2 bedrockruntime ConverseStream with thinking payloads — the one transport that deliberately uses an SDK."
tags: [epic-10]
timestamp: 2026-07-08T12:54:49Z
epic: 10
issue: 01
slug: bedrock-adapter
size: L
status: in-progress
gh_issue: 35
resource: https://github.com/julienlegoux/kern-proxy/issues/35
depends_on: []
blocked_by: ["#18", "#19"]
---

# Add the Bedrock ConverseStream adapter

## Summary

Port the Bedrock adapter ([Epic 10](/epic-10-bedrock/EPIC_10.md)) on `aws-sdk-go-v2` bedrockruntime `ConverseStream` — the one transport where an SDK is used deliberately (SigV4 + event-stream framing). Thinking payloads included; the full AWS auth matrix lands separately in [Issue 02](./02-bedrock-aws-auth-matrix.md).

## Scope

- New `ai/apis/bedrock` package: Converse request building (messages, tools, inference config) and `ConverseStream` event decode to unified events.
- Thinking payloads (reasoning content blocks) encode/decode.
- Default credential-chain auth only in this issue (enough to run the adapter); the matrix comes next.
- TDD gate: port the upstream `bedrock-*` tests first against a mocked bedrockruntime client; goldens for the Converse input shape.
- `// Ports:` headers; flip the matching PORTING.md rows.

## Out of scope

- The full AWS auth matrix including bearer-token mode and ambient sentinels — [Issue 02](./02-bedrock-aws-auth-matrix.md).
- Provider binding/catalog entry — [Epic 11](/epic-11-catalog-all-providers/EPIC_11.md).

## Acceptance criteria / Definition of done

- `bedrock-*` test ports pass with matching golden Converse inputs; thinking payloads round-trip.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated; `aws-sdk-go-v2` added to `go.mod`.

## Relevant files / areas

- New: `ai/apis/bedrock/` — path follows the plan's layout; no adapter code exists yet.
- Read-only context: `ai/events.go`, `ai/stream.go`, `ai/providers/faux`.

## Dependencies

Blocks [Issue 02](./02-bedrock-aws-auth-matrix.md). Epic-level: needs epics 1–4.

## PR size note

Sized L: Converse encode + stream decode + mocked-client test harness form one unit. If hand-written code approaches ~1000 lines, split thinking payloads into a follow-up.
