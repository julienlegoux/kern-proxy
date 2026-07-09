// Package ai is a unified LLM API for Go: one message model and one
// streaming event protocol across 35 providers — Anthropic, OpenAI, Google
// (Gemini & Vertex), Mistral, AWS Bedrock, Azure, GitHub Copilot, OpenAI
// Codex, OpenRouter, Groq, xAI, and more. It is a full-parity port of the
// TypeScript @earendil-works/pi-ai package that tracks upstream over time.
//
// Most consumers start from the providers subpackage, which registers every
// built-in provider and resolves credentials (env API keys or stored OAuth)
// automatically:
//
//	models := providers.Models(nil)
//	model := models.GetModel("anthropic", "claude-sonnet-4-5")
//	stream := models.Stream(ctx, model, ai.Context{
//		Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("Hello!")}},
//	}, nil)
//	msg, err := stream.Result(ctx)
//
// # Core types
//
// A conversation is a [Context]: a system prompt, a [Message] history, and
// optional [Tool] declarations. Messages are a closed union — [UserMessage],
// [AssistantMessage], [ToolResultMessage] — discriminated by a "role" field,
// so an entire session round-trips through encoding/json and can be resumed
// on any provider (see [Messages] and the package examples).
//
// # Streaming
//
// Every request returns a [Stream]. Consume typed events ([TextDeltaEvent],
// [ThinkingDeltaEvent], [ToolCallDeltaEvent], …) from [Stream.Events], or
// just block on [Stream.Result] for the final [AssistantMessage]. Streams
// terminate with exactly one done/error event; provider failures arrive
// in-band via the message's StopReason and ErrorMessage, never as a lost
// goroutine or a silent hang.
//
// # Cost and tokens
//
// [AssistantMessage.Usage] carries unified token accounting, including
// cache-read/write splits. [CalculateCost] prices a [Usage] against the
// model's per-million-token price sheet; the embedded catalog
// (ai/catalog subpackage) supplies pricing for every known model.
//
// # Testing without a network
//
// The ai/providers/faux subpackage is an in-process provider that scripts
// canned responses and replays the full streaming event protocol — the
// executable specification this package's examples run against.
package ai
