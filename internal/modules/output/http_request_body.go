package output

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/cannectors/runtime/internal/logger"
	"github.com/cannectors/runtime/internal/metadata"
	"github.com/cannectors/runtime/internal/template"
)

// buildBodyForRecord builds the request body for a single record (template or JSON marshal).
// Returns an error only on marshal failure; invalid JSON from templates is logged but not returned.
func (h *HTTPRequestModule) buildBodyForRecord(record map[string]any, recordIndex int) ([]byte, error) {
	if h.request.bodyTemplateRaw != "" {
		target := h.bodyTargetFor(record)
		bodyStr, err := h.renderField(h.request.bodyTemplateRaw, target, record)
		if err != nil {
			return nil, fmt.Errorf("evaluating body template: %w", err)
		}
		body := []byte(bodyStr)
		if target == template.TargetJSON {
			if validationErr := validateJSON(body); validationErr != nil {
				logger.Warn("body template produced invalid JSON, continuing anyway",
					slog.Int("record_index", recordIndex),
					slog.String("error", validationErr.Error()),
					slog.String("body_preview", truncateString(string(body), 100)),
				)
			}
		}
		return body, nil
	}
	if h.defaultBodyDisabled {
		// AC5: methods outside POST/PUT/PATCH default to no body.
		return nil, nil
	}
	// Strip metadata before serialization
	recordForBody := metadata.StripFromRecord(record, metadata.DefaultFieldName)
	body, err := json.Marshal(recordForBody)
	if err != nil {
		logger.Error("failed to marshal record",
			slog.String("module_type", "httpRequest"),
			slog.Int("record_index", recordIndex),
			slog.String("error", err.Error()),
		)
		return nil, err
	}
	return body, nil
}
