package bedrock

// Ports addCustomHeadersMiddleware from bedrock-converse-stream.ts: a
// build-step middleware attaching caller-supplied custom headers to the
// outgoing Bedrock request. The Build step runs after request serialization
// but before SigV4 signing (a Finalize-step concern in aws-sdk-go-v2, see
// bedrockruntime/auth.go's signRequestMiddleware), so injected headers are
// covered by the signature -- matching upstream's own `step: "build"`
// registration. Reserved SigV4/auth headers (x-amz-*, authorization, host)
// are silently skipped case-insensitively; every other caller header
// overrides any existing same-named header on the request.
//
// Ports: packages/ai/src/api/bedrock-converse-stream.ts

import (
	"context"
	"strings"

	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// customHeadersMiddlewareID names the registered Build-step middleware.
// Ports the MIDDLEWARE_NAME constant ("pi-ai-custom-headers"), renamed for
// this port.
const customHeadersMiddlewareID = "kern-proxy-bedrock-custom-headers"

// isReservedBedrockHeader reports whether key participates in SigV4 signing
// or bearer-token auth, and so must never be overwritten by a
// caller-supplied header. Ports isReservedHeader.
func isReservedBedrockHeader(key string) bool {
	lower := strings.ToLower(key)
	return lower == "authorization" || lower == "host" || strings.HasPrefix(lower, "x-amz-")
}

// addCustomHeadersMiddleware returns a bedrockruntime.Options.APIOptions
// entry that registers the custom-headers Build middleware on a client's
// middleware stack. Ports the client.middlewareStack.add(...) call at the
// end of addCustomHeadersMiddleware.
func addCustomHeadersMiddleware(headers map[string]string) func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Build.Add(customHeadersBuildMiddleware(headers), middleware.After)
	}
}

// customHeadersBuildMiddleware builds the Build-step middleware itself,
// factored out from addCustomHeadersMiddleware so it's directly invokable in
// tests without a full middleware.Stack (mirroring how the upstream test
// captures and calls the registered handler directly).
func customHeadersBuildMiddleware(headers map[string]string) middleware.BuildMiddleware {
	return middleware.BuildMiddlewareFunc(customHeadersMiddlewareID, func(
		ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler,
	) (middleware.BuildOutput, middleware.Metadata, error) {
		if req, ok := in.Request.(*smithyhttp.Request); ok {
			for key, value := range headers {
				if !isReservedBedrockHeader(key) {
					req.Header.Set(key, value)
				}
			}
		}
		return next.HandleBuild(ctx, in)
	})
}
