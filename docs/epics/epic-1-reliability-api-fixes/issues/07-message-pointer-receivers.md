---
type: Issue
title: "Unify Message implementations on pointer receivers"
description: "Make UserMessage and ToolResultMessage implement Message via pointer receivers like AssistantMessage already does; update construction sites and tests."
tags: [epic-1]
timestamp: 2026-07-09T13:50:00Z
resource: https://github.com/julienlegoux/kern-proxy/issues/93
epic: 1
issue: 7
slug: message-pointer-receivers
size: S
status: done
gh_issue: 93
gh_pr: 104
depends_on: []
---

# Unify Message implementations on pointer receivers

## Summary

`ai/types.go:195-244`: `UserMessage` and `ToolResultMessage` implement
`Message` by value while `AssistantMessage` uses a pointer receiver.
Unify all three on pointer receivers — `AssistantMessage` is mutated during
streaming and its `MarshalJSON` is already on the pointer, so the pointer
convention is the one to keep. Pre-v1 breaking change, mechanical.

## Scope

- Move `UserMessage` and `ToolResultMessage` method sets (including any `MarshalJSON`/interface methods) to pointer receivers in `ai/types.go`.
- Update every construction site that stores the value type into a `Message` (message-history assembly in `ai/json.go`, adapters' transform paths, tests, testbed/examples) to use `&UserMessage{...}` / `&ToolResultMessage{...}`.
- Verify JSON round-tripping (`ai.Messages`, `Context.UnmarshalJSON`, `ai/json.go:280-309`) still decodes into the pointer forms.

## Out of scope

- Any semantic change to message marshaling or the `Message` interface itself.
- Restructuring `StreamOptions` or other port-fidelity surfaces — project-wide non-goal.

## Acceptance criteria / Definition of done

- All three message types implement `Message` via pointer receivers.
- All construction sites and tests updated; no value-type `Message` stores remain (`go vet` and the compiler enforce most of this).
- JSON serialize/deserialize round-trip tests still pass.
- `go test -race ./...` clean.

## Relevant files / areas

- `ai/types.go` (the three message types), `ai/json.go` / `ai/json_test.go`
- Adapters under `ai/apis/*` and `ai/apis/transform.go` where messages are constructed
- `testbed/`, examples

## Dependencies

None within this epic, but it touches many files — coordinate merge order with
in-flight retry PRs (02–05) to keep rebases small. Part of
[Epic 1](/epic-1-reliability-api-fixes/EPIC_1.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.
