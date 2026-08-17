package persistence

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Variable names under which the persisted state reaches templates and
// expressions. They mirror the JSON field names of State, which is the shape
// the state file already has — one vocabulary from disk to template.
const (
	VarLastTimestamp = "lastTimestamp"
	VarLastID        = "lastId"
)

// EpochTimestamp is the value `state.lastTimestamp` takes before anything has
// been persisted.
//
// A zero timestamp rather than an absent one, because the incremental filters
// this feeds are comparisons: `LastModifiedDate gt {{ state.lastTimestamp }}`
// has to render *something* on the very first run, and the epoch is the value
// that makes it mean "everything so far" — which is what a first run wants.
const EpochTimestamp = "1970-01-01T00:00:00Z"

// RenderState builds the `state` variable map exposed to templates and
// parameter expressions.
//
// It exists to give the three inputs one vocabulary. They had three: the
// database input published `lastRunTimestamp` / `lastRunId`, soapPolling
// published `lastTimestamp` / `lastID`, and httpPolling published nothing at
// all — its state could only leave through a query parameter whose name was
// fixed in the config. A pipeline author moving between two inputs had to
// relearn the names, and the names were nowhere near the state file's own.
//
// lastTimestamp is always present, defaulting to the epoch. lastId is present
// only once an ID has been persisted: unlike a timestamp there is no value that
// means "no cursor yet", so the key is absent and `default(…)` can supply one.
//
// cfg gates what a persisted state is allowed to publish: a state file written
// before `timestamp.enabled` was turned off still holds a timestamp, and
// rendering it would inject a watermark the pipeline has since said it does not
// track. A nil cfg means "no gating" — the database input tracks its cursor
// through `incremental`, not through statePersistence.
func RenderState(state *State, cfg *StatePersistenceConfig) map[string]any {
	vars := map[string]any{
		VarLastTimestamp: EpochTimestamp,
	}
	if state == nil {
		return vars
	}
	if state.LastTimestamp != nil && (cfg == nil || cfg.TimestampEnabled()) {
		vars[VarLastTimestamp] = state.LastTimestamp.Format(time.RFC3339)
	}
	if state.LastID != nil && (cfg == nil || cfg.IDEnabled()) {
		vars[VarLastID] = *state.LastID
	}
	return vars
}

// knownStateVars are the only names a `state.` reference may use.
var knownStateVars = []string{VarLastTimestamp, VarLastID}

// templateExpr matches the body of a `{{ … }}` expression. References are only
// looked for inside one: an endpoint path like `/api/state.json` is text, not a
// variable.
//
// (?s) so `.` crosses newlines: a body template is free to spread an expression
// over several lines, and an expression this misses is an expression whose typo
// ships.
var templateExpr = regexp.MustCompile(`(?s)\{\{(.*?)\}\}`)

// stateRef matches a `state.<name>` reference. The leading guard rejects a
// reference reached through another variable (`record.state.x`), which is a
// record field that happens to be called state, not this map.
var stateRef = regexp.MustCompile(`(^|[^.\w])state\.(\w+)`)

// quotedLiteral matches a single- or double-quoted string inside an expression.
// Their contents are data, not code: `{{ x | default('state.unknown') }}` reads
// no state variable, and rejecting it as an unknown one would be wrong.
var quotedLiteral = regexp.MustCompile(`'[^']*'|"[^"]*"`)

// StateReferences returns the state variable names src reads, in order of
// appearance and without duplicates.
func StateReferences(src string) []string {
	if !strings.Contains(src, "state.") {
		return nil
	}
	var names []string
	for _, expr := range templateExpr.FindAllStringSubmatch(src, -1) {
		code := quotedLiteral.ReplaceAllString(expr[1], "''")
		for _, ref := range stateRef.FindAllStringSubmatch(code, -1) {
			if !slices.Contains(names, ref[2]) {
				names = append(names, ref[2])
			}
		}
	}
	return names
}

// ValidateStateReferences rejects a template reading a state variable that does
// not exist.
//
// Rendering is deliberately lenient — `{{ state.lastId | default("0") }}` has to
// survive a missing lastId — so an unknown name renders as nothing instead of
// failing. In an incremental filter that is the worst outcome available: a typo
// in `{{ state.lastTimestmp }}` turns `LastModifiedDate gt <watermark>` into
// `LastModifiedDate gt `, and the pipeline reports the source's 400 rather than
// the typo. Names are fixed and few, so they can be checked once, when the
// module is built.
func ValidateStateReferences(field, src string) error {
	for _, name := range StateReferences(src) {
		if !slices.Contains(knownStateVars, name) {
			return fmt.Errorf("%s references unknown state variable %q, known variables are %s",
				field, "state."+name, strings.Join(knownStateVars, ", "))
		}
	}
	return nil
}
