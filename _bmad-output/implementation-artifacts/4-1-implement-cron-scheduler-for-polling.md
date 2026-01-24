# Story 4.1: Implement CRON Scheduler for Polling

Status: review

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,  
I want the runtime to schedule Input modules with CRON expressions,  
So that I can execute periodic data polling automatically.

## Acceptance Criteria

**Given** I have a connector with HTTP Polling Input module and CRON schedule  
**When** The runtime starts with the connector configuration  
**Then** The runtime parses CRON expressions correctly (FR52, FR62)  
**And** The runtime schedules Input module execution according to CRON schedule  
**And** The runtime executes the complete pipeline (Input → Filter → Output) on schedule  
**And** The runtime handles overlapping executions gracefully  
**And** The runtime logs scheduled execution times (FR47)  
**And** The scheduler uses Go library (robfig/cron) as specified in Architecture

## Tasks / Subtasks

- [x] Task 1: Add robfig/cron dependency (AC: scheduler uses Go library robfig/cron)
  - [x] Add `github.com/robfig/cron/v3` to go.mod
  - [x] Run `go mod tidy` to download dependency
  - [x] Verify dependency is compatible with Go 1.23.5
- [x] Task 2: Implement CRON expression parsing (AC: runtime parses CRON expressions correctly)
  - [x] Use robfig/cron parser to validate CRON expressions
  - [x] Support standard CRON format (5 fields: minute, hour, day, month, weekday)
  - [x] Support extended CRON format (6 fields: second, minute, hour, day, month, weekday)
  - [x] Validate CRON expressions from Pipeline.Schedule field
  - [x] Return clear error messages for invalid CRON expressions
  - [x] Add unit tests for CRON parsing
- [x] Task 3: Implement pipeline registration in scheduler (AC: schedules Input module execution)
  - [x] Implement `Scheduler.Register()` method in `internal/scheduler/scheduler.go`
  - [x] Validate pipeline has valid Schedule field (non-empty CRON expression)
  - [x] Validate pipeline is enabled (Pipeline.Enabled == true)
  - [x] Store pipeline with its schedule in scheduler's internal map
  - [x] Handle duplicate pipeline registration (update existing or error)
  - [x] Add unit tests for pipeline registration
- [x] Task 4: Implement scheduled pipeline execution (AC: executes complete pipeline on schedule)
  - [x] Implement `Scheduler.Start()` method to begin scheduling
  - [x] Use robfig/cron to schedule each registered pipeline
  - [x] On CRON trigger, execute complete pipeline (Input → Filter → Output)
  - [x] Use existing `runtime.Executor` to execute pipelines
  - [x] Pass context with cancellation support for graceful shutdown
  - [x] Handle execution errors without stopping scheduler
  - [x] Add unit tests for scheduled execution
- [x] Task 5: Implement overlapping execution handling (AC: handles overlapping executions gracefully)
  - [x] Track running executions per pipeline
  - [x] Skip new execution if previous execution is still running (skip if busy)
  - [x] Log when execution is skipped due to overlap
  - [x] Ensure thread-safe execution tracking
  - [x] Add unit tests for overlap handling
- [x] Task 6: Implement execution logging (AC: logs scheduled execution times)
  - [x] Log when pipeline execution is scheduled (before execution)
  - [x] Log when pipeline execution starts
  - [x] Log when pipeline execution completes (success or error)
  - [x] Include pipeline ID, schedule, and execution time in logs
  - [x] Use structured logging (slog) consistent with existing codebase
  - [x] Add logging tests
- [x] Task 7: Implement scheduler stop functionality (AC: graceful shutdown)
  - [x] Implement `Scheduler.Stop()` method to halt all scheduled executions
  - [x] Stop all CRON jobs gracefully
  - [x] Wait for in-flight executions to complete (with timeout)
  - [x] Clear registered pipelines
  - [x] Add unit tests for stop functionality
- [x] Task 8: Integrate scheduler with CLI (AC: runtime starts with connector configuration)
  - [x] Update CLI `run` command to auto-detect scheduler mode (when Schedule is present in config)
  - [x] Load pipeline configuration from file
  - [x] Register pipeline with scheduler if Schedule field is present
  - [x] Start scheduler and keep running until interrupt (SIGINT/SIGTERM)
  - [x] Handle graceful shutdown on interrupt
  - [x] Existing CLI tests pass (scheduler mode is transparent)
- [x] Task 9: Add scheduler documentation and examples (AC: complete implementation)
  - [x] Update README.md with scheduler usage examples
  - [x] Add example configuration files with CRON schedules
  - [x] Document CRON expression format and examples
  - [x] Document scheduler behavior (overlap handling, logging, etc.)
  - [x] Add troubleshooting section for common issues

## Dev Notes

### Architecture Requirements

**CRON Scheduler Implementation:**
- **Location:** `canectors-runtime/internal/scheduler/scheduler.go` (already exists, needs implementation)
- **Library:** `github.com/robfig/cron/v3` (to be added) [Source: _bmad-output/planning-artifacts/architecture.md#CRON Scheduler for Polling]
- **Purpose:** Schedule periodic execution of connector pipelines based on CRON expressions
- **Scope:** HTTP Polling Input modules with Schedule field configured

**Pipeline Structure:**
- Pipeline already has `Schedule` field in `pkg/connector/types.go` (line 33-34)
- Schedule field is optional (omitempty) - only pipelines with Schedule are scheduled
- Schedule field contains CRON expression string (e.g., "0 */1 * * *" for hourly)

**Integration Points:**
- Scheduler uses `runtime.Executor` to execute pipelines
- Scheduler integrates with CLI main.go for long-running execution
- Scheduler uses existing logger package for structured logging

**Existing Code Structure:**
- `internal/scheduler/scheduler.go` exists with skeleton implementation
- Methods return `ErrNotImplemented` - need to implement Register, Start, Stop
- Scheduler struct has `pipelines map[string]*connector.Pipeline` field
- `runtime.Executor` exists and can execute complete pipelines (Input → Filter → Output)

### Project Structure Notes

**File Locations:**
- Scheduler implementation: `canectors-runtime/internal/scheduler/scheduler.go`
- Scheduler tests: `canectors-runtime/internal/scheduler/scheduler_test.go`
- Pipeline types: `canectors-runtime/pkg/connector/types.go`
- Runtime executor: `canectors-runtime/internal/runtime/executor.go`
- CLI integration: `canectors-runtime/cmd/canectors/main.go`

**Dependencies:**
- Add `github.com/robfig/cron/v3` to go.mod
- Use existing `github.com/canectors/runtime/pkg/connector` for Pipeline type
- Use existing `github.com/canectors/runtime/internal/runtime` for Executor
- Use existing `github.com/canectors/runtime/internal/logger` for logging

### Technical Requirements

**CRON Expression Format:**
- Support standard 5-field format: `minute hour day month weekday`
- Support extended 6-field format: `second minute hour day month weekday`
- Examples:
  - `"0 */1 * * *"` - Every hour at minute 0
  - `"0 0 * * *"` - Every day at midnight
  - `"*/5 * * * *"` - Every 5 minutes
  - `"0 9 * * 1-5"` - Every weekday at 9 AM

**Scheduler Behavior:**
- Register pipelines with valid Schedule and Enabled=true
- Start scheduler to begin periodic execution
- Execute complete pipeline (Input → Filter → Output) on each trigger
- Skip execution if previous execution is still running (prevent overlap)
- Log all scheduled executions with timing information
- Stop gracefully on shutdown signal

**Error Handling:**
- Invalid CRON expressions: return error during Register, don't start scheduler
- Pipeline execution errors: log error, continue scheduling (don't stop scheduler)
- Overlapping executions: skip new execution, log warning
- Shutdown: wait for in-flight executions to complete (with timeout)

**Thread Safety:**
- Scheduler must be thread-safe for concurrent access
- Pipeline execution tracking must be thread-safe
- Use mutexes or channels for synchronization

**Testing Requirements:**
- Unit tests for CRON parsing and validation
- Unit tests for scheduler registration, start, stop
- Unit tests for overlap handling
- Integration tests for CLI scheduler mode
- Test with various CRON expressions
- Test error handling scenarios

### Library and Framework Requirements

**Go Standard Library:**
- `context` for cancellation and timeouts
- `sync` for thread-safe operations (mutexes)
- `os/signal` for graceful shutdown handling
- `time` for execution timing

**External Dependencies:**
- `github.com/robfig/cron/v3` - CRON expression parsing and scheduling [Source: _bmad-output/planning-artifacts/architecture.md#CRON Scheduler for Polling]
  - Version: Latest v3 (check compatibility with Go 1.23.5)
  - Usage: Parse CRON expressions, schedule jobs, manage cron instance

**Existing Dependencies (already in go.mod):**
- `github.com/canectors/runtime/pkg/connector` - Pipeline types
- `github.com/canectors/runtime/internal/runtime` - Executor for pipeline execution
- `github.com/canectors/runtime/internal/logger` - Structured logging

### Previous Story Intelligence

**From Story 3.6 (Authentication Handling):**
- Authentication is handled in `internal/auth/` package
- Input and Output modules use shared authentication
- Pipeline execution already supports authentication via module configuration
- No changes needed to authentication for scheduler integration

**From Story 3.1-3.5 (Module Execution):**
- Input modules (HTTPPolling) are implemented and working
- Filter modules (Mapping, Conditions) are implemented and working
- Output modules (HTTPRequest) are implemented and working
- Runtime executor orchestrates complete pipeline execution
- Pipeline execution is deterministic (NFR24, NFR25)

**From Story 2.3 (Pipeline Orchestration):**
- `runtime.Executor` exists and can execute complete pipelines
- Executor handles Input → Filter → Output flow
- Executor supports dry-run mode (for future Story 4.2)
- Executor returns structured execution results

**From Story 2.1 (Go CLI Project Structure):**
- Project structure follows Go best practices
- `/internal/scheduler/` directory already exists
- Scheduler skeleton code already exists with TODO comments
- CLI structure in `/cmd/canectors/main.go` ready for integration

### Git Intelligence Summary

**Recent Work Patterns:**
- Authentication package extracted to `internal/auth/` (Story 3.6)
- Module execution implemented with shared authentication (Stories 3.1-3.6)
- Pipeline executor implemented with Input/Filter/Output orchestration (Story 2.3)
- Go project structure established with proper layout (Story 2.1)

**Code Patterns Established:**
- Use structured logging with `internal/logger` package
- Use error wrapping with context: `fmt.Errorf("context: %w", err)`
- Use interfaces for module abstraction (input.Module, filter.Module, output.Module)
- Use context.Context for cancellation and timeouts
- Follow Go naming conventions (PascalCase for exported, camelCase for unexported)

### Latest Technical Information

**robfig/cron/v3 Library:**
- Latest stable version: v3.0.0 or later
- Supports standard and extended CRON formats
- Thread-safe cron instance
- Supports timezone configuration
- Supports job removal and management
- Documentation: https://pkg.go.dev/github.com/robfig/cron/v3

**CRON Expression Best Practices:**
- Use 5-field format for most cases (simpler)
- Use 6-field format only if second-level precision needed
- Validate expressions before registration
- Test expressions with cron parser before use
- Document expected execution times for users

### Project Context Reference

**From project-context.md:**
- Go Runtime CLI is separate project from Next.js application
- Cross-platform compilation required (Windows, Mac, Linux)
- Follow Go best practices and conventions
- Use `golangci-lint run` after implementation to verify linting passes
- Runtime must be deterministic (NFR24, NFR25)

**From Architecture:**
- CRON Scheduler uses `robfig/cron` library [Source: _bmad-output/planning-artifacts/architecture.md#CRON Scheduler for Polling]
- Scheduler is part of runtime CLI (Epic 2, Part 3/3)
- Scheduler executes in workers Docker (Railway/Fly.io)
- Scheduler is integrated into runtime CLI, not external service

**From PRD:**
- FR52: Runtime can execute Input modules with CRON scheduling (polling)
- FR62: System can configure HTTP Request Input module with polling and CRON scheduling
- FR47: System can generate execution logs with clear, explicit messages

**From Epics:**
- Epic 4: Advanced Runtime Features (Part 3/3 of Epic 2 from Architecture)
- Story 4.1 is first story in Epic 4
- Story 4.1 enables periodic data polling with CRON scheduling
- Subsequent stories: 4.2 (Dry-Run), 4.3 (Logging), 4.4 (Error Handling), 4.5 (CLI Commands), 4.6 (Cross-Platform)

### References

- **Architecture Decision:** [Source: _bmad-output/planning-artifacts/architecture.md#CRON Scheduler for Polling]
- **Epic Definition:** [Source: _bmad-output/planning-artifacts/epics.md#Epic 4: Advanced Runtime Features]
- **Story Requirements:** [Source: _bmad-output/planning-artifacts/epics.md#Story 4.1: Implement CRON Scheduler for Polling]
- **Pipeline Types:** [Source: canectors-runtime/pkg/connector/types.go]
- **Scheduler Skeleton:** [Source: canectors-runtime/internal/scheduler/scheduler.go]
- **Runtime Executor:** [Source: canectors-runtime/internal/runtime/executor.go]
- **Project Context:** [Source: _bmad-output/project-context.md]
- **robfig/cron Documentation:** https://pkg.go.dev/github.com/robfig/cron/v3

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (Cursor Agent)

### Debug Log References

- Fixed data race in scheduler Stop() function - moved cron.Stop() before setting started=false to prevent race between executePipeline's wg.Add(1) and Stop's wg.Wait()

### Completion Notes List

- ✅ Added `github.com/robfig/cron/v3 v3.0.1` dependency to go.mod
- ✅ Implemented complete scheduler in `internal/scheduler/scheduler.go`:
  - `ValidateCronExpression()` - validates 5 and 6 field CRON expressions
  - `Scheduler.Register()` - registers pipeline with CRON schedule
  - `Scheduler.Unregister()` - removes pipeline from scheduler
  - `Scheduler.Start()` - starts CRON scheduler
  - `Scheduler.Stop()` - graceful shutdown with timeout
  - Thread-safe execution with overlap detection
  - Structured logging for all operations
- ✅ Created comprehensive test suite (27 tests) in `internal/scheduler/scheduler_test.go`
- ✅ Integrated scheduler into `run` command - auto-detects `schedule` field in config
- ✅ Created `PipelineExecutorAdapter` to integrate runtime.Executor with scheduler
- ✅ Updated README.md with scheduler documentation and CRON examples
- ✅ Added example configurations: `13-scheduled.json` and `13-scheduled.yaml`
- ✅ Updated configs/examples/README.md with scheduler example documentation
- ✅ All tests pass with race detector (100+ tests)
- ✅ golangci-lint passes with 0 issues

### File List

**New Files:**
- `canectors-runtime/internal/scheduler/scheduler_test.go`
- `canectors-runtime/configs/examples/13-scheduled.json`
- `canectors-runtime/configs/examples/13-scheduled.yaml`

**Modified Files:**
- `canectors-runtime/go.mod` - Added robfig/cron/v3 dependency
- `canectors-runtime/go.sum` - Updated with new dependency
- `canectors-runtime/internal/scheduler/scheduler.go` - Complete scheduler implementation
- `canectors-runtime/cmd/canectors/main.go` - Added schedule command and PipelineExecutorAdapter
- `canectors-runtime/README.md` - Added scheduler documentation
- `canectors-runtime/configs/examples/README.md` - Added scheduled example documentation

## Change Log

- 2026-01-21: Story 4.1 implementation completed - CRON scheduler for polling with CLI integration, tests, and documentation
- 2026-01-21: Refactored CLI - removed `schedule` command, scheduler mode auto-detected from `schedule` field in config (per user request)
