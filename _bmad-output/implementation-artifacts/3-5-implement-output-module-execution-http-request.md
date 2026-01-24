# Story 3.5: Implement Output Module Execution (HTTP Request)

Status: review

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,  
I want the runtime to execute HTTP Request Output modules,  
So that I can send data to target REST APIs.

## Acceptance Criteria

**Given** I have a connector with HTTP Request Output module configured  
**When** The runtime executes the Output module with transformed data  
**Then** The runtime sends HTTP requests (POST/PUT/PATCH) to the configured endpoint (FR83)  
**And** The runtime handles authentication (API key or OAuth2 basic) (FR45)  
**And** The runtime formats data according to target API schema  
**And** The runtime handles HTTP response codes and errors (FR46)  
**And** The runtime returns execution status and response data (FR42)  
**And** The execution is deterministic (NFR24, NFR25)

## Tasks / Subtasks

- [x] Task 1: Implement core HTTP request sending logic (AC: sends HTTP requests to configured endpoint)
  - [x] Parse output module configuration (endpoint, method, headers, body format)
  - [x] Support HTTP methods: POST, PUT, PATCH (MVP), GET, DELETE (future)
  - [x] Construct HTTP request with proper method and endpoint
  - [x] Format request body from transformed records (JSON by default)
  - [x] Support single record per request (default)
  - [x] Support batch requests (multiple records per request) if configured
  - [x] Add unit tests for basic HTTP request scenarios
- [x] Task 2: Implement authentication handling (AC: handles authentication with target system)
  - [x] Support API key authentication (header or query parameter)
  - [x] Support OAuth2 basic authentication flow
  - [x] Extract credentials from module configuration
  - [x] Add authentication headers/parameters to requests
  - [x] Handle authentication errors gracefully
  - [x] Support different authentication methods per request
  - [x] Add unit tests for authentication scenarios
- [x] Task 3: Implement request formatting and body construction (AC: formats data according to target API schema)
  - [x] Convert records to JSON format (default)
  - [x] Support custom body formatting if configured
  - [x] Handle nested objects and arrays in records
  - [x] Support path parameter substitution in endpoint URLs
  - [x] Support query parameter construction from record data
  - [x] Support custom headers from record data
  - [x] Handle empty records gracefully
  - [x] Add unit tests for request formatting scenarios
- [x] Task 4: Implement HTTP response handling (AC: handles HTTP response codes and errors)
  - [x] Parse HTTP response status codes
  - [x] Handle success responses (2xx status codes)
  - [x] Handle client errors (4xx status codes) with appropriate error messages
  - [x] Handle server errors (5xx status codes) with retry logic support
  - [x] Parse response body for error details
  - [x] Support success condition configuration (custom success criteria)
  - [x] Return structured error information
  - [x] Add unit tests for response handling scenarios
- [x] Task 5: Implement error handling and retry logic (AC: handles HTTP response codes and errors)
  - [x] Detect transient errors (5xx, network errors, timeouts)
  - [x] Apply retry logic for transient errors (configurable retry count and backoff)
  - [x] Stop execution for fatal errors (4xx client errors)
  - [x] Log all errors with structured context (endpoint, method, status, error)
  - [x] Support error handling modes: "fail" (stop), "skip" (skip record), "log" (log and continue)
  - [x] Return error result with details for pipeline executor
  - [x] Add unit tests for error handling scenarios
- [x] Task 6: Implement execution status reporting (AC: returns execution status and response data)
  - [x] Track number of records successfully sent
  - [x] Track number of records that failed
  - [x] Return execution status with success/failure counts
  - [x] Return response data (status codes, response bodies) for successful requests
  - [x] Return error details for failed requests
  - [x] Support detailed logging of execution results
  - [x] Add unit tests for status reporting scenarios
- [x] Task 7: Integrate with pipeline executor (AC: returns execution status and response data)
  - [x] Ensure `HTTPRequest.Send()` implements `output.Module` interface
  - [x] Receive records from Filter modules via `Send(records []map[string]interface{})`
  - [x] Handle empty input records gracefully (return success with 0 sent)
  - [x] Ensure output module is called after all Filter modules complete
  - [x] Test integration with pipeline executor (Story 2.3)
  - [x] Test end-to-end pipeline: Input → Filter → Output
  - [x] Add integration tests with complete pipeline execution
- [x] Task 8: Ensure deterministic execution (AC: execution is deterministic)
  - [x] Ensure same input records + same config = same HTTP requests (no randomness)
  - [x] Ensure request formatting is deterministic (same data = same request body)
  - [x] Ensure authentication is deterministic (same credentials = same auth headers)
  - [x] Ensure error handling is deterministic (same error = same handling)
  - [x] No time-dependent logic in request construction (except timestamps if required by API)
  - [x] Add tests to verify deterministic behavior
  - [x] Document any non-deterministic behaviors (if any)

## Dev Notes

### Architecture Requirements

**Output Module Execution:**
- **Location:** `canectors-runtime/internal/modules/output/http_request.go`
- **Interface:** Implements `output.Module` interface with `Send(records []map[string]interface{}) (int, error)` and `Close() error`
- **Configuration:** Reads from `connector.Pipeline.Output` object, output with `type: "httpRequest"`
- **Purpose:** Sends transformed data records to target REST APIs via HTTP requests

**Module Interface Integration:**
- Must implement `output.Module` interface:
  ```go
  type Module interface {
      Send(records []map[string]interface{}) (int, error)
      Close() error
  }
  ```
- Called by `Executor.executeOutput()` in `internal/runtime/pipeline.go` after all Filter modules complete
- Receives records from Filter modules (or Input module if no filters)
- Returns number of records successfully sent and any error

**Configuration Structure:**
- Output module configuration is in `connector.Pipeline.Output` object
- HTTP Request output configuration includes:
  - `type`: "httpRequest" (required)
  - `endpoint`: Target API endpoint URL (required)
  - `method`: HTTP method - "POST", "PUT", "PATCH" (required, MVP supports POST/PUT/PATCH)
  - `headers`: Custom HTTP headers (optional, map[string]string)
  - `authentication`: Authentication configuration (optional)
    - `type`: "apiKey" or "oauth2Basic"
    - `apiKey`: For API key auth - `location`: "header" or "query", `name`: header/param name, `value`: credential value
    - `oauth2Basic`: For OAuth2 basic - `clientId`, `clientSecret`, `tokenUrl` (future: full OAuth2 flow)
  - `request`: Request configuration (optional)
    - `bodyFrom`: "records" (default) - send records as JSON array, or "record" - send single record per request
    - `pathParams`: Path parameter substitution from record data (optional, map[string]string)
    - `query`: Query parameters from record data (optional, map[string]string)
  - `success`: Success condition configuration (optional)
    - `statusCodes`: Array of HTTP status codes considered success (default: [200, 201, 202, 204])
  - `enabled`: Boolean, default true (optional)
  - `onError`: "fail", "skip", "log" - error handling mode (optional, default: "fail")
  - `timeoutMs`: Timeout in milliseconds (optional, default: 30000)
  - `retry`: Retry configuration (optional)
    - `maxRetries`: Maximum retry attempts (default: 3)
    - `backoffMs`: Initial backoff in milliseconds (default: 1000)
    - `backoffMultiplier`: Backoff multiplier (default: 2.0)

**HTTP Request Format:**
- **Default Behavior:** Send records as JSON array in request body
  ```json
  POST /api/endpoint
  Content-Type: application/json
  
  [
    { "field1": "value1", "field2": "value2" },
    { "field1": "value3", "field2": "value4" }
  ]
  ```
- **Single Record Mode:** Send one record per request (if `request.bodyFrom: "record"`)
  - Multiple HTTP requests, one per record
  - Useful for APIs that don't accept batch requests
- **Path Parameters:** Substitute path parameters from record data
  - Example: endpoint `/api/users/{userId}/orders`, pathParams: `{"userId": "user.id"}`
  - Extracts `user.id` from record and substitutes in URL
- **Query Parameters:** Add query parameters from record data
  - Example: query: `{"status": "record.status", "limit": "100"}`
  - Constructs query string from record values

**Authentication Handling:**
- **API Key Authentication:**
  - Location: "header" - adds header (e.g., `X-API-Key: <value>`)
  - Location: "query" - adds query parameter (e.g., `?api_key=<value>`)
  - Credential value from configuration (encrypted at rest, decrypted at runtime)
- **OAuth2 Basic Authentication (MVP):**
  - Client credentials flow: POST to tokenUrl with clientId/clientSecret
  - Receive access token, add as Bearer token in Authorization header
  - Token caching: cache token until expiration to avoid repeated token requests
  - Token refresh: request new token when expired
- **Future:** Full OAuth2 flow with refresh tokens, authorization code flow (post-MVP)

**Response Handling:**
- **Success Status Codes:** Default [200, 201, 202, 204]
  - Configurable via `success.statusCodes` array
  - Any status code in this array is considered success
- **Client Errors (4xx):** Typically fatal, don't retry
  - 400 Bad Request: Invalid request format
  - 401 Unauthorized: Authentication failed
  - 403 Forbidden: Access denied
  - 404 Not Found: Resource not found
  - 422 Unprocessable Entity: Validation errors
- **Server Errors (5xx):** Transient, retry with backoff
  - 500 Internal Server Error
  - 502 Bad Gateway
  - 503 Service Unavailable
  - 504 Gateway Timeout
- **Network Errors:** Transient, retry with backoff
  - Connection timeout
  - DNS resolution failure
  - Connection refused

**Error Handling Strategy:**
- **Error Modes:**
  - `"fail"` (default): Stop processing on first error, return error
  - `"skip"`: Skip failed record, continue with next record
  - `"log"`: Log error, continue processing (may produce partial results)
- **Retry Logic:**
  - Only retry transient errors (5xx, network errors)
  - Exponential backoff: initial backoff * (backoffMultiplier ^ retryAttempt)
  - Maximum retries configurable (default: 3)
  - After max retries, apply error mode (fail/skip/log)
- **Error Context:** Include endpoint, method, status code, response body, record index in error details
- **Logging:** Log errors with structured context (endpoint, method, status, error, record index) using logger package

**Deterministic Execution:**
- Same input records + same configuration = same HTTP requests sent
- Request body formatting must be deterministic (JSON serialization order, etc.)
- Authentication must be deterministic (same credentials = same auth headers)
- Error handling must be deterministic (same error = same handling)
- No random behavior in request construction
- Timestamps in request body (if required by API) should use consistent format

**Integration with Pipeline Executor:**
- Called by `Executor.executeOutput()` in `internal/runtime/pipeline.go`
- Receives records from Filter modules (or Input module if no filters)
- Must handle empty input gracefully (return success with 0 sent)
- Must preserve execution context (pipeline ID, etc.) for logging
- **Close() method:** Release HTTP client resources, close connections
- Called automatically by pipeline executor via defer

### Project Structure Notes

**File Organization:**
```
canectors-runtime/
├── internal/
│   └── modules/
│       └── output/
│           ├── output.go              # Module interface (already defined)
│           ├── http_request.go        # HTTP Request implementation (to be created)
│           ├── http_request_test.go   # HTTP Request tests (to be created)
│           └── auth.go                # Authentication helpers (optional, can be in http_request.go)
└── internal/
    └── runtime/
        └── pipeline.go               # Executor that calls Output.Send() (Story 2.3)
```

**Integration Points:**
- HTTP Request will be used by:
  - `Executor.executeOutput()` - pipeline execution (Story 2.3)
  - Future output modules - reference implementation
- HTTP Request depends on:
  - `pkg/connector.Pipeline` type (already defined)
  - `pkg/connector.ModuleConfig` type (already defined)
  - `output.Module` interface (already defined in `output.go`)
  - Logger: `internal/logger` package (already available)
  - HTTP client: Go standard library `net/http` or `golang.org/x/net/http2` (if HTTP/2 needed)

**Module Instantiation:**
- HTTP Request struct should be created from `ModuleConfig`
- Constructor: `NewHTTPRequestFromConfig(config *connector.ModuleConfig) (*HTTPRequest, error)`
- Configuration validation should happen in constructor
- HTTP client initialization in constructor (reusable client for multiple requests)

**HTTP Client Design:**
- **Client Reuse:** Create single HTTP client per module instance, reuse for all requests
- **Timeout Configuration:** Set timeout from `timeoutMs` configuration
- **Transport Configuration:** Default transport, can be customized for future needs (HTTP/2, custom TLS, etc.)
- **Connection Pooling:** Use default Go HTTP client connection pooling
- **Request Context:** Use request context for cancellation and timeout

**Authentication Implementation Design:**
- **API Key:** Simple header/query parameter injection
- **OAuth2 Basic (MVP):**
  - Token request: POST to tokenUrl with clientId/clientSecret
  - Token storage: In-memory cache with expiration time
  - Token refresh: Check expiration before each request, refresh if needed
  - Error handling: If token request fails, return error (don't retry token request)
- **Credential Storage:** Credentials encrypted at rest, decrypted at runtime
  - Use encryption utility from project (if available) or implement basic decryption
  - Credentials passed in configuration (encrypted values)

### Previous Story Intelligence

**From Story 3.4 (Condition Filter):**
- Module interface pattern: `Process()` for filter, `Send()` for output
- Configuration structure: `type`, `enabled`, `onError`, `timeoutMs` in `ModuleConfig`
- Error handling modes: "fail", "skip", "log" with structured logging
- Deterministic execution is critical: same input = same output
- Integration pattern: Called by pipeline executor in sequence
- Testing pattern: Unit tests for core logic, integration tests with pipeline executor
- File location: `internal/modules/` directory

**From Story 3.3 (Mapping Filter):**
- Module configuration validation in constructor
- Error handling with structured context logging
- Integration with pipeline executor via interface
- Deterministic execution requirements
- Testing patterns: Unit tests + integration tests

**From Story 3.2 (Webhook Input):**
- HTTP server implementation patterns
- Request/response handling
- Error handling and logging

**From Story 3.1 (HTTP Polling Input):**
- HTTP client usage patterns
- Request construction and execution
- Response parsing and error handling
- Authentication handling (API key, OAuth2 basic)

**Key Learnings:**
- Follow established module patterns from previous stories
- Use same error handling and logging patterns
- Maintain deterministic execution guarantees
- Test integration with pipeline executor
- Follow Go best practices for HTTP client usage
- Reuse HTTP client for multiple requests (connection pooling)
- Handle authentication tokens with caching and refresh

### Testing Requirements

**Unit Tests:**
- HTTP request construction with different methods (POST, PUT, PATCH)
- Request body formatting (JSON array, single record)
- Path parameter substitution in endpoint URLs
- Query parameter construction from record data
- Custom headers from configuration and record data
- API key authentication (header and query parameter locations)
- OAuth2 basic authentication (token request, token caching, token refresh)
- Success status code handling (2xx codes)
- Client error handling (4xx codes)
- Server error handling (5xx codes)
- Network error handling (timeout, connection refused)
- Retry logic with exponential backoff
- Error handling modes: "fail", "skip", "log"
- Empty input records handling
- Deterministic execution (same input = same requests)
- Close() method resource cleanup

**Integration Tests:**
- Pipeline execution with HTTP Request output module
- End-to-end test: Input → Filter → Output
- Chained modules: Input → Mapping → Condition → Output
- Integration with Executor from Story 2.3
- Error propagation through pipeline
- Authentication token caching across multiple requests
- Batch request handling (multiple records per request)
- Single record mode (one request per record)

**Test Data:**
- Create test fixtures in `/internal/modules/output/testdata/`:
  - `valid-http-request-config.json` - Valid HTTP request configuration
  - `http-request-with-api-key.json` - Configuration with API key auth
  - `http-request-with-oauth2.json` - Configuration with OAuth2 basic auth
  - `http-request-with-path-params.json` - Configuration with path parameters
  - `http-request-with-query-params.json` - Configuration with query parameters
  - `http-request-batch-mode.json` - Configuration for batch requests
  - `http-request-single-record-mode.json` - Configuration for single record per request
  - `http-request-custom-success-codes.json` - Configuration with custom success codes
  - `http-request-retry-config.json` - Configuration with retry settings
  - `http-request-error-skip-mode.json` - Configuration with onError="skip"
  - `http-request-error-log-mode.json` - Configuration with onError="log"

**Mock HTTP Server:**
- Use `net/http/httptest` package for testing
- Create test server that responds with different status codes
- Test server that simulates network errors
- Test server that simulates OAuth2 token endpoint
- Test server that validates request format and authentication

### References

- **Source:** `canectors-BMAD/_bmad-output/planning-artifacts/epics.md#Story-3.5` - Story requirements and acceptance criteria
- **Source:** `canectors-BMAD/_bmad-output/planning-artifacts/architecture.md` - Architecture patterns, module structure, deterministic execution requirements, HTTP client patterns
- **Source:** `canectors-runtime/internal/modules/output/output.go` - Output module interface definition
- **Source:** `canectors-runtime/internal/modules/filter/mapping.go` - Reference implementation for module pattern (Story 3.3)
- **Source:** `canectors-runtime/internal/modules/filter/condition.go` - Reference implementation for module pattern (Story 3.4)
- **Source:** `canectors-runtime/internal/modules/input/http_polling.go` - Reference implementation for HTTP client usage (Story 3.1)
- **Source:** `canectors-runtime/pkg/connector/types.go` - Pipeline and ModuleConfig type definitions
- **Source:** `canectors-runtime/internal/config/schema/pipeline-schema.json` - HTTP Request output schema definition (type, endpoint, method, headers, authentication, request, success)
- **Source:** `canectors-BMAD/_bmad-output/project-context.md` - Go runtime patterns, testing standards, code organization, HTTP client best practices
- **Source:** `canectors-BMAD/_bmad-output/implementation-artifacts/3-4-implement-filter-module-execution-conditions.md` - Previous story learnings and patterns
- **Source:** `canectors-BMAD/_bmad-output/implementation-artifacts/3-3-implement-filter-module-execution-mapping.md` - Previous story learnings and patterns
- **Source:** `canectors-BMAD/_bmad-output/implementation-artifacts/3-1-implement-input-module-execution-http-polling.md` - HTTP client patterns and authentication handling

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (via Cursor)

### Debug Log References

- All tests pass: 45.1s for output module tests
- Full test suite passes: cmd, config, logger, filter, input, output, runtime, connector
- golangci-lint: 0 issues

### Completion Notes List

- **Task 1:** Implemented core HTTP request module with POST/PUT/PATCH support, batch and single-record modes, JSON body formatting
- **Task 2:** Implemented full authentication: API key (header/query), Bearer token, Basic auth, OAuth2 client credentials with token caching
- **Task 3:** Implemented request formatting: path parameters, query parameters (static and from record), headers from record data, nested object support
- **Task 4:** Implemented HTTP response handling: custom success codes, structured HTTPError with response body capture
- **Task 5:** Implemented retry logic with exponential backoff for transient errors (5xx, 429, network), no retry for 4xx client errors
- **Task 6:** Status reporting via (int, error) return: sent count, HTTPError with full context
- **Task 7:** Full integration with pipeline executor, implements output.Module interface
- **Task 8:** Verified deterministic execution: same input/config = same output, no random values added

### File List

**New Files:**
- `canectors-runtime/internal/modules/output/http_request.go` - Main HTTP request output module implementation
- `canectors-runtime/internal/modules/output/http_request_test.go` - Unit tests for HTTP request module
- `canectors-runtime/internal/modules/output/integration_test.go` - Integration tests with pipeline executor

**Modified Files:**
- `canectors-runtime/internal/modules/output/output.go` - Cleaned up to contain only Module interface

## Change Log

| Date | Changes |
|------|---------|
| 2026-01-16 | Story 3.5 created - Ready for development |
| 2026-01-20 | Story 3.5 implementation completed - All 8 tasks done, all tests passing |
