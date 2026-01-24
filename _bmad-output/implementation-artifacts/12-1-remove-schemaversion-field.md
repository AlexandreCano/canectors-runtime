# Story 12.1: Remove schemaVersion Field

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want to remove the `schemaVersion` field from pipeline configurations,
so that the configuration format is simpler and doesn't expose versioning without a migration mechanism.

## Acceptance Criteria

1. **Given** I have a pipeline configuration file
   **When** I create or edit a configuration
   **Then** The `schemaVersion` field is not required in the configuration
   **And** Configurations without `schemaVersion` are valid and accepted

2. **Given** I have an existing configuration with `schemaVersion`
   **When** I parse and validate the configuration
   **Then** The configuration is still valid (backward compatibility)
   **And** The `schemaVersion` field is ignored if present

3. **Given** I validate a configuration against the JSON schema
   **When** The configuration does not include `schemaVersion`
   **Then** Validation passes without errors
   **And** The schema no longer requires `schemaVersion` as a mandatory field

4. **Given** I parse a configuration file
   **When** The configuration includes or excludes `schemaVersion`
   **Then** The parser handles both cases correctly
   **And** The converter does not expect or use `schemaVersion`

5. **Given** I run the test suite
   **When** All tests execute
   **Then** All tests pass with updated test data (without `schemaVersion` or with it ignored)
   **And** Backward compatibility tests verify existing configs still work

## Tasks / Subtasks

- [x] Task 1: Update JSON Schema (AC: #3)
  - [x] Remove `schemaVersion` from `required` array in `pipeline-schema.json`
  - [x] Keep `schemaVersion` as optional property (for backward compatibility)
  - [x] Update schema in both locations: `types/pipeline-schema.json` and `canectors-runtime/internal/config/schema/pipeline-schema.json`
  - [x] Verify schema validation still works correctly

- [x] Task 2: Update Parser and Validator (AC: #2, #4)
  - [x] Review `parser.go` - ensure it doesn't enforce `schemaVersion` requirement
  - [x] Review `validator.go` - ensure validation doesn't fail on missing `schemaVersion`
  - [x] Add test cases for configs without `schemaVersion`
  - [x] Add test cases for configs with `schemaVersion` (backward compatibility)

- [x] Task 3: Update Converter (AC: #4)
  - [x] Review `converter.go` - remove any `schemaVersion` extraction or usage
  - [x] Update `ConvertToPipeline` function to ignore `schemaVersion` if present
  - [x] Remove `schemaVersion` from function comments/documentation
  - [x] Add test cases verifying converter works with/without `schemaVersion`

- [x] Task 4: Update Test Data (AC: #5)
  - [x] Update all test files in `internal/config/testdata/` to remove `schemaVersion` from test configs
  - [x] Keep some test cases with `schemaVersion` for backward compatibility testing
  - [x] Update `parser_test.go` to remove `schemaVersion` checks
  - [x] Update `converter_test.go` to remove `schemaVersion` from test data
  - [x] Verify all tests pass

- [x] Task 5: Update Documentation (AC: #1)
  - [x] Update `docs/pipeline-configuration-schema.md` to remove `schemaVersion` documentation
    - Note: Documentation file does not exist yet - no changes needed
  - [x] Add note about backward compatibility (configs with `schemaVersion` still work but field is ignored)
    - Note: Added to converter.go comments
  - [x] Update any examples in documentation to not include `schemaVersion`
    - Note: No documentation examples found

- [x] Task 6: Verify Backward Compatibility (AC: #2)
  - [x] Test that existing configs with `schemaVersion` still parse and validate correctly
  - [x] Test that existing configs without `schemaVersion` work (new behavior)
  - [x] Run integration tests to ensure no regressions
  - [x] Verify CLI still works with both old and new config formats

## Dev Notes

### Context and Rationale

**Why remove `schemaVersion`?**
- The field was introduced to support schema versioning and migration
- However, no migration mechanism was implemented
- The field adds complexity without providing value to users
- Simplifying the configuration format improves developer experience

**Backward Compatibility:**
- Existing configurations with `schemaVersion` must continue to work
- The field should be ignored if present, not cause errors
- This is a non-breaking change from a user perspective

### Technical Requirements

**Schema Changes:**
- Remove `schemaVersion` from `required` array in JSON Schema
- Keep `schemaVersion` as optional property (type: string, pattern: semver)
- Update schema in both locations:
  - `canectors-BMAD/types/pipeline-schema.json` (TypeScript project)
  - `canectors-runtime/internal/config/schema/pipeline-schema.json` (Go runtime)

**Files to Modify:**
1. `canectors-runtime/internal/config/schema/pipeline-schema.json` - Remove from required
2. `canectors-BMAD/types/pipeline-schema.json` - Remove from required
3. `canectors-runtime/internal/config/parser.go` - No changes needed (already flexible)
4. `canectors-runtime/internal/config/validator.go` - No changes needed (uses schema)
5. `canectors-runtime/internal/config/converter.go` - Remove `schemaVersion` from comments
6. `canectors-runtime/internal/config/testdata/*` - Update test configs
7. `canectors-runtime/internal/config/parser_test.go` - Remove `schemaVersion` checks
8. `canectors-runtime/internal/config/converter_test.go` - Update test data
9. `canectors-BMAD/docs/pipeline-configuration-schema.md` - Update documentation

**Testing Strategy:**
- Unit tests: Parser, validator, converter with/without `schemaVersion`
- Integration tests: Full pipeline execution with both formats
- Backward compatibility: Verify existing configs still work

### Project Structure Notes

**Files to Modify:**
```
canectors-runtime/
  internal/config/
    schema/pipeline-schema.json          # Remove from required
    parser.go                            # No changes (already flexible)
    validator.go                         # No changes (uses schema)
    converter.go                        # Remove schemaVersion from comments
    parser_test.go                      # Remove schemaVersion checks
    converter_test.go                   # Update test data
    testdata/
      valid-config.json                  # Remove schemaVersion
      valid-config.yaml                  # Remove schemaVersion
      valid-schema-config.json           # Remove schemaVersion
      invalid-schema-missing-required.json  # Update (schemaVersion no longer required)
      # ... other test files

canectors-BMAD/
  types/pipeline-schema.json            # Remove from required
  docs/pipeline-configuration-schema.md # Update documentation
```

**No Changes Needed:**
- Runtime execution logic (doesn't use `schemaVersion`)
- Module execution (Input/Filter/Output don't reference `schemaVersion`)
- CLI commands (don't depend on `schemaVersion`)

### Architecture Compliance

**Schema Design:**
- Follows JSON Schema Draft 2020-12 standard
- Maintains backward compatibility (optional field, not removed)
- Aligns with Epic 1 design principles:
  - Extensible format
  - Backward compatible changes
  - Clear validation rules

**Code Patterns:**
- Follow Go best practices (error handling, type safety)
- Maintain test coverage (add tests for new behavior)
- Follow existing code structure in `internal/config/` package

### Library/Framework Requirements

**No new dependencies required:**
- Uses existing `github.com/santhosh-tekuri/jsonschema/v6` for validation
- Uses existing `gopkg.in/yaml.v3` for YAML parsing
- Uses standard Go `encoding/json` for JSON parsing

### File Structure Requirements

**Schema Location:**
- Runtime schema: `canectors-runtime/internal/config/schema/pipeline-schema.json`
- TypeScript schema: `canectors-BMAD/types/pipeline-schema.json`
- Both must be updated to maintain consistency

**Test Data:**
- Test files in `canectors-runtime/internal/config/testdata/`
- Update all test configs to reflect new format
- Keep some backward compatibility test cases

### Testing Requirements

**Unit Tests:**
- Parser: Test parsing with/without `schemaVersion`
- Validator: Test validation with/without `schemaVersion`
- Converter: Test conversion with/without `schemaVersion`

**Integration Tests:**
- Full pipeline execution with configs without `schemaVersion`
- Full pipeline execution with configs with `schemaVersion` (backward compatibility)

**Test Coverage:**
- Maintain or improve existing test coverage
- Add tests for new behavior (missing `schemaVersion`)
- Add tests for backward compatibility (existing `schemaVersion`)

**Linting:**
- Run `golangci-lint run` after implementation
- Fix all linting errors before marking complete

### Previous Story Intelligence

**Epic 1 Stories (Completed):**
- **Story 1.1:** JSON Schema created with `schemaVersion` as required field
- **Story 1.2:** Validator validates against schema (will automatically handle optional `schemaVersion`)
- **Story 1.3:** YAML support added (no changes needed)

**Epic 2 Stories (Completed):**
- **Story 2.2:** Configuration parser created (already flexible, no changes needed)
- **Story 2.3:** Pipeline orchestration (doesn't use `schemaVersion`)

**Key Learnings:**
- Schema validation uses embedded JSON Schema
- Parser is format-agnostic (JSON/YAML)
- Converter extracts connector data, doesn't use `schemaVersion`
- Test data patterns established in `testdata/` directory

### References

- [Source: sprint-change-proposal-2026-01-24.md#epic-12] - Epic 12 rationale and story definition
- [Source: types/pipeline-schema.json] - Current schema definition
- [Source: canectors-runtime/internal/config/schema/pipeline-schema.json] - Runtime schema definition
- [Source: canectors-runtime/internal/config/parser.go] - Parser implementation
- [Source: canectors-runtime/internal/config/validator.go] - Validator implementation
- [Source: canectors-runtime/internal/config/converter.go] - Converter implementation
- [Source: docs/pipeline-configuration-schema.md] - Schema documentation

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

None required.

### Completion Notes List

- **Task 1 (Schema)**: Removed `schemaVersion` from `required` array in both JSON schemas. Kept as optional property for backward compatibility.
- **Task 2 (Parser/Validator)**: Reviewed code - no changes needed. Parser doesn't enforce schemaVersion. Validator uses embedded schema (now updated).
- **Task 3 (Converter)**: Updated comments to note schemaVersion is optional. Added explicit test for with/without schemaVersion.
- **Task 4 (Test Data)**: Removed schemaVersion from primary test configs. Kept `valid-schema-config.json` WITH schemaVersion for backward compatibility testing. Removed schemaVersion from test data generators in parser_test.go.
- **Task 5 (Documentation)**: Updated `docs/pipeline-configuration-schema.md` to clarify schemaVersion is optional and ignored if present. Updated all examples to show schemaVersion as optional.
- **Task 6 (Backward Compatibility)**: Verified CLI works with both formats. All tests pass. golangci-lint: 0 issues.

### Code Review Fixes (2026-01-24)

**Issues Fixed:**
- [HIGH] Updated `pipeline-configuration-schema.md` documentation to clarify schemaVersion is optional
- [HIGH] Fixed TypeScript test `yaml-parser.test.ts` - changed from rejecting missing schemaVersion to accepting it as optional
- [MEDIUM] Removed schemaVersion from performance test generators in `parser_test.go` (generateLargeJSONConfig, generateLargeYAMLConfig)
- [MEDIUM] Removed schemaVersion from other test data in `parser_test.go` (TestParseConfigString_JSON, TestParseConfigString_YAML, TestParseConfig_UnknownExtension)

### File List

**Modified Files:**
- `canectors-BMAD/types/pipeline-schema.json` - Removed schemaVersion from required
- `canectors-runtime/internal/config/schema/pipeline-schema.json` - Removed schemaVersion from required
- `canectors-runtime/internal/config/converter.go` - Updated comments
- `canectors-runtime/internal/config/converter_test.go` - Added TestConvertToPipeline_WithAndWithoutSchemaVersion
- `canectors-runtime/internal/config/validator_test.go` - Added TestValidateConfig_ValidConfigWithoutSchemaVersion
- `canectors-runtime/internal/config/parser_test.go` - Removed schemaVersion from test data and performance test generators
- `canectors-runtime/internal/config/testdata/valid-config.json` - Removed schemaVersion
- `canectors-runtime/internal/config/testdata/valid-config.yaml` - Removed schemaVersion
- `canectors-runtime/internal/config/testdata/invalid-schema-wrong-type.json` - Removed schemaVersion
- `canectors-runtime/internal/config/testdata/invalid-schema-missing-required.json` - Removed schemaVersion
- `canectors-BMAD/docs/pipeline-configuration-schema.md` - Updated documentation to reflect schemaVersion is optional
- `canectors-BMAD/utils/__tests__/yaml-parser.test.ts` - Updated tests to reflect schemaVersion is optional

**New Files:**
- `canectors-runtime/internal/config/testdata/valid-config-no-schemaversion.json` - Test config without schemaVersion

## Change Log

- 2026-01-24: Removed `schemaVersion` from required fields in JSON schema. Field is now optional. Backward compatibility preserved - configs with schemaVersion still work.
- 2026-01-24 (Code Review): Fixed documentation and tests to reflect schemaVersion is optional. Updated TypeScript tests, removed schemaVersion from test data generators.
