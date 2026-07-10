// Package jsonschema validates tool-call arguments against a tool's JSON
// Schema parameters, applying upstream's coercion pass before validation so
// stringly-typed LLM output ("42", "true") is accepted exactly where the TS
// library accepts it.
package jsonschema

// Ports: packages/ai/src/utils/validation.ts
//
// Deviations from upstream:
//   - TypeBox schema values do not exist in Go: Tool.Parameters is always a
//     plain JSON Schema document, so the TYPEBOX_KIND branch (skip coercion)
//     and the TypeBox Value.Convert pre-pass (a no-op for plain schemas) are
//     not ported; the coercion pass always runs.
//   - packages/ai/src/utils/typebox-helpers.ts (StringEnum) is a TypeBox
//     schema-authoring convenience with no Go equivalent.
//   - Validation is santhosh-tekuri/jsonschema/v6 instead of TypeBox Compile;
//     error detail strings differ, but the "Validation failed for tool ..."
//     shape and path formatting are preserved.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	ai "github.com/julienlegoux/kern-link/ai"
)

var errPrinter = message.NewPrinter(language.English)

// validatorCache mirrors upstream's WeakMap keyed by schema identity; raw
// schema bytes are the closest stable identity for a json.RawMessage.
var (
	validatorMu    sync.Mutex
	validatorCache = map[string]*jsonschema.Schema{}
)

func getValidator(raw []byte) (*jsonschema.Schema, error) {
	key := string(raw)
	validatorMu.Lock()
	defer validatorMu.Unlock()
	if sch, ok := validatorCache[key]; ok {
		return sch, nil
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("tool.json", doc); err != nil {
		return nil, err
	}
	sch, err := c.Compile("tool.json")
	if err != nil {
		return nil, err
	}
	validatorCache[key] = sch
	return sch, nil
}

// getSubSchemaValidator compiles a schema fragment encountered during the
// coercion walk, returning nil when it cannot be compiled standalone (same as
// upstream's try/catch around getValidator).
func getSubSchemaValidator(schema any) *jsonschema.Schema {
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil
	}
	sch, err := getValidator(raw)
	if err != nil {
		return nil
	}
	return sch
}

func getSchemaTypes(schema map[string]any) []string {
	switch t := schema["type"].(type) {
	case string:
		return []string{t}
	case []any:
		var types []string
		for _, v := range t {
			if s, ok := v.(string); ok {
				types = append(types, s)
			}
		}
		return types
	}
	return nil
}

func matchesJSONType(value any, typ string) bool {
	switch typ {
	case "number":
		_, ok := value.(float64)
		return ok
	case "integer":
		f, ok := value.(float64)
		return ok && f == math.Trunc(f)
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "null":
		return value == nil
	case "array":
		_, ok := value.([]any)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	}
	return false
}

// jsNumber is JS Number(string) for a non-empty trimmed string: the parsed
// value and whether it is finite.
func jsNumber(s string) (float64, bool) {
	t := strings.TrimSpace(s)
	if t == "" {
		return 0, true
	}
	body, neg := t, false
	if len(body) > 0 && (body[0] == '+' || body[0] == '-') {
		neg = body[0] == '-'
		body = body[1:]
	}
	if len(body) > 2 && body[0] == '0' {
		var base int
		switch body[1] {
		case 'x', 'X':
			base = 16
		case 'o', 'O':
			base = 8
		case 'b', 'B':
			base = 2
		}
		if base != 0 {
			if neg || t[0] == '+' {
				return 0, false // JS: signed radix literals are NaN
			}
			u, err := strconv.ParseUint(body[2:], base, 64)
			if err != nil {
				return 0, false
			}
			return float64(u), true
		}
	}
	// Reject spellings Go accepts but JS does not ("inf", "nan"); JS's own
	// "Infinity" is non-finite either way.
	if strings.ContainsAny(t, "iInN") {
		return 0, false
	}
	f, err := strconv.ParseFloat(t, 64)
	if err != nil || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// jsNumberString is JS String(number) for finite values.
func jsNumberString(f float64) string {
	if f == 0 {
		return "0" // covers -0
	}
	abs := math.Abs(f)
	if abs >= 1e21 || abs < 1e-6 {
		s := strconv.FormatFloat(f, 'e', -1, 64)
		// JS exponents carry no zero padding: 1e+21, 1e-7.
		if i := strings.IndexByte(s, 'e'); i >= 0 && len(s) > i+2 {
			mantissa, exp := s[:i+2], strings.TrimLeft(s[i+2:], "0")
			if exp == "" {
				exp = "0"
			}
			s = mantissa + exp
		}
		return s
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// coercePrimitiveByType returns the coerced value and whether a conversion
// happened (upstream compares candidate !== value; every conversion branch
// changes the value's type, so a taken branch means changed).
func coercePrimitiveByType(value any, typ string) (any, bool) {
	switch typ {
	case "number":
		if value == nil {
			return float64(0), true
		}
		if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
			if parsed, finite := jsNumber(s); finite {
				return parsed, true
			}
		}
		if b, ok := value.(bool); ok {
			if b {
				return float64(1), true
			}
			return float64(0), true
		}
		return value, false
	case "integer":
		if value == nil {
			return float64(0), true
		}
		if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
			if parsed, finite := jsNumber(s); finite && parsed == math.Trunc(parsed) {
				return parsed, true
			}
		}
		if b, ok := value.(bool); ok {
			if b {
				return float64(1), true
			}
			return float64(0), true
		}
		return value, false
	case "boolean":
		if value == nil {
			return false, true
		}
		if s, ok := value.(string); ok {
			if s == "true" {
				return true, true
			}
			if s == "false" {
				return false, true
			}
		}
		if f, ok := value.(float64); ok {
			if f == 1 {
				return true, true
			}
			if f == 0 {
				return false, true
			}
		}
		return value, false
	case "string":
		if value == nil {
			return "", true
		}
		if f, ok := value.(float64); ok {
			return jsNumberString(f), true
		}
		if b, ok := value.(bool); ok {
			if b {
				return "true", true
			}
			return "false", true
		}
		return value, false
	case "null":
		if value == "" || value == float64(0) || value == false {
			return nil, true
		}
		return value, false
	}
	return value, false
}

func applySchemaObjectCoercion(value map[string]any, schema map[string]any) {
	properties, _ := schema["properties"].(map[string]any)
	definedKeys := make(map[string]bool, len(properties))
	for key := range properties {
		definedKeys[key] = true
	}

	for key, propertySchema := range properties {
		if v, ok := value[key]; ok {
			value[key] = coerceWithJSONSchema(v, propertySchema)
		}
	}

	if additional, ok := schema["additionalProperties"].(map[string]any); ok {
		for key, propertyValue := range value {
			if definedKeys[key] {
				continue
			}
			value[key] = coerceWithJSONSchema(propertyValue, additional)
		}
	}
}

func applySchemaArrayCoercion(value []any, schema map[string]any) {
	if items, ok := schema["items"].([]any); ok {
		for index := range value {
			if index >= len(items) {
				continue
			}
			value[index] = coerceWithJSONSchema(value[index], items[index])
		}
		return
	}

	if items, ok := schema["items"].(map[string]any); ok {
		for index := range value {
			value[index] = coerceWithJSONSchema(value[index], items)
		}
	}
}

func coerceWithUnionSchema(value any, schemas []any) any {
	for _, schema := range schemas {
		candidate := deepCloneValue(value)
		coerced := coerceWithJSONSchema(candidate, schema)
		validator := getSubSchemaValidator(schema)
		if validator != nil && validator.Validate(coerced) == nil {
			return coerced
		}
	}
	return value
}

func coerceWithJSONSchema(value any, schema any) any {
	schemaObj, ok := schema.(map[string]any)
	if !ok {
		return value
	}
	nextValue := value

	if allOf, ok := schemaObj["allOf"].([]any); ok {
		for _, nested := range allOf {
			nextValue = coerceWithJSONSchema(nextValue, nested)
		}
	}

	if anyOf, ok := schemaObj["anyOf"].([]any); ok {
		nextValue = coerceWithUnionSchema(nextValue, anyOf)
	}

	if oneOf, ok := schemaObj["oneOf"].([]any); ok {
		nextValue = coerceWithUnionSchema(nextValue, oneOf)
	}

	schemaTypes := getSchemaTypes(schemaObj)
	matchesUnionMember := len(schemaTypes) > 1 && slices.ContainsFunc(schemaTypes, func(t string) bool {
		return matchesJSONType(nextValue, t)
	})
	if len(schemaTypes) > 0 && !matchesUnionMember {
		for _, schemaType := range schemaTypes {
			if candidate, changed := coercePrimitiveByType(nextValue, schemaType); changed {
				nextValue = candidate
				break
			}
		}
	}

	if slices.Contains(schemaTypes, "object") {
		if obj, ok := nextValue.(map[string]any); ok {
			applySchemaObjectCoercion(obj, schemaObj)
		}
	}

	if slices.Contains(schemaTypes, "array") {
		if arr, ok := nextValue.([]any); ok {
			applySchemaArrayCoercion(arr, schemaObj)
		}
	}

	return nextValue
}

// deepCloneValue copies a JSON-shaped value (upstream structuredClone).
func deepCloneValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, e := range v {
			out[k] = deepCloneValue(e)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, e := range v {
			out[i] = deepCloneValue(e)
		}
		return out
	default:
		return v
	}
}

func formatValidationPath(verr *jsonschema.ValidationError) string {
	path := strings.Join(verr.InstanceLocation, ".")
	if required, ok := verr.ErrorKind.(*kind.Required); ok && len(required.Missing) > 0 {
		if path == "" {
			return required.Missing[0]
		}
		return path + "." + required.Missing[0]
	}
	if path == "" {
		return "root"
	}
	return path
}

func leafErrors(verr *jsonschema.ValidationError) []*jsonschema.ValidationError {
	if len(verr.Causes) == 0 {
		return []*jsonschema.ValidationError{verr}
	}
	var leaves []*jsonschema.ValidationError
	for _, cause := range verr.Causes {
		leaves = append(leaves, leafErrors(cause)...)
	}
	return leaves
}

// ValidateToolCall finds a tool by name and validates the tool call arguments
// against its JSON Schema.
func ValidateToolCall(tools []ai.Tool, toolCall ai.ToolCall) (map[string]any, error) {
	for _, tool := range tools {
		if tool.Name == toolCall.Name {
			return ValidateToolArguments(tool, toolCall)
		}
	}
	return nil, fmt.Errorf("Tool %q not found", toolCall.Name)
}

// ValidateToolArguments validates the tool call arguments against the tool's
// JSON Schema parameters, returning the (potentially coerced) arguments. The
// input arguments are never mutated.
func ValidateToolArguments(tool ai.Tool, toolCall ai.ToolCall) (map[string]any, error) {
	// structuredClone via a JSON round-trip: also normalizes any hand-built
	// Go values (int, nil map) to their JSON-decoded shapes.
	rawArgs, err := json.Marshal(toolCall.Arguments)
	if err != nil {
		return nil, fmt.Errorf("tool %q arguments are not JSON-encodable: %w", toolCall.Name, err)
	}
	var args any
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, err
	}

	validator, err := getValidator(tool.Parameters)
	if err != nil {
		return nil, fmt.Errorf("compiling parameters schema for tool %q: %w", toolCall.Name, err)
	}

	var schemaDoc any
	if err := json.Unmarshal(tool.Parameters, &schemaDoc); err != nil {
		return nil, fmt.Errorf("compiling parameters schema for tool %q: %w", toolCall.Name, err)
	}

	// Arguments are always a JSON object, and object values survive the
	// coercion pass as objects, so upstream's top-level non-object fallback
	// (validate-coerced-or-return-original) is unreachable here.
	coerced := coerceWithJSONSchema(args, schemaDoc)

	verr := validator.Validate(coerced)
	if verr == nil {
		result, _ := coerced.(map[string]any)
		return result, nil
	}

	errorLines := "Unknown validation error"
	if validationErr, ok := verr.(*jsonschema.ValidationError); ok {
		var lines []string
		for _, leaf := range leafErrors(validationErr) {
			lines = append(lines, fmt.Sprintf("  - %s: %s", formatValidationPath(leaf), leaf.ErrorKind.LocalizedString(errPrinter)))
		}
		if len(lines) > 0 {
			errorLines = strings.Join(lines, "\n")
		}
	}

	receivedArgs, err := json.MarshalIndent(toolCall.Arguments, "", "  ")
	if err != nil {
		receivedArgs = rawArgs
	}

	return nil, fmt.Errorf("Validation failed for tool %q:\n%s\n\nReceived arguments:\n%s", toolCall.Name, errorLines, receivedArgs)
}
