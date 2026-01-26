// Package filter provides implementations for filter modules.
// Enrichment module fetches additional data from external APIs and merges it into records.
package filter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/canectors/runtime/internal/auth"
	"github.com/canectors/runtime/internal/cache"
	"github.com/canectors/runtime/internal/errhandling"
	"github.com/canectors/runtime/internal/logger"
	"github.com/canectors/runtime/pkg/connector"
)

// Default configuration values for enrichment module
const (
	defaultEnrichmentTimeout  = 30 * time.Second
	defaultCacheMaxSize       = 1000
	defaultCacheTTLSeconds    = 300 // 5 minutes
	defaultEnrichmentStrategy = "merge"
)

// Error codes for enrichment module
const (
	ErrCodeEnrichmentEndpointMissing = "ENRICHMENT_ENDPOINT_MISSING"
	ErrCodeEnrichmentKeyMissing      = "ENRICHMENT_KEY_MISSING"
	ErrCodeEnrichmentKeyExtract      = "ENRICHMENT_KEY_EXTRACT"
	ErrCodeEnrichmentHTTPError       = "ENRICHMENT_HTTP_ERROR"
	ErrCodeEnrichmentJSONParse       = "ENRICHMENT_JSON_PARSE"
	ErrCodeEnrichmentMerge           = "ENRICHMENT_MERGE"
)

// Error types for enrichment module
var (
	ErrEnrichmentEndpointMissing = fmt.Errorf("enrichment endpoint is required")
	ErrEnrichmentKeyMissing      = fmt.Errorf("enrichment key configuration is required")
	ErrEnrichmentKeyInvalid      = fmt.Errorf("enrichment key paramType must be 'query', 'path', or 'header'")
)

// KeyConfig defines how to extract a key from a record and use it in HTTP requests.
type KeyConfig struct {
	// Field is the dot-notation path to extract the key value from the record (e.g., "customer.id")
	Field string `json:"field"`
	// ParamType specifies how to include the key in the request: "query", "path", or "header"
	ParamType string `json:"paramType"`
	// ParamName is the parameter name to use in the request
	ParamName string `json:"paramName"`
}

// CacheConfig defines cache behavior for the enrichment module.
type CacheConfig struct {
	// MaxSize is the maximum number of entries in the cache (default 1000)
	MaxSize int `json:"maxSize"`
	// DefaultTTL is the TTL for cache entries in seconds (default 300)
	DefaultTTL int `json:"defaultTTL"`
}

// EnrichmentConfig represents the configuration for an enrichment filter module.
type EnrichmentConfig struct {
	// Endpoint is the HTTP endpoint URL (required). May contain {key} placeholder for path params.
	Endpoint string `json:"endpoint"`
	// Key defines how to extract and use the key value (required)
	Key KeyConfig `json:"key"`
	// Auth is the optional authentication configuration
	Auth *connector.AuthConfig `json:"auth,omitempty"`
	// Cache defines cache behavior (optional, uses defaults if not specified)
	Cache CacheConfig `json:"cache"`
	// MergeStrategy defines how to merge enrichment data: "merge" (default), "replace", "append"
	MergeStrategy string `json:"mergeStrategy"`
	// DataField is the JSON field containing the data in the response (optional)
	DataField string `json:"dataField"`
	// OnError specifies error handling mode: "fail" (default), "skip", "log"
	OnError string `json:"onError"`
	// TimeoutMs is the request timeout in milliseconds (default 30000)
	TimeoutMs int `json:"timeoutMs"`
	// Headers are custom HTTP headers to include in requests
	Headers map[string]string `json:"headers"`
}

// EnrichmentModule implements a filter that enriches records with external API data.
// It supports caching to avoid redundant API calls for records with the same key value.
//
// Thread Safety:
//   - The cache is thread-safe (uses mutex internally)
//   - Process() can be called from multiple goroutines
//
// Error Handling:
//   - HTTP errors are not cached (only successful responses are cached)
//   - onError mode controls behavior: fail (stop pipeline), skip (drop record), log (continue)
type EnrichmentModule struct {
	endpoint      string
	keyField      string
	keyParamType  string
	keyParamName  string
	authHandler   auth.Handler
	httpClient    *http.Client
	cache         cache.Cache
	mergeStrategy string
	dataField     string
	onError       string
	headers       map[string]string
	cacheTTL      time.Duration
}

// EnrichmentError carries structured context for enrichment failures.
type EnrichmentError struct {
	Code        string
	Message     string
	RecordIndex int
	Endpoint    string
	StatusCode  int
	KeyValue    string
	Details     map[string]interface{}
}

func (e *EnrichmentError) Error() string {
	return e.Message
}

// sanitizeURL removes sensitive information from URLs for error messages.
// Masks query parameters and fragments to prevent exposing credentials or tokens.
func sanitizeURL(urlStr string) string {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		// If parsing fails, return a safe placeholder
		return "[invalid URL]"
	}
	// Remove query parameters and fragments
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

// newEnrichmentError creates an EnrichmentError with context.
func newEnrichmentError(code, message string, recordIdx int, endpoint string, statusCode int, keyValue string) *EnrichmentError {
	// Sanitize endpoint URL in error message to avoid exposing sensitive data
	sanitizedEndpoint := sanitizeURL(endpoint)
	return &EnrichmentError{
		Code:        code,
		Message:     message,
		RecordIndex: recordIdx,
		Endpoint:    sanitizedEndpoint,
		StatusCode:  statusCode,
		KeyValue:    keyValue,
		Details:     make(map[string]interface{}),
	}
}

// NewEnrichmentFromConfig creates a new enrichment filter module from configuration.
// It validates the configuration and initializes the HTTP client and cache.
//
// Required config fields:
//   - endpoint: The HTTP endpoint URL
//   - key: Key extraction configuration (field, paramType, paramName)
//
// Optional config fields:
//   - auth: Authentication configuration
//   - cache: Cache configuration (maxSize, defaultTTL)
//   - mergeStrategy: How to merge data ("merge", "replace", "append")
//   - dataField: JSON field containing the data array
//   - onError: Error handling mode ("fail", "skip", "log")
//   - timeoutMs: Request timeout in milliseconds
//   - headers: Custom HTTP headers
func NewEnrichmentFromConfig(config EnrichmentConfig) (*EnrichmentModule, error) {
	// Validate endpoint
	if config.Endpoint == "" {
		return nil, newEnrichmentError(ErrCodeEnrichmentEndpointMissing, "enrichment endpoint is required", -1, "", 0, "")
	}

	// Validate key configuration
	if err := validateKeyConfig(config.Key); err != nil {
		return nil, err
	}

	// Normalize merge strategy
	mergeStrategy := config.MergeStrategy
	if mergeStrategy == "" {
		mergeStrategy = defaultEnrichmentStrategy
	}
	if mergeStrategy != "merge" && mergeStrategy != "replace" && mergeStrategy != "append" {
		logger.Warn("invalid mergeStrategy for enrichment module; defaulting to merge",
			slog.String("merge_strategy", mergeStrategy),
		)
		mergeStrategy = defaultEnrichmentStrategy
	}

	// Normalize onError
	onError := config.OnError
	if onError == "" {
		onError = OnErrorFail
	}
	if onError != OnErrorFail && onError != OnErrorSkip && onError != OnErrorLog {
		logger.Warn("invalid onError value for enrichment module; defaulting to fail",
			slog.String("on_error", onError),
		)
		onError = OnErrorFail
	}

	// Set timeout
	timeout := defaultEnrichmentTimeout
	if config.TimeoutMs > 0 {
		timeout = time.Duration(config.TimeoutMs) * time.Millisecond
	}

	// Create HTTP client
	httpClient := &http.Client{
		Timeout: timeout,
	}

	// Create auth handler if configured
	var authHandler auth.Handler
	if config.Auth != nil {
		var err error
		authHandler, err = auth.NewHandler(config.Auth, httpClient)
		if err != nil {
			return nil, fmt.Errorf("creating enrichment auth handler: %w", err)
		}
	}

	// Set cache configuration
	cacheMaxSize := config.Cache.MaxSize
	if cacheMaxSize <= 0 {
		cacheMaxSize = defaultCacheMaxSize
	}
	cacheTTLSeconds := config.Cache.DefaultTTL
	if cacheTTLSeconds <= 0 {
		cacheTTLSeconds = defaultCacheTTLSeconds
	}
	cacheTTL := time.Duration(cacheTTLSeconds) * time.Second

	// Create cache
	lruCache := cache.NewLRUCache(cacheMaxSize, cacheTTL)

	logger.Debug("enrichment module initialized",
		slog.String("endpoint", config.Endpoint),
		slog.String("key_field", config.Key.Field),
		slog.String("key_param_type", config.Key.ParamType),
		slog.String("merge_strategy", mergeStrategy),
		slog.String("on_error", onError),
		slog.Int("cache_max_size", cacheMaxSize),
		slog.Int("cache_ttl_seconds", cacheTTLSeconds),
		slog.Bool("has_auth", authHandler != nil),
	)

	return &EnrichmentModule{
		endpoint:      config.Endpoint,
		keyField:      config.Key.Field,
		keyParamType:  config.Key.ParamType,
		keyParamName:  config.Key.ParamName,
		authHandler:   authHandler,
		httpClient:    httpClient,
		cache:         lruCache,
		mergeStrategy: mergeStrategy,
		dataField:     config.DataField,
		onError:       onError,
		headers:       config.Headers,
		cacheTTL:      cacheTTL,
	}, nil
}

// validateKeyConfig validates the key extraction configuration.
func validateKeyConfig(key KeyConfig) error {
	if key.Field == "" {
		return newEnrichmentError(ErrCodeEnrichmentKeyMissing, "enrichment key field is required", -1, "", 0, "")
	}
	if key.ParamType == "" {
		return newEnrichmentError(ErrCodeEnrichmentKeyMissing, "enrichment key paramType is required", -1, "", 0, "")
	}
	if key.ParamType != "query" && key.ParamType != "path" && key.ParamType != "header" {
		return newEnrichmentError(ErrCodeEnrichmentKeyInvalid, "enrichment key paramType must be 'query', 'path', or 'header'", -1, "", 0, "")
	}
	if key.ParamName == "" {
		return newEnrichmentError(ErrCodeEnrichmentKeyMissing, "enrichment key paramName is required", -1, "", 0, "")
	}
	return nil
}

// ParseEnrichmentConfig parses an enrichment filter configuration from raw config map.
func ParseEnrichmentConfig(cfg map[string]interface{}) (EnrichmentConfig, error) {
	config := EnrichmentConfig{}

	// Parse endpoint (required)
	if endpoint, ok := cfg["endpoint"].(string); ok {
		config.Endpoint = endpoint
	}

	// Parse key configuration (required)
	if keyRaw, ok := cfg["key"].(map[string]interface{}); ok {
		if field, ok := keyRaw["field"].(string); ok {
			config.Key.Field = field
		}
		if paramType, ok := keyRaw["paramType"].(string); ok {
			config.Key.ParamType = paramType
		}
		if paramName, ok := keyRaw["paramName"].(string); ok {
			config.Key.ParamName = paramName
		}
	}

	// Parse auth configuration (optional)
	if authRaw, ok := cfg["auth"].(map[string]interface{}); ok {
		config.Auth = parseAuthConfig(authRaw)
	}

	// Parse cache configuration (optional)
	if cacheRaw, ok := cfg["cache"].(map[string]interface{}); ok {
		if maxSize, ok := cacheRaw["maxSize"].(float64); ok {
			maxSizeInt := int(maxSize)
			if maxSizeInt > 0 {
				config.Cache.MaxSize = maxSizeInt
			}
		}
		if ttl, ok := cacheRaw["defaultTTL"].(float64); ok {
			ttlInt := int(ttl)
			if ttlInt > 0 {
				config.Cache.DefaultTTL = ttlInt
			}
		}
	}

	// Parse other optional fields
	if mergeStrategy, ok := cfg["mergeStrategy"].(string); ok {
		config.MergeStrategy = mergeStrategy
	}
	if dataField, ok := cfg["dataField"].(string); ok {
		config.DataField = dataField
	}
	if onError, ok := cfg["onError"].(string); ok {
		config.OnError = onError
	}
	if timeoutMs, ok := cfg["timeoutMs"].(float64); ok {
		config.TimeoutMs = int(timeoutMs)
	}

	// Parse headers
	if headersRaw, ok := cfg["headers"].(map[string]interface{}); ok {
		config.Headers = make(map[string]string)
		for k, v := range headersRaw {
			if strVal, ok := v.(string); ok {
				config.Headers[k] = strVal
			}
		}
	}

	return config, nil
}

// parseAuthConfig converts a raw map to AuthConfig.
func parseAuthConfig(raw map[string]interface{}) *connector.AuthConfig {
	authConfig := &connector.AuthConfig{
		Credentials: make(map[string]string),
	}

	if authType, ok := raw["type"].(string); ok {
		authConfig.Type = authType
	}

	// Extract credentials from nested object or flat structure
	if creds, ok := raw["credentials"].(map[string]interface{}); ok {
		for k, v := range creds {
			if strVal, ok := v.(string); ok {
				authConfig.Credentials[k] = strVal
			}
		}
	} else {
		// Flat structure: all non-type fields are credentials
		for k, v := range raw {
			if k != "type" {
				if strVal, ok := v.(string); ok {
					authConfig.Credentials[k] = strVal
				}
			}
		}
	}

	return authConfig
}

// Process enriches each input record with data from an external API.
// For each record:
//  1. Extracts the key value from the record
//  2. Checks the cache for existing data
//  3. If cache miss, makes an HTTP request to fetch the data
//  4. Merges the fetched data into the record
//  5. Caches successful responses
//
// The context can be used to cancel long-running operations.
func (m *EnrichmentModule) Process(ctx context.Context, records []map[string]interface{}) ([]map[string]interface{}, error) {
	if records == nil {
		return []map[string]interface{}{}, nil
	}

	startTime := time.Now()
	inputCount := len(records)

	logger.Debug("filter processing started",
		slog.String("module_type", "enrichment"),
		slog.Int("input_records", inputCount),
		slog.String("on_error", m.onError),
	)

	result := make([]map[string]interface{}, 0, len(records))
	skippedCount := 0
	errorCount := 0
	cacheHits := 0
	cacheMisses := 0

	for recordIdx, record := range records {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		enrichedRecord, wasCacheHit, err := m.processRecord(ctx, record, recordIdx)
		if wasCacheHit {
			cacheHits++
		} else if err == nil {
			cacheMisses++
		}

		if err != nil {
			errorCount++
			switch m.onError {
			case OnErrorFail:
				duration := time.Since(startTime)
				logger.Error("filter processing failed",
					slog.String("module_type", "enrichment"),
					slog.Int("record_index", recordIdx),
					slog.Duration("duration", duration),
					slog.String("error", err.Error()),
				)
				return nil, err
			case OnErrorSkip:
				skippedCount++
				logger.Warn("skipping record due to enrichment error",
					slog.String("module_type", "enrichment"),
					slog.Int("record_index", recordIdx),
					slog.String("error", err.Error()),
				)
				continue
			case OnErrorLog:
				logger.Error("enrichment error (continuing)",
					slog.String("module_type", "enrichment"),
					slog.Int("record_index", recordIdx),
					slog.String("error", err.Error()),
				)
				// For log mode, add the original record (not enriched)
				result = append(result, record)
				continue
			}
		}
		result = append(result, enrichedRecord)
	}

	duration := time.Since(startTime)
	outputCount := len(result)

	logger.Info("filter processing completed",
		slog.String("module_type", "enrichment"),
		slog.Int("input_records", inputCount),
		slog.Int("output_records", outputCount),
		slog.Int("skipped_records", skippedCount),
		slog.Int("error_count", errorCount),
		slog.Int("cache_hits", cacheHits),
		slog.Int("cache_misses", cacheMisses),
		slog.Duration("duration", duration),
	)

	return result, nil
}

// processRecord enriches a single record with data from the external API.
// Returns the enriched record, whether it was a cache hit, and any error.
func (m *EnrichmentModule) processRecord(ctx context.Context, record map[string]interface{}, recordIdx int) (map[string]interface{}, bool, error) {
	// Extract key value from record
	keyValue, err := m.extractKeyValue(record, recordIdx)
	if err != nil {
		return nil, false, err
	}

	// Check cache first
	cacheKey := m.buildCacheKey(keyValue)
	if cachedData, found := m.cache.Get(cacheKey); found {
		enrichmentData, ok := cachedData.(map[string]interface{})
		if ok {
			logger.Debug("enrichment cache hit",
				slog.String("module_type", "enrichment"),
				slog.Int("record_index", recordIdx),
				slog.String("key_value", keyValue),
			)
			enrichedRecord := m.mergeData(record, enrichmentData)
			return enrichedRecord, true, nil
		}
	}

	// Cache miss - make HTTP request
	enrichmentData, err := m.fetchEnrichmentData(ctx, keyValue, recordIdx)
	if err != nil {
		return nil, false, err
	}

	// Cache successful response (don't cache errors)
	m.cache.Set(cacheKey, enrichmentData, m.cacheTTL)

	logger.Debug("enrichment cache miss (fetched and cached)",
		slog.String("module_type", "enrichment"),
		slog.Int("record_index", recordIdx),
		slog.String("key_value", keyValue),
	)

	// Merge data into record
	enrichedRecord := m.mergeData(record, enrichmentData)

	return enrichedRecord, false, nil
}

// extractKeyValue extracts the key value from a record using the configured field path.
func (m *EnrichmentModule) extractKeyValue(record map[string]interface{}, recordIdx int) (string, error) {
	value, found := getNestedValue(record, m.keyField)
	if !found {
		return "", newEnrichmentError(
			ErrCodeEnrichmentKeyExtract,
			fmt.Sprintf("enrichment failed to extract key from record %d: field '%s' not found", recordIdx, m.keyField),
			recordIdx, m.endpoint, 0, "",
		)
	}

	// Convert value to string
	var keyValue string
	switch v := value.(type) {
	case string:
		keyValue = v
	case float64:
		// Handle JSON numbers
		if v == float64(int64(v)) {
			keyValue = fmt.Sprintf("%d", int64(v))
		} else {
			keyValue = fmt.Sprintf("%v", v)
		}
	case int, int64, int32:
		keyValue = fmt.Sprintf("%v", v)
	case nil:
		return "", newEnrichmentError(
			ErrCodeEnrichmentKeyExtract,
			fmt.Sprintf("enrichment failed to extract key from record %d: field '%s' is null", recordIdx, m.keyField),
			recordIdx, m.endpoint, 0, "",
		)
	default:
		keyValue = fmt.Sprintf("%v", v)
	}

	if keyValue == "" {
		return "", newEnrichmentError(
			ErrCodeEnrichmentKeyExtract,
			fmt.Sprintf("enrichment failed to extract key from record %d: field '%s' is empty", recordIdx, m.keyField),
			recordIdx, m.endpoint, 0, "",
		)
	}

	return keyValue, nil
}

// buildCacheKey creates a unique cache key from the key value.
func (m *EnrichmentModule) buildCacheKey(keyValue string) string {
	// Include endpoint in cache key to isolate different enrichment modules
	return m.endpoint + ":" + keyValue
}

// fetchEnrichmentData fetches data from the external API for the given key value.
func (m *EnrichmentModule) fetchEnrichmentData(ctx context.Context, keyValue string, recordIdx int) (map[string]interface{}, error) {
	// Build request URL
	requestURL, err := m.buildRequestURL(keyValue)
	if err != nil {
		return nil, newEnrichmentError(
			ErrCodeEnrichmentHTTPError,
			fmt.Sprintf("enrichment failed to build request URL: %v", err),
			recordIdx, m.endpoint, 0, keyValue,
		)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, newEnrichmentError(
			ErrCodeEnrichmentHTTPError,
			fmt.Sprintf("enrichment failed to create request: %v", err),
			recordIdx, m.endpoint, 0, keyValue,
		)
	}

	// Set headers
	req.Header.Set("User-Agent", "Canectors-Runtime/1.0")
	req.Header.Set("Accept", "application/json")
	for key, value := range m.headers {
		req.Header.Set(key, value)
	}

	// Apply key as header if configured
	if m.keyParamType == "header" {
		req.Header.Set(m.keyParamName, keyValue)
	}

	// Apply authentication
	if m.authHandler != nil {
		if authErr := m.authHandler.ApplyAuth(ctx, req); authErr != nil {
			return nil, newEnrichmentError(
				ErrCodeEnrichmentHTTPError,
				fmt.Sprintf("enrichment failed to apply auth: %v", authErr),
				recordIdx, m.endpoint, 0, keyValue,
			)
		}
	}

	// Execute request
	resp, err := m.httpClient.Do(req)
	if err != nil {
		classifiedErr := errhandling.ClassifyNetworkError(err)
		return nil, newEnrichmentError(
			ErrCodeEnrichmentHTTPError,
			fmt.Sprintf("enrichment HTTP request failed: %v", classifiedErr),
			recordIdx, m.endpoint, 0, keyValue,
		)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Warn("failed to close enrichment response body",
				slog.String("error", closeErr.Error()),
			)
		}
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, newEnrichmentError(
			ErrCodeEnrichmentHTTPError,
			fmt.Sprintf("enrichment failed to read response: %v", err),
			recordIdx, m.endpoint, resp.StatusCode, keyValue,
		)
	}

	// Handle HTTP errors
	if resp.StatusCode >= 400 {
		bodySnippet := string(body)
		if len(bodySnippet) > 200 {
			bodySnippet = bodySnippet[:200] + "..."
		}
		return nil, newEnrichmentError(
			ErrCodeEnrichmentHTTPError,
			fmt.Sprintf("enrichment HTTP error %d: %s", resp.StatusCode, bodySnippet),
			recordIdx, m.endpoint, resp.StatusCode, keyValue,
		)
	}

	// Parse JSON response
	var responseData map[string]interface{}
	if err := json.Unmarshal(body, &responseData); err != nil {
		return nil, newEnrichmentError(
			ErrCodeEnrichmentJSONParse,
			fmt.Sprintf("enrichment failed to parse response: %v", err),
			recordIdx, m.endpoint, resp.StatusCode, keyValue,
		)
	}

	// Extract data field if configured
	if m.dataField != "" {
		if data, ok := responseData[m.dataField]; ok {
			if dataMap, ok := data.(map[string]interface{}); ok {
				return dataMap, nil
			}
			// If dataField points to an array with single element, use that
			if dataArr, ok := data.([]interface{}); ok && len(dataArr) == 1 {
				if dataMap, ok := dataArr[0].(map[string]interface{}); ok {
					return dataMap, nil
				}
			}
		}
		// dataField not found or not an object - return empty
		logger.Warn("enrichment dataField not found or invalid",
			slog.String("data_field", m.dataField),
			slog.Int("record_index", recordIdx),
		)
		return make(map[string]interface{}), nil
	}

	return responseData, nil
}

// buildRequestURL constructs the HTTP request URL based on the key configuration.
func (m *EnrichmentModule) buildRequestURL(keyValue string) (string, error) {
	switch m.keyParamType {
	case "path":
		// Replace {paramName} in endpoint with key value
		placeholder := "{" + m.keyParamName + "}"
		requestURL := strings.Replace(m.endpoint, placeholder, url.PathEscape(keyValue), 1)
		return requestURL, nil

	case "query":
		// Add key as query parameter
		parsedURL, err := url.Parse(m.endpoint)
		if err != nil {
			return "", fmt.Errorf("parsing endpoint URL: %w", err)
		}
		q := parsedURL.Query()
		q.Set(m.keyParamName, keyValue)
		parsedURL.RawQuery = q.Encode()
		return parsedURL.String(), nil

	case "header":
		// Header is added in fetchEnrichmentData, return endpoint as-is
		return m.endpoint, nil

	default:
		return "", fmt.Errorf("unknown key paramType: %s", m.keyParamType)
	}
}

// mergeData merges enrichment data into the record based on the merge strategy.
func (m *EnrichmentModule) mergeData(record, enrichmentData map[string]interface{}) map[string]interface{} {
	switch m.mergeStrategy {
	case "merge":
		return m.deepMerge(record, enrichmentData)

	case "replace":
		// Replace the entire record with enrichment data
		// Keep original fields that aren't in enrichment data
		result := make(map[string]interface{})
		for k, v := range record {
			result[k] = v
		}
		for k, v := range enrichmentData {
			result[k] = v
		}
		return result

	case "append":
		// Append enrichment data under a special key
		result := make(map[string]interface{})
		for k, v := range record {
			result[k] = v
		}
		result["_enrichment"] = enrichmentData
		return result

	default:
		return m.deepMerge(record, enrichmentData)
	}
}

// deepMerge performs a deep merge of two maps.
// Values from b override values from a, except for nested maps which are merged recursively.
func (m *EnrichmentModule) deepMerge(a, b map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy all from a
	for k, v := range a {
		result[k] = v
	}

	// Merge/override from b
	for k, vb := range b {
		if va, exists := result[k]; exists {
			// If both are maps, merge recursively
			if mapA, okA := va.(map[string]interface{}); okA {
				if mapB, okB := vb.(map[string]interface{}); okB {
					result[k] = m.deepMerge(mapA, mapB)
					continue
				}
			}
		}
		// Otherwise, b overrides
		result[k] = vb
	}

	return result
}

// GetCacheStats returns the current cache statistics.
func (m *EnrichmentModule) GetCacheStats() cache.Stats {
	return m.cache.Stats()
}

// ClearCache clears all entries from the cache.
func (m *EnrichmentModule) ClearCache() {
	m.cache.Clear()
}
