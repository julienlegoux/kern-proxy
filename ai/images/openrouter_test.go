package images

// Ports: packages/ai/test/openrouter-images.test.ts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/julienlegoux/kern-link/ai"
)

func TestGenerateImagesOpenRouter_ReturnsTextPlusImagesInFinalOutput(t *testing.T) {
	var gotBody map[string]any
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "img-1",
			"usage": {"prompt_tokens": 12, "completion_tokens": 34, "prompt_tokens_details": {"cached_tokens": 0}},
			"choices": [{"message": {"content": "Here is your image.", "images": [{"image_url": "data:image/png;base64,ZmFrZS1wbmc="}]}}]
		}`))
	}))
	defer srv.Close()

	model := &Model{
		ID:       "google/gemini-3.1-flash-image-preview",
		Name:     "Gemini 3.1 Flash Image Preview",
		Api:      "openrouter-images",
		Provider: "openrouter",
		BaseURL:  srv.URL,
		Input:    []ai.Modality{ai.ModalityText, ai.ModalityImage},
		Output:   []ai.Modality{ai.ModalityText, ai.ModalityImage},
		Cost:     ai.ModelCost{Input: 0.015, Output: 0.03},
		Headers:  map[string]string{"HTTP-Referer": "https://example.com"},
	}
	imgCtx := Context{Input: []ai.UserContentPart{ai.TextContent{Text: "Generate a dog"}}}

	out := generateImagesOpenRouter(context.Background(), model, imgCtx, &Options{APIKey: "test"})

	if out.StopReason != StopReasonStop {
		t.Fatalf("StopReason = %q, want %q (error: %s)", out.StopReason, StopReasonStop, out.ErrorMessage)
	}
	if out.ResponseID != "img-1" {
		t.Fatalf("ResponseID = %q, want %q", out.ResponseID, "img-1")
	}
	if len(out.Output) != 2 {
		t.Fatalf("Output = %#v, want 2 blocks", out.Output)
	}
	text, ok := out.Output[0].(ai.TextContent)
	if !ok || text.Text != "Here is your image." {
		t.Fatalf("Output[0] = %#v, want text %q", out.Output[0], "Here is your image.")
	}
	img, ok := out.Output[1].(ai.ImageContent)
	if !ok || img.MimeType != "image/png" || img.Data != "ZmFrZS1wbmc=" {
		t.Fatalf("Output[1] = %#v, want image/png ZmFrZS1wbmc=", out.Output[1])
	}

	if gotAuth != "Bearer test" {
		t.Fatalf("Authorization header = %q, want %q", gotAuth, "Bearer test")
	}
	if stream, _ := gotBody["stream"].(bool); stream {
		t.Fatalf("stream = %v, want false", gotBody["stream"])
	}
	modalities, _ := gotBody["modalities"].([]any)
	if len(modalities) != 2 || modalities[0] != "image" || modalities[1] != "text" {
		t.Fatalf("modalities = %#v, want [image text]", gotBody["modalities"])
	}
}

func TestGenerateImagesOpenRouter_PassesThroughAbortSignalAndReturnsAbortedResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	model := &Model{
		ID:       "black-forest-labs/flux.2-pro",
		Api:      "openrouter-images",
		Provider: "openrouter",
		BaseURL:  srv.URL,
		Output:   []ai.Modality{ai.ModalityImage},
	}
	imgCtx := Context{Input: []ai.UserContentPart{ai.TextContent{Text: "Generate a dog"}}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	out := generateImagesOpenRouter(ctx, model, imgCtx, &Options{APIKey: "test"})

	if out.StopReason != StopReasonAborted {
		t.Fatalf("StopReason = %q, want %q", out.StopReason, StopReasonAborted)
	}
	if out.ErrorMessage == "" {
		t.Fatalf("ErrorMessage is empty, want a non-empty message")
	}
}

func TestGenerateImagesOpenRouter_NoAPIKeyReturnsErrorResult(t *testing.T) {
	model := &Model{ID: "m", Api: "openrouter-images", Provider: "openrouter", BaseURL: "https://example.test"}
	imgCtx := Context{Input: []ai.UserContentPart{ai.TextContent{Text: "hi"}}}

	out := generateImagesOpenRouter(context.Background(), model, imgCtx, nil)

	if out.StopReason != StopReasonError {
		t.Fatalf("StopReason = %q, want %q", out.StopReason, StopReasonError)
	}
	if !strings.Contains(out.ErrorMessage, "No API key") {
		t.Fatalf("ErrorMessage = %q, want it to mention a missing API key", out.ErrorMessage)
	}
}

func TestGenerateImagesOpenRouter_NonOKStatusReturnsErrorResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"nope"}`))
	}))
	defer srv.Close()

	model := &Model{ID: "m", Api: "openrouter-images", Provider: "openrouter", BaseURL: srv.URL, Output: []ai.Modality{ai.ModalityImage}}
	imgCtx := Context{Input: []ai.UserContentPart{ai.TextContent{Text: "hi"}}}

	out := generateImagesOpenRouter(context.Background(), model, imgCtx, &Options{APIKey: "test"})

	if out.StopReason != StopReasonError {
		t.Fatalf("StopReason = %q, want %q", out.StopReason, StopReasonError)
	}
	if !strings.Contains(out.ErrorMessage, "403") {
		t.Fatalf("ErrorMessage = %q, want it to mention status 403", out.ErrorMessage)
	}
}

func TestGenerateImagesOpenRouter_ResolvesFinalAssistantImagesResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"images":[{"image_url":{"url":"data:image/png;base64,ZmFrZQ=="}}]}}]}`))
	}))
	defer srv.Close()

	model := &Model{ID: "black-forest-labs/flux.2-pro", Api: "openrouter-images", Provider: "openrouter", BaseURL: srv.URL, Output: []ai.Modality{ai.ModalityImage}}
	imgCtx := Context{Input: []ai.UserContentPart{ai.TextContent{Text: "Generate a dog"}}}

	out := generateImagesOpenRouter(context.Background(), model, imgCtx, &Options{APIKey: "test"})

	hasImage := false
	for _, item := range out.Output {
		if _, ok := item.(ai.ImageContent); ok {
			hasImage = true
		}
	}
	if !hasImage {
		t.Fatalf("Output = %#v, want at least one image block", out.Output)
	}
}

func TestGenerateImagesOpenRouter_RegisteredUnderOpenRouterImagesAPI(t *testing.T) {
	if fn := GetAPIProvider("openrouter-images"); fn == nil {
		t.Fatal("openrouter-images adapter is not registered")
	}
}

func TestGenerateImagesOpenRouter_LiveSmoke(t *testing.T) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		t.Skip("OPENROUTER_API_KEY not set; skipping live smoke test")
	}

	model := CatalogModel("openrouter", "google/gemini-2.5-flash-image")
	if model == nil {
		t.Fatal("catalog model google/gemini-2.5-flash-image not found")
	}
	imgCtx := Context{
		Input: []ai.UserContentPart{ai.TextContent{Text: "Generate a simple red circle on a plain white background. No text."}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	out := GenerateImages(ctx, model, imgCtx, &Options{APIKey: apiKey})

	if out.StopReason != StopReasonStop {
		t.Fatalf("live smoke returned stop reason %q: %s", out.StopReason, out.ErrorMessage)
	}
	hasImage := false
	for _, item := range out.Output {
		if _, ok := item.(ai.ImageContent); ok {
			hasImage = true
		}
	}
	if !hasImage {
		t.Fatalf("live smoke returned no image output")
	}
}
