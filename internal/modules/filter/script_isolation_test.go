package filter

import (
	"context"
	"testing"
)

// TestScript_GlobalsDoNotLeakBetweenExecutions locks the runtime's isolation.
//
// Modules are reused across scheduled ticks, so a runtime kept on the module
// would carry its globals from one execution to the next. That is not merely a
// semantic change: a script appending to a global — a running list, a lookup map
// that grows — would become an unbounded memory leak, invisible today because a
// fresh runtime per tick resets it. Isolation costs about a microsecond per
// execution, so it is kept.
//
// Without this test, an optimisation that hoisted the runtime back onto the
// module would pass everything else.
func TestScript_GlobalsDoNotLeakBetweenExecutions(t *testing.T) {
	module, err := NewScriptFromConfig(ScriptConfig{
		Script: `
			var callCount = 0;
			function transform(record) {
				callCount++;
				record.callCount = callCount;
				return record;
			}`,
	})
	if err != nil {
		t.Fatalf("NewScriptFromConfig() error = %v", err)
	}

	for execution := 1; execution <= 3; execution++ {
		out, err := module.Process(context.Background(), []map[string]any{{"id": "1"}})
		if err != nil {
			t.Fatalf("execution %d: Process() error = %v", execution, err)
		}
		if len(out) != 1 {
			t.Fatalf("execution %d: got %d records, want 1", execution, len(out))
		}
		// Each execution starts from a fresh runtime, so the counter is always 1.
		if got := out[0]["callCount"]; got != int64(1) && got != 1.0 {
			t.Errorf("execution %d: callCount = %v (%T), want 1 — globals leaked "+
				"between executions", execution, got, got)
		}
	}
}

// Within a single execution the runtime is shared across records, which is what
// makes per-batch state work and keeps the counter meaningful.
func TestScript_GlobalsPersistWithinOneExecution(t *testing.T) {
	module, err := NewScriptFromConfig(ScriptConfig{
		Script: `
			var seen = 0;
			function transform(record) {
				seen++;
				record.seen = seen;
				return record;
			}`,
	})
	if err != nil {
		t.Fatalf("NewScriptFromConfig() error = %v", err)
	}

	records := []map[string]any{{"id": "1"}, {"id": "2"}, {"id": "3"}}
	out, err := module.Process(context.Background(), records)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	for i, record := range out {
		want := i + 1
		if got := record["seen"]; got != int64(want) && got != float64(want) {
			t.Errorf("record %d: seen = %v (%T), want %d", i, got, got, want)
		}
	}
}
