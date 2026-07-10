package codex

// Ports the WebSocket/fallback tests from
// upstream/.upstream-clone/packages/ai/test/openai-codex-stream.test.ts that
// stream_test.go's HTTP/SSE-only suite (issue 03) didn't cover -- issue 04's
// scope. Unlike upstream (which mocks the global WebSocket/fetch
// constructors), every test here drives a real local WebSocket server over an
// actual TCP/HTTP-upgraded connection via github.com/coder/websocket, which is
// also this package's answer to the epic's "WebSocket integration test
// against a local ws server" acceptance criterion -- there's no separate
// bespoke integration test because every case below already is one.
//
// A single httptest.Server plays both roles a Codex base URL resolves to
// (resolveCodexURL for SSE, resolveCodexWebSocketURL for WebSocket): its
// handler branches on the Upgrade header, matching how upstream's tests stub
// both `fetch` and `WebSocket` against the same conceptual endpoint.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/julienlegoux/kern-link/ai"
)

// readCodexWSRequest reads and decodes one client message. Uses t.Errorf (not
// Fatalf): this runs on the httptest handler's own goroutine, and only the
// goroutine executing the test function may call Fatal/FailNow.
func readCodexWSRequest(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	_, data, err := conn.Read(context.Background())
	if err != nil {
		t.Errorf("read websocket request: %v", err)
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Errorf("unmarshal websocket request: %v", err)
		return nil
	}
	return m
}

// sendCodexWSMessage marshals and writes one server->client message (see
// readCodexWSRequest on why this only uses Errorf).
func sendCodexWSMessage(t *testing.T, conn *websocket.Conn, msg any) {
	t.Helper()
	data, err := json.Marshal(msg)
	if err != nil {
		t.Errorf("marshal websocket message: %v", err)
		return
	}
	if err := conn.Write(context.Background(), websocket.MessageText, data); err != nil {
		t.Errorf("write websocket message: %v", err)
	}
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

// TestStream_WebSocketHappyPathOverRealServer is the epic's "WebSocket
// integration test against a local ws server": a full round trip over a real
// WebSocket connection (no fallback, no retry), verifying the transport
// itself decodes correctly end to end.
func TestStream_WebSocketHappyPathOverRealServer(t *testing.T) {
	t.Cleanup(func() { resetCodexWebSocketState("") })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()

		req := readCodexWSRequest(t, conn)
		if req["type"] != "response.create" {
			t.Errorf("request type = %v, want response.create", req["type"])
		}
		if req["model"] != "gpt-5.1-codex" {
			t.Errorf("request model = %v, want gpt-5.1-codex", req["model"])
		}

		sendCodexWSMessage(t, conn, map[string]any{
			"type": "response.output_item.added",
			"item": map[string]any{"type": "message", "id": "msg_1", "role": "assistant", "status": "in_progress", "content": []any{}},
		})
		sendCodexWSMessage(t, conn, map[string]any{"type": "response.output_text.delta", "delta": "Hello"})
		sendCodexWSMessage(t, conn, map[string]any{
			"type": "response.output_item.done",
			"item": map[string]any{
				"type": "message", "id": "msg_1", "role": "assistant", "status": "completed",
				"content": []any{map[string]any{"type": "output_text", "text": "Hello"}},
			},
		})
		sendCodexWSMessage(t, conn, map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"status": "completed",
				"usage":  map[string]any{"input_tokens": 5, "output_tokens": 3, "total_tokens": 8},
			},
		})
		_ = conn.Close(websocket.StatusNormalClosure, "done")
	}))
	t.Cleanup(srv.Close)

	model := codexModel(srv.URL)
	token := mockCodexToken(t, "acc_test")
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("Say hello"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: token, Transport: ai.TransportWebSocket})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonStop {
		t.Fatalf("stopReason = %q, want stop (errorMessage=%q)", result.StopReason, result.ErrorMessage)
	}
	text, ok := result.Content[0].(ai.TextContent)
	if !ok || text.Text != "Hello" {
		t.Errorf("content[0] = %#v, want text Hello", result.Content[0])
	}
}

// TestStream_WebSocketConnectTimeoutFallsBackToSSE ports "falls back to SSE
// when websocket connect does not open before the connect timeout": the
// WebSocket upgrade never completes, so the connect timeout should fire and
// the request should fall back to (and succeed over) SSE.
func TestStream_WebSocketConnectTimeoutFallsBackToSSE(t *testing.T) {
	t.Cleanup(func() { resetCodexWebSocketState("") })

	block := make(chan struct{})
	defer close(block)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isWebSocketUpgrade(r) {
			<-block // never completes the handshake
			return
		}
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(codexSSEPayload("completed")))
	}))
	t.Cleanup(srv.Close)

	model := codexModel(srv.URL)
	token := mockCodexToken(t, "")
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("Say hello"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{
		APIKey:                  token,
		SessionID:               "ws-connect-timeout",
		Transport:               ai.TransportAuto,
		WebsocketConnectTimeout: 50 * time.Millisecond,
	})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	text, _ := result.Content[0].(ai.TextContent)
	if text.Text != "Hello" {
		t.Fatalf("content text = %q, want Hello (fell back to SSE, errorMessage=%q)", text.Text, result.ErrorMessage)
	}

	stats, ok := codexWebSocketDebugStats("ws-connect-timeout")
	if !ok {
		t.Fatalf("no websocket stats recorded for session")
	}
	if stats.failures != 1 || stats.fallbacks != 1 || !stats.fallbackActive {
		t.Errorf("stats = %+v, want 1 failure, 1 fallback, active", stats)
	}
	if !strings.Contains(stats.lastError, "WebSocket connect timeout after 50ms") {
		t.Errorf("lastError = %q, want a connect-timeout message", stats.lastError)
	}
}

// TestStream_WebSocketConnectionLimitReachedRetriesOnce ports "reconnects
// once when the websocket connection limit is reached before output starts":
// the first connection's in-band error is the connection-limit sentinel, so
// a second connection should be opened automatically and its response used,
// with no SSE fallback.
func TestStream_WebSocketConnectionLimitReachedRetriesOnce(t *testing.T) {
	t.Cleanup(func() { resetCodexWebSocketState("") })

	var connections int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()

		n := atomic.AddInt32(&connections, 1)
		_ = readCodexWSRequest(t, conn)
		if n == 1 {
			sendCodexWSMessage(t, conn, map[string]any{
				"type":  "error",
				"error": map[string]any{"code": "websocket_connection_limit_reached"},
			})
			return
		}
		sendCodexWSMessage(t, conn, map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id": "resp_1", "status": "completed",
				"usage": map[string]any{"input_tokens": 5, "output_tokens": 3, "total_tokens": 8},
			},
		})
	}))
	t.Cleanup(srv.Close)

	model := codexModel(srv.URL)
	token := mockCodexToken(t, "")
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("hi"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{APIKey: token, Transport: ai.TransportWebSocket})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonStop {
		t.Fatalf("stopReason = %q, want stop (errorMessage=%q)", result.StopReason, result.ErrorMessage)
	}
	if got := atomic.LoadInt32(&connections); got != 2 {
		t.Errorf("connections = %d, want 2 (one retry)", got)
	}
}

// TestStream_WebSocketIdleBeforeFirstEventFallsBackToSSE ports "falls back to
// SSE when a websocket is idle before the first event": the server accepts
// the connection but never sends a message, so the idle timeout should fire
// before any output starts and the request should fall back to SSE.
func TestStream_WebSocketIdleBeforeFirstEventFallsBackToSSE(t *testing.T) {
	t.Cleanup(func() { resetCodexWebSocketState("") })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isWebSocketUpgrade(r) {
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			defer func() { _ = conn.CloseNow() }()
			_ = readCodexWSRequest(t, conn)
			<-r.Context().Done() // consume the request, then go silent
			return
		}
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(codexSSEPayload("completed")))
	}))
	t.Cleanup(srv.Close)

	model := codexModel(srv.URL)
	token := mockCodexToken(t, "")
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("Say hello"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{
		APIKey:    token,
		SessionID: "ws-idle-before-start",
		Transport: ai.TransportAuto,
		Timeout:   50 * time.Millisecond,
	})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	text, _ := result.Content[0].(ai.TextContent)
	if text.Text != "Hello" {
		t.Fatalf("content text = %q, want Hello (fell back to SSE, errorMessage=%q)", text.Text, result.ErrorMessage)
	}

	stats, ok := codexWebSocketDebugStats("ws-idle-before-start")
	if !ok || stats.failures != 1 || stats.fallbacks != 1 || !stats.fallbackActive {
		t.Errorf("stats = %+v (ok=%v), want 1 failure, 1 fallback, active", stats, ok)
	}
}

// TestStream_WebSocketIdleAfterStartErrors ports "errors when a websocket is
// idle after the stream started": once one (non-terminal) message has
// arrived, output has "started" -- a subsequent idle timeout must be the
// final error, not trigger an SSE fallback.
func TestStream_WebSocketIdleAfterStartErrors(t *testing.T) {
	t.Cleanup(func() { resetCodexWebSocketState("") })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isWebSocketUpgrade(r) {
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			defer func() { _ = conn.CloseNow() }()
			_ = readCodexWSRequest(t, conn)
			sendCodexWSMessage(t, conn, map[string]any{
				"type": "response.output_item.added",
				"item": map[string]any{"type": "message", "id": "msg_1", "role": "assistant", "status": "in_progress", "content": []any{}},
			})
			<-r.Context().Done() // one event, then go silent
			return
		}
		t.Errorf("unexpected SSE request: output had already started, this should be a final error")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	model := codexModel(srv.URL)
	token := mockCodexToken(t, "")
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("Say hello"), Timestamp: time.Now().UnixMilli()}}}

	stream := Stream(context.Background(), model, chat, &ai.StreamOptions{
		APIKey:    token,
		Transport: ai.TransportAuto,
		Timeout:   50 * time.Millisecond,
	})
	result, err := stream.Result(context.Background())
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result.StopReason != ai.StopReasonError {
		t.Fatalf("stopReason = %q, want error", result.StopReason)
	}
	if !strings.Contains(result.ErrorMessage, "WebSocket idle timeout after 50ms") {
		t.Errorf("errorMessage = %q, want an idle-timeout message", result.ErrorMessage)
	}
}

// TestStream_FallbackMemoryPersistsForSession verifies the epic's second
// acceptance criterion directly: once a session has fallen back to SSE, a
// later request in that same session skips the WebSocket attempt entirely
// (ports isWebSocketSseFallbackActive's stickiness -- there is no dedicated
// upstream test for this, only the debug-stats fields it maintains, so this
// asserts the externally observable behavior instead: the WebSocket handler
// is never contacted a second time).
func TestStream_FallbackMemoryPersistsForSession(t *testing.T) {
	t.Cleanup(func() { resetCodexWebSocketState("") })

	var wsConnections int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isWebSocketUpgrade(r) {
			atomic.AddInt32(&wsConnections, 1)
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			defer func() { _ = conn.CloseNow() }()
			_ = readCodexWSRequest(t, conn)
			<-r.Context().Done() // stays silent -> idle timeout, triggering the fallback
			return
		}
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(codexSSEPayload("completed")))
	}))
	t.Cleanup(srv.Close)

	model := codexModel(srv.URL)
	token := mockCodexToken(t, "")
	chat := ai.Context{Messages: []ai.Message{&ai.UserMessage{Content: ai.UserText("Say hello"), Timestamp: time.Now().UnixMilli()}}}
	opts := &ai.StreamOptions{
		APIKey:    token,
		SessionID: "fallback-memory-session",
		Transport: ai.TransportAuto,
		Timeout:   50 * time.Millisecond,
	}

	first, err := Stream(context.Background(), model, chat, opts).Result(context.Background())
	if err != nil {
		t.Fatalf("first Result: %v", err)
	}
	if text, _ := first.Content[0].(ai.TextContent); text.Text != "Hello" {
		t.Fatalf("first call didn't fall back to SSE: %#v (errorMessage=%q)", first.Content, first.ErrorMessage)
	}
	if got := atomic.LoadInt32(&wsConnections); got != 1 {
		t.Fatalf("websocket connections after first call = %d, want 1", got)
	}

	second, err := Stream(context.Background(), model, chat, opts).Result(context.Background())
	if err != nil {
		t.Fatalf("second Result: %v", err)
	}
	if text, _ := second.Content[0].(ai.TextContent); text.Text != "Hello" {
		t.Fatalf("second call unexpected content: %#v (errorMessage=%q)", second.Content, second.ErrorMessage)
	}
	if got := atomic.LoadInt32(&wsConnections); got != 1 {
		t.Errorf("websocket connections after second call = %d, want still 1 (fallback memory should have skipped the websocket attempt)", got)
	}
}
