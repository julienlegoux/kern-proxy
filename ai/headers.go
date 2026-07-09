package ai

import "net/http"

// Ports: packages/ai/src/utils/headers.ts

// HeadersToRecord flattens an http.Header into a simple map (first value per
// key), mirroring the TS Headers-to-record conversion used for OnResponse.
func HeadersToRecord(headers http.Header) map[string]string {
	result := make(map[string]string, len(headers))
	for key := range headers {
		result[http.CanonicalHeaderKey(key)] = headers.Get(key)
	}
	return result
}

// ProviderHeadersToRecord drops nil (suppressed) values and returns nil when
// nothing remains.
func ProviderHeadersToRecord(headers ProviderHeaders) map[string]string {
	if headers == nil {
		return nil
	}
	result := make(map[string]string, len(headers))
	for key, value := range headers {
		if value != nil {
			result[key] = *value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// MergeProviderHeaders overlays override headers onto defaults: override
// values win per key, and a nil override value suppresses the default. The
// result is a plain header map ready for a request.
func MergeProviderHeaders(defaults map[string]string, overrides ProviderHeaders) map[string]string {
	merged := make(map[string]string, len(defaults)+len(overrides))
	for k, v := range defaults {
		merged[k] = v
	}
	for k, v := range overrides {
		if v == nil {
			delete(merged, k)
		} else {
			merged[k] = *v
		}
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}
