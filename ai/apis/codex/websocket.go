package codex

// Ports: packages/ai/src/api/openai-codex-responses.ts

// Ports the WebSocket transport half of
// packages/ai/src/api/openai-codex-responses.ts -- acquireWebSocket/
// connectWebSocket/parseWebSocket/processWebSocketStream, the
// extractAccountId-adjacent header builder buildWebSocketHeaders, the
// CodexApiError/CodexProtocolError error classes, and the per-session
// fallback-memory + debug-stats bookkeeping (isWebSocketSseFallbackActive/
// recordWebSocketSseFallback/recordWebSocketFailure/
// getOpenAICodexWebSocketDebugStats) -- epic 7 issue 04's scope.
//
// Deliberately not ported: upstream's session-scoped WebSocket connection
// *cache* (the busy/idle/age-limit bookkeeping in acquireWebSocket that lets
// a later request reuse an already-open connection) and the
// "websocket-cached" delta-continuation mode built on top of it
// (buildCachedWebSocketRequestBody, previous_response_id / input-delta
// requests). Issue 04's Scope section lists WebSocket transport, the zstd
// SSE-fallback compression, per-session fallback memory, and connection-limit
// retry -- it does not list connection reuse or delta requests, and every
// request here opens (and closes) its own WebSocket connection. Because no
// connection is ever cached, `useCachedContext`'s upstream branch never finds
// a reusable entry, so this is a safe, behavior-preserving scope cut for the
// "auto"/"websocket-cached" transport values: both always send the full
// request body, same as "websocket". A follow-up issue can add connection
// reuse if the latency win is worth the extra state.
import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/julienlegoux/kern-proxy/ai"
	"github.com/julienlegoux/kern-proxy/ai/apis/openairesponses"
)

// openAIBetaResponsesWebSockets ports OPENAI_BETA_RESPONSES_WEBSOCKETS.
const openAIBetaResponsesWebSockets = "responses_websockets=2026-02-06"

// defaultWebSocketConnectTimeout ports DEFAULT_WEBSOCKET_CONNECT_TIMEOUT_MS.
const defaultWebSocketConnectTimeout = 15 * time.Second

// websocketConnectionLimitReachedCode ports WEBSOCKET_CONNECTION_LIMIT_REACHED_CODE.
const websocketConnectionLimitReachedCode = "websocket_connection_limit_reached"

// codexAPIError ports CodexApiError: a structured error the Codex backend
// reports in-band (a `type: "error"` WebSocket message, or a
// `response.failed` event). Never triggers an SSE fallback on its own --
// isCodexNonTransportError treats it as a real (non-transport) failure --
// except when Code is the connection-limit sentinel and no output has
// started yet, which retries the WebSocket connection once
// (isWebSocketConnectionLimitReached).
type codexAPIError struct {
	msg  string
	code string
}

func (e *codexAPIError) Error() string { return e.msg }

// codexProtocolError ports CodexProtocolError: malformed JSON received over
// the WebSocket transport. Also non-transport (see codexAPIError).
type codexProtocolError struct{ msg string }

func (e *codexProtocolError) Error() string { return e.msg }

// isCodexNonTransportError ports isCodexNonTransportError: true for either
// in-band error class above, false for ordinary transport failures (closed
// connections, idle timeouts, connect timeouts, network errors) that should
// fall back to SSE instead of propagating directly.
func isCodexNonTransportError(err error) bool {
	var apiErr *codexAPIError
	var protoErr *codexProtocolError
	return errors.As(err, &apiErr) || errors.As(err, &protoErr)
}

// isWebSocketConnectionLimitReached ports isWebSocketConnectionLimitReachedError.
func isWebSocketConnectionLimitReached(err error) bool {
	var apiErr *codexAPIError
	return errors.As(err, &apiErr) && apiErr.code == websocketConnectionLimitReachedCode
}

// --- per-session fallback memory + debug stats --------------------------

// codexWebSocketStats ports the subset of OpenAICodexWebSocketDebugStats this
// port tracks (the fields the connection-cache/delta-request machinery would
// have populated -- connectionsCreated, cachedContextRequests, etc. -- have no
// meaning here, see this file's package doc on the connection-cache scope
// cut).
type codexWebSocketStats struct {
	requests       int
	failures       int
	fallbacks      int
	fallbackActive bool
	lastError      string
}

var (
	codexWSMu       sync.Mutex
	codexWSFallback = map[string]bool{}
	codexWSStats    = map[string]*codexWebSocketStats{}
)

// codexWebSocketFallbackActive ports isWebSocketSseFallbackActive: once a
// session has fallen back to SSE, it stays there for the process lifetime (an
// empty sessionID is never sticky, matching upstream's `sessionId ? ... :
// false`).
func codexWebSocketFallbackActive(sessionID string) bool {
	if sessionID == "" {
		return false
	}
	codexWSMu.Lock()
	defer codexWSMu.Unlock()
	return codexWSFallback[sessionID]
}

// codexWSStatLocked returns (creating if needed) the stats entry for
// sessionID. Callers must hold codexWSMu.
func codexWSStatLocked(sessionID string) *codexWebSocketStats {
	s, ok := codexWSStats[sessionID]
	if !ok {
		s = &codexWebSocketStats{}
		codexWSStats[sessionID] = s
	}
	return s
}

// codexRecordWebSocketRequest ports the `stats.requests++` bump in
// processWebSocketStream.
func codexRecordWebSocketRequest(sessionID string) {
	if sessionID == "" {
		return
	}
	codexWSMu.Lock()
	defer codexWSMu.Unlock()
	codexWSStatLocked(sessionID).requests++
}

// codexRecordWebSocketSSEFallback ports recordWebSocketSseFallback.
func codexRecordWebSocketSSEFallback(sessionID string) {
	if sessionID == "" {
		return
	}
	codexWSMu.Lock()
	defer codexWSMu.Unlock()
	codexWSFallback[sessionID] = true
	s := codexWSStatLocked(sessionID)
	s.fallbacks++
	s.fallbackActive = true
}

// codexRecordWebSocketFailure ports recordWebSocketFailure.
func codexRecordWebSocketFailure(sessionID string, err error) {
	if sessionID == "" {
		return
	}
	codexWSMu.Lock()
	defer codexWSMu.Unlock()
	codexWSFallback[sessionID] = true
	s := codexWSStatLocked(sessionID)
	s.failures++
	s.lastError = err.Error()
	s.fallbackActive = true
}

// codexWebSocketDebugStats ports getOpenAICodexWebSocketDebugStats.
func codexWebSocketDebugStats(sessionID string) (codexWebSocketStats, bool) {
	codexWSMu.Lock()
	defer codexWSMu.Unlock()
	s, ok := codexWSStats[sessionID]
	if !ok {
		return codexWebSocketStats{}, false
	}
	return *s, true
}

// resetCodexWebSocketState ports resetOpenAICodexWebSocketDebugStats: an
// empty sessionID clears every session (used between tests), matching
// upstream's no-argument call.
func resetCodexWebSocketState(sessionID string) {
	codexWSMu.Lock()
	defer codexWSMu.Unlock()
	if sessionID == "" {
		codexWSFallback = map[string]bool{}
		codexWSStats = map[string]*codexWebSocketStats{}
		return
	}
	delete(codexWSFallback, sessionID)
	delete(codexWSStats, sessionID)
}

// --- transport dispatch ---------------------------------------------------

// attemptCodexWebSocket ports the `while (true) { ... }` connection-limit
// retry loop in stream(): one WebSocket exchange, retried exactly once if the
// backend reports a connection-limit error before any output has started.
// started reports whether at least one message was ever received (ports
// startWebSocketOutputOnFirstEvent's onStart signal) -- once true, a
// subsequent failure must propagate as the final error rather than fall back
// to SSE (codex.go's run() makes that call using the returned err/started
// pair).
func attemptCodexWebSocket(
	ctx context.Context,
	out *ai.Stream,
	output *ai.AssistantMessage,
	model *ai.Model,
	opts *ai.StreamOptions,
	bodyBytes []byte,
	accountID, apiKey string,
) (started bool, err error) {
	retried := false
	for {
		started, err = processCodexWebSocketStream(ctx, out, output, model, opts, bodyBytes, accountID, apiKey)
		if err == nil {
			return started, nil
		}
		if ctx.Err() == nil && !started && isWebSocketConnectionLimitReached(err) && !retried {
			retried = true
			continue
		}
		return started, err
	}
}

// processCodexWebSocketStream ports processWebSocketStream (minus the
// connection-cache/delta-continuation half, see package doc): opens one
// WebSocket connection, sends the `response.create` request, and decodes the
// resulting event stream into out/output via the shared
// openairesponses.DecodeStream (fed through an io.Pipe so the WS-specific
// in-band error classification in this file's pump can run ahead of it).
func processCodexWebSocketStream(
	ctx context.Context,
	out *ai.Stream,
	output *ai.AssistantMessage,
	model *ai.Model,
	opts *ai.StreamOptions,
	bodyBytes []byte,
	accountID, apiKey string,
) (started bool, err error) {
	wsURL, err := resolveCodexWebSocketURL(model.BaseURL)
	if err != nil {
		return false, err
	}

	var sessionID string
	if opts != nil {
		sessionID = opts.SessionID
	}
	requestID := sessionID
	if requestID == "" {
		requestID = newCodexRequestID()
	}
	headers := buildWebSocketHeaders(model, opts, accountID, apiKey, requestID)

	connectTimeout := defaultWebSocketConnectTimeout
	if opts != nil && opts.WebsocketConnectTimeout > 0 {
		connectTimeout = opts.WebsocketConnectTimeout
	}
	dialCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	conn, _, dialErr := websocket.Dial(dialCtx, wsURL, &websocket.DialOptions{HTTPHeader: toHTTPHeader(headers)})
	cancel()
	if dialErr != nil {
		if ctx.Err() != nil {
			return false, errors.New("Request was aborted")
		}
		if errors.Is(dialErr, context.DeadlineExceeded) {
			return false, fmt.Errorf("WebSocket connect timeout after %dms", connectTimeout.Milliseconds())
		}
		return false, dialErr
	}
	defer func() { _ = conn.CloseNow() }()

	codexRecordWebSocketRequest(sessionID)

	sendPayload, err := buildWebSocketSendPayload(bodyBytes)
	if err != nil {
		return false, err
	}
	if err := conn.Write(ctx, websocket.MessageText, sendPayload); err != nil {
		if ctx.Err() != nil {
			return false, errors.New("Request was aborted")
		}
		return false, err
	}

	var idleTimeout time.Duration
	if opts != nil {
		idleTimeout = opts.Timeout
	}

	pr, pw := io.Pipe()
	var startedFlag atomic.Bool
	go pumpCodexWebSocketMessages(ctx, conn, pw, idleTimeout, out, output, &startedFlag)

	var serviceTier string
	if opts != nil {
		serviceTier = opts.ServiceTier
	}
	decodeErr := openairesponses.DecodeStream(out, output, model, pr, openairesponses.ServiceTierOptions{
		RequestServiceTier: serviceTier,
		ResolveServiceTier: resolveCodexServiceTier,
	})
	return startedFlag.Load(), decodeErr
}

// buildWebSocketSendPayload ports `JSON.stringify({ type: "response.create",
// ...requestBody })`: bodyBytes is the same already-serialized (and possibly
// OnPayload-replaced) request payload the SSE path sends, so the type field
// is merged in by round-tripping through a generic map rather than a typed
// struct -- OnPayload can replace the payload with any JSON-shaped value, not
// just *wireRequest.
func buildWebSocketSendPayload(bodyBytes []byte) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]any{}
	}
	m["type"] = "response.create"
	return json.Marshal(m)
}

// pumpCodexWebSocketMessages reads WebSocket messages and feeds them, framed
// as SSE `data:` events, into pw for openairesponses.DecodeStream to consume
// on the other end of the pipe -- ports parseWebSocket + mapCodexEvents
// together: each message is classified (in-band error/response.failed
// detection) before being forwarded, and the pump stops right after
// forwarding a terminal response.completed/incomplete message (ports
// parseWebSocket's `done` flag), closing the pipe cleanly so DecodeStream
// returns nil having already seen its terminal event.
func pumpCodexWebSocketMessages(
	ctx context.Context,
	conn *websocket.Conn,
	pw *io.PipeWriter,
	idleTimeout time.Duration,
	out *ai.Stream,
	output *ai.AssistantMessage,
	started *atomic.Bool,
) {
	for {
		readCtx := ctx
		var cancel context.CancelFunc
		if idleTimeout > 0 {
			readCtx, cancel = context.WithTimeout(ctx, idleTimeout)
		}
		_, data, err := conn.Read(readCtx)
		if cancel != nil {
			cancel()
		}
		if err != nil {
			switch {
			case ctx.Err() != nil:
				_ = pw.CloseWithError(errors.New("Request was aborted"))
			case idleTimeout > 0 && readCtx.Err() != nil:
				_ = pw.CloseWithError(fmt.Errorf("WebSocket idle timeout after %dms", idleTimeout.Milliseconds()))
			default:
				_ = pw.CloseWithError(fmt.Errorf("WebSocket closed: %w", err))
			}
			return
		}

		// A message only counts as "output has started" once it's an actual
		// forwarded event, not an in-band error/response.failed -- ports
		// upstream's composition order (mapCodexEvents throws before
		// startWebSocketOutputOnFirstEvent's onStart ever fires for those
		// message types), which is what lets the connection-limit-reached
		// retry recognize "no real output happened yet" even though a
		// message was technically received.
		frame, classifyErr, terminal := classifyCodexWebSocketMessage(data)
		if classifyErr != nil {
			_ = pw.CloseWithError(classifyErr)
			return
		}

		if !started.Swap(true) {
			out.Push(ai.StartEvent{Partial: output.Clone()})
		}
		if _, werr := pw.Write(frame); werr != nil {
			return
		}
		if terminal {
			_ = pw.Close()
			return
		}
	}
}

// codexWSEnvelope is the subset of a Codex WebSocket message this file needs
// to classify it, mirroring extractCodexEventError's two possible error
// shapes (top-level code/message, or a nested `error` object).
type codexWSEnvelope struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Response *struct {
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	} `json:"response"`
}

// classifyCodexWebSocketMessage ports mapCodexEvents' per-message handling:
// an "error" message becomes a codexAPIError; a "response.failed" message
// becomes one too (from its nested response.error); "response.done" is
// normalized to "response.completed" (upstream yields it as such) so the
// shared openairesponses.DecodeStream -- which only recognizes the literal
// "response.completed"/"response.incomplete" strings -- treats it as
// terminal; everything else is forwarded verbatim, framed as one SSE `data:`
// event.
func classifyCodexWebSocketMessage(data []byte) (frame []byte, err error, terminal bool) {
	var env codexWSEnvelope
	if jsonErr := json.Unmarshal(data, &env); jsonErr != nil {
		return nil, &codexProtocolError{msg: fmt.Sprintf("Invalid Codex WebSocket JSON: %v", jsonErr)}, false
	}

	switch env.Type {
	case "error":
		code := env.Code
		if code == "" && env.Error != nil {
			code = env.Error.Code
		}
		msg := env.Message
		if msg == "" && env.Error != nil {
			msg = env.Error.Message
		}
		text := msg
		if text == "" {
			text = code
		}
		if text == "" {
			text = string(data)
		}
		return nil, &codexAPIError{msg: "Codex error: " + text, code: code}, false

	case "response.failed":
		msg := "Codex response failed"
		code := ""
		if env.Response != nil && env.Response.Error != nil {
			if env.Response.Error.Message != "" {
				msg = env.Response.Error.Message
			}
			code = env.Response.Error.Code
		}
		return nil, &codexAPIError{msg: msg, code: code}, false

	case "response.done":
		data = rewriteCodexWebSocketType(data, "response.completed")
		terminal = true

	case "response.completed", "response.incomplete":
		terminal = true
	}

	return append(append([]byte("data: "), data...), []byte("\n\n")...), nil, terminal
}

// rewriteCodexWebSocketType rewrites a JSON object's top-level "type" field.
// On any decode failure it returns data unchanged (the shared decoder simply
// won't recognize the original type, degrading no worse than not rewriting
// at all).
func rewriteCodexWebSocketType(data []byte, newType string) []byte {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return data
	}
	rewritten, err := json.Marshal(newType)
	if err != nil {
		return data
	}
	m["type"] = rewritten
	out, err := json.Marshal(m)
	if err != nil {
		return data
	}
	return out
}

// --- headers, URL, ids -----------------------------------------------------

// buildWebSocketHeaders ports buildBaseCodexHeaders + buildWebSocketHeaders:
// the WebSocket transport drops the SSE-only accept/content-type headers and
// negotiates the WebSocket-flavored OpenAI-Beta value, always stamping
// x-client-request-id/session-id with requestId (sessionId when present, else
// a generated id) -- unlike the SSE path's buildSSEHeaders, which only sets
// those two headers when a sessionId was actually provided.
func buildWebSocketHeaders(model *ai.Model, opts *ai.StreamOptions, accountID, apiKey, requestID string) map[string]string {
	defaults := map[string]string{
		"authorization":      "Bearer " + apiKey,
		"chatgpt-account-id": accountID,
		"originator":         "pi",
		"user-agent":         fmt.Sprintf("pi (%s; %s)", runtime.GOOS, runtime.GOARCH),
	}
	for k, v := range model.Headers {
		defaults[k] = v
	}
	delete(defaults, "accept")
	delete(defaults, "content-type")
	defaults["openai-beta"] = openAIBetaResponsesWebSockets
	defaults["x-client-request-id"] = requestID
	defaults["session-id"] = requestID

	var optHeaders ai.ProviderHeaders
	if opts != nil {
		optHeaders = opts.Headers
	}
	return ai.MergeProviderHeaders(defaults, optHeaders)
}

// toHTTPHeader converts the plain header map this package builds into the
// http.Header shape websocket.DialOptions expects.
func toHTTPHeader(headers map[string]string) http.Header {
	h := make(http.Header, len(headers))
	for k, v := range headers {
		h.Set(k, v)
	}
	return h
}

// resolveCodexWebSocketURL ports resolveCodexWebSocketUrl: the Codex SSE URL
// with its scheme swapped for the WebSocket equivalent.
func resolveCodexWebSocketURL(baseURL string) (string, error) {
	u, err := url.Parse(resolveCodexURL(baseURL))
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	}
	return u.String(), nil
}

// newCodexRequestID ports createCodexRequestId's fallback branch (Go has no
// crypto.randomUUID equivalent in the standard library, so this always uses
// the timestamp+random-suffix form upstream falls back to when
// crypto.randomUUID is unavailable; both forms are just opaque correlation
// ids to the backend).
func newCodexRequestID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return fmt.Sprintf("codex_%d_%s", time.Now().UnixMilli(), hex.EncodeToString(buf[:]))
	}
	return fmt.Sprintf("codex_%d", time.Now().UnixMilli())
}
