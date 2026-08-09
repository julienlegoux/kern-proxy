---
type: Issue
title: "Port constrained-sampling.ts as a shared grammar package, with Tool.ConstrainedSampling"
description: "Add the GrammarFormat / GrammarVariants / ConstrainedSamplingConfig types on ai.Tool and port upstream's grammar resolution and JSON-delta buffer, with its 229-line test."
tags: [epic-4]
timestamp: 2026-08-09T05:17:46Z
epic: 4
issue: 01
slug: constrained-sampling-core
size: M
status: open
gh_issue: 147
resource: https://github.com/kern-ia/kern-link/issues/147
depends_on: []
---

# Port constrained-sampling.ts as a shared grammar package, with Tool.ConstrainedSampling

## Summary

`src/api/constrained-sampling.ts` (+148, new at `936aff00`) is the module all
four adapters in this epic call into. It is pure logic — no wire calls, no
provider knowledge — so it ports first and everything else in the epic builds on
it.

It provides five exported symbols:

- `resolveGrammarConstrainedSampling(tool, supportsOpenAIGrammarTools)` — picks
  `openai_lark` over `openai_regex`, infers the single required string property
  from the tool's JSON schema, returns `{format, definition, inputProperty}` or
  `undefined`.
- `resolveJsonSchemaStrictSampling(tool, supportsStrictMode)` — returns `true`
  when strict is supported, `undefined` when the config only *prefers* it, and
  **throws** when `strict: "require"` meets a provider that cannot do it.
- `createGrammarToolInputProperties(tools, supportsOpenAIGrammarTools)` — the
  name→inputProperty map every adapter threads through its converters.
- `getGrammarToolInput(toolName, arguments, inputProperty)` — reads the input
  string back out of a tool call's arguments, throwing if it is not a string.
- `appendGrammarToolInputJsonDelta(buffer, inputProperty, nextInput, close)` —
  synthesizes JSON deltas (`{"prop":"` … `"}`) from a provider's raw grammar
  output so a custom tool call streams like an ordinary JSON one.

**Correction to this epic's framing.** The epic body and
[decision 12](../../../planning/scope/12-constrained-sampling.md) both say
`ConstrainedSamplingConfig` "lands in `ai.StreamOptions`, continuing the
established per-adapter-options deviation". That was written before the upstream
shape was read. At `936aff00` the field is
`Tool.constrainedSampling?: false | ConstrainedSamplingConfig` (`src/types.ts`) —
per **tool**, not per request. So it goes on `ai.Tool` and **no deviation is
needed at all**; `StreamOptions` is not touched. Say so in the PR body and in
`docs/PORTING.md` so the next reader does not re-derive it.

## Scope

- `ai/types.go` — extend `Tool` (`:243-247`, currently `Name`, `Description`,
  `Parameters`):
  - `GrammarFormat string` with `GrammarFormatOpenAILark = "openai_lark"` and
    `GrammarFormatOpenAIRegex = "openai_regex"`.
  - `GrammarVariants map[GrammarFormat]string`.
  - `ConstrainedSamplingConfig` — upstream is a discriminated union of
    `{type:"json_schema", strict:"prefer"|"require"}` and
    `{type:"grammar", variants}`, and the field itself is
    `false | ConstrainedSamplingConfig`. Go has no union: model it as one struct
    with `Type`, `Strict`, `Variants` and make the field a **pointer**
    (`ConstrainedSampling *ConstrainedSamplingConfig json:"constrainedSampling,omitempty"`),
    so nil covers both "absent" and upstream's explicit `false`. If the
    marshalled form must be able to emit literal `false`, that needs a custom
    marshaller in `ai/json.go` — decide, implement one way, and document the
    choice in the type's doc comment.
  - Validate on decode what upstream validates at use time only if it is free;
    otherwise leave validation to the resolver below and say so.
- New package `ai/apis/internal/grammar` (importable from every `ai/apis/*`
  adapter, invisible to consumers — the placement rule in
  [CONVENTIONS.md](../../../planning/CONVENTIONS.md), "Naming & file layout"),
  with a `// Ports: packages/ai/src/api/constrained-sampling.ts` header:
  - `type Constrained struct { Format, Definition, InputProperty string }`
  - `ResolveGrammar(tool ai.Tool, supportsOpenAIGrammarTools bool) (*Constrained, error)`
  - `ResolveJSONSchemaStrict(tool ai.Tool, supportsStrictMode bool) (*bool, error)`
  - `InputProperties(tools []ai.Tool, supportsOpenAIGrammarTools bool) (map[string]string, error)`
  - `ToolInput(toolName string, arguments map[string]any, inputProperty string) (string, error)`
  - `type InputJSONBuffer struct { Input string; Started, Closed bool }` and
    `AppendInputJSONDelta(buf *InputJSONBuffer, inputProperty, nextInput string, close bool) (string, error)`
- **Errors, not panics.** Upstream throws in five places; Go returns errors.
  Keep the message text upstream-verbatim (`Tool "x" cannot use grammar
  constrained sampling: …`, `grammar tool input for property "p" changed
  non-monotonically`, …) — [CONVENTIONS.md](../../../planning/CONVENTIONS.md)
  makes error text load-bearing. The callers in issues 03/06/11 decide how a
  resolution error surfaces (in-band error event via `ai.LazyStream`).
- **Iteration order.** `InputProperties` returns a map; Go map iteration is
  random. Callers only ever do lookups by name, so a map is fine — but assert it
  in a test rather than leaving it implicit, and never range over it to build
  request payload order.
- Port `test/constrained-sampling.test.ts` (229 lines) as
  `ai/apis/internal/grammar/grammar_test.go`, discrete named functions per
  [CONVENTIONS.md](../../../planning/CONVENTIONS.md) ("Testing").
- `docs/PORTING.md` — add the `src/api/constrained-sampling.ts` →
  `ai/apis/internal/grammar` row, and record the `Tool.ConstrainedSampling`
  placement correction described above.

## Out of scope

- **Every adapter call site.** No adapter changes behavior in this PR; it may
  not even import the new package yet. Wiring is issues 03 (completions), 06
  (responses shared), 10 (azure), 11 (codex).
- The `supportsOpenAIGrammarTools` / `supportsStrictMode` compat flags
  themselves — [Epic 2 issue 04](/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md)
  ships those. This PR consumes them as parameters.
- Any non-OpenAI grammar format. Upstream defines exactly two.

## Acceptance criteria / Definition of done

- [ ] `TestResolveGrammarPrefersLarkOverRegex` — a tool carrying both variants
      resolves to `format: "lark"` with the lark definition.
- [ ] `TestResolveGrammarReturnsNilWhenUnsupported` — with
      `supportsOpenAIGrammarTools=false`, a grammar tool resolves to `(nil, nil)`,
      not an error.
- [ ] `TestResolveGrammarRejectsMultiPropertySchema` — a schema whose `required`
      is not exactly one string property errors, and the message matches
      upstream's `Tool "…" cannot use grammar constrained sampling: grammar
      constrained sampling requires exactly one required string property.`
- [ ] `TestResolveGrammarRejectsEmptyVariants` — whitespace-only definitions
      count as absent and produce upstream's "no supported grammar variant was
      provided" error.
- [ ] `TestResolveJSONSchemaStrictRequireErrorsWhenUnsupported` — `strict:
      "require"` + `supportsStrictMode=false` errors; `strict: "prefer"` in the
      same conditions returns `(nil, nil)`.
- [ ] `TestAppendInputJSONDeltaOpensClosesAndIsMonotonic` — the first call emits
      `{"prop":"`, subsequent calls emit only the escaped increment, the closing
      call appends `"}`, a re-close with identical input returns `("", nil)`, and
      a shrinking or diverging `nextInput` errors.
- [ ] `TestAppendInputJSONDeltaEscapesJSONControlCharacters` — an input
      containing `"`, `\`, and a newline produces a delta that concatenates into
      valid JSON (assert by unmarshalling the assembled buffer).
- [ ] `TestToolConstrainedSamplingRoundTrips` — a `Tool` with a grammar config
      marshals and unmarshals to an equal value, and a `Tool` without one emits
      no `constrainedSampling` key.
- [ ] `ai/apis/internal/grammar/grammar.go` carries the `// Ports:` header;
      `docs/PORTING.md` has the new mapping row and the placement note.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(apis): port grammar-constrained sampling resolution`.

## Relevant files / areas

- `ai/types.go:243-247` — the `Tool` struct.
- `ai/json.go` — where a custom marshaller would go if the `false` literal must
  survive.
- `ai/apis/internal/` — currently holds only `httpretry` (which
  [Epic 3 issue 06](/epic-3-catalog-schema-and-export-tooling/issues/06-images-adapter-retry.md)
  moves to `ai/internal/httpretry`); the new `grammar` package sits beside it.
- `ai/hash.go:14` `ShortHash` — not needed here, but the same helper later
  issues use.
- Upstream: `packages/ai/src/api/constrained-sampling.ts`,
  `packages/ai/src/types.ts`, `packages/ai/test/constrained-sampling.test.ts` at
  `936aff00`.

## Dependencies

- **Blocked by**: [Epic 2 issue 04](/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md)
  in practice (the compat flags the resolvers are called with), though this PR
  takes them as plain `bool` parameters and compiles without it.
- **Blocks**: issues 03, 06, 10, 11 — every grammar call site in the epic.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
