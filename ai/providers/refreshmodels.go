package providers

// Ports: none — original: shared fetchJSON plumbing for the dynamic-provider
// RefreshModels hooks (no upstream provider file wires a `refreshModels` hook).

// Native (not upstream-ported: no upstream provider file wires a
// `refreshModels` hook — see docs/PLAN.md's phase-11 note and
// docs/epics/epic-11-catalog-all-providers/issues/03-vendor-bindings-refreshmodels.md).
// fetchJSON is the shared plumbing behind the four dynamic-provider
// RefreshModels functions (OpenRouter, Vercel AI Gateway, NVIDIA, GitHub
// Copilot): a plain GET decoded as JSON, with package-level URL variables so
// tests can point every one of them at an httptest server instead of the
// network.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// fetchJSON issues an HTTP GET against url with the given headers and
// decodes the JSON response body into out. A non-2xx response is reported
// as an error carrying the status and response body.
func fetchJSON(ctx context.Context, url string, headers map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("providers: GET %s: %s: %s", url, resp.Status, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(out)
}
