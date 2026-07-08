package catalog

// Ports: packages/ai/src/types.ts's Model<TApi>["compat"] discriminated
// union — see compat_test.go's doc comment for why this needs a runtime
// check in Go where upstream's TS compiler enforces it statically.

import (
	"fmt"
	"reflect"

	"github.com/julienlegoux/kern-proxy/ai"
)

// compatFieldsByApi lists, for each Api that upstream's Model<TApi> allows a
// compat block at all, the ai.Compat struct field names that Api's own
// compat interface owns (packages/ai/src/types.ts: OpenAICompletionsCompat,
// OpenAIResponsesCompat, AnthropicMessagesCompat). Any Api not present here
// maps to upstream's `compat?: never` — a catalog entry for that Api must
// leave Compat nil.
var compatFieldsByApi = map[ai.Api]map[string]bool{
	ai.ApiOpenAICompletions: fieldSet(
		"SupportsStore", "SupportsDeveloperRole", "SupportsReasoningEffort",
		"SupportsUsageInStreaming", "MaxTokensField", "RequiresToolResultName",
		"RequiresAssistantAfterToolResult", "RequiresThinkingAsText",
		"RequiresReasoningContentOnAssistantMessages", "ThinkingFormat",
		"ChatTemplateKwargs", "OpenRouterRouting", "VercelGatewayRouting",
		"ZaiToolStream", "SupportsStrictMode", "CacheControlFormat",
		"SendSessionAffinityHeaders", "SupportsLongCacheRetention",
	),
	ai.ApiOpenAIResponses: fieldSet(
		"SupportsDeveloperRole", "SendSessionIDHeader", "SupportsLongCacheRetention",
	),
	ai.ApiAnthropicMessages: fieldSet(
		"SupportsEagerToolInputStreaming", "SupportsLongCacheRetention",
		"SendSessionAffinityHeaders", "SupportsCacheControlOnTools",
		"SupportsTemperature", "ForceAdaptiveThinking", "AllowEmptySignature",
	),
}

func fieldSet(names ...string) map[string]bool {
	out := make(map[string]bool, len(names))
	for _, n := range names {
		out[n] = true
	}
	return out
}

// validateCompatForApi reports an error if compat sets any ai.Compat field
// that api's upstream compat interface doesn't own, or if compat is
// non-nil for an api that accepts no compat block at all. A nil compat is
// always valid.
func validateCompatForApi(api ai.Api, compat *ai.Compat) error {
	if compat == nil {
		return nil
	}
	allowed, ok := compatFieldsByApi[api]
	if !ok {
		return fmt.Errorf("api %q does not accept a compat block, but one is set", api)
	}
	v := reflect.ValueOf(*compat)
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if allowed[field.Name] {
			continue
		}
		if !v.Field(i).IsZero() {
			return fmt.Errorf("field %s is set but not valid for api %q", field.Name, api)
		}
	}
	return nil
}
