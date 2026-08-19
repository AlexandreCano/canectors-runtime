package output

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/cannectors/runtime/internal/auth"
	"github.com/cannectors/runtime/internal/logger"
	"github.com/cannectors/runtime/pkg/connector"
)

// Maximum size for body preview (1MB) to prevent memory issues with very
// large payloads.
const maxBodyPreviewSize = 1 * 1024 * 1024

// PreviewOperations prepares request previews without actually sending HTTP
// requests. Used in dry-run mode.
//
// Returns one preview per request that would be made:
//   - Batch mode (requestMode="batch"): 1 preview for all records, or one
//     preview per batch of batchSize records when batchSize is set.
//   - Single record mode (requestMode="single"): returns N previews.
//
// Known limitation: a batch preview always shows the default JSON array of the
// batch's records. When a body template is configured, Send renders that
// template instead (against the batch's first record), so the previewed body
// does not match what would be sent.
//
// By default, authentication headers are masked. Set opts.ShowCredentials
// to true to display actual credential values (debugging only).
func (h *HTTPRequestModule) PreviewOperations(records []map[string]any, opts PreviewOptions) ([]connector.OperationPreview, error) {
	if len(records) == 0 {
		return []connector.OperationPreview{}, nil
	}
	if h.request.RequestMode == requestModeSingle {
		return h.previewSingleRecordMode(records, opts)
	}
	return h.previewBatchMode(records, opts)
}

// previewBatchMode builds one preview per request sendBatchMode would emit,
// using the same chunking helper so a dry-run cannot understate the number of
// requests.
func (h *HTTPRequestModule) previewBatchMode(records []map[string]any, opts PreviewOptions) ([]connector.OperationPreview, error) {
	batches := chunkRecords(records, h.request.BatchSize)
	previews := make([]connector.OperationPreview, 0, len(batches))
	for _, batch := range batches {
		preview, err := h.previewOneBatch(batch, opts)
		if err != nil {
			return nil, err
		}
		previews = append(previews, preview)
	}
	return previews, nil
}

func (h *HTTPRequestModule) previewOneBatch(records []map[string]any, opts PreviewOptions) (connector.OperationPreview, error) {
	endpoint, err := h.resolveEndpointForBatch(h.endpoint, records)
	if err != nil {
		return connector.OperationPreview{}, err
	}
	bodyPreview, err := formatJSONPreview(records)
	if err != nil {
		return connector.OperationPreview{}, fmt.Errorf("formatting body preview: %w", err)
	}
	var batchHeaders map[string]string
	if len(records) > 0 {
		batchHeaders, err = h.extractHeadersFromRecord(records[0])
		if err != nil {
			return connector.OperationPreview{}, err
		}
	}
	headers, err := h.buildPreviewHeaders(batchHeaders, opts)
	if err != nil {
		return connector.OperationPreview{}, err
	}
	return httpOperationPreview(endpoint, h.method, headers, bodyPreview, len(records)), nil
}

func (h *HTTPRequestModule) previewSingleRecordMode(records []map[string]any, opts PreviewOptions) ([]connector.OperationPreview, error) {
	previews := make([]connector.OperationPreview, 0, len(records))
	for _, record := range records {
		endpoint, err := h.resolveEndpointForRecord(record)
		if err != nil {
			return nil, err
		}
		bodyPreview, err := formatJSONPreview(record)
		if err != nil {
			return nil, fmt.Errorf("formatting body preview: %w", err)
		}
		recordHeaders, err := h.extractHeadersFromRecord(record)
		if err != nil {
			return nil, err
		}
		headers, err := h.buildPreviewHeaders(recordHeaders, opts)
		if err != nil {
			return nil, err
		}
		previews = append(previews, httpOperationPreview(endpoint, h.method, headers, bodyPreview, 1))
	}
	return previews, nil
}

// buildPreviewHeaders applies auth masking (or unmasking) on top of the base
// headers map.
func (h *HTTPRequestModule) buildPreviewHeaders(recordHeaders map[string]string, opts PreviewOptions) (map[string]string, error) {
	headers, err := h.buildBaseHeadersMap(recordHeaders)
	if err != nil {
		return nil, err
	}
	if opts.ShowCredentials {
		h.addUnmaskedAuthHeaders(headers)
	} else {
		h.addMaskedAuthHeaders(headers)
	}
	return headers, nil
}

func (h *HTTPRequestModule) addMaskedAuthHeaders(headers map[string]string) {
	if h.authHandler == nil {
		return
	}
	switch h.authHandler.Type() {
	case authTypeAPIKey:
		headers[defaultAPIKeyHeader] = maskValue(authTypeAPIKey)
	case "bearer":
		headers["Authorization"] = "Bearer " + maskValue("token")
	case "basic":
		headers["Authorization"] = "Basic " + maskValue("credentials")
	case "oauth2":
		headers["Authorization"] = "Bearer " + maskValue("oauth2-token")
	default:
		if _, hasAuth := headers["Authorization"]; !hasAuth {
			headers["Authorization"] = maskValue("auth")
		}
	}
}

// addUnmaskedAuthHeaders adds authentication headers with real values by
// running a dry apply through the auth handler. WARNING: exposes credentials.
//
// To keep previews side-effect-free, handlers that implement
// auth.PreviewAuthHandler are preferred. That interface guarantees no network
// I/O (e.g., OAuth2 returns ErrPreviewUnavailable if no token is cached, in
// which case we fall back to masked headers rather than fetching a fresh token
// from tokenUrl). Handlers without PreviewAuth (api-key, bearer, basic) have
// stateless ApplyAuth implementations and are safe to call directly.
func (h *HTTPRequestModule) addUnmaskedAuthHeaders(headers map[string]string) {
	if h.authHandler == nil {
		return
	}
	ctx := context.Background()
	mockReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	if err != nil {
		logger.Warn("failed to build mock request for credential preview, falling back to masked headers",
			slog.String("error", err.Error()),
		)
		h.addMaskedAuthHeaders(headers)
		return
	}

	if previewer, ok := h.authHandler.(auth.PreviewAuthHandler); ok {
		if err := previewer.PreviewAuth(ctx, mockReq); err != nil {
			if !errors.Is(err, auth.ErrPreviewUnavailable) {
				logger.Warn("failed to apply auth preview for credential preview, falling back to masked headers",
					slog.String("error", err.Error()),
				)
			}
			h.addMaskedAuthHeaders(headers)
			return
		}
	} else if err := h.authHandler.ApplyAuth(ctx, mockReq); err != nil {
		logger.Warn("failed to apply auth for credential preview, falling back to masked headers",
			slog.String("error", err.Error()),
		)
		h.addMaskedAuthHeaders(headers)
		return
	}

	for key, values := range mockReq.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
}

func maskValue(valueType string) string {
	return "[MASKED-" + strings.ToUpper(valueType) + "]"
}

// formatJSONPreview formats data as indented JSON. If the result exceeds
// maxBodyPreviewSize, it truncates at the nearest line boundary and appends
// a "(truncated, X bytes total)" marker.
func formatJSONPreview(data any) (string, error) {
	formatted, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	s := string(formatted)
	if len(s) <= maxBodyPreviewSize {
		return s, nil
	}
	truncated := s[:maxBodyPreviewSize]
	if lastNewline := strings.LastIndex(truncated, "\n"); lastNewline > maxBodyPreviewSize-100 {
		truncated = truncated[:lastNewline]
	}
	return truncated + fmt.Sprintf("\n... (truncated, %d bytes total, %d bytes shown)", len(formatted), len(truncated)), nil
}
