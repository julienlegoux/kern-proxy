package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunHelp_DocumentsLoginAndList(t *testing.T) {
	var buf bytes.Buffer
	if err := runHelp(&buf); err != nil {
		t.Fatalf("runHelp: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"login", "list", "pi-ai"} {
		if !strings.Contains(out, want) {
			t.Errorf("help output missing %q, got:\n%s", want, out)
		}
	}
}
