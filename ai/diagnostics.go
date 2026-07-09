package ai

import (
	"fmt"
	"time"
)

// Ports: packages/ai/src/utils/diagnostics.ts

// DiagnosticErrorInfo is a redacted, serializable view of an error.
type DiagnosticErrorInfo struct {
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
	Stack   string `json:"stack,omitempty"`
	Code    any    `json:"code,omitempty"` // string or number
}

// AssistantMessageDiagnostic is a structured non-fatal error attachment on an
// AssistantMessage (failures and recoveries observed during a request).
type AssistantMessageDiagnostic struct {
	Type      string               `json:"type"`
	Timestamp int64                `json:"timestamp"`
	Error     *DiagnosticErrorInfo `json:"error,omitempty"`
	Details   map[string]any       `json:"details,omitempty"`
}

// FormatThrownValue renders any recovered value as an error message.
func FormatThrownValue(value any) string {
	switch v := value.(type) {
	case error:
		return v.Error()
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

// ExtractDiagnosticError converts an arbitrary recovered value into
// DiagnosticErrorInfo.
func ExtractDiagnosticError(err any) *DiagnosticErrorInfo {
	e, ok := err.(error)
	if !ok {
		return &DiagnosticErrorInfo{Name: "ThrownValue", Message: FormatThrownValue(err)}
	}
	info := &DiagnosticErrorInfo{Message: e.Error()}
	if coder, ok := e.(interface{ Code() string }); ok {
		info.Code = coder.Code()
	}
	return info
}

// NewAssistantMessageDiagnostic builds a diagnostic stamped with the current
// time.
func NewAssistantMessageDiagnostic(diagType string, err any, details map[string]any) AssistantMessageDiagnostic {
	return AssistantMessageDiagnostic{
		Type:      diagType,
		Timestamp: time.Now().UnixMilli(),
		Error:     ExtractDiagnosticError(err),
		Details:   details,
	}
}

// AppendDiagnostic appends a diagnostic to the message (copy-on-write, like
// the TS immutable append).
func AppendDiagnostic(message *AssistantMessage, diagnostic AssistantMessageDiagnostic) {
	next := make([]AssistantMessageDiagnostic, 0, len(message.Diagnostics)+1)
	next = append(next, message.Diagnostics...)
	next = append(next, diagnostic)
	message.Diagnostics = next
}
