---
type: Decision
title: "Constrained sampling / grammars"
description: "Whether upstream's grammar-constrained sampling support is ported in this sync."
tags: [decision, scope]
timestamp: 2026-08-10T09:20:00Z
phase: scope
decision: 12
slug: constrained-sampling
status: decided
verdict: "Port it with the OpenAI-family adapter milestone, without the cut-candidate caveat the recommendation attached — cost is no longer an admissible reason for a deviation."
decided_via: discussion
depends_on: [core-types-migration, adapter-updates]
---

# Question

Upstream added `src/api/constrained-sampling.ts` (+148) with
`GrammarFormat` (`"openai_lark" | "openai_regex"`), `GrammarVariants`, and
`ConstrainedSamplingConfig` in `types.ts`.

Unlike deferred tools ([decision 11](/scope/11-deferred-tools.md)), this one is
*not* entangled with the core enums. It is an additive options surface: a
consumer who never sets a grammar never encounters it, and the type additions
are inert structs rather than new values in an existing union.

It is, however, per-adapter: the two grammar formats are OpenAI-specific, so the
wiring lands in the OpenAI-family adapters that
[decision 06](/scope/06-adapter-updates.md) is already touching.

# Options

- **Port with the OpenAI-family adapter epic** — 148 lines plus per-adapter
  wiring, while that code is already open.
- **Defer as a deviation** — smallest program; a genuinely optional feature no
  current consumer has asked for.
- **Port the types only** — inert structs, no adapter wiring. Worst of both:
  carries the surface without the function.

# Recommendation

**Port with the OpenAI-family adapter epic.** The marginal cost is low precisely
because of sequencing: `openai-completions.ts` and `openai-responses-shared.ts`
are being ported anyway, and grammar support is a field in the request builders
that are already being rewritten. Doing it later means reopening the same files
and re-deriving the same wire shapes.

That said, this is the strongest *candidate* for cutting if the program needs to
shrink — it is genuinely optional, cleanly separable, and no downstream consumer
has asked for it. It is listed here as its own decision rather than folded into
[06](/scope/06-adapter-updates.md) for exactly that reason: so cutting it later
is a one-line verdict change rather than an epic rewrite.

Note the Go-side shape question the epic must settle: `ConstrainedSamplingConfig`
would land in `ai.StreamOptions`, which `docs/PORTING.md` records as already
carrying the Anthropic-specific knobs because "Go can't overload the single
`StreamFunc` signature". This continues an established deviation rather than
creating a new one. *(Superseded — see the Verdict: the field is
`Tool.constrainedSampling` upstream, so it lands on `ai.Tool` and no deviation is
involved. This paragraph is kept as the record of what was recommended.)*

# Verdict

**Port it with the OpenAI-family adapter milestone.** Decided in discussion,
2026-08-09 — the recommendation is upheld, but **its caveat is struck**.

The recommendation proposed porting while noting this was "the strongest
candidate for cutting if the program needs to shrink". That reservation rested
entirely on cost and optionality, which the parity principle established in
[decision 09](/scope/09-new-oauth-flows.md) and recorded in
[decision 02](/scope/02-parity-bar.md) no longer admits as grounds for a
deviation. Constrained sampling is ordinary Go-portable code; it is ported.

Sequencing is unchanged and is the reason it belongs to that milestone rather
than its own: the two grammar formats are OpenAI-specific, and the request
builders they attach to are being rewritten there anyway. Porting it later would
mean reopening the same files and re-deriving the same wire shapes.

The Go-side shape question is settled, and **not** the way this decision first
recorded it. Amended 2026-08-10 against upstream at `936aff00`:
`packages/ai/src/types.ts:506` declares
`constrainedSampling?: false | ConstrainedSamplingConfig` on the `Tool`
interface (`:501-507`, with `ConstrainedSamplingConfig` at `:492-500`), while
`StreamOptions` (`:175-219`) carries no such field. The configuration is per
**tool**, not per request, so `ConstrainedSamplingConfig` lands on `ai.Tool`
(`ai/types.go:243-247`) and the per-adapter-options deviation `docs/PORTING.md`
records for the Anthropic knobs is **not** involved — nothing is deviated from
here.

The earlier reading ("lands in `ai.StreamOptions`, continuing the established
deviation") was written before the upstream shape was read; it is corrected here,
in [EPIC_4.md](../../epics/epic-4-openai-family-adapters/EPIC_4.md) and in
[SCOPE.md](/SCOPE.md)'s milestone-4 bullet, so the question is not re-opened.
