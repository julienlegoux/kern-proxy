package oauth

// Ports: packages/ai/src/utils/oauth/anthropic.ts (postJson — shared HTTP
// JSON POST helper reused by every flow's token exchange/refresh).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// postJSONTimeout matches upstream's AbortSignal.timeout(30_000) on every
// token-endpoint request.
const postJSONTimeout = 30 * time.Second

// httpClient is package-level so tests can point it at an httptest server;
// production code never needs to override it.
var httpClient = http.DefaultClient

// clock is stubbed in tests to control expires computation, mirroring
// ai.authClock's package-level-var pattern (ai/resolve.go).
var clock = func() int64 { return time.Now().UnixMilli() }

// PostJSON POSTs body as JSON to url and returns the raw response body,
// erroring on a non-2xx status. Matches upstream's postJson, whose thrown
// Error bakes the status/url/body into its message.
func PostJSON(ctx context.Context, url string, body map[string]any) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("oauth: encode request body: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, postJSONTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("oauth: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth: http request failed. url=%s: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("oauth: read response body. url=%s: %w", url, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("oauth: http request failed. status=%d; url=%s; body=%s", resp.StatusCode, url, respBody)
	}

	return respBody, nil
}
