---
type: Decision
title: "Existing adapter updates"
description: "Bringing the nine existing wire adapters up to their 0.84.1 upstream counterparts."
tags: [decision, scope]
timestamp: 2026-08-09T01:30:23Z
phase: scope
decision: 06
slug: adapter-updates
status: decided
verdict: "All nine adapters, split across two epics: the OpenAI family (completions, responses, azure, codex) together so the shared core moves once, then anthropic, google+vertex, mistral and bedrock. simple-options and lazy stay with the core-types milestone."
decided_via: triage
depends_on: [core-types-migration, catalog-schema-and-tooling]
---

# Question

Every one of kern-link's nine wire adapters has upstream changes in range:

| Upstream file | Δ | Go package |
|---|---|---|
| `api/mistral-conversations.ts` | +423 | `ai/apis/mistral` |
| `api/openai-responses-shared.ts` | +412 | `ai/apis/openairesponses` |
| `api/openai-completions.ts` | +396 | `ai/apis/openaicompletions` |
| `api/anthropic-messages.ts` | +252 | `ai/apis/anthropic` |
| `api/openai-codex-responses.ts` | +220 | `ai/apis/codex` |
| `api/bedrock-converse-stream.ts` | +146 | `ai/apis/bedrock` |
| `api/google-generative-ai.ts`, `google-shared.ts`, `google-vertex.ts` | — | `ai/apis/google`, `google/vertex` |
| `api/azure-openai-responses.ts` | — | `ai/apis/azure` |
| `api/simple-options.ts`, `api/lazy.ts` | — | `ai/apis/simpleopts.go`, `ai/lazy.go` |

This is the bulk of the semantic porting work and the part with the most
per-provider wire detail.

# Options

- **All nine, one epic** — coherent, but far too large: this is well over 2000
  lines of upstream change plus its tests.
- **All nine, split into two or three epics by adapter family** — e.g. the
  OpenAI family (completions / responses-shared / responses / azure / codex),
  then Anthropic + Google + Vertex, then Mistral + Bedrock.
- **Only the adapters a consumer uses today** — cheapest, but leaves the port
  claiming 35 providers while several silently lag upstream.

# Recommendation

**All nine, split by adapter family across two epics**, grouped so that shared
code moves once:

- **OpenAI family** — `openai-responses-shared` is the shared core the Azure and
  Codex variants are built on (`docs/PORTING.md` records that its converters are
  exported for exactly that reason), so `openaicompletions`, `openairesponses`,
  `azure`, and `codex` must move together or the shared layer gets ported twice.
- **The rest** — `anthropic`, `google` + `vertex`, `mistral`, `bedrock`. These
  are mutually independent and can be issue-level parallel inside one epic.

Keep `simple-options` and `lazy.ts` with the core-types epic
([04](/scope/04-core-types-migration.md)) rather than here — they are
options-shape changes, not wire changes, and every adapter compiles against them.

# Verdict

**All nine, split by adapter family across two epics.** Accepted as recommended.

- **Milestone 3 — OpenAI family**: `openaicompletions`, `openairesponses`,
  `azure`, `codex`. These move together because `openai-responses-shared` is the
  shared core beneath Azure and Codex — `docs/PORTING.md` records its converters
  as exported for exactly that purpose — and splitting them would port the shared
  layer twice.
- **Milestone 4 — the rest**: `anthropic`, `google` + `vertex`, `mistral`,
  `bedrock`. Mutually independent, so they can be issue-level parallel inside
  one epic.

`simple-options` and `lazy.ts` stay with milestone 1
([decision 04](/scope/04-core-types-migration.md)): they are options-shape
changes every adapter compiles against, not wire changes.

Two riders attach to these epics rather than standing alone: constrained
sampling to milestone 3 ([decision 12](/scope/12-constrained-sampling.md)) and
the Copilot headers gap to milestone 4
([decision 16](/scope/16-copilot-headers-gap.md)).
