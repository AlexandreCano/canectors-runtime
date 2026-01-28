# Story 2.1: Initialize Go CLI Project Structure

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,  
I want to have a Go CLI project with proper structure following Go best practices,  
So that I can build a portable runtime for executing connector pipelines that works cross-platform.

## Acceptance Criteria

**Given** I am setting up the CLI runtime project  
**When** I initialize the Go project structure  
**Then** The project follows Go best practices with:
- `/cmd/cannectors/` for main entry point
- `/internal/modules/` for Input/Filter/Output modules
- `/internal/runtime/` for pipeline execution engine
- `/internal/config/` for configuration parsing and validation
- `/internal/scheduler/` for CRON scheduling
- `/internal/logger/` for logging functionality
- `/pkg/connector/` for public types

**And** The project includes `go.mod` with proper module name  
**And** The project includes `.gitignore` for Go artifacts  
**And** The project includes basic README with setup instructions  
**And** The project is ready for cross-platform compilation (NFR39)

## Tasks / Subtasks

- [x] Task 1: Initialize Go module and project structure (AC: All)
  - [x] Create project directory `cannectors-runtime/` (separate from Next.js project)
  - [x] Initialize Go module: `go mod init github.com/cannectors/runtime` (or appropriate module path)
  - [x] Create directory structure: `/cmd/`, `/internal/`, `/pkg/`, `/configs/`
  - [x] Create subdirectories: `/cmd/cannectors/`, `/internal/modules/input/`, `/internal/modules/filter/`, `/internal/modules/output/`, `/internal/runtime/`, `/internal/config/`, `/internal/scheduler/`, `/internal/logger/`, `/pkg/connector/`
  - [x] Create `.gitignore` for Go artifacts (binaries, test coverage, vendor, etc.)
  - [x] Verify structure matches architecture specification
- [x] Task 2: Create main entry point (AC: All)
  - [x] Create `/cmd/cannectors/main.go` with basic CLI structure
  - [x] Set up command-line argument parsing (use `cobra` or `flag` package)
  - [x] Add basic help command
  - [x] Add version command
  - [x] Structure for future commands: `run`, `validate`, `list`
- [x] Task 3: Create basic package structure (AC: All)
  - [x] Create `/pkg/connector/types.go` with basic connector types (will be expanded in Story 2.2)
  - [x] Create placeholder files in `/internal/` directories to establish structure
  - [x] Add package documentation comments
  - [x] Ensure all packages compile without errors
- [x] Task 4: Setup development tooling (AC: All)
  - [x] Create `Makefile` or build scripts for common tasks (build, test, lint, clean)
  - [x] Configure `gofmt` and `golint` (or `golangci-lint`) for code quality
  - [x] Add GitHub Actions workflow for CI/CD (build, test, cross-platform compilation)
  - [x] Document build process in README
- [x] Task 5: Create README and documentation (AC: README with setup instructions)
  - [x] Write README.md with project overview
  - [x] Document project structure and purpose of each directory
  - [x] Add setup instructions (Go version requirements, installation steps)
  - [x] Add build instructions for cross-platform compilation
  - [x] Document development workflow
  - [x] Add links to architecture document and related epics

## Dev Notes

### Architecture Requirements

**Project Location:**
- Separate project: `cannectors-runtime/` (independent from Next.js `cannectors/` project)
- Root directory: At same level as main `cannectors/` project or in subdirectory
- This is a standalone Go project that will be deployed separately (Docker workers on Railway/Fly.io)

**Go Module Structure:**
- Module name: `github.com/cannectors/runtime` (or appropriate path based on repository structure)
- Go version: Latest stable (1.21+ recommended for 2026)
- Follow standard Go project layout: https://github.com/golang-standards/project-layout

**Directory Structure (from Architecture):**
```
cannectors-runtime/
├── README.md
├── go.mod
├── go.sum
├── .gitignore
├── .github/
│   └── workflows/
│       └── ci.yml
├── cmd/
│   └── cannectors/
│       └── main.go
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   ├── validator.go
│   │   └── config_test.go
│   ├── modules/
│   │   ├── input/
│   │   │   ├── http_polling.go
│   │   │   ├── webhook.go
│   │   │   └── input_test.go
│   │   ├── filter/
│   │   │   ├── mapping.go
│   │   │   ├── condition.go
│   │   │   └── filter_test.go
│   │   └── output/
│   │       ├── http_request.go
│   │       └── output_test.go
│   ├── runtime/
│   │   ├── pipeline.go
│   │   ├── executor.go
│   │   └── runtime_test.go
│   ├── scheduler/
│   │   ├── cron.go
│   │   └── scheduler_test.go
│   └── logger/
│       └── logger.go
├── pkg/
│   └── connector/
│       └── types.go
└── configs/
    └── example-connector.json
```

**Key Architectural Decisions:**
- **Language:** Go (latest stable) for portability and performance [Source: _bmad-output/planning-artifacts/architecture.md#Runtime CLI Architecture]
- **CRON Library:** `robfig/cron` for scheduling [Source: _bmad-output/planning-artifacts/architecture.md#CRON Scheduler]
- **Structure:** Standard Go project layout with `/cmd/`, `/internal/`, `/pkg/` separation
- **Cross-platform:** Must compile for Windows, Mac, Linux (NFR39)
- **Portability:** Runtime must be portable and deterministic (NFR24, NFR25)

### Project Structure Notes

**Go Best Practices:**
- Use `/internal/` for private packages (not importable by external projects)
- Use `/pkg/` for public packages (importable by external projects)
- Use `/cmd/` for application entry points
- Keep package names short and descriptive
- Follow Go naming conventions: `camelCase` for exported, `camelCase` for unexported

**Integration Points:**
- Will use JSON Schema from Story 1.1 for validation (Story 2.2)
- Will parse pipeline configurations from JSON/YAML files (Story 2.2)
- Will execute modules Input/Filter/Output (Epic 3)
- Will use CRON scheduler for polling (Story 4.1)
- Will be deployed as Docker workers (Railway/Fly.io)

**Dependencies to Add (in future stories):**
- JSON Schema validation library (for Story 2.2)
- YAML parser (for Story 2.2)
- `robfig/cron` for CRON scheduling (for Story 4.1)
- HTTP client library (for Epic 3)
- Logging library (structured logging)

### Testing Requirements

**Project Structure Testing:**
- Verify all directories exist and are properly organized
- Verify Go module initializes correctly
- Verify project compiles without errors
- Verify `.gitignore` excludes appropriate files

**Build Testing:**
- Test `go build` succeeds
- Test cross-platform compilation (GOOS/GOARCH)
- Test CI/CD workflow runs successfully
- Verify build artifacts are correct

**Documentation Testing:**
- Verify README is complete and accurate
- Verify setup instructions work for new developers
- Verify build instructions are clear

### References

- **Epic 2 Overview:** [Source: _bmad-output/planning-artifacts/epics.md#Epic 2: CLI Runtime Foundation]
- **Story 2.1 Details:** [Source: _bmad-output/planning-artifacts/epics.md#Story 2.1: Initialize Go CLI Project Structure]
- **CLI Runtime Architecture:** [Source: _bmad-output/planning-artifacts/architecture.md#Runtime CLI (Go) - Separate Project]
- **Project Structure:** [Source: _bmad-output/planning-artifacts/architecture.md#Complete Project Directory Structure]
- **Go Project Layout:** https://github.com/golang-standards/project-layout
- **Project Context:** [Source: _bmad-output/project-context.md]
- **PRD Runtime Architecture:** [Source: _bmad-output/planning-artifacts/prd.md#Runtime Architecture]

### Critical Implementation Rules

**From Project Context:**
- Runtime CLI is separate project (Go) from Next.js application [Source: _bmad-output/project-context.md#Technology Stack]
- Cross-platform compilation required (Windows, Mac, Linux) [Source: _bmad-output/project-context.md#Development Workflow Rules]
- Follow Go best practices and conventions

**From Architecture:**
- Structure: `/cmd/`, `/internal/`, `/pkg/`, `/configs/` [Source: _bmad-output/planning-artifacts/architecture.md#CLI Runtime (Go)]
- Modules: `/internal/modules/{type}/` for Input/Filter/Output [Source: _bmad-output/planning-artifacts/architecture.md#CLI Runtime (Go)]
- Config parsing: `/internal/config/` for validation JSON/YAML [Source: _bmad-output/planning-artifacts/architecture.md#CLI Runtime (Go)]
- CRON library: `robfig/cron` [Source: _bmad-output/planning-artifacts/architecture.md#CRON Scheduler for Polling]

**Go-Specific Rules:**
- Use standard Go project layout
- Keep packages focused and cohesive
- Use `internal/` for private code
- Use `pkg/` only for truly public APIs
- Follow Go naming conventions
- Write tests alongside code (`*_test.go`)

### Library and Framework Requirements

**Go Standard Library:**
- `flag` or `cobra` for CLI argument parsing
- `encoding/json` for JSON parsing (Story 2.2)
- `os`, `path/filepath` for file operations
- `context` for cancellation and timeouts

**External Dependencies (to be added in future stories):**
- JSON Schema validator (for Story 2.2 - research Go libraries)
- YAML parser: `gopkg.in/yaml.v3` (for Story 2.2)
- CRON scheduler: `github.com/robfig/cron/v3` (for Story 4.1)
- HTTP client: `net/http` (standard library) or `github.com/go-resty/resty/v2` (for Epic 3)
- Logging: `log/slog` (Go 1.21+) or structured logging library

**Build Tools:**
- `gofmt` for formatting
- `golangci-lint` for linting (recommended)
- `go test` for testing
- `go build` for compilation

### Previous Story Intelligence

**Epic 1 Stories (Completed):**
- **Story 1.1:** JSON Schema for pipeline configurations created at `/types/pipeline-schema.json` [Source: _bmad-output/implementation-artifacts/1-1-define-pipeline-configuration-schema.md]
- **Story 1.2:** Configuration validator created with Ajv (TypeScript) [Source: _bmad-output/implementation-artifacts/1-2-create-configuration-validator.md]
- **Story 1.3:** YAML alternative format support added [Source: _bmad-output/implementation-artifacts/1-3-support-yaml-alternative-format.md]

**Key Learnings from Epic 1:**
- Pipeline configuration format is JSON Schema Draft 2020-12
- Schema location: `/types/pipeline-schema.json` in Next.js project
- Schema uses extensible type system (not fixed enums) for future module types
- Schema validates type-specific required fields using conditional validation (`if/then/else`)
- Configuration format supports: connector metadata, input module, filter modules array, output module, authentication, error handling, CRON scheduling
- Format is designed for evolution and backward compatibility

**Integration with Epic 1:**
- Story 2.2 will need to validate configurations against the JSON Schema
- Go runtime will need to parse the same JSON Schema (or use equivalent validation)
- Consider: Should Go runtime use the same JSON Schema file, or have a Go-native validation?
- Recommendation: Use JSON Schema file from Next.js project for consistency, or generate Go types from schema

**Epic Context:**
- Epic 2 is Priority 2 (CLI Runtime) - Partie 1/3
- Must be completed after Epic 1 (Format Configuration) is done ✅
- Epic 2 has 3 stories: 2.1 (this story), 2.2 (Configuration Parser), 2.3 (Pipeline Orchestration)
- This story establishes the foundation for all CLI runtime work

### Git Intelligence Summary

**Recent Work:**
- Epic 1 completed: Pipeline configuration schema and validation implemented
- No previous Go CLI runtime work detected
- This is greenfield Go project initialization

**Repository Structure:**
- Main project: `cannectors/` (Next.js T3 Stack)
- Runtime project: `cannectors-runtime/` (Go - to be created)
- Both projects should be in same repository or separate repositories (decision needed)

### Latest Technical Information

**Go Version:**
- Go 1.21+ recommended (latest stable as of 2026)
- Use `go mod` for dependency management
- Support Go modules (standard since Go 1.11)

**Go Project Layout:**
- Follow standard Go project layout: https://github.com/golang-standards/project-layout
- Use `/internal/` for private packages
- Use `/pkg/` for public packages
- Use `/cmd/` for application entry points

**CLI Framework Options:**
- `flag` (standard library) - simple, built-in
- `cobra` (spf13/cobra) - feature-rich, used by kubectl, docker, etc.
- Recommendation: Start with `cobra` for better CLI structure and future extensibility

**Cross-Platform Compilation:**
- Use `GOOS` and `GOARCH` environment variables
- Example: `GOOS=windows GOARCH=amd64 go build`
- CI/CD should build for all platforms: windows/amd64, darwin/amd64, darwin/arm64, linux/amd64

**CI/CD for Go:**
- GitHub Actions workflow for building and testing
- Cross-platform builds in CI
- Test coverage reporting
- Linting with `golangci-lint`

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Debug Log References

- Installed Go 1.23.5 in user home directory (~/go)
- Used GOPATH=$HOME/gopath to avoid GOROOT conflict

### Completion Notes List

- **Task 1**: Created `cannectors-runtime/` project at `/home/alexandrecano/Workspace/cannectors-runtime/` with complete directory structure matching architecture specification. Go module initialized as `github.com/cannectors/runtime` with Go 1.23.5.

- **Task 2**: Implemented main entry point using Cobra CLI framework (v1.10.2). CLI includes: help, version, run (with --dry-run flag), validate, and list (with --type filter) commands. Version info supports ldflags injection for build metadata.

- **Task 3**: Created all placeholder packages with proper Go documentation comments. Types defined in `pkg/connector/types.go` include: Pipeline, ModuleConfig, AuthConfig, ErrorHandling, ExecutionResult, ExecutionError. All packages compile without errors.

- **Task 4**: Created comprehensive Makefile with targets for build, test, coverage, format, vet, lint, and cross-platform compilation. GitHub Actions CI workflow configured with lint, test, and multi-platform build jobs. Cross-platform builds verified for Linux amd64, macOS amd64/arm64, and Windows amd64.

- **Task 5**: Created README.md with complete documentation including project overview, structure, installation, usage, development workflow, and roadmap. Example configuration file created at `configs/example-connector.json`.

### Tests Created

- `pkg/connector/types_test.go`: 4 tests for JSON serialization of Pipeline, ModuleConfig, ExecutionResult types
- `internal/logger/logger_test.go`: 5 tests for logger initialization, level setting, and context methods
- All 9 tests pass

### File List

**New Files Created (in /home/alexandrecano/Workspace/cannectors-runtime/):**
- `go.mod` - Go module definition
- `go.sum` - Go dependency checksums
- `.gitignore` - Git ignore patterns for Go artifacts
- `.editorconfig` - Editor configuration for code style (added in review)
- `.golangci.yml` - Linter configuration (added in review)
- `LICENSE` - MIT License (added in review)
- `CONTRIBUTING.md` - Contribution guidelines (added in review)
- `Makefile` - Build automation
- `README.md` - Project documentation
- `cmd/cannectors/main.go` - CLI entry point with Cobra
- `internal/config/config.go` - Configuration loader placeholder
- `internal/runtime/pipeline.go` - Pipeline executor placeholder
- `internal/modules/input/input.go` - Input module placeholders
- `internal/modules/filter/filter.go` - Filter module placeholders
- `internal/modules/output/output.go` - Output module placeholders
- `internal/scheduler/scheduler.go` - CRON scheduler placeholder
- `internal/logger/logger.go` - Structured logging with slog
- `internal/logger/logger_test.go` - Logger tests
- `pkg/connector/types.go` - Public connector types
- `pkg/connector/types_test.go` - Types tests
- `configs/example-connector.json` - Example pipeline configuration
- `.github/workflows/ci.yml` - GitHub Actions CI/CD workflow

### Change Log

- **2026-01-15**: Story 2.1 implemented - Go CLI project structure initialized with all directories, CLI entry point (Cobra), package placeholders, Makefile, CI/CD workflow, and documentation. Cross-platform compilation verified for Linux, macOS, and Windows.
- **2026-01-15**: Code Review completed (Dev Agent Amelia). Fixes applied:
  - Fixed README.md Go version (1.21 → 1.23) and broken architecture link
  - Added LICENSE file (MIT)
  - Added CONTRIBUTING.md
  - Added .golangci.yml linter configuration
  - Added .editorconfig for code style consistency
  - Fixed all placeholder functions to return proper `ErrNotImplemented` errors instead of `nil, nil`
  - Git repository initialized by user
