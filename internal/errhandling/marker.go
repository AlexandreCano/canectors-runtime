package errhandling

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cannectors/runtime/internal/urlsan"
)

// RecordErrorsField is the reserved record key under which error markers
// accumulate.
//
// It holds a list, not a single marker: a record can fail in more than one
// filter along the chain — an enrichment call, then a persistence call — and a
// scalar field would overwrite the first failure, which is usually the root
// cause.
const RecordErrorsField = "_errors"

// Marker keys, as they appear inside a record. They are the names pipeline
// authors write in conditions and templates (`_errors[0].statusCode`), so they
// are part of the configuration surface and must not drift.
const (
	MarkerKeyModule      = "module"
	MarkerKeyCategory    = "category"
	MarkerKeyRetryable   = "retryable"
	MarkerKeyCode        = "code"
	MarkerKeyStatusCode  = "statusCode"
	MarkerKeyMessage     = "message"
	MarkerKeyAttempts    = "attempts"
	MarkerKeyRecordIndex = "recordIndex"
)

// Marker is the machine-readable trace of one module failure, attached to the
// record that failed so downstream filters and outputs can route on it.
//
// The classification is not a new taxonomy. Category and Retryable come from
// ClassifiedError, which already encodes the technical/functional distinction —
// Retryable is that distinction, and Category refines it. A second vocabulary
// would leave the runtime with two answers to the same question.
type Marker struct {
	// Module is the failing module's type, e.g. "http_call".
	Module string

	// Category is the classified category (network, validation, server, ...).
	Category ErrorCategory

	// Retryable reports whether the failure is transient. This is the field to
	// branch on when choosing between "replay later" and "park definitively":
	// a 400 is false, a 503 is true.
	Retryable bool

	// Code is the module-specific error code when the error implements
	// ModuleError, empty otherwise.
	Code string

	// StatusCode is the protocol status code, 0 when the failure has none — a
	// connection timeout does not carry one.
	StatusCode int

	// Message is the failure message with embedded URLs stripped of their
	// secrets.
	Message string

	// Attempts is the number of attempts performed before giving up. 0 when the
	// module does not track attempts.
	Attempts int

	// RecordIndex is the record's 0-based position in the batch handed to the
	// module, or -1 when unknown.
	RecordIndex int
}

// MarkerSource carries the module-side context a Marker cannot derive from the
// error alone.
//
// It is a struct rather than positional parameters because Module, RecordIndex
// and Attempts would otherwise be three adjacent scalars, two of them ints — a
// silent swap waiting to happen at every call site.
type MarkerSource struct {
	Module      string
	RecordIndex int
	Attempts    int

	// Overrides reclassifies specific status codes for this module.
	Overrides ClassificationOverrides
}

// ClassificationOverrides lets a module restate what a given protocol status
// code means for it, because the default rules cannot know the API's dialect.
//
// The canonical case is a 404 on an enrichment lookup: the generic rules call it
// not_found and non-retryable, which is right, but an API that answers 404 for
// "no match" makes it an ordinary functional outcome the pipeline should route
// rather than an anomaly. Conversely some gateways answer 409 or 425 for a
// transient lock, which the default rules would freeze as functional.
//
// This governs the marker only. Whether a request is replayed is decided
// earlier, by retry.retryableStatusCodes — the two are deliberately separate:
// a pipeline may want to stop retrying a code while still recording it as
// technical, or the reverse.
type ClassificationOverrides struct {
	// Functional forces the listed status codes to a non-retryable
	// classification, keeping their category when it does not contradict that.
	Functional []int

	// Technical forces the listed status codes to a retryable classification,
	// keeping their category when it does not contradict that.
	Technical []int
}

// IsEmpty reports whether no override is configured, so callers can skip the
// work entirely on the overwhelmingly common path.
func (o ClassificationOverrides) IsEmpty() bool {
	return len(o.Functional) == 0 && len(o.Technical) == 0
}

// apply restates a classification according to the overrides. Functional wins
// over Technical when a status code is listed in both: the safer reading of a
// contradictory configuration is the one that stops the runtime from replaying
// a request forever.
//
// An override changes Retryable — that is what it is for. The category is only
// rewritten when it would contradict the new verdict: reclassifying a 404 as
// functional must not turn `not_found` into a flat `validation`, or the
// counters and any routing by category lose the one detail that told the
// operator what actually happened.
func (o ClassificationOverrides) apply(statusCode int, category ErrorCategory, retryable bool) (ErrorCategory, bool) {
	if statusCode == 0 || o.IsEmpty() {
		return category, retryable
	}
	if slices.Contains(o.Functional, statusCode) {
		// Unknown is replaced too: it holds nothing worth preserving.
		if isTechnicalCategory(category) || category == CategoryUnknown {
			return CategoryValidation, false
		}
		return category, false
	}
	if slices.Contains(o.Technical, statusCode) {
		if !isTechnicalCategory(category) {
			return CategoryServer, true
		}
		return category, true
	}
	return category, retryable
}

// isTechnicalCategory reports whether a category describes a failure of the
// transport or of the far side, as opposed to one of the request itself.
func isTechnicalCategory(category ErrorCategory) bool {
	switch category {
	case CategoryNetwork, CategoryServer, CategoryRateLimit:
		return true
	case CategoryAuthentication, CategoryValidation, CategoryNotFound, CategoryUnknown:
		return false
	default:
		return false
	}
}

// NewMarker builds a Marker describing a module failure.
//
// The message is redacted, not merely copied: module errors interpolate the
// endpoint they were calling, and that endpoint can carry an API key in its
// query string.
func NewMarker(src MarkerSource, err error) Marker {
	m := Marker{
		Module:      src.Module,
		Category:    CategoryUnknown,
		Attempts:    src.Attempts,
		RecordIndex: src.RecordIndex,
	}
	if err == nil {
		return m
	}

	cl := ClassifyError(err)
	m.StatusCode = cl.StatusCode
	m.Category, m.Retryable = src.Overrides.apply(cl.StatusCode, cl.Category, cl.Retryable)

	// err.Error() rather than cl.Message: the classified message is a short
	// label ("bad request") while the error carries what actually happened and
	// which field or endpoint was involved.
	m.Message = urlsan.RedactText(err.Error())

	if me, ok := AsModuleError(err); ok {
		if c := me.ErrorCode(); c != "" {
			m.Code = c
		}
		if m.Module == "" {
			m.Module = me.ErrorModule()
		}
		if m.RecordIndex < 0 {
			if idx := me.ErrorRecordIndex(); idx >= 0 {
				m.RecordIndex = idx
			}
		}
	}

	return m
}

// Map renders the marker as it is stored inside a record.
//
// Fields that do not apply are omitted rather than zero-filled: a timeout has
// no status code, and reporting `statusCode: 0` would invite conditions to
// compare against a value that means "none".
func (m Marker) Map() map[string]any {
	out := map[string]any{
		MarkerKeyModule:    m.Module,
		MarkerKeyCategory:  string(m.Category),
		MarkerKeyRetryable: m.Retryable,
		MarkerKeyMessage:   m.Message,
	}
	if m.Code != "" {
		out[MarkerKeyCode] = m.Code
	}
	if m.StatusCode != 0 {
		out[MarkerKeyStatusCode] = m.StatusCode
	}
	if m.Attempts > 0 {
		out[MarkerKeyAttempts] = m.Attempts
	}
	if m.RecordIndex >= 0 {
		out[MarkerKeyRecordIndex] = m.RecordIndex
	}
	return out
}

// AnnotateRecord appends the marker to the record's reserved error list and
// returns the record.
//
// The record is mutated in place and returned, matching how the rest of the
// runtime treats records (decoded JSON maps passed by reference).
//
// A nil record is materialized rather than skipped: losing the trace of a
// failure because the record was empty would defeat the point of the marker.
//
// If the reserved key already holds something that is not a marker list, that
// value is replaced. The field is reserved and the filters that write record
// fields refuse to target it, so a foreign value there is a bug — and a real
// failure must not go unrecorded because of it.
//
// The list is copied rather than appended to in place: a record that was
// shallow-copied after a first failure — the `loop` filter expanding a parent,
// a module cloning a record — shares the slice header with its twin, and
// appending into the shared backing array would let one record's second marker
// overwrite the other's.
func AnnotateRecord(record map[string]any, m Marker) map[string]any {
	if record == nil {
		record = make(map[string]any, 1)
	}

	existing, _ := record[RecordErrorsField].([]any)
	markers := make([]any, len(existing), len(existing)+1)
	copy(markers, existing)
	record[RecordErrorsField] = append(markers, m.Map())
	return record
}

// RecordMarkers returns the markers attached to a record, in the order they
// were recorded. It returns nil when the record carries none.
//
// Markers are returned as the raw maps stored in the record: that is the shape
// conditions and templates address, and re-typing them here would introduce a
// second representation to keep in sync.
func RecordMarkers(record map[string]any) []map[string]any {
	raw, ok := record[RecordErrorsField].([]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, entry := range raw {
		if m, ok := entry.(map[string]any); ok {
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// HasMarkers reports whether the record carries at least one error marker.
func HasMarkers(record map[string]any) bool {
	return len(RecordMarkers(record)) > 0
}

// IsReservedRecordPath reports whether a record field path writes into the
// reserved error-marker key — the key itself, or anything nested under it.
//
// Only the root segment is inspected: `_errors`, `_errors[0]` and
// `_errors.anything` all target the reserved list, while a field merely named
// after it further down a path (`payload._errors`) is the pipeline author's own
// data and stays theirs.
func IsReservedRecordPath(path string) bool {
	root := path
	if i := strings.IndexAny(root, ".["); i >= 0 {
		root = root[:i]
	}
	return root == RecordErrorsField
}

// ValidateRecordTarget rejects a field path that would write into the reserved
// error-marker key.
//
// The list is the runtime's own trace of what failed. A pipeline that can
// overwrite it can forge a success, which is worse than not having the marker at
// all — the point of the field is that it can be trusted.
//
// Two filters are deliberately left out of this check. `remove` must be able to
// drop `_errors`, because it is the only way to keep the markers — and the
// internal messages they carry — out of the payload sent to an output. `script`
// receives the whole record and could rewrite anything; policing one key there
// would suggest a guarantee the filter cannot make. Both are documented as
// escape hatches rather than silently tolerated.
func ValidateRecordTarget(path string) error {
	if IsReservedRecordPath(path) {
		return fmt.Errorf(
			"field path %q writes into %q, which is reserved for the runtime's error markers",
			path, RecordErrorsField,
		)
	}
	return nil
}
