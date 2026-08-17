package jsonschema

// Ports: packages/ai/test/validation.test.ts

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	ai "github.com/kern-ia/kern-link/ai"
)

// echoToolWithPlainSchema mirrors upstream's createToolCallWithPlainSchema:
// the value schema is wrapped in {type:object, properties:{value}, required}.
func echoToolWithPlainSchema(valueSchema string, value any) (ai.Tool, ai.ToolCall) {
	tool := ai.Tool{
		Name:        "echo",
		Description: "Echo tool",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {"value": ` + valueSchema + `},
			"required": ["value"]
		}`),
	}
	toolCall := ai.ToolCall{
		ID:        "tool-1",
		Name:      "echo",
		Arguments: map[string]any{"value": value},
	}
	return tool, toolCall
}

// Upstream's "still validates when Function constructor is unavailable" test
// exercises TypeBox Value.Convert under a JS eval restriction; the JS-specific
// premise does not port, but the observable coercion ("42" -> 42 for a number
// property) must hold through the plain-schema path.
func TestValidateToolArgumentsCoercesStringifiedNumber(t *testing.T) {
	tool := ai.Tool{
		Name:        "echo",
		Description: "Echo tool",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"count":{"type":"number"}},"required":["count"]}`),
	}
	toolCall := ai.ToolCall{ID: "tool-1", Name: "echo", Arguments: map[string]any{"count": "42"}}

	got, err := ValidateToolArguments(tool, toolCall)
	if err != nil {
		t.Fatalf("ValidateToolArguments: %v", err)
	}
	want := map[string]any{"count": float64(42)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestValidateToolArgumentsCoercesPlainJSONSchemas(t *testing.T) {
	passingCases := []struct {
		schema   string
		input    any
		expected any
	}{
		{`{"type":"number"}`, "42", float64(42)},
		{`{"type":"number"}`, true, float64(1)},
		{`{"type":"number"}`, nil, float64(0)},
		{`{"type":"integer"}`, "42", float64(42)},
		{`{"type":"boolean"}`, "true", true},
		{`{"type":"boolean"}`, "false", false},
		{`{"type":"boolean"}`, float64(1), true},
		{`{"type":"boolean"}`, float64(0), false},
		{`{"type":"string"}`, nil, ""},
		{`{"type":"string"}`, true, "true"},
		{`{"type":"null"}`, "", nil},
		{`{"type":"null"}`, float64(0), nil},
		{`{"type":"null"}`, false, nil},
		{`{"type":["number","string"]}`, "1", "1"},
		{`{"type":["boolean","number"]}`, "1", float64(1)},
	}

	for _, tc := range passingCases {
		tool, toolCall := echoToolWithPlainSchema(tc.schema, tc.input)
		got, err := ValidateToolArguments(tool, toolCall)
		if err != nil {
			t.Errorf("schema %s input %#v: unexpected error: %v", tc.schema, tc.input, err)
			continue
		}
		want := map[string]any{"value": tc.expected}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("schema %s input %#v: got %#v, want %#v", tc.schema, tc.input, got, want)
		}
	}
}

func TestValidateToolArgumentsRejectsInvalidCoercions(t *testing.T) {
	failingCases := []struct {
		schema string
		input  any
	}{
		{`{"type":"boolean"}`, "1"},
		{`{"type":"boolean"}`, "0"},
		{`{"type":"null"}`, "null"},
		{`{"type":"integer"}`, "42.1"},
	}

	for _, tc := range failingCases {
		tool, toolCall := echoToolWithPlainSchema(tc.schema, tc.input)
		_, err := ValidateToolArguments(tool, toolCall)
		if err == nil {
			t.Errorf("schema %s input %#v: expected error, got none", tc.schema, tc.input)
			continue
		}
		if !strings.Contains(err.Error(), "Validation failed") {
			t.Errorf("schema %s input %#v: error %q does not contain %q", tc.schema, tc.input, err, "Validation failed")
		}
	}
}

// --- supplementary coverage for the ported coercion pass ---------------------
// The upstream suite only exercises primitive coercion through the wrapper
// object; these lock the recursive object/array/union branches of
// coerceWithJsonSchema and the ported error formatting.

func TestValidateToolArgumentsCoercesNestedStructures(t *testing.T) {
	cases := []struct {
		name     string
		schema   string
		input    any
		expected any
	}{
		{
			"nested object property",
			`{"type":"object","properties":{"n":{"type":"number"}}}`,
			map[string]any{"n": "7"},
			map[string]any{"n": float64(7)},
		},
		{
			"additionalProperties schema",
			`{"type":"object","additionalProperties":{"type":"boolean"}}`,
			map[string]any{"on": "true"},
			map[string]any{"on": true},
		},
		{
			"array items schema",
			`{"type":"array","items":{"type":"integer"}}`,
			[]any{"1", "2"},
			[]any{float64(1), float64(2)},
		},
		{
			"allOf applies sequentially",
			`{"allOf":[{"type":"number"}]}`,
			"5",
			float64(5),
		},
		{
			"anyOf picks first validating branch",
			`{"anyOf":[{"type":"boolean"},{"type":"number"}]}`,
			"1",
			float64(1),
		},
		{
			"oneOf picks validating branch",
			`{"oneOf":[{"type":"boolean"}]}`,
			"true",
			true,
		},
	}

	for _, tc := range cases {
		tool, toolCall := echoToolWithPlainSchema(tc.schema, tc.input)
		got, err := ValidateToolArguments(tool, toolCall)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			continue
		}
		want := map[string]any{"value": tc.expected}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %#v, want %#v", tc.name, got, want)
		}
	}
}

// Tuple-form items ([]schemas) is draft-07 syntax; the coercion pass reads it
// regardless of dialect, and the validator honors it when the document
// declares draft-07.
func TestValidateToolArgumentsCoercesTupleItems(t *testing.T) {
	tool := ai.Tool{
		Name:        "echo",
		Description: "Echo tool",
		Parameters: json.RawMessage(`{
			"$schema": "http://json-schema.org/draft-07/schema#",
			"type": "object",
			"properties": {"value": {"type":"array","items":[{"type":"number"},{"type":"boolean"}]}},
			"required": ["value"]
		}`),
	}
	toolCall := ai.ToolCall{ID: "tool-1", Name: "echo", Arguments: map[string]any{"value": []any{"3", float64(1)}}}

	got, err := ValidateToolArguments(tool, toolCall)
	if err != nil {
		t.Fatalf("ValidateToolArguments: %v", err)
	}
	want := map[string]any{"value": []any{float64(3), true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestValidateToolArgumentsErrorFormat(t *testing.T) {
	tool := ai.Tool{
		Name:        "echo",
		Description: "Echo tool",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"value":{"type":"number"}},"required":["value"]}`),
	}

	// Missing required property: the ported formatValidationPath surfaces the
	// property name itself.
	_, err := ValidateToolArguments(tool, ai.ToolCall{ID: "tool-1", Name: "echo", Arguments: map[string]any{}})
	if err == nil {
		t.Fatal("expected error for missing required property")
	}
	msg := err.Error()
	if !strings.Contains(msg, `Validation failed for tool "echo":`) {
		t.Errorf("error %q missing header", msg)
	}
	if !strings.Contains(msg, "  - value: ") {
		t.Errorf("error %q missing required-property path %q", msg, "  - value: ")
	}
	if !strings.Contains(msg, "Received arguments:") {
		t.Errorf("error %q missing received-arguments section", msg)
	}

	// Type mismatch: instance path /value formats as "value".
	_, err = ValidateToolArguments(tool, ai.ToolCall{ID: "tool-1", Name: "echo", Arguments: map[string]any{"value": []any{}}})
	if err == nil {
		t.Fatal("expected error for type mismatch")
	}
	if !strings.Contains(err.Error(), "  - value: ") {
		t.Errorf("error %q missing instance path %q", err, "  - value: ")
	}
}

func TestValidateToolCall(t *testing.T) {
	tool := ai.Tool{
		Name:        "echo",
		Description: "Echo tool",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"value":{"type":"number"}},"required":["value"]}`),
	}
	tools := []ai.Tool{tool}

	got, err := ValidateToolCall(tools, ai.ToolCall{ID: "tool-1", Name: "echo", Arguments: map[string]any{"value": "42"}})
	if err != nil {
		t.Fatalf("ValidateToolCall: %v", err)
	}
	if !reflect.DeepEqual(got, map[string]any{"value": float64(42)}) {
		t.Errorf("got %#v", got)
	}

	_, err = ValidateToolCall(tools, ai.ToolCall{ID: "tool-2", Name: "missing", Arguments: map[string]any{}})
	if err == nil || !strings.Contains(err.Error(), `Tool "missing" not found`) {
		t.Errorf("expected tool-not-found error, got %v", err)
	}
}

// The original toolCall arguments must not be mutated by coercion (upstream
// operates on a structuredClone).
func TestValidateToolArgumentsDoesNotMutateInput(t *testing.T) {
	tool, toolCall := echoToolWithPlainSchema(`{"type":"number"}`, "42")
	if _, err := ValidateToolArguments(tool, toolCall); err != nil {
		t.Fatalf("ValidateToolArguments: %v", err)
	}
	if toolCall.Arguments["value"] != "42" {
		t.Errorf("input arguments mutated: %#v", toolCall.Arguments)
	}
}
