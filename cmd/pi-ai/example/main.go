// Command toolcall-example is the small end-to-end example program this
// issue's acceptance criteria call for: it streams one user turn offering a
// get_weather tool through the in-process faux provider by default, or a
// real built-in provider when that provider's API key is present in the
// environment, and prints the resulting transcript.
//
// The round-trip logic lives in cmd/pi-ai/toolcallexample, tested directly
// (a deterministic faux-provider test, plus a GEMINI_API_KEY-gated live
// smoke test); this program is a thin runnable wrapper over that package.
package main

// Ports: none — original: runnable wrapper demoing a tool-call round trip
// over cmd/pi-ai/toolcallexample.

import (
	"context"
	"fmt"
	"os"

	"github.com/kern-ia/kern-link/ai"
	"github.com/kern-ia/kern-link/ai/providers"
	"github.com/kern-ia/kern-link/ai/providers/faux"
	"github.com/kern-ia/kern-link/cmd/pi-ai/toolcallexample"
)

const prompt = "What is the weather in Paris? Use the get_weather tool, then answer in one short sentence."

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run() error {
	models, model, apiKey, label := selectBackend()
	fmt.Printf("Running the tool-call round trip against %s (%s)...\n\n", label, model.ID)

	final, err := toolcallexample.Run(context.Background(), models, model, prompt, nil, apiKey)
	if err != nil {
		return err
	}

	fmt.Printf("Final stopReason: %s\n", final.StopReason)
	for _, block := range final.Content {
		if text, ok := block.(ai.TextContent); ok {
			fmt.Println(text.Text)
		}
	}
	return nil
}

// selectBackend uses the real Google provider when GEMINI_API_KEY is
// present (the same env-gated pattern as this repo's live smoke tests),
// falling back to the in-process faux provider — scripted with a canned
// tool-call-then-answer response — so the example always runs offline too.
func selectBackend() (models ai.Models, model *ai.Model, apiKey, label string) {
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		all := providers.Models(nil)
		if m := all.GetModel("google", "gemini-2.5-flash"); m != nil {
			// The embedded catalog's baseUrl already includes "/v1beta"
			// (ai/catalog/data/models/google.json), but ai/apis/google's
			// requestURL appends "/v1beta/models/..." itself, doubling the
			// path segment and 404ing. Pass a corrected copy for this
			// request only; fixing the catalog/adapter mismatch itself is
			// outside this CLI issue's scope.
			fixed := *m
			fixed.BaseURL = "https://generativelanguage.googleapis.com"
			return all, &fixed, key, "Google Gemini (live)"
		}
	}

	handle := faux.New(nil)
	handle.SetResponses(
		faux.Step(faux.AssistantMessage(
			[]ai.AssistantContentPart{faux.ToolCall("get_weather", map[string]any{"location": "Paris"}, nil)},
			&faux.AssistantMessageOptions{StopReason: ai.StopReasonToolUse},
		)),
		faux.Step(faux.TextMessage("It's sunny and 22C in Paris.", nil)),
	)
	fauxModels := ai.CreateModels(nil)
	fauxModels.SetProvider(handle.Provider)
	return fauxModels, handle.GetModel(""), "", "the in-process faux provider"
}
