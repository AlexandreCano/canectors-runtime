package filter

import (
	"context"
	"sync"
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

// TestScript_ConcurrentExecutionsOnOneModule covers the webhook path, where the
// gap between reuse and isolation actually shows.
//
// Deliveries are served by maxConcurrent workers calling into the *same* module
// instance, so Process runs in parallel on it. A goja.Runtime is not
// goroutine-safe: a runtime hoisted onto the module would corrupt state or panic
// here, and the sequential isolation test above would not notice — it only ever
// runs one execution at a time. Run with -race.
func TestScript_ConcurrentExecutionsOnOneModule(t *testing.T) {
	module, err := NewScriptFromConfig(ScriptConfig{
		Script: `
			var callCount = 0;
			function transform(record) {
				callCount++;
				record.callCount = callCount;
				record.doubled = record.value * 2;
				return record;
			}`,
	})
	if err != nil {
		t.Fatalf("NewScriptFromConfig() error = %v", err)
	}

	const deliveries = 32
	var wg sync.WaitGroup
	results := make([][]map[string]any, deliveries)
	errs := make([]error, deliveries)

	for i := range deliveries {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = module.Process(
				context.Background(), []map[string]any{{"value": i}},
			)
		}()
	}
	wg.Wait()

	for i := range deliveries {
		if errs[i] != nil {
			t.Fatalf("delivery %d: Process() error = %v", i, errs[i])
		}
		if len(results[i]) != 1 {
			t.Fatalf("delivery %d: got %d records, want 1", i, len(results[i]))
		}
		record := results[i][0]
		// Each delivery got its own runtime, so no counter was shared.
		if got := record["callCount"]; got != int64(1) && got != 1.0 {
			t.Errorf("delivery %d: callCount = %v, want 1 — a runtime was shared "+
				"between concurrent executions", i, got)
		}
		// And no delivery saw another's record.
		if got := record["doubled"]; got != int64(i*2) && got != float64(i*2) {
			t.Errorf("delivery %d: doubled = %v (%T), want %d", i, got, got, i*2)
		}
	}
}
