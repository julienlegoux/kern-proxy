---
type: Issue
title: "Add the Vertex variant with ADC auth"
description: "Vertex endpoints over the shared Google converters, with Application Default Credentials via golang.org/x/oauth2/google and the ambient ADC sentinel."
tags: [epic-8]
timestamp: 2026-07-08T08:20:00Z
epic: 8
issue: 02
slug: vertex-variant-adc
size: M
status: pr-open
gh_issue: 33
gh_pr: 63
resource: https://github.com/julienlegoux/kern-proxy/issues/33
depends_on: [01]
---

# Add the Vertex variant with ADC auth

## Summary

Port the Vertex AI variant over [Issue 01](./01-google-adapter.md)'s shared converters: Vertex endpoint/project/region request shape plus the Application Default Credentials auth path via `golang.org/x/oauth2/google`, honoring the ambient **Vertex ADC sentinel** that Epic 3's env-key map already defines.

## Scope

- `ai/apis/google/vertex`: Vertex endpoint shaping over the shared converters.
- ADC token sourcing via `golang.org/x/oauth2/google`; wire the ambient Vertex ADC sentinel from the auth env-key map (`ai/resolve.go` side is done — this consumes it).
- TDD gate: port the upstream Vertex tests first, goldens via `OnPayload`.
- `// Ports:` headers; flip the matching PORTING.md rows.

## Out of scope

- Google converter changes — [Issue 01](./01-google-adapter.md).
- Provider bindings/catalog entries — [Epic 11](/epic-11-catalog-all-providers/EPIC_11.md).

## Acceptance criteria / Definition of done

- Vertex test ports pass with matching goldens; ADC resolution covered with a fake token source (no live GCP needed).
- Env-gated live smoke where applicable.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- New: `ai/apis/google/vertex/`; new dep `golang.org/x/oauth2` in `go.mod`.
- Read-only context: `ai/resolve.go` / `ai/resolve_test.go` (ambient sentinel behavior).

## Dependencies

Blocked by [Issue 01](./01-google-adapter.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.
