---
type: Issue
title: "Add // Ports: headers to the 16 non-test source files"
description: "Add top-of-file // Ports: upstream-source headers (no @ sha) to the 16 non-test .go files under ai/ and cmd/ that lack them, closing the traceability gap."
tags: [epic-1]
timestamp: 2026-07-08T21:40:00Z
epic: 1
issue: 03
slug: add-ports-headers-source-files
size: S
status: pr-open
gh_issue: 79
gh_pr: 82
resource: https://github.com/julienlegoux/kern-proxy/issues/79
depends_on: []
---

# Add // Ports: headers to the 16 non-test source files

## Summary
Sixteen non-test `.go` files under `ai/` and `cmd/` lack the `// Ports:` header the
repo mandates for upstream-source traceability. This PR adds a top-of-file
`// Ports: packages/ai/src/…` comment (upstream TS source only — **no `@ <sha>`**, no
file has ever carried it) to each, matching the format of siblings that already have
one (e.g. `ai/apis/bedrock/bedrock.go`:
`// Ports: packages/ai/src/api/bedrock-converse-stream.ts`). This is Fix 3
(traceability gap) of the epic and is comment-only.

## Scope
Add a `// Ports:` header to each of these 16 files, deriving the path from
`docs/archived/PORTING.md` rows and sibling files in the same package that already
carry headers:

- **Bedrock (7)** — `ai/apis/bedrock/`: `awsconvert.go`, `errors.go`, `helpers.go`,
  `messages.go`, `stream.go`, `thinking.go`, `wire.go` → `src/api/bedrock*.ts` /
  `src/api/bedrock-convert-messages.ts`.
- **Codex (1)** — `ai/apis/codex/websocket.go` → codex websocket portion of
  `src/api/codex*.ts`.
- **Google (1)** — `ai/apis/google/stream.go` → stream-decode portion of
  `src/api/google*.ts`.
- **Mistral (2)** — `ai/apis/mistral/messages.go`, `ai/apis/mistral/stream.go` →
  `src/api/mistral-conversations.ts`.
- `ai/internal/partialjson/partial.go` → `src/utils/json-parse.ts` + npm `partial-json`.
- `ai/json.go` → its upstream JSON util (confirm from file body / PORTING.md).
- `ai/providers/refreshmodels.go` → `refreshModels` provider machinery (confirm from
  PORTING.md).
- `cmd/pi-ai/example/main.go`, `cmd/pi-ai/toolcallexample/roundtrip.go` — likely
  original Go demo harnesses; if no upstream counterpart, add an explicit
  `// Ports: none — original <purpose>` note rather than inventing a mapping.

## Out of scope
- The 56 `_test.go` files (explicitly excluded).
- The `@ <sha>` suffix on headers — naming the upstream file is enough.
- Any behavior change — this PR is comment-only, no test changes.

## Acceptance criteria / Definition of done
- Each of the 16 files carries a top-of-file `// Ports:` header in the established
  format (bundle path under `packages/ai/src/…`, or the `none — original` form for
  genuine originals).
- Paths derived from `docs/archived/PORTING.md` and verified against sibling headers,
  not guessed.
- Grep confirms no non-test `.go` file under `ai/` or `cmd/` lacks a `// Ports:` header.
- `go build ./...`, `go test ./... -race`, and `go vet ./...` clean (comment-only, so
  no behavior should change).

## Relevant files / areas
- `ai/apis/bedrock/{awsconvert,errors,helpers,messages,stream,thinking,wire}.go`
- `ai/apis/codex/websocket.go`, `ai/apis/google/stream.go`
- `ai/apis/mistral/{messages,stream}.go`
- `ai/internal/partialjson/partial.go`, `ai/json.go`, `ai/providers/refreshmodels.go`
- `cmd/pi-ai/example/main.go`, `cmd/pi-ai/toolcallexample/roundtrip.go`
- Reference: `docs/archived/PORTING.md`, and existing headers such as
  `ai/apis/bedrock/bedrock.go` / `ai/apis/google/google.go`

## Dependencies
None. Independent of [Issue 01](./01-wire-provider-oauth-bindings.md) and
[Issue 02](./02-stop-doubling-google-v1beta.md). Part of
[Epic 1](/epic-1-fix-implementation-review-findings/EPIC_1.md).

## PR size note
Target ~500 changed lines; comment-only across 16 files, so well under. Expected S.
