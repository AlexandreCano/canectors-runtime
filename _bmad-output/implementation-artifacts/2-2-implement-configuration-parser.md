# Story 2.2: Implement Configuration Parser

Status: review

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,  
I want the CLI to parse and validate pipeline configuration files,  
So that I can load connector declarations before execution.

## Acceptance Criteria

**Given** I have a pipeline configuration file (JSON or YAML)  
**When** I parse the configuration file  
**Then** The parser correctly reads JSON format (FR21)  
**And** The parser correctly reads YAML format (FR21)  
**And** The parser validates the configuration against the schema from Epic 1  
**And** The parser returns clear error messages for invalid configurations (FR49)  
**And** The parser returns structured configuration data ready for execution  
**And** The parsing is fast (<1 second for typical configurations)

## Tasks / Subtasks

- [x] Task 1: Implement JSON parsing (AC: read JSON format, fast parsing)
  - [x] Use Go standard library `encoding/json` for JSON parsing
  - [x] Implement `ParseJSON` function that accepts file path or content string
  - [x] Handle file I/O errors gracefully with clear messages
  - [x] Handle JSON syntax errors with line/column information when available
  - [x] Return structured `ParseResult` type with parsed data and errors
  - [x] Ensure parsing completes in <1 second for typical configs (100-500 lines)
  - [x] Add unit tests for valid JSON, invalid JSON, file errors
- [x] Task 2: Implement YAML parsing (AC: read YAML format, fast parsing)
  - [x] Install `gopkg.in/yaml.v3` library for YAML parsing
  - [x] Implement `ParseYAML` function that accepts file path or content string
  - [x] Handle file I/O errors gracefully with clear messages
  - [x] Handle YAML syntax errors with line/column information
  - [x] Return structured `ParseResult` type with parsed data and errors
  - [x] Support both YAML 1.1 and 1.2 standards
  - [x] Ensure parsing completes in <1 second for typical configs
  - [x] Add unit tests for valid YAML, invalid YAML, file errors
- [x] Task 3: Integrate JSON Schema validation (AC: validate against schema, clear errors)
  - [x] Research and select Go JSON Schema validator library (e.g., `xeipuuv/gojsonschema` or `qri-io/jsonschema`)
  - [x] Install JSON Schema validator library
  - [x] Load pipeline schema from `/types/pipeline-schema.json` (or embedded schema)
  - [x] Implement `ValidateConfig` function that validates parsed config against schema
  - [x] Format validation errors with clear messages (path, type, expected value)
  - [x] Return structured `ValidationResult` type with valid status and errors array
  - [x] Handle schema loading errors gracefully
  - [x] Add unit tests for valid configs, invalid configs, schema errors
- [x] Task 4: Create unified configuration parser (AC: all above, structured data)
  - [x] Implement `ParseConfig` function that auto-detects format (JSON/YAML) by file extension or content
  - [x] Combine parsing and validation into single operation
  - [x] Return unified `ConfigResult` type with parsed data, validation status, and all errors
  - [x] Implement error aggregation (parsing errors + validation errors)
  - [x] Add helper functions for format detection (`IsJSON`, `IsYAML`)
  - [x] Add unit tests for format auto-detection, unified parsing
- [x] Task 5: Integrate with CLI command (AC: all above, CLI integration)
  - [x] Update `validate` command in `/cmd/canectors/main.go` to use new parser
  - [x] Update `run` command to use parser for loading configs before execution
  - [x] Display parsing and validation errors in user-friendly format
  - [x] Exit with appropriate error codes (0 = success, 1 = validation error, 2 = parsing error)
  - [x] Add command-line flags for verbose error output
  - [x] Update CLI help text with usage examples
  - [x] Add integration tests for CLI commands with valid/invalid configs

## Dev Notes

### Architecture Requirements

**Go Module Structure:**
- Parser implementation: `/internal/config/parser.go`
- Validation implementation: `/internal/config/validator.go`
- Types and interfaces: `/internal/config/types.go` (or extend `/pkg/connector/types.go`)
- Tests: `/internal/config/parser_test.go`, `/internal/config/validator_test.go`

**Schema Location:**
- Pipeline schema: `/types/pipeline-schema.json` in Next.js project (Epic 1)
- Options for Go runtime:
  1. **Embed schema at build time** (recommended) - Use `embed` package to include schema in binary
  2. **Copy schema to runtime project** - Mirror schema file in `canectors-runtime/configs/pipeline-schema.json`
  3. **Fetch schema from API** - Not recommended for CLI runtime (portability, offline support)
- **Recommendation:** Embed schema using Go 1.16+ `embed` package for single-binary distribution

**Library Choices:**
- **JSON parsing:** `encoding/json` (Go standard library)
- **YAML parsing:** `gopkg.in/yaml.v3` (most common, stable, maintained)
- **JSON Schema validation:** Research required:
  - `github.com/xeipuuv/gojsonschema` - Popular, actively maintained
  - `github.com/qri-io/jsonschema` - Modern, well-documented
  - **Decision needed:** Choose based on JSON Schema Draft 2020-12 support, performance, error messages

**Error Handling:**
- All parsing errors should include file path, line/column when available
- Validation errors should include JSON path (e.g., `/connector/input/endpoint`), error type, expected value
- Use Go `error` interface consistently, return structured error types for detailed error handling

### Project Structure Notes

**File Organization:**
```
canectors-runtime/
├── internal/
│   └── config/
│       ├── parser.go          # JSON/YAML parsing
│       ├── parser_test.go
│       ├── validator.go       # JSON Schema validation
│       ├── validator_test.go
│       └── types.go           # ParseResult, ValidationResult, ConfigResult types
├── configs/
│   └── pipeline-schema.json   # Schema file (if not embedded)
└── cmd/
    └── canectors/
        └── main.go            # CLI commands using parser
```

**Integration Points:**
- Parser will be used by:
  - `validate` command - validate config files before use
  - `run` command - parse and validate configs before execution
  - Future commands that need config parsing
- Parser depends on:
  - Pipeline schema from Epic 1 (Story 1.1)
  - Go YAML library (`gopkg.in/yaml.v3`)
  - Go JSON Schema validator (to be selected)

**Dependencies to Add:**
```go
// go.mod additions
require (
    gopkg.in/yaml.v3 v3.0.1
    github.com/xeipuuv/gojsonschema v1.2.0  // OR qri-io/jsonschema - decision needed
)
```

### Testing Requirements

**Unit Tests:**
- JSON parsing: valid JSON, invalid JSON, file errors, empty files
- YAML parsing: valid YAML, invalid YAML, file errors, empty files
- Format detection: JSON files, YAML files, ambiguous content
- Schema validation: valid configs, invalid configs (missing fields, wrong types, invalid values)
- Error formatting: parsing errors, validation errors, aggregated errors

**Integration Tests:**
- CLI `validate` command with valid/invalid configs
- CLI `run` command with valid/invalid configs
- Error output formatting and exit codes

**Test Data:**
- Create test fixtures in `/internal/config/testdata/`:
  - `valid-config.json`
  - `valid-config.yaml`
  - `invalid-json.json`
  - `invalid-yaml.yaml`
  - `invalid-schema.json` (valid JSON but invalid schema)
  - `missing-required.json`

**Performance Tests:**
- Measure parsing time for typical configs (100-500 lines)
- Target: <1 second for parsing + validation
- Test with large configs (1000+ lines) to ensure scalability

### References

- **Epic 2 Overview:** [Source: _bmad-output/planning-artifacts/epics.md#Epic 2: CLI Runtime Foundation]
- **Story 2.2 Details:** [Source: _bmad-output/planning-artifacts/epics.md#Story 2.2: Implement Configuration Parser]
- **Story 2.1 (Previous):** [Source: _bmad-output/implementation-artifacts/2-1-initialize-go-cli-project-structure.md]
- **Schema Definition (Epic 1):** [Source: _bmad-output/implementation-artifacts/1-1-define-pipeline-configuration-schema.md]
- **Schema File:** [Source: types/pipeline-schema.json]
- **Validator (TypeScript/Epic 1):** [Source: _bmad-output/implementation-artifacts/1-2-create-configuration-validator.md]
- **YAML Support (Epic 1):** [Source: _bmad-output/implementation-artifacts/1-3-support-yaml-alternative-format.md]
- **CLI Runtime Architecture:** [Source: _bmad-output/planning-artifacts/architecture.md#Runtime CLI (Go) - Separate Project]
- **Project Context:** [Source: _bmad-output/project-context.md]
- **PRD Requirements:** [Source: _bmad-output/planning-artifacts/prd.md#Runtime Architecture]

### Critical Implementation Rules

**From Project Context:**
- Runtime CLI is separate Go project from Next.js application [Source: _bmad-output/project-context.md#Technology Stack]
- Cross-platform compilation required (Windows, Mac, Linux) [Source: _bmad-output/project-context.md#Development Workflow Rules]
- Follow Go best practices and conventions [Source: _bmad-output/project-context.md#Critical Implementation Rules]

**From Architecture:**
- Config parsing: `/internal/config/` for validation JSON/YAML [Source: _bmad-output/planning-artifacts/architecture.md#CLI Runtime (Go)]
- Pipeline schema: JSON Schema Draft 2020-12 [Source: _bmad-output/planning-artifacts/architecture.md#Pipeline Configuration Format]
- Format support: JSON primary, YAML alternative [Source: _bmad-output/planning-artifacts/architecture.md#Pipeline Configuration Format]

**From Epic 1 Learnings:**
- Pipeline schema location: `/types/pipeline-schema.json` in Next.js project
- Schema version: 1.1.0 (current), 1.0.0 (supported) [Source: docs/pipeline-configuration-schema.md]
- Schema uses extensible type system (not fixed enums) for module types
- Schema validates type-specific required fields using conditional validation (`if/then/else`)
- Configuration format supports: connector metadata, input module, filter modules array, output module, authentication, error handling, CRON scheduling
- Format is designed for evolution and backward compatibility

**Go-Specific Rules:**
- Use standard Go project layout
- Keep packages focused and cohesive
- Use `internal/` for private code
- Follow Go naming conventions (`camelCase` for exported, `camelCase` for unexported)
- Write tests alongside code (`*_test.go`)
- Use `embed` package for schema embedding (Go 1.16+)
- Handle errors explicitly (no silent failures)
- Return structured error types for detailed error information

### Library and Framework Requirements

**Go Standard Library:**
- `encoding/json` - JSON parsing
- `os`, `path/filepath` - File operations
- `embed` - Schema embedding (Go 1.16+)
- `errors` - Error wrapping
- `fmt` - Error formatting

**External Dependencies:**
- `gopkg.in/yaml.v3` - YAML parsing (v3.0.1 or later)
- JSON Schema validator (decision needed):
  - `github.com/xeipuuv/gojsonschema` v1.2.0+ - Popular option
  - `github.com/qri-io/jsonschema` v0.4.0+ - Modern alternative
  - **Research required:** Check JSON Schema Draft 2020-12 support, error message quality, performance

**Build Tools:**
- `go test` - Testing
- `go build` - Compilation
- `go mod tidy` - Dependency management

### Previous Story Intelligence

**Epic 1 Stories (Completed):**
- **Story 1.1:** JSON Schema for pipeline configurations created at `/types/pipeline-schema.json` [Source: _bmad-output/implementation-artifacts/1-1-define-pipeline-configuration-schema.md]
  - Schema uses JSON Schema Draft 2020-12
  - Schema supports versioning (`schemaVersion` field)
  - Schema validates type-specific required fields conditionally
  - Schema is extensible for future module types
- **Story 1.2:** Configuration validator created with Ajv (TypeScript) [Source: _bmad-output/implementation-artifacts/1-2-create-configuration-validator.md]
  - Validator uses Ajv with JSON Schema Draft 2020-12
  - Validator reports errors with JSON path, type, expected value
  - Validator supports both JSON and YAML formats
  - Validation completes in <1 second for typical configs
  - **Key learning:** Schema validation requires workaround for Ajv 8.x (removes `$schema` property)
- **Story 1.3:** YAML alternative format support added [Source: _bmad-output/implementation-artifacts/1-3-support-yaml-alternative-format.md]
  - YAML parser uses `yaml` package (TypeScript)
  - YAML parser handles both YAML 1.1 and 1.2 standards
  - YAML configs validated against same schema as JSON
  - YAML parsing errors include line/column information

**Key Learnings from Epic 1:**
- Schema location: `/types/pipeline-schema.json` in Next.js project (root level)
- Schema format: JSON Schema Draft 2020-12 (but Ajv 8.x doesn't validate meta-schema)
- Schema validation approach: Load schema, remove `$schema` property if needed, validate config
- Error formatting: JSON path (e.g., `/connector/input/endpoint`), error type, expected value, suggestion
- Format support: Both JSON and YAML are first-class, validated against same schema
- Performance: Parsing + validation should complete in <1 second for typical configs

**Story 2.1 (Previous in Epic 2):**
- Go CLI project initialized at `/home/alexandrecano/Workspace/canectors-runtime/` [Source: _bmad-output/implementation-artifacts/2-1-initialize-go-cli-project-structure.md]
- Project structure: `/cmd/canectors/`, `/internal/`, `/pkg/connector/`
- CLI framework: Cobra (v1.10.2) for command-line parsing
- Types defined: `Pipeline`, `ModuleConfig`, `AuthConfig`, `ErrorHandling`, `ExecutionResult` in `/pkg/connector/types.go`
- Logger implemented: Structured logging with `log/slog` (Go 1.21+)
- **Key learning:** Project uses Go 1.23.5, follows standard Go project layout

**Integration with Epic 1:**
- Story 2.2 needs to use the same schema as Epic 1 (`/types/pipeline-schema.json`)
- Go runtime should validate against same schema for consistency
- Options: Embed schema, copy to runtime project, or fetch from API (not recommended)
- Recommendation: Embed schema using Go `embed` package for portability

**Epic Context:**
- Epic 2 is Priority 2 (CLI Runtime) - Partie 1/3
- Epic 2 has 3 stories: 2.1 (Done ✅), 2.2 (This story), 2.3 (Pipeline Orchestration)
- Story 2.2 must complete before Story 2.3 (orchestration needs parsed configs)

### Git Intelligence Summary

**Recent Work:**
- Epic 1 completed: Pipeline configuration schema and validation implemented (TypeScript)
- Story 2.1 completed: Go CLI project structure initialized
- No previous Go configuration parser work detected
- This is greenfield Go parsing implementation

**Repository Structure:**
- Main project: `canectors/` (Next.js T3 Stack)
- Runtime project: `canectors-runtime/` (Go - separate project)
- Schema location: `/types/pipeline-schema.json` in main project

**Files from Previous Stories:**
- `/types/pipeline-schema.json` - Pipeline schema (Epic 1)
- `/utils/pipeline-validator.ts` - TypeScript validator (Epic 1)
- `/utils/yaml-parser.ts` - TypeScript YAML parser (Epic 1)
- `/canectors-runtime/cmd/canectors/main.go` - CLI entry point (Story 2.1)
- `/canectors-runtime/pkg/connector/types.go` - Go types (Story 2.1)

### Latest Technical Information

**Go JSON Schema Validation:**
- Research needed: JSON Schema Draft 2020-12 support in Go libraries
- `github.com/xeipuuv/gojsonschema` - Popular, supports Draft 7, check Draft 2020-12 support
- `github.com/qri-io/jsonschema` - Modern, check Draft 2020-12 support
- Consider: If Draft 2020-12 not fully supported, may need to adapt schema or use fallback validation

**Go YAML Libraries:**
- `gopkg.in/yaml.v3` - Most common, stable, maintained (recommended)
- Alternative: `github.com/go-yaml/yaml` (less maintained)
- Recommendation: Use `gopkg.in/yaml.v3` v3.0.1 or later

**Schema Embedding:**
- Go 1.16+ supports `embed` package for embedding files at build time
- Pattern:
  ```go
  //go:embed pipeline-schema.json
  var schemaBytes []byte
  ```
- Benefits: Single binary, no external file dependency, works offline
- Consider: Schema updates require rebuild (acceptable for CLI runtime)

**Error Handling Patterns:**
- Use Go `error` interface consistently
- Create custom error types for structured errors:
  ```go
  type ParseError struct {
      Path    string
      Line    int
      Column  int
      Message string
  }
  
  type ValidationError struct {
      Path     string
      Type     string
      Expected string
      Message  string
  }
  ```
- Wrap errors for context: `fmt.Errorf("parsing config: %w", err)`

**Performance Considerations:**
- Cache compiled schema validator (single compilation per runtime instance)
- Use streaming JSON/YAML parsing for large files if needed
- Profile parsing and validation with typical configs (100-500 lines)
- Target: <1 second total (parsing + validation)

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (via Cursor)

### Debug Log References

N/A - No debugging issues encountered

### Completion Notes List

- **Task 1 (JSON Parsing):** Implemented `ParseJSONFile` and `ParseJSONString` functions using Go standard library `encoding/json`. Added `ParseResult` and `ParseError` types with line/column information for syntax errors. 10 unit tests covering valid JSON, invalid JSON, empty files, and performance.

- **Task 2 (YAML Parsing):** Implemented `ParseYAMLFile` and `ParseYAMLString` functions using `gopkg.in/yaml.v3`. YAML errors include line information when available. 10 unit tests covering valid YAML, invalid YAML, empty files, comments-only files, and performance.

- **Task 3 (JSON Schema Validation):** Selected `github.com/santhosh-tekuri/jsonschema/v6` library for Draft 2020-12 support. Embedded pipeline schema using Go `embed` package. Implemented `ValidateConfig` function returning structured `ValidationResult`. 7 unit tests covering valid configs, missing required fields, wrong types, nil/empty data.

- **Task 4 (Unified Parser):** Implemented `ParseConfig` and `ParseConfigString` functions with auto-detection of JSON/YAML format. Added helper functions `DetectFormat`, `IsJSON`, `IsYAML`. Combined parsing and validation into single operation with `ConfigResult` type. 15 unit tests covering format detection, unified parsing, and auto-detection.

- **Task 5 (CLI Integration):** Created Cobra-based CLI with `validate` and `run` commands. Implemented user-friendly error output with `--verbose` and `--quiet` flags. Exit codes: 0 (success), 1 (validation error), 2 (parse error), 3 (runtime error). 13 integration tests covering all CLI commands and flags.

### File List

**New Files:**
- `internal/config/types.go` - ParseResult, ParseError, ValidationResult, ValidationError, ConfigResult types
- `internal/config/parser.go` - JSON/YAML parsing functions, unified parser, format detection
- `internal/config/validator.go` - JSON Schema validation with embedded schema
- `internal/config/parser_test.go` - Unit tests for parsing (40+ tests)
- `internal/config/validator_test.go` - Unit tests for validation (7 tests)
- `internal/config/schema/pipeline-schema.json` - Embedded pipeline schema
- `internal/config/testdata/valid-config.json` - Test fixture
- `internal/config/testdata/valid-config.yaml` - Test fixture
- `internal/config/testdata/valid-schema-config.json` - Test fixture
- `internal/config/testdata/invalid-json.json` - Test fixture
- `internal/config/testdata/invalid-yaml.yaml` - Test fixture
- `internal/config/testdata/invalid-schema-missing-required.json` - Test fixture
- `internal/config/testdata/invalid-schema-wrong-type.json` - Test fixture
- `internal/config/testdata/empty.json` - Test fixture
- `internal/config/testdata/empty.yaml` - Test fixture
- `cmd/canectors/main.go` - CLI entry point with validate and run commands
- `cmd/canectors/main_test.go` - CLI integration tests (13 tests)

**Modified Files:**
- `go.mod` - Added dependencies: gopkg.in/yaml.v3, github.com/santhosh-tekuri/jsonschema/v6
- `go.sum` - Updated with new dependency checksums

**Deleted Files:**
- `internal/config/config.go` - Replaced by new parser.go and types.go

## Senior Developer Review (AI)

**Review Date:** 2026-01-15
**Reviewer:** Amelia (Dev Agent - Claude Opus 4.5)
**Outcome:** ✅ APPROVED (after fixes)

### Issues Found & Fixed

**HIGH Severity (3):**
1. ✅ FIXED: Deleted obsolete `internal/config/config.go` file with stub code
2. ✅ FIXED: Removed unused `formatPath` function from `cmd/canectors/main.go`
3. ✅ FIXED: Added `sync.Once` for thread-safe schema compilation in `validator.go`

**MEDIUM Severity (4):**
4. ✅ FIXED: Added time assertions to performance tests (AC: <1 second)
5. ✅ FIXED: Added YAML 1.2 boolean/octal tests for version compliance
6. ✅ FIXED: Added tests for `AllErrors()` method (100% coverage)
7. ✅ FIXED: Fixed `valid-config.yaml` test fixture for full schema validation

**Coverage Improvement:**
- Before: 75.8%
- After: 83.8%

### New Tests Added
- `TestParseYAMLString_YAML12BooleanValues` - YAML 1.2 boolean handling
- `TestParseYAMLString_YAML12OctalNumbers` - YAML 1.2 octal numbers
- `TestConfigResult_AllErrors` - AllErrors() aggregation
- `TestConfigResult_AllErrors_Empty` - Empty errors
- `TestConfigResult_AllErrors_OnlyParseErrors` - Parse errors only
- `TestConfigResult_AllErrors_OnlyValidationErrors` - Validation errors only
- `TestParseError_Error` - ParseError.Error() formatting
- `TestValidationError_Error` - ValidationError.Error() formatting

## Change Log

| Date | Change | Author |
|------|--------|--------|
| 2026-01-15 | Story implementation complete - All 5 tasks implemented with 50+ tests | Dev Agent (Claude Opus 4.5) |
| 2026-01-15 | Code review fixes: 3 HIGH + 4 MEDIUM issues fixed, coverage 75.8%→83.8% | Amelia (Dev Agent - Code Review) |