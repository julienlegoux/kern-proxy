---
type: Guide
title: Usage guide
description: Installing kern-proxy and using the unified API — model registry, streaming, tool calls, thinking levels, cost tracking, and session persistence.
tags: [usage, quick-start, streaming, tools]
timestamp: "2026-07-09"
---

# Usage guide

kern-proxy is a Go library — there is no server to run. You import it, build a
model registry, and stream.

# Install

```sh
go get github.com/julienlegoux/kern-proxy
```

Import paths:

| Package | Purpose |
|---|---|
| `github.com/julienlegoux/kern-proxy/ai` | Core types: `Context`, `Model`, messages, events, options, cost |
| `github.com/julienlegoux/kern-proxy/ai/providers` | Built-in provider registry (`providers.Models`) |
| `github.com/julienlegoux/kern-proxy/ai/auth` | Persistent credential store, env-key helpers |
| `github.com/julienlegoux/kern-proxy/ai/providers/faux` | In-process fake provider for tests |

# Quick start

Set a provider API key (e.g. `ANTHROPIC_API_KEY`), then:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/auth"
	"github.com/julienlegoux/kern-proxy/ai/providers"
)

func main() {
	ctx := context.Background()

	credPath, err := auth.DefaultPath() // ~/.pi/agent/auth.json
	if err != nil {
		log.Fatal(err)
	}
	models := providers.Models(&ai.CreateModelsOptions{
		Credentials: auth.NewFileCredentialStore(credPath),
	})

	model := models.GetModel("anthropic", "claude-sonnet-4-5")
	if model == nil {
		log.Fatal("unknown model")
	}

	chat := ai.Context{
		SystemPrompt: "You are a helpful assistant.",
		Messages: []ai.Message{
			&ai.UserMessage{
				Content:   ai.UserText("Hello, who are you?"),
				Timestamp: time.Now().UnixMilli(),
			},
		},
	}

	stream := models.StreamSimple(ctx, model, chat, &ai.SimpleStreamOptions{})
	for ev := range stream.Events(ctx) {
		if e, ok := ev.(ai.TextDeltaEvent); ok {
			fmt.Print(e.Delta)
		}
	}

	msg, err := stream.Result(ctx)
	if err != nil {
		log.Fatal(err) // only ctx cancellation; provider failures are in msg
	}

	cost := ai.CalculateCost(model, &msg.Usage)
	fmt.Printf("\ntokens in=%d out=%d  $%.6f\n",
		msg.Usage.Input, msg.Usage.Output, cost.Total)
}
```

`providers.Models(nil)` also works: it defaults to an in-memory credential
store and the process environment, so env API keys alone are enough. Use
`models.CompleteSimple(ctx, model, chat, opts)` when you don't need deltas —
it returns the final `*ai.AssistantMessage` directly.

# Error handling

Provider failures are **in-band**, mirroring upstream pi-ai:

- `stream.Result(ctx)` returns a Go error only when `ctx` is cancelled before
  the stream terminates. Everything else — HTTP errors, rate limits, provider
  aborts — arrives as an `AssistantMessage` with `StopReason` set to
  `ai.StopReasonError` or `ai.StopReasonAborted` and `ErrorMessage` populated.
- Always inspect `msg.StopReason` after a turn. Stop reasons:
  `StopReasonStop`, `StopReasonLength`, `StopReasonToolUse`,
  `StopReasonError`, `StopReasonAborted`.
- `ai.IsRetryableAssistantError(msg)` classifies transient failures (you
  supply the retry loop); `ai.IsContextOverflow(msg, model.ContextWindow)`
  detects context-window overflow across providers.
- `stream.Events(ctx)` consumes from a shared queue — use a single consumer
  per stream.
- If you `break` out of the `Events` loop before the stream terminates, cancel
  `ctx`. The channel is unbuffered, so the goroutine feeding it blocks forever
  on the next event otherwise. Passing `context.Background()` is only safe when
  the loop always drains to completion.

# Streaming events

Every event implements `ai.Event` (`EventKind()`); non-terminal events carry a
`Partial *AssistantMessage` snapshot of the accumulating message.

| Event | Payload |
|---|---|
| `StartEvent` | turn started |
| `TextStartEvent` / `TextDeltaEvent{Delta}` / `TextEndEvent{Content}` | assistant text, per `ContentIndex` |
| `ThinkingStartEvent` / `ThinkingDeltaEvent{Delta}` / `ThinkingEndEvent{Content}` | reasoning tokens |
| `ToolCallStartEvent` / `ToolCallDeltaEvent{Delta}` / `ToolCallEndEvent{ToolCall}` | tool call, raw argument JSON in deltas |
| `DoneEvent{Reason, Message}` | terminal success |
| `ErrorEvent{Reason, Error}` | terminal failure (in-band) |

Helpers: `ai.IsTerminalEvent(ev)`, `ai.EventResult(ev)`.

# Tool calls

Declare tools on the context; loop while the model stops with
`StopReasonToolUse`, appending a `ToolResultMessage` per call:

```go
tools := []ai.Tool{{
	Name:        "get_time",
	Description: "Get the current local date and time.",
	Parameters:  ai.JSONSchema(`{"type":"object","properties":{}}`),
}}

messages := []ai.Message{
	&ai.UserMessage{Content: ai.UserText(input), Timestamp: time.Now().UnixMilli()},
}

for round := 0; round < 8; round++ {
	chat := ai.Context{SystemPrompt: sys, Messages: messages, Tools: tools}

	message, err := models.CompleteSimple(ctx, model, chat, opts)
	if err != nil {
		return err
	}
	messages = append(messages, message)

	if message.StopReason != ai.StopReasonToolUse {
		break
	}
	for _, block := range message.Content {
		call, ok := block.(ai.ToolCall)
		if !ok {
			continue
		}
		output, isErr := runTool(call.Name, call.Arguments) // Arguments is map[string]any
		messages = append(messages, &ai.ToolResultMessage{
			ToolCallID: call.ID,
			ToolName:   call.Name,
			Content:    []ai.UserContentPart{ai.TextContent{Text: output}},
			IsError:    isErr,
			Timestamp:  time.Now().UnixMilli(),
		})
	}
}
```

`ai.ToolCall.Arguments` is already a decoded `map[string]any` — no JSON
parsing needed on your side.

# Thinking levels

Reasoning is a portable knob, translated per provider:

```go
opts := &ai.SimpleStreamOptions{Reasoning: ai.ThinkingMedium}
```

Levels: `ThinkingMinimal`, `ThinkingLow`, `ThinkingMedium`, `ThinkingHigh`,
`ThinkingXHigh` (empty = off/default). Discovery and clamping:

- `ai.GetSupportedThinkingLevels(model)` — what the model supports
  (non-reasoning models report only `ThinkingOff`).
- `ai.ClampThinkingLevel(model, level)` — snaps to the nearest supported
  level; `StreamSimple` applies this internally.
- `SimpleStreamOptions.ThinkingBudgets` sets per-level token budgets for
  budget-based providers.

Lower-level per-provider knobs (Anthropic effort/budget, Google thinking
level, Bedrock reasoning, …) live on the embedded `ai.StreamOptions`.

# Usage and cost

`AssistantMessage.Usage` carries `Input`, `Output`, `CacheRead`, `CacheWrite`,
`TotalTokens`, and optionally `Reasoning` and `CacheWrite1h`.
`ai.CalculateCost(model, &usage)` fills and returns `usage.Cost` from the
model's catalog prices (`model.Cost`, $/M tokens). To budget before a call,
`ai.EstimateContextTokens(chat)` estimates tokens without a request.

# Model registry

- `models.GetProviders()` / `models.GetModels("")` enumerate the embedded
  catalog (~35 providers).
- Dynamic providers (OpenRouter, Vercel AI Gateway, NVIDIA, GitHub Copilot)
  start empty: call `models.Refresh(ctx, providerID)` (or `""` for all) to
  fetch their live model lists; `provider.CanRefreshModels()` reports which.
- `models.GetAuth(ctx, model)` checks configuration without sending a request:
  `(nil, nil)` means unconfigured; `AuthResult.Source` is a display label like
  `"ANTHROPIC_API_KEY"` or `"OAuth"`.

See [auth](/auth.md) for env vars, the credential store, and OAuth logins.

# Session persistence and model hand-off

A conversation is just `[]ai.Message`, and every message JSON-round-trips with
a `"role"` discriminator — so persistence is plain `encoding/json`. Nothing
else is needed: no custom codec, no type registry, no bookkeeping of which
message was which.

## Serializing a session

Wrap the slice in `ai.Messages` to marshal it. The wrapper exists for the
decode side (a bare `[]ai.Message` can't be unmarshalled — `ai.Message` is an
interface), and it costs nothing on the encode side:

```go
raw, err := json.Marshal(ai.Messages(messages))
if err != nil {
	return err
}
if err := os.WriteFile("session.json", raw, 0o600); err != nil {
	return err
}
```

## Restoring a session

`ai.Messages` decodes the polymorphic list back into the concrete pointer
types — `*ai.UserMessage`, `*ai.AssistantMessage`, `*ai.ToolResultMessage` —
by reading each message's `"role"`:

```go
raw, err := os.ReadFile("session.json")
if err != nil {
	return err
}
var restored ai.Messages
if err := json.Unmarshal(raw, &restored); err != nil {
	return err
}
```

`ai.Context` also unmarshals directly, so a whole session — system prompt,
messages, tools — can be stored and resumed as a single value rather than
reassembled field by field. Marshal the context itself (not just its
messages) and you get a document you can hand straight back:

```go
// persist the whole session
raw, err := json.Marshal(chat) // chat is an ai.Context
if err != nil {
	return err
}

// ...later, in another process
var chat ai.Context
if err := json.Unmarshal(raw, &chat); err != nil {
	return err
}
```

`Context.UnmarshalJSON` decodes the `messages` array polymorphically, exactly
as `ai.Messages` does above — the two are the same mechanism at different
granularities.

## Continuing the conversation

Restoring is only useful if you can keep going. Append the next turn and hand
the context to any model — including one from a different provider, which is
what makes this a hand-off rather than just a reload:

```go
chat.Messages = append(chat.Messages, &ai.UserMessage{
	Content:   ai.UserText("And 3+3?"),
	Timestamp: time.Now().UnixMilli(),
})

next := models.GetModel("openai", "gpt-5")
stream := models.StreamSimple(ctx, next, chat, opts)
```

Both halves of this round trip are compiled and asserted as runnable examples
in `ai/example_session_test.go` (`ExampleMessages`,
`ExampleContext_UnmarshalJSON`), so they cannot drift from the API.

One caveat on hand-off: thinking and tool-call blocks carry opaque
provider-scoped signatures that only matter when replaying on the same
provider. Cross-provider hand-off relies on the plain text and tool content,
which is exactly what the adapters send.

# Testing without a network

The `faux` provider runs in-process and exercises the full event, tool, and
usage flow:

```go
handle := faux.New(&faux.Options{})
models.SetProvider(handle.Provider)
handle.SetResponses(faux.Step(faux.TextMessage("hi", nil)))
```

Builders: `faux.Text`, `faux.Thinking`, `faux.ToolCall`, `faux.AssistantMessage`.

For the layering behind all of this, see [architecture](/architecture.md).
