# Story 3.1: Implement Input Module Execution (HTTP Polling)

Status: review

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,  
I want the runtime to execute HTTP Request Input modules with polling,  
So that I can retrieve data from source REST APIs.

## Acceptance Criteria

**Given** I have a connector with HTTP Request Input module configured  
**When** The runtime executes the Input module  
**Then** The runtime makes HTTP GET requests to the configured endpoint (FR64)  
**And** The runtime handles authentication (API key or OAuth2 basic) (FR44)  
**And** The runtime handles pagination if configured  
**And** The runtime returns retrieved data for processing by Filter modules (FR40)  
**And** The runtime handles HTTP errors gracefully (FR46)  
**And** The execution is deterministic (NFR24, NFR25)

## Tasks / Subtasks

- [x] Task 1: Implement HTTP GET request execution (AC: makes HTTP GET requests to configured endpoint)
  - [x] Create HTTP client with proper configuration
  - [x] Parse endpoint URL from module configuration
  - [x] Execute HTTP GET request to endpoint
  - [x] Parse JSON response body into `[]map[string]interface{}`
  - [x] Handle different response formats (JSON array, JSON object with array field)
  - [x] Add unit tests for HTTP GET execution
- [x] Task 2: Implement authentication handling (AC: handles API key or OAuth2 basic)
  - [x] Support API key authentication in headers or query params
  - [x] Support OAuth2 basic authentication flow
  - [x] Extract authentication config from module configuration
  - [x] Apply authentication to HTTP requests
  - [x] Handle authentication errors gracefully
  - [x] Add unit tests for authentication scenarios
- [x] Task 3: Implement pagination handling (AC: handles pagination if configured)
  - [x] Detect pagination configuration (page-based, offset-based, cursor-based)
  - [x] Implement page-based pagination iteration
  - [x] Implement offset-based pagination iteration
  - [x] Implement cursor-based pagination iteration
  - [x] Aggregate all paginated results into single dataset
  - [x] Add unit tests for pagination scenarios
- [x] Task 4: Implement error handling (AC: handles HTTP errors gracefully)
  - [x] Handle HTTP error status codes (4xx, 5xx)
  - [x] Handle network errors (timeout, connection refused)
  - [x] Return structured errors with context
  - [x] Log errors with sufficient context for debugging
  - [x] Ensure no data loss on errors
  - [x] Add unit tests for error scenarios
- [x] Task 5: Ensure deterministic execution (AC: execution is deterministic)
  - [x] No random behavior in HTTP requests
  - [x] Consistent request ordering for pagination
  - [x] Fixed timeout values (no random delays)
  - [x] Same input configuration = same output data
  - [x] Add tests to verify deterministic behavior
- [x] Task 6: Integrate with pipeline executor (AC: returns data for Filter modules)
  - [x] Ensure `HTTPPolling.Fetch()` returns `[]map[string]interface{}`
  - [x] Verify integration with `Executor.Execute()` from Story 2.3
  - [x] Test end-to-end with pipeline execution
  - [x] Add integration tests with pipeline executor

## Dev Notes

### Architecture Requirements

**HTTP Polling Input Module:**
- **Location:** `cannectors-runtime/internal/modules/input/http_polling.go`
- **Interface:** Implements `input.Module` interface with `Fetch()` method
- **Return Type:** `Fetch() ([]map[string]interface{}, error)` - Returns slice of records
- **Configuration:** Reads from `connector.Pipeline.Input` module configuration
- **Purpose:** Retrieves data from source REST APIs via HTTP GET requests

**Module Interface Integration:**
- Must implement `input.Module` interface:
  ```go
  type Module interface {
      Fetch() ([]map[string]interface{}, error)
  }
  ```
- Note: `Close()` is not needed for input modules as they don't maintain persistent resources.
  Unlike `output.Module`, input modules create resources on-demand (HTTP client per request)
  that are automatically garbage collected.
- `Fetch()` method is called by `Executor.Execute()` from Story 2.3
- Data returned by `Fetch()` is passed to Filter modules for transformation

**Configuration Structure:**
- Input module configuration is in `connector.Pipeline.Input` field
- Configuration includes:
  - `type`: "httpPolling"
  - `endpoint`: HTTP endpoint URL (required)
  - `method`: HTTP method (GET for polling)
  - `auth`: Authentication configuration (API key, OAuth2 basic)
  - `pagination`: Pagination configuration (page-based, offset-based, cursor-based)
  - `headers`: Custom HTTP headers (optional)
  - `timeout`: Request timeout in seconds (optional)

**Authentication Support:**
- **API Key:** 
  - Location: Header (`Authorization: Bearer <key>`) or query param (`?api_key=<key>`)
  - Configuration: `auth.type: "apiKey"`, `auth.value: "<key>", `auth.location: "header" | "query"`
- **OAuth2 Basic:**
  - Flow: Client credentials grant
  - Configuration: `auth.type: "oauth2"`, `auth.clientId: "<id>", `auth.clientSecret: "<secret>", `auth.tokenUrl: "<url>"`

**Pagination Support:**
- **Page-based:** `pagination.type: "page"`, `pagination.pageParam: "page"`, `pagination.totalPagesField: "total_pages"`
- **Offset-based:** `pagination.type: "offset"`, `pagination.offsetParam: "offset"`, `pagination.limitParam: "limit"`
- **Cursor-based:** `pagination.type: "cursor"`, `pagination.cursorParam: "cursor"`, `pagination.nextCursorField: "next_cursor"`

**Deterministic Execution:**
- Same configuration + same input = same output
- Fixed request ordering (no random order)
- Consistent pagination iteration (same order every time)
- No time-dependent logic (except timestamps in data if present)
- Same error handling (same error = same result)

**Error Handling Strategy:**
- **HTTP Errors (4xx, 5xx):** Return error with status code and message
- **Network Errors:** Return error with connection details
- **Timeout Errors:** Return error with timeout value
- **JSON Parse Errors:** Return error with parse details
- **Pagination Errors:** Return error with pagination details
- **Authentication Errors:** Return error with auth details
- All errors must include context (endpoint, method, status code, etc.)

### Project Structure Notes

**File Organization:**
```
cannectors-runtime/
├── internal/
│   └── modules/
│       └── input/
│           ├── input.go              # Module interface (already defined)
│           ├── http_polling.go       # HTTPPolling implementation (to be completed)
│           └── http_polling_test.go  # Unit tests for HTTPPolling
├── pkg/
│   └── connector/
│       └── types.go                  # Pipeline, ModuleConfig, AuthConfig types (already defined)
└── internal/
    └── runtime/
        └── pipeline.go               # Executor that calls Input.Fetch() (Story 2.3)
```

**Integration Points:**
- HTTPPolling will be used by:
  - `Executor.Execute()` - pipeline execution (Story 2.3)
  - Future scheduler - CRON scheduling for polling (Epic 4, Story 4.1)
- HTTPPolling depends on:
  - `pkg/connector.Pipeline` type (already defined)
  - `pkg/connector.ModuleConfig` type (already defined)
  - `pkg/connector.AuthConfig` type (already defined)
  - `input.Module` interface (already defined in `input.go`)
  - Logger: `internal/logger` package (already available)

**Module Instantiation:**
- HTTPPolling struct should be created from `ModuleConfig`
- Constructor: `NewHTTPPolling(config *connector.ModuleConfig) (*HTTPPolling, error)`
- Configuration validation should happen in constructor
- Module should be ready to use after instantiation

### Testing Requirements

**Unit Tests:**
- HTTPPolling.Fetch() with successful HTTP GET request
- HTTPPolling.Fetch() with JSON array response
- HTTPPolling.Fetch() with JSON object containing array field
- HTTPPolling.Fetch() with API key authentication (header)
- HTTPPolling.Fetch() with API key authentication (query param)
- HTTPPolling.Fetch() with OAuth2 basic authentication
- HTTPPolling.Fetch() with page-based pagination
- HTTPPolling.Fetch() with offset-based pagination
- HTTPPolling.Fetch() with cursor-based pagination
- HTTPPolling.Fetch() with HTTP error status codes (400, 401, 403, 404, 500)
- HTTPPolling.Fetch() with network errors (timeout, connection refused)
- HTTPPolling.Fetch() with JSON parse errors
- HTTPPolling.Fetch() with pagination errors
- HTTPPolling.Fetch() with authentication errors
- HTTPPolling.Fetch() with empty response
- HTTPPolling.Fetch() with large datasets (performance test)
- Deterministic execution (same input = same output)

**Integration Tests:**
- Pipeline execution with HTTPPolling input module
- End-to-end test: HTTPPolling → Filter → Output
- Integration with Executor from Story 2.3

**Test Data:**
- Create test HTTP server for mocking API responses
- Test fixtures in `/internal/modules/input/testdata/`:
  - `valid-http-polling-config.json` - Valid HTTP polling configuration
  - `http-polling-with-auth.json` - Configuration with authentication
  - `http-polling-with-pagination.json` - Configuration with pagination
  - `http-polling-error-401.json` - Configuration that will return 401
  - `http-polling-error-timeout.json` - Configuration that will timeout

**HTTP Test Server:**
- Use `net/http/httptest` package for creating test HTTP server
- Mock different response scenarios:
  - Successful JSON array response
  - Successful JSON object with array field
  - Paginated responses (multiple pages)
  - Error responses (4xx, 5xx)
  - Timeout scenarios
  - Authentication required scenarios

### References

- **Epic 3 Overview:** [Source: _bmad-output/planning-artifacts/epics.md#Epic 3: Module Execution]
- **Story 3.1 Details:** [Source: _bmad-output/planning-artifacts/epics.md#Story 3.1: Implement Input Module Execution (HTTP Polling)]
- **Story 2.3 (Previous):** [Source: _bmad-output/implementation-artifacts/2-3-implement-pipeline-orchestration.md]
- **Input Module Interface:** [Source: cannectors-runtime/internal/modules/input/input.go]
- **Pipeline Types:** [Source: cannectors-runtime/pkg/connector/types.go]
- **Executor Implementation:** [Source: cannectors-runtime/internal/runtime/pipeline.go]
- **CLI Runtime Architecture:** [Source: _bmad-output/planning-artifacts/architecture.md#Runtime CLI (Go) - Separate Project]
- **Project Context:** [Source: _bmad-output/project-context.md]
- **PRD Requirements:** [Source: _bmad-output/planning-artifacts/prd.md#Input Modules]

### Critical Implementation Rules

**From Project Context:**
- Runtime CLI is separate Go project from Next.js application [Source: _bmad-output/project-context.md#Technology Stack]
- Go version: 1.23.5 (latest stable) [Source: cannectors-runtime/go.mod]
- Follow Go best practices and conventions [Source: _bmad-output/project-context.md#Critical Implementation Rules]

**From Architecture:**
- Input modules: `/internal/modules/input/` for input module implementations [Source: _bmad-output/planning-artifacts/architecture.md#CLI Runtime (Go)]
- HTTP polling: Primary input method for retrieving data from REST APIs [Source: _bmad-output/planning-artifacts/architecture.md#Input Modules]
- Deterministic execution required (NFR24, NFR25) [Source: _bmad-output/planning-artifacts/architecture.md#Reliability]

**From Epic 2 & Story 2.3 Learnings:**
- Pipeline orchestration is implemented and tested (Story 2.3)
- Executor.Execute() expects `Input.Module.Fetch()` to return `[]map[string]interface{}`
- Module interfaces are defined but not yet implemented (Epic 3)
- Structured logging with `log/slog` is used throughout runtime
- Error handling follows consistent patterns (structured errors with context)

**From Epic 3 Context:**
- Story 3.1 is the first story in Epic 3 (Module Execution)
- Story 3.2 will implement Webhook Input module
- Story 3.3 will implement Mapping Filter module
- Story 3.4 will implement Condition Filter module
- Story 3.5 will implement HTTP Request Output module
- Story 3.6 will implement Authentication Handling (may be integrated into this story)

**Go-Specific Rules:**
- Use standard Go HTTP client (`net/http`)
- Use `context.Context` for request cancellation and timeouts
- Follow Go naming conventions (`camelCase` for exported, `camelCase` for unexported)
- Use `internal/` package for private code (not imported by external packages)
- Write tests alongside code (`*_test.go`)
- Handle errors explicitly (no silent failures)
- Use structured logging (`slog`) for consistency
- Return structured error types for detailed error information

**HTTP Client Best Practices:**
- Use `http.Client` with configurable timeout
- Set appropriate User-Agent header
- Handle context cancellation for request timeout
- Close response body to prevent resource leaks
- Use `defer response.Body.Close()` pattern
- Handle different HTTP status codes appropriately

### Library and Framework Requirements

**Go Standard Library:**
- `net/http` - HTTP client for making GET requests
- `net/url` - URL parsing and query parameter manipulation
- `context` - Context for cancellation and timeouts
- `encoding/json` - JSON parsing for response bodies
- `errors` - Error wrapping and handling
- `fmt` - Error formatting
- `io` - Reading response bodies
- `time` - Timeout configuration
- `log/slog` - Structured logging (Go 1.21+)

**External Dependencies:**
- No new dependencies required for Story 3.1
- Existing dependencies:
  - Standard library only (no external HTTP libraries)
  - `internal/logger` - Structured logging wrapper (already available)

**Build Tools:**
- `go test` - Testing
- `go build` - Compilation
- `go mod tidy` - Dependency management

**HTTP Client Pattern:**
```go
// Example HTTP client setup
client := &http.Client{
    Timeout: time.Second * 30, // Configurable timeout
}

req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
// Add headers, authentication, etc.
resp, err := client.Do(req)
// Handle response, parse JSON, etc.
```

### Previous Story Intelligence

**Epic 2 Stories (Completed):**
- **Story 2.1:** Go CLI project initialized [Source: _bmad-output/implementation-artifacts/2-1-initialize-go-cli-project-structure.md]
  - Project structure: `/cmd/cannectors/`, `/internal/`, `/pkg/connector/`
  - Module interfaces defined: `input.Module`, `filter.Module`, `output.Module`
  - Stub implementations exist: `HTTPPolling` struct with `ErrNotImplemented` in `Fetch()`
  - **Key learning:** Module interfaces are defined but not implemented (Epic 3)
- **Story 2.2:** Configuration parser implemented [Source: _bmad-output/implementation-artifacts/2-2-implement-configuration-parser.md]
  - JSON/YAML parsing with auto-detection
  - JSON Schema validation with embedded schema
  - Configuration types defined: `Pipeline`, `ModuleConfig`, `AuthConfig`
  - **Key learning:** Configuration structure supports authentication and pagination configs
- **Story 2.3:** Pipeline orchestration implemented [Source: _bmad-output/implementation-artifacts/2-3-implement-pipeline-orchestration.md]
  - Executor.Execute() orchestrates Input → Filter → Output
  - Executor calls `Input.Module.Fetch()` and expects `[]map[string]interface{}` return
  - Error handling with structured `ExecutionResult`
  - **Key learning:** Input modules must return slice of records for Filter modules to process

**Key Learnings from Epic 2:**
- Configuration parsing is complete and tested
- Pipeline types are well-defined in `pkg/connector/types.go`
- Executor expects `Input.Module.Fetch()` to return `[]map[string]interface{}`
- Structured logging with `log/slog` is used throughout
- Error handling follows consistent patterns (structured errors with context)

**Integration with Epic 3:**
- Story 3.1 implements the first real module (HTTPPolling) that will be used by Executor
- Story 3.2 will implement Webhook Input module (alternative to HTTP polling)
- Stories 3.3-3.5 will implement Filter and Output modules
- Story 3.6 will implement Authentication Handling (may overlap with Story 3.1)

**Epic Context:**
- Epic 3 is Priority 2 (CLI Runtime) - Partie 2/3
- Epic 3 has 6 stories: 3.1 (HTTP Polling), 3.2 (Webhook), 3.3-3.6 (Filter, Output, Auth)
- Story 3.1 is the first story in Epic 3, establishing patterns for other modules

### Git Intelligence Summary

**Recent Work:**
- Story 2.1 completed: Go CLI project structure initialized
- Story 2.2 completed: Configuration parser implemented
- Story 2.3 completed: Pipeline orchestration implemented
- Current state: `HTTPPolling` struct exists with `ErrNotImplemented` in `Fetch()` method

**Repository Structure:**
- Main project: `cannectors/` (Next.js T3 Stack)
- Runtime project: `cannectors-runtime/` (Go - separate project)
- Schema location: `/types/pipeline-schema.json` in main project

**Files from Previous Stories:**
- `/pkg/connector/types.go` - Pipeline types (Story 2.1)
- `/internal/config/parser.go` - Configuration parser (Story 2.2)
- `/internal/config/validator.go` - Schema validator (Story 2.2)
- `/internal/runtime/pipeline.go` - Pipeline executor (Story 2.3)
- `/internal/modules/input/input.go` - Input module interface (Story 2.1)
- `/internal/modules/input/input.go` - HTTPPolling struct stub (Story 2.1, to be completed in 3.1)

**Current HTTPPolling Stub:**
```go
// From cannectors-runtime/internal/modules/input/input.go
type HTTPPolling struct {
    endpoint string
    interval int
}

func (h *HTTPPolling) Fetch() ([]map[string]interface{}, error) {
    return nil, ErrNotImplemented
}
```

### Latest Technical Information

**Go HTTP Client Patterns:**
- Use `http.Client` with configurable timeout (not `http.DefaultClient`)
- Pattern:
  ```go
  client := &http.Client{
      Timeout: time.Second * 30,
  }
  req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
  resp, err := client.Do(req)
  ```
- Benefits: Testable, configurable, supports context cancellation

**JSON Parsing Patterns:**
- Parse JSON response body into `[]map[string]interface{}`
- Handle different response formats:
  - Array response: `[{"id": 1}, {"id": 2}]`
  - Object with array field: `{"data": [{"id": 1}, {"id": 2}]}`
- Use `json.Decoder` for streaming large responses (if needed)

**Error Handling Patterns:**
- Use Go `error` interface consistently
- Wrap errors for context: `fmt.Errorf("fetching data from %s: %w", endpoint, err)`
- Return structured errors with HTTP status codes
- Log errors with context: `logger.Error("http polling failed", "endpoint", endpoint, "error", err)`

**Pagination Patterns:**
- Iterate through pages/offsets/cursors until no more data
- Aggregate all results into single `[]map[string]interface{}`
- Handle pagination errors (infinite loop detection, max pages limit)
- Deterministic: Same pagination order every time

**Authentication Patterns:**
- **API Key in Header:** `req.Header.Set("Authorization", "Bearer "+apiKey)`
- **API Key in Query:** `url.Query().Set("api_key", apiKey)`
- **OAuth2 Basic:** Get token from token endpoint, use in Authorization header

**Testing Patterns:**
- Use `net/http/httptest` for creating test HTTP server
- Mock different response scenarios (success, error, pagination)
- Test authentication scenarios with mock server
- Test pagination with multiple mock responses
- Test error scenarios (timeout, network errors, HTTP errors)

**Deterministic Execution:**
- No random behavior in HTTP requests
- Fixed request ordering for pagination
- Consistent timeout values (no random delays)
- Same input configuration = same output data

**Performance Considerations:**
- HTTP client timeout should be configurable
- Handle large datasets efficiently (streaming may be needed in future)
- Logging should not block execution
- Error handling should be fast (fail fast on errors)

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (via Cursor)

### Debug Log References

- All tests pass: 32 unit tests + 3 integration tests in `internal/modules/input/`
- Full test suite passes with no regressions
- Test coverage includes: HTTP GET, authentication (API key, bearer, basic, OAuth2), pagination (page, offset, cursor), error handling, deterministic execution

### Completion Notes List

1. **Task 1 (HTTP GET)**: Implemented `HTTPPolling` struct with `NewHTTPPollingFromConfig()` constructor. Uses `net/http` client with configurable timeout, custom headers, User-Agent. Supports both JSON array and object responses with `dataField` extraction.

2. **Task 2 (Authentication)**: Implemented 4 auth types:
   - `api-key`: Header (Bearer) or query param location
   - `bearer`: Direct token in Authorization header
   - `basic`: HTTP Basic Auth
   - `oauth2`: Client credentials flow with token caching and expiry handling

3. **Task 3 (Pagination)**: Implemented 3 pagination strategies:
   - `page`: Page-number based with `totalPagesField`
   - `offset`: Offset/limit based with `totalField`
   - `cursor`: Cursor-based with `nextCursorField`
   - All strategies aggregate results into single `[]map[string]interface{}`

4. **Task 4 (Error Handling)**: Structured `HTTPError` type with status code, endpoint, message. Handles 4xx/5xx HTTP errors, network errors (timeout, connection refused), JSON parse errors. All errors logged with context via `log/slog`.

5. **Task 5 (Deterministic)**: No random behavior, fixed timeout values, consistent pagination order. Same input = same output verified with tests.

6. **Task 6 (Integration)**: `HTTPPolling` implements `input.Module` interface. `Fetch()` returns `[]map[string]interface{}` compatible with `Executor.Execute()`. Integration tests verify end-to-end flow.

### File List

**New Files:**
- `cannectors-runtime/internal/modules/input/http_polling.go` - HTTPPolling implementation (~550 lines)
- `cannectors-runtime/internal/modules/input/http_polling_test.go` - Unit and integration tests (~1300 lines)

**Modified Files:**
- `cannectors-runtime/internal/modules/input/input.go` - Cleaned up stub, kept Module interface
- `cannectors-runtime/internal/runtime/pipeline.go` - Updated to work with HTTPPolling input module
- `cannectors-runtime/internal/runtime/pipeline_test.go` - Updated tests to verify HTTPPolling integration

### Change Log

- 2026-01-17: Story 3.1 implementation complete - HTTP Polling Input module with authentication, pagination, error handling, and integration tests
