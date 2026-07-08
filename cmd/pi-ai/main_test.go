package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun_NoArgsPrintsHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(nil, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Errorf("expected usage text on stdout, got:\n%s", stdout.String())
	}
}

func TestRun_HelpCommand(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}} {
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr)
		if code != 0 {
			t.Errorf("run(%v) exit code = %d, want 0", args, code)
		}
		if !strings.Contains(stdout.String(), "Usage:") {
			t.Errorf("run(%v): expected usage text on stdout, got:\n%s", args, stdout.String())
		}
	}
}

func TestRun_ListCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}
	// The real built-in catalog (ai/providers) should include "anthropic".
	if !strings.Contains(stdout.String(), "anthropic") {
		t.Errorf("expected the real catalog to include anthropic, got:\n%s", stdout.String())
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"bogus"}, &stdout, &stderr)
	if code == 0 {
		t.Error("expected a non-zero exit code for an unknown command")
	}
	if !strings.Contains(stderr.String(), "bogus") {
		t.Errorf("expected stderr to name the unknown command, got:\n%s", stderr.String())
	}
}
