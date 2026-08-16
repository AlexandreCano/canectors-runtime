package filter

import (
	"context"
	"testing"

	"github.com/cannectors/runtime/internal/errhandling"
)

// markedRecord builds a record carrying one error marker, as an upstream filter
// in log mode would leave it.
func markedRecord(id string, statusCode int, category errhandling.ErrorCategory, retryable bool) map[string]any {
	return errhandling.AnnotateRecord(
		map[string]any{"id": id},
		errhandling.Marker{
			Module:      "http_call",
			Category:    category,
			Retryable:   retryable,
			StatusCode:  statusCode,
			Message:     "http_call failed",
			RecordIndex: -1,
		},
	)
}

// AC4: the whole point of the marker — a condition can route on it, so a
// functional failure can be parked while a technical one is left to be replayed.
//
// Note on the presence idiom: the reserved key is simply absent from a record
// that never failed, and expr rejects len(nil). `_errors != nil` is therefore
// the guard to write, on its own or before a len() call. Injecting an empty
// list into the evaluation environment would remove the trap, but the record
// map is passed to expr.Run as the environment itself, so it would cost a map
// copy per record per condition filter on every pipeline — a real price for a
// cosmetic gain.
func TestConditionRoutesOnErrorMarker(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		record     map[string]any
		wantMatch  bool
	}{
		{
			name:       "non-retryable failure is selected",
			expression: `_errors[0].retryable == false`,
			record:     markedRecord("1", 400, errhandling.CategoryValidation, false),
			wantMatch:  true,
		},
		{
			name:       "retryable failure is not selected",
			expression: `_errors[0].retryable == false`,
			record:     markedRecord("2", 503, errhandling.CategoryServer, true),
			wantMatch:  false,
		},
		{
			name:       "routing on the status code",
			expression: `_errors[0].statusCode == 400`,
			record:     markedRecord("3", 400, errhandling.CategoryValidation, false),
			wantMatch:  true,
		},
		{
			name:       "routing on the category",
			expression: `_errors[0].category == "validation"`,
			record:     markedRecord("4", 400, errhandling.CategoryValidation, false),
			wantMatch:  true,
		},
		{
			name:       "presence test on a record that failed",
			expression: `_errors != nil`,
			record:     markedRecord("5", 500, errhandling.CategoryServer, true),
			wantMatch:  true,
		},
		{
			name:       "presence test on a clean record",
			expression: `_errors != nil`,
			record:     map[string]any{"id": "6"},
			wantMatch:  false,
		},
		{
			// The nil guard is the documented idiom: the reserved key is absent
			// from a record that never failed, and expr rejects len(nil).
			name:       "guarded count on a clean record",
			expression: `_errors != nil && len(_errors) > 0`,
			record:     map[string]any{"id": "7"},
			wantMatch:  false,
		},
		{
			name:       "guarded count on a failed record",
			expression: `_errors != nil && len(_errors) > 0`,
			record:     markedRecord("8", 400, errhandling.CategoryValidation, false),
			wantMatch:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// then keeps the record, else drops it, so the output count tells us
			// which branch the record took.
			module, err := NewConditionFromConfig(
				ConditionConfig{
					Expression: tt.expression,
					Else:       []*NestedModuleConfig{{Type: "drop"}},
				},
				func(*NestedModuleConfig, int) (Module, error) {
					return NewDrop(), nil
				},
			)
			if err != nil {
				t.Fatalf("failed to create condition module: %v", err)
			}

			result, err := module.Process(context.Background(), []map[string]any{tt.record})
			if err != nil {
				t.Fatalf("Process returned an error: %v", err)
			}

			matched := len(result) == 1
			if matched != tt.wantMatch {
				t.Errorf("expression %q matched = %v, want %v (got %d records)",
					tt.expression, matched, tt.wantMatch, len(result))
			}
		})
	}
}
