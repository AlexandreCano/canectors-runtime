package filter

import (
	"context"
	"testing"
)

const benchmarkScript = `
function transform(record) {
  record.upper = String(record.name).toUpperCase();
  record.total = (record.qty || 0) * (record.price || 0);
  return record;
}`

// These two benchmarks are why the runtime is isolated per execution rather than
// kept on the module. Building a module compiles the script; starting an
// execution only instantiates a runtime from the compiled program. The second
// figure is what isolation actually costs, and it is small enough that trading
// it for persistent globals — and the unbounded leak they allow — never made
// sense. Compare against a tick, which is milliseconds.
func BenchmarkScript_BuildModule(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := NewScriptFromConfig(ScriptConfig{Script: benchmarkScript}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkScript_NewRunFromCompiledProgram(b *testing.B) {
	module, err := NewScriptFromConfig(ScriptConfig{Script: benchmarkScript})
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := module.newScriptRun(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkScript_Process100Records(b *testing.B) {
	module, err := NewScriptFromConfig(ScriptConfig{Script: benchmarkScript})
	if err != nil {
		b.Fatal(err)
	}
	records := make([]map[string]any, 100)
	for i := range records {
		records[i] = map[string]any{"name": "widget", "qty": 3, "price": 4.5}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := module.Process(context.Background(), records); err != nil {
			b.Fatal(err)
		}
	}
}
