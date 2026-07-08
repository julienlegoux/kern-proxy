package bedrock

// Tests that a resolved clientConfig is correctly plumbed into the real
// aws-sdk-go-v2 bedrockruntime.Client: Region/BaseEndpoint/
// BearerAuthTokenProvider/AuthSchemePreference/APIOptions are all
// inspectable via Client.Options() without any network access or live AWS
// credentials -- config.LoadDefaultConfig only resolves lazily on retrieval,
// which none of these assertions trigger. Profile resolution itself is
// exercised at the pure resolveClientConfig level (clientauth_test.go); the
// SDK's own handling of a shared-config profile is that SDK's tested
// responsibility, not this port's.

import (
	"context"
	"testing"
)

func TestNewBedrockRuntimeClient_PlumbsRegionAndEndpoint(t *testing.T) {
	client, err := newBedrockRuntimeClient(context.Background(), clientConfig{
		Region:   "eu-central-1",
		Endpoint: "https://bedrock-runtime.eu-central-1.amazonaws.com",
	})
	if err != nil {
		t.Fatalf("newBedrockRuntimeClient: %v", err)
	}
	opts := client.Options()
	if opts.Region != "eu-central-1" {
		t.Errorf("Region = %q, want eu-central-1", opts.Region)
	}
	if opts.BaseEndpoint == nil || *opts.BaseEndpoint != "https://bedrock-runtime.eu-central-1.amazonaws.com" {
		t.Errorf("BaseEndpoint = %v", opts.BaseEndpoint)
	}
}

func TestNewBedrockRuntimeClient_NoEndpointLeavesBaseEndpointNil(t *testing.T) {
	client, err := newBedrockRuntimeClient(context.Background(), clientConfig{Region: "us-east-1"})
	if err != nil {
		t.Fatalf("newBedrockRuntimeClient: %v", err)
	}
	if client.Options().BaseEndpoint != nil {
		t.Errorf("BaseEndpoint = %v, want nil", *client.Options().BaseEndpoint)
	}
}

func TestNewBedrockRuntimeClient_BearerTokenSetsProviderAndSchemePreference(t *testing.T) {
	client, err := newBedrockRuntimeClient(context.Background(), clientConfig{
		Region:      "us-east-1",
		BearerToken: "tok-explicit",
	})
	if err != nil {
		t.Fatalf("newBedrockRuntimeClient: %v", err)
	}
	opts := client.Options()
	if opts.BearerAuthTokenProvider == nil {
		t.Fatal("BearerAuthTokenProvider = nil, want set")
	}
	tok, err := opts.BearerAuthTokenProvider.RetrieveBearerToken(context.Background())
	if err != nil {
		t.Fatalf("RetrieveBearerToken: %v", err)
	}
	if tok.Value != "tok-explicit" {
		t.Errorf("token value = %q, want tok-explicit", tok.Value)
	}
	if len(opts.AuthSchemePreference) != 1 || opts.AuthSchemePreference[0] != "httpBearerAuth" {
		t.Errorf("AuthSchemePreference = %v, want [httpBearerAuth]", opts.AuthSchemePreference)
	}
}

func TestNewBedrockRuntimeClient_NoBearerTokenLeavesProviderNil(t *testing.T) {
	client, err := newBedrockRuntimeClient(context.Background(), clientConfig{Region: "us-east-1"})
	if err != nil {
		t.Fatalf("newBedrockRuntimeClient: %v", err)
	}
	if client.Options().BearerAuthTokenProvider != nil {
		t.Error("BearerAuthTokenProvider set, want nil")
	}
	if len(client.Options().AuthSchemePreference) != 0 {
		t.Errorf("AuthSchemePreference = %v, want empty", client.Options().AuthSchemePreference)
	}
}

func TestNewBedrockRuntimeClient_ExplicitCredentialsAreRetrievable(t *testing.T) {
	client, err := newBedrockRuntimeClient(context.Background(), clientConfig{
		Region:          "us-east-1",
		AccessKeyID:     "AKIAEXAMPLE",
		SecretAccessKey: "secretexample",
		SessionToken:    "sessiontoken",
	})
	if err != nil {
		t.Fatalf("newBedrockRuntimeClient: %v", err)
	}
	creds, err := client.Options().Credentials.Retrieve(context.Background())
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if creds.AccessKeyID != "AKIAEXAMPLE" || creds.SecretAccessKey != "secretexample" || creds.SessionToken != "sessiontoken" {
		t.Errorf("creds = %+v", creds)
	}
}

func TestNewBedrockRuntimeClient_HeadersRegisterAnAPIOption(t *testing.T) {
	withHeaders, err := newBedrockRuntimeClient(context.Background(), clientConfig{
		Region:  "us-east-1",
		Headers: map[string]string{"x-custom": "v"},
	})
	if err != nil {
		t.Fatalf("newBedrockRuntimeClient: %v", err)
	}
	withoutHeaders, err := newBedrockRuntimeClient(context.Background(), clientConfig{Region: "us-east-1"})
	if err != nil {
		t.Fatalf("newBedrockRuntimeClient: %v", err)
	}
	got := len(withHeaders.Options().APIOptions)
	want := len(withoutHeaders.Options().APIOptions) + 1
	if got != want {
		t.Errorf("len(APIOptions) = %d, want %d (one more than the no-headers baseline)", got, want)
	}
}
