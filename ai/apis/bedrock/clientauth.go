package bedrock

// Ports the config-building half of bedrock-converse-stream.ts's stream
// function -- everything from `const config: BedrockRuntimeClientConfig = {`
// through `if (useBearerToken) { ... }`, i.e. the full AWS auth matrix this
// issue adds: explicit access key/secret/session credentials, profile,
// region resolution (ARN extraction, built-in endpoint derivation, ambient
// AWS_PROFILE precedence), and bearer-token auth.
//
// Risk #3 (does aws-sdk-go-v2 express bearer-token auth for bedrockruntime?)
// resolved SDK-native: bedrockruntime.Options has both a
// BearerAuthTokenProvider field and an AuthSchemePreference field, and the
// generated auth.go already advertises smithyauth.SchemeIDHTTPBearer
// alongside SigV4 for every operation -- see client.go's
// newBedrockRuntimeClient, which sets both. No hand-rolled HTTP signing was
// needed.
//
// Deviation: the Node-only proxy-agent (HttpProxyAgent/HttpsProxyAgent) and
// AWS_BEDROCK_FORCE_HTTP1 branch is not ported. Both work around a
// Node-specific quirk (NodeHttp2Handler having no HTTP-proxy-agent support),
// which has no Go equivalent -- net/http's Transport already supports
// HTTP(S)_PROXY env vars and negotiates HTTP/1.1 or HTTP/2 per connection
// without a handler swap. No upstream or ported test exercises this branch.
//
// No upstream test exists for the explicit-credentials/bearer-token/
// skip-auth/custom-headers cells (bedrock-endpoint-resolution.test.ts only
// covers profile/region/endpoint); those cells are covered by this port's
// own clientauth_test.go/headers_middleware_test.go instead.
//
// Ports: packages/ai/src/api/bedrock-converse-stream.ts

import (
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/julienlegoux/kern-link/ai"
)

// clientConfig is the resolved Bedrock client configuration: everything the
// SDK's default credential chain, explicit credentials, profile, region,
// endpoint pinning, and bearer-token auth need to build the real
// bedrockruntime.Client (see client.go). Computed independently of any AWS
// SDK type so the auth-matrix resolution logic is unit-testable without
// live credentials or network access.
type clientConfig struct {
	// Region, non-empty, pins the client's signing/endpoint region. Empty
	// leaves region resolution to the SDK's own default chain.
	Region string
	// Profile, non-empty, selects a named profile from the shared AWS
	// config/credentials files.
	Profile string
	// Endpoint, non-empty, pins an explicit Bedrock runtime endpoint
	// (custom VPC/proxy endpoints, or a built-in regional endpoint derived
	// from the model's baseUrl).
	Endpoint string
	// AccessKeyID/SecretAccessKey/SessionToken pin explicit static
	// credentials (bypassing the SDK's default credential chain) when both
	// AccessKeyID and SecretAccessKey are non-empty.
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	// BearerToken, non-empty, switches auth to Bedrock's HTTP bearer scheme
	// instead of SigV4.
	BearerToken string
	// Headers carries caller-supplied custom headers to attach to every
	// request; nil/empty means none.
	Headers map[string]string
	// MaxRetries is the resolved retry cap (StreamOptions.MaxRetries, or
	// defaultMaxRetries when unset). It counts retries, not attempts --
	// client.go converts.
	MaxRetries int
	// MaxRetryDelay caps the SDK's exponential backoff between attempts.
	// Zero leaves the SDK's own maximum in place.
	MaxRetryDelay time.Duration
}

// arnRegionPattern extracts the region from a Bedrock inference-profile ARN,
// including the GovCloud "arn:aws-us-gov:..." partition. Ports the regex
// literal inline in bedrock-converse-stream.ts's stream function.
var arnRegionPattern = regexp.MustCompile(`^arn:aws(?:-[a-z0-9-]+)?:bedrock:([a-z0-9-]+):`)

// standardBedrockEndpointPattern matches a built-in AWS Bedrock runtime
// endpoint hostname (with optional "-fips" and ".cn" partition suffixes),
// capturing its region. Ports getStandardBedrockEndpointRegion's regex.
var standardBedrockEndpointPattern = regexp.MustCompile(`^bedrock-runtime(?:-fips)?\.([a-z0-9-]+)\.amazonaws\.com(?:\.cn)?$`)

// resolveClientConfig computes the Bedrock client configuration for one
// Stream call. Ports the config-building statements from
// bedrock-converse-stream.ts's stream function (region/profile/endpoint/
// credentials/bearer-token), reusing getConfiguredBedrockRegion from
// thinking.go (now also driving real client region resolution, not just the
// GovCloud thinking-display check).
func resolveClientConfig(model *ai.Model, opts *ai.StreamOptions) clientConfig {
	var (
		optRegion, optProfile, optBearerToken string
		env                                   ai.ProviderEnv
	)
	cfg := clientConfig{}
	if opts != nil {
		optRegion = opts.BedrockRegion
		optProfile = opts.BedrockProfile
		optBearerToken = opts.BedrockBearerToken
		env = opts.Env
		cfg.Headers = ai.ProviderHeadersToRecord(opts.Headers)
	}

	cfg.MaxRetries = defaultMaxRetries
	if opts != nil && opts.MaxRetries != nil {
		cfg.MaxRetries = *opts.MaxRetries
	}
	if opts != nil {
		cfg.MaxRetryDelay = opts.EffectiveMaxRetryDelay()
	}

	cfg.Profile = firstNonEmptyString(optProfile, providerEnvValue("AWS_PROFILE", env))

	configuredRegion := getConfiguredBedrockRegion(optRegion, env)
	// hasAmbientConfiguredProfile intentionally checks only the real process
	// environment (env=nil below), never the per-request env override:
	// matches upstream's own `Boolean(getProviderEnvValue("AWS_PROFILE"))`
	// call, which passes no `env` argument.
	hasAmbientConfiguredProfile := providerEnvValue("AWS_PROFILE", nil) != ""
	endpointRegion := getStandardBedrockEndpointRegion(model.BaseURL)
	useExplicitEndpoint := shouldUseExplicitBedrockEndpoint(model.BaseURL, configuredRegion, hasAmbientConfiguredProfile)

	if useExplicitEndpoint {
		cfg.Endpoint = model.BaseURL
	}

	skipAuth := providerEnvValue("AWS_BEDROCK_SKIP_AUTH", env) == "1"
	bearerToken := firstNonEmptyString(optBearerToken, providerEnvValue("AWS_BEARER_TOKEN_BEDROCK", env))

	if arnMatch := arnRegionPattern.FindStringSubmatch(model.ID); arnMatch != nil {
		cfg.Region = arnMatch[1]
	} else if configuredRegion != "" {
		cfg.Region = configuredRegion
	} else if endpointRegion != "" && useExplicitEndpoint {
		cfg.Region = endpointRegion
	} else if !hasAmbientConfiguredProfile {
		cfg.Region = "us-east-1"
	}

	if skipAuth {
		cfg.AccessKeyID = "dummy-access-key"
		cfg.SecretAccessKey = "dummy-secret-key"
	} else if id, secret, session, ok := getConfiguredBedrockCredentials(env); ok {
		cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken = id, secret, session
	}

	if bearerToken != "" && !skipAuth {
		cfg.BearerToken = bearerToken
	}

	return cfg
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// getConfiguredBedrockCredentials reads explicit static AWS credentials from
// the ambient env-key map (AWS_ACCESS_KEY_ID + AWS_SECRET_ACCESS_KEY,
// optionally AWS_SESSION_TOKEN); ok is false unless both the access key id
// and secret are present. Ports getConfiguredBedrockCredentials.
func getConfiguredBedrockCredentials(env ai.ProviderEnv) (accessKeyID, secretAccessKey, sessionToken string, ok bool) {
	accessKeyID = providerEnvValue("AWS_ACCESS_KEY_ID", env)
	secretAccessKey = providerEnvValue("AWS_SECRET_ACCESS_KEY", env)
	if accessKeyID == "" || secretAccessKey == "" {
		return "", "", "", false
	}
	sessionToken = providerEnvValue("AWS_SESSION_TOKEN", env)
	return accessKeyID, secretAccessKey, sessionToken, true
}

// getStandardBedrockEndpointRegion extracts the region from a built-in AWS
// Bedrock runtime endpoint hostname, or "" if baseURL isn't one (custom/VPC
// endpoints, or no baseURL at all). Ports getStandardBedrockEndpointRegion.
func getStandardBedrockEndpointRegion(baseURL string) string {
	if baseURL == "" {
		return ""
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	match := standardBedrockEndpointPattern.FindStringSubmatch(strings.ToLower(u.Hostname()))
	if match == nil {
		return ""
	}
	return match[1]
}

// shouldUseExplicitBedrockEndpoint reports whether baseURL should be pinned
// as the client's explicit endpoint: always for non-standard (custom/VPC)
// endpoints, and for standard built-in endpoints only when neither an
// explicit region nor an ambient AWS_PROFILE is configured (so
// AWS_REGION/AWS_PROFILE-driven region selection isn't silently overridden
// by a catalog default). Ports shouldUseExplicitBedrockEndpoint.
func shouldUseExplicitBedrockEndpoint(baseURL, configuredRegion string, hasAmbientConfiguredProfile bool) bool {
	if getStandardBedrockEndpointRegion(baseURL) == "" {
		return true
	}
	return configuredRegion == "" && !hasAmbientConfiguredProfile
}
