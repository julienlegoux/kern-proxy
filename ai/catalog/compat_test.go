package catalog

// Ports: no direct upstream test — packages/ai/src/types.ts's Model<TApi>
// generic makes compat's shape a compile-time discriminated union (compat is
// `never` for any TApi other than "openai-completions"/"openai-responses"/
// "anthropic-messages", and each of those only accepts its own compat
// interface's fields). ai.Compat instead merges all three into one flat,
// untyped struct (see ai/model.go's doc comment), so nothing at compile time
// stops a catalog entry from setting a field the model's Api doesn't read.
// This test is the runtime equivalent of that erased compile-time check,
// run over every embedded catalog entry — the epic 11 issue 01 acceptance
// criterion "enforce that each model's Compat sub-struct matches its Api".

import (
	"testing"

	"github.com/julienlegoux/kern-link/ai"
)

func TestCatalogCompatMatchesApi(t *testing.T) {
	for _, provider := range Providers() {
		for _, model := range BuiltinModels(provider) {
			if err := validateCompatForApi(model.Api, model.Compat); err != nil {
				t.Errorf("%s/%s (api %q): %v", provider, model.ID, model.Api, err)
			}
		}
	}
}

func TestValidateCompatForApi_RejectsFieldsNotOwnedByApi(t *testing.T) {
	trueVal := true
	cases := []struct {
		name    string
		api     ai.Api
		compat  *ai.Compat
		wantErr bool
	}{
		{"nil compat always ok", ai.ApiBedrockConverseStream, nil, false},
		{"openai-completions-only field on openai-completions", ai.ApiOpenAICompletions, &ai.Compat{SupportsStore: &trueVal}, false},
		{"shared field on openai-responses", ai.ApiOpenAIResponses, &ai.Compat{SupportsLongCacheRetention: &trueVal}, false},
		{"anthropic-only field on anthropic-messages", ai.ApiAnthropicMessages, &ai.Compat{ForceAdaptiveThinking: &trueVal}, false},
		{"openai-completions field on openai-responses is rejected", ai.ApiOpenAIResponses, &ai.Compat{SupportsStore: &trueVal}, true},
		{"anthropic-only field on openai-completions is rejected", ai.ApiOpenAICompletions, &ai.Compat{ForceAdaptiveThinking: &trueVal}, true},
		{"any compat on an unrelated api is rejected", ai.ApiBedrockConverseStream, &ai.Compat{SupportsStore: &trueVal}, true},
		{"any compat on mistral is rejected", ai.ApiMistralConversations, &ai.Compat{SendSessionAffinityHeaders: &trueVal}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateCompatForApi(c.api, c.compat)
			if (err != nil) != c.wantErr {
				t.Errorf("validateCompatForApi(%q, %+v) error = %v, wantErr %v", c.api, c.compat, err, c.wantErr)
			}
		})
	}
}
