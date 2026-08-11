---
type: Epic
title: "Remaining adapters"
description: "Sync the Anthropic, Google/Vertex, Mistral and Bedrock adapters, close the long-standing Copilot dynamic-headers gap, and classify cloudflare-stream."
tags: [epic]
timestamp: 2026-08-11T20:10:00Z
epic: 5
slug: remaining-adapters
status: open
gh_issue: 122
milestone: 22
resource: https://github.com/kern-ia/kern-link/issues/122
source: docs/planning/SCOPE.md#milestone-5-remaining-adapters
---

# Epic 5: Remaining adapters

## Goal

The adapters outside the OpenAI family. They are mutually independent, so the
work parallelises at issue level within the epic — but each one compiles against
the same epic-2 core, which is why they are grouped rather than scattered.

## Scope

- `anthropic-messages.ts` (+252) → `ai/apis/anthropic`
- `google-generative-ai.ts`, `google-shared.ts`, `google-vertex.ts` →
  `ai/apis/google`, `ai/apis/google/vertex`
- `mistral-conversations.ts` (+423) → `ai/apis/mistral`
- `bedrock-converse-stream.ts` (+146) → `ai/apis/bedrock`
- **The Copilot dynamic-headers gap**: `src/api/github-copilot-headers.ts`
  (`X-Initiator`, `Copilot-Vision-Request`), unported since the initial port and
  logged at `docs/PORTING.md:46`. It sits outside the 0.80.3→0.84.1 delta, but
  it is the same failure mode the governing principle names — an upstream file
  with no Go base — in files this epic opens anyway. **Check whether it changed
  in range before porting it.**
- `cloudflare-stream.ts` needs a one-line classification here: it is Cloudflare
  streaming support, not a provider binding, and probably belongs beside
  `cloudflare_auth.go`.

## Out of scope

- The OpenAI-family adapters — epic 4 owns those, except the Copilot header
  builders, which [issue 01](/epic-5-remaining-adapters/issues/01-copilot-dynamic-headers.md)
  owns end to end.
- The classifier audit of the error strings these adapters emit; epic 9 audits
  after the adapters have moved, except where this epic's own change to an
  error string forces a classifier check in the same PR.
- `pi-messages`, which is a new tenth adapter package rather than an update —
  epic 8.
- Idiomatic-Go cleanups of ported code.

## Acceptance criteria

1. Anthropic, Google, Vertex, Mistral and Bedrock match upstream at `936aff00`
   and compile against the epic-2 core.
2. The Copilot dynamic-headers file is ported (or, if it proves structurally
   non-portable, recorded as a deviation with the reason), and
   `docs/PORTING.md:46`'s entry is updated to say so.
3. `cloudflare-stream.ts` has a recorded classification and a Go home.
4. Ported upstream tests come across with the code; new coverage is offline
   `httptest`.
5. Every file touched carries a `// Ports:` header and is dispositioned in
   `docs/PORTING.md`.
6. CI green: `go test ./... -race -v`, `bash upstream/sync_test.sh`,
   `golangci-lint` v2.12.2.

## Dependencies

- [Epic 2: Core types and Models contracts](/epic-2-core-types-and-models-contracts/EPIC_2.md)
- [Epic 4](/epic-4-openai-family-adapters/EPIC_4.md) issue 01 — creates
  `ai/apis/internal/grammar` and `ResolveJSONSchemaStrictSampling`, which
  [issue 03](/epic-5-remaining-adapters/issues/03-anthropic-strict-tools-and-signed-thinking.md),
  [issue 05](/epic-5-remaining-adapters/issues/05-google-shared-converters.md),
  [issue 07](/epic-5-remaining-adapters/issues/07-mistral-stop-reasons-and-strict-tools.md)
  and [issue 09](/epic-5-remaining-adapters/issues/09-bedrock-stop-reasons-strict-tools-and-claude-5.md)
  call directly, and
  [issue 04](/epic-5-remaining-adapters/issues/04-anthropic-deferred-tools.md),
  [issue 06](/epic-5-remaining-adapters/issues/06-google-and-vertex-stream-and-params.md)
  and [issue 08](/epic-5-remaining-adapters/issues/08-mistral-wire-and-header-parity.md)
  need transitively through 03, 05 and 07. Issue 03 states the contradiction
  with the paragraph below outright: "the epic file says epic 5 is independent
  of Epic 4; that is true of the wire work and false of this one function" —
  that is what forced this amendment.

The wire work stays independent: issues 01, 02, 10 and 11 touch no code that
needs `ai/apis/internal/grammar`, so they and the rest of this epic can still
run concurrently with Epic 4 once epic 2 has merged.

## Context

- [Technical specs](../../planning/SPECS.md)
- [Conventions](../../planning/CONVENTIONS.md)
- [Upstream sync scope](../../planning/SCOPE.md)
- [Decision 06 — adapter updates](../../planning/scope/06-adapter-updates.md)
- [Decision 16 — Copilot headers gap](../../planning/scope/16-copilot-headers-gap.md)

## Notes

- Project-wide, not this epic's own boundary: **no new direct dependencies** —
  raw `net/http` + `ai/internal/sse` everywhere except Bedrock
  (`aws-sdk-go-v2`) and Vertex ADC, a deliberate deviation from upstream's SDK
  delegation that erodes one plausible commit at a time under a soft rule.
- **Error text stays upstream-verbatim**; `ai/retry.go` and `ai/overflow.go`
  classify failures by matching it.
- The governing principle: a deviation must be justified by structural
  non-portability, never by cost.
- Upstream target frozen at `936aff00`.

**Decisions taken while reconciling this epic with its issues** (epic 0 issue
08), recorded because a later reader will otherwise re-open them — none of the
five is structurally non-portable, so the governing principle above already
rules out the cost-driven alternative in every case:

- [Issue 06](/epic-5-remaining-adapters/issues/06-google-and-vertex-stream-and-params.md)
  honors `opts.Fetch` (`TestGoogleHonorsInjectedFetch`) rather than porting
  upstream's rejection. Matches Epic 4 issue 02, which honors `Fetch` for the
  OpenAI family — the library gets one `Fetch` contract instead of two.
- [Issue 08](/epic-5-remaining-adapters/issues/08-mistral-wire-and-header-parity.md)
  adopts the 60s default request timeout rather than recording a `PORTING.md`
  deviation. A hung Mistral SSE connection would otherwise block forever —
  exactly the failure upstream's timeout bounds — and nothing makes a default
  timeout structurally harder to add in Go than in TypeScript.
- [Issue 09](/epic-5-remaining-adapters/issues/09-bedrock-stop-reasons-strict-tools-and-claude-5.md)
  matches upstream's two-lookup shape for the thinking-budget default/override
  split, rather than collapsing it to one lookup with an explanatory comment.
  The two-lookup shape costs nothing extra to write and keeps this passage
  diffable against upstream if the two lookups ever stop agreeing.
- [Issue 02](/epic-5-remaining-adapters/issues/02-anthropic-stream-lifecycle.md)
  emits the matching `TextDeltaEvent` / `ThinkingDeltaEvent` when
  `content_block_start` carries non-empty prefilled text or thinking, rather
  than only seeding the final content block. Every other content source in the
  decode path reaches consumers as a delta before it reaches the final
  message; a prefilled block that skipped that path would be the one
  unannounced exception to it.
- [Issue 11](/epic-5-remaining-adapters/issues/11-cloudflare-stream-classification.md)
  ports the dispatch-time placeholder resolution rather than recording a
  deviation. It is ~20 lines, idempotent against the existing auth-time pass,
  and Epic 6 issue 03 (#172) already named the alternative a regression: a
  literal `{CLOUDFLARE_ACCOUNT_ID}` shipping in a request URL.

**Branch, base-branch and status-commit conventions** (`docs/planning/CONVENTIONS.md:213-224`).
Feature branches are named `issue-<NN>-<slug>` (`:216-218`). PRs target `develop` and
merge with a merge commit — no rebase, no squash (`:213-219`). Each status transition
gets its own `docs(epics): …` commit, separate from the implementation commit
(`:221-224`). `Closes #N` will not auto-close the issue, because PRs merge into
`develop` rather than the repo's default branch — the explicit close at reconcile is
the normal route, not a fallback.
