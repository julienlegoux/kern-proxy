// Package google implements the Google Generative AI (Gemini) wire adapter:
// shared request/response converters (contents, tools, generation config),
// thinking-signature handling, and streaming decode into the unified event
// protocol, over raw net/http and the hand-rolled SSE parser from
// ai/internal/sse (upstream delegates transport to the @google/genai SDK; the
// Go port has no such SDK, so it speaks the Generative Language REST API
// directly -- see PORTING.md's "Vendor SDKs" deviation).
//
// ConvertMessages/ConvertTools/MapToolChoice/MapStopReason and DecodeStream
// (stream.go) are exported so the Vertex variant (epic 8, issue 02) can reuse
// them unchanged, differing only in endpoint resolution and auth.
//
// Ports: packages/ai/src/api/google-shared.ts, google-generative-ai.ts
package google

import (
	"encoding/json"
	"strings"

	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis"
)

// GoogleContent is one entry of the Gemini `contents` array. Parts are kept
// as map[string]any (rather than a fixed struct) since a Part's shape varies
// by kind (text, inlineData, functionCall, functionResponse, thought) and
// this mirrors the flexible object-literal construction in google-shared.ts.
type GoogleContent struct {
	Role  string           `json:"role"`
	Parts []map[string]any `json:"parts"`
}

// GoogleFunctionDeclaration tool group, matching the Gemini
// `{functionDeclarations: [...]}` tools[0] shape.
type GoogleToolGroup struct {
	FunctionDeclarations []map[string]any `json:"functionDeclarations"`
}

// IsThinkingPart reports whether a streamed Gemini Part should be treated as
// "thinking": the definitive marker is `thought: true`; thoughtSignature can
// appear on any part type and does not itself indicate thinking content. See
// https://ai.google.dev/gemini-api/docs/thought-signatures. Ports
// isThinkingPart from google-shared.ts.
func IsThinkingPart(thought bool, _ string) bool {
	return thought
}

// RetainThoughtSignature preserves the last non-empty thought signature for
// the current streamed block: some backends only send thoughtSignature on
// the first delta for a given part; later deltas may omit it. Ports
// retainThoughtSignature from google-shared.ts.
func RetainThoughtSignature(existing, incoming string) string {
	if incoming != "" {
		return incoming
	}
	return existing
}

// base64SignaturePattern matches the base64 (TYPE_BYTES) encoding Google
// requires for thought signatures.
func isValidThoughtSignature(sig string) bool {
	if sig == "" {
		return false
	}
	if len(sig)%4 != 0 {
		return false
	}
	for _, r := range sig {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '+' && r != '/' && r != '=' {
			return false
		}
	}
	return true
}

// resolveThoughtSignature only keeps signatures from the same provider/model
// with valid base64. Ports resolveThoughtSignature from google-shared.ts.
func resolveThoughtSignature(isSameProviderAndModel bool, signature string) string {
	if isSameProviderAndModel && isValidThoughtSignature(signature) {
		return signature
	}
	return ""
}

// RequiresToolCallID reports whether modelID needs explicit tool-call ids in
// function calls/responses (models served behind Cloud Code Assist that
// aren't native Gemini). Ports requiresToolCallId from google-shared.ts.
func RequiresToolCallID(modelID string) bool {
	return strings.HasPrefix(modelID, "claude-") || strings.HasPrefix(modelID, "gpt-oss-")
}

// geminiMajorVersion extracts the leading version number from a
// "gemini-N..." or "gemini-live-N..." model id, or -1 if it doesn't match.
func geminiMajorVersion(modelID string) int {
	id := strings.ToLower(modelID)
	id = strings.TrimPrefix(id, "gemini-live-")
	if !strings.HasPrefix(id, "gemini-") {
		return -1
	}
	rest := strings.TrimPrefix(id, "gemini-")
	i := 0
	for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
		i++
	}
	if i == 0 {
		return -1
	}
	n := 0
	for _, c := range rest[:i] {
		n = n*10 + int(c-'0')
	}
	return n
}

// supportsMultimodalFunctionResponse reports whether modelID accepts images
// nested inside a functionResponse's parts (Gemini 3+); older models and
// non-Gemini models behind Cloud Code Assist need a separate user image turn.
// Ports supportsMultimodalFunctionResponse from google-shared.ts.
func supportsMultimodalFunctionResponse(modelID string) bool {
	if v := geminiMajorVersion(modelID); v != -1 {
		return v >= 3
	}
	return true
}

// ConvertMessages converts the unified message history into Gemini `Content`
// entries. Ports convertMessages from google-shared.ts.
func ConvertMessages(model *ai.Model, chat ai.Context) []GoogleContent {
	var contents []GoogleContent
	normalizeToolCallID := func(id string, m *ai.Model, _ *ai.AssistantMessage) string {
		if !RequiresToolCallID(m.ID) {
			return id
		}
		return normalizeID(id)
	}

	transformed := apis.TransformMessages(chat.Messages, model, normalizeToolCallID)

	for i := 0; i < len(transformed); i++ {
		switch msg := transformed[i].(type) {
		case *ai.UserMessage:
			if msg.Content.Plain != nil {
				text := ai.SanitizeSurrogates(*msg.Content.Plain)
				contents = append(contents, GoogleContent{Role: "user", Parts: []map[string]any{{"text": text}}})
				continue
			}
			var parts []map[string]any
			for _, item := range msg.Content.Blocks {
				switch b := item.(type) {
				case ai.TextContent:
					parts = append(parts, map[string]any{"text": ai.SanitizeSurrogates(b.Text)})
				case ai.ImageContent:
					parts = append(parts, inlineDataPart(b))
				}
			}
			if len(parts) == 0 {
				continue
			}
			contents = append(contents, GoogleContent{Role: "user", Parts: parts})

		case *ai.AssistantMessage:
			isSameProviderAndModel := msg.Provider == model.Provider && msg.Model == model.ID
			var parts []map[string]any
			for _, block := range msg.Content {
				switch b := block.(type) {
				case ai.TextContent:
					if strings.TrimSpace(b.Text) == "" {
						continue
					}
					part := map[string]any{"text": ai.SanitizeSurrogates(b.Text)}
					if sig := resolveThoughtSignature(isSameProviderAndModel, b.TextSignature); sig != "" {
						part["thoughtSignature"] = sig
					}
					parts = append(parts, part)

				case ai.ThinkingContent:
					if strings.TrimSpace(b.Thinking) == "" {
						continue
					}
					if isSameProviderAndModel {
						part := map[string]any{"thought": true, "text": ai.SanitizeSurrogates(b.Thinking)}
						if sig := resolveThoughtSignature(isSameProviderAndModel, b.ThinkingSignature); sig != "" {
							part["thoughtSignature"] = sig
						}
						parts = append(parts, part)
					} else {
						parts = append(parts, map[string]any{"text": ai.SanitizeSurrogates(b.Thinking)})
					}

				case ai.ToolCall:
					args := any(b.Arguments)
					if b.Arguments == nil {
						args = map[string]any{}
					}
					fc := map[string]any{"name": b.Name, "args": args}
					if RequiresToolCallID(model.ID) {
						fc["id"] = b.ID
					}
					part := map[string]any{"functionCall": fc}
					if sig := resolveThoughtSignature(isSameProviderAndModel, b.ThoughtSignature); sig != "" {
						part["thoughtSignature"] = sig
					}
					parts = append(parts, part)
				}
			}
			if len(parts) == 0 {
				continue
			}
			contents = append(contents, GoogleContent{Role: "model", Parts: parts})

		case *ai.ToolResultMessage:
			part := functionResponsePart(msg, model)
			if len(contents) > 0 && contents[len(contents)-1].Role == "user" && hasFunctionResponse(contents[len(contents)-1]) {
				last := &contents[len(contents)-1]
				last.Parts = append(last.Parts, part)
			} else {
				contents = append(contents, GoogleContent{Role: "user", Parts: []map[string]any{part}})
			}

			if hasImageContent(msg.Content) && model.SupportsImageInput() && !supportsMultimodalFunctionResponse(model.ID) {
				imgParts := []map[string]any{{"text": "Tool result image:"}}
				for _, c := range msg.Content {
					if img, ok := c.(ai.ImageContent); ok {
						imgParts = append(imgParts, inlineDataPart(img))
					}
				}
				contents = append(contents, GoogleContent{Role: "user", Parts: imgParts})
			}
		}
	}

	return contents
}

func hasFunctionResponse(c GoogleContent) bool {
	for _, p := range c.Parts {
		if _, ok := p["functionResponse"]; ok {
			return true
		}
	}
	return false
}

func hasImageContent(content []ai.UserContentPart) bool {
	for _, c := range content {
		if _, ok := c.(ai.ImageContent); ok {
			return true
		}
	}
	return false
}

func inlineDataPart(b ai.ImageContent) map[string]any {
	return map[string]any{"inlineData": map[string]any{"mimeType": b.MimeType, "data": b.Data}}
}

// functionResponsePart builds one `functionResponse` part for a tool result,
// including nested image parts on models that support multimodal function
// responses (Gemini 3+). Ports the toolResult branch of convertMessages.
func functionResponsePart(m *ai.ToolResultMessage, model *ai.Model) map[string]any {
	var texts []string
	var images []ai.ImageContent
	for _, c := range m.Content {
		switch b := c.(type) {
		case ai.TextContent:
			texts = append(texts, b.Text)
		case ai.ImageContent:
			if model.SupportsImageInput() {
				images = append(images, b)
			}
		}
	}
	hasText := len(texts) > 0
	hasImages := len(images) > 0

	var value string
	switch {
	case hasText:
		value = ai.SanitizeSurrogates(strings.Join(texts, "\n"))
	case hasImages:
		value = "(see attached image)"
	default:
		value = ""
	}

	response := map[string]any{}
	if m.IsError {
		response["error"] = value
	} else {
		response["output"] = value
	}

	fr := map[string]any{"name": m.ToolName, "response": response}
	if hasImages && supportsMultimodalFunctionResponse(model.ID) {
		var imgParts []map[string]any
		for _, img := range images {
			imgParts = append(imgParts, inlineDataPart(img))
		}
		fr["parts"] = imgParts
	}
	if RequiresToolCallID(model.ID) {
		fr["id"] = m.ToolCallID
	}
	return map[string]any{"functionResponse": fr}
}

// normalizeID adapts a tool-call id to Gemini's id constraints when required
// (Claude/gpt-oss models behind Cloud Code Assist): strip disallowed
// characters and cap at 64 chars. Ports the normalizeToolCallId closure from
// convertMessages in google-shared.ts.
func normalizeID(id string) string {
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	out := b.String()
	if len(out) > 64 {
		out = out[:64]
	}
	return out
}

// jsonSchemaMetaKeywords are meta-declarations stripped when useParameters
// downgrades a tool's JSON Schema to the legacy OpenAPI-3.03 `parameters`
// field. Ports JSON_SCHEMA_META_DECLARATIONS from google-shared.ts.
var jsonSchemaMetaKeywords = map[string]bool{
	"$schema": true, "$id": true, "$anchor": true, "$dynamicAnchor": true,
	"$vocabulary": true, "$comment": true, "$defs": true, "definitions": true,
}

// sanitizeForOpenAPI strips JSON Schema meta-declarations recursively so the
// legacy `parameters` field only carries fields OpenAPI 3.03 understands.
// Ports sanitizeForOpenApi from google-shared.ts.
func sanitizeForOpenAPI(schema any) any {
	m, ok := schema.(map[string]any)
	if !ok {
		return schema
	}
	out := map[string]any{}
	for k, v := range m {
		if jsonSchemaMetaKeywords[k] {
			continue
		}
		out[k] = sanitizeForOpenAPI(v)
	}
	return out
}

// ConvertTools converts tools to Gemini function-declaration format. By
// default uses `parametersJsonSchema` (full JSON Schema support); useParameters
// switches to the legacy OpenAPI 3.03 `parameters` field, needed for Cloud
// Code Assist with Claude models. Ports convertTools from google-shared.ts.
func ConvertTools(tools []ai.Tool, useParameters bool) []GoogleToolGroup {
	if len(tools) == 0 {
		return nil
	}
	decls := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		decl := map[string]any{"name": t.Name, "description": t.Description}
		var parsed any
		if len(t.Parameters) > 0 {
			_ = json.Unmarshal(t.Parameters, &parsed)
		}
		if useParameters {
			decl["parameters"] = sanitizeForOpenAPI(parsed)
		} else {
			decl["parametersJsonSchema"] = parsed
		}
		decls = append(decls, decl)
	}
	return []GoogleToolGroup{{FunctionDeclarations: decls}}
}

// MapToolChoice maps the abstract tool-choice string to Gemini's
// FunctionCallingConfigMode wire value. Ports mapToolChoice from
// google-shared.ts.
func MapToolChoice(choice string) string {
	switch choice {
	case "none":
		return "NONE"
	case "any":
		return "ANY"
	default:
		return "AUTO"
	}
}

// MapStopReasonString maps a raw string finish reason (as seen on streamed
// chunks) to ai.StopReason. Ports both mapStopReason and mapStopReasonString
// from google-shared.ts: upstream's SDK-typed mapStopReason exhaustively
// switches over the FinishReason enum's ~15 values (everything but STOP and
// MAX_TOKENS maps to "error"), and mapStopReasonString is upstream's own
// plain-string equivalent for raw API responses. Since this Go port talks the
// REST API directly rather than through the @google/genai SDK, every finish
// reason arrives as a plain string, so one function covers both: the default
// branch already reproduces the enum switch's "everything else is error"
// behavior.
func MapStopReasonString(reason string) ai.StopReason {
	switch reason {
	case "STOP":
		return ai.StopReasonStop
	case "MAX_TOKENS":
		return ai.StopReasonLength
	default:
		return ai.StopReasonError
	}
}
