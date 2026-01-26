# Story 14.4: Dynamic Enrichment Inside Filters (Input Inside Filter + Cache)

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want to enrich records dynamically within filter modules by executing HTTP requests to external APIs,
so that I can add additional data to records based on their current values, with configurable caching to limit redundant API calls.

## Acceptance Criteria

1. **Given** I have a filter module with enrichment configuration
   **When** I configure the enrichment with an HTTP endpoint and cache settings
   **Then** The runtime creates an enrichment filter module instance
   **And** The enrichment can fetch data from external APIs using HTTP GET requests
   **And** The enrichment supports authentication (API key, Bearer, Basic, OAuth2) like input modules
   **And** The enrichment supports configurable caching to avoid redundant requests

2. **Given** I have an enrichment filter module with a valid configuration
   **When** The runtime processes records through the filter
   **Then** For each record, the enrichment extracts a key value from the record
   **And** The enrichment checks the cache for existing data using the key
   **And** If cache miss, the enrichment makes an HTTP request to the configured endpoint
   **And** The HTTP request includes the key value as a parameter (query param, path param, or header)
   **And** The enrichment stores the response in the cache with the key
   **And** The enrichment merges the fetched data into the record
   **And** If cache hit, the enrichment uses cached data without making HTTP request

3. **Given** I have an enrichment filter with cache configuration
   **When** The runtime processes multiple records with the same key value
   **Then** Only the first record triggers an HTTP request
   **And** Subsequent records with the same key use cached data
   **And** Cache entries expire according to TTL configuration
   **And** Cache size is limited according to maxSize configuration
   **And** Cache eviction follows LRU (Least Recently Used) policy when size limit is reached

4. **Given** I have an enrichment filter that fails to fetch data
   **When** The HTTP request fails or returns an error
   **Then** The error handling follows the onError configuration (fail, skip, log)
   **And** Failed requests are not cached
   **And** Clear error messages include HTTP status codes and endpoint information
   **And** The record processing continues or stops according to onError mode

5. **Given** I have an enrichment filter module in my pipeline
   **When** The pipeline executes
   **Then** The enrichment filter processes records between input and output modules
   **And** The enrichment filter can be combined with other filter modules (mapping, condition, script)
   **And** The execution is deterministic and repeatable
   **And** Cache is scoped per filter module instance (not shared across instances)

## Tasks / Subtasks

- [x] Task 1: Design enrichment filter module structure (AC: #1, #5)
  - [x] Create `internal/modules/filter/enrichment.go` file
  - [x] Define EnrichmentModule struct implementing filter.Module interface
  - [x] Add HTTP client, cache, and configuration fields
  - [x] Implement Process() method signature
  - [x] Design cache interface for pluggability

- [x] Task 2: Implement cache package (AC: #3)
  - [x] Create `internal/cache/` package
  - [x] Implement LRU cache with TTL support
  - [x] Add thread-safe operations (mutex protection)
  - [x] Implement cache eviction when size limit reached
  - [x] Add cache statistics (hits, misses, evictions)
  - [x] Support configurable maxSize and defaultTTL

- [x] Task 3: Implement enrichment module configuration parsing (AC: #1, #4)
  - [x] Create EnrichmentConfig type with endpoint, key extraction, cache settings
  - [x] Add ParseEnrichmentConfig function to extract config from ModuleConfig
  - [x] Validate endpoint URL is required and valid
  - [x] Validate key extraction configuration (field path, parameter type)
  - [x] Validate cache configuration (maxSize, defaultTTL)
  - [x] Validate authentication configuration (optional, same as input modules)
  - [x] Validate onError mode (fail, skip, log)

- [x] Task 4: Implement HTTP request execution (AC: #2)
  - [x] Create HTTP client with timeout configuration
  - [x] Implement key value extraction from record (field path support)
  - [x] Build HTTP request URL with key as parameter (query, path, or header)
  - [x] Apply authentication (reuse auth.Handler from input modules)
  - [x] Execute HTTP GET request
  - [x] Parse JSON response
  - [x] Handle HTTP errors (4xx, 5xx) appropriately

- [x] Task 5: Implement cache integration (AC: #2, #3)
  - [x] Check cache before making HTTP request
  - [x] Store successful responses in cache with key
  - [x] Respect TTL for cache entries
  - [x] Implement cache key generation from record key value
  - [x] Handle cache misses (make HTTP request)
  - [x] Handle cache hits (use cached data, skip HTTP request)

- [x] Task 6: Implement data merging (AC: #2)
  - [x] Extract data from HTTP response (support dataField like input modules)
  - [x] Merge enrichment data into record (deep merge or field mapping)
  - [x] Support merge strategy configuration (merge, replace, append)
  - [x] Handle nested objects and arrays correctly
  - [x] Preserve original record fields unless explicitly replaced

- [x] Task 7: Implement error handling (AC: #4)
  - [x] Catch HTTP request errors
  - [x] Catch JSON parsing errors
  - [x] Respect onError configuration (fail, skip, log)
  - [x] Log enrichment errors with record index and endpoint
  - [x] Return structured errors for enrichment failures
  - [x] Do not cache error responses

- [x] Task 8: Register enrichment module in registry (AC: #1, #5)
  - [x] Create NewEnrichmentFromConfig constructor function
  - [x] Register enrichment module in registry with type "enrichment"
  - [x] Register in builtins.go (avoiding import cycle)
  - [x] Verify module appears in filter registry

- [x] Task 9: Update pipeline schema for enrichment module (AC: #1, #4)
  - [x] Add enrichment filter type to pipeline-schema.json
  - [x] Define enrichment module configuration structure
  - [x] Document required fields (endpoint, key)
  - [x] Document optional fields (auth, cache, mergeStrategy, onError)
  - [x] Add validation rules and examples

- [x] Task 10: Add tests for enrichment module (AC: #1, #2, #3, #4, #5)
  - [x] Test enrichment module creation with valid config
  - [x] Test enrichment module creation with invalid config
  - [x] Test HTTP request execution with key extraction
  - [x] Test cache hit (use cached data, no HTTP request)
  - [x] Test cache miss (make HTTP request, store in cache)
  - [x] Test cache TTL expiration
  - [x] Test cache size limit and LRU eviction
  - [x] Test data merging strategies
  - [x] Test error handling (HTTP errors, JSON errors, onError modes)
  - [x] Test authentication (API key, Bearer, Basic, OAuth2)
  - [x] Test enrichment filter in pipeline with other filters
  - [x] Test cache isolation between filter instances

- [x] Task 11: Update documentation (AC: #1, #4)
  - [x] Created example config: configs/examples/17-filters-enrichment.yaml
  - [x] Document enrichment filter module in README.md
  - [x] Add security considerations (rate limiting, cache size limits)

## Dev Notes

### Relevant Architecture Patterns and Constraints

**Enrichment Module Design:**
- Enrichment module is a separate filter module type (not part of mapping or script)
- Enrichment executes HTTP requests similar to input modules but within filter context
- Enrichment enriches records with external data fetched on-demand
- Cache prevents redundant API calls for records with same key values
- Cache is in-memory, scoped per filter module instance

**HTTP Request Execution:**
- Reuse HTTP client patterns from `internal/modules/input/http_polling.go`
- Reuse authentication handlers from `internal/auth/` package
- Support same auth types as input modules: API key, Bearer, Basic, OAuth2
- Support timeout configuration (default 30 seconds)
- Support custom headers configuration
- Support dataField extraction (for object responses with nested data arrays)

**Key Extraction:**
- Key is extracted from record using field path (e.g., "customer.id", "order.customerId")
- Key value is used in HTTP request (query param, path param, or header)
- Key value is used as cache key (with optional prefix for namespacing)
- Support for nested field paths (dot notation)

**Cache Implementation:**
- In-memory LRU cache with TTL support
- Thread-safe operations (mutex protection for concurrent access)
- Configurable maxSize (default: 1000 entries)
- Configurable defaultTTL (default: 5 minutes)
- LRU eviction when size limit reached
- Cache key = enrichment module ID + record key value
- Cache entries store: {data: map[string]interface{}, expiresAt: time.Time}

**Data Merging:**
- Merge strategy: "merge" (deep merge, default), "replace" (replace entire record), "append" (append to arrays)
- Deep merge preserves nested structures
- Field mapping support (optional, map enrichment fields to record fields)
- Preserve original record fields unless explicitly replaced

**Error Handling:**
- HTTP errors (4xx, 5xx) are classified using errhandling package
- Network errors are retryable (but not retried automatically in filter context)
- JSON parsing errors are non-retryable
- onError modes: "fail" (default), "skip", "log"
- Errors should include context: record index, endpoint, HTTP status code

**Security Considerations:**
- Cache size limits prevent memory exhaustion (DoS protection)
- HTTP timeout limits prevent hanging requests
- Authentication credentials are never logged
- Cache keys should not contain sensitive data (use hashing if needed)

**Module Registration:**
- Enrichment module registered as type "enrichment" in filter registry
- Follows same pattern as mapping, condition, and script modules
- Auto-registered via init() function in enrichment.go (or builtins.go to avoid cycles)

**Integration with Pipeline:**
- Enrichment filter can be used standalone or with other filters
- Enrichment filter processes records in sequence with other filters
- Enrichment filter follows same Module interface as other filters
- Cache is isolated per filter instance (not shared across pipeline executions)

### Project Structure Notes

**Files Created:**
- `internal/modules/filter/enrichment.go` - Enrichment module implementation
- `internal/modules/filter/enrichment_test.go` - Enrichment module tests
- `internal/cache/lru.go` - LRU cache implementation
- `internal/cache/lru_test.go` - LRU cache tests
- `configs/examples/17-filters-enrichment.yaml` - Example enrichment configuration

**Files Modified:**
- `internal/config/schema/pipeline-schema.json` - Add enrichment filter schema
- `internal/registry/builtins.go` - Register enrichment module

**New Dependencies:**
```go
// No new external dependencies required
// Use standard library: sync, time, net/http
// Reuse existing: internal/auth, internal/errhandling, internal/logger
```

### References

- [Source: internal/modules/filter/script.go] - Script module implementation pattern
- [Source: internal/modules/filter/mapping.go] - Mapping module implementation pattern
- [Source: internal/modules/filter/condition.go] - Condition module implementation pattern
- [Source: internal/modules/input/http_polling.go] - HTTP request execution pattern
- [Source: internal/auth/auth.go] - Authentication handler patterns
- [Source: internal/auth/oauth2.go] - OAuth2 token caching pattern (reference for cache implementation)
- [Source: internal/registry/registry.go] - Module registration pattern
- [Source: internal/factory/modules.go] - Module factory usage
- [Source: docs/MODULE_EXTENSIBILITY.md] - Module extensibility guidelines
- [Source: docs/ARCHITECTURE.md] - Architecture patterns and constraints
- [Source: _bmad-output/planning-artifacts/sprint-change-proposal-2026-01-24.md] - Story origin and rationale

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A - all tests passing

### Completion Notes List

- Implemented LRU cache with TTL support in `internal/cache/lru.go`
- Implemented enrichment filter module in `internal/modules/filter/enrichment.go`
- All tests passing (55 test cases total: 12 for cache, 43 for enrichment)
- Linting passes with 0 issues
- Registered enrichment module in builtins.go
- Updated pipeline schema with enrichment filter definitions
- Created example configuration at `configs/examples/17-filters-enrichment.yaml`
- Added comprehensive authentication tests (API key, Bearer, Basic, OAuth2)
- Added cache TTL expiration tests in enrichment context
- Added cache size limit and LRU eviction tests in enrichment context
- Improved cache config validation in ParseEnrichmentConfig
- Improved error messages to sanitize URLs (remove query params/fragments)
- Added enrichment filter documentation in README.md
- Added security considerations (rate limiting, cache limits) in README.md

### File List

**Created:**
- internal/cache/lru.go
- internal/cache/lru_test.go
- internal/modules/filter/enrichment.go
- internal/modules/filter/enrichment_test.go
- configs/examples/17-filters-enrichment.yaml

**Modified:**
- internal/registry/builtins.go
- internal/config/schema/pipeline-schema.json
- _bmad-output/implementation-artifacts/sprint-status.yaml
- README.md (added enrichment filter documentation and security considerations)
