# Story 3.4: Implement Filter Module Execution (Conditions)

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,  
I want the runtime to execute Condition Filter modules,  
So that I can route or filter data based on conditional logic.

## Acceptance Criteria

**Given** I have a connector with Condition Filter module configured  
**When** The runtime executes the Filter module with input data  
**Then** The runtime evaluates if/else conditions based on data values (FR73)  
**And** The runtime routes data to appropriate paths based on conditions  
**And** The runtime filters out data that doesn't match conditions  
**And** The runtime handles complex condition expressions  
**And** The runtime returns filtered/routed data for Output modules (FR41)  
**And** The execution is deterministic (NFR24, NFR25)

## Tasks / Subtasks

- [x] Task 1: Implement core condition evaluation logic (AC: evaluates if/else conditions based on data values)
  - [x] Parse condition expression from filter module config
  - [x] Support `lang` field: "simple" (default), "cel", "jsonata", "jmespath"
  - [x] Implement simple expression evaluator (MVP: basic comparisons, logical operators)
  - [x] Evaluate condition against each input record
  - [x] Support field path access (dot notation: "user.status", "order.total")
  - [x] Support comparison operators: ==, !=, <, >, <=, >=
  - [x] Support logical operators: &&, ||, !
  - [x] Support value types: string, number, boolean, null
  - [x] Add unit tests for basic condition evaluation scenarios
- [x] Task 2: Implement onTrue/onFalse routing behavior (AC: routes data to appropriate paths based on conditions)
  - [x] Support `onTrue`: "continue" (default) or "skip"
  - [x] Support `onFalse`: "continue" or "skip" (default: "skip")
  - [x] Apply routing logic: if condition true → use onTrue, if false → use onFalse
  - [x] Continue: pass record to next module
  - [x] Skip: filter out record (don't pass to next module)
  - [x] Handle edge cases: null values, missing fields, type mismatches
  - [x] Add unit tests for routing behavior scenarios
- [x] Task 2b: Implement nested modules support (AC: routes data to appropriate paths based on conditions)
  - [x] Support `then` field: contains a nested filter module configuration (optional)
  - [x] Support `else` field: contains a nested filter module configuration (optional)
  - [x] When condition evaluates to `true` → execute `then` module if present, otherwise apply `onTrue` behavior
  - [x] When condition evaluates to `false` → execute `else` module if present, otherwise apply `onFalse` behavior
  - [x] Nested modules can be any filter module type (mapping, condition, etc.)
  - [x] Execute nested module with current record data
  - [x] Pass nested module output to next module in pipeline
  - [x] Support recursive nesting (condition with nested condition)
  - [x] Handle nested module errors appropriately
  - [x] Add unit tests for nested module scenarios
- [x] Task 3: Implement complex condition expressions (AC: handles complex condition expressions)
  - [x] Support nested field access (e.g., "user.profile.status")
  - [x] Support array element access (e.g., "items[0].price")
  - [x] Support multiple conditions with logical operators (e.g., "status == 'active' && amount > 100")
  - [x] Support parentheses for grouping (e.g., "(a || b) && c")
  - [x] Support string operations: contains, startsWith, endsWith (if simple lang)
  - [x] Support numeric operations: +, -, *, /, % (if simple lang) - Note: Deferred to future as not MVP-critical
  - [x] Handle type coercion for comparisons (string to number, etc.)
  - [x] Add unit tests for complex expression scenarios
- [x] Task 4: Implement expression language support (AC: handles complex condition expressions)
  - [x] Support "simple" language (default): basic expression evaluator
  - [x] Support "cel" language: Common Expression Language (future: integrate CEL library)
  - [x] Support "jsonata" language: JSONata expressions (future: integrate JSONata library)
  - [x] Support "jmespath" language: JMESPath expressions (future: integrate JMESPath library)
  - [x] MVP: Implement "simple" language fully, stub others for future
  - [x] Parse `lang` field from config, default to "simple"
  - [x] Route to appropriate evaluator based on `lang`
  - [x] Add unit tests for language selection
- [x] Task 5: Implement error handling and validation (AC: handles complex condition expressions)
  - [x] Validate expression syntax before evaluation
  - [x] Detect and report syntax errors with clear messages
  - [x] Handle evaluation errors gracefully (missing fields, type errors)
  - [x] Support error handling modes: "fail" (stop on error), "skip" (skip record), "log" (log and continue)
  - [x] Log condition evaluation errors with context (expression, record, error)
  - [x] Return error result with details for pipeline executor
  - [x] Add unit tests for error handling scenarios
- [x] Task 6: Integrate with pipeline executor (AC: returns filtered/routed data for Output modules)
  - [x] Ensure `Condition.Process()` returns `[]map[string]interface{}` format
  - [x] Handle empty input records (return empty array)
  - [x] Handle empty conditions (return records unchanged or empty based on config)
  - [x] Ensure output format matches expected schema for next Filter or Output modules
  - [x] Test integration with pipeline executor (Story 2.3)
  - [x] Test chaining with other filter modules (e.g., Mapping → Condition → Output)
  - [x] Add integration tests with end-to-end pipeline execution
- [x] Task 7: Ensure deterministic execution (AC: execution is deterministic)
  - [x] Ensure same input + same condition = same output (no randomness)
  - [x] Ensure condition evaluation is deterministic (same expression + same data = same result)
  - [x] Ensure routing logic is deterministic (same condition result = same routing)
  - [x] Ensure error handling is deterministic (same error = same handling)
  - [x] No time-dependent logic in condition evaluation
  - [x] Add tests to verify deterministic behavior
  - [x] Document any non-deterministic behaviors (if any) - None identified

## Dev Notes

### Architecture Requirements

**Filter Module Execution:**
- **Location:** `cannectors-runtime/internal/modules/filter/condition.go`
- **Interface:** Implements `filter.Module` interface with `Process(records []map[string]interface{}) ([]map[string]interface{}, error)`
- **Configuration:** Reads from `connector.Pipeline.Filters[]` array, filter with `type: "condition"`
- **Purpose:** Filters or routes data records based on conditional expressions

**Module Interface Integration:**
- Must implement `filter.Module` interface:
  ```go
  type Module interface {
      Process(records []map[string]interface{}) ([]map[string]interface{}, error)
  }
  ```
- Called by `Executor.executeFilters()` in sequence with other filter modules
- Receives records from Input module or previous Filter module
- Returns filtered/routed records for next Filter module or Output module

**Configuration Structure:**
- Filter module configuration is in `connector.Pipeline.Filters[]` array
- Condition filter configuration includes:
  - `type`: "condition" (required)
  - `lang`: "simple" (default), "cel", "jsonata", "jmespath" (optional, default: "simple")
  - `expression`: Condition expression string (required for type="condition")
  - `onTrue`: "continue" (default) or "skip" (optional) - used when `then` is not present
  - `onFalse`: "continue" or "skip" (default: "skip") (optional) - used when `else` is not present
  - `then`: Nested filter module configuration (optional) - executed when condition is true
  - `else`: Nested filter module configuration (optional) - executed when condition is false
  - `enabled`: Boolean, default true (optional)
  - `onError`: "fail", "skip", "log" - error handling mode (optional, inherits from defaults)
  - `timeoutMs`: Timeout in milliseconds (optional, inherits from defaults)

**Nested Modules Support:**
- **`then` field:** Contains a complete filter module configuration (any type: mapping, condition, etc.)
  - Executed when condition evaluates to `true`
  - Receives the current record as input
  - Output is passed to the next module in the pipeline
  - If `then` is present, `onTrue` behavior is ignored for that branch
- **`else` field:** Contains a complete filter module configuration (any type: mapping, condition, etc.)
  - Executed when condition evaluates to `false`
  - Receives the current record as input
  - Output is passed to the next module in the pipeline
  - If `else` is present, `onFalse` behavior is ignored for that branch
- **Priority:** Nested modules (`then`/`else`) take precedence over simple routing (`onTrue`/`onFalse`)
- **Recursive nesting:** Nested modules can themselves be condition modules with their own `then`/`else`
- **Example configuration:**
  ```json
  {
    "type": "condition",
    "expression": "x == 1",
    "then": {
      "type": "mapping",
      "mappings": [
        { "source": "field1", "target": "field2" }
      ]
    },
    "else": {
      "type": "mapping",
      "mappings": [
        { "source": "field1", "target": "field3" }
      ]
    }
  }
  ```

**Expression Language Support:**
- **"simple" (MVP, default):** Basic expression evaluator implemented in Go
  - Field access: dot notation ("user.status", "order.items[0].price")
  - Comparisons: ==, !=, <, >, <=, >=
  - Logical operators: &&, ||, !
  - Value types: string (quoted), number, boolean (true/false), null
  - String operations: contains(), startsWith(), endsWith() (optional MVP)
  - Numeric operations: +, -, *, /, % (optional MVP)
  - Parentheses for grouping
- **"cel" (Future):** Common Expression Language - integrate CEL library
- **"jsonata" (Future):** JSONata expressions - integrate JSONata library
- **"jmespath" (Future):** JMESPath expressions - integrate JMESPath library

**Simple Expression Syntax (MVP):**
- **Field Access:**
  - `status` - top-level field
  - `user.status` - nested field
  - `items[0].price` - array element access
- **Comparisons:**
  - `status == 'active'` - string equality
  - `amount > 100` - numeric comparison
  - `isActive == true` - boolean comparison
  - `field != null` - null check
- **Logical Operators:**
  - `status == 'active' && amount > 100` - AND
  - `status == 'active' || status == 'pending'` - OR
  - `!isDeleted` - NOT
- **Grouping:**
  - `(a || b) && c` - parentheses for precedence
- **String Operations (optional MVP):**
  - `name.contains('test')` - string contains
  - `email.startsWith('admin')` - string starts with
  - `path.endsWith('.json')` - string ends with

**Routing Behavior:**
- **Simple Routing (when `then`/`else` not present):**
  - **onTrue:**
    - `"continue"` (default): Pass record to next module
    - `"skip"`: Filter out record (don't pass to next module)
  - **onFalse:**
    - `"continue"`: Pass record to next module (even if condition false)
    - `"skip"` (default): Filter out record (don't pass to next module)
- **Nested Module Routing (when `then`/`else` present):**
  - If condition evaluates to `true`:
    - If `then` module is present → execute `then` module with current record, pass output to next module
    - If `then` module is not present → apply `onTrue` behavior (continue/skip)
  - If condition evaluates to `false`:
    - If `else` module is present → execute `else` module with current record, pass output to next module
    - If `else` module is not present → apply `onFalse` behavior (continue/skip)
- **Priority:** Nested modules (`then`/`else`) take precedence over simple routing (`onTrue`/`onFalse`)
- **Error Handling:**
  - If condition evaluation fails → apply error handling mode
  - If nested module execution fails → apply nested module's error handling mode

**Error Handling Strategy:**
- **Evaluation Errors:** Syntax errors, missing fields, type errors, invalid expressions
- **Error Modes:**
  - `"fail"`: Stop processing, return error (default)
  - `"skip"`: Skip record with error, continue processing other records
  - `"log"`: Log error, continue processing (may produce partial results)
- **Error Context:** Include expression, field path, record index, error message in error details
- **Logging:** Log errors with structured context (expression, record index, error) using logger package

**Deterministic Execution:**
- Same input records + same condition expression = same output records
- Condition evaluation must be deterministic (no random behavior)
- Routing logic must be deterministic (same condition result = same routing)
- Error handling must be deterministic (same error = same handling)
- No time-dependent logic in condition evaluation

**Integration with Pipeline Executor:**
- Called by `Executor.executeFilters()` in `internal/runtime/pipeline.go`
- Receives records from Input module or previous Filter module
- Returns filtered/routed records for next Filter module or Output module
- Must handle empty input gracefully (return empty array)
- Must preserve record structure for downstream modules
- **Nested Module Integration:**
  - When executing nested modules (`then`/`else`), use same module factory pattern as pipeline executor
  - Instantiate nested module using `filter.NewModule()` or similar factory function
  - Execute nested module's `Process()` method with current record
  - Handle nested module output: if nested module returns empty array, treat as filtered out
  - Support recursive nesting: nested condition modules can have their own nested modules

### Project Structure Notes

**File Organization:**
```
cannectors-runtime/
├── internal/
│   └── modules/
│       └── filter/
│           ├── filter.go              # Module interface (already defined)
│           ├── mapping.go             # Mapping implementation (Story 3.3)
│           ├── mapping_test.go        # Mapping tests (Story 3.3)
│           ├── condition.go           # Condition implementation (to be created)
│           ├── condition_test.go      # Condition tests (to be created)
│           └── evaluator.go           # Simple expression evaluator (optional, can be in condition.go)
└── internal/
    └── runtime/
        └── pipeline.go               # Executor that calls Filter.Process() (Story 2.3)
```

**Integration Points:**
- Condition will be used by:
  - `Executor.executeFilters()` - pipeline execution (Story 2.3)
  - Future filter modules - chained execution
- Condition depends on:
  - `pkg/connector.Pipeline` type (already defined)
  - `pkg/connector.ModuleConfig` type (already defined)
  - `filter.Module` interface (already defined in `filter.go`)
  - Logger: `internal/logger` package (already available)

**Module Instantiation:**
- Condition struct should be created from `ModuleConfig`
- Constructor: `NewCondition(config *connector.ModuleConfig) (*Condition, error)`
- Configuration validation should happen in constructor
- Expression parsing can happen in constructor or lazily on first use

**Expression Evaluator Design:**
- **Simple Evaluator (MVP):**
  - Parse expression into AST (Abstract Syntax Tree)
  - Evaluate AST against record data
  - Support field access, comparisons, logical operators
  - Handle type coercion and error cases
- **Future Evaluators:**
  - CEL: Use `github.com/google/cel-go` library
  - JSONata: Use `github.com/oliveagle/jsonpath` or similar
  - JMESPath: Use `github.com/jmespath/go-jmespath` library

**Nested Module Execution Design:**
- **Module Instantiation:**
  - Parse nested module configuration from `then`/`else` fields
  - Create appropriate filter module instance (mapping, condition, etc.) based on nested `type`
  - Use same module factory pattern as pipeline executor
- **Execution Flow:**
  - Evaluate condition expression
  - If true and `then` present: instantiate and execute `then` module with current record
  - If false and `else` present: instantiate and execute `else` module with current record
  - Pass nested module output to next module in pipeline
  - If nested module is condition type: support recursive execution
- **Error Handling:**
  - Nested module errors should follow nested module's `onError` configuration
  - If nested module fails and `onError="fail"`: propagate error up
  - If nested module fails and `onError="skip"`: skip record, continue with next record
  - If nested module fails and `onError="log"`: log error, continue with next record

### Previous Story Intelligence

**From Story 3.3 (Mapping Filter):**
- Filter modules implement `filter.Module` interface with `Process()` method
- Configuration structure: `type`, `enabled`, `onError`, `timeoutMs` in `ModuleConfig`
- Error handling modes: "fail", "skip", "log" with structured logging
- Deterministic execution is critical: same input = same output
- Integration pattern: Called by `Executor.executeFilters()` in sequence
- Testing pattern: Unit tests for core logic, integration tests with pipeline executor
- File location: `internal/modules/filter/` directory

**From Story 3.2 (Webhook Input):**
- Module configuration validation in constructor
- Error handling with structured context logging
- Integration with pipeline executor via interface

**From Story 3.1 (HTTP Polling Input):**
- Module interface pattern: `Fetch()` for input, `Process()` for filter
- Configuration structure: `type`, `endpoint`, `auth`, etc. in `ModuleConfig`
- Deterministic execution requirements
- Testing patterns: Unit tests + integration tests

**Key Learnings:**
- Follow established filter module patterns from Story 3.3
- Use same error handling and logging patterns
- Maintain deterministic execution guarantees
- Test integration with pipeline executor
- Follow Go best practices for module design

### Testing Requirements

**Unit Tests:**
- Condition evaluation with simple expressions (==, !=, <, >, <=, >=)
- Condition evaluation with logical operators (&&, ||, !)
- Condition evaluation with nested field access ("user.status")
- Condition evaluation with array element access ("items[0].price")
- Condition evaluation with different value types (string, number, boolean, null)
- Routing behavior: onTrue="continue" (pass record)
- Routing behavior: onTrue="skip" (filter record)
- Routing behavior: onFalse="continue" (pass record)
- Routing behavior: onFalse="skip" (filter record, default)
- Nested module: then with mapping module (execute mapping when condition true)
- Nested module: else with mapping module (execute mapping when condition false)
- Nested module: then with condition module (recursive nesting)
- Nested module: else with condition module (recursive nesting)
- Nested module priority: then/else takes precedence over onTrue/onFalse
- Nested module error handling: errors in nested modules propagate correctly
- Complex expressions with parentheses grouping
- String operations: contains(), startsWith(), endsWith() (if implemented)
- Numeric operations: +, -, *, /, % (if implemented)
- Type coercion in comparisons (string to number, etc.)
- Missing field handling (null, undefined)
- Syntax error detection and reporting
- Evaluation error handling (type errors, invalid operations)
- Error handling modes: "fail", "skip", "log"
- Deterministic execution (same input = same output)
- Empty input records handling
- Empty condition handling

**Integration Tests:**
- Pipeline execution with Condition filter module
- End-to-end test: Input → Condition → Output
- Chained filters: Mapping → Condition → Output
- Condition with nested mapping in then branch
- Condition with nested mapping in else branch
- Condition with nested condition (recursive nesting)
- Integration with Executor from Story 2.3
- Error propagation through pipeline
- Error propagation from nested modules

**Test Data:**
- Create test fixtures in `/internal/modules/filter/testdata/`:
  - `valid-condition-config.json` - Valid condition configuration
  - `condition-with-onTrue-skip.json` - Configuration with onTrue="skip"
  - `condition-with-onFalse-continue.json` - Configuration with onFalse="continue"
  - `condition-with-nested-then.json` - Configuration with nested mapping in then
  - `condition-with-nested-else.json` - Configuration with nested mapping in else
  - `condition-with-nested-both.json` - Configuration with both then and else nested modules
  - `condition-recursive-nesting.json` - Configuration with nested condition module
  - `condition-complex-expression.json` - Complex expression with multiple operators
  - `condition-syntax-error.json` - Configuration with syntax error
  - `condition-missing-field.json` - Configuration that accesses missing field

### References

- **Source:** `cannectors-BMAD/_bmad-output/planning-artifacts/epics.md#Story-3.4` - Story requirements and acceptance criteria
- **Source:** `cannectors-BMAD/_bmad-output/planning-artifacts/architecture.md` - Architecture patterns, module structure, deterministic execution requirements
- **Source:** `cannectors-runtime/internal/modules/filter/mapping.go` - Reference implementation for filter module pattern (Story 3.3)
- **Source:** `cannectors-runtime/internal/modules/filter/filter.go` - Filter module interface definition
- **Source:** `cannectors-runtime/pkg/connector/types.go` - Pipeline and ModuleConfig type definitions
- **Source:** `cannectors-runtime/internal/config/schema/pipeline-schema.json` - Condition filter schema definition (type, lang, expression, onTrue, onFalse)
- **Source:** `cannectors-BMAD/_bmad-output/project-context.md` - Go runtime patterns, testing standards, code organization
- **Source:** `cannectors-BMAD/_bmad-output/implementation-artifacts/3-3-implement-filter-module-execution-mapping.md` - Previous story learnings and patterns

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (via Cursor)

### Debug Log References

N/A - No debug issues encountered during implementation.

### Completion Notes List

- **Task 1:** Implemented core condition evaluation logic with expression parser (tokenizer, AST, evaluator). Supports string/number/boolean/null comparisons (==, !=, <, >, <=, >=), logical operators (&&, ||, !), nested field access (dot notation), and array element access (bracket notation).

- **Task 2:** Implemented onTrue/onFalse routing behavior with defaults (onTrue=continue, onFalse=skip). Handles null values, missing fields, and type mismatches gracefully.

- **Task 2b:** Implemented nested modules support (then/else) allowing condition modules to execute nested mapping or condition modules. Supports recursive nesting. Priority: then/else takes precedence over onTrue/onFalse.

- **Task 3:** Implemented complex expressions with parentheses grouping and string operations (contains, startsWith, endsWith). Added MethodCallExpr AST node for method call syntax.

- **Task 4:** Implemented expression language support with "simple" as default. CEL, JSONata, JMESPath stubbed with ErrUnsupportedLang for future implementation.

- **Task 5:** Implemented comprehensive error handling with syntax validation during construction, structured error context (ConditionError), and error modes (fail/skip/log).

- **Task 6:** Integrated with pipeline executor by updating main.go to support "condition" filter type. Added parseConditionConfig and parseNestedModuleConfig functions.

- **Task 7:** Verified deterministic execution with tests running 100+ iterations confirming consistent results.

- **Post-Review Fixes:** Allowed empty expressions to default to pass-through based on routing config, normalized invalid onError values, expanded nested module parsing to accept module-style config blocks, and added pipeline-level integration tests for condition filters and mapping→condition chains.

### File List

**New Files:**
- `cannectors-runtime/internal/modules/filter/condition.go` - Condition module implementation
- `cannectors-runtime/internal/modules/filter/condition_test.go` - Comprehensive test suite (450+ lines)

**Modified Files:**
- `cannectors-runtime/internal/modules/filter/filter.go` - Removed stub Condition struct
- `cannectors-runtime/cmd/cannectors/main.go` - Added condition filter support in createFilterModules
- `cannectors-runtime/internal/runtime/pipeline_test.go` - Added condition filter pipeline integration tests
- `cannectors-runtime/go.mod` - Added expr-lang dependency for condition evaluation
- `cannectors-runtime/go.sum` - Added expr-lang dependency checksums

## Change Log

| Date | Changes |
|------|---------|
| 2026-01-19 | Story 3.4 completed - Implemented Condition Filter Module with expression parser, routing behavior, nested modules, error handling, and pipeline integration |
| 2026-01-19 | Refactored to use `github.com/expr-lang/expr` library for expression evaluation - simplified code from ~900 lines to ~350 lines, better escape sequence handling, more robust parsing |
| 2026-01-19 | Post-review fixes: empty expression handling, nested module config parsing, onError normalization, and pipeline integration tests for condition filters |
