# Story 4.4: Implement Error Handling and Retry Logic

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,  
I want the runtime to handle errors and retry failed operations,  
So that connector executions are robust and reliable.

## Acceptance Criteria

**Given** I have a connector with error handling and retry configuration  
**When** An error occurs during execution  
**Then** The runtime detects and categorizes errors (network, authentication, validation, etc.)  
**And** The runtime applies retry logic for transient errors (FR46, FR28)  
**And** The runtime stops execution for fatal errors  
**And** The runtime logs all errors with context (NFR32)  
**And** The runtime does not cause data loss on errors (NFR33)  
**And** The runtime handles timeouts and connection errors gracefully  
**And** The retry logic is configurable per module (FR28)

## Tasks / Subtasks

- [x] Task 1: Enhance error categorization and classification (AC: detects and categorizes errors)
  - [x] Create error types/categories: NetworkError, AuthenticationError, ValidationError, FatalError, TransientError
  - [x] Implement error classification function to categorize errors by type
  - [x] Classify HTTP errors by status code (4xx vs 5xx, retryable vs non-retryable)
  - [x] Classify network errors (timeout, connection refused, DNS errors)
  - [x] Classify authentication errors (401, 403) as fatal (no retry)
  - [x] Classify validation errors (400, 422) as fatal (no retry)
  - [x] Classify server errors (500, 502, 503, 504) as transient (retryable)
  - [x] Classify rate limiting (429) as transient (retryable with backoff)
  - [x] Add unit tests for error classification
- [x] Task 2: Implement retry configuration parsing and resolution (AC: retry logic configurable per module)
  - [x] Parse retry configuration from pipeline schema (module > defaults > errorHandling precedence)
  - [x] Resolve retry config for each module (Input, Filter, Output)
  - [x] Support retry configuration fields: maxAttempts, delayMs, backoffMultiplier, maxDelayMs, retryableStatusCodes
  - [x] Apply default retry config if not specified (maxAttempts=3, delayMs=1000, backoffMultiplier=2, maxDelayMs=30000)
  - [x] Validate retry configuration (maxAttempts 0-10, delays >= 0, backoffMultiplier >= 1)
  - [x] Add unit tests for retry config parsing and resolution
- [x] Task 3: Implement exponential backoff retry mechanism (AC: applies retry logic for transient errors)
  - [x] Create retry executor with exponential backoff algorithm
  - [x] Calculate retry delay: min(delayMs * (backoffMultiplier ^ attempt), maxDelayMs)
  - [x] Implement retry loop with attempt tracking
  - [x] Support configurable maxAttempts (0 = no retry)
  - [x] Support configurable retryableStatusCodes (default: [429, 500, 502, 503, 504])
  - [x] Retry only on transient errors (not on fatal errors)
  - [x] Log each retry attempt with attempt number and delay
  - [x] Add unit tests for retry mechanism with various configurations
- [x] Task 4: Integrate retry logic into Input module execution (AC: retry logic configurable per module)
  - [x] Apply retry logic to HTTP Polling Input module
  - [x] Retry on network errors (timeout, connection refused)
  - [x] Retry on retryable HTTP status codes (500, 502, 503, 504, 429)
  - [x] Do not retry on fatal errors (400, 401, 403, 404, 422)
  - [x] Use module-specific retry config or fallback to defaults
  - [x] Log retry attempts with context (module, attempt, delay, error)
  - [x] Add unit tests for Input module retry logic
- [x] Task 5: Integrate retry logic into Output module execution (AC: retry logic configurable per module)
  - [x] Apply retry logic to HTTP Request Output module
  - [x] Retry on network errors (timeout, connection refused)
  - [x] Retry on retryable HTTP status codes (500, 502, 503, 504, 429)
  - [x] Do not retry on fatal errors (400, 401, 403, 404, 422)
  - [x] Use module-specific retry config or fallback to defaults
  - [x] Handle batch mode retries (retry entire batch or individual records)
  - [x] Handle single record mode retries (retry individual record)
  - [x] Log retry attempts with context (module, attempt, delay, error, record info)
  - [x] Add unit tests for Output module retry logic
- [x] Task 6: Implement error handling strategies (onError: fail, skip, log) (AC: stops execution for fatal errors)
  - [x] Parse onError configuration from module config or defaults
  - [x] Implement "fail" strategy: stop execution, return error (default)
  - [x] Implement "skip" strategy: skip failed record/module, continue execution
  - [x] Implement "log" strategy: log error, continue execution (non-fatal)
  - [x] Apply error handling strategy based on error category (fatal vs transient)
  - [x] Fatal errors always use "fail" strategy (cannot skip or log-only)
  - [x] Transient errors respect configured onError strategy
  - [x] Log error handling decisions (strategy applied, error category)
  - [x] Add unit tests for error handling strategies
- [x] Task 7: Enhance error logging with full context (AC: logs all errors with context, NFR32)
  - [x] Log error category (network, authentication, validation, fatal, transient)
  - [x] Log error classification details (HTTP status, error type, retryable status)
  - [x] Log retry attempts with full context (attempt number, delay, backoff calculation)
  - [x] Log error handling strategy applied (fail, skip, log)
  - [x] Log module context (Input, Filter, Output) with error
  - [x] Log record context for failed records (record index, record data snippet)
  - [x] Log execution context (pipeline ID, stage, timing)
  - [x] Use structured logging with all context fields
  - [x] Ensure error logs are actionable (clear what went wrong and where)
  - [x] Add tests for error logging with context
- [x] Task 8: Implement timeout handling (AC: handles timeouts gracefully)
  - [x] Parse timeout configuration from module config or defaults (timeoutMs)
  - [x] Apply timeout to HTTP requests (Input and Output modules)
  - [x] Create context with timeout for each HTTP request
  - [x] Detect timeout errors and classify as transient (retryable)
  - [x] Log timeout errors with context (module, timeout duration, endpoint)
  - [x] Handle timeout errors with retry logic (if configured)
  - [x] Add unit tests for timeout handling
- [x] Task 9: Ensure no data loss on errors (AC: does not cause data loss on errors, NFR33)
  - [x] Verify Input module does not lose data on retry (re-fetch on retry)
  - [x] Verify Filter module does not lose data on error (process all records before error)
  - [x] Verify Output module handles partial failures correctly (batch mode: retry failed records)
  - [x] Implement idempotency checks where possible (prevent duplicate sends)
  - [x] Log data integrity checks (records processed, records failed, records retried)
  - [x] Add integration tests for data integrity on errors
- [x] Task 10: Update executor to orchestrate error handling and retry (AC: complete implementation)
  - [x] Integrate error classification into executor
  - [x] Integrate retry logic into executor for all modules
  - [x] Apply error handling strategies (fail, skip, log) in executor
  - [x] Coordinate retry logic across Input → Filter → Output flow
  - [x] Handle errors at each stage appropriately (Input errors stop, Filter errors stop, Output errors retry)
  - [x] Update ExecutionResult to include retry information (retry attempts, retry delays)
  - [x] Update ExecutionResult to include error details (error category, error type, error message)
  - [x] Add integration tests for complete pipeline error handling
- [x] Task 11: Update documentation and examples (AC: complete implementation)
  - [x] Update README.md with error handling and retry documentation:
    - Error categorization and classification
    - Retry configuration (maxAttempts, delayMs, backoffMultiplier, maxDelayMs, retryableStatusCodes)
    - Error handling strategies (fail, skip, log)
    - Timeout configuration
    - Examples of retry configurations
    - Examples of error handling scenarios
  - [x] Add example configurations with retry settings
  - [x] Add troubleshooting section for common error scenarios
  - [x] Update CLI help text if needed

## Dev Notes

### Architecture Requirements

**Error Handling and Retry Logic Implementation:**
- **Location:** `cannectors-runtime/internal/runtime/pipeline.go` (Executor), `cannectors-runtime/internal/modules/` (modules), `cannectors-runtime/pkg/connector/types.go` (types)
- **Purpose:** Handle errors robustly and retry failed operations for reliable connector executions
- **Scope:** All pipeline execution stages (Input, Filter, Output) with configurable retry and error handling

**Current State:**
- Pipeline schema already defines retry configuration structure (retryConfig with maxAttempts, delayMs, backoffMultiplier, maxDelayMs, retryableStatusCodes)
- Pipeline schema defines error handling (onError: fail, skip, log) and timeout (timeoutMs)
- `ErrorHandling` type exists in `pkg/connector/types.go` but is basic (RetryCount, RetryDelay, OnError)
- Executor has basic error handling but no retry logic
- Output module (HTTP Request) may have some retry logic but needs enhancement
- **Missing:** Error categorization, retry configuration parsing, exponential backoff, error handling strategies, comprehensive error logging

**Integration Points:**
- Executor orchestrates error handling and retry across all modules
- Modules (Input, Filter, Output) use retry logic for their operations
- Logger package used for structured error logging
- Configuration parser resolves retry config from pipeline schema

**Existing Code Structure:**
- `pkg/connector/types.go` - Pipeline types with ErrorHandling struct
- `internal/runtime/pipeline.go` - Executor with basic error handling
- `internal/modules/input/http_polling.go` - HTTP Polling Input module
- `internal/modules/output/http_request.go` - HTTP Request Output module
- `internal/config/schema/pipeline-schema.json` - Schema with retry configuration
- `internal/logger/logger.go` - Structured logging package

### Project Structure Notes

**File Locations:**
- Error types and classification: `cannectors-runtime/internal/runtime/errors.go` (new file)
- Retry executor: `cannectors-runtime/internal/runtime/retry.go` (new file)
- Executor implementation: `cannectors-runtime/internal/runtime/pipeline.go`
- Executor tests: `cannectors-runtime/internal/runtime/pipeline_test.go`
- Module implementations: `cannectors-runtime/internal/modules/{input,filter,output}/`
- Pipeline types: `cannectors-runtime/pkg/connector/types.go`
- Configuration parser: `cannectors-runtime/internal/config/converter.go`

**Dependencies:**
- Use existing `github.com/cannectors/runtime/pkg/connector` for types
- Use existing `github.com/cannectors/runtime/internal/runtime` for Executor
- Use existing `github.com/cannectors/runtime/internal/logger` for logging
- Use existing `github.com/cannectors/runtime/internal/modules/*` for modules
- Use existing `github.com/cannectors/runtime/internal/config` for configuration parsing

### Technical Requirements

**Error Categorization:**
- **Network Errors:** Timeout, connection refused, DNS errors, network unreachable (transient, retryable)
- **Authentication Errors:** 401 Unauthorized, 403 Forbidden (fatal, not retryable)
- **Validation Errors:** 400 Bad Request, 422 Unprocessable Entity (fatal, not retryable)
- **Server Errors:** 500 Internal Server Error, 502 Bad Gateway, 503 Service Unavailable, 504 Gateway Timeout (transient, retryable)
- **Rate Limiting:** 429 Too Many Requests (transient, retryable with backoff)
- **Not Found:** 404 Not Found (fatal, not retryable)
- **Other 4xx:** Client errors (fatal, not retryable)
- **Other 5xx:** Server errors (transient, retryable)

**Retry Configuration:**
- **Precedence:** module.retry > moduleDefaults.retry > errorHandling.retry
- **maxAttempts:** Maximum retry attempts (0 = no retry, default: 3, max: 10)
- **delayMs:** Initial delay between retries in milliseconds (default: 1000)
- **backoffMultiplier:** Multiplier for exponential backoff (default: 2, min: 1, max: 10)
- **maxDelayMs:** Maximum delay between retries (default: 30000)
- **retryableStatusCodes:** HTTP status codes that trigger retry (default: [429, 500, 502, 503, 504])

**Exponential Backoff Algorithm:**
- Calculate delay: `min(delayMs * (backoffMultiplier ^ attempt), maxDelayMs)`
- Example: delayMs=1000, backoffMultiplier=2, maxDelayMs=30000
  - Attempt 1: 1000ms
  - Attempt 2: 2000ms
  - Attempt 3: 4000ms
  - Attempt 4: 8000ms
  - Attempt 5: 16000ms
  - Attempt 6+: 30000ms (capped)

**Error Handling Strategies:**
- **fail:** Stop execution, return error (default for fatal errors)
- **skip:** Skip failed record/module, continue execution (for transient errors)
- **log:** Log error, continue execution (for non-fatal errors)

**Timeout Handling:**
- Parse timeout from module config or defaults (timeoutMs)
- Apply timeout to HTTP requests using context.WithTimeout
- Detect timeout errors and classify as transient (retryable)
- Log timeout errors with context

**Data Integrity:**
- Input module: Re-fetch data on retry (no data loss)
- Filter module: Process all records before error (no partial processing)
- Output module: Retry failed records in batch mode (no data loss)
- Implement idempotency checks where possible

**Testing Requirements:**
- Unit tests for error classification
- Unit tests for retry configuration parsing
- Unit tests for exponential backoff algorithm
- Unit tests for retry mechanism
- Unit tests for error handling strategies
- Unit tests for timeout handling
- Integration tests for complete pipeline error handling
- Integration tests for data integrity on errors

### Library and Framework Requirements

**Go Standard Library:**
- `context` - Request context with timeout and cancellation
- `time` - Timing, delays, and duration calculations
- `errors` - Error wrapping and unwrapping
- `fmt` - Formatted output for error messages
- `net/http` - HTTP client with timeout support

**Existing Dependencies (already in go.mod):**
- `github.com/cannectors/runtime/pkg/connector` - Pipeline and execution result types
- `github.com/cannectors/runtime/internal/runtime` - Executor
- `github.com/cannectors/runtime/internal/modules/*` - Module interfaces
- `github.com/cannectors/runtime/internal/logger` - Logger package
- `github.com/cannectors/runtime/internal/config` - Configuration parsing

**No New Dependencies Required:**
- All functionality can be implemented using existing dependencies
- Standard library provides timeout, context, and error handling
- Exponential backoff can be implemented with standard library

### Previous Story Intelligence

**From Story 4.3 (Execution Logging):**
- Logger package has comprehensive execution logging helpers (WithExecution, LogExecutionStart, LogStageStart, LogStageEnd, LogExecutionEnd, LogMetrics, LogError)
- Error logging should use LogError helper with full context
- Structured logging with consistent field names (pipeline_id, stage, module_type, error, error_code)
- Error logs must include sufficient context for debugging (NFR32)
- All error logs should be machine-readable (JSON format)

**From Story 4.2 (Dry-Run Mode):**
- Executor already has error handling for dry-run mode
- Output module has preview functionality that should handle errors gracefully
- Error reporting in dry-run mode should be clear and actionable

**From Story 4.1 (CRON Scheduler):**
- Scheduler uses executor for pipeline execution
- Scheduled executions should handle errors gracefully (don't stop scheduler)
- Error handling should work correctly in scheduled executions

**From Story 3.6 (Authentication Handling):**
- Authentication is handled in `internal/auth/` package
- Authentication errors (401, 403) should be classified as fatal (no retry)
- Authentication headers should be masked in error logs (security)

**From Story 3.5 (Output Module Execution):**
- HTTP Request module has basic retry logic but needs enhancement
- Output module supports batch and single record modes
- Retry logic should handle both batch mode (retry entire batch) and single record mode (retry individual record)
- Output module already has some error handling but needs comprehensive retry

**From Story 3.1-3.4 (Module Execution):**
- Input modules (HTTPPolling, Webhook) are implemented
- Filter modules (Mapping, Conditions) are implemented
- All modules should use structured error logging
- Module execution errors should be logged with context

**From Story 2.3 (Pipeline Orchestration):**
- `runtime.Executor` orchestrates Input → Filter → Output flow
- Executor already has basic error handling (returns ExecutionResult with status and errors)
- Executor error handling is comprehensive but needs retry logic
- Executor returns structured `ExecutionResult` with status and metrics

**From Story 2.1 (Go CLI Project Structure):**
- Project structure follows Go best practices
- Test structure co-located with source files
- Logger package already initialized in project structure

### Git Intelligence Summary

**Recent Work Patterns:**
- Execution logging implementation (Story 4.3) - comprehensive logging with context
- Dry-run mode implementation (Story 4.2) - preview and error reporting
- CRON scheduler implementation (Story 4.1) - scheduled execution with error handling
- Authentication package extracted to `internal/auth/` (Story 3.6)
- Module execution implemented with shared authentication (Stories 3.1-3.6)
- Pipeline executor implemented with Input/Filter/Output orchestration (Story 2.3)

**Code Patterns Established:**
- Use structured logging with `internal/logger` package
- Use error wrapping with context: `fmt.Errorf("context: %w", err)`
- Use interfaces for module abstraction (input.Module, filter.Module, output.Module)
- Use context.Context for cancellation and timeouts
- Follow Go naming conventions (PascalCase for exported, camelCase for unexported)
- Return structured results with error information
- Use `slog` structured logging with JSON output

**Files Recently Modified:**
- `internal/logger/logger.go` - Comprehensive execution logging helpers
- `internal/runtime/pipeline.go` - Pipeline executor with logging integration
- `internal/modules/output/http_request.go` - HTTP Request output module
- `internal/modules/input/http_polling.go` - HTTP Polling input module
- `internal/modules/filter/mapping.go` - Mapping filter module
- `internal/modules/filter/condition.go` - Condition filter module

### Latest Technical Information

**Go Error Handling Best Practices:**
- Use error wrapping: `fmt.Errorf("context: %w", err)`
- Use errors.Is() and errors.As() for error checking
- Create custom error types for different error categories
- Include context in error messages (what, where, why)
- Use structured error logging with all context fields

**Exponential Backoff Best Practices:**
- Start with small delay and increase exponentially
- Cap maximum delay to prevent excessive waits
- Add jitter to prevent thundering herd (optional, not required for MVP)
- Log each retry attempt with attempt number and delay
- Respect maxAttempts configuration (0 = no retry)

**HTTP Error Handling Best Practices:**
- Classify HTTP errors by status code (4xx vs 5xx)
- Retry on transient errors (5xx, 429)
- Do not retry on client errors (4xx except 429)
- Use exponential backoff for retries
- Log HTTP errors with status code and response body snippet

**Timeout Handling Best Practices:**
- Use context.WithTimeout for HTTP requests
- Set reasonable timeout values (default: 30 seconds)
- Detect timeout errors and classify as transient
- Retry on timeout errors (if configured)
- Log timeout errors with timeout duration

**Data Integrity Best Practices:**
- Re-fetch data on retry (Input modules)
- Process all records before error (Filter modules)
- Retry failed records in batch mode (Output modules)
- Implement idempotency checks where possible
- Log data integrity information (records processed, failed, retried)

### Project Context Reference

**From project-context.md:**
- Go Runtime CLI is separate project from Next.js application
- Cross-platform compilation required (Windows, Mac, Linux)
- Follow Go best practices and conventions
- Use `golangci-lint run` after implementation to verify linting passes
- Runtime must be deterministic (NFR24, NFR25)
- **Go Runtime CLI:** Run `golangci-lint run` at the end of each implementation to ensure code quality

**From Architecture:**
- Error handling and retry logic is part of runtime CLI (Epic 2, Part 3/3 - Advanced Runtime Features)
- Runtime must handle errors robustly (FR46, FR28)
- Runtime must not cause data loss on errors (NFR33)
- Runtime must log errors with sufficient context (NFR32)

**From PRD:**
- FR46: System can execute connectors with error handling and retry logic
- FR28: System can generate connector declarations with retry logic configurations
- FR48: System can detect and report mapping errors during execution
- NFR30: System must recover automatically from transient errors
- NFR31: System must handle errors robustly and explicitly
- NFR32: Errors must be logged with sufficient context for debugging
- NFR33: Errors during connector execution must not cause data loss

**From Epics:**
- Epic 4: Advanced Runtime Features (Part 3/3 of Epic 2 from Architecture)
- Story 4.4 is fourth story in Epic 4 (after 4.1 CRON Scheduler, 4.2 Dry-Run Mode, 4.3 Execution Logging)
- Story 4.4 enables robust error handling and retry logic for reliable connector executions
- Subsequent stories: 4.5 (CLI Commands), 4.6 (Cross-Platform)

### References

- **Architecture Decision:** [Source: _bmad-output/planning-artifacts/architecture.md#Runtime CLI Architecture]
- **Epic Definition:** [Source: _bmad-output/planning-artifacts/epics.md#Epic 4: Advanced Runtime Features]
- **Story Requirements:** [Source: _bmad-output/planning-artifacts/epics.md#Story 4.4: Implement Error Handling and Retry Logic]
- **Pipeline Schema:** [Source: cannectors-runtime/internal/config/schema/pipeline-schema.json#retryConfig]
- **Pipeline Types:** [Source: cannectors-runtime/pkg/connector/types.go#ErrorHandling]
- **Executor Implementation:** [Source: cannectors-runtime/internal/runtime/pipeline.go]
- **Logger Implementation:** [Source: cannectors-runtime/internal/logger/logger.go]
- **Output Module:** [Source: cannectors-runtime/internal/modules/output/http_request.go]
- **Input Module:** [Source: cannectors-runtime/internal/modules/input/http_polling.go]
- **Project Context:** [Source: _bmad-output/project-context.md]

## Dev Agent Record

### Agent Model Used

Code review follow-up (fixes applied by Dev Agent).

### Debug Log References

(Optional.)

### Completion Notes List

- **Code review fixes (2026-01-22):** ExecutionResult extended with RetryInfo, ErrorCategory, ErrorType; converter implements module > defaults > errorHandling and retryConfig parsing; errhandling integrated into executor (buildExecutionError, RetryInfoProvider); HTTP Polling timeoutMs + backward-compat timeout (seconds); retry_test subtest name fix (attempt 10); README updated (4.4, errhandling, error/retry docs, project structure); File List and placeholders updated.

### File List

- `cannectors-runtime/README.md`
- `cannectors-runtime/internal/config/converter.go`
- `cannectors-runtime/internal/errhandling/errors.go`
- `cannectors-runtime/internal/errhandling/errors_test.go`
- `cannectors-runtime/internal/errhandling/retry.go`
- `cannectors-runtime/internal/errhandling/retry_test.go`
- `cannectors-runtime/internal/modules/input/http_polling.go`
- `cannectors-runtime/internal/modules/output/http_request.go`
- `cannectors-runtime/internal/modules/output/http_request_test.go`
- `cannectors-runtime/internal/runtime/errors.go`
- `cannectors-runtime/internal/runtime/errors_test.go`
- `cannectors-runtime/internal/runtime/retry.go`
- `cannectors-runtime/internal/runtime/retry_test.go`
- `cannectors-runtime/internal/runtime/pipeline.go`
- `cannectors-runtime/pkg/connector/types.go`
- `_bmad-output/implementation-artifacts/4-4-implement-error-handling-and-retry-logic.md`
