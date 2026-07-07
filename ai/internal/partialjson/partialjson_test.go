package partialjson

// Ports the repair/parse behavior of packages/ai/src/utils/json-parse.ts and
// the npm partial-json semantics pi-ai relies on for streaming tool args.

import (
	"reflect"
	"testing"
)

func TestRepairEscapesRawControlCharacters(t *testing.T) {
	got := Repair("{\"a\": \"line1\nline2\tend\"}")
	want := `{"a": "line1\nline2\tend"}`
	if got != want {
		t.Errorf("Repair = %q, want %q", got, want)
	}
}

func TestRepairDoublesInvalidEscapes(t *testing.T) {
	got := Repair(`{"path": "C:\Users\x"}`)
	want := `{"path": "C:\\Users\\x"}`
	if got != want {
		t.Errorf("Repair = %q, want %q", got, want)
	}
}

func TestRepairPreservesValidEscapes(t *testing.T) {
	in := `{"a": "quote:\" slash:\\ unicode:é tab:\t"}`
	if got := Repair(in); got != in {
		t.Errorf("Repair changed valid input: %q", got)
	}
}

func TestRepairInvalidUnicodeEscape(t *testing.T) {
	// Upstream keeps `\u` with bad digits unchanged: after the 4-hex-digit
	// check fails, 'u' still matches the valid-escape set, so the sequence
	// passes through as-is.
	in := `{"a": "\uZZZZ"}`
	if got := Repair(in); got != in {
		t.Errorf("Repair = %q, want unchanged %q", got, in)
	}
}

func TestRepairTrailingBackslash(t *testing.T) {
	got := Repair(`{"a": "x\`)
	want := `{"a": "x\\`
	if got != want {
		t.Errorf("Repair = %q, want %q", got, want)
	}
}

func TestParseWithRepair(t *testing.T) {
	v, err := ParseWithRepair("{\"a\": \"has\nnewline\"}")
	if err != nil {
		t.Fatalf("ParseWithRepair: %v", err)
	}
	m := v.(map[string]any)
	if m["a"] != "has\nnewline" {
		t.Errorf("got %#v", m)
	}
}

func TestParseStreamingComplete(t *testing.T) {
	v := ParseStreamingObject(`{"city": "Paris", "n": 3}`)
	want := map[string]any{"city": "Paris", "n": float64(3)}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("got %#v", v)
	}
}

func TestParseStreamingTruncated(t *testing.T) {
	cases := []struct {
		in   string
		want map[string]any
	}{
		{``, map[string]any{}},
		{`   `, map[string]any{}},
		{`{`, map[string]any{}},
		{`{"`, map[string]any{}},
		{`{"ci`, map[string]any{}},
		{`{"city"`, map[string]any{}},
		{`{"city":`, map[string]any{}},
		{`{"city": "Pa`, map[string]any{"city": "Pa"}},
		{`{"city": "Paris"`, map[string]any{"city": "Paris"}},
		{`{"city": "Paris",`, map[string]any{"city": "Paris"}},
		{`{"city": "Paris", "n`, map[string]any{"city": "Paris"}},
		{`{"city": "Paris", "n":`, map[string]any{"city": "Paris"}},
		{`{"city": "Paris", "n": 3`, map[string]any{"city": "Paris", "n": float64(3)}},
		{`{"a": true, "b": fal`, map[string]any{"a": true, "b": false}},
		{`{"a": nul`, map[string]any{"a": nil}},
		{`{"nested": {"x": [1, 2`, map[string]any{"nested": map[string]any{"x": []any{float64(1), float64(2)}}}},
		{`{"s": "trailing\`, map[string]any{"s": "trailing"}},
		{`{"s": "uni\u00e`, map[string]any{"s": "uni"}},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := ParseStreamingObject(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParseStreamingObject(%q) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseStreamingArraysAndScalars(t *testing.T) {
	if got := ParseStreaming(`[1, "two", tru`); !reflect.DeepEqual(got, []any{float64(1), "two", true}) {
		t.Errorf("array: %#v", got)
	}
	if got := ParseStreaming(`"hel`); got != "hel" {
		t.Errorf("string: %#v", got)
	}
	if got := ParseStreaming(`nul`); !reflect.DeepEqual(got, map[string]any{}) {
		// TS: `result ?? {}` turns a null into an empty object.
		t.Errorf("null: %#v", got)
	}
	if got := ParseStreaming(`garbage!!`); !reflect.DeepEqual(got, map[string]any{}) {
		t.Errorf("garbage: %#v", got)
	}
}

func TestParseStreamingRepairedControlChars(t *testing.T) {
	// Raw newline inside a truncated string: repair + partial parse combine.
	got := ParseStreamingObject("{\"code\": \"line1\nline2")
	if got["code"] != "line1\nline2" {
		t.Errorf("got %#v", got)
	}
}

func TestParseStreamingUnicodeEscapePair(t *testing.T) {
	got := ParseStreamingObject(`{"s": "🙈"}`)
	if got["s"] != "🙈" {
		t.Errorf("surrogate pair: %#v", got)
	}
}
