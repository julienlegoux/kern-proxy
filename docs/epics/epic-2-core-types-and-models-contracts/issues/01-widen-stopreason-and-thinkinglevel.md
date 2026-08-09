---
type: Issue
title: "Widen StopReason and ThinkingLevel, and sweep for non-exhaustive switches"
description: "Add the pending/deferred stop reasons and the max thinking level, wire them into the thinking ladder and clamps, and record the manual sweep for switch sites the compiler cannot flag."
tags: [epic-2]
timestamp: 2026-08-09T04:31:17Z
epic: 2
issue: 01
slug: widen-stopreason-and-thinkinglevel
size: M
status: open
gh_issue: 129
resource: https://github.com/kern-ia/kern-link/issues/129
depends_on: []
---

# Widen StopReason and ThinkingLevel, and sweep for non-exhaustive switches

## Summary

Upstream `936aff00` widens two string unions the whole library switches on:

```ts
export type StopReason = "pending" | "stop" | "length" | "toolUse" | "error" | "aborted" | "deferred";
export type ThinkingLevel = "minimal" | "low" | "medium" | "high" | "xhigh" | "max";
```

Both are string-typed constants in Go, so adding values is additive and
*nothing* — not the compiler, not `golangci-lint` — will flag a `switch` that
silently stops being exhaustive. `exhaustive` is deliberately absent from
`.golangci.yml` ([CONVENTIONS.md](../../../planning/CONVENTIONS.md), "Linting is
deliberately minimal"), and this is named risk 3 of the program
([decision 21](../../../planning/scope/21-risks-and-assumptions.md), via
[decision 04](../../../planning/scope/04-core-types-migration.md)).

That is why the two enums and the manual sweep ship together as the epic's first
PR: every later issue in this epic adds code that switches on these values, and
the sweep is only meaningful against the pre-existing tree.

## Scope

- `ai/types.go:37-41` — add `StopReasonPending StopReason = "pending"` and
  `StopReasonDeferred StopReason = "deferred"`, keeping upstream's declaration
  order (`pending`, `stop`, `length`, `toolUse`, `error`, `aborted`,
  `deferred`).
- `ai/model.go:8-14` — add `ThinkingMax ThinkingLevel = "max"`.
- `ai/cost.go:24-27` — append `ThinkingMax` to `extendedThinkingLevels` after
  `ThinkingXHigh`, matching upstream's `EXTENDED_THINKING_LEVELS`.
- `ai/cost.go:33` `GetSupportedThinkingLevels` — `"max"` now requires an
  explicit `ThinkingLevelMap` entry exactly as `"xhigh"` does (upstream:
  `if (level === "xhigh" || level === "max") return mapped !== undefined;`).
- `ai/apis/simpleopts.go:49` `ClampReasoning` — `"xhigh"` **or** `"max"` clamps
  to `"high"` (upstream `clampReasoning` now excludes both).
- `ai/events.go:124-131` — `DoneEvent.Reason`'s doc comment must admit
  `"deferred"` alongside `stop`/`length`/`toolUse`; upstream widened the `done`
  event's `Extract<StopReason, ...>`. `ErrorEvent` is unchanged
  (`error`/`aborted`).
- **The sweep** — grep the whole tree for every site that switches on, compares,
  or enumerates `StopReason` and `ThinkingLevel`, decide per site whether the new
  values need handling, and record the result (see Acceptance criteria).

## Out of scope

- **Emitting** `pending` or `deferred` — nothing produces them until
  [issue 10](/epic-2-core-types-and-models-contracts/issues/10-deferred-response-dispatch.md)
  and
  [issue 12](/epic-2-core-types-and-models-contracts/issues/12-faux-deferred-responses.md).
  This PR ships the values and the switch audit, and says so in its body.
- `AnthropicEffortMax` (`ai/options.go:57`) already exists and is a different,
  adapter-specific enum — leave it alone.
- Catalog `thinkingLevelMap` entries that would actually enable `"max"` on a
  model — the catalog is regenerated in
  [Epic 3](/epic-3-catalog-schema-and-export-tooling/EPIC_3.md).
- Adapter-side mapping of the new levels onto vendor request fields — epics 4
  and 5.

## Acceptance criteria / Definition of done

- [ ] `go doc ./ai StopReason` lists seven constants including
      `StopReasonPending` and `StopReasonDeferred`; `go doc ./ai ThinkingLevel`
      shows `ThinkingMax`.
- [ ] A named test proves the ladder: `TestGetSupportedThinkingLevelsRequiresExplicitMaxEntry`
      — a reasoning model with no `ThinkingLevelMap` entry for `"max"` does not
      list `"max"`; one with an explicit entry does.
- [ ] A named test proves the clamp: `TestClampReasoningMapsMaxToHigh` —
      `apis.ClampReasoning(ai.ThinkingMax)` returns `ai.ThinkingHigh`.
- [ ] A named test proves `"max"` participates in `ClampThinkingLevel`'s
      upward/downward walk (`ai/cost.go:54`) rather than falling through to
      `available[0]`.
- [ ] `pending`/`deferred` round-trip through the event and message codecs:
      `go test ./ai -run TestJSON` stays green with a case asserting
      `StopReason("deferred")` survives marshal→unmarshal.
- [ ] **The sweep is performed and recorded.** Run at minimum
      `grep -rn "StopReason\|ThinkingLevel" --include=*.go .` and review every
      hit that branches on a value. Post the resulting site list — each marked
      *handled* or *deliberately unchanged, because …* — as a comment on the
      epic issue [#119](https://github.com/kern-ia/kern-link/issues/119) **and**
      in the PR body. A sweep whose result lives only in a merged branch's diff
      does not satisfy this.
- [ ] `go build ./...` and `GOTMPDIR=$PWD/.gotmp go test ./...` pass locally
      (`-race` is CI-only — no local C toolchain).
- [ ] CI green: `go test ./... -race -v`, `bash upstream/sync_test.sh`,
      `golangci-lint` v2.12.2.
- [ ] `gofmt -l .` prints nothing; every touched ported file keeps its
      `// Ports:` header.
- [ ] Conventional Commit with a scope, e.g.
      `feat(ai)!: add the pending/deferred stop reasons and the max thinking level`.

## Relevant files / areas

- `ai/types.go:33-41` — `StopReason` and its constants.
- `ai/model.go:5-25` — `ThinkingLevel`, `ModelThinkingLevel`, `ThinkingOff`.
- `ai/cost.go:24-94` — `extendedThinkingLevels`, `GetSupportedThinkingLevels`,
  `ClampThinkingLevel`.
- `ai/apis/simpleopts.go:49` — `ClampReasoning`.
- `ai/events.go:124-137` — `DoneEvent` / `ErrorEvent` reason contracts.
- `ai/json.go` — event and message codecs (`StopReason` is a plain string, so no
  codec change is expected; assert it rather than assume it).
- Upstream sources for this issue, at the frozen revision:
  ```bash
  gh api "repos/earendil-works/pi/contents/packages/ai/src/types.ts?ref=936aff00918de1187f085f123c2812d8f2d67745" -H "Accept: application/vnd.github.raw"
  gh api "repos/earendil-works/pi/contents/packages/ai/src/models.ts?ref=936aff00918de1187f085f123c2812d8f2d67745" -H "Accept: application/vnd.github.raw"
  ```

## Dependencies

- **Blocked by**: None — first issue of the epic. (Epic 2 as a whole waits on
  [Epic 1](/epic-1-repository-hygiene/EPIC_1.md)'s module-path move.)
- **Blocks**: [Issue 02](/epic-2-core-types-and-models-contracts/issues/02-message-model-deferred-fields.md),
  [Issue 06](/epic-2-core-types-and-models-contracts/issues/06-simple-options-and-lazy.md),
  [Issue 10](/epic-2-core-types-and-models-contracts/issues/10-deferred-response-dispatch.md),
  [Issue 12](/epic-2-core-types-and-models-contracts/issues/12-faux-deferred-responses.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR. Expected here: a few dozen lines of production code plus tests — the sweep
is the work, not the diff.
