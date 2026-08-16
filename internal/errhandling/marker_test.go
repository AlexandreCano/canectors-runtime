package errhandling

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"testing"
)

func TestNewMarkerFromClassifiableError(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		wantCategory  ErrorCategory
		wantRetryable bool
	}{
		{"400", 400, CategoryValidation, false},
		{"429", 429, CategoryRateLimit, true},
		{"503", 503, CategoryServer, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &fakeModuleError{msg: "http_call failed", statusCode: tt.statusCode}

			m := NewMarker(MarkerSource{Module: "http_call", RecordIndex: 2, Attempts: 3}, err)

			if m.Module != "http_call" {
				t.Errorf("module = %q, want http_call", m.Module)
			}
			if m.Category != tt.wantCategory {
				t.Errorf("category = %q, want %q", m.Category, tt.wantCategory)
			}
			if m.Retryable != tt.wantRetryable {
				t.Errorf("retryable = %v, want %v", m.Retryable, tt.wantRetryable)
			}
			if m.StatusCode != tt.statusCode {
				t.Errorf("statusCode = %d, want %d", m.StatusCode, tt.statusCode)
			}
			if m.Attempts != 3 {
				t.Errorf("attempts = %d, want 3", m.Attempts)
			}
			if m.RecordIndex != 2 {
				t.Errorf("recordIndex = %d, want 2", m.RecordIndex)
			}
		})
	}
}

// AC3: a connection timeout is a retryable network failure and carries no
// status code.
func TestNewMarkerFromTimeoutHasNoStatusCode(t *testing.T) {
	err := fmt.Errorf("http_call request failed: %w", context.DeadlineExceeded)

	m := NewMarker(MarkerSource{Module: "http_call", RecordIndex: 0}, err)

	if m.Category != CategoryNetwork {
		t.Errorf("category = %q, want %q", m.Category, CategoryNetwork)
	}
	if !m.Retryable {
		t.Error("retryable = false, want true for a timeout")
	}
	if m.StatusCode != 0 {
		t.Errorf("statusCode = %d, want 0", m.StatusCode)
	}
	if _, present := m.Map()[MarkerKeyStatusCode]; present {
		t.Error("statusCode key present in map, want omitted when there is no status")
	}
}

func TestNewMarkerNilError(t *testing.T) {
	m := NewMarker(MarkerSource{Module: "http_call", RecordIndex: 1}, nil)

	if m.Category != CategoryUnknown {
		t.Errorf("category = %q, want %q", m.Category, CategoryUnknown)
	}
	if m.Message != "" {
		t.Errorf("message = %q, want empty", m.Message)
	}
}

// AC6: no secret reaches the marker's message.
func TestNewMarkerRedactsSecretsFromMessage(t *testing.T) {
	const secret = "s3cr3t-api-key"
	err := &fakeModuleError{
		msg:        `http_call request failed: Get "https://api.example.com/v1/orders?apikey=` + secret + `"`,
		statusCode: 500,
	}

	m := NewMarker(MarkerSource{Module: "http_call", RecordIndex: 0}, err)

	if strings.Contains(m.Message, secret) {
		t.Errorf("secret survived in marker message: %q", m.Message)
	}
	if !strings.Contains(m.Message, "api.example.com") {
		t.Errorf("redaction removed the diagnostic value too: %q", m.Message)
	}
}

func TestNewMarkerTakesCodeFromModuleError(t *testing.T) {
	err := &fakeCodedError{
		fakeModuleError: fakeModuleError{msg: "boom", statusCode: 400},
		code:            "HTTP_CALL_HTTP_ERROR",
		module:          "http_call",
		recordIndex:     7,
	}

	m := NewMarker(MarkerSource{RecordIndex: -1}, err)

	if m.Code != "HTTP_CALL_HTTP_ERROR" {
		t.Errorf("code = %q, want HTTP_CALL_HTTP_ERROR", m.Code)
	}
	if m.Module != "http_call" {
		t.Errorf("module = %q, want http_call (fallback to ModuleError)", m.Module)
	}
	if m.RecordIndex != 7 {
		t.Errorf("recordIndex = %d, want 7 (fallback to ModuleError)", m.RecordIndex)
	}
}

func TestMarkerMapOmitsInapplicableFields(t *testing.T) {
	m := Marker{
		Module:      "http_call",
		Category:    CategoryNetwork,
		Retryable:   true,
		Message:     "timeout",
		RecordIndex: -1,
	}

	got := m.Map()

	for _, key := range []string{MarkerKeyCode, MarkerKeyStatusCode, MarkerKeyAttempts, MarkerKeyRecordIndex} {
		if _, present := got[key]; present {
			t.Errorf("key %q present, want omitted", key)
		}
	}
	for _, key := range []string{MarkerKeyModule, MarkerKeyCategory, MarkerKeyRetryable, MarkerKeyMessage} {
		if _, present := got[key]; !present {
			t.Errorf("key %q missing, want always present", key)
		}
	}
}

// AC5: two failures on two different filters leave two markers, in order.
func TestAnnotateRecordAccumulatesInOrder(t *testing.T) {
	record := map[string]any{"id": "42"}

	record = AnnotateRecord(record, Marker{Module: "http_call", Category: CategoryValidation, StatusCode: 400, RecordIndex: -1})
	record = AnnotateRecord(record, Marker{Module: "sql_call", Category: CategoryServer, StatusCode: 503, RecordIndex: -1})

	markers := RecordMarkers(record)
	if len(markers) != 2 {
		t.Fatalf("got %d markers, want 2", len(markers))
	}
	if markers[0][MarkerKeyModule] != "http_call" {
		t.Errorf("first marker module = %v, want http_call", markers[0][MarkerKeyModule])
	}
	if markers[1][MarkerKeyModule] != "sql_call" {
		t.Errorf("second marker module = %v, want sql_call", markers[1][MarkerKeyModule])
	}
	if record["id"] != "42" {
		t.Error("annotation clobbered the record's own fields")
	}
}

func TestAnnotateRecordOnNilRecord(t *testing.T) {
	got := AnnotateRecord(nil, Marker{Module: "http_call", RecordIndex: -1})

	if !HasMarkers(got) {
		t.Error("marker lost when annotating a nil record")
	}
}

// The reserved key is protected upstream by set/mapping; if a foreign value
// reaches it anyway, the real failure must still be recorded.
func TestAnnotateRecordReplacesForeignReservedValue(t *testing.T) {
	record := map[string]any{RecordErrorsField: "not a marker list"}

	record = AnnotateRecord(record, Marker{Module: "http_call", RecordIndex: -1})

	markers := RecordMarkers(record)
	if len(markers) != 1 {
		t.Fatalf("got %d markers, want 1", len(markers))
	}
	if markers[0][MarkerKeyModule] != "http_call" {
		t.Errorf("module = %v, want http_call", markers[0][MarkerKeyModule])
	}
}

func TestRecordMarkersAndHasMarkers(t *testing.T) {
	t.Run("record without markers", func(t *testing.T) {
		record := map[string]any{"id": "1"}
		if HasMarkers(record) {
			t.Error("HasMarkers = true on a clean record")
		}
		if RecordMarkers(record) != nil {
			t.Error("RecordMarkers returned a non-nil slice on a clean record")
		}
	})

	t.Run("empty list", func(t *testing.T) {
		record := map[string]any{RecordErrorsField: []any{}}
		if HasMarkers(record) {
			t.Error("HasMarkers = true on an empty marker list")
		}
	})

	t.Run("list of non-maps is ignored", func(t *testing.T) {
		record := map[string]any{RecordErrorsField: []any{"nope", 42}}
		if HasMarkers(record) {
			t.Error("HasMarkers = true on a list holding no marker maps")
		}
	})
}

// fakeCodedError adds ModuleError to fakeModuleError so the marker can be tested
// against both interfaces at once.
type fakeCodedError struct {
	fakeModuleError
	code        string
	module      string
	recordIndex int
}

func (e *fakeCodedError) ErrorCode() string            { return e.code }
func (e *fakeCodedError) ErrorModule() string          { return e.module }
func (e *fakeCodedError) ErrorRecordIndex() int        { return e.recordIndex }
func (e *fakeCodedError) ErrorDetails() map[string]any { return nil }

// AC9: the execution report must break failures down by category, and count
// markers rather than records so a record that failed twice is not reported once.
func TestMarkerTally(t *testing.T) {
	t.Run("an untroubled run reports nothing", func(t *testing.T) {
		if got := NewMarkerTally().Counts(); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("counts per category", func(t *testing.T) {
		tally := NewMarkerTally()
		tally.Record(CategoryValidation)
		tally.Record(CategoryValidation)
		tally.Record(CategoryServer)

		got := tally.Counts()

		if got[string(CategoryValidation)] != 2 {
			t.Errorf("validation = %d, want 2", got[string(CategoryValidation)])
		}
		if got[string(CategoryServer)] != 1 {
			t.Errorf("server = %d, want 1", got[string(CategoryServer)])
		}
		if len(got) != 2 {
			t.Errorf("got %d categories, want 2: %v", len(got), got)
		}
	})

	t.Run("an empty category is recorded as unknown", func(t *testing.T) {
		tally := NewMarkerTally()
		tally.Record("")

		if got := tally.Counts()[string(CategoryUnknown)]; got != 1 {
			t.Errorf("unknown = %d, want 1", got)
		}
	})

	t.Run("Counts returns a snapshot", func(t *testing.T) {
		tally := NewMarkerTally()
		tally.Record(CategoryServer)
		snapshot := tally.Counts()
		tally.Record(CategoryServer)

		if snapshot[string(CategoryServer)] != 1 {
			t.Errorf("snapshot mutated to %v", snapshot)
		}
	})

	t.Run("a nil tally is inert", func(t *testing.T) {
		var tally *MarkerTally
		tally.Record(CategoryServer)
		if got := tally.Counts(); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
}

// The tally must survive the routing the marker exists to enable: a record
// dropped downstream because it carried a failure is still a failure that
// happened, and a report that forgets it is the misleading kind of accurate.
func TestHandleRecordErrorCountsMarkersEvenWhenTheRecordIsDiscarded(t *testing.T) {
	tally := NewMarkerTally()
	ctx := ContextWithMarkerTally(context.Background(), tally)

	failure := &fakeModuleError{msg: "boom", statusCode: 503}
	outcome, marked := HandleRecordError(ctx, OnErrorLog, map[string]any{"id": "1"},
		MarkerSource{Module: "http_call", RecordIndex: 0}, failure)

	if outcome != OutcomeKeep {
		t.Fatalf("outcome = %v, want keep", outcome)
	}
	// The pipeline routes the record out here — the counter must not care.
	_ = marked

	if got := tally.Counts()[string(CategoryServer)]; got != 1 {
		t.Errorf("server = %d, want 1", got)
	}
}

func TestHandleRecordErrorCountsNothingOutsideLogMode(t *testing.T) {
	for _, strategy := range []OnErrorStrategy{OnErrorFail, OnErrorSkip} {
		t.Run(string(strategy), func(t *testing.T) {
			tally := NewMarkerTally()
			ctx := ContextWithMarkerTally(context.Background(), tally)

			HandleRecordError(ctx, strategy, map[string]any{"id": "1"},
				MarkerSource{Module: "http_call", RecordIndex: 0},
				&fakeModuleError{msg: "boom", statusCode: 503})

			if got := tally.Counts(); got != nil {
				t.Errorf("got %v, want nil: only log mode leaves a marker", got)
			}
		})
	}
}

// A module used outside a pipeline run gets a context with no tally in it.
func TestHandleRecordErrorWithoutTally(t *testing.T) {
	outcome, marked := HandleRecordError(context.Background(), OnErrorLog,
		map[string]any{"id": "1"}, MarkerSource{Module: "http_call", RecordIndex: 0},
		&fakeModuleError{msg: "boom", statusCode: 503})

	if outcome != OutcomeKeep {
		t.Fatalf("outcome = %v, want keep", outcome)
	}
	if !HasMarkers(marked) {
		t.Error("record should still carry its marker")
	}
}

// A record shallow-copied after a first failure shares the slice header with
// its twin; annotating either must not overwrite the other's markers.
func TestAnnotateRecordDoesNotAliasACopiedRecord(t *testing.T) {
	original := AnnotateRecord(map[string]any{"id": "1"}, Marker{
		Module: "http_call", Category: CategoryValidation, RecordIndex: -1,
	})

	twin := maps.Clone(original)

	original = AnnotateRecord(original, Marker{Module: "sql_call", Category: CategoryServer, RecordIndex: -1})
	twin = AnnotateRecord(twin, Marker{Module: "soap_call", Category: CategoryNetwork, RecordIndex: -1})

	originalSecond := RecordMarkers(original)[1][MarkerKeyModule]
	twinSecond := RecordMarkers(twin)[1][MarkerKeyModule]

	if originalSecond != "sql_call" {
		t.Errorf("original's second marker = %v, want sql_call — the twin overwrote it", originalSecond)
	}
	if twinSecond != "soap_call" {
		t.Errorf("twin's second marker = %v, want soap_call", twinSecond)
	}
}
