---
type: Issue
title: "Extend Compat with the 0.84.1 flags, BedrockCompat, and SessionAffinityFormat"
description: "Add the twelve new per-api compatibility flags, replace sendSessionIdHeader with sessionAffinityFormat, and fold in the new BedrockCompat struct."
tags: [epic-2]
timestamp: 2026-08-09T04:31:17Z
epic: 2
issue: 04
slug: compat-flags-and-bedrock-compat
size: M
status: open
gh_issue: 132
resource: https://github.com/kern-ia/kern-link/issues/132
depends_on: []
---

# Extend Compat with the 0.84.1 flags, BedrockCompat, and SessionAffinityFormat

## Summary

Upstream's three per-api compat interfaces gained twelve flags between v0.80.3
and v0.84.1, added a fourth interface (`BedrockCompat`), and replaced the
boolean `sendSessionIdHeader` with a three-valued `SessionAffinityFormat`.

kern-link merges all of them into one flat `Compat` struct
(`ai/model.go:113-150`) because the wire JSON is a flat object either way — a
deviation already recorded in the type's own doc comment. This PR extends that
struct. It ships ahead of the adapters (epics 4 and 5) and ahead of the
regenerated catalog (epic 3) because both of those *consume* these fields:
without them, the regenerated catalog has keys nothing decodes.

## Scope

Add to `ai/model.go`'s `Compat`, keeping the existing section comments that
group fields by api:

**openai-completions**

- `SupportsFinishReason *bool` — when false, pi infers `stop`/`toolUse` at
  stream end. Default true.
- `ChatTemplateArgs map[string]ChatTemplateKwargValue` — sent as
  `chat_template_args` when `ThinkingFormat` is `"baseten"`.
- `ThinkingFormatBaseten ThinkingFormat = "baseten"` — a new value on the
  existing `ThinkingFormat` enum (`ai/model.go:95-106`).
- `SupportsThinkingTokenBudget *bool` — top-level `thinking_token_budget`
  (vLLM). Default false.
- `SupportsOpenAIGrammarTools *bool` — Lark/regex grammar tools. Default false.
- `DeferredToolsMode string` — provider-specific deferred-tool serialization;
  the only upstream value is `"kimi"`.

**openai-responses** (which now also covers `azure-openai-responses` and
`openai-codex-responses` upstream — a type-level widening with no Go equivalent,
since Go's `Compat` is already flat; note it in the struct comment)

- `SupportsStrictMode` already exists for completions; upstream now declares it
  on responses too — one field serves both in the flat struct.
- `SupportsOpenAIGrammarTools` (shared, as above).
- `SupportsAdditionalTools *bool` — message-anchored `additional_tools` input
  items. Default false.
- `SupportsToolSearch *bool` — client-executed tool search for deferred tools.
  Default false.
- `SupportsExplicitPromptCacheMode *bool` — `prompt_cache_options`
  (OpenAI GPT-5.6+); older models reject the parameter. Default false.

**anthropic-messages**

- `SupportsStrictTools *bool` — Anthropic strict tool schemas. Default false.
- `SupportsToolReferences *bool` — deferred tools loaded via `tool_reference`
  blocks in tool results. Upstream's default is conditional (true for
  first-party Anthropic models except Haiku and pre-4.5); port the *field* and
  its documented default here, not the model-by-model defaulting, which is
  catalog data.

**bedrock-converse-stream** (new upstream interface `BedrockCompat`)

- `SupportsStrictMode` — reuse the existing flat field; extend its comment to
  name Bedrock. If the flat merge makes the two meanings ambiguous, split the
  field and record why in the struct comment.

**Session affinity**

- New type `SessionAffinityFormat string` with `"openai"`,
  `"openai-nosession"`, `"openrouter"` (upstream declares it in `types.ts`
  beside `ProviderHeaders`).
- Replace `SendSessionIDHeader *bool` (`ai/model.go:142`) with
  `SessionAffinityFormat SessionAffinityFormat`
  (`json:"sessionAffinityFormat,omitempty"`), matching upstream's removal.
- `SendSessionAffinityHeaders *bool` (`ai/model.go:138`) stays; upstream kept it
  and only rewrote its doc comment.

## Out of scope

- **Every adapter's use of these flags.** `SendSessionIDHeader` is read by
  `ai/apis/openairesponses`; this PR makes the minimum mechanical edit needed to
  keep the tree compiling and leaves behavior to
  [Epic 4](/epic-4-openai-family-adapters/EPIC_4.md). Say so explicitly in the
  PR body, and do not "improve" the adapter while you are in there.
- **Catalog values** for any new flag — [Epic 3](/epic-3-catalog-schema-and-export-tooling/EPIC_3.md)
  regenerates `ai/catalog/data/models/*.json`, and `ai/catalog/compat.go`'s
  base-URL auto-detection is epic 3/4 territory unless a new field breaks it.
- `Tool.constrainedSampling` and the grammar types the
  `SupportsOpenAIGrammarTools` flag gates — [Epic 4](/epic-4-openai-family-adapters/EPIC_4.md),
  per [decision 12](../../../planning/scope/12-constrained-sampling.md).
- Anything about `pi-messages` compat — [Epic 8](/epic-8-pi-messages-and-radius/EPIC_8.md).

## Acceptance criteria / Definition of done

- [ ] Every field above exists on `ai.Compat` with the upstream JSON key,
      tri-state `*bool` for booleans that upstream documents with a default
      (matching the struct's existing "nil = auto-detect/default" contract at
      `ai/model.go:117`).
- [ ] `SendSessionIDHeader` no longer exists:
      `grep -rn "SendSessionIDHeader" --include=*.go .` returns nothing.
- [ ] `TestCompatRoundTripsSessionAffinityFormat` — a catalog entry with
      `"sessionAffinityFormat":"openrouter"` decodes to
      `ai.SessionAffinityOpenRouter` and re-marshals to the same key.
- [ ] `TestCompatOmitsUnsetFlags` — a `Compat` with only one field set marshals
      to a single-key object, so the regenerated catalog stays diffable.
- [ ] The currently embedded catalog still loads:
      `GOTMPDIR=$PWD/.gotmp go test ./ai/catalog/...` green with data unchanged.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2).
- [ ] `gofmt -l .` prints nothing; `ai/model.go` keeps its `// Ports:` header.
- [ ] Conventional Commit, e.g.
      `feat(ai): extend Compat with the 0.84.1 compatibility flags`.

## Relevant files / areas

- `ai/model.go:91-150` — `ThinkingFormat`, `ChatTemplateKwargValue`, `Compat`.
- `ai/catalog/compat.go` + `ai/catalog/compat_test.go` — base-URL-driven compat
  defaults; check whether any new field participates before assuming it doesn't.
- `ai/apis/openairesponses/` — the only current reader of
  `SendSessionIDHeader`; mechanical compile fix only.
- `ai/catalog/data/models/*.json` — must keep decoding untouched.
- Upstream: `src/types.ts` at `936aff00`, interfaces `OpenAICompletionsCompat`,
  `OpenAIResponsesCompat`, `AnthropicMessagesCompat`, `BedrockCompat`.

## Dependencies

- **Blocked by**: None. Independent of issues 01–03; it also edits
  `ai/model.go`, so rebase on whichever of 01/03 lands first.
- **Blocks**: Nothing inside this epic; epics 3, 4 and 5 all read these fields.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
