package errhandling

import "errors"

// ModuleError is the optional interface implemented by typed module errors so
// the runtime can extract a uniform set of fields (code, module, record
// index, details) for ExecutionError without needing to know the concrete
// type. Modules should embed their existing fields freely; the methods below
// are accessors over them.
//
// The methods use an "Error" prefix to avoid colliding with struct fields of
// the same name — Go disallows a struct field and a method to share an
// identifier, and most existing module error types already expose Code,
// RecordIndex, Details as public fields.
type ModuleError interface {
	error
	ErrorCode() string
	ErrorModule() string
	ErrorRecordIndex() int
	ErrorDetails() map[string]any
}

// AsModuleError reports whether err (or any error in its Unwrap chain)
// implements ModuleError, and returns the matching value.
func AsModuleError(err error) (ModuleError, bool) {
	if err == nil {
		return nil, false
	}
	var me ModuleError
	if !errors.As(err, &me) {
		return nil, false
	}
	return me, true
}

// Classifiable is the optional interface implemented by module error types that
// know their own classification — typically because they carry a protocol
// status code, a SQL error category or a SOAP fault code that the generic
// classifier cannot see.
//
// It exists because a typed module error is opaque to ClassifyError: it is not
// a *ClassifiedError, and it is not a net/url/context error either, so it falls
// through to CategoryUnknown/Retryable. That made a SQL constraint violation —
// a failure that will never succeed on replay — report as "unknown, retryable",
// the opposite of the truth.
//
// Implementations return nil when they have nothing to add, so classification
// falls back to the generic rules. That is the case for an error whose
// StatusCode is 0: a connection timeout carries no status, and the network
// classifier reads it correctly on its own.
type Classifiable interface {
	error

	// Classify returns the error's classification, or nil to defer to the
	// generic rules.
	Classify() *ClassifiedError
}

// explicitClassification returns the classification an error states about
// itself, or nil when it states none and the generic rules must decide.
//
// It is the single definition of the precedence between the two ways an error
// can carry a verdict, because ClassifyError and GetErrorCategory both need it
// and two copies would eventually disagree:
//
//  1. A *ClassifiedError already in the chain. It is the verdict the runtime
//     actually acted on — including any retry.retryableStatusCodes override —
//     and a marker must report what happened, not what the defaults would have
//     said.
//  2. Failing that, a Classifiable module error. A typed module error carrying
//     a SQL category or a SOAP fault code is invisible to every generic rule,
//     so without this it would classify as unknown/retryable (Story 26.10 AC0).
func explicitClassification(err error) *ClassifiedError {
	var classified *ClassifiedError
	if errors.As(err, &classified) {
		return classified
	}
	return classifyFromChain(err)
}

// classifyFromChain returns the classification of the first error in err's
// Unwrap chain that both implements Classifiable and has something to say.
//
// It keeps walking past a Classifiable whose Classify returns nil instead of
// stopping there: errors.As stops at the first match, so a module error that
// declines to classify would otherwise hide a wrapped one that would have.
func classifyFromChain(err error) *ClassifiedError {
	for e := err; e != nil; {
		var c Classifiable
		if !errors.As(e, &c) {
			return nil
		}
		if cl := c.Classify(); cl != nil {
			return cl
		}
		e = errors.Unwrap(c)
	}
	return nil
}
