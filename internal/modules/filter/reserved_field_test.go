package filter

import (
	"strings"
	"testing"

	"github.com/cannectors/runtime/internal/errhandling"
	"github.com/cannectors/runtime/internal/moduleconfig"
)

// AC7: the reserved marker list is the runtime's own record of what failed. A
// pipeline able to overwrite it could forge a success, so set and mapping must
// refuse the write at construction time — not silently at runtime.
func TestSetRejectsReservedTarget(t *testing.T) {
	rejected := []string{
		"_errors",
		"_errors[0]",
		"_errors[0].statusCode",
		"_errors.forged",
	}

	for _, target := range rejected {
		t.Run(target, func(t *testing.T) {
			_, err := NewSetFromConfig(SetConfig{
				Target:   target,
				Value:    "forged",
				HasValue: true,
			})
			if err == nil {
				t.Fatalf("set accepted reserved target %q", target)
			}
			if !strings.Contains(err.Error(), errhandling.RecordErrorsField) {
				t.Errorf("error does not name the reserved field: %v", err)
			}
		})
	}
}

// A field merely named after the reserved key further down a path is the
// author's own data and must stay writable.
func TestSetAllowsNestedFieldNamedLikeReserved(t *testing.T) {
	allowed := []string{
		"payload._errors",
		"_errorsCount",
		"errors",
	}

	for _, target := range allowed {
		t.Run(target, func(t *testing.T) {
			if _, err := NewSetFromConfig(SetConfig{
				Target:   target,
				Value:    "ok",
				HasValue: true,
			}); err != nil {
				t.Errorf("set rejected legitimate target %q: %v", target, err)
			}
		})
	}
}

func TestMappingRejectsReservedTarget(t *testing.T) {
	source := "id"

	_, err := parseMappingConfig(FieldMapping{
		Source: &source,
		Target: "_errors",
	}, 0)
	if err == nil {
		t.Fatal("mapping accepted a reserved target")
	}
	if !strings.Contains(err.Error(), errhandling.RecordErrorsField) {
		t.Errorf("error does not name the reserved field: %v", err)
	}
}

func TestMappingAllowsRegularTarget(t *testing.T) {
	source := "id"

	if _, err := parseMappingConfig(FieldMapping{
		Source: &source,
		Target: "customer.id",
	}, 0); err != nil {
		t.Errorf("mapping rejected a legitimate target: %v", err)
	}
}

// resultKey is the third way a filter writes a record field, and it reaches the
// reserved list just as set and mapping do — a pipeline declaring
// `resultKey: _errors` would silently overwrite the runtime's own trace of what
// failed on the very record that failed.
func TestSQLCallRejectsReservedResultKey(t *testing.T) {
	_, err := NewSQLCallFromConfig(SQLCallConfig{
		SQLRequestBase: moduleconfig.SQLRequestBase{
			ConnectionStringRef: "${SQL_CALL_TEST_DSN}",
			Driver:              "sqlite",
			Query:               "SELECT 1",
		},
		MergeStrategy: "append",
		ResultKey:     errhandling.RecordErrorsField,
	})
	if err == nil {
		t.Fatal("sql_call accepted a reserved resultKey")
	}
	if !strings.Contains(err.Error(), errhandling.RecordErrorsField) {
		t.Errorf("error does not name the reserved field: %v", err)
	}
}

func TestSOAPCallRejectsReservedResultKey(t *testing.T) {
	_, err := NewSOAPCallFromConfig(SOAPCallConfig{
		SOAPRequestBase: moduleconfig.SOAPRequestBase{
			Endpoint:  "https://example.com/service",
			Operation: "Lookup",
			Body:      `<Lookup/>`,
		},
		MergeStrategy: "append",
		ResultKey:     errhandling.RecordErrorsField,
	})
	if err == nil {
		t.Fatal("soap_call accepted a reserved resultKey")
	}
	if !strings.Contains(err.Error(), errhandling.RecordErrorsField) {
		t.Errorf("error does not name the reserved field: %v", err)
	}
}

func TestIsReservedRecordPath(t *testing.T) {
	tests := map[string]bool{
		"_errors":              true,
		"_errors[0]":           true,
		"_errors[0].retryable": true,
		"_errors.nested":       true,
		"payload._errors":      false,
		"_errorsCount":         false,
		"errors":               false,
		"":                     false,
		"customer.id":          false,
		"_metadata.loop.x":     false,
	}

	for path, want := range tests {
		if got := errhandling.IsReservedRecordPath(path); got != want {
			t.Errorf("IsReservedRecordPath(%q) = %v, want %v", path, got, want)
		}
	}
}
