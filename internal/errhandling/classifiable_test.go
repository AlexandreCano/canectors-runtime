package errhandling

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// fakeModuleError stands in for a typed module error (HTTPCallError and its
// SOAP/SQL siblings): it carries a protocol status code and is opaque to the
// generic classifier.
type fakeModuleError struct {
	msg        string
	statusCode int
	cause      error
}

func (e *fakeModuleError) Error() string { return e.msg }

func (e *fakeModuleError) Unwrap() error { return e.cause }

func (e *fakeModuleError) Classify() *ClassifiedError {
	if e.statusCode == 0 {
		return nil
	}
	return ClassifyHTTPStatus(e.statusCode, e.msg)
}

// unclassifiedModuleError carries no status code and does not self-classify.
type unclassifiedModuleError struct{ msg string }

func (e *unclassifiedModuleError) Error() string { return e.msg }

func TestClassifyErrorUsesClassifiable(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		wantCategory  ErrorCategory
		wantRetryable bool
	}{
		{"400 is a non-retryable validation error", 400, CategoryValidation, false},
		{"401 is a non-retryable auth error", 401, CategoryAuthentication, false},
		{"404 is a non-retryable not-found error", 404, CategoryNotFound, false},
		{"422 is a non-retryable validation error", 422, CategoryValidation, false},
		{"429 is a retryable rate-limit error", 429, CategoryRateLimit, true},
		{"503 is a retryable server error", 503, CategoryServer, true},
		{"500 is a retryable server error", 500, CategoryServer, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &fakeModuleError{msg: "module failed", statusCode: tt.statusCode}

			cl := ClassifyError(err)
			if cl.Category != tt.wantCategory {
				t.Errorf("ClassifyError category = %q, want %q", cl.Category, tt.wantCategory)
			}
			if cl.Retryable != tt.wantRetryable {
				t.Errorf("ClassifyError retryable = %v, want %v", cl.Retryable, tt.wantRetryable)
			}
			if cl.StatusCode != tt.statusCode {
				t.Errorf("ClassifyError statusCode = %d, want %d", cl.StatusCode, tt.statusCode)
			}

			if got := GetErrorCategory(err); got != tt.wantCategory {
				t.Errorf("GetErrorCategory = %q, want %q", got, tt.wantCategory)
			}
			if got := IsRetryable(err); got != tt.wantRetryable {
				t.Errorf("IsRetryable = %v, want %v", got, tt.wantRetryable)
			}
		})
	}
}

// A module error found through a wrapping chain must still be consulted:
// modules wrap their typed errors with fmt.Errorf on the way up.
func TestClassifyErrorFindsClassifiableThroughWrapping(t *testing.T) {
	inner := &fakeModuleError{msg: "http_call failed", statusCode: 400}
	wrapped := fmt.Errorf("executing filter module 2: %w", inner)

	cl := ClassifyError(wrapped)
	if cl.Category != CategoryValidation {
		t.Errorf("category = %q, want %q", cl.Category, CategoryValidation)
	}
	if cl.Retryable {
		t.Error("retryable = true, want false: a 400 must not be replayed")
	}
	if !IsFatal(wrapped) {
		t.Error("IsFatal = false, want true for a wrapped 400")
	}
}

// Classify returning nil means "I have nothing to add": classification must fall
// back to the generic rules rather than reporting an empty classification.
func TestClassifiableReturningNilFallsBackToGenericRules(t *testing.T) {
	t.Run("timeout cause is still read as a retryable network error", func(t *testing.T) {
		err := &fakeModuleError{
			msg:        "http_call request failed",
			statusCode: 0,
			cause:      context.DeadlineExceeded,
		}

		cl := ClassifyError(err)
		if cl.Category != CategoryNetwork {
			t.Errorf("category = %q, want %q", cl.Category, CategoryNetwork)
		}
		if !cl.Retryable {
			t.Error("retryable = false, want true for a timeout")
		}
		if cl.StatusCode != 0 {
			t.Errorf("statusCode = %d, want 0: a timeout carries no status", cl.StatusCode)
		}
	})

	t.Run("no status and no known cause stays unknown", func(t *testing.T) {
		err := &fakeModuleError{msg: "http_call failed to extract key", statusCode: 0}

		cl := ClassifyError(err)
		if cl.Category != CategoryUnknown {
			t.Errorf("category = %q, want %q", cl.Category, CategoryUnknown)
		}
	})
}

// A module error that does not implement Classifiable must keep the previous
// behavior, so adding the interface is opt-in.
func TestNonClassifiableErrorKeepsGenericBehaviour(t *testing.T) {
	err := &unclassifiedModuleError{msg: "boom"}

	cl := ClassifyError(err)
	if cl.Category != CategoryUnknown {
		t.Errorf("category = %q, want %q", cl.Category, CategoryUnknown)
	}
	if !cl.Retryable {
		t.Error("retryable = false, want true: unknown errors stay retryable by default")
	}
}

func TestClassifyFromChain(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		if cl := classifyFromChain(nil); cl != nil {
			t.Errorf("classifyFromChain(nil) = %+v, want nil", cl)
		}
	})

	t.Run("plain error", func(t *testing.T) {
		if cl := classifyFromChain(errors.New("plain")); cl != nil {
			t.Errorf("classifyFromChain returned %+v for a plain error, want nil", cl)
		}
	})

	t.Run("wrapped classifiable", func(t *testing.T) {
		inner := &fakeModuleError{msg: "inner", statusCode: 500}
		cl := classifyFromChain(fmt.Errorf("outer: %w", inner))
		if cl == nil {
			t.Fatal("classifyFromChain did not find the wrapped value")
		}
		if cl.StatusCode != 500 {
			t.Errorf("statusCode = %d, want 500", cl.StatusCode)
		}
	})

	// errors.As stops at the first match: a module error that declines to
	// classify must not hide a wrapped one that would have.
	t.Run("declining classifiable does not hide a deeper one", func(t *testing.T) {
		deep := &fakeModuleError{msg: "deep", statusCode: 503}
		outer := &fakeModuleError{msg: "outer", statusCode: 0, cause: deep}

		cl := classifyFromChain(outer)
		if cl == nil {
			t.Fatal("classifyFromChain stopped at the declining error")
		}
		if cl.StatusCode != 503 || cl.Category != CategoryServer {
			t.Errorf("got %d/%q, want 503/%q", cl.StatusCode, cl.Category, CategoryServer)
		}
	})
}
