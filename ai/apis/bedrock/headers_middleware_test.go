package bedrock

// Ports test/bedrock-custom-headers.test.ts. Upstream drives a full
// (mocked) BedrockRuntimeClient through streamBedrock/streamSimpleBedrock
// and inspects middlewareStack.add() calls; that mock exists only because JS
// has no equivalent to smithy-go's typed BuildMiddleware. Here the
// middleware closure itself is directly invokable (mirroring the TS test's
// own `reg.handler(nextSpy)(fakeArgs)` step), so these tests call it
// directly against a fake smithy Build request/handler instead of driving a
// whole Stream call through a mocked SDK client.

import (
	"context"
	"testing"

	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

func fakeBuildHandler(called *bool) middleware.BuildHandler {
	return middleware.BuildHandlerFunc(func(ctx context.Context, in middleware.BuildInput) (middleware.BuildOutput, middleware.Metadata, error) {
		*called = true
		return middleware.BuildOutput{}, middleware.Metadata{}, nil
	})
}

func newFakeBuildRequest() *smithyhttp.Request {
	return smithyhttp.NewStackRequest().(*smithyhttp.Request)
}

func TestCustomHeadersMiddleware_InjectsCallerHeaderHappyPath(t *testing.T) {
	mw := customHeadersBuildMiddleware(map[string]string{"x-custom": "v"})
	req := newFakeBuildRequest()
	var nextCalled bool

	_, _, err := mw.HandleBuild(context.Background(), middleware.BuildInput{Request: req}, fakeBuildHandler(&nextCalled))
	if err != nil {
		t.Fatalf("HandleBuild: %v", err)
	}
	if !nextCalled {
		t.Error("expected next handler to be called")
	}
	if got := req.Header.Get("x-custom"); got != "v" {
		t.Errorf(`Header.Get("x-custom") = %q, want "v"`, got)
	}
}

func TestCustomHeadersMiddleware_SkipsReservedHeadersCaseInsensitively(t *testing.T) {
	mw := customHeadersBuildMiddleware(map[string]string{
		"authorization": "evil",
		"x-amz-date":    "evil",
		"x-allowed":     "ok",
		"Authorization": "evil2",
		"X-Amz-Date":    "evil2",
		"HOST":          "evil3",
	})
	req := newFakeBuildRequest()
	req.Header.Set("Authorization", "real-auth")
	req.Header.Set("X-Amz-Date", "real-date")
	req.Header.Set("Host", "real-host")
	var nextCalled bool

	_, _, err := mw.HandleBuild(context.Background(), middleware.BuildInput{Request: req}, fakeBuildHandler(&nextCalled))
	if err != nil {
		t.Fatalf("HandleBuild: %v", err)
	}
	if !nextCalled {
		t.Error("expected next handler to be called")
	}

	if got := req.Header.Get("Authorization"); got != "real-auth" {
		t.Errorf("Authorization = %q, want unchanged real-auth", got)
	}
	if got := req.Header.Get("X-Amz-Date"); got != "real-date" {
		t.Errorf("X-Amz-Date = %q, want unchanged real-date", got)
	}
	if got := req.Header.Get("Host"); got != "real-host" {
		t.Errorf("Host = %q, want unchanged real-host", got)
	}
	if got := req.Header.Get("x-allowed"); got != "ok" {
		t.Errorf("x-allowed = %q, want ok", got)
	}
}

func TestCustomHeadersMiddleware_NonSmithyRequestPassesThroughUnchanged(t *testing.T) {
	mw := customHeadersBuildMiddleware(map[string]string{"x-custom": "v"})
	var nextCalled bool

	// A structural guard mirroring VC3's "passes through unchanged when the
	// request has no headers" case: an unrecognized Request type must not
	// panic, and must still call next.
	_, _, err := mw.HandleBuild(context.Background(), middleware.BuildInput{Request: "not-a-smithy-request"}, fakeBuildHandler(&nextCalled))
	if err != nil {
		t.Fatalf("HandleBuild: %v", err)
	}
	if !nextCalled {
		t.Error("expected next handler to be called even for a non-smithy request")
	}
}

func TestAddCustomHeadersMiddleware_RegistersOnBuildStep(t *testing.T) {
	stack := middleware.NewStack("test", smithyhttp.NewStackRequest)

	if err := addCustomHeadersMiddleware(map[string]string{"x-custom": "v"})(stack); err != nil {
		t.Fatalf("addCustomHeadersMiddleware: %v", err)
	}

	if _, ok := stack.Build.Get(customHeadersMiddlewareID); !ok {
		t.Error("expected the custom-headers middleware to be registered on the Build step")
	}
}
