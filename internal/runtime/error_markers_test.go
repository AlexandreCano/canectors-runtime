// Package runtime provides the pipeline execution engine.
package runtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cannectors/runtime/internal/factory"
	"github.com/cannectors/runtime/internal/modules/input"
	"github.com/cannectors/runtime/pkg/connector"
)

// AC9, and the case the record-scanning implementation got wrong: the failures
// a run reports must survive the routing the marker exists to enable.
//
// The pipeline here is the canonical one — mark the failures with onError: log,
// then route the marked records out so they are not shipped downstream. Counting
// the markers left on the surviving records at the end of the filter chain finds
// none, and a run where every single record failed prints
// "✓ Pipeline executed successfully" with nothing else.
func TestExecutor_ErrorCounts_SurviveRoutedOutRecords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Neither record carries "amount", so the mapping below fails on both.
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "10"},
			{"id": "20"},
		})
	}))
	defer server.Close()

	inputModule, err := input.NewHTTPPollingFromConfig(&connector.ModuleConfig{
		Type: "http-polling",
		Raw:  mustJSON(map[string]any{"endpoint": server.URL}),
	})
	if err != nil {
		t.Fatalf("NewHTTPPollingFromConfig() error = %v", err)
	}

	markingMapping := connector.ModuleConfig{
		Type: "mapping",
		Raw: mustJSON(map[string]any{
			"onError": "log",
			"mappings": []map[string]any{
				{"source": "id", "target": "id"},
				{"source": "amount", "target": "amount", "onMissing": "fail"},
			},
		}),
	}
	// Stands in for the `condition` branch a real pipeline uses to park its
	// rejects: whatever the shape, the marked records leave the stream here.
	dropRejects := connector.ModuleConfig{Type: "drop", Raw: mustJSON(map[string]any{})}

	filterModules, err := factory.CreateFilterModules([]connector.ModuleConfig{markingMapping, dropRejects})
	if err != nil {
		t.Fatalf("CreateFilterModules() error = %v", err)
	}

	outputModule := NewMockOutputModule(nil)
	executor := NewExecutorWithModules(inputModule, filterModules, outputModule, false)

	result, err := executor.Execute(&connector.Pipeline{
		ID:      "test-error-counts-routed-out",
		Name:    "Test Pipeline",
		Version: "1.0.0",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("status = %s, want success — log mode must not fail the run", result.Status)
	}

	// Guards the premise: without this the test would pass on an implementation
	// that only counts surviving records, because none were routed out.
	if result.RecordsProcessed != 0 {
		t.Fatalf("RecordsProcessed = %d, want 0 — the marked records must have left the stream",
			result.RecordsProcessed)
	}

	if got := result.ErrorCounts["validation"]; got != 2 {
		t.Errorf("ErrorCounts[validation] = %d, want 2 (counts = %v)", got, result.ErrorCounts)
	}
}

// The counterpart: a run where nothing failed reports no counters at all,
// rather than a map of zeros the CLI would then have to filter out.
func TestExecutor_ErrorCounts_EmptyOnACleanRun(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "10"}})
	}))
	defer server.Close()

	inputModule, err := input.NewHTTPPollingFromConfig(&connector.ModuleConfig{
		Type: "http-polling",
		Raw:  mustJSON(map[string]any{"endpoint": server.URL}),
	})
	if err != nil {
		t.Fatalf("NewHTTPPollingFromConfig() error = %v", err)
	}

	filterModules, err := factory.CreateFilterModules([]connector.ModuleConfig{{
		Type: "mapping",
		Raw: mustJSON(map[string]any{
			"mappings": []map[string]any{{"source": "id", "target": "id"}},
		}),
	}})
	if err != nil {
		t.Fatalf("CreateFilterModules() error = %v", err)
	}

	executor := NewExecutorWithModules(inputModule, filterModules, NewMockOutputModule(nil), false)

	result, err := executor.Execute(&connector.Pipeline{
		ID:      "test-error-counts-clean",
		Name:    "Test Pipeline",
		Version: "1.0.0",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.ErrorCounts != nil {
		t.Errorf("ErrorCounts = %v, want nil on a clean run", result.ErrorCounts)
	}
}
