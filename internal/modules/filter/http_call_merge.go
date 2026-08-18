package filter

import (
	"errors"

	"github.com/cannectors/runtime/internal/cache"
	"github.com/cannectors/runtime/internal/errhandling"
)

// mergeData folds the response into the record through the contract shared
// with soap_call and sql_call. A list response only fits the append strategy,
// so the mismatch surfaces as an http_call error that onError arbitrates
// instead of an empty enrichment (Story 25.3).
func (m *HTTPCallModule) mergeData(record map[string]any, responseData any, recordIdx int, keyValues map[string]string) (map[string]any, error) {
	merged, err := mergeCallResult("http_call", m.mergeStrategy, m.resultKey, record, responseData)
	if err != nil {
		// The contract error is kept as the cause so a caller can route on the
		// shared sentinels (errors.Is) as well as on the http_call code. Its own
		// message is reused verbatim rather than err.Error(), which would prefix
		// the classification a second time.
		message := err.Error()
		var contractErr *errhandling.ClassifiedError
		if errors.As(err, &contractErr) {
			message = contractErr.Message
		}
		return nil, newHTTPCallError(
			ErrCodeHTTPCallMergeMismatch,
			message,
			recordIdx, m.endpoint, 0, m.compositeKeyString(keyValues),
		).withCause(err, 0)
	}
	return merged, nil
}

// GetCacheStats returns the current cache statistics. When the cache is
// disabled it returns the zero-valued Stats.
func (m *HTTPCallModule) GetCacheStats() cache.Stats {
	if !m.cacheEnabled {
		return cache.Stats{}
	}
	return m.cache.Stats()
}

// ClearCache clears all entries from the cache. No-op when caching is disabled.
func (m *HTTPCallModule) ClearCache() {
	if !m.cacheEnabled {
		return
	}
	m.cache.Clear()
}
