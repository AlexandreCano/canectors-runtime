// Package output provides implementations for output modules.
package output

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cannectors/runtime/internal/httpclient"
	"github.com/cannectors/runtime/internal/logger"
	"github.com/cannectors/runtime/pkg/connector"
)

// StubModule is a placeholder output module for testing the pipeline flow.
// It implements both Module and PreviewableModule for dry-run support.
type StubModule struct {
	ModuleType string
	Endpoint   string
	Method     string
}

// NewStub creates a new stub output module.
func NewStub(moduleType, endpoint, method string) *StubModule {
	return &StubModule{
		ModuleType: moduleType,
		Endpoint:   endpoint,
		Method:     method,
	}
}

// Send simulates sending records (stub behavior).
func (m *StubModule) Send(_ context.Context, records []map[string]any) (int, error) {
	logger.Info("Output module sending data",
		slog.String("type", m.ModuleType),
		slog.String("endpoint", httpclient.SanitizeURL(m.Endpoint)),
		slog.String("method", m.Method),
		slog.Int("records", len(records)))

	return len(records), nil
}

// Close releases resources (no-op for stub).
func (m *StubModule) Close() error {
	return nil
}

// PreviewOperations generates a preview of what would be sent (for dry-run mode).
func (m *StubModule) PreviewOperations(records []map[string]any, _ PreviewOptions) ([]connector.OperationPreview, error) {
	if len(records) == 0 {
		return nil, nil
	}

	headers := map[string]string{
		"Content-Type": "application/json",
		"User-Agent":   "Cannectors-Runtime/1.0 (stub)",
	}
	bodyPreview := fmt.Sprintf("[%d records would be sent]", len(records))

	return []connector.OperationPreview{
		httpOperationPreview(m.Endpoint, m.Method, headers, bodyPreview, len(records)),
	}, nil
}

// Verify StubModule implements Module and PreviewableModule
var _ Module = (*StubModule)(nil)
var _ PreviewableModule = (*StubModule)(nil)
