package errhandling

import "context"

// RecordOutcome is what a module should do with a record whose processing
// failed, once the configured onError strategy has been applied.
type RecordOutcome int

const (
	// OutcomeStop aborts processing and propagates the error (onError: fail).
	OutcomeStop RecordOutcome = iota

	// OutcomeDrop removes the record from the stream with no trace left in the
	// data (onError: skip).
	OutcomeDrop

	// OutcomeKeep passes the record downstream carrying an error marker
	// (onError: log).
	OutcomeKeep
)

// String makes the outcome readable in logs and test failures.
func (o RecordOutcome) String() string {
	switch o {
	case OutcomeStop:
		return "stop"
	case OutcomeDrop:
		return "drop"
	case OutcomeKeep:
		return "keep"
	default:
		return "unknown"
	}
}

// HandleRecordError applies an onError strategy to a failed record and returns
// what the module should do with it, plus the record to forward when the
// outcome is OutcomeKeep.
//
// It exists so the marker is built in exactly one place. The
// `switch onError { fail / skip / log }` is currently copied into every module,
// and letting each copy assemble its own marker is how the marker's shape would
// drift from module to module — the way resultKey drifted between the call
// filters (Story 25.2).
//
// Logging stays with the caller: the message wording is module-specific, and
// centralizing it here would flatten every module's diagnostics into one
// generic line.
//
// An unrecognized strategy yields OutcomeStop. A module told to apply a
// strategy nobody defined must not decide on its own to keep going with data it
// could not process.
//
// ctx carries the run's MarkerTally when there is one, so the marker is counted
// here — where it is created — rather than by scanning the surviving records at
// the end of the filter chain, which misses every record the pipeline routed
// out on the strength of that very marker.
func HandleRecordError(
	ctx context.Context,
	strategy OnErrorStrategy,
	record map[string]any,
	src MarkerSource,
	err error,
) (RecordOutcome, map[string]any) {
	switch strategy {
	case OnErrorSkip:
		return OutcomeDrop, nil
	case OnErrorLog:
		marker := NewMarker(src, err)
		MarkerTallyFrom(ctx).Record(marker.Category)
		return OutcomeKeep, AnnotateRecord(record, marker)
	case OnErrorFail:
		return OutcomeStop, nil
	default:
		return OutcomeStop, nil
	}
}
