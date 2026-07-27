package runtime

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/cannectors/runtime/internal/modules/input"
	"github.com/cannectors/runtime/internal/modules/output"
	"github.com/cannectors/runtime/pkg/connector"
)

// waitForWebhook waits for the webhook server to be ready (address available).
// Returns true if server is ready, false if timeout.

func waitForWebhook(w *input.Webhook, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if w.Address() != "" {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// waitForRecords waits for the output module to receive the expected number of records.
// Returns true if expected records received, false if timeout.
func waitForRecords(outputModule *MockOutputModule, expected int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if len(outputModule.sentRecords) >= expected {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func TestWebhook_ExecutorIntegration(t *testing.T) {
	config := &connector.ModuleConfig{
		Type: "webhook",
		Raw: mustJSON(map[string]any{
			"path":          "/webhook/test",
			"listenAddress": "127.0.0.1:0",
		}),
	}

	webhook, err := input.NewWebhookFromConfig(config)
	if err != nil {
		t.Fatalf("NewWebhookFromConfig() error = %v", err)
	}

	pipeline := &connector.Pipeline{
		ID:      "pipeline-1",
		Name:    "Test Pipeline",
		Version: "1.0.0",
		Enabled: true,
	}

	outputModule := NewMockOutputModule(nil)
	executor := NewExecutorWithModules(nil, nil, outputModule, false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := func(_ context.Context, data []map[string]any) error {
		_, execErr := executor.ExecuteWithRecords(pipeline, data)
		return execErr
	}

	go func() {
		_ = webhook.Start(ctx, handler)
	}()

	if !waitForWebhook(webhook, 2*time.Second) {
		t.Fatal("Webhook server did not start within timeout")
	}

	addr := webhook.Address()
	payload := `[{"id": 1}, {"id": 2}]`
	resp, err := http.Post("http://"+addr+"/webhook/test", "application/json", bytes.NewBufferString(payload))
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Response status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if !waitForRecords(outputModule, 2, 2*time.Second) {
		t.Fatalf("Timed out waiting for 2 records, got %d", len(outputModule.sentRecords))
	}
}

// concurrentOutputModule records everything it is sent and whether it was closed,
// safely under parallel Send calls.
type concurrentOutputModule struct {
	mu      sync.Mutex
	records []map[string]any
	closed  bool
}

func (m *concurrentOutputModule) Send(_ context.Context, records []map[string]any) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		// What a real database output does once its pool is handed back.
		return 0, errors.New("output is closed")
	}
	m.records = append(m.records, records...)
	return len(records), nil
}

func (m *concurrentOutputModule) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *concurrentOutputModule) snapshot() (int, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.records), m.closed
}

var _ output.Module = (*concurrentOutputModule)(nil)

// TestExecutor_RetainedModulesSurviveConcurrentExecutions covers the webhook
// path as it actually runs: one Executor and one set of modules, serving
// deliveries from several worker goroutines at once.
//
// It pins the two things that path depends on. The modules must still be open
// when the last delivery lands — before RetainModules, the executor closed the
// output at the end of the first execution and every later delivery failed with
// "sql: database is closed" (removing the RetainModules call below reproduces
// exactly that). And the Executor itself must tolerate parallel executions,
// since nothing gives each delivery its own. Run with -race.
func TestExecutor_RetainedModulesSurviveConcurrentExecutions(t *testing.T) {
	pipeline := &connector.Pipeline{
		ID:      "webhook-pipeline",
		Name:    "Webhook Pipeline",
		Version: "1.0.0",
		Enabled: true,
	}

	outputModule := &concurrentOutputModule{}
	executor := NewExecutorWithModules(nil, nil, outputModule, false)
	executor.RetainModules()

	const deliveries = 24
	var wg sync.WaitGroup
	errs := make([]error, deliveries)

	for i := range deliveries {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = executor.ExecuteWithRecordsContext(
				context.Background(), pipeline, []map[string]any{{"id": i}},
			)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("delivery %d failed: %v", i, err)
		}
	}

	sent, closed := outputModule.snapshot()
	if closed {
		t.Error("the retained output was closed by an execution; later deliveries would fail")
	}
	if sent != deliveries {
		t.Errorf("output received %d records, want %d", sent, deliveries)
	}
}
