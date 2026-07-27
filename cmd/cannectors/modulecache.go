package main

import (
	"fmt"
	"sync"

	"github.com/cannectors/runtime/internal/factory"
	"github.com/cannectors/runtime/internal/modules/filter"
	"github.com/cannectors/runtime/internal/modules/input"
	"github.com/cannectors/runtime/internal/modules/output"
	"github.com/cannectors/runtime/pkg/connector"
)

// A scheduled pipeline used to rebuild its whole module chain on every tick and
// throw it away, which meant reopening a database pool per tick — about half the
// duration of a short tick, and roughly 7 500 PostgreSQL sessions a day at a
// fifteen-second schedule — while every cache started cold: the http_call and
// sql_call LRUs, the HTTP connection pools, the compiled script.
//
// Modules are therefore built once per pipeline and reused. The cache lives here
// because the adapter is already what knows how to build a chain from a pipeline;
// the scheduler is deliberately kept unaware of module construction.
//
// See docs/PLAN_REUTILISATION_MODULES.md for the measurements and the decisions.

// moduleSet is one pipeline's module chain, owned by the cache rather than by the
// executor that runs it.
type moduleSet struct {
	input   input.Module
	filters []filter.Module
	output  output.Module
}

// Close releases every module, reporting the first failure while still closing
// the rest — a broken input must not leave a database pool open.
func (m *moduleSet) Close() error {
	var firstErr error
	record := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if m.input != nil {
		record(m.input.Close())
	}
	if m.output != nil {
		record(m.output.Close())
	}
	// filter.Module has no Close: only the ones holding a resource implement it
	// (sql_call and its connection pool), so the assertion is how they are found.
	for _, f := range m.filters {
		if closer, ok := f.(interface{ Close() error }); ok {
			record(closer.Close())
		}
	}
	return firstErr
}

// moduleCache holds the module chain of each pipeline for the lifetime of the
// process.
type moduleCache struct {
	mu  sync.Mutex
	set map[string]*moduleSet
}

// get returns the pipeline's modules, building them on first use.
//
// The build happens under the lock. Two executions of the same pipeline never
// overlap (the scheduler serializes them), and a build only happens once per
// pipeline, so holding the lock costs nothing measurable while keeping the
// invariant simple: one chain per pipeline, built exactly once.
func (c *moduleCache) get(pipeline *connector.Pipeline) (*moduleSet, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.set == nil {
		c.set = map[string]*moduleSet{}
	}
	if existing, ok := c.set[pipeline.ID]; ok {
		return existing, nil
	}

	built, err := buildModuleSet(pipeline)
	if err != nil {
		return nil, err
	}
	c.set[pipeline.ID] = built
	return built, nil
}

// evict drops a pipeline's modules and closes them.
//
// Called after a failed execution so the next tick starts from a fresh chain.
// Rebuilding a pipeline that fails every tick is exactly what the runtime did
// before this cache existed, so the worst case of evicting is the old behavior
// rather than a regression. Executions that merely log or skip bad records
// finish successfully and keep their modules.
func (c *moduleCache) evict(pipelineID string) {
	c.mu.Lock()
	set, ok := c.set[pipelineID]
	delete(c.set, pipelineID)
	c.mu.Unlock()

	if ok && set != nil {
		_ = set.Close()
	}
}

// Close releases every cached chain, for shutdown.
func (c *moduleCache) Close() error {
	c.mu.Lock()
	sets := c.set
	c.set = nil
	c.mu.Unlock()

	var firstErr error
	for id, set := range sets {
		if err := set.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("closing modules of pipeline %q: %w", id, err)
		}
	}
	return firstErr
}

// buildModuleSet creates the three module kinds, closing whatever already
// succeeded if a later one fails.
func buildModuleSet(pipeline *connector.Pipeline) (*moduleSet, error) {
	set := &moduleSet{}

	inputModule, err := factory.CreateInputModule(pipeline.Input)
	if err != nil {
		return nil, &moduleBuildError{code: "INPUT_CREATION_FAILED", err: err}
	}
	set.input = inputModule

	filterModules, err := factory.CreateFilterModules(pipeline.Filters)
	if err != nil {
		_ = set.Close()
		return nil, &moduleBuildError{code: "FILTER_CREATION_FAILED", err: err}
	}
	set.filters = filterModules

	outputModule, err := factory.CreateOutputModule(pipeline.Output)
	if err != nil {
		_ = set.Close()
		return nil, &moduleBuildError{code: "OUTPUT_CREATION_FAILED", err: err}
	}
	set.output = outputModule

	return set, nil
}

// moduleBuildError carries the error code the execution result is expected to
// report, so the adapter does not have to guess which stage failed.
type moduleBuildError struct {
	code string
	err  error
}

func (e *moduleBuildError) Error() string { return e.err.Error() }
func (e *moduleBuildError) Unwrap() error { return e.err }
func (e *moduleBuildError) Code() string  { return e.code }
