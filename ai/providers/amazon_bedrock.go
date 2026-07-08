package providers

// Ports: packages/ai/src/providers/amazon-bedrock.ts. Bedrock auth is
// ambient: the AWS SDK's default credential chain handles the actual
// request signing (see ai/apis/bedrock/clientauth.go), so Resolve only
// reports whether the provider looks configured. A stored credential key is
// surfaced as a bearer token.

import (
	"context"

	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis/bedrock"
	"github.com/julienlegoux/kern-proxy/ai/catalog"
)

func resolveBedrockAuth(_ context.Context, input ai.APIKeyResolveInput) (*ai.AuthResult, error) {
	if input.Credential != nil && input.Credential.Key != "" {
		return &ai.AuthResult{Auth: ai.ModelAuth{APIKey: input.Credential.Key}, Source: "stored credential"}, nil
	}
	if input.Ctx.Env("AWS_BEARER_TOKEN_BEDROCK") != "" {
		return &ai.AuthResult{Auth: ai.ModelAuth{}, Source: "AWS_BEARER_TOKEN_BEDROCK"}, nil
	}
	if input.Ctx.Env("AWS_PROFILE") != "" {
		return &ai.AuthResult{Auth: ai.ModelAuth{}, Source: "AWS_PROFILE"}, nil
	}
	if input.Ctx.Env("AWS_ACCESS_KEY_ID") != "" && input.Ctx.Env("AWS_SECRET_ACCESS_KEY") != "" {
		return &ai.AuthResult{Auth: ai.ModelAuth{}, Source: "AWS access keys"}, nil
	}
	if input.Ctx.Env("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI") != "" {
		return &ai.AuthResult{Auth: ai.ModelAuth{}, Source: "ECS task role"}, nil
	}
	if input.Ctx.Env("AWS_CONTAINER_CREDENTIALS_FULL_URI") != "" {
		return &ai.AuthResult{Auth: ai.ModelAuth{}, Source: "ECS task role"}, nil
	}
	if input.Ctx.Env("AWS_WEB_IDENTITY_TOKEN_FILE") != "" {
		return &ai.AuthResult{Auth: ai.ModelAuth{}, Source: "web identity token"}, nil
	}
	return nil, nil
}

// AmazonBedrockProvider builds the Amazon Bedrock provider binding.
func AmazonBedrockProvider() ai.Provider {
	return ai.CreateProvider(ai.CreateProviderOptions{
		ID:   "amazon-bedrock",
		Name: "Amazon Bedrock",
		Auth: ai.ProviderAuth{APIKey: &ai.APIKeyAuth{
			Name:    "AWS credentials",
			Resolve: resolveBedrockAuth,
		}},
		Models: catalog.BuiltinModels("amazon-bedrock"),
		Api:    ai.StreamFuncs{StreamFunc: bedrock.Stream, StreamSimpleFunc: bedrock.StreamSimple},
	})
}
