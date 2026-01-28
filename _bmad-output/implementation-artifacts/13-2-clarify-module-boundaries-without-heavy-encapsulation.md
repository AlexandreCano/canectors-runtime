# Story 13.2: Clarify Module Boundaries Without Heavy Encapsulation

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want clear, minimal, and stable module interfaces with well-documented boundaries,
so that I can implement custom modules confidently without over-engineering or breaking changes.

## Acceptance Criteria

1. **Given** I am implementing a new input module
   **When** I read the module interface documentation
   **Then** The interface clearly defines what an input module must do (fetch data, close resources)
   **And** The interface does not expose implementation details or internal state
   **And** The documentation explains the contract (context usage, error handling, return format)
   **And** The interface is stable (no breaking changes expected in future versions)

2. **Given** I am implementing a new filter module
   **When** I read the module interface documentation
   **Then** The interface clearly defines what a filter module must do (transform records)
   **And** The interface does not expose implementation details or internal state
   **And** The documentation explains the contract (context usage, error handling, record format)
   **And** The interface is stable (no breaking changes expected in future versions)

3. **Given** I am implementing a new output module
   **When** I read the module interface documentation
   **Then** The interface clearly defines what an output module must do (send data, close resources)
   **And** The interface does not expose implementation details or internal state
   **And** The documentation explains the contract (context usage, error handling, return format)
   **And** The interface is stable (no breaking changes expected in future versions)

4. **Given** I am reviewing module boundaries
   **When** I examine the current interfaces
   **Then** Each interface has a single, clear responsibility
   **And** Interfaces do not leak implementation details (no concrete types, no internal methods)
   **And** Optional capabilities (like PreviewableModule) are clearly separated via interface composition
   **And** The boundaries between modules and the runtime are well-defined

5. **Given** I am documenting module responsibilities
   **When** I create or update module documentation
   **Then** Each module type has clear documentation of its responsibilities
   **And** The documentation explains what modules should NOT do (e.g., input modules don't transform data)
   **And** The documentation includes examples of correct usage
   **And** The documentation is accessible in godoc and README

6. **Given** the runtime executes a pipeline
   **When** modules are created and used
   **Then** The runtime only interacts with modules through their public interfaces
   **And** Modules do not need to know about other modules or the runtime internals
   **And** The boundaries are enforced at compile time (interface compliance)
   **And** No regressions in existing functionality

## Tasks / Subtasks

- [x] Task 1: Review and Refine Module Interfaces (AC: #1–#3, #4)
  - [x] Review `input.Module` interface - ensure it's minimal and stable
  - [x] Review `filter.Module` interface - ensure it's minimal and stable
  - [x] Review `output.Module` interface - ensure it's minimal and stable
  - [x] Review `output.PreviewableModule` - ensure it's properly separated via composition
  - [x] Identify any methods or types that leak implementation details
  - [x] Remove or refactor any non-essential interface methods
  - [x] Ensure interfaces follow Go interface best practices (small, focused, stable)

- [x] Task 2: Document Module Contracts and Boundaries (AC: #1–#3, #5)
  - [x] Add comprehensive godoc comments to `input.Module` explaining:
    - What input modules are responsible for (fetching data)
    - What they should NOT do (transformations, sending data)
    - Context usage patterns (cancellation, timeouts)
    - Error handling expectations
    - Return format expectations (`[]map[string]interface{}`)
  - [x] Add comprehensive godoc comments to `filter.Module` explaining:
    - What filter modules are responsible for (transforming records)
    - What they should NOT do (fetching data, sending data)
    - Context usage patterns
    - Error handling expectations
    - Record format expectations
  - [x] Add comprehensive godoc comments to `output.Module` explaining:
    - What output modules are responsible for (sending data)
    - What they should NOT do (fetching data, transformations)
    - Context usage patterns
    - Error handling expectations
    - Return format expectations (count of sent records)
  - [x] Document `output.PreviewableModule` as an optional extension interface
  - [x] Update `docs/MODULE_EXTENSIBILITY.md` with boundary clarifications

- [x] Task 3: Document Module Responsibilities (AC: #5)
  - [x] Create or update `docs/MODULE_BOUNDARIES.md` (or add section to existing docs) explaining:
    - Clear separation of concerns: Input (fetch) → Filter (transform) → Output (send)
    - What each module type is responsible for
    - What each module type should NOT do
    - Examples of correct and incorrect module implementations
    - Common pitfalls and how to avoid them
  - [x] Add examples showing proper module boundaries
  - [x] Document anti-patterns (what NOT to do)

- [x] Task 4: Verify Runtime Respects Boundaries (AC: #6)
  - [x] Review `internal/runtime/pipeline.go` - ensure it only uses public interfaces
  - [x] Review `internal/factory/modules.go` - ensure it only uses public interfaces
  - [x] Verify no runtime code accesses module internals (private fields, unexported methods)
  - [x] Ensure modules don't need to import runtime internals
  - [x] Verify interface compliance checks are in place (`var _ Interface = (*Type)(nil)`)

- [x] Task 5: Update Tests and Examples (AC: #6)
  - [x] Ensure all tests use interfaces, not concrete types
  - [x] Add tests that verify boundary compliance (modules don't access runtime internals)
  - [x] Update example code in documentation to show proper boundary usage
  - [x] Run full test suite to ensure no regressions

- [x] Task 6: Final Validation (AC: #1–#6)
  - [x] Review all interfaces for minimalism and stability
  - [x] Verify documentation is complete and clear
  - [x] Run `golangci-lint run` and fix any issues
  - [x] Ensure all existing pipelines continue to work
  - [x] Verify no breaking changes to public APIs

## Dev Notes

### Context and Rationale

**Why clarify boundaries?**

- **Extensibility**: Clear boundaries make it easier for contributors to add modules without understanding runtime internals
- **Stability**: Minimal, stable interfaces reduce the risk of breaking changes
- **Maintainability**: Well-defined boundaries make the codebase easier to understand and maintain
- **Testing**: Clear boundaries make modules easier to test in isolation

**Current state:**

- Story 13.1 implemented the registry pattern, enabling extensibility
- Module interfaces exist (`input.Module`, `filter.Module`, `output.Module`) but may need refinement
- Documentation exists (`docs/MODULE_EXTENSIBILITY.md`) but may need boundary clarifications
- Interfaces are relatively minimal but may benefit from better documentation

**Target state:**

- **Minimal interfaces**: Each interface has only the essential methods
- **Clear documentation**: Comprehensive godoc explaining contracts, responsibilities, and boundaries
- **Stable contracts**: Interfaces are designed to remain stable across versions
- **Boundary documentation**: Clear explanation of what modules should and should NOT do
- **Enforced boundaries**: Runtime and modules interact only through public interfaces

### Architecture Compliance

- **Go runtime**: This is the `cannectors-runtime` CLI. All work is in Go.
- **Layout**: Keep `internal/modules/`, `internal/registry/`, `pkg/connector/` structure
- **Interfaces**: Refine existing interfaces (`input.Module`, `filter.Module`, `output.Module`) - do not add unnecessary methods
- **Documentation**: Use godoc comments and `docs/` directory for comprehensive documentation
- **Stability**: Ensure any changes maintain backward compatibility

### Technical Requirements

- **Go version**: Use project's current Go version (see `go.mod`)
- **Interface design**: Follow Go best practices - small, focused interfaces
- **Documentation**: Comprehensive godoc comments with examples
- **Testing**: Ensure all tests pass, no regressions
- **Linting**: Run `golangci-lint run` before marking done

### Project Structure Notes

- **Interfaces**: Located in `internal/modules/input/input.go`, `internal/modules/filter/filter.go`, `internal/modules/output/output.go`
- **Documentation**: Update `docs/MODULE_EXTENSIBILITY.md` and create/update `docs/MODULE_BOUNDARIES.md`
- **Runtime**: Review `internal/runtime/executor.go` and `internal/factory/modules.go` for boundary compliance
- **Tests**: Add boundary compliance tests if needed

### File Structure Requirements

- **Modify**: `internal/modules/input/input.go` - enhance godoc comments
- **Modify**: `internal/modules/filter/filter.go` - enhance godoc comments
- **Modify**: `internal/modules/output/output.go` - enhance godoc comments
- **Modify**: `docs/MODULE_EXTENSIBILITY.md` - add boundary clarifications
- **Create/Modify**: `docs/MODULE_BOUNDARIES.md` - comprehensive boundary documentation
- **Review**: `internal/runtime/executor.go` - verify boundary compliance
- **Review**: `internal/factory/modules.go` - verify boundary compliance

### Testing Requirements

- **Unit**: Verify interfaces are minimal and stable
- **Integration**: Ensure existing pipelines continue to work
- **Documentation**: Verify examples in documentation are correct
- **No regressions**: Full test suite green

### References

- [Source: _bmad-output/planning-artifacts/sprint-change-proposal-2026-01-24.md] – Epic 13, story 13.2 (clarify module boundaries without heavy encapsulation)
- [Source: internal/modules/input/input.go] – Current `input.Module` interface
- [Source: internal/modules/filter/filter.go] – Current `filter.Module` interface
- [Source: internal/modules/output/output.go] – Current `output.Module` interface
- [Source: docs/MODULE_EXTENSIBILITY.md] – Current module extensibility documentation
- [Source: internal/registry/registry.go] – Module registry implementation (Story 13.1)
- [Source: _bmad-output/implementation-artifacts/13-1-simplify-module-extensibility-open-source-friendly.md] – Previous story in Epic 13

### Previous Story Intelligence

**Story 13.1 (Simplify Module Extensibility):**
- Registry pattern implemented in `internal/registry/registry.go`
- Built-in modules registered via `internal/registry/builtins.go`
- Factory refactored to use registry instead of switch statements
- Documentation created in `docs/MODULE_EXTENSIBILITY.md`
- All tests pass, no regressions

**Key learnings:**
- Registry pattern enables extensibility without core modifications
- Interfaces are already relatively minimal
- Documentation exists but may need boundary clarifications
- Interface compliance checks (`var _ Interface = (*Type)(nil)`) are in place

**Integration with Story 13.2:**
- Story 13.2 builds on the extensibility foundation from 13.1
- Focus shifts from "how to add modules" to "what are the boundaries"
- Documentation should clarify responsibilities and anti-patterns
- Interfaces may need minor refinements but should remain stable

### Project Context Reference

- **Runtime CLI**: Go, latest stable. Lint with `golangci-lint run`. Use `internal/` for non-public code, `pkg/connector` for public types.
- **Interface design**: Follow Go best practices - prefer small, focused interfaces over large ones
- **Documentation**: Use godoc comments extensively. Keep `docs/` directory for comprehensive guides.
- **Stability**: Public interfaces should remain stable. Use interface composition for optional capabilities (like `PreviewableModule`).

## Dev Agent Record

### Agent Model Used

claude-sonnet-4-5-20250929 (Code Review)

### Debug Log References

### Completion Notes List

**Implementation Summary (Initial):**
- ✅ Reviewed all module interfaces (`input.Module`, `filter.Module`, `output.Module`, `output.PreviewableModule`) - confirmed they are minimal, stable, and follow Go best practices
- ✅ Added comprehensive godoc comments to all three core interfaces explaining responsibilities, boundaries, context usage, error handling, and return formats
- ✅ Created `docs/MODULE_BOUNDARIES.md` with detailed documentation on module boundaries, responsibilities, anti-patterns, and examples
- ✅ Updated `docs/MODULE_EXTENSIBILITY.md` with boundary clarifications and best practices
- ✅ Verified runtime (`internal/runtime/pipeline.go`) and factory (`internal/factory/modules.go`) only use public interfaces
- ✅ Confirmed no runtime code accesses module internals - all interactions are through interface methods
- ✅ Verified interface compliance checks are in place across all module implementations
- ✅ All tests pass (full test suite: `go test ./...`)
- ✅ Linting passes (`golangci-lint run`)
- ✅ No breaking changes to public APIs - interfaces remain stable

**Code Review Fixes Applied:**
- ✅ **AC#1-3 Enhancement**: Added complete, compilable implementation examples in godoc for all three interface types
- ✅ **AC#2 Fix**: Documented `RequestPreview` as HTTP-specific with guidance for non-HTTP modules
- ✅ **AC#3 Fix**: Created 3 example tests (`example_test.go`) demonstrating correct module implementations
- ✅ **AC#4 Fix**: Created boundary compliance test verifying modules don't import runtime internals
- ✅ **AC#5 Enhancement**: Example tests provide testable, runnable code examples for developers
- ✅ **AC#6 Fix**: Added compile-time interface compliance checks in runtime package
- ✅ **Documentation Fix**: Eliminated duplication between MODULE_EXTENSIBILITY.md and MODULE_BOUNDARIES.md
- ✅ **Filter Error Fix**: Corrected obsolete "Epic 3" error message
- ✅ **README Enhancement**: Added "Extending Cannectors" section with module boundaries documentation
- ✅ All tests pass including new examples and boundary compliance test

**Key Achievements:**
- Clear, comprehensive documentation of module boundaries and responsibilities
- Enhanced godoc comments WITH compilable examples provide complete implementation guidance
- Example tests demonstrate correct usage patterns and serve as documentation
- Boundary compliance test prevents future violations of module/runtime separation
- Documentation includes examples of correct usage and anti-patterns to avoid
- Runtime boundary compliance verified at compile-time AND test-time
- All acceptance criteria satisfied with adversarial review feedback addressed

### File List

**Modified Files (Initial Implementation):**
- `internal/modules/input/input.go` - Enhanced godoc comments for `input.Module` interface
- `internal/modules/filter/filter.go` - Enhanced godoc comments for `filter.Module` interface + fixed obsolete error message
- `internal/modules/output/output.go` - Enhanced godoc comments for `output.Module` and `output.PreviewableModule` interfaces + documented HTTP-specific nature of RequestPreview
- `docs/MODULE_EXTENSIBILITY.md` - Added boundary clarifications section and updated best practices
- `docs/MODULE_BOUNDARIES.md` - Created comprehensive boundary documentation (new file)

**Modified Files (Code Review Fixes):**
- `internal/modules/input/input.go` - Added complete implementation example in godoc
- `internal/modules/filter/filter.go` - Added complete implementation example in godoc, fixed "Epic 3" error message
- `internal/modules/output/output.go` - Added complete implementation example in godoc, clarified RequestPreview HTTP-specificity
- `internal/runtime/pipeline.go` - Added interface compliance checks and boundary documentation
- `docs/MODULE_EXTENSIBILITY.md` - Removed duplication, added cross-references to MODULE_BOUNDARIES.md
- `README.md` - Added "Extending Cannectors" section with module boundaries documentation

**New Files (Code Review Fixes):**
- `internal/modules/boundary_test.go` - Boundary compliance test (verifies modules don't import runtime internals)
- `internal/modules/input/example_test.go` - Example test demonstrating correct input module implementation
- `internal/modules/filter/example_test.go` - Example test demonstrating correct filter module implementation
- `internal/modules/output/example_test.go` - Example test demonstrating correct output module implementation

**Reviewed Files (no changes needed):**
- `internal/runtime/pipeline.go` - Verified uses only public interfaces ✅
- `internal/factory/modules.go` - Verified uses only public interfaces ✅
