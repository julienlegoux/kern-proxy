package ai

// Ports: packages/ai/src/auth/context.ts

import (
	"os"
	"path/filepath"
	"strings"
)

type processAuthContext struct{}

// Env reads the process environment, trimming whitespace; an empty value
// means unset (matching the TS behavior of returning undefined for empty).
func (processAuthContext) Env(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

// FileExists checks a path, expanding a leading "~" to the home directory.
func (processAuthContext) FileExists(path string) bool {
	expanded := ExpandHomePath(path)
	info, err := os.Stat(expanded)
	return err == nil && !info.IsDir()
}

// DefaultAuthContext returns the process-environment AuthContext.
func DefaultAuthContext() AuthContext { return processAuthContext{} }

// ExpandHomePath expands a leading "~" or "~/" to the user home directory.
func ExpandHomePath(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		if path == "~" {
			return home
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
