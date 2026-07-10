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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/smithy-go/auth/bearer"

	"github.com/julienlegoux/kern-link/ai/apis/internal/httpretry"
)

// defaultMaxRetries is the retry cap used when StreamOptions.MaxRetries is
// nil. It tracks the shared default the raw-HTTP adapters get from
// httpretry.DefaultMaxRetries, so "2 retries" means the same thing whichever
// adapter a caller reaches for -- even though the retrying here is done by the
// AWS SDK rather than by that package.
const defaultMaxRetries = httpretry.DefaultMaxRetries

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
		o.Retryer = newRetryer(cfg)
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

// newRetryer maps StreamOptions onto the AWS SDK's standard retryer, which
// owns the transport here and therefore the retrying.
//
// Two deviations from the shared httpretry loop the raw-HTTP adapters use,
// both inherent to delegating retries to the SDK:
//
//   - MaxRetryDelay caps the SDK's *exponential backoff* (StandardOptions.
//     MaxBackoff). The SDK does not expose a separate clamp for a
//     server-requested Retry-After, so a Bedrock response asking for a longer
//     wait than the SDK's own backoff schedule is not clamped the way
//     httpretry clamps a 429's retry-after header.
//   - Which errors are retryable is decided by the SDK's own retryable-error
//     classes (throttling, transient 5xx, connection resets), not by
//     ai/retry.go's text patterns. In practice the SDK already declines to
//     retry Bedrock's quota and validation errors, so the two agree on the
//     cases that matter; where they disagree, the SDK wins.
//
// MaxAttempts counts the initial attempt, so it is MaxRetries+1: a
// MaxRetries of 0 yields exactly one attempt.
func newRetryer(cfg clientConfig) aws.Retryer {
	return retry.NewStandard(func(o *retry.StandardOptions) {
		o.MaxAttempts = cfg.MaxRetries + 1
		if cfg.MaxRetryDelay > 0 {
			o.MaxBackoff = cfg.MaxRetryDelay
			o.Backoff = retry.NewExponentialJitterBackoff(cfg.MaxRetryDelay)
		}
	})
}
