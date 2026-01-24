# Story 1.3: Support YAML Alternative Format

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,  
I want to use YAML format as an alternative to JSON for pipeline configurations,  
so that I can choose the format that best fits my workflow preferences.

## Acceptance Criteria

**Given** I have a pipeline configuration in YAML format  
**When** I parse and validate the YAML configuration  
**Then** The system correctly parses YAML syntax  
**And** The system validates YAML configurations against the same schema as JSON  
**And** The system can convert between JSON and YAML formats without data loss  
**And** The system maintains format compatibility (NFR37, NFR38)  
**And** The configuration remains readable and editable with standard text editors (FR24, NFR46)

## Tasks / Subtasks

- [x] Task 1: Install and configure YAML parser (AC: parse YAML syntax)
  - [x] Install `yaml` package (recommended for TypeScript native support)
  - [x] Install TypeScript types if needed (yaml has native types)
  - [x] Create YAML parsing utility module: `/utils/yaml-parser.ts`
  - [x] Implement parseYAML function that takes YAML string/file path
  - [x] Handle YAML parsing errors gracefully with clear messages
  - [x] Return structured parsing result (success: boolean, data: object, error?: string)
  - [x] Support both YAML 1.1 and 1.2 standards
- [x] Task 2: Implement YAML to JSON conversion (AC: convert without data loss)
  - [x] Implement yamlToJson function using YAML parser
  - [x] Ensure all data types are preserved (strings, numbers, booleans, arrays, objects)
  - [x] Handle YAML-specific features (multi-line strings, anchors, aliases) correctly
  - [x] Preserve nested structures and arrays
  - [x] Test conversion with all MVP module types (httpPolling, webhook, mapping, condition, httpRequest)
  - [x] Verify no data loss in round-trip conversion (YAML → JSON → YAML)
- [x] Task 3: Implement JSON to YAML conversion (AC: convert without data loss, readable)
  - [x] Implement jsonToYaml function using YAML stringifier
  - [x] Configure YAML output for readability (indentation, line breaks)
  - [x] Preserve all data types and structures
  - [x] Format output for human readability (proper indentation, clear structure)
  - [x] Ensure output is editable with standard text editors
  - [x] Test conversion with all MVP module types
  - [x] Verify no data loss in round-trip conversion (JSON → YAML → JSON)
- [x] Task 4: Integrate YAML validation with schema (AC: validate against same schema)
  - [x] Import validation utilities (will be available from Story 1.2, but can prepare interface)
  - [x] Implement validateYAML function that:
    - Parses YAML to JSON
    - Validates JSON against pipeline schema (from Story 1.1)
    - Returns validation results with YAML context (line numbers if possible)
  - [x] Handle validation errors with YAML file context
  - [x] Preserve YAML-specific error information in error messages
  - [x] Ensure validation uses same schema as JSON validation
- [x] Task 5: Create YAML examples and utilities (AC: format compatibility, readable)
  - [x] Convert existing JSON examples to YAML format
  - [x] Create YAML examples in `/types/examples/*.yaml`:
    - minimal-connector.yaml
    - full-mvp-connector.yaml
    - webhook-connector.yaml
  - [x] Ensure YAML examples are readable and well-formatted
  - [x] Document YAML format usage and best practices
  - [x] Create utility functions for format detection (JSON vs YAML)
  - [x] Create utility for auto-converting between formats when needed

## Dev Notes

### Architecture Requirements

**YAML Parser Location:**
- Primary: `/utils/yaml-parser.ts` (shared utility, follows T3 Stack structure)
- Alternative: `/server/services/yaml-parser/` if service needed
- Follow T3 Stack conventions [Source: _bmad-output/planning-artifacts/architecture.md#Structure Patterns]

**YAML Library:**
- **Recommended: `yaml` package** (native TypeScript support, active maintenance)
- Version: Latest stable (check npm for current version, ~2.8+ as of 2026)
- Alternative: `js-yaml` (requires @types/js-yaml, less TypeScript-friendly)
- **Decision: Use `yaml` package** for better TypeScript experience and active maintenance

**Schema Integration:**
- Use schema from Story 1.1: `/types/pipeline-schema.json`
- YAML configurations must validate against same schema as JSON
- Validation will be done via JSON Schema (YAML → JSON → validate)
- Story 1.2 will use YAML parsing utilities from this story for validation

### Technical Implementation Details

**YAML Parsing Strategy:**

The YAML parser will:
- Parse YAML syntax correctly (YAML 1.1 and 1.2)
- Convert YAML to JSON for validation (JSON Schema validation)
- Preserve all data types and structures
- Handle YAML-specific features appropriately:
  - Multi-line strings (preserve formatting or convert to single-line)
  - Anchors and aliases (resolve to actual values)
  - Comments (may be lost in JSON conversion, but preserve in YAML output)
  - Flow vs block style (prefer block style for readability)

**YAML to JSON Conversion:**
```typescript
import { parse } from 'yaml';

function yamlToJson(yamlString: string): object {
  try {
    const parsed = parse(yamlString, {
      // Options for parsing
      strict: false,        // Allow non-standard YAML features
      prettyErrors: true,   // Better error messages
    });
    return parsed;
  } catch (error) {
    // Handle parsing errors with context
    throw new YAMLParsingError(error.message, error.line, error.column);
  }
}
```

**JSON to YAML Conversion:**
```typescript
import { stringify } from 'yaml';

function jsonToYaml(jsonObject: object): string {
  return stringify(jsonObject, {
    indent: 2,              // 2-space indentation for readability
    lineWidth: 0,           // No line wrapping (preserve structure)
    quotingType: '"',       // Use double quotes for strings
    defaultStringType: 'PLAIN', // Use plain strings when possible
    defaultKeyOrder: 'asc', // Sort keys alphabetically for consistency
  });
}
```

**Format Compatibility:**
- YAML and JSON must represent the same data structure
- Round-trip conversion must preserve data (YAML → JSON → YAML → JSON)
- Schema validation applies to both formats equally
- Format choice is user preference, not functional difference

**Readability Requirements:**
- YAML output must be human-readable
- Proper indentation (2 spaces recommended)
- Clear structure and organization
- Comments support (YAML supports comments, JSON does not)
- Editable with standard text editors (VS Code, vim, etc.)

### Project Structure Notes

**File Organization:**
- YAML parser utility: `/utils/yaml-parser.ts` (recommended)
- YAML examples: `/types/examples/*.yaml` (parallel to JSON examples)
- Tests: `/utils/__tests__/yaml-parser.test.ts` (co-located)
- Documentation: Update `/docs/pipeline-configuration-schema.md` with YAML examples

**Integration Points:**
- YAML parser will be used by:
  - Story 1.2 (Configuration Validator) - YAML validation
  - Story 2.2 (CLI Configuration Parser) - YAML file parsing
  - Frontend (Epic 3) - YAML format support in UI
  - CI/CD pipelines - YAML configuration files

**Example Files:**
- Convert existing JSON examples to YAML:
  - `/types/examples/minimal-connector.yaml`
  - `/types/examples/full-mvp-connector.yaml`
  - `/types/examples/webhook-connector.yaml`
- Ensure YAML examples demonstrate all MVP features
- Format examples for maximum readability

### Testing Requirements

**YAML Parsing Testing:**
- **Valid YAML configurations**: Test all MVP module types:
  - httpPolling input with CRON schedule
  - webhook input
  - mapping filter
  - condition filter
  - httpRequest output
  - Complete pipelines (Input → Filter → Output)
- **YAML syntax errors**: Test various error types:
  - Invalid indentation
  - Missing colons
  - Invalid characters
  - Unclosed strings
  - Invalid structure
- **YAML-specific features**:
  - Multi-line strings
  - Anchors and aliases
  - Comments (preserve or handle appropriately)
  - Flow vs block style

**Conversion Testing:**
- **YAML → JSON**: Test conversion correctness:
  - All data types preserved
  - Nested structures preserved
  - Arrays preserved
  - No data loss
- **JSON → YAML**: Test conversion correctness:
  - All data types preserved
  - Readable output format
  - Proper indentation
  - No data loss
- **Round-trip conversion**: Test both directions:
  - YAML → JSON → YAML (may lose comments, but data preserved)
  - JSON → YAML → JSON (data preserved)

**Format Compatibility Testing:**
- Same configuration in JSON and YAML validate identically
- Both formats produce same runtime behavior
- Format choice is transparent to validation and execution

**Integration Testing:**
- YAML files can be validated using Story 1.2 validator (after Story 1.2 completion)
- YAML files can be parsed by CLI (Story 2.2, future)
- YAML examples match JSON examples in structure and content

### References

- **Epic 1 Overview:** [Source: _bmad-output/planning-artifacts/epics.md#Epic 1]
- **Story 1.3 Details:** [Source: _bmad-output/planning-artifacts/epics.md#Story 1.3]
- **Story 1.1 Implementation:** [Source: _bmad-output/implementation-artifacts/1-1-define-pipeline-configuration-schema.md]
- **Story 1.2 Dependencies:** [Source: _bmad-output/implementation-artifacts/1-2-create-configuration-validator.md]
- **Pipeline Schema:** [Source: types/pipeline-schema.json]
- **Schema Documentation:** [Source: docs/pipeline-configuration-schema.md]
- **JSON Examples:** [Source: types/examples/*.json]
- **Project Context:** [Source: _bmad-output/project-context.md]
- **Architecture Patterns:** [Source: _bmad-output/planning-artifacts/architecture.md#Structure Patterns]
- **YAML Package Documentation:** [External: https://github.com/eemeli/yaml]

### Critical Implementation Rules

**From Project Context:**
- TypeScript strict mode mandatory - no `any` types [Source: _bmad-output/project-context.md#Language-Specific Rules]
- Use ESM (`import`/`export`) - no CommonJS `require()` unless necessary
- Type imports: `import type { ... }` for type-only imports
- Error handling: Use custom error types for YAML parsing errors
- Follow T3 Stack conventions [Source: _bmad-output/project-context.md#Framework-Specific Rules]

**From Architecture:**
- Format: JSON primary, YAML alternative (this story)
- Compatibility: YAML must be equivalent to JSON (same schema validation)
- Readability: YAML output must be human-readable and editable
- Format choice: User preference, not functional difference
- Schema: Same schema validates both formats

**YAML Library:**
- Use `yaml` package (native TypeScript, active maintenance)
- Do NOT use `js-yaml` (requires separate types, less TypeScript-friendly)
- Configure parser for YAML 1.2 support (default)
- Configure stringifier for readability (2-space indent, no line wrapping)

### Library and Framework Requirements

**YAML Parsing:**
- **yaml**: Latest stable version (check npm, ~2.8+ as of 2026)
  - Native TypeScript support (no @types package needed)
  - Active maintenance and security updates
  - YAML 1.1 and 1.2 support
  - Good performance and error messages
- **Installation**: `npm install yaml`

**File I/O:**
- Node.js `fs/promises` for async file reading
- Support both `.yaml` and `.yml` file extensions
- Auto-detect format from file extension or content

**Testing:**
- **Vitest**: Already configured (via Story 1.1)
- Use JSON examples from Story 1.1 as source for YAML examples
- Create YAML-specific test fixtures
- Test round-trip conversions

### Previous Story Intelligence

**Story 1.1 (Define Pipeline Configuration Schema):**
- Created JSON Schema in `/types/pipeline-schema.json`
- Schema uses JSON Schema Draft 2020-12
- Schema includes conditional validation (`if/then/else`) for type-specific required fields
- Schema supports extensibility (unknown module types, additional properties)
- Example configurations in `/types/examples/*.json`:
  - minimal-connector.json
  - full-mvp-connector.json
  - webhook-connector.json
  - future-extensibility-example.json
- Documentation in `/docs/pipeline-configuration-schema.md`
- Dependencies: `ajv@^8.17.1`, `ajv-formats@^3.0.1` already installed

**Key Learnings from Story 1.1:**
- Schema structure is well-defined and stable
- Example files demonstrate all MVP scenarios
- Schema validation is done via JSON Schema (works on JSON objects)
- YAML must convert to JSON for validation (YAML → JSON → validate)

**Story 1.2 (Create Configuration Validator) - Dependency:**
- Story 1.2 will use YAML parsing utilities from this story
- Story 1.2 validator will import `yamlToJson` from this story's utilities
- This story must provide clean, reusable YAML parsing functions
- This story establishes YAML support foundation that Story 1.2 builds upon

**Patterns to Follow:**
- Create utility functions that can be imported by other modules
- Export clean API: `parseYAML`, `yamlToJson`, `jsonToYaml`, `validateYAML`
- Handle errors gracefully with clear messages
- Preserve data integrity in conversions
- Format YAML output for readability

### Git Intelligence Summary

**Recent Work:**
- Story 1.1 created pipeline schema and JSON examples
- Story 1.2 created validator (will use YAML utilities from this story)
- No YAML dependencies installed yet
- Test infrastructure (Vitest) already configured

**Files Created/Modified in Story 1.1:**
- `/types/pipeline-schema.json` - Schema file (YAML must validate against this)
- `/types/examples/*.json` - JSON examples (convert to YAML in this story)
- `/docs/pipeline-configuration-schema.md` - Documentation (update with YAML examples)
- `package.json` - Dependencies (add yaml package in this story)

**Files to Create in This Story:**
- `/utils/yaml-parser.ts` - YAML parsing utilities
- `/utils/__tests__/yaml-parser.test.ts` - Tests
- `/types/examples/*.yaml` - YAML example files

### Latest Technical Information

**YAML Package (Recommended):**
- **Package**: `yaml` (by eemeli)
- **Latest stable**: ~2.8+ (as of 2026, check npm for exact version)
- **TypeScript**: Native TypeScript support (no @types needed)
- **Features**: YAML 1.1 and 1.2 support, good error messages, active maintenance
- **Performance**: Fast parsing and stringification
- **Security**: Active security updates and vulnerability patches
- **Weekly downloads**: ~83 million (as of 2025, indicating widespread adoption)

**YAML vs js-yaml:**
- **yaml package**: Native TypeScript, better for TypeScript projects
- **js-yaml**: Requires @types/js-yaml, less TypeScript-friendly
- **Recommendation**: Use `yaml` package for this project

**YAML Format Considerations:**
- YAML 1.2 is current standard (yaml package supports both 1.1 and 1.2)
- YAML supports comments (JSON does not) - may be lost in conversion
- YAML supports anchors/aliases - resolve to actual values in JSON
- YAML multi-line strings - preserve or convert appropriately
- YAML flow vs block style - prefer block style for readability

**Conversion Strategy:**
- YAML → JSON: Parse YAML, result is JSON-compatible object
- JSON → YAML: Stringify JSON object to YAML format
- Round-trip: May lose comments and formatting, but data preserved
- Validation: Always validate JSON representation (YAML → JSON → validate)

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (Amelia - Developer Agent)

### Debug Log References

- Fixed AJV compatibility issue with JSON Schema 2020-12 `$schema` property
- Fixed invalid regex escape sequences (`\-` and `\,`) in CRON pattern validation
- Applied same fixes to both yaml-parser.ts and pipeline-schema.test.ts for consistency

### Code Review Fixes (2026-01-15)

- **CRITICAL FIX**: Added file path support to `parseYAML()` function (Task 1 completion)
- **CRITICAL FIX**: Added line/column information to `ParseResult` interface and return values
- **CRITICAL FIX**: Added comprehensive tests for `convertFormat()` function (15+ new tests)
- **HIGH FIX**: Improved error handling in `convertFormat()` with proper SyntaxError for invalid JSON
- **HIGH FIX**: Enhanced validation error path handling (empty paths default to '/')
- **MEDIUM FIX**: Added `advanced-erp-sync.yaml` to File List documentation
- **Total tests**: Increased from 65 to 80+ tests

### Completion Notes List

- **Task 1**: Installed `yaml` package v2.8.0, created `/utils/yaml-parser.ts` with `parseYAML()` function supporting YAML 1.1 and 1.2 standards, file path support, and line/column error information
- **Task 2**: Implemented `yamlToJson()` function preserving all data types (strings, numbers, booleans, arrays, objects, null)
- **Task 3**: Implemented `jsonToYaml()` function producing readable YAML with 2-space indentation and block style
- **Task 4**: Implemented `validateYAML()` function validating YAML against pipeline schema with proper error reporting and improved path handling
- **Task 5**: Created YAML examples (minimal-connector.yaml, full-mvp-connector.yaml, webhook-connector.yaml, advanced-erp-sync.yaml) with comments, updated documentation, implemented `convertFormat()` and `detectFormat()` utilities

### Implementation Decisions

1. **Schema Loading**: Lazy-loaded schema validator for performance; removed `$schema` property and fixed regex patterns at load time
2. **Error Types**: Created `YAMLParsingError` custom error class with line/column information
3. **Format Detection**: Implemented `detectFormat()` based on content analysis and file extension
4. **Round-trip Preservation**: YAML→JSON→YAML preserves data but may lose comments (documented behavior)
5. **File Path Support**: `parseYAML()` accepts both string content and file paths (detected by path separators or .yaml/.yml extension)
6. **Error Information**: `ParseResult` includes line/column information for better debugging
7. **Format Conversion**: `convertFormat()` with proper error handling (SyntaxError for invalid JSON, YAMLParsingError for invalid YAML)

### File List

**New Files:**
- `/utils/yaml-parser.ts` - YAML parsing, conversion, and validation utilities
- `/utils/__tests__/yaml-parser.test.ts` - Comprehensive tests (80+ tests)
- `/types/examples/minimal-connector.yaml` - Minimal YAML example
- `/types/examples/full-mvp-connector.yaml` - Full MVP YAML example
- `/types/examples/webhook-connector.yaml` - Webhook YAML example
- `/types/examples/advanced-erp-sync.yaml` - Advanced ERP sync example (v1.1 features)

**Modified Files:**
- `/package.json` - Added `yaml` dependency and `@types/node` devDependency
- `/docs/pipeline-configuration-schema.md` - Added YAML format documentation
- `/types/__tests__/pipeline-schema.test.ts` - Fixed AJV compatibility with JSON Schema 2020-12

## Change Log

| Date | Change | Author |
|------|--------|--------|
| 2026-01-15 | Story 1.3 implementation complete - YAML support added | Amelia (Dev Agent) |
| 2026-01-15 | Code review fixes: file path support, line/column errors, convertFormat tests, improved error handling | Code Reviewer (AI) |
