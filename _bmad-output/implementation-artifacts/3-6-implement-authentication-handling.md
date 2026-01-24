# Story 3.6: Implement Authentication Handling

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,  
I want the runtime to handle authentication (API key, OAuth2 basic),  
So that connectors can authenticate with source and target systems.

## Acceptance Criteria

**Given** I have a connector with authentication configured (API key or OAuth2 basic)  
**When** The runtime executes Input or Output modules  
**Then** The runtime adds API key to request headers if configured (FR44, FR45)  
**And** The runtime handles OAuth2 basic authentication flow if configured  
**And** The runtime securely stores and retrieves credentials (NFR15)  
**And** The runtime handles authentication errors gracefully  
**And** The runtime supports different authentication methods per module  
**And** Credentials are encrypted at rest (NFR15)

## Tasks / Subtasks

- [x] Task 1: Extract shared authentication package (AC: supports different authentication methods per module)
  - [x] Create `internal/auth/` package for shared authentication handling
  - [x] Define `AuthHandler` interface for authentication application
  - [x] Move authentication logic from input/http_polling.go to shared package
  - [x] Move authentication logic from output/http_request.go to shared package
  - [x] Support API key authentication (header and query parameter locations)
  - [x] Support Bearer token authentication
  - [x] Support Basic authentication (username/password)
  - [x] Support OAuth2 client credentials flow with token caching
  - [x] Add unit tests for authentication package
- [x] Task 2: Implement OAuth2 token management (AC: handles OAuth2 basic authentication flow)
  - [x] Implement OAuth2 client credentials token request
  - [x] Implement token caching with expiration tracking
  - [x] Implement thread-safe token refresh mechanism
  - [x] Handle token expiration and automatic refresh
  - [x] Handle token request failures gracefully
  - [x] Support concurrent token requests (prevent duplicate token fetches)
  - [x] Add unit tests for OAuth2 token management
- [x] Task 3: Implement credential security (AC: securely stores and retrieves credentials)
  - [x] Ensure credentials are encrypted at rest in configuration (handled by config layer)
  - [x] Decrypt credentials at runtime before use (config layer decrypts before passing to auth package)
  - [x] Never log credential values in logs
  - [x] Credential rotation requires module restart (no hot-reload - by design)
  - [x] Add security tests for credential handling
- [x] Task 4: Implement API key authentication (AC: adds API key to request headers if configured)
  - [x] Support API key in header location (e.g., X-API-Key)
  - [x] Support API key in query parameter location (e.g., ?api_key=...)
  - [x] Support custom header/parameter names
  - [x] Handle missing API key configuration gracefully
  - [x] Add unit tests for API key authentication
- [x] Task 5: Update Input modules to use shared authentication (AC: runtime handles authentication in Input modules)
  - [x] Refactor HTTPPolling module to use shared auth package
  - [x] Refactor Webhook module to use shared auth package (if needed)
  - [x] Ensure backward compatibility with existing configurations
  - [x] Update tests to use shared authentication
  - [x] Verify Input modules still work with authentication
- [x] Task 6: Update Output modules to use shared authentication (AC: runtime handles authentication in Output modules)
  - [x] Refactor HTTPRequest module to use shared auth package
  - [x] Ensure backward compatibility with existing configurations
  - [x] Update tests to use shared authentication
  - [x] Verify Output modules still work with authentication
- [x] Task 7: Implement authentication error handling (AC: handles authentication errors gracefully)
  - [x] Handle API key authentication errors with clear messages
  - [x] Handle OAuth2 token request failures
  - [x] Handle OAuth2 token refresh failures
  - [x] Handle invalid authentication configuration
  - [x] Provide structured error messages with context
  - [x] Add error handling tests
- [x] Task 8: Ensure deterministic authentication (AC: execution is deterministic)
  - [x] Ensure same credentials = same authentication headers
  - [x] Ensure OAuth2 token caching doesn't introduce non-determinism
  - [x] Ensure authentication is applied consistently across requests
  - [x] No random behavior in authentication handling
  - [x] Add tests to verify deterministic authentication

## Dev Notes

### Architecture Requirements

**Authentication Handling:**
- **Location:** `canectors-runtime/internal/auth/auth.go` (new shared package)
- **Purpose:** Centralized authentication handling for Input and Output modules
- **Scope:** API key, Bearer token, Basic auth, OAuth2 client credentials

**Module Integration:**
- Input modules: HTTPPolling, Webhook (if needed)
- Output modules: HTTPRequest
- Both use shared `auth` package for consistent authentication handling

**Configuration Structure:**
- Authentication configuration is in `connector.AuthConfig` type (already defined in `pkg/connector/types.go`)
- Authentication configuration includes:
  - `type`: "api-key", "bearer", "basic", "oauth2"
  - `apiKey`: For API key auth - `location`: "header" or "query", `name`: header/param name, `value`: credential value
  - `bearer`: For Bearer token - `token`: token value
  - `basic`: For Basic auth - `username`: username, `password`: password
  - `oauth2`: For OAuth2 - `clientId`, `clientSecret`, `tokenUrl`

**Authentication Types Supported:**
- **API Key:** Header or query parameter location
- **Bearer Token:** Authorization header with Bearer token
- **Basic Auth:** HTTP Basic Authentication (username:password)
- **OAuth2 Client Credentials:** Client credentials flow with token caching

**OAuth2 Token Management:**
- Token request: POST to tokenUrl with clientId/clientSecret
- Token storage: In-memory cache with expiration time (per module instance)
- Token refresh: Check expiration before each request, refresh if needed
- Thread-safety: Use mutex for concurrent token access
- Error handling: If token request fails, return error (don't retry token request indefinitely)

**Credential Security:**
- Credentials encrypted at rest in configuration (handled by config layer)
  - The config layer (internal/config) handles encryption/decryption of credentials
  - Runtime receives decrypted credentials via connector.AuthConfig
  - No encryption logic in auth package (separation of concerns)
- Credentials decrypted at runtime before use (by config layer, before reaching auth package)
- Never log credential values (log authentication type only)
  - Error messages never contain credential values
  - OAuth2 token response bodies never logged (may contain sensitive data)
  - Security tests verify no credential leakage
- Credential rotation: Configuration changes require module restart (no hot-reload)
  - Runtime does not support changing credentials during execution
  - To rotate credentials: update config file and restart runtime

**Error Handling:**
- Invalid authentication configuration → clear error message
- Authentication failure → structured error with context
- OAuth2 token request failure → error with tokenUrl and status code
- Missing credentials → clear error indicating missing field

**Deterministic Execution:**
- Same credentials + same configuration = same authentication headers
- OAuth2 token caching doesn't introduce randomness (same token reused until expiration)
- Authentication applied consistently across all requests

### Project Structure Notes

**File Organization:**
```
canectors-runtime/
├── internal/
│   ├── auth/                      # NEW: Shared authentication package
│   │   ├── auth.go                # AuthHandler interface and implementations
│   │   ├── oauth2.go              # OAuth2 token management
│   │   ├── auth_test.go           # Authentication tests
│   │   └── oauth2_test.go         # OAuth2 tests
│   └── modules/
│       ├── input/
│       │   ├── http_polling.go    # UPDATED: Use shared auth package
│       │   └── webhook.go         # UPDATED: Use shared auth package (if needed)
│       └── output/
│           └── http_request.go    # UPDATED: Use shared auth package
```

**Integration Points:**
- Authentication will be used by:
  - `HTTPPolling` input module (Story 3.1)
  - `Webhook` input module (Story 3.2, if authentication needed)
  - `HTTPRequest` output module (Story 3.5)
- Authentication depends on:
  - `pkg/connector.AuthConfig` type (already defined)
  - `pkg/connector.ModuleConfig` type (already defined)
  - Logger: `internal/logger` package (already available)
  - HTTP client: Go standard library `net/http`

**Module Instantiation:**
- Auth handler created from `connector.AuthConfig`
- Constructor: `auth.NewHandler(config *connector.AuthConfig) (Handler, error)`
- Configuration validation happens in constructor
- OAuth2 token cache initialized in constructor (empty, fetches on first use)

**Authentication Handler Design:**
- **Interface:** `Handler` interface with `ApplyAuth(ctx context.Context, req *http.Request) error`
- **Implementations:** 
  - `APIKeyHandler` - API key in header or query
  - `BearerHandler` - Bearer token in header
  - `BasicHandler` - Basic auth in header
  - `OAuth2Handler` - OAuth2 with token caching
- **Thread-safety:** OAuth2 handler uses mutex for token cache access
- **Error handling:** All handlers return structured errors

### Previous Story Intelligence

**From Story 3.5 (HTTP Request Output):**
- Authentication currently implemented directly in HTTPRequest module
- OAuth2 token caching with mutex for thread-safety
- Support for API key, Bearer token, Basic auth, OAuth2
- Error handling patterns: structured errors with context
- Testing patterns: Unit tests with mock HTTP servers
- File location: `internal/modules/output/http_request.go`

**From Story 3.1 (HTTP Polling Input):**
- Authentication currently implemented directly in HTTPPolling module
- OAuth2 token caching with expiration tracking
- Support for API key, Bearer token, Basic auth, OAuth2
- Similar patterns to output module (duplication to be eliminated)
- File location: `internal/modules/input/http_polling.go`

**From Story 3.4 (Condition Filter):**
- Module interface pattern: Interface-based design for extensibility
- Configuration validation in constructor
- Error handling with structured context logging
- Deterministic execution requirements
- Testing patterns: Unit tests + integration tests

**From Story 3.3 (Mapping Filter):**
- Module configuration validation patterns
- Error handling with structured context
- Integration with pipeline executor via interface
- Testing patterns: Unit tests + integration tests

**Key Learnings:**
- Extract shared functionality to prevent duplication (auth handling duplicated in input/output)
- Use interface-based design for extensibility
- Thread-safety is critical for OAuth2 token caching (multiple goroutines may access)
- Structured error handling helps debugging
- Test authentication scenarios thoroughly (API key, OAuth2, errors)
- Maintain backward compatibility when refactoring

### Testing Requirements

**Unit Tests:**
- API key authentication (header location)
- API key authentication (query parameter location)
- Bearer token authentication
- Basic authentication
- OAuth2 token request (success and failure)
- OAuth2 token caching (expiration and refresh)
- OAuth2 thread-safety (concurrent token requests)
- Missing credential handling
- Invalid authentication configuration
- Error handling for all authentication types
- Deterministic authentication (same input = same output)

**Integration Tests:**
- Input module with authentication (HTTPPolling)
- Output module with authentication (HTTPRequest)
- Pipeline execution with authenticated Input → Output
- OAuth2 token refresh during pipeline execution
- Error propagation from authentication failures

**Test Data:**
- Create test fixtures in `/internal/auth/testdata/`:
  - `valid-api-key-config.json` - Valid API key configuration
  - `valid-bearer-config.json` - Valid Bearer token configuration
  - `valid-basic-config.json` - Valid Basic auth configuration
  - `valid-oauth2-config.json` - Valid OAuth2 configuration
  - `invalid-auth-config.json` - Invalid authentication configuration

**Mock HTTP Server:**
- Use `net/http/httptest` package for testing
- Create test server that validates authentication
- Create OAuth2 token endpoint for testing
- Test server that simulates authentication failures

### References

- **Source:** `canectors-BMAD/_bmad-output/planning-artifacts/epics.md#Story-3.6` - Story requirements and acceptance criteria
- **Source:** `canectors-BMAD/_bmad-output/planning-artifacts/architecture.md` - Architecture patterns, authentication requirements, security requirements, deterministic execution requirements
- **Source:** `canectors-runtime/pkg/connector/types.go` - AuthConfig type definition
- **Source:** `canectors-runtime/internal/modules/input/http_polling.go` - Current authentication implementation in Input module
- **Source:** `canectors-runtime/internal/modules/output/http_request.go` - Current authentication implementation in Output module
- **Source:** `canectors-runtime/internal/config/schema/pipeline-schema.json` - Authentication configuration schema
- **Source:** `canectors-BMAD/_bmad-output/project-context.md` - Go runtime patterns, testing standards, code organization, security best practices
- **Source:** `canectors-BMAD/_bmad-output/implementation-artifacts/3-5-implement-output-module-execution-http-request.md` - Previous story learnings and authentication patterns
- **Source:** `canectors-BMAD/_bmad-output/implementation-artifacts/3-1-implement-input-module-execution-http-polling.md` - Previous story learnings and authentication patterns
- **Source:** `canectors-BMAD/_bmad-output/implementation-artifacts/3-4-implement-filter-module-execution-conditions.md` - Previous story learnings and patterns

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Debug Log References

- All tests pass: `go test ./...` shows 100% pass rate
- Linter clean: `golangci-lint run` reports 0 issues

### Completion Notes List

1. **Task 1-4**: Created new `internal/auth/` package with:
   - `auth.go`: Handler interface + APIKey, Bearer, Basic implementations
   - `oauth2.go`: OAuth2 client credentials with thread-safe token caching (RWMutex)
   - `auth_test.go`: 20+ unit tests for all auth types + security tests
   - `oauth2_test.go`: 12+ OAuth2 tests (caching, expiry, concurrency, errors, invalidation)

2. **Task 5**: Refactored `http_polling.go`:
   - Removed 150+ lines of duplicated auth code
   - Now uses shared `auth.Handler` interface
   - Automatic OAuth2 token invalidation on 401 Unauthorized responses
   - Updated test to check `X-API-Key` header (correct behavior vs old `Authorization: Bearer`)

3. **Task 6**: Refactored `http_request.go`:
   - Removed 180+ lines of duplicated auth code
   - Now uses shared `auth.Handler` interface
   - Automatic OAuth2 token invalidation and retry on 401 Unauthorized
   - Removed unused `sync.RWMutex` and OAuth2 token fields
   - Updated 5 tests to expect validation at creation time (fail-fast)

4. **Task 7**: Auth error handling via structured errors:
   - `ErrMissingAPIKey`, `ErrMissingBearerToken`, `ErrMissingBasicAuth`, `ErrMissingOAuth2Creds`
   - OAuth2 errors include status code but never log response body (may contain secrets)
   - Error messages never contain credential values (security tests verify)

5. **Task 8**: Deterministic auth verified via:
   - `TestDeterministicAuth` in auth_test.go
   - `TestOAuth2Handler_DeterministicWithCache` in oauth2_test.go
   - Same credentials + config = identical auth headers

6. **Code Review Fixes (Post-Implementation)**:
   - Added automatic OAuth2 token refresh on 401 Unauthorized (both Input and Output modules)
   - Added security tests to verify credentials never exposed in error messages
   - Improved OAuth2 error handling documentation
   - Clarified credential encryption handled by config layer (not auth package)
   - Documented credential rotation requires restart (no hot-reload support)

**Design Decision**: Kept custom OAuth2 implementation instead of `golang.org/x/oauth2` library:
- Existing code already working and tested
- Simple use case (client credentials only)
- Avoids additional dependency for ~100 lines of code

**Security Notes**:
- Credentials encrypted at rest by config layer (internal/config package)
- Runtime receives decrypted credentials, but never logs them
- Security tests verify no credential leakage in error messages
- OAuth2 token response bodies never logged (security best practice)

### File List

**New Files:**
- `canectors-runtime/internal/auth/auth.go`
- `canectors-runtime/internal/auth/oauth2.go`
- `canectors-runtime/internal/auth/auth_test.go`
- `canectors-runtime/internal/auth/oauth2_test.go`

**Modified Files:**
- `canectors-runtime/internal/modules/input/http_polling.go` (removed auth code, uses shared package)
- `canectors-runtime/internal/modules/input/http_polling_test.go` (updated API key header test)
- `canectors-runtime/internal/modules/output/http_request.go` (removed auth code, uses shared package)
- `canectors-runtime/internal/modules/output/http_request_test.go` (updated validation timing tests)

## Change Log

- 2026-01-21: Story 3.6 implementation complete - extracted shared auth package, refactored Input/Output modules
- 2026-01-21: Code review fixes - Added OAuth2 automatic token refresh on 401, security tests, improved documentation
