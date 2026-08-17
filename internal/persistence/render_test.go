package persistence

import (
	"testing"
	"time"
)

// AC7: with nothing persisted yet, the timestamp still renders — as the epoch,
// so an incremental filter written naively means "everything so far" on the
// first run instead of producing a malformed comparison.
func TestRenderStateFirstRun(t *testing.T) {
	for name, state := range map[string]*State{
		"nil state":   nil,
		"empty state": {},
	} {
		t.Run(name, func(t *testing.T) {
			vars := RenderState(state, nil)

			if vars[VarLastTimestamp] != EpochTimestamp {
				t.Errorf("lastTimestamp = %v, want %q", vars[VarLastTimestamp], EpochTimestamp)
			}
			if _, present := vars[VarLastID]; present {
				t.Errorf("lastId present (%v); it must stay absent so default() can supply one", vars[VarLastID])
			}
		})
	}
}

func TestRenderStatePersistedValues(t *testing.T) {
	ts := time.Date(2026, 3, 15, 8, 30, 0, 0, time.UTC)
	id := "cursor-42"

	vars := RenderState(&State{LastTimestamp: &ts, LastID: &id}, nil)

	if vars[VarLastTimestamp] != "2026-03-15T08:30:00Z" {
		t.Errorf("lastTimestamp = %v, want RFC3339", vars[VarLastTimestamp])
	}
	if vars[VarLastID] != "cursor-42" {
		t.Errorf("lastId = %v, want cursor-42", vars[VarLastID])
	}
}

// A timestamp with an offset is normalized to RFC3339 as stored, so the same
// instant renders identically whatever the machine's zone.
func TestRenderStateTimestampKeepsItsOffset(t *testing.T) {
	zone := time.FixedZone("CET", 3600)
	ts := time.Date(2026, 3, 15, 9, 30, 0, 0, zone)

	vars := RenderState(&State{LastTimestamp: &ts}, nil)

	if vars[VarLastTimestamp] != "2026-03-15T09:30:00+01:00" {
		t.Errorf("lastTimestamp = %v, want the stored offset preserved", vars[VarLastTimestamp])
	}
}

// A state file written while a cursor was enabled outlives the config that
// wrote it. Rendering what the pipeline has since stopped tracking would inject
// a watermark it does not maintain, so the config gates what gets published.
func TestRenderStateHonorsTheConfig(t *testing.T) {
	ts := time.Date(2026, 3, 15, 8, 30, 0, 0, time.UTC)
	id := "cursor-42"
	state := &State{LastTimestamp: &ts, LastID: &id}

	vars := RenderState(state, &StatePersistenceConfig{
		Timestamp: &TimestampConfig{Enabled: false},
		ID:        &IDConfig{Enabled: true},
	})

	if vars[VarLastTimestamp] != EpochTimestamp {
		t.Errorf("lastTimestamp = %v, want the epoch: timestamp persistence is off", vars[VarLastTimestamp])
	}
	if vars[VarLastID] != id {
		t.Errorf("lastId = %v, want %q: ID persistence is on", vars[VarLastID], id)
	}
}

// Only the timestamp persisted: the ID key stays absent rather than nil, so an
// expression using it can tell "never set" from "set to nothing".
func TestRenderStateTimestampOnly(t *testing.T) {
	ts := time.Date(2026, 3, 15, 8, 30, 0, 0, time.UTC)

	vars := RenderState(&State{LastTimestamp: &ts}, nil)

	if _, present := vars[VarLastID]; present {
		t.Error("lastId present when no ID was ever persisted")
	}
}

// An expression is free to span several lines in a body template, and a typo
// hidden in one of them is exactly the case this validation exists for.
func TestValidateStateReferencesSpansNewlines(t *testing.T) {
	src := "{\n  \"since\": \"{{\n    state.lastTimestmp\n  }}\"\n}"

	if err := ValidateStateReferences("body", src); err == nil {
		t.Error("multi-line expression with a typo was accepted")
	}
}

// A quoted string inside an expression is data, not a reference: `default()`
// values must not be read as state variable names.
func TestValidateStateReferencesIgnoresQuotedLiterals(t *testing.T) {
	for name, src := range map[string]string{
		"single quotes":              `{{ record.kind | default('state.unknown') }}`,
		"double quotes":              `{{ record.kind | default("state.unknown") }}`,
		"alongside a real reference": `{{ state.lastId | default('state.none') }}`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateStateReferences("endpoint", src); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
