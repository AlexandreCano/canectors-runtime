# Story 4.2: Implement Dry-Run Mode

Status: review

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,  
I want to execute connectors in dry-run mode,  
So that I can validate configurations without side effects on target systems.

## Acceptance Criteria

**Given** I have a connector configuration  
**When** I execute the connector in dry-run mode (FR38)  
**Then** The runtime executes Input modules and retrieves data normally  
**And** The runtime executes Filter modules and transforms data normally  
**And** The runtime does NOT execute Output modules (no HTTP requests sent)  
**And** The runtime shows what would have been sent to target system  
**And** The runtime validates the complete pipeline flow  
**And** The runtime reports any errors or issues found (FR48)  
**And** No side effects occur on target systems

## Tasks / Subtasks

- [x] Task 1: Enhance output module to support request preview (AC: shows what would have been sent)
  - [x] Add `PreviewRequest()` method to `output.Module` interface
  - [x] Implement preview in `HTTPRequestModule` that prepares request without sending
  - [x] Preview should include: endpoint URL, HTTP method, headers, request body (JSON preview)
  - [x] Preview should handle both batch mode and single record mode
  - [x] Preview should resolve path parameters and query params correctly
  - [x] Preview should show authentication headers (masked for security)
  - [x] Add unit tests for preview functionality
- [x] Task 2: Enhance executor to call preview in dry-run mode (AC: shows what would have been sent)
  - [x] Update `executeOutput()` to call `PreviewRequest()` when in dry-run mode
  - [x] Store preview information in execution result or separate structure
  - [x] Ensure preview is called even if output module would normally fail
  - [x] Add unit tests for executor dry-run preview
- [x] Task 3: Enhance CLI output to display dry-run preview (AC: shows what would have been sent)
  - [x] Update `printExecutionResult()` to show preview information in dry-run mode
  - [x] Display formatted preview: endpoint, method, headers, body preview
  - [x] Show record count that would have been sent
  - [x] Format JSON body preview with indentation for readability
  - [x] Mask sensitive authentication headers in preview
  - [x] Add verbose mode to show full request details
- [x] Task 4: Ensure complete pipeline validation in dry-run (AC: validates complete pipeline flow)
  - [x] Verify Input module execution works normally in dry-run
  - [x] Verify Filter module execution works normally in dry-run
  - [x] Verify all validation errors are reported correctly
  - [x] Verify error handling works the same in dry-run as normal mode
  - [x] Add integration tests for complete pipeline dry-run flow
- [x] Task 5: Enhance error reporting for dry-run mode (AC: reports any errors or issues found)
  - [x] Ensure Input errors are reported clearly in dry-run mode
  - [x] Ensure Filter errors are reported clearly in dry-run mode
  - [x] Ensure Output validation errors are reported (even though output isn't executed)
  - [x] Ensure preview errors are reported if request preparation fails
  - [x] Add tests for error reporting in dry-run mode
- [x] Task 6: Update scheduler to support dry-run mode (AC: complete implementation)
  - [x] Pass dry-run flag to `PipelineExecutorAdapter` when scheduler is in dry-run mode
  - [x] Update scheduler CLI integration to support `--dry-run` flag
  - [x] Ensure scheduled executions respect dry-run mode
  - [x] Add tests for scheduler dry-run mode (covered by executor tests)
- [x] Task 7: Update documentation and examples (AC: complete implementation)
  - [x] Update README.md with dry-run mode usage examples
  - [x] Add example showing dry-run output format
  - [x] Document what is shown in dry-run preview
  - [x] Add troubleshooting section for dry-run mode
  - [x] Update CLI help text if needed

## Dev Notes

### Architecture Requirements

**Dry-Run Mode Implementation:**
- **Location:** `canectors-runtime/internal/runtime/pipeline.go` (Executor), `canectors-runtime/internal/modules/output/` (Output modules)
- **Purpose:** Validate pipeline configurations without side effects on target systems
- **Scope:** All pipeline executions (single run and scheduled)

**Current State:**
- `Executor` already has `dryRun` field and skips output execution when `dryRun=true`
- CLI already has `--dry-run` flag defined (line 149 in `main.go`)
- Executor already passes `dryRun` flag when creating executor (line 283 in `main.go`)
- **Missing:** Preview functionality to show what would have been sent

**Integration Points:**
- Executor uses output modules via `output.Module` interface
- CLI uses executor for pipeline execution
- Scheduler uses executor adapter for scheduled executions
- All modules must support preview mode

**Existing Code Structure:**
- `internal/runtime/pipeline.go` - Executor with dry-run support (partial)
- `internal/modules/output/http_request.go` - HTTP Request output module
- `cmd/canectors/main.go` - CLI with `--dry-run` flag
- `internal/scheduler/scheduler.go` - Scheduler with executor adapter

### Project Structure Notes

**File Locations:**
- Executor implementation: `canectors-runtime/internal/runtime/pipeline.go`
- Executor tests: `canectors-runtime/internal/runtime/pipeline_test.go`
- Output module interface: `canectors-runtime/internal/modules/output/output.go`
- HTTP Request module: `canectors-runtime/internal/modules/output/http_request.go`
- HTTP Request tests: `canectors-runtime/internal/modules/output/http_request_test.go`
- CLI integration: `canectors-runtime/cmd/canectors/main.go`
- CLI tests: `canectors-runtime/cmd/canectors/main_test.go`
- Scheduler integration: `canectors-runtime/cmd/canectors/main.go` (PipelineExecutorAdapter)

**Dependencies:**
- Use existing `github.com/canectors/runtime/pkg/connector` for types
- Use existing `github.com/canectors/runtime/internal/runtime` for Executor
- Use existing `github.com/canectors/runtime/internal/logger` for logging
- Use existing `github.com/canectors/runtime/internal/modules/output` for output modules

### Technical Requirements

**Dry-Run Mode Behavior:**
- Input modules execute normally (fetch data from source)
- Filter modules execute normally (transform data)
- Output modules do NOT execute (no HTTP requests sent)
- Output modules MUST provide preview of what would have been sent
- Complete pipeline flow is validated (Input → Filter → Output preparation)
- All errors are reported (Input errors, Filter errors, Output validation errors)

**Preview Requirements:**
- Show endpoint URL (with resolved path parameters and query params)
- Show HTTP method (POST, PUT, PATCH)
- Show request headers (with authentication headers masked)
- Show request body preview (formatted JSON, truncated if too large)
- Show record count that would have been sent
- Handle both batch mode (single request) and single record mode (multiple requests)

**Error Handling:**
- Input errors: Stop execution, report error (same as normal mode)
- Filter errors: Stop execution, report error (same as normal mode)
- Output validation errors: Report error even in dry-run (e.g., invalid endpoint, missing auth)
- Preview errors: Report if request preparation fails (e.g., JSON marshal error)

**CLI Output Requirements:**
- Clear indication that dry-run mode is active
- Show preview information in readable format
- Format JSON body with indentation for readability
- Truncate large bodies with indication of truncation
- Mask sensitive information (authentication headers, API keys)
- Verbose mode shows full request details

**Scheduler Integration:**
- Scheduler must respect `--dry-run` flag when running scheduled pipelines
- All scheduled executions in dry-run mode should show preview
- Dry-run mode should not affect scheduler timing or scheduling logic

**Testing Requirements:**
- Unit tests for output module preview functionality
- Unit tests for executor dry-run preview
- Integration tests for complete pipeline dry-run flow
- CLI tests for dry-run output format
- Scheduler tests for dry-run mode
- Error handling tests for dry-run mode

### Library and Framework Requirements

**Go Standard Library:**
- `context` for request context
- `encoding/json` for JSON formatting in preview
- `fmt` for formatted output
- `strings` for string manipulation in preview

**Existing Dependencies (already in go.mod):**
- `github.com/canectors/runtime/pkg/connector` - Pipeline and execution result types
- `github.com/canectors/runtime/internal/runtime` - Executor
- `github.com/canectors/runtime/internal/modules/output` - Output module interface
- `github.com/canectors/runtime/internal/logger` - Structured logging
- `github.com/canectors/runtime/internal/auth` - Authentication handling

**No New Dependencies Required:**
- All functionality can be implemented using existing dependencies
- JSON formatting can use standard library `encoding/json`

### Previous Story Intelligence

**From Story 4.1 (CRON Scheduler):**
- Scheduler uses `PipelineExecutorAdapter` to execute pipelines
- Scheduler already supports executor with dry-run flag (via adapter)
- CLI already has scheduler mode that auto-detects from `schedule` field
- Scheduler integration is in `cmd/canectors/main.go`

**From Story 3.6 (Authentication Handling):**
- Authentication is handled in `internal/auth/` package
- Output modules use shared authentication via `auth.Handler`
- Authentication headers should be masked in preview for security

**From Story 3.5 (Output Module Execution):**
- HTTP Request module is fully implemented with batch and single record modes
- Output module supports path parameters, query params, headers from record
- Output module has retry logic and error handling
- Output module structure is ready for preview functionality

**From Story 3.1-3.4 (Module Execution):**
- Input modules (HTTPPolling) are implemented and working
- Filter modules (Mapping, Conditions) are implemented and working
- Runtime executor orchestrates complete pipeline execution
- Pipeline execution flow is well-established

**From Story 2.3 (Pipeline Orchestration):**
- `runtime.Executor` exists and orchestrates Input → Filter → Output flow
- Executor already has `dryRun` field and basic dry-run support
- Executor returns structured `ExecutionResult` with status and metrics
- Executor error handling is comprehensive

**From Story 2.1 (Go CLI Project Structure):**
- Project structure follows Go best practices
- CLI structure in `/cmd/canectors/main.go` ready for enhancements
- Test structure co-located with source files

### Git Intelligence Summary

**Recent Work Patterns:**
- CRON scheduler implementation (Story 4.1) - scheduler.go with executor adapter
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

**Files Recently Modified:**
- `internal/scheduler/scheduler.go` - CRON scheduler implementation
- `cmd/canectors/main.go` - CLI with scheduler integration
- `internal/modules/output/http_request.go` - HTTP Request output module
- `internal/runtime/pipeline.go` - Pipeline executor

### Latest Technical Information

**Go Best Practices for Preview Functionality:**
- Use interface methods for preview: `PreviewRequest() (RequestPreview, error)`
- Structure preview data with clear types: `RequestPreview` struct
- Format JSON with `json.MarshalIndent()` for readable output
- Truncate large bodies with indication (e.g., "... (truncated, 10KB total)")
- Mask sensitive data in preview (authentication headers, API keys)

**CLI Output Best Practices:**
- Use consistent formatting for preview output
- Indent nested structures for readability
- Use color coding if available (optional, not required)
- Provide clear indication of dry-run mode vs normal mode
- Show summary information (record count, endpoint, method) prominently

**Security Considerations:**
- Always mask authentication headers in preview (Authorization, X-API-Key, etc.)
- Mask API keys and tokens in preview output
- Do not log sensitive information in preview
- Consider environment variable to control preview verbosity

### Project Context Reference

**From project-context.md:**
- Go Runtime CLI is separate project from Next.js application
- Cross-platform compilation required (Windows, Mac, Linux)
- Follow Go best practices and conventions
- Use `golangci-lint run` after implementation to verify linting passes
- Runtime must be deterministic (NFR24, NFR25)

**From Architecture:**
- Dry-run mode is part of runtime CLI (Epic 2, Part 3/3 - Advanced Runtime Features)
- Dry-run mode enables validation without side effects (FR38)
- Runtime must generate clear execution logs (FR47)
- Runtime must report errors clearly (FR48)

**From PRD:**
- FR38: System can execute connectors in dry-run mode (validate without side effects)
- FR48: System can report errors and issues found during execution
- FR47: System can generate execution logs with clear, explicit messages

**From Epics:**
- Epic 4: Advanced Runtime Features (Part 3/3 of Epic 2 from Architecture)
- Story 4.2 is second story in Epic 4 (after 4.1 CRON Scheduler)
- Story 4.2 enables validation without side effects
- Subsequent stories: 4.3 (Logging), 4.4 (Error Handling), 4.5 (CLI Commands), 4.6 (Cross-Platform)

### References

- **Architecture Decision:** [Source: _bmad-output/planning-artifacts/architecture.md#Runtime CLI Architecture]
- **Epic Definition:** [Source: _bmad-output/planning-artifacts/epics.md#Epic 4: Advanced Runtime Features]
- **Story Requirements:** [Source: _bmad-output/planning-artifacts/epics.md#Story 4.2: Implement Dry-Run Mode]
- **Executor Implementation:** [Source: canectors-runtime/internal/runtime/pipeline.go]
- **Output Module Interface:** [Source: canectors-runtime/internal/modules/output/output.go]
- **HTTP Request Module:** [Source: canectors-runtime/internal/modules/output/http_request.go]
- **CLI Implementation:** [Source: canectors-runtime/cmd/canectors/main.go]
- **Scheduler Implementation:** [Source: canectors-runtime/internal/scheduler/scheduler.go]
- **Project Context:** [Source: _bmad-output/project-context.md]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Debug Log References

### Completion Notes List

- **Task 1 (2026-01-21):** Added `PreviewRequest()` method to output module for dry-run preview functionality.
  - Created `RequestPreview` struct with endpoint, method, headers, body preview, and record count
  - Created `PreviewableModule` interface extending `Module` with `PreviewRequest()` method
  - Implemented preview for batch mode (single request for all records) and single record mode (one preview per record)
  - Authentication headers are masked for security using `[MASKED-*]` format
  - Body preview is formatted with indentation for readability
  - 13 unit tests added covering all preview scenarios
  - All tests pass, linting passes with 0 issues

- **Task 2 (2026-01-21):** Enhanced executor to call preview in dry-run mode.
  - Added `DryRunPreview` field and `RequestPreview` struct to `ExecutionResult` in connector types
  - Created `executeDryRunPreview()` method that checks for `PreviewableModule` interface
  - Updated `Execute()` and `ExecuteWithRecords()` to generate previews in dry-run mode
  - Preview errors are logged but don't fail execution (informational)
  - 8 unit tests added covering all executor dry-run preview scenarios
  - All tests pass, linting passes with 0 issues

- **Task 3 (2026-01-21):** Enhanced CLI output to display dry-run preview.
  - Added `printDryRunPreview()` function to display formatted preview in dry-run mode
  - Added `printBodyPreview()` with truncation for large payloads
  - Verbose mode shows full headers and body details
  - Updated `createOutputModule()` to use real `HTTPRequestModule` for preview support

- **Task 4 (2026-01-21):** Complete pipeline validation tests in dry-run mode.
  - Added 6 integration tests for Input → Filter → Preview flow
  - Verified input and filter modules execute normally in dry-run mode
  - Verified filtered data is passed to preview correctly

- **Task 5 (2026-01-21):** Enhanced error reporting for dry-run mode.
  - Added 6 tests for comprehensive error reporting
  - Input/Filter errors include module, code, and detailed message
  - Preview errors don't fail execution (informational only)
  - Nil pipeline/input errors are reported clearly

- **Task 6 (2026-01-21):** Scheduler dry-run integration verified.
  - `PipelineExecutorAdapter` already passes `dryRun` flag to executor
  - Scheduled executions respect dry-run mode

- **Task 7 (2026-01-21):** Documentation updated.
  - Added comprehensive dry-run section to README.md
  - Documented preview output format and verbose mode
  - Updated roadmap to mark Story 4.2 complete

### File List

- `internal/modules/output/output.go` - Added `RequestPreview` struct and `PreviewableModule` interface
- `internal/modules/output/http_request.go` - Added `PreviewRequest()` implementation with masking
- `internal/modules/output/http_request_test.go` - Added 13 unit tests for preview functionality
- `pkg/connector/types.go` - Added `DryRunPreview` field and `RequestPreview` struct to `ExecutionResult`
- `internal/runtime/pipeline.go` - Added `executeDryRunPreview()` method and integration in Execute/ExecuteWithRecords
- `internal/runtime/pipeline_test.go` - Added 21 unit tests for dry-run preview and error handling
- `cmd/canectors/main.go` - Added `printDryRunPreview()` and updated `createOutputModule()` for preview support
- `README.md` - Added comprehensive dry-run documentation section

## Change Log

| Date | Change | Author |
|------|--------|--------|
| 2026-01-21 | Story implementation complete - all 7 tasks done | Claude Opus 4.5 |
