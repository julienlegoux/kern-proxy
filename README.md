# kern-proxy

[![Test](https://github.com/julienlegoux/kern-proxy/actions/workflows/test.yml/badge.svg)](https://github.com/julienlegoux/kern-proxy/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/julienlegoux/kern-proxy/ai.svg)](https://pkg.go.dev/github.com/julienlegoux/kern-proxy/ai)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A unified LLM API for Go: one streaming interface across 35 providers —
Anthropic, OpenAI, Google (Gemini & Vertex), Mistral, AWS Bedrock, Azure,
GitHub Copilot, OpenAI Codex, OpenRouter, Groq, xAI, and more — with automatic
credential resolution (env keys and OAuth), token & cost tracking, and
conversations you can persist and hand off to a different model mid-session.

kern-proxy is a full-parity Go port of
[`@earendil-works/pi-ai`](https://github.com/earendil-works/pi/tree/main/packages/ai)
that tracks upstream over time.

## Features

- **One message model, every provider** — write against `ai.Context` /
  `ai.Message` once; adapters translate to each vendor's wire protocol
  (Anthropic Messages, OpenAI Completions & Responses, Gemini, Mistral,
  Bedrock Converse, …).
- **Unified streaming events** — text, thinking, and tool-call deltas arrive
  as one typed event protocol, ending in exactly one done/error event.
  Provider failures are in-band, never lost.
- **Auth that just works** — per-provider env keys, a cross-process-safe
  credential store, and real OAuth flows (Claude Pro/Max, GitHub Copilot,
  ChatGPT Plus/Pro) via the bundled `pi-ai login` CLI.
- **Cost & token accounting** — an embedded model catalog with per-model
  pricing, `ai.CalculateCost`, token estimation, and cache-read/write
  breakdowns.
- **Session persistence & model hand-off** — conversations are plain
  JSON-serializable `[]ai.Message`; resume any conversation on any provider
  ([how](docs/usage.md#session-persistence-and-model-hand-off)).
- **Tool calling, thinking levels, retry/overflow classifiers** — portable
  across providers, clamped to what each model supports.
- **Offline-testable** — the in-process `faux` provider exercises the full
  streaming contract without a network.

## Install

```sh
go get github.com/julienlegoux/kern-proxy
```

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/providers"
)

func main() {
	ctx := context.Background()

	// Reads ANTHROPIC_API_KEY (or stored OAuth credentials) automatically.
	models := providers.Models(nil)

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
		log.Fatal(err)
	}

	cost := ai.CalculateCost(model, &msg.Usage)
	fmt.Printf("\ntokens in=%d out=%d  $%.6f\n",
		msg.Usage.Input, msg.Usage.Output, cost.Total)
}
```

A conversation is just `[]ai.Message` and round-trips through `encoding/json`,
so saving a session and resuming it — on the same model or a different
provider — needs no extra machinery. See
[Session persistence and model hand-off](docs/usage.md#session-persistence-and-model-hand-off).

## Authentication

Set the provider's env var (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`,
`GEMINI_API_KEY`, …) or log in once over OAuth:

```sh
go run github.com/julienlegoux/kern-proxy/cmd/pi-ai login
```

Credentials land in `~/.pi/agent/auth.json` and are picked up (and refreshed)
automatically. Full details — resolution order, every env var, Bedrock/Vertex
specifics — in [docs/auth.md](docs/auth.md).

### Credential modes

The two paths carry different terms-of-service risk:

- **API keys** (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, AWS creds, Google ADC, …)
  — billed per token under a developer agreement written for programmatic
  access. No ToS risk. Use these in anything you ship.
- **Subscription OAuth** (Claude Pro/Max, ChatGPT Plus/Pro, GitHub Copilot) —
  `pi-ai login` yields a first-party client's credential, and kern-proxy then
  presents itself as that client (`user-agent: claude-cli/…`,
  `Editor-Version: vscode/…`). This is inherited upstream behavior and it is
  fine for personal use. Shipping it in a product means directing users to
  impersonate a first-party client against a subscription not licensed for
  programmatic access — providers can revoke the account.

See [docs/auth.md](docs/auth.md#credential-modes-and-terms-of-service-risk) for
the details.

## Documentation

| Doc | What's in it |
|---|---|
| [pkg.go.dev](https://pkg.go.dev/github.com/julienlegoux/kern-proxy/ai) | API reference with runnable examples |
| [docs/usage.md](docs/usage.md) | Streaming, tool calls, thinking levels, cost tracking, persistence & model hand-off, offline testing |
| [docs/architecture.md](docs/architecture.md) | Package layering, the unified message/event model, request flow, provider list |
| [docs/auth.md](docs/auth.md) | Env keys per provider, credential store, OAuth flows, the `pi-ai` CLI |
| [docs/PORTING.md](docs/PORTING.md) | Upstream→Go file mapping, intentional deviations, sync procedure |

## Development

```sh
go test ./... -race
```

Everything runs offline; live provider tests are gated behind environment API
keys, mirroring upstream.

## Upstream & parity

The port targets full functional parity with the upstream TypeScript package.
The pinned revision lives in `upstream/UPSTREAM.lock`; `upstream/sync.sh`
diffs upstream since the pin (also run weekly in CI, which opens an issue when
upstream moved). Every ported file carries a `// Ports:` header naming its
upstream source — see [docs/PORTING.md](docs/PORTING.md).

## License

[MIT](LICENSE). Ported from [`@earendil-works/pi-ai`](https://github.com/earendil-works/pi)
(MIT, © Mario Zechner) — see [NOTICE](NOTICE).
