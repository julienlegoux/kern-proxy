---
type: Issue
title: "Close the Copilot dynamic-headers gap: port github-copilot-headers.ts and wire its three call sites"
description: "Port the unported X-Initiator / Openai-Intent / Copilot-Vision-Request header builder and apply it in the anthropic, openai-completions and openai-responses adapters."
tags: [epic-5]
timestamp: 2026-08-10T04:00:00Z
epic: 5
issue: 01
slug: copilot-dynamic-headers
size: S
status: open
gh_issue: 159
resource: https://github.com/kern-ia/kern-link/issues/159
depends_on: []
---

# Close the Copilot dynamic-headers gap: port github-copilot-headers.ts and wire its three call sites

## Summary

`src/api/github-copilot-headers.ts` is the port's **only** open follow-up
(`docs/PORTING.md:46`, `SPECS.md`'s single "known gap"). It has been unported
since the initial port, and [decision 16](../../../planning/scope/16-copilot-headers-gap.md)
closes it inside this program because it is the exact failure mode the parity
principle names — an upstream file with no Go base, sitting in files this epic
opens anyway.

**Verified in range**: `git diff 244f1dea..936aff00 -- packages/ai/src/api/github-copilot-headers.ts`
is **empty**. The file did not change between 0.80.3 and 0.84.1, so the 0.80.3
version is also the 0.84.1 version — the check
[decision 16](../../../planning/scope/16-copilot-headers-gap.md) asked for is
done, and no rebase against the newer revision is needed.

The whole upstream file is 37 lines and exports three functions:

```ts
inferCopilotInitiator(messages): "user" | "agent"   // last message not role "user" → "agent"
hasCopilotVisionInput(messages): boolean            // any image part in a user or toolResult message
buildCopilotDynamicHeaders({messages, hasImages}): Record<string, string>
//   → { "X-Initiator": …, "Openai-Intent": "conversation-edits" }  (+ "Copilot-Vision-Request": "true" when hasImages)
```

Three adapters import it at `936aff00`, and all three apply it the same way —
only when `model.provider === "github-copilot"`:

| Upstream file | Call site | Go destination |
|---|---|---|
| `api/anthropic-messages.ts:524-531` | inside `stream`, threaded into `createClient` | `ai/apis/anthropic/anthropic.go` `buildHeaders` (`:1193`) |
| `api/openai-completions.ts:646-653` | top of the header builder | `ai/apis/openaicompletions` header builder |
| `api/openai-responses.ts:224-231` | top of the header builder | `ai/apis/openairesponses` header builder |

Epic 4's [issue 09](/epic-4-openai-family-adapters/issues/09-openai-responses-compat-and-wiring.md)
explicitly declines the Copilot headers and points here, so this issue owns all
three call sites — including the two in packages epic 4 otherwise owns.

## Scope

- **New package `ai/apis/internal/copilotheaders`** (matching the placement of
  epic 4's `ai/apis/internal/grammar`: importable from every `ai/apis/*`
  adapter, invisible to consumers — [CONVENTIONS.md](../../../planning/CONVENTIONS.md),
  "Naming & file layout"), carrying a
  `// Ports: packages/ai/src/api/github-copilot-headers.ts` header and three
  exported functions:
  - `InferInitiator(messages []ai.Message) string` — `"agent"` when the last
    message's role is not `user`, else `"user"`. An empty slice yields `"user"`
    (upstream: `last && last.role !== "user"`, so a missing last message falls
    through to `"user"`).
  - `HasVisionInput(messages []ai.Message) bool` — true when any
    `*ai.UserMessage` or `*ai.ToolResultMessage` content part is an
    `ai.ImageContent`. Assistant messages are **not** inspected, matching
    upstream.
  - `Build(messages []ai.Message, hasImages bool) map[string]string` — returns
    the two always-on headers plus `Copilot-Vision-Request: "true"` when
    `hasImages`. **Header names keep upstream's exact casing** (`X-Initiator`,
    `Openai-Intent`, `Copilot-Vision-Request`); do not normalize to lowercase,
    and check how the destination adapter's header map is cased before merging
    (see below).
- **Wire the three call sites**, each gated on `model.Provider == "github-copilot"`:
  - `ai/apis/anthropic/anthropic.go` `buildHeaders` (`:1193`) — the Go port has
    no `createClient`; `buildHeaders` already takes `chat ai.Context`, so the
    messages are in hand. Merge the dynamic headers **before**
    `ai.MergeProviderHeaders` applies `model.Headers` / `opts.Headers`, so an
    explicit caller header still wins (upstream passes them as
    `defaultHeaders`, which the SDK merges under caller headers).
  - `ai/apis/openaicompletions` — same gate, in the package's header builder.
  - `ai/apis/openairesponses` — same gate, in the package's header builder.
- **`docs/PORTING.md:46`** — the row currently reads
  `| src/api/github-copilot-headers.ts | none | **not ported** (…; open follow-up) |`.
  Replace it with a `ported` disposition naming the new package. This row going
  green is the point of the issue, not bookkeeping after it. Do not edit
  `docs/planning/SPECS.md` in this PR — a merge-blocking `docs/planning/` edit
  goes through a drift record, not an implementation PR, and `EPIC_5.md` never
  authorized this one.

## Out of scope

- `providers/github-copilot.ts` (+13 in range) and the Copilot OAuth flow — a
  provider binding, not this adapter file. See the note in the PR body.
- Anything else in `openai-completions` / `openai-responses`; touch only the
  header builder in those two packages. Everything else there belongs to
  [Epic 4](/epic-4-openai-family-adapters/EPIC_4.md), and this PR must stay
  trivially rebaseable onto it.
- The rest of the Anthropic sync — issues 02–04 of this epic.

## Acceptance criteria / Definition of done

- [ ] `TestInferInitiatorAgentAfterAssistantTurn` — a message list ending in an
      `*ai.AssistantMessage` or `*ai.ToolResultMessage` yields `"agent"`; one
      ending in an `*ai.UserMessage` yields `"user"`; an empty list yields
      `"user"`.
- [ ] `TestHasVisionInputSeesUserAndToolResultImages` — an image in a user
      message and an image in a tool-result message each return true; an image
      in an assistant message and a text-only history return false.
- [ ] `TestBuildOmitsVisionHeaderWithoutImages` — the map is exactly
      `{"X-Initiator", "Openai-Intent"}` with `Openai-Intent ==
      "conversation-edits"`; with `hasImages` true it also carries
      `Copilot-Vision-Request: "true"`.
- [ ] `TestAnthropicSendsCopilotHeaders` — an offline `httptest` request for a
      model with `Provider == "github-copilot"` carries all three headers with
      upstream's casing; the same model under any other provider carries none of
      them.
- [ ] Equivalent `httptest` assertions in `ai/apis/openaicompletions` and
      `ai/apis/openairesponses`.
- [ ] `TestCopilotHeadersDoNotOverrideCallerHeaders` — an explicit
      `opts.Headers["X-Initiator"]` survives.
- [ ] `docs/PORTING.md` has no remaining `**not ported**` row for
      `src/api/github-copilot-headers.ts`.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `feat(apis): send Copilot dynamic headers from the three adapters that need them`.

## Relevant files / areas

- `ai/apis/anthropic/anthropic.go:1193` — `buildHeaders(model, chat, opts, apiKey)`.
- `ai/apis/openaicompletions/` and `ai/apis/openairesponses/` — the header
  builders (find them by their session-affinity header block; upstream puts the
  Copilot block immediately above it).
- `ai/apis/internal/httpretry` — the existing precedent for a shared,
  consumer-invisible adapter helper package; `ai/apis/internal/grammar` is not
  a second one yet — it is the package Epic 4 issue 01 creates, following the
  same layout.
- `ai/headers.go` — `ai.MergeProviderHeaders` and its null-deletion semantics.
- `docs/PORTING.md:46` — the row this issue closes.
- Upstream: `packages/ai/src/api/github-copilot-headers.ts` (unchanged in
  range), call sites at `anthropic-messages.ts:524`, `openai-completions.ts:646`,
  `openai-responses.ts:224`.
- Upstream test: `test/github-copilot-anthropic.test.ts` (+15 in range).

## Dependencies

- **Blocked by**: None.
- **Blocks**: None inside this epic, but it edits two files
  [Epic 4](/epic-4-openai-family-adapters/EPIC_4.md) rewrites heavily. Land it
  early or rebase it late; do not let it sit half-merged.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
