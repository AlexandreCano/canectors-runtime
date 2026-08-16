package errhandling

import (
	"context"
	"maps"
	"sync"
)

// MarkerTally counts the error markers produced during one pipeline run,
// keyed by category.
//
// It counts at creation time rather than by scanning the records at the end of
// the filter chain. Scanning only sees the records that survived: a pipeline
// that routes its rejects out — with `drop`, or a `condition` branch that does
// not forward them — is exactly the pipeline the marker exists to enable, and
// it would report zero failures. The count must survive the routing it makes
// possible.
type MarkerTally struct {
	mu     sync.Mutex
	counts map[string]int
}

// NewMarkerTally returns an empty tally ready to record markers.
func NewMarkerTally() *MarkerTally {
	return &MarkerTally{counts: make(map[string]int, 4)}
}

// Record adds one marker of the given category to the tally.
//
// The lock is not decorative: filter modules run on the goroutine of whoever
// drives the pipeline, but a webhook input serves several requests
// concurrently, each carrying the same run-scoped tally.
func (t *MarkerTally) Record(category ErrorCategory) {
	if t == nil {
		return
	}
	if category == "" {
		category = CategoryUnknown
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.counts[string(category)]++
}

// Counts returns a snapshot of the per-category counts, or nil when nothing
// failed — an untroubled run reports no counters rather than a map of zeros.
func (t *MarkerTally) Counts() map[string]int {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.counts) == 0 {
		return nil
	}
	out := make(map[string]int, len(t.counts))
	maps.Copy(out, t.counts)
	return out
}

type markerTallyKey struct{}

// ContextWithMarkerTally returns a context carrying the tally, so modules can
// report their markers without every module signature growing a parameter that
// only the runtime cares about.
func ContextWithMarkerTally(ctx context.Context, tally *MarkerTally) context.Context {
	return context.WithValue(ctx, markerTallyKey{}, tally)
}

// MarkerTallyFrom returns the tally carried by ctx, or nil when there is none —
// which is the normal case for a module used outside a pipeline run, and why
// every method on *MarkerTally tolerates a nil receiver.
func MarkerTallyFrom(ctx context.Context) *MarkerTally {
	if ctx == nil {
		return nil
	}
	tally, _ := ctx.Value(markerTallyKey{}).(*MarkerTally)
	return tally
}
