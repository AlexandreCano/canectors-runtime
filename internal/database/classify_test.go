package database

import (
	"errors"
	"testing"

	"github.com/cannectors/runtime/internal/errhandling"
)

// A database failure already knows whether it can be replayed. The generic
// classifier cannot see that — it unwraps to the raw driver error and reports
// "unknown, retryable" — so the verdict has to survive through Classifiable.
func TestDatabaseErrorClassify(t *testing.T) {
	tests := []struct {
		name          string
		err           *DatabaseError
		wantCategory  errhandling.ErrorCategory
		wantRetryable bool
	}{
		{
			name:          "constraint violation is functional",
			err:           NewConstraintError("insert", "duplicate key", errors.New("pq: duplicate key")),
			wantCategory:  errhandling.CategoryValidation,
			wantRetryable: false,
		},
		{
			name:          "connection loss is a retryable network failure",
			err:           NewConnectionError("connection refused", errors.New("dial tcp: connect: connection refused")),
			wantCategory:  errhandling.CategoryNetwork,
			wantRetryable: true,
		},
		{
			name:          "timeout is a retryable network failure",
			err:           NewTimeoutError("select", "operation timed out", errors.New("context deadline exceeded")),
			wantCategory:  errhandling.CategoryNetwork,
			wantRetryable: true,
		},
		{
			name:          "deadlock is a retryable server failure",
			err:           NewQueryError("update", "deadlock detected", "UPDATE t SET x = $1", 1, errors.New("deadlock"), true),
			wantCategory:  errhandling.CategoryServer,
			wantRetryable: true,
		},
		{
			name:          "syntax error is functional",
			err:           NewQueryError("select", "SQL syntax error", "SELCT 1", 0, errors.New("syntax error"), false),
			wantCategory:  errhandling.CategoryValidation,
			wantRetryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Through ClassifyError, not Classify directly: the point is that
			// the runtime reaches the verdict, not that the method exists.
			cl := errhandling.ClassifyError(tt.err)
			if cl.Category != tt.wantCategory {
				t.Errorf("category = %q, want %q", cl.Category, tt.wantCategory)
			}
			if cl.Retryable != tt.wantRetryable {
				t.Errorf("retryable = %v, want %v", cl.Retryable, tt.wantRetryable)
			}
		})
	}
}

// Modules wrap their database errors on the way up; the classification must
// survive that.
func TestDatabaseErrorClassifyThroughWrapping(t *testing.T) {
	err := NewConstraintError("insert", "duplicate key", errors.New("pq: duplicate key"))

	if errhandling.IsRetryable(errors.Join(errors.New("sql_call record 3"), err)) {
		t.Error("IsRetryable = true, want false: a constraint violation never succeeds on replay")
	}
}
