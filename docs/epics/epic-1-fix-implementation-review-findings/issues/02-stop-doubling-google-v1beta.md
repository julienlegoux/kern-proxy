---
type: Issue
title: "Stop doubling the Google /v1beta path segment"
description: "Normalize requestURL in ai/apis/google/google.go to strip a trailing /v1beta so Google requests stop 404ing on a doubled path segment; drop the smoke-test workaround."
tags: [epic-1]
timestamp: 2026-07-08T22:00:00Z
epic: 1
issue: 02
slug: stop-doubling-google-v1beta
size: S
status: done
gh_issue: 78
gh_pr: 81
resource: https://github.com/julienlegoux/kern-proxy/issues/78
depends_on: []
---

# Stop doubling the Google /v1beta path segment

## Summary
Both `ai/catalog/data/models/google.json:7` and `ai/providers/google.go:18` set the
base URL `https://generativelanguage.googleapis.com/v1beta`, while
`ai/apis/google/google.go:438-444` `requestURL` appends `/v1beta/models/...` again,
producing `.../v1beta/v1beta/models/...` → 404. Today `live_smoke_test.go:34` only
passes via a per-request `model.BaseURL` override workaround. This PR normalizes the
path in `requestURL` and removes the workaround. This is Fix 2 (CRITICAL) of the epic.

## Scope
- `ai/apis/google/google.go` `requestURL` — after
  `base := strings.TrimRight(model.BaseURL, "/")`, strip a trailing `/v1beta`
  (`base = strings.TrimSuffix(base, "/v1beta")`) before the
  `fmt.Sprintf(".../v1beta/models/%s...")`. Add a short comment explaining why.
- `cmd/pi-ai/toolcallexample/live_smoke_test.go:34-41` — remove the `model.BaseURL`
  override workaround and its comment; use the catalog model as-is.

Chosen approach (normalize in `requestURL`) is verified safe: in the non-vertex google
package `model.BaseURL` is read *only* at `google.go:439` inside `requestURL`; nothing
else consumes it. Vertex's separate `customHost` logic is untouched. Trimming a
trailing `/v1beta` is robust whether or not the base URL carries the segment, touches
one file plus its test, and edits no catalog data.

## Out of scope
- Editing `google.json` or `ai/providers/google.go` base URLs (the fix is in
  `requestURL`, not the data).
- Vertex `customHost` logic.
- The OAuth wiring and `// Ports:` header pass (separate issues in this epic).

## Acceptance criteria / Definition of done
- TDD, test-first (red → green), per repo convention.
- `ai/apis/google/google_test.go` — a `requestURL` case asserting a base URL of
  `https://generativelanguage.googleapis.com/v1beta` yields a single
  `/v1beta/models/<id>:streamGenerateContent?alt=sse` segment.
- A regression test through the real catalog:
  `providers.Models(nil).GetModel("google", ...)` → build request URL → assert no
  `/v1beta/v1beta`. Place it where it can import `providers` without an import cycle
  into `ai/apis/google` (e.g. under `cmd/pi-ai/toolcallexample` or a provider-level
  test).
- The live smoke test passes **without** the removed workaround (run if creds
  available; otherwise the unit-level regression test stands in).
- `go build ./...`, `go test ./... -race`, and `go vet ./...` clean.

## Relevant files / areas
- `ai/apis/google/google.go` (`requestURL`, ~lines 438-444)
- `ai/apis/google/google_test.go`
- `cmd/pi-ai/toolcallexample/live_smoke_test.go` (lines 34-41)
- Reference only (not edited): `ai/catalog/data/models/google.json`,
  `ai/providers/google.go`

## Dependencies
None. Independent of [Issue 01](./01-wire-provider-oauth-bindings.md) and
[Issue 03](./03-add-ports-headers-source-files.md). Part of
[Epic 1](/epic-1-fix-implementation-review-findings/EPIC_1.md).

## PR size note
Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.
Expected S — one-line normalization plus two focused tests and a workaround removal.
