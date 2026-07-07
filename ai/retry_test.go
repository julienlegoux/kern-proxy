package ai

// Ports: packages/ai/test/retry.test.ts

import "testing"

func errorMsg(errorMessage string) *AssistantMessage {
	return &AssistantMessage{
		Api:          ApiOpenAICompletions,
		Provider:     "faux",
		Model:        "faux-model",
		StopReason:   StopReasonError,
		ErrorMessage: errorMessage,
	}
}

func TestRetryClassification(t *testing.T) {
	openAIExplicitRetry := "An error occurred while processing your request. You can retry your request, or contact us through our help center at help.openai.com if the error persists. Please include the request ID req_******** in your message."
	bedrockExplicitRetry := `{"message":"The system encountered an unexpected error during processing. Try your request again."}`

	cases := []struct {
		name string
		msg  *AssistantMessage
		want bool
	}{
		{"openai explicit retry guidance", errorMsg(openAIExplicitRetry), true},
		{"bedrock explicit retry guidance", errorMsg(bedrockExplicitRetry), true},
		{"provider limit errors stay non-retryable", errorMsg("429 quota exceeded"), false},
		{"overloaded", errorMsg("overloaded_error"), true},
		{"524 no body", errorMsg("524 status code (no body)"), true},
		{"non-error message", &AssistantMessage{StopReason: StopReasonStop}, false},
		{"error without message", &AssistantMessage{StopReason: StopReasonError}, false},
		{"insufficient quota", errorMsg("insufficient_quota: your account ran out"), false},
		{"monthly usage limit", errorMsg("GoUsageLimitError: Monthly usage limit reached"), false},
		{"premature anthropic stream end", errorMsg("Anthropic stream ended before message_stop"), true},
		{"websocket closed", errorMsg("websocket closed unexpectedly"), true},
		{"socket hang up", errorMsg("socket hang up"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsRetryableAssistantError(tc.msg); got != tc.want {
				t.Errorf("IsRetryableAssistantError(%q) = %v, want %v", tc.msg.ErrorMessage, got, tc.want)
			}
		})
	}
}
