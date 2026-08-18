package filter

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/cannectors/runtime/internal/auth"
	"github.com/cannectors/runtime/internal/cache"
	"github.com/cannectors/runtime/internal/httpclient"
	"github.com/cannectors/runtime/internal/logger"
	"github.com/cannectors/runtime/internal/moduleconfig"
	"github.com/cannectors/runtime/internal/template"
	"github.com/cannectors/runtime/pkg/connector"
)

// normalizeHTTPCallMethod uppercases the configured method and validates that
// it is a well-formed RFC 7230 token. GET is used when empty.
func normalizeHTTPCallMethod(method string) (string, error) {
	return httpclient.NormalizeAndValidateMethod(method)
}

// buildHTTPCallAuth wraps auth.NewHandler with a module-specific error
// message.
func buildHTTPCallAuth(authCfg *connector.AuthConfig, client *http.Client) (auth.Handler, error) {
	if authCfg == nil {
		return nil, nil
	}
	h, err := auth.NewHandler(authCfg, client)
	if err != nil {
		return nil, fmt.Errorf("creating http_call auth handler: %w", err)
	}
	return h, nil
}

// buildHTTPCallCache creates the LRU cache used to memoize responses when
// the cache is enabled. Returns (nil, 0) when caching is disabled — the
// caller must guard cache.Get/Set with the returned `enabled` boolean.
// Story 24.11 AC4: cache.enabled must be honored.
func buildHTTPCallCache(cfg moduleconfig.CacheConfig) (cache.Cache, time.Duration, bool) {
	if !cfg.Enabled {
		return nil, 0, false
	}
	maxSize := cfg.MaxSize
	if maxSize <= 0 {
		maxSize = defaultCacheMaxSize
	}
	ttlSeconds := cfg.TTLSeconds
	if ttlSeconds <= 0 {
		ttlSeconds = defaultCacheTTLSeconds
	}
	ttl := time.Duration(ttlSeconds) * time.Second
	return cache.NewLRUCache(maxSize, ttl), ttl, true
}

// loadHTTPCallBodyTemplate loads a POST/PUT body template from disk and
// validates its syntax (compiling it for the JSON target). Returns ("", nil)
// when path is empty.
func loadHTTPCallBodyTemplate(engine *template.Engine, path string) (string, error) {
	if path == "" {
		return "", nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("loading http_call body template file %q: %w", path, err)
	}
	raw := string(content)
	if err := engine.Validate(raw, template.TargetJSON, false); err != nil {
		return "", fmt.Errorf("invalid template syntax in %q: %w", path, err)
	}
	logger.Debug("loaded http_call body template file",
		slog.String("file", path),
		slog.Int("size", len(content)),
	)
	return raw, nil
}

// validateHTTPCallTemplates compiles the endpoint and header templates for
// their targets up-front, surfacing syntax errors at construction time.
func validateHTTPCallTemplates(engine *template.Engine, endpoint string, headers map[string]string) error {
	if err := engine.Validate(endpoint, template.TargetURL, false); err != nil {
		return fmt.Errorf("invalid template syntax in endpoint: %w", err)
	}
	for name, v := range headers {
		if err := engine.Validate(v, template.TargetText, false); err != nil {
			return fmt.Errorf("invalid template syntax in header %q: %w", name, err)
		}
	}
	return nil
}

// httpCallHasTemplating returns true when the module configuration contains
// any {{ }} template variables worth evaluating per-record.
func httpCallHasTemplating(endpoint string, headers map[string]string, hasBodyTemplate bool) bool {
	if template.HasVariables(endpoint) || hasBodyTemplate {
		return true
	}
	for _, v := range headers {
		if template.HasVariables(v) {
			return true
		}
	}
	return false
}

// validateKeysConfig validates the keys extraction configuration, requiring
// at least one entry. Used when the call has no body — the keys are then the
// only way to parameterize the request per record.
func validateKeysConfig(keys []moduleconfig.KeyConfig) error {
	if len(keys) == 0 {
		return newHTTPCallError(ErrCodeHTTPCallKeyMissing, "http_call keys is required (at least one key config)", -1, "", 0, "")
	}
	return validateKeyEntries(keys)
}

// validateKeyEntries validates each key entry independently of whether the
// list is required. Story 24.11 AC11 requires invalid keys to be rejected at
// construction even when a body is configured.
func validateKeyEntries(keys []moduleconfig.KeyConfig) error {
	for i, key := range keys {
		if key.Field == "" {
			return newHTTPCallError(ErrCodeHTTPCallKeyMissing, fmt.Sprintf("http_call keys[%d] field is required", i), -1, "", 0, "")
		}
		if key.ParamType == "" {
			return newHTTPCallError(ErrCodeHTTPCallKeyMissing, fmt.Sprintf("http_call keys[%d] paramType is required", i), -1, "", 0, "")
		}
		if key.ParamType != "query" && key.ParamType != "path" && key.ParamType != "header" {
			return newHTTPCallError(ErrCodeHTTPCallKeyInvalid, "http_call key paramType must be 'query', 'path', or 'header'", -1, "", 0, "")
		}
		if key.ParamName == "" {
			return newHTTPCallError(ErrCodeHTTPCallKeyMissing, fmt.Sprintf("http_call keys[%d] paramName is required", i), -1, "", 0, "")
		}
	}
	return nil
}
