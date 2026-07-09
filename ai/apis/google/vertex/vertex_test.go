package vertex

// Ports test/google-vertex-api-key-resolution.test.ts. Upstream asserts the
// @google/genai SDK constructor call shape (vertexai/project/location/apiKey/
// httpOptions); this Go port has no SDK and speaks the REST API directly, so
// the equivalent assertions are the request URL and auth header actually
// sent, captured with an httptest server (same idiom as the sibling
// google/anthropic/openairesponses packages).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/julienlegoux/kern-proxy/ai"
)

func testModel(baseURL string) *ai.Model {
	return &ai.Model{
		ID:            "gemini-3-flash-preview",
		Name:          "Gemini 3 Flash Preview",
		Api:           ai.ApiGoogleVertex,
		Provider:      "google-vertex",
		BaseURL:       baseURL,
		Input:         []ai.Modality{ai.ModalityText, ai.ModalityImage},
		ContextWindow: 1000000,
		MaxTokens:     8192,
	}
}

func testChat() ai.Context {
	return ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hello"), Timestamp: time.Now().UnixMilli()}}}
}

// --- request URL building ---

func TestRequestURL_APIKeyPathIsProjectless(t *testing.T) {
	model := testModel("")
	url := requestURL(model, true, "", "")
	want := "https://aiplatform.googleapis.com/v1/publishers/google/models/gemini-3-flash-preview:streamGenerateContent?alt=sse"
	if url != want {
		t.Errorf("url = %q, want %q", url, want)
	}
}

func TestRequestURL_ADCPathIsProjectScoped(t *testing.T) {
	model := testModel("")
	url := requestURL(model, false, "test-project", "us-central1")
	want := "https://us-central1-aiplatform.googleapis.com/v1/projects/test-project/locations/us-central1/publishers/google/models/gemini-3-flash-preview:streamGenerateContent?alt=sse"
	if url != want {
		t.Errorf("url = %q, want %q", url, want)
	}
}

// TestRequestURL_IgnoresTemplateBaseURL ports "does not forward generated
// Vertex base URL placeholders": a catalog baseUrl still containing the
// unsubstituted "{location}" template must not leak into the request URL.
func TestRequestURL_IgnoresTemplateBaseURL(t *testing.T) {
	model := testModel("https://{location}-aiplatform.googleapis.com")
	url := requestURL(model, false, "test-project", "us-central1")
	want := "https://us-central1-aiplatform.googleapis.com/v1/projects/test-project/locations/us-central1/publishers/google/models/gemini-3-flash-preview:streamGenerateContent?alt=sse"
	if url != want {
		t.Errorf("url = %q, want %q (template placeholder must not leak through)", url, want)
	}
}

// TestRequestURL_HonorsCustomBaseURL ports "forwards custom baseUrl to the
// ADC/API key client": a genuine override replaces the default host.
func TestRequestURL_HonorsCustomBaseURL(t *testing.T) {
	model := testModel("https://proxy.example.com")
	url := requestURL(model, false, "test-project", "us-central1")
	want := "https://proxy.example.com/v1/projects/test-project/locations/us-central1/publishers/google/models/gemini-3-flash-preview:streamGenerateContent?alt=sse"
	if url != want {
		t.Errorf("url = %q, want %q", url, want)
	}
}

// --- headers ---

func TestBuildHeaders_APIKeyPathSetsXGoogAPIKey(t *testing.T) {
	headers := buildHeaders(testModel(""), nil, "my-key")
	if headers["x-goog-api-key"] != "my-key" {
		t.Errorf("headers = %#v, want x-goog-api-key=my-key", headers)
	}
	if _, ok := headers["authorization"]; ok {
		t.Errorf("headers = %#v, want no authorization header on the API-key path", headers)
	}
}

func TestBuildADCHeaders_SetsBearerAuthorization(t *testing.T) {
	headers := buildADCHeaders(testModel(""), nil, "fake-adc-token")
	if headers["authorization"] != "Bearer fake-adc-token" {
		t.Errorf("headers = %#v, want authorization=Bearer fake-adc-token", headers)
	}
	if _, ok := headers["x-goog-api-key"]; ok {
		t.Errorf("headers = %#v, want no x-goog-api-key header on the ADC path", headers)
	}
}

// --- Stream integration ---

func TestStream_APIKeyPathHitsProjectlessEndpointWithAPIKeyHeader(t *testing.T) {
	var gotPath, gotAPIKey, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("x-goog-api-key")
		gotAuth = r.Header.Get("authorization")
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}]}` + "\n\n"))
	}))
	defer srv.Close()

	model := testModel(srv.URL)
	stream := Stream(context.Background(), model, testChat(), &ai.StreamOptions{APIKey: "AIzaSyExampleRealisticLookingApiKey123456"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonStop {
		t.Fatalf("stopReason = %q (err=%q), want stop", result.StopReason, result.ErrorMessage)
	}
	if gotPath != "/v1/publishers/google/models/gemini-3-flash-preview:streamGenerateContent" {
		t.Errorf("path = %q, want the projectless publisher path", gotPath)
	}
	if gotAPIKey != "AIzaSyExampleRealisticLookingApiKey123456" {
		t.Errorf("x-goog-api-key = %q, want the configured key", gotAPIKey)
	}
	if gotAuth != "" {
		t.Errorf("authorization = %q, want empty on the API-key path", gotAuth)
	}
}

func TestStream_ADCPathHitsProjectScopedEndpointWithBearerToken(t *testing.T) {
	prev := adcTokenFunc
	t.Cleanup(func() { adcTokenFunc = prev })
	adcTokenFunc = func(ctx context.Context) (string, error) { return "fake-adc-token", nil }

	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("authorization")
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}]}` + "\n\n"))
	}))
	defer srv.Close()

	model := testModel(srv.URL)
	opts := &ai.StreamOptions{GoogleVertexProject: "test-project", GoogleVertexLocation: "us-central1"}
	stream := Stream(context.Background(), model, testChat(), opts)
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonStop {
		t.Fatalf("stopReason = %q (err=%q), want stop", result.StopReason, result.ErrorMessage)
	}
	if gotPath != "/v1/projects/test-project/locations/us-central1/publishers/google/models/gemini-3-flash-preview:streamGenerateContent" {
		t.Errorf("path = %q, want the project-scoped path", gotPath)
	}
	if gotAuth != "Bearer fake-adc-token" {
		t.Errorf("authorization = %q, want Bearer fake-adc-token", gotAuth)
	}
}

// TestStream_MarkerAPIKeyFallsBackToADC ports "falls back to ADC when
// options.apiKey is the gcp-vertex-credentials marker" /
// "...is a placeholder marker".
func TestStream_MarkerAPIKeyFallsBackToADC(t *testing.T) {
	for _, marker := range []string{"gcp-vertex-credentials", "<authenticated>", ""} {
		t.Run(marker, func(t *testing.T) {
			prev := adcTokenFunc
			t.Cleanup(func() { adcTokenFunc = prev })
			adcTokenFunc = func(ctx context.Context) (string, error) { return "fake-adc-token", nil }

			var gotAuth string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get("authorization")
				w.Header().Set("content-type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}]}` + "\n\n"))
			}))
			defer srv.Close()

			model := testModel(srv.URL)
			opts := &ai.StreamOptions{APIKey: marker, GoogleVertexProject: "test-project", GoogleVertexLocation: "us-central1"}
			stream := Stream(context.Background(), model, testChat(), opts)
			result, err := stream.Result(context.Background())
			if err != nil {
				t.Fatalf("Result: %v", err)
			}
			if result.StopReason != ai.StopReasonStop {
				t.Fatalf("stopReason = %q (err=%q), want stop", result.StopReason, result.ErrorMessage)
			}
			if gotAuth != "Bearer fake-adc-token" {
				t.Errorf("authorization = %q, want Bearer fake-adc-token (ADC fallback)", gotAuth)
			}
		})
	}
}

// TestStream_ADCPathErrorsWithoutProject ports the "Vertex AI requires a
// project ID" error path: no ambient GOOGLE_CLOUD_PROJECT/GCLOUD_PROJECT and
// no options.project.
func TestStream_ADCPathErrorsWithoutProject(t *testing.T) {
	model := testModel("https://example.invalid")
	stream := Stream(context.Background(), model, testChat(), &ai.StreamOptions{GoogleVertexLocation: "us-central1"})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonError {
		t.Fatalf("stopReason = %q, want error", result.StopReason)
	}
	if result.ErrorMessage == "" {
		t.Error("errorMessage = \"\", want a project-required message")
	}
}

// --- buildParams (representative coverage; full thinking-config matrix is
// already exercised by the sibling ai/apis/google package, whose helpers this
// duplicates per upstream's own google-vertex.ts/google-generative-ai.ts
// duplication) ---

func TestBuildParams_ReusesSharedConverters(t *testing.T) {
	model := testModel("")
	chat := ai.Context{
		SystemPrompt: "be helpful",
		Messages:     []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}},
	}
	params := buildParams(model, chat, &ai.StreamOptions{})
	if len(params.Contents) != 1 {
		t.Fatalf("contents = %#v, want 1", params.Contents)
	}
	if params.SystemInstruction == nil {
		t.Fatal("systemInstruction = nil, want set")
	}
	b, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("marshaled params are empty")
	}
}

func TestBuildParams_ThinkingBudgetForNonGemini3Model(t *testing.T) {
	model := testModel("")
	model.ID = "gemini-2.5-pro"
	model.Reasoning = true
	on := true
	budget := 2048
	params := buildParams(model, testChat(), &ai.StreamOptions{ThinkingEnabled: &on, ThinkingBudgetTokens: &budget})
	if params.GenerationConfig.ThinkingConfig == nil {
		t.Fatalf("thinkingConfig = nil, want set")
	}
	if params.GenerationConfig.ThinkingConfig.ThinkingBudget == nil || *params.GenerationConfig.ThinkingConfig.ThinkingBudget != 2048 {
		t.Errorf("thinkingBudget = %v, want 2048", params.GenerationConfig.ThinkingConfig.ThinkingBudget)
	}
}

func TestBuildParams_ThinkingLevelForGemini3ProModel(t *testing.T) {
	model := testModel("")
	model.ID = "gemini-3-pro"
	model.Reasoning = true
	on := true
	params := buildParams(model, testChat(), &ai.StreamOptions{ThinkingEnabled: &on, GoogleThinkingLevel: ai.GoogleThinkingLevelHigh})
	if params.GenerationConfig.ThinkingConfig == nil || params.GenerationConfig.ThinkingConfig.ThinkingLevel != "HIGH" {
		t.Errorf("thinkingConfig = %#v, want thinkingLevel HIGH", params.GenerationConfig.ThinkingConfig)
	}
}

func TestBuildParams_ThinkingDisabledUsesBudgetZeroForBudgetModel(t *testing.T) {
	model := testModel("")
	model.ID = "gemini-2.5-pro"
	model.Reasoning = true
	off := false
	params := buildParams(model, testChat(), &ai.StreamOptions{ThinkingEnabled: &off})
	if params.GenerationConfig.ThinkingConfig == nil || params.GenerationConfig.ThinkingConfig.ThinkingBudget == nil || *params.GenerationConfig.ThinkingConfig.ThinkingBudget != 0 {
		t.Errorf("thinkingConfig = %#v, want thinkingBudget=0", params.GenerationConfig.ThinkingConfig)
	}
}

func TestStreamSimple_UsesBudgetBasedThinkingByDefault(t *testing.T) {
	prev := adcTokenFunc
	t.Cleanup(func() { adcTokenFunc = prev })
	adcTokenFunc = func(ctx context.Context) (string, error) { return "fake-adc-token", nil }

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make(map[string]any)
		_ = json.NewDecoder(r.Body).Decode(&body)
		config, _ := body["generationConfig"].(map[string]any)
		thinkingConfig, ok := config["thinkingConfig"].(map[string]any)
		if !ok {
			t.Errorf("body = %#v, want generationConfig.thinkingConfig", body)
		} else if _, hasBudget := thinkingConfig["thinkingBudget"]; !hasBudget {
			t.Errorf("thinkingConfig = %#v, want thinkingBudget for a 2.5-pro model", thinkingConfig)
		}
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}]}` + "\n\n"))
	}))
	defer srv.Close()

	model := testModel(srv.URL)
	model.ID = "gemini-2.5-pro"
	model.Reasoning = true
	opts := &ai.SimpleStreamOptions{
		StreamOptions: ai.StreamOptions{GoogleVertexProject: "test-project", GoogleVertexLocation: "us-central1"},
		Reasoning:     ai.ThinkingMedium,
	}
	stream := StreamSimple(context.Background(), model, testChat(), opts)
	if _, err := stream.Result(context.Background()); err != nil {
		t.Fatalf("Result: %v", err)
	}
}
