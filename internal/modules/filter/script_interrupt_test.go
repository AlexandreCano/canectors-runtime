package filter

import (
	"context"
	"testing"
	"time"
)

// TestScript_RunawayLoopIsInterruptedByContext locks the property that a script
// which never returns cannot pin the process: canceling the context must stop
// it. Without this, a SIGTERM leaves the runtime spinning on a core and only
// SIGKILL ends it, which stalls systemctl stop and rolling deploys.
func TestScript_RunawayLoopIsInterruptedByContext(t *testing.T) {
	module, err := NewScriptFromConfig(ScriptConfig{
		Script: "function transform(record) { while (true) {} return record; }",
	})
	if err != nil {
		t.Fatalf("NewScriptFromConfig() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = module.Process(ctx, []map[string]any{{"id": "1"}})
	}()

	// Let the loop get going, then ask it to stop.
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Process did not return within 5s of canceling the context: " +
			"a runaway script cannot be stopped")
	}
}
