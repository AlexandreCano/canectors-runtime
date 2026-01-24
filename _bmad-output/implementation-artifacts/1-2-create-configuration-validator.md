# Story 1.2: Create Configuration Validator

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,  
I want to validate pipeline configuration files against the schema,  
so that I can ensure configurations are syntactically correct before use.

## Acceptance Criteria

**Given** I have a pipeline configuration file (JSON, and YAML after Story 1.3)  
**When** I validate the configuration against the schema  
**Then** The validator reports all syntax errors with clear messages  
**And** The validator reports all semantic errors (missing required fields, invalid values)  
**And** The validator confirms when a configuration is valid  
**And** The validator supports JSON format (YAML support added after Story 1.3 completes)  
**And** The validation is fast (<1 second for typical configurations)

**Note:** YAML support will be added after Story 1.3 (Support YAML Alternative Format) is completed. This story focuses on JSON validation first, then extends to YAML using the YAML parsing utilities from Story 1.3.

## Tasks / Subtasks

- [x] Task 1: Implement JSON validation (AC: syntax, semantic, fast)
  - [x] Load pipeline-schema.json schema file
  - [x] Initialize Ajv validator with JSON Schema Draft 2020-12 support
  - [x] Add ajv-formats plugin for format validation (dates, URIs, etc.)
  - [x] Implement validateJSON function that takes JSON string/file path
  - [x] Report all validation errors with clear messages
  - [x] Return structured validation result (valid: boolean, errors: array)
  - [x] Ensure validation completes in <1 second for typical configs
- [x] Task 2: Extend validator for YAML support (AC: YAML format support - AFTER Story 1.3)
  - [x] **Prerequisite:** Story 1.3 must be completed first (YAML parsing utilities)
  - [x] Import YAML parsing utilities from Story 1.3 implementation
  - [x] Implement validateYAML function that uses YAML parser from Story 1.3
  - [x] Convert YAML to JSON using utilities from Story 1.3
  - [x] Validate converted JSON against schema
  - [x] Preserve YAML-specific error context in error messages
  - [x] Integrate YAML validation into main validatePipelineConfig function
- [x] Task 3: Implement file I/O and CLI interface (AC: file validation, fast)
  - [x] Implement file reading function (JSON files initially, YAML after Story 1.3)
  - [x] Auto-detect file format from extension (.json initially, .yaml/.yml after Story 1.3)
  - [x] Create CLI command or function: `validatePipelineConfig(filePath)`
  - [x] Format error output for CLI usage (human-readable messages)
  - [x] Format success output (confirmation message)
  - [x] Ensure file I/O doesn't slow down validation significantly
- [x] Task 4: Implement error reporting (AC: clear messages, all errors)
  - [x] Parse Ajv validation errors into structured format
  - [x] Extract field paths from JSON Schema error paths
  - [x] Generate user-friendly error messages with context
  - [x] Report all errors (not just first error - Ajv allErrors: true)
  - [x] Include error location (line, column if possible, JSON path)
  - [x] Include error type (syntax, missing required, invalid type, pattern mismatch)
  - [x] Provide suggestions for common errors
- [x] Task 5: Create validation utility and tests (AC: all ACs)
  - [x] Create validation utility module: `/utils/pipeline-validator.ts` or `/server/services/pipeline-validator/`
  - [x] Export public API: `validatePipelineConfig(filePath | configObject, format?: 'json' | 'yaml')`
  - [x] Write unit tests for valid configurations (all MVP module types)
  - [x] Write unit tests for invalid configurations (syntax errors, semantic errors)
  - [x] Write unit tests for edge cases (empty files, null values, missing required fields)
  - [x] Write unit tests for YAML parsing and validation
  - [x] Write integration tests for file-based validation
  - [x] Test performance (<1 second for typical configs)

## Dev Notes

### Architecture Requirements

**Validator Location:**
- Primary: `/utils/pipeline-validator.ts` (shared utility, follows T3 Stack structure)
- Alternative: `/server/services/pipeline-validator/` if validation service needed
- Follow T3 Stack conventions [Source: _bmad-output/planning-artifacts/architecture.md#Structure Patterns]

**Validation Library:**
- JavaScript/TypeScript: **Ajv** (already installed via Story 1.1)
- Ajv version: ^8.17.1 (check package.json for exact version)
- JSON Schema support: Draft 2020-12 (schema uses this version)
- Format validation: **ajv-formats** plugin (already installed)
- YAML parsing: **js-yaml** or **yaml** package (need to install)

**Schema File:**
- Schema location: `/types/pipeline-schema.json` (created in Story 1.1)
- Schema URI: `https://canectors.io/schemas/pipeline/v1.0.0/pipeline-schema.json`
- Must load schema at runtime for validation
- Schema is JSON Schema Draft 2020-12 with conditional validation (`if/then/else`)

### Technical Implementation Details

**Validation Strategy:**

The validator uses JSON Schema validation with Ajv, which validates:
- **Structure validation**: Root structure, connector object, module arrays
- **Type validation**: Field types (string, number, array, object, boolean)
- **Required fields**: Basic required fields (schemaVersion, connector.name, etc.)
- **Type-specific required fields**: Conditional validation for module types:
  - `httpPolling` input: requires `endpoint` and `schedule`
  - `webhook` input: requires `path`
  - `httpRequest` output: requires `endpoint` and `method`
  - `mapping` filter: requires `mappings` array
  - `condition` filter: requires `expression`
- **Pattern validation**: CRON expressions, connector names, semantic versions
- **Enum validation**: HTTP methods, error handling actions
- **Format validation**: URIs, email addresses (via ajv-formats)

**What Runtime Validates (Not This Story):**
- Business logic constraints
- Cross-field dependencies (e.g., authentication matches module type)
- Execution-time validations (endpoint reachability, credential validity)

**Ajv Configuration:**
```typescript
import Ajv from 'ajv';
import addFormats from 'ajv-formats';

const ajv = new Ajv({
  allErrors: true,        // Report all errors, not just first
  strict: false,          // Allow unknown keywords (for extensibility)
  validateFormats: true,  // Validate format strings (dates, URIs, etc.)
  verbose: true,          // Include schema and data paths in errors
});

addFormats(ajv); // Add format validators (date, uri, email, etc.)
```

**YAML Support:**
- Install: `npm install js-yaml @types/js-yaml` or `npm install yaml`
- Parse YAML to JSON before validation
- Convert JSON errors back to YAML context (if possible)
- Support both `.yaml` and `.yml` extensions
- Handle YAML-specific syntax errors (indentation, quotes, etc.)

**Error Reporting Format:**
```typescript
interface ValidationResult {
  valid: boolean;
  errors?: ValidationError[];
}

interface ValidationError {
  path: string;              // JSON path (e.g., "/connector/input/endpoint")
  message: string;           // User-friendly error message
  type: 'syntax' | 'missing' | 'type' | 'pattern' | 'format' | 'enum';
  value?: unknown;           // Invalid value (if applicable)
  expected?: string;         // Expected value or format
  suggestion?: string;       // Suggestion for fixing (optional)
}
```

**Performance Requirements:**
- Validation must complete in <1 second for typical configurations
- Typical configuration: ~100-500 lines JSON/YAML, 1-5 modules
- Optimize schema loading (cache compiled schema)
- Optimize file I/O (stream reading for large files if needed)
- Profile validation performance with test cases

### Project Structure Notes

**File Organization:**
- Validator utility: `/utils/pipeline-validator.ts` (recommended)
- Validator service: `/server/services/pipeline-validator/` (if service needed)
- Tests: `/utils/__tests__/pipeline-validator.test.ts` (co-located)
- Examples: Use existing examples from Story 1.1: `/types/examples/*.json`

**Integration Points:**
- Validator will be used by:
  - Story 2.2 (CLI Configuration Parser) - validation before execution
  - Story 1.3 (YAML Alternative Format) - **Story 1.3 provides YAML parsing utilities that this validator will use**
  - Frontend (Epic 3) - validation in UI before saving
  - CI/CD pipelines - validation in automated workflows

**Story Dependency:**
- **Story 1.3 must be completed BEFORE extending this validator for YAML support**
- Story 1.3 will provide YAML parsing and conversion utilities
- This validator will import and use those utilities for YAML validation
- Initial implementation: JSON validation only
- Extended implementation (after Story 1.3): Add YAML support using Story 1.3 utilities

**CLI Integration (Future - Story 2.2):**
- CLI will call validator before parsing and executing
- CLI command: `canectors validate <config-file>`
- This story provides the validation function that CLI will use

**TypeScript Integration:**
- Validator should export TypeScript types for validation results
- Types should match the structure used by consuming code
- Consider sharing types between validator and schema (if generated types exist)

### Testing Requirements

**Validation Testing:**
- **Valid configurations**: Test all MVP module types pass validation:
  - httpPolling input with CRON schedule
  - webhook input
  - mapping filter
  - condition filter
  - httpRequest output
  - Complete pipelines (Input → Filter → Output)
- **Invalid configurations**: Test various error types:
  - Missing required fields (schemaVersion, connector.name, module.type)
  - Missing type-specific required fields (endpoint for httpPolling, etc.)
  - Invalid field types (string instead of number, array instead of object)
  - Pattern mismatches (invalid CRON expression, invalid connector name)
  - Enum violations (invalid HTTP method, invalid error action)
  - Format violations (invalid URI, invalid semantic version)
- **Edge cases**:
  - Empty files
  - Null values in required fields
  - Empty arrays where arrays are required
  - Unknown module types (should be accepted due to extensibility)
  - Additional properties (should be accepted due to extensibility)

**YAML Testing (After Story 1.3):**
- Valid YAML configurations (all MVP scenarios) - uses YAML parser from Story 1.3
- YAML parsing errors (syntax errors, indentation issues) - handled by Story 1.3 parser
- YAML to JSON conversion correctness - uses conversion from Story 1.3
- Error reporting with YAML context (line numbers if possible) - uses utilities from Story 1.3

**Performance Testing:**
- Measure validation time for typical configs (target: <1 second)
- Measure validation time for large configs (500+ lines)
- Profile schema loading and compilation time
- Optimize if validation exceeds 1 second threshold

**Integration Testing:**
- File-based validation (JSON files)
- File-based validation (YAML files)
- Auto-detection of file format from extension
- Error handling for missing files, permission errors

### References

- **Epic 1 Overview:** [Source: _bmad-output/planning-artifacts/epics.md#Epic 1]
- **Story 1.2 Details:** [Source: _bmad-output/planning-artifacts/epics.md#Story 1.2]
- **Story 1.1 Implementation:** [Source: _bmad-output/implementation-artifacts/1-1-define-pipeline-configuration-schema.md]
- **Pipeline Schema:** [Source: types/pipeline-schema.json]
- **Schema Documentation:** [Source: docs/pipeline-configuration-schema.md]
- **Validation Strategy:** [Source: docs/pipeline-configuration-schema.md#Validation Strategy]
- **Project Context:** [Source: _bmad-output/project-context.md]
- **Architecture Patterns:** [Source: _bmad-output/planning-artifacts/architecture.md#Structure Patterns]
- **Ajv Documentation:** [External: https://ajv.js.org/]

### Critical Implementation Rules

**From Project Context:**
- TypeScript strict mode mandatory - no `any` types [Source: _bmad-output/project-context.md#Language-Specific Rules]
- Use ESM (`import`/`export`) - no CommonJS `require()` unless necessary
- Type imports: `import type { ... }` for type-only imports
- Error handling: Use custom error types, not `throw new Error()` for validation errors
- Follow T3 Stack conventions [Source: _bmad-output/project-context.md#Framework-Specific Rules]

**From Architecture:**
- Format: JSON primary, YAML alternative (Story 1.3)
- Validation: Must validate against schema from Story 1.1
- Error messages: Clear, user-friendly, actionable
- Performance: <1 second for typical configurations
- Extensibility: Must support unknown module types (due to schema extensibility)

**Validation Library:**
- Use Ajv (already installed) - fastest JSON Schema validator
- Use ajv-formats plugin (already installed) for format validation
- Configure Ajv with `allErrors: true` to report all errors
- Configure Ajv with `strict: false` to allow extensibility

### Library and Framework Requirements

**Validation:**
- **Ajv**: ^8.17.1 (already installed via Story 1.1)
- **ajv-formats**: ^3.0.1 (already installed via Story 1.1)
- **YAML parser**: Need to install `js-yaml` or `yaml` package
  - Option A: `js-yaml` (widely used, stable)
  - Option B: `yaml` (newer, faster, better TypeScript support)
  - Recommendation: `yaml` package for better TypeScript support

**File I/O:**
- Node.js `fs/promises` for async file reading
- Path resolution utilities for schema file loading

**Testing:**
- **Vitest**: Already configured (via Story 1.1)
- Use test examples from `/types/examples/*.json`
- Create additional test fixtures for error cases

### Previous Story Intelligence

**Story 1.1 (Define Pipeline Configuration Schema):**
- Created JSON Schema in `/types/pipeline-schema.json`
- Schema uses JSON Schema Draft 2020-12
- Schema includes conditional validation (`if/then/else`) for type-specific required fields
- Schema supports extensibility (unknown module types, additional properties)
- Schema validates: structure, types, required fields, patterns, enums, formats
- Documentation in `/docs/pipeline-configuration-schema.md`
- Test suite in `/types/__tests__/pipeline-schema.test.ts` using Ajv
- Example configurations in `/types/examples/*.json`
- Dependencies: `ajv@^8.17.1`, `ajv-formats@^3.0.1` already installed

**Key Learnings from Story 1.1:**
- Ajv is configured with `allErrors: true` in tests (report all errors)
- Schema validation uses conditional validation for type-specific fields
- Schema allows extensibility (`additionalProperties: true` on modules)
- Test patterns use Ajv compilation and validation
- Example files demonstrate valid configurations for all MVP scenarios

**Patterns to Follow:**
- Use same Ajv configuration as in test file: `allErrors: true`, `strict: false`
- Load schema from `/types/pipeline-schema.json` (absolute or relative path)
- Compile schema once and reuse for multiple validations (performance)
- Format errors similar to Ajv error structure but user-friendly
- Support both file paths and in-memory objects for validation

### Git Intelligence Summary

**Recent Work:**
- Story 1.1 created pipeline schema and test infrastructure
- Ajv and ajv-formats already installed in package.json
- Test infrastructure (Vitest) already configured
- Example configurations available for testing

**Files Created/Modified in Story 1.1:**
- `/types/pipeline-schema.json` - Main schema file (LOAD THIS)
- `/types/__tests__/pipeline-schema.test.ts` - Test file (REFERENCE FOR PATTERNS)
- `/types/examples/*.json` - Example files (USE FOR TESTING)
- `/docs/pipeline-configuration-schema.md` - Documentation (REFERENCE)
- `package.json` - Dependencies (Ajv already installed)

**No YAML dependencies installed yet** - Need to add YAML parser

### Latest Technical Information

**Ajv (JSON Schema Validator):**
- Latest stable: 8.17.1 (installed)
- JSON Schema Draft 2020-12 fully supported
- Conditional validation (`if/then/else`) supported
- Format validation via ajv-formats plugin
- Performance: Fastest JSON Schema validator for Node.js
- Best practice: Compile schema once, validate many times

**YAML Support (After Story 1.3):**
- **Story 1.3 will install and configure YAML parser** (js-yaml or yaml package)
- **Story 1.3 will provide YAML parsing utilities** that this validator will import
- This validator will use Story 1.3 utilities for:
  - YAML → JSON conversion
  - YAML error context preservation
  - YAML-specific error handling
- **Do NOT install YAML parser in this story** - Story 1.3 handles that

**Performance Optimization:**
- Compile schema once at module load (cache compiled validator)
- Lazy-load schema file (only when validator is first used)
- Use schema caching for repeated validations
- Profile validation with test cases to ensure <1 second

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Debug Log References

None - Implementation completed without issues.

### Completion Notes List

- **Task 1 Complete**: Implemented `validateJSON()` function in `/utils/pipeline-validator.ts` using Ajv with JSON Schema Draft 2020-12 support. Schema is compiled once and cached for performance.
- **Task 2 Complete**: Integrated Story 1.3 YAML utilities (`parseYAML`, `detectFormat`) to implement `validateYAMLConfig()`. YAML parsing errors include line/column context when available.
- **Task 3 Complete**: Implemented `validatePipelineConfig()` as main API supporting file paths (JSON, YAML, YML) and content strings. Auto-detects format from file extension or content analysis.
- **Task 4 Complete**: Comprehensive error reporting with types (syntax, missing, type, pattern, format, enum), user-friendly messages, JSON paths, and suggestions for common errors.
- **Task 5 Complete**: Created 51 comprehensive tests covering valid configs, invalid configs, YAML syntax errors, JSON syntax errors, error reporting, edge cases, and performance tests.
- **All 165 tests pass** (51 new + 114 existing from Stories 1.1 and 1.3) with no regressions.
- **Performance verified**: Validation completes in <100ms for typical configurations (well under 1 second requirement).

### Code Review Fixes (2026-01-15)

Adversarial code review identified and fixed the following issues:

**🔴 CRITICAL fixes:**
1. **Added `validateFormats: true`** to Ajv config in `pipeline-validator.ts` and `yaml-parser.ts` - was missing per story specification
2. **Moved `ajv` and `ajv-formats` to dependencies** in `package.json` - were incorrectly in devDependencies but used at runtime

**🟡 MEDIUM fixes:**
3. **Improved performance tests** - now use median of 10 runs instead of single measurement, added schema caching verification test
4. **Added explicit error handling** for schema loading - better error messages if schema file missing or corrupted
5. **Added documentation** explaining `$schema` deletion workaround (Ajv 8.x doesn't support Draft 2020-12 meta-schema)

### File List

**New Files:**
- `utils/pipeline-validator.ts` - Main validation utility module with `validateJSON()`, `validateYAMLConfig()`, and `validatePipelineConfig()` functions
- `utils/__tests__/pipeline-validator.test.ts` - Comprehensive test suite (51 tests)

**Modified Files (Code Review Fixes):**
- `utils/yaml-parser.ts` - Added `validateFormats: true` to Ajv config
- `package.json` - Moved `ajv` and `ajv-formats` from devDependencies to dependencies

**Dependencies (now in dependencies, not devDependencies):**
- `ajv@^8.17.1` - JSON Schema validator
- `ajv-formats@^3.0.1` - Format validation plugin
- `yaml@^2.8.0` - YAML parser (via Story 1.3)

### Change Log

- 2026-01-15: Story 1.2 implementation complete - Created pipeline configuration validator with JSON and YAML support
- 2026-01-15: Code review fixes applied - Added validateFormats, fixed dependencies location, improved tests and documentation
