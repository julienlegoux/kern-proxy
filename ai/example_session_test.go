package ai_test

// Runnable versions of the session serialize/restore example in
// docs/usage.md. Keeping them here means the documented snippet is compiled
// and its output asserted on every `go test ./...` run, so it cannot drift
// away from the API the way a fenced code block silently does.

import (
	"encoding/json"
	"fmt"

	"github.com/julienlegoux/kern-link/ai"
)

// ExampleMessages shows the round trip a caller persists a conversation with:
// marshal []ai.Message, store the bytes, decode them back into ai.Messages.
// The "role" discriminator on every message is what lets the polymorphic
// slice decode without the caller tracking types.
func ExampleMessages() {
	messages := []ai.Message{
		&ai.UserMessage{Content: ai.UserText("What is 2+2?"), Timestamp: 1},
		&ai.AssistantMessage{
			Content:    []ai.AssistantContentPart{ai.TextContent{Text: "4"}},
			Api:        ai.ApiAnthropicMessages,
			Provider:   "anthropic",
			Model:      "claude-haiku-4-5",
			StopReason: ai.StopReasonStop,
			Timestamp:  2,
		},
	}

	raw, err := json.Marshal(ai.Messages(messages))
	if err != nil {
		panic(err)
	}

	var restored ai.Messages
	if err := json.Unmarshal(raw, &restored); err != nil {
		panic(err)
	}

	for _, m := range restored {
		fmt.Printf("%s: %T\n", m.MessageRole(), m)
	}
	// Output:
	// user: *ai.UserMessage
	// assistant: *ai.AssistantMessage
}

// ExampleContext_UnmarshalJSON shows the other half: a whole ai.Context —
// system prompt, messages, tools — is itself a JSON document, so a session
// can be stored and resumed as one value rather than reassembled field by
// field.
func ExampleContext_UnmarshalJSON() {
	saved := []byte(`{
		"systemPrompt": "Be brief.",
		"messages": [
			{"role": "user", "content": "What is 2+2?", "timestamp": 1},
			{"role": "assistant", "content": [{"type": "text", "text": "4"}],
			 "api": "anthropic-messages", "provider": "anthropic",
			 "model": "claude-haiku-4-5", "usage": {}, "stopReason": "stop", "timestamp": 2}
		]
	}`)

	var chat ai.Context
	if err := json.Unmarshal(saved, &chat); err != nil {
		panic(err)
	}

	// Continue the conversation: append the next turn and hand the whole
	// context to any model, including one from a different provider.
	chat.Messages = append(chat.Messages, &ai.UserMessage{
		Content:   ai.UserText("And 3+3?"),
		Timestamp: 3,
	})

	fmt.Println(chat.SystemPrompt)
	fmt.Println(len(chat.Messages), "messages")
	fmt.Println(chat.Messages[1].(*ai.AssistantMessage).Content[0].(ai.TextContent).Text)
	// Output:
	// Be brief.
	// 3 messages
	// 4
}
