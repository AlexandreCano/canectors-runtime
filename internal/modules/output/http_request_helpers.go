package output

import (
	"encoding/json"
	"fmt"
	"mime"
	"strings"

	"github.com/cannectors/runtime/internal/recordpath"
	"github.com/cannectors/runtime/internal/template"
)

// templateEngine is shared by every output module so the compiled-template cache
// is reused across module instances (compiled templates are safe for concurrent
// renders).
var templateEngine = template.NewEngine()

// validateJSON returns an error when data is empty or not valid JSON.
func validateJSON(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty JSON")
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// jsonContentTypeFromHeaders reports whether the Content-Type header indicates a
// JSON payload. mime.ParseMediaType handles charset and other parameters
// (e.g. "application/json; charset=utf-8").
func jsonContentTypeFromHeaders(headers map[string]string) bool {
	contentType := headers[headerContentType]
	if contentType == "" {
		contentType = defaultContentType
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

// bodyTargetFor returns the escaping target for this record's body. The
// Content-Type header may itself be a template, in which case the effective
// media type is only known per record; an unresolvable one falls back to JSON
// escaping (over-escaping is safer than emitting a broken payload).
func (h *HTTPRequestModule) bodyTargetFor(record map[string]any) template.Target {
	if !h.contentTypeTemplated {
		return h.bodyTarget
	}
	rendered, err := h.renderField(h.headers[headerContentType], template.TargetText, record)
	if err != nil || rendered == "" {
		return template.TargetJSON
	}
	if jsonContentTypeFromHeaders(map[string]string{headerContentType: rendered}) {
		return template.TargetJSON
	}
	return template.TargetText
}

// renderField compiles (cached) and renders a template field for the given
// record, escaping substituted values for the target. REST fields use lenient
// undefined handling (missing fields render empty).
func (h *HTTPRequestModule) renderField(src string, target template.Target, record map[string]any) (string, error) {
	compiled, err := h.engine.Compile(src, target, false)
	if err != nil {
		return "", err
	}
	return compiled.Render(template.ContextForRecord(record))
}

// truncateString truncates s to maxLen characters, appending "..." when the
// string is shortened. Intended for log output, not user-visible messages.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// requireRecordFieldString extracts a string value from a record using
// dot-notation path traversal with array indexing support (e.g.
// "user.profile.id" or "items[0].name"), treating absent, null, or empty values
// as errors.
// Used by key resolution (Story 24.12 AC16) where a missing key must surface
// instead of silently dropping a path/query/header parameter.
func requireRecordFieldString(record map[string]any, path string) (string, error) {
	value, ok := recordpath.Get(record, path)
	if !ok {
		return "", fmt.Errorf("field %q is missing in record", path)
	}
	if value == nil {
		return "", fmt.Errorf("field %q is null in record", path)
	}
	str := template.ValueToString(value)
	if str == "" {
		return "", fmt.Errorf("field %q is empty in record", path)
	}
	return str, nil
}
