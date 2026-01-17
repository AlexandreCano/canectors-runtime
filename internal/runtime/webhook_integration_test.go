package runtime

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/canectors/runtime/internal/modules/input"
	"github.com/canectors/runtime/pkg/connector"
)

func TestWebhook_ExecutorIntegration(t *testing.T) {
	config := &connector.ModuleConfig{
		Type: "webhook",
		Config: map[string]interface{}{
			"endpoint":      "/webhook/test",
			"listenAddress": "127.0.0.1:0",
		},
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

	handler := func(data []map[string]interface{}) error {
		_, execErr := executor.ExecuteWithRecords(pipeline, data)
		return execErr
	}

	go func() {
		_ = webhook.Start(ctx, handler)
	}()
	time.Sleep(100 * time.Millisecond)

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

	time.Sleep(50 * time.Millisecond)
	if len(outputModule.sentRecords) != 2 {
		t.Errorf("Output received %d records, want 2", len(outputModule.sentRecords))
	}
}
