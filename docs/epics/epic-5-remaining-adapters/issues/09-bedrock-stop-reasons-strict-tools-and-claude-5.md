---
type: Issue
title: "bedrock: pending and raw stop reasons, strict tool schemas, and the Claude 5 model matrix"
description: "Start the Bedrock stream at pending, surface the raw stop reason in the error text, send strict tool specs behind the Bedrock compat flag, and recognise opus-5 and sonnet-5 across the thinking and cache matrices."
tags: [epic-5]
timestamp: 2026-08-11T18:15:00Z
epic: 5
issue: 09
slug: bedrock-stop-reasons-strict-tools-and-claude-5
size: M
status: open
gh_issue: 167
resource: https://github.com/kern-ia/kern-link/issues/167
depends_on: []
---

# bedrock: pending and raw stop reasons, strict tool schemas, and the Claude 5 model matrix

## Summary

Three independent groups of change from `bedrock-converse-stream.ts` (+146),
all in the request/stream path:

1. **`pending` and `rawStopReason`.** The stream starts at `"pending"`;
   `messageStop` records `output.rawStopReason = item.messageStop.stopReason`;
   `mapStopReason` returns `{stopReason, errorMessage?}` and produces
   `"Provider stopped with: <reason>"` for any unrecognized reason (and no
   message when the reason is absent entirely); after the loop, a still-pending
   reason throws `"Bedrock stream ended without a stop reason"`, and the
   error branch prefers `output.errorMessage` over
   `"An unknown error occurred"`. **All of these strings are copied verbatim** —
   `ai/retry.go` and `ai/overflow.go` classify by matching error text
   ([CONVENTIONS.md](../../../planning/CONVENTIONS.md)).

2. **Strict tool schemas.** `convertToolConfig` takes a
   `supportsStrictMode` argument (from `model.compat?.supportsStrictMode ??
   false` — the `BedrockCompat` interface Epic 2
   [issue 04](/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md)
   folds into the flat `Compat`) and emits `"strict": true` inside `toolSpec`
   when `resolveJsonSchemaStrictSampling` resolves true. Unlike Anthropic, the
   `inputSchema.json` is **not** reshaped — only the flag is added. Upstream also
   split the guard `if (!tools?.length || toolChoice === "none")` into two
   statements with no behavior change; ignore that.

3. **The Claude 5 matrix and the `max` thinking level.** Three model-matching
   lists gained entries, and the budget table gained one:
   - `supportsAdaptiveThinking`: `+ opus-5`
   - `supportsNativeXhighEffort`: `+ opus-5`, `+ sonnet-5`
   - `supportsPromptCaching`: `+ opus-5` in the Claude 5 branch
   - `buildAdditionalModelRequestFields`: `max: 16384` in the default budgets,
     and the clamp becomes `reasoning === "xhigh" || reasoning === "max"` →
     `high`. The comments changed too: "Claude doesn't support xhigh, clamp to
     high" → "Budget-based Claude clamps extended levels to high", and "Custom
     budgets override defaults (xhigh not in ThinkingBudgets, use high)" →
     "Custom budgets only cover token-based levels through high."

## Scope

- `ai/apis/bedrock/stream.go`:
  - `mapStopReason` (`:208`) — return `(ai.StopReason, string)`; the default
    branch yields `Provider stopped with: <reason>` when the reason is
    non-empty, and no message when it is empty.
  - `DecodeStream` (`:28`), `messageStop` branch — set `output.RawStopReason`
    from the raw SDK value before mapping, and `output.ErrorMessage` from the
    mapper.
- `ai/apis/bedrock/bedrock.go`:
  - `run` (`:146`) — initialize `ai.StopReasonPending` (currently
    `ai.StopReasonStop`, `:151`) and add the pending guard with the exact
    message `Bedrock stream ended without a stop reason` before the existing
    aborted/error branch; that branch must prefer `output.ErrorMessage`.
  - `buildParams` (`:239`) — thread the resolved `SupportsStrictMode` into
    `convertToolConfig`.
- `ai/apis/bedrock/messages.go`:
  - `convertToolConfig` (`:283`) — the `supportsStrictMode` parameter and the
    `strict` flag on `wireToolSpec` (`wire.go`). Strict resolution goes through
    `grammar.ResolveJSONSchemaStrictSampling` (epic 4
    [issue 01](/epic-4-openai-family-adapters/issues/01-constrained-sampling-core.md)),
    which **throws** on `strict: "require"` against an unsupporting provider —
    that must end the stream in-band, never panic
    ([CONVENTIONS.md](../../../planning/CONVENTIONS.md)).
  - `supportsPromptCaching` (`:110`) — add `opus-5` to the Claude 5 branch.
- `ai/apis/bedrock/thinking.go`:
  - `supportsAdaptiveThinking` (`:30`) — add `opus-5`.
  - `supportsNativeXhighEffort` (`:43`) — add `opus-5` and `sonnet-5`.
  - `buildAdditionalModelRequestFields` (`:102`) — clamp `ai.ThinkingMax` to
    `ai.ThinkingHigh` alongside `ai.ThinkingXHigh` (`:139`), and make sure
    `defaultThinkingBudgets` yields 16384 for both. Upstream looks the default
    up by the **unclamped** level and the custom override by the **clamped**
    one; the Go code uses the clamped level for both. **Match upstream's
    two-lookup shape** rather than leaving a comment that they happen to agree
    today — decided in [EPIC_5.md](/epic-5-remaining-adapters/EPIC_5.md)'s
    `## Notes`, since the two-lookup shape costs nothing extra and stays
    diffable against upstream if the two ever stop agreeing.
- Tests, offline, in `ai/apis/bedrock/stop_reason_test.go` plus the existing
  `thinking_test.go` and `messages_test.go`.
- `// Ports:` headers stay accurate.

## Out of scope

- Credential resolution, bearer tokens, and the failure diagnostic —
  [issue 10](/epic-5-remaining-adapters/issues/10-bedrock-credentials-and-diagnostics.md).
- `aws-sdk-go-v2` version bumps. The `strict` field on `toolSpec` may not exist
  in the pinned SDK's `types.Tool`; kern-link builds its own `wireRequest`
  (`wire.go`) and converts in `awsconvert.go`, so check whether the field
  survives that conversion **before** assuming a dependency bump is needed —
  and if one genuinely is, that is a `docs/planning/DRIFT.md` candidate and a
  conversation, not a silent `go get` ([EPIC_5](/epic-5-remaining-adapters/EPIC_5.md),
  "no new direct dependencies").

## Acceptance criteria / Definition of done

- [ ] `TestBedrockStreamWithoutStopReasonFails` — an event stream that delivers
      content but no `messageStop` ends with an `ErrorEvent` whose message is
      exactly `Bedrock stream ended without a stop reason`.
- [ ] `TestBedrockRawStopReasonInErrorText` — `messageStop.stopReason:
      "guardrail_intervened"` yields `RawStopReason == "guardrail_intervened"`,
      `StopReason` `error`, and `ErrorMessage == "Provider stopped with:
      guardrail_intervened"`; an absent reason yields `An unknown error
      occurred`.
- [ ] `TestBedrockKnownStopReasonsRecordRaw` — `end_turn`, `max_tokens`,
      `tool_use` map as before **and** set `RawStopReason` to the literal
      value, still producing a `DoneEvent`.
- [ ] `TestConvertToolConfigSendsStrictWhenSupported` — a tool with
      `ConstrainedSampling` `{type:"json_schema", strict:"prefer"}` against
      `SupportsStrictMode: true` serializes `toolSpec.strict == true` with an
      unchanged `inputSchema.json`; with the flag false there is no `strict`
      key and the bytes match today's output.
- [ ] `TestConvertToolConfigRequireStrictUnsupportedErrors` — `strict:
      "require"` against `SupportsStrictMode: false` ends the stream with an
      `ErrorEvent`, no panic.
- [ ] `TestOpus5UsesAdaptiveThinking` and `TestOpus5AndSonnet5MapXhighToEffort`
      — model ids containing `opus-5` / `sonnet-5` take the adaptive branch and
      the native `xhigh` effort.
- [ ] `TestOpus5SupportsPromptCaching` — cache points are injected for an
      `opus-5` model.
- [ ] `TestMaxThinkingLevelClampsToHigh` — `ai.ThinkingMax` produces the same
      `additionalModelRequestFields` as `ai.ThinkingXHigh` on a budget-based
      Claude model.
- [ ] Ported upstream coverage: both cases of
      `test/bedrock-raw-stop-reason.test.ts` and the four new `opus-5` /
      `sonnet-5` cases of `test/bedrock-thinking-payload.test.ts` come across as
      discrete Go tests.
- [ ] `docs/PORTING.md`'s row for `src/api/bedrock-converse-stream.ts` still
      describes the Go code after this change (unchanged: already `ported`).
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `fix(bedrock): surface the raw stop reason instead of an unknown error`.

## Relevant files / areas

- `ai/apis/bedrock/bedrock.go` (282 lines) — `:146` `run`, `:151` the output
  initializer, `:239` `buildParams`.
- `ai/apis/bedrock/stream.go` (219 lines) — `:28` `DecodeStream`, `:208`
  `mapStopReason`.
- `ai/apis/bedrock/messages.go` (310 lines) — `:110` `supportsPromptCaching`,
  `:283` `convertToolConfig`.
- `ai/apis/bedrock/thinking.go` (179 lines) — `:30` `supportsAdaptiveThinking`,
  `:43` `supportsNativeXhighEffort`, `:102`
  `buildAdditionalModelRequestFields`, `:139` the xhigh clamp.
- `ai/apis/bedrock/wire.go` (125 lines) and `awsconvert.go` (186 lines) — the
  hand-built request and its conversion into the SDK types.
- `ai/apis/internal/grammar` — epic 4 issue 01's package.
- Upstream: `src/api/bedrock-converse-stream.ts` at `936aff00` —
  `mapStopReason`, `convertToolConfig`, `supportsAdaptiveThinking`,
  `supportsNativeXhighEffort`, `supportsPromptCaching`,
  `buildAdditionalModelRequestFields`, and the post-loop guards.
- Upstream tests: `test/bedrock-raw-stop-reason.test.ts` (new, 82 lines),
  `test/bedrock-thinking-payload.test.ts` (+20).

## Dependencies

- **Blocked by**: Epic 4
  [issue 01](/epic-4-openai-family-adapters/issues/01-constrained-sampling-core.md)
  (cross-epic — `ResolveJSONSchemaStrictSampling`); Epic 2
  [issue 01](/epic-2-core-types-and-models-contracts/issues/01-widen-stopreason-and-thinkinglevel.md)
  (`StopReasonPending`, `ThinkingMax`),
  [issue 02](/epic-2-core-types-and-models-contracts/issues/02-message-model-deferred-fields.md)
  (`RawStopReason`),
  [issue 04](/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md)
  (`BedrockCompat`'s `SupportsStrictMode`).
- **Blocks**: None.

## PR size note

`M` — ~400 changed lines: three groups over five files from a +146 upstream
diff, with eight named tests and six ported upstream cases. Re-checked against
REPORT_5's "plausibly L" flag and kept `M`, because the Claude 5 group is four
list entries plus one clamp and the strict-tool group only adds a `strict` flag
to `toolSpec` without reshaping `inputSchema.json` — neither carries the schema
surgery that makes the Anthropic sibling heavier. Split past ~500, and
REPORT_5's named seam holds: stop reasons versus Claude 5 matrix + strict tools.
