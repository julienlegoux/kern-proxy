---
type: Issue
title: "Port the 0.84.1 message-model additions: DeferredHandle, JsonValue, and the new message fields"
description: "Add DeferredHandle and JsonValue plus the new AssistantMessage, ToolResultMessage, and ToolCall fields, with JSON round-trip coverage."
tags: [epic-2]
timestamp: 2026-08-09T04:31:17Z
epic: 2
issue: 02
slug: message-model-deferred-fields
size: M
status: open
gh_issue: 130
resource: https://github.com/kern-ia/kern-link/issues/130
depends_on: [1]
---

# Port the 0.84.1 message-model additions: DeferredHandle, JsonValue, and the new message fields

## Summary

`types.ts` at `936aff00` adds two new types and five new message fields. They
are small individually and inseparable in practice: every one of them travels
through `ai/json.go`, and a session saved by one version has to load in the
other — the unified message model *is* kern-link's persistence story
([SPECS.md](../../../planning/SPECS.md), "Data model & storage").

```ts
export type JsonValue = string | number | boolean | null | JsonValue[] | { [key: string]: JsonValue };

export interface DeferredHandle {
	provider: string;
	modelId: string;
	api: string;
	id: string;              // provider token: response id, or batch id + row id
	expiresAt?: number;
	pollAfterMs?: number;
	data?: JsonValue;        // provider conversion data to rebuild the final message
}
```

## Scope

- `ai/types.go` — add `JsonValue` and `DeferredHandle`. `JsonValue` is
  TypeScript's recursive JSON union; in Go the honest mapping is `any` holding
  decoded JSON (the same call `ChatTemplateKwargValue` already made at
  `ai/model.go:111`). Follow that precedent, and say so in the type's doc
  comment rather than inventing a sum type.
- `AssistantMessage` (`ai/types.go:194-206`) gains:
  - `Deferred *DeferredHandle` (`json:"deferred,omitempty"`),
  - `RawStopReason string` (`json:"rawStopReason,omitempty"`),
  - `EndTurn *bool` (`json:"endTurn,omitempty"`) — upstream's `endTurn?: boolean`
    is a tri-state (absent / true / false) and is documented as "preserved for
    debugging, does not affect control flow"; a plain `bool` loses the
    distinction.
- `ToolResultMessage` (`ai/types.go:229-237`) gains:
  - `Usage *Usage` (`json:"usage,omitempty"`) — usage from the tool execution
    itself, deliberately outside main LLM context accounting,
  - `AddedToolNames []string` (`json:"addedToolNames,omitempty"`) — names from
    `Context.Tools` that became available after this result. This is the input
    that [issue 11](/epic-2-core-types-and-models-contracts/issues/11-deferred-tools-split.md)
    reads.
- `ToolCall` gains `Namespace string` (`json:"namespace,omitempty"`) — the
  OpenAI Responses namespace for dynamically loaded tools.
- `AssistantMessage.Clone()` (`ai/types.go:212`) must keep doing the right thing
  with the new fields: `Deferred` is a pointer into shared state, so decide and
  document whether the clone shares or copies it, and cover the decision with a
  test.
- `ai/json.go` — extend the marshal/unmarshal paths and the existing round-trip
  tests to cover every new field.

## Out of scope

- **`Tool.constrainedSampling`, `GrammarFormat`, `GrammarVariants`,
  `ConstrainedSamplingConfig`** — also new in `types.ts` at this revision, but
  [decision 12](../../../planning/scope/12-constrained-sampling.md) puts them in
  [Epic 4](/epic-4-openai-family-adapters/EPIC_4.md) with the request builders
  that use them.
- **The `pi-messages` API constant and `PiMessagesOptions`** — new in `types.ts`
  too, owned by [Epic 8](/epic-8-pi-messages-and-radius/EPIC_8.md).
- **New provider IDs** (`radius`, `baseten`, `qwen-token-plan*`). Go's
  `ProviderId` is `= string` (`ai/types.go:22`) with no closed union to widen;
  the bindings belong to epics 6 and 8.
- Anything that *produces* a `DeferredHandle` — issues 10 and 12.
- Adapter-side population of `RawStopReason` / `EndTurn` — epics 4 and 5. This
  PR ships the fields and their codecs.

## Acceptance criteria / Definition of done

- [ ] `TestAssistantMessageRoundTripsDeferredHandle` — an `AssistantMessage`
      carrying a populated `DeferredHandle` (including `Data` holding a nested
      object) marshals and unmarshals to an equal value.
- [ ] `TestAssistantMessageOmitsUnsetDeferredFields` — marshalling a message
      with none of the new fields set produces JSON containing none of
      `deferred`, `rawStopReason`, `endTurn` (the wire shape must not change for
      existing consumers).
- [ ] `TestToolResultMessageRoundTripsAddedToolNamesAndUsage` covers both new
      `ToolResultMessage` fields; `TestToolCallRoundTripsNamespace` covers
      `ToolCall.Namespace`.
- [ ] `TestAssistantMessageCloneHandlesDeferred` asserts the documented clone
      semantics for `Deferred`.
- [ ] `EndTurn` distinguishes absent from `false` — a test asserts unmarshalling
      `{"endTurn":false}` yields a non-nil pointer to `false`, and omitting the
      key yields nil.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2).
- [ ] `gofmt -l .` prints nothing; `ai/types.go` and `ai/json.go` keep their
      `// Ports:` headers.
- [ ] Conventional Commit, e.g.
      `feat(ai): add DeferredHandle and the 0.84.1 message-model fields`.

## Relevant files / areas

- `ai/types.go:33-41` (StopReason, from
  [issue 01](/epic-2-core-types-and-models-contracts/issues/01-widen-stopreason-and-thinkinglevel.md)),
  `:186-254` (`UserMessage`, `AssistantMessage`, `ToolResultMessage`, `Tool`,
  `Context`).
- `ai/model.go:108-111` — `ChatTemplateKwargValue`, the existing precedent for
  "decoded JSON as `any`".
- `ai/json.go` (313 lines) — every `MarshalJSON`/`UnmarshalJSON` for the message
  model; `ai/json_test.go` is table-driven here by convention
  ([CONVENTIONS.md](../../../planning/CONVENTIONS.md), Testing).
- `ai/message_receivers_test.go` — locks messages onto pointer receivers; do not
  break it.
- Upstream: `src/types.ts` at `936aff00` (fetch recipe in
  [issue 01](/epic-2-core-types-and-models-contracts/issues/01-widen-stopreason-and-thinkinglevel.md)).

## Dependencies

- **Blocked by**: [Issue 01](/epic-2-core-types-and-models-contracts/issues/01-widen-stopreason-and-thinkinglevel.md)
  — `AssistantMessage.Deferred` is meaningless without `StopReasonDeferred`, and
  both PRs edit `ai/types.go`.
- **Blocks**: [Issue 10](/epic-2-core-types-and-models-contracts/issues/10-deferred-response-dispatch.md),
  [Issue 11](/epic-2-core-types-and-models-contracts/issues/11-deferred-tools-split.md),
  [Issue 12](/epic-2-core-types-and-models-contracts/issues/12-faux-deferred-responses.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
