// Package partialjson ports pi-ai's streaming/partial JSON handling: repair of
// malformed string literals plus best-effort parsing of truncated JSON, used
// to keep tool-call arguments usable on every streaming delta.
//
// Ports: packages/ai/src/utils/json-parse.ts (+ the npm partial-json parser)
package partialjson

import (
	"encoding/json"
	"fmt"
	"strings"
)

var validJSONEscapes = map[byte]bool{
	'"': true, '\\': true, '/': true, 'b': true, 'f': true, 'n': true, 'r': true, 't': true, 'u': true,
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func escapeControlCharacter(c byte) string {
	switch c {
	case '\b':
		return `\b`
	case '\f':
		return `\f`
	case '\n':
		return `\n`
	case '\r':
		return `\r`
	case '\t':
		return `\t`
	default:
		return fmt.Sprintf(`\u%04x`, c)
	}
}

// Repair fixes malformed JSON string literals by escaping raw control
// characters inside strings and doubling backslashes before invalid escapes.
func Repair(jsonStr string) string {
	var repaired strings.Builder
	repaired.Grow(len(jsonStr))
	inString := false

	for index := 0; index < len(jsonStr); index++ {
		c := jsonStr[index]

		if !inString {
			repaired.WriteByte(c)
			if c == '"' {
				inString = true
			}
			continue
		}

		if c == '"' {
			repaired.WriteByte(c)
			inString = false
			continue
		}

		if c == '\\' {
			if index+1 >= len(jsonStr) {
				repaired.WriteString(`\\`)
				continue
			}
			next := jsonStr[index+1]

			if next == 'u' {
				digits := ""
				if index+6 <= len(jsonStr) {
					digits = jsonStr[index+2 : index+6]
				}
				if len(digits) == 4 && isHexDigit(digits[0]) && isHexDigit(digits[1]) && isHexDigit(digits[2]) && isHexDigit(digits[3]) {
					repaired.WriteString(`\u` + digits)
					index += 5
					continue
				}
			}

			if validJSONEscapes[next] {
				repaired.WriteByte('\\')
				repaired.WriteByte(next)
				index++
				continue
			}

			repaired.WriteString(`\\`)
			continue
		}

		if c <= 0x1f {
			repaired.WriteString(escapeControlCharacter(c))
		} else {
			repaired.WriteByte(c)
		}
	}

	return repaired.String()
}

// ParseWithRepair parses JSON strictly, retrying once with Repair applied.
func ParseWithRepair(jsonStr string) (any, error) {
	var out any
	if err := json.Unmarshal([]byte(jsonStr), &out); err != nil {
		repaired := Repair(jsonStr)
		if repaired != jsonStr {
			var repairedOut any
			if err2 := json.Unmarshal([]byte(repaired), &repairedOut); err2 == nil {
				return repairedOut, nil
			}
		}
		return nil, err
	}
	return out, nil
}

// ParseStreaming parses potentially incomplete JSON during streaming. It
// always returns a usable value: strict parse, then repaired strict parse,
// then partial parse, then repaired partial parse, then an empty object.
func ParseStreaming(partial string) any {
	if strings.TrimSpace(partial) == "" {
		return map[string]any{}
	}
	if v, err := ParseWithRepair(partial); err == nil {
		return normalizeStreamingValue(v)
	}
	if v, err := ParsePartial(partial); err == nil {
		return normalizeStreamingValue(v)
	}
	if v, err := ParsePartial(Repair(partial)); err == nil {
		return normalizeStreamingValue(v)
	}
	return map[string]any{}
}

// ParseStreamingObject is ParseStreaming narrowed to objects, matching how
// adapters use it for tool-call arguments.
func ParseStreamingObject(partial string) map[string]any {
	v := ParseStreaming(partial)
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

// normalizeStreamingValue mirrors TS `result ?? {}`: a JSON null becomes an
// empty object so callers always get a value.
func normalizeStreamingValue(v any) any {
	if v == nil {
		return map[string]any{}
	}
	return v
}
