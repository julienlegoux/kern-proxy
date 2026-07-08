package bedrock

// Builds the real bedrockruntime.Client from a resolved clientConfig,
// applying the AWS auth matrix resolveClientConfig computed: an explicit
// region/profile/static-credentials override onto the SDK's default
// credential chain, an explicit endpoint when pinned, bearer-token auth
// (BearerAuthTokenProvider + AuthSchemePreference -- see clientauth.go's
// package doc for the risk #3 finding that this is SDK-native), and the
// custom-headers middleware when the caller supplied any.
//
// Ports the `new BedrockRuntimeClient(config)` call (plus the
// immediately-following custom-headers middleware registration) from
// bedrock-converse-stream.ts's stream function.
//
// Ports: packages/ai/src/api/bedrock-converse-stream.ts

import (
	"context"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/smithy-go/auth/bearer"
)

// newBedrockRuntimeClient builds a *bedrockruntime.Client for cfg. Region,
// profile, and explicit static credentials feed aws-sdk-go-v2's own
// config.LoadDefaultConfig resolution (so profiles/env/IMDS/SSO fall back
// exactly as the SDK's default chain already does when unset); endpoint,
// bearer-token auth, and custom headers are bedrockruntime-specific service
// options applied afterward.
func newBedrockRuntimeClient(ctx context.Context, cfg clientConfig) (*bedrockruntime.Client, error) {
	var loadOpts []func(*awsconfig.LoadOptions) error
	if cfg.Region != "" {
		loadOpts = append(loadOpts, awsconfig.WithRegion(cfg.Region))
	}
	if cfg.Profile != "" {
		loadOpts = append(loadOpts, awsconfig.WithSharedConfigProfile(cfg.Profile))
	}
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, err
	}

	return bedrockruntime.NewFromConfig(awsCfg, func(o *bedrockruntime.Options) {
		if cfg.Endpoint != "" {
			endpoint := cfg.Endpoint
			o.BaseEndpoint = &endpoint
		}
		if cfg.BearerToken != "" {
			o.BearerAuthTokenProvider = bearer.StaticTokenProvider{Token: bearer.Token{Value: cfg.BearerToken}}
			o.AuthSchemePreference = []string{"httpBearerAuth"}
		}
		if len(cfg.Headers) > 0 {
			o.APIOptions = append(o.APIOptions, addCustomHeadersMiddleware(cfg.Headers))
		}
	}), nil
}
