# Story 2.3: Implement Pipeline Orchestration

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,  
I want the runtime to orchestrate the complete pipeline (Input → Filter → Output),  
So that I can execute end-to-end connector workflows.

## Acceptance Criteria

**Given** I have a complete connector pipeline configuration  
**When** The runtime executes the pipeline  
**Then** The runtime executes Input module first to retrieve data (FR40)  
**And** The runtime executes Filter modules in sequence to transform data (FR41)  
**And** The runtime executes Output module to send data to target (FR42)  
**And** The runtime handles errors at any stage and stops execution gracefully (FR46)  
**And** The runtime maintains data flow between modules correctly  
**And** The execution is deterministic and repeatable (FR43, NFR24, NFR25)

## Tasks / Subtasks

- [x] Task 1: Implement core pipeline execution engine (AC: execute Input → Filter → Output, deterministic)
  - [x] Create `Executor.Execute()` method that orchestrates the pipeline
  - [x] Implement Input module execution (call Input.Fetch())
  - [x] Implement Filter modules execution in sequence (call Filter.Process() for each)
  - [x] Implement Output module execution (call Output.Send())
  - [x] Ensure data flows correctly between modules (Input → Filter → Output)
  - [x] Add unit tests for successful pipeline execution
  - [x] Add integration tests with mock modules
- [x] Task 2: Implement error handling and graceful failure (AC: handle errors, stop gracefully)
  - [x] Handle errors from Input module (log error, stop execution)
  - [x] Handle errors from Filter modules (log error, stop execution)
  - [x] Handle errors from Output module (log error, stop execution)
  - [x] Return structured ExecutionResult with error details
  - [x] Ensure no data loss on errors (don't send partial data to output)
  - [x] Add unit tests for error scenarios
- [x] Task 3: Implement execution result tracking (AC: deterministic, repeatable)
  - [x] Track execution start time (StartedAt)
  - [x] Track execution completion time (CompletedAt)
  - [x] Track records processed count
  - [x] Track records failed count
  - [x] Set execution status (success, error, partial)
  - [x] Ensure deterministic execution (same input = same output)
  - [x] Add unit tests for execution result tracking
- [x] Task 4: Integrate with CLI run command (AC: all above, CLI integration)
  - [x] Update `runPipeline()` in `cmd/canectors/main.go` to use Executor
  - [x] Convert parsed config to `connector.Pipeline` type
  - [x] Create Executor instance with dry-run flag support
  - [x] Execute pipeline and display results
  - [x] Handle execution errors with appropriate exit codes
  - [x] Add integration tests for CLI run command
- [x] Task 5: Add logging and observability (AC: all above, clear execution flow)
  - [x] Log pipeline execution start
  - [x] Log each module execution (Input, Filter, Output)
  - [x] Log execution completion with summary
  - [x] Log errors with context (module, error details)
  - [x] Use structured logging (slog) for consistency
  - [x] Add verbose logging support for debugging

## Dev Notes

### Architecture Requirements

**Pipeline Execution Flow:**
1. **Input Module Execution**: Execute Input module to fetch data from source
   - Call `Input.Module.Fetch()` to retrieve data
   - Data returned as `[]map[string]interface{}` (slice of records)
   - Handle Input module errors (network, authentication, etc.)
2. **Filter Modules Execution**: Execute Filter modules in sequence to transform data
   - Iterate through `Pipeline.Filters` array in order
   - For each filter, call `Filter.Module.Process(records)`
   - Pass output of previous filter as input to next filter
   - Handle Filter module errors (transformation failures, validation errors)
3. **Output Module Execution**: Execute Output module to send data to target
   - Call `Output.Module.Send(records)` with transformed data
   - Track number of records successfully sent
   - Handle Output module errors (network, authentication, validation)

**Error Handling Strategy:**
- **Input errors**: Stop execution immediately, return error result
- **Filter errors**: Stop execution immediately, return error result (no partial data sent)
- **Output errors**: Stop execution, return error result with partial success count if applicable
- **Error propagation**: Wrap errors with context (module type, module name, error details)
- **No data loss**: Never send partial data to output if any stage fails

**Deterministic Execution:**
- Same pipeline configuration + same input data = same output
- No random behavior, no time-dependent logic (except timestamps)
- Execution order is fixed: Input → Filters (in order) → Output
- Error handling is deterministic (same error = same handling)

**Module Interface Integration:**
- Input modules implement `input.Module` interface with `Fetch()` method
- Filter modules implement `filter.Module` interface with `Process()` method
- Output modules implement `output.Module` interface with `Send()` method
- Modules are not yet implemented (Epic 3), so use mocks for testing

### Project Structure Notes

**File Organization:**
```
canectors-runtime/
├── internal/
│   └── runtime/
│       ├── pipeline.go          # Executor implementation (to be completed)
│       ├── pipeline_test.go     # Unit tests for Executor
│       └── executor.go          # Optional: separate executor logic if needed
├── cmd/
│   └── canectors/
│       └── main.go              # CLI run command integration
└── pkg/
    └── connector/
        └── types.go             # Pipeline, ExecutionResult types (already defined)
```

**Integration Points:**
- Executor will be used by:
  - `run` command - execute pipelines from config files
  - Future scheduler - execute pipelines on CRON schedule (Epic 4)
- Executor depends on:
  - `pkg/connector.Pipeline` type (already defined)
  - `pkg/connector.ExecutionResult` type (already defined)
  - Module interfaces: `input.Module`, `filter.Module`, `output.Module` (not yet implemented, use mocks)
  - Logger: `internal/logger` package (already available)

**Module Instantiation:**
- Modules are not yet implemented (Epic 3)
- For Story 2.3, create mock implementations for testing
- Executor should accept module instances (dependency injection pattern)
- Future: Module factory will create modules from `ModuleConfig` (Epic 3)

### Testing Requirements

**Unit Tests:**
- Executor.Execute() with successful pipeline execution
- Executor.Execute() with Input module error
- Executor.Execute() with Filter module error
- Executor.Execute() with Output module error
- Executor.Execute() with empty input data
- Executor.Execute() with multiple Filter modules
- ExecutionResult tracking (timestamps, counts, status)
- Deterministic execution (same input = same output)

**Integration Tests:**
- CLI `run` command with valid config
- CLI `run` command with execution error
- CLI `run` command with dry-run mode
- Error handling and exit codes

**Test Data:**
- Create test fixtures in `/internal/runtime/testdata/`:
  - `valid-pipeline-config.json` - Complete valid pipeline config
  - `pipeline-with-filters.json` - Pipeline with multiple filters
  - `pipeline-error-input.json` - Pipeline that will fail at Input stage
  - `pipeline-error-filter.json` - Pipeline that will fail at Filter stage
  - `pipeline-error-output.json` - Pipeline that will fail at Output stage

**Mock Modules:**
- Create mock implementations for testing:
  - `MockInputModule` - Returns test data or errors
  - `MockFilterModule` - Transforms data or returns errors
  - `MockOutputModule` - Sends data or returns errors
- Mocks should be in test files or `internal/runtime/mocks/` package

### References

- **Epic 2 Overview:** [Source: _bmad-output/planning-artifacts/epics.md#Epic 2: CLI Runtime Foundation]
- **Story 2.3 Details:** [Source: _bmad-output/planning-artifacts/epics.md#Story 2.3: Implement Pipeline Orchestration]
- **Story 2.2 (Previous):** [Source: _bmad-output/implementation-artifacts/2-2-implement-configuration-parser.md]
- **Story 2.1 (Previous):** [Source: _bmad-output/implementation-artifacts/2-1-initialize-go-cli-project-structure.md]
- **Pipeline Types:** [Source: canectors-runtime/pkg/connector/types.go]
- **Module Interfaces:** [Source: canectors-runtime/internal/modules/input/input.go, filter/filter.go, output/output.go]
- **CLI Runtime Architecture:** [Source: _bmad-output/planning-artifacts/architecture.md#Runtime CLI (Go) - Separate Project]
- **Project Context:** [Source: _bmad-output/project-context.md]
- **PRD Requirements:** [Source: _bmad-output/planning-artifacts/prd.md#Connector Execution]

### Critical Implementation Rules

**From Project Context:**
- Runtime CLI is separate Go project from Next.js application [Source: _bmad-output/project-context.md#Technology Stack]
- Cross-platform compilation required (Windows, Mac, Linux) [Source: _bmad-output/project-context.md#Development Workflow Rules]
- Follow Go best practices and conventions [Source: _bmad-output/project-context.md#Critical Implementation Rules]

**From Architecture:**
- Pipeline execution: `/internal/runtime/` for orchestration [Source: _bmad-output/planning-artifacts/architecture.md#CLI Runtime (Go)]
- Execution flow: Input → Filter → Output [Source: _bmad-output/planning-artifacts/architecture.md#Connector Model: Declarative Modular Pipeline]
- Deterministic execution required (NFR24, NFR25) [Source: _bmad-output/planning-artifacts/architecture.md#Reliability]

**From Epic 2 Learnings:**
- Configuration parser is implemented and tested (Story 2.2)
- Pipeline schema validation is in place
- CLI commands are set up with proper error handling
- Types are defined in `pkg/connector/types.go`

**Go-Specific Rules:**
- Use standard Go project layout
- Keep packages focused and cohesive
- Use `internal/` for private code
- Follow Go naming conventions (`camelCase` for exported, `camelCase` for unexported)
- Write tests alongside code (`*_test.go`)
- Handle errors explicitly (no silent failures)
- Use structured logging (`slog`) for consistency
- Return structured error types for detailed error information

### Library and Framework Requirements

**Go Standard Library:**
- `context` - Context for cancellation and timeouts (if needed)
- `errors` - Error wrapping and handling
- `fmt` - Error formatting
- `time` - Timestamp tracking

**External Dependencies:**
- No new dependencies required for Story 2.3
- Existing dependencies:
  - `github.com/spf13/cobra` - CLI framework (already in use)
  - `log/slog` - Structured logging (Go 1.21+)

**Build Tools:**
- `go test` - Testing
- `go build` - Compilation
- `go mod tidy` - Dependency management

### Previous Story Intelligence

**Epic 2 Stories (Completed):**
- **Story 2.1:** Go CLI project initialized at `/home/alexandrecano/Workspace/canectors-runtime/` [Source: _bmad-output/implementation-artifacts/2-1-initialize-go-cli-project-structure.md]
  - Project structure: `/cmd/canectors/`, `/internal/`, `/pkg/connector/`
  - CLI framework: Cobra (v1.10.2) for command-line parsing
  - Types defined: `Pipeline`, `ModuleConfig`, `AuthConfig`, `ErrorHandling`, `ExecutionResult` in `/pkg/connector/types.go`
  - Logger implemented: Structured logging with `log/slog` (Go 1.21+)
  - **Key learning:** Project uses Go 1.23.5, follows standard Go project layout
- **Story 2.2:** Configuration parser implemented [Source: _bmad-output/implementation-artifacts/2-2-implement-configuration-parser.md]
  - JSON/YAML parsing with auto-detection
  - JSON Schema validation with embedded schema
  - CLI `validate` and `run` commands implemented
  - Error handling with structured error types
  - **Key learning:** Parser returns `ConfigResult` with parsed data ready for execution

**Key Learnings from Epic 2:**
- Configuration parsing is complete and tested
- Pipeline types are well-defined in `pkg/connector/types.go`
- CLI structure is in place with proper error handling
- Module interfaces are defined but not yet implemented (Epic 3)
- For Story 2.3, use mock modules for testing

**Integration with Epic 3:**
- Story 2.3 orchestrates modules but doesn't implement them
- Module implementations will be added in Epic 3 (Stories 3.1, 3.3, 3.5)
- Executor should be designed to work with module interfaces (dependency injection)
- Module factory will be added in Epic 3 to create modules from `ModuleConfig`

**Epic Context:**
- Epic 2 is Priority 2 (CLI Runtime) - Partie 1/3
- Epic 2 has 3 stories: 2.1 (Done ✅), 2.2 (Done ✅), 2.3 (This story)
- Story 2.3 completes Epic 2 foundation before Epic 3 (Module Execution)

### Git Intelligence Summary

**Recent Work:**
- Story 2.1 completed: Go CLI project structure initialized
- Story 2.2 completed: Configuration parser implemented
- Current state: Pipeline execution stub exists in `internal/runtime/pipeline.go` with `ErrNotImplemented`
- Module interfaces defined but not implemented (Epic 3)

**Repository Structure:**
- Main project: `canectors/` (Next.js T3 Stack)
- Runtime project: `canectors-runtime/` (Go - separate project)
- Schema location: `/types/pipeline-schema.json` in main project

**Files from Previous Stories:**
- `/pkg/connector/types.go` - Pipeline types (Story 2.1)
- `/internal/config/parser.go` - Configuration parser (Story 2.2)
- `/internal/config/validator.go` - Schema validator (Story 2.2)
- `/cmd/canectors/main.go` - CLI entry point (Story 2.1, updated in 2.2)
- `/internal/runtime/pipeline.go` - Pipeline executor stub (Story 2.1, to be completed in 2.3)

### Latest Technical Information

**Go Pipeline Execution Patterns:**
- Use dependency injection for modules (accept interfaces, not concrete types)
- Pattern:
  ```go
  type Executor struct {
      inputModule  input.Module
      filterModules []filter.Module
      outputModule output.Module
      dryRun       bool
  }
  ```
- Benefits: Testable, flexible, follows Go best practices

**Error Handling Patterns:**
- Use Go `error` interface consistently
- Wrap errors for context: `fmt.Errorf("executing input module: %w", err)`
- Return structured `ExecutionResult` with error details
- Log errors with context: `logger.Error("pipeline execution failed", "module", "input", "error", err)`

**Deterministic Execution:**
- No random number generation
- No time-dependent logic (except timestamps for tracking)
- Fixed execution order: Input → Filters (in order) → Output
- Same input data = same output data

**Module Interface Design:**
- Input: `Fetch() ([]map[string]interface{}, error)` - Returns slice of records
- Filter: `Process([]map[string]interface{}) ([]map[string]interface{}, error)` - Transforms records
- Output: `Send([]map[string]interface{}) (int, error)` - Sends records, returns count
- All modules have `Close()` method for resource cleanup

**Testing Strategy:**
- Use table-driven tests for multiple scenarios
- Create mock modules for testing
- Test error propagation through pipeline
- Test data flow between modules
- Verify deterministic execution

**Performance Considerations:**
- Pipeline execution should be efficient (no unnecessary allocations)
- Handle large datasets (streaming may be needed in future)
- Logging should not block execution
- Error handling should be fast (fail fast on errors)

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Debug Log References

- No debug issues encountered

### Completion Notes List

- Implemented `Executor.Execute()` method with full Input → Filter → Output orchestration
- Created comprehensive mock modules for testing (MockInputModule, MockFilterModule, MockOutputModule)
- Implemented error handling with graceful failure and structured error results
- Added execution result tracking (timestamps, record counts, status)
- Created configuration converter (`ConvertToPipeline`) to transform parsed config to Pipeline struct
- Integrated with CLI `run` command with `--dry-run` flag support
- Added stub modules in CLI for testing until Epic 3 implements real modules
- Implemented structured JSON logging using internal logger package
- All 94 tests passing across all packages
- Red-green-refactor cycle followed for all implementations

### File List

**New Files:**
- `canectors-runtime/internal/runtime/pipeline_test.go` - Unit tests for Executor (12 tests)
- `canectors-runtime/internal/config/converter.go` - Configuration to Pipeline converter
- `canectors-runtime/internal/config/converter_test.go` - Converter tests (11 tests)

**Modified Files:**
- `canectors-runtime/internal/runtime/pipeline.go` - Executor implementation with logging
- `canectors-runtime/cmd/canectors/main.go` - CLI run command integration with stub modules (committed in prior session)
- `canectors-runtime/cmd/canectors/main_test.go` - CLI integration tests including dry-run (14 tests)

### Change Log

- 2026-01-15: Story 2.3 implementation complete - Pipeline orchestration with Input → Filter → Output execution, error handling, result tracking, CLI integration, and structured logging
- 2026-01-15: [Code Review] Added missing CLI dry-run integration test (TestCLI_RunDryRun), updated File List to include main_test.go, corrected test count to 94
