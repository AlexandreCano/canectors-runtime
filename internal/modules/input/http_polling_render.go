package input

import (
	"fmt"
	"mime"
	"slices"
	"strings"

	"github.com/cannectors/runtime/internal/httpclient"
	"github.com/cannectors/runtime/internal/logger"
	"github.com/cannectors/runtime/internal/persistence"
	"github.com/cannectors/runtime/internal/template"
)

// validateStateTemplates checks, once at construction, every field this module
// renders against the state.
//
// Two things are caught here rather than at request time: a `state.` variable
// that does not exist, which renders as nothing and turns an incremental filter
// into a malformed one; and a pipeline reading a variable it does not persist,
// which is not an error — the variable still renders — but means every run
// polls from the epoch forever, with the source alone to notice.
//
// The persistence check is per variable, not per module: `timestamp.enabled`
// and `id.enabled` are separate switches, so a pipeline persisting only the ID
// while filtering on `state.lastTimestamp` has exactly the same silent full
// refetch as one persisting nothing at all.
func (h *HTTPPolling) validateStateTemplates() error {
	fields := map[string]string{
		"endpoint": h.endpoint,
		"body":     h.bodyTemplate,
	}
	for name, value := range h.headers {
		fields[fmt.Sprintf("headers[%q]", name)] = value
	}
	for name, value := range h.queryParams {
		fields[fmt.Sprintf("queryParams[%q]", name)] = value
	}

	var read []string
	for field, src := range fields {
		if err := persistence.ValidateStateReferences(field, src); err != nil {
			return fmt.Errorf("httpPolling %w", err)
		}
		for _, name := range persistence.StateReferences(src) {
			if !slices.Contains(read, name) {
				read = append(read, name)
			}
		}
	}

	// Sorted because fields iterates a map: the same pipeline should not warn in
	// a different order from one run to the next.
	slices.Sort(read)
	for _, name := range read {
		if h.persistedVariable(name) {
			continue
		}
		logger.Warn("pipeline reads a state variable it does not persist; every run will start from the epoch",
			"module_type", "httpPolling",
			"variable", "state."+name,
			"endpoint", httpclient.SanitizeURL(h.endpoint),
		)
	}
	return nil
}

// persistedVariable reports whether the state variable name is actually written
// back at the end of a run.
func (h *HTTPPolling) persistedVariable(name string) bool {
	switch name {
	case persistence.VarLastTimestamp:
		return h.persistenceConfig.TimestampEnabled()
	case persistence.VarLastID:
		return h.persistenceConfig.IDEnabled()
	default:
		// Unreachable: ValidateStateReferences has already rejected any other
		// name. Warning about it would be noise on top of an error.
		return true
	}
}

// stateRenderContext exposes the persisted state to this module's templates.
//
// There is no record here — an input has not fetched anything yet — so `record`
// is empty and `state` is the whole point.
func (h *HTTPPolling) stateRenderContext() template.RenderContext {
	return template.RenderContext{State: persistence.RenderState(h.lastState, h.persistenceConfig)}
}

// renderStateField renders a configured field against the state context.
//
// This module had no template rendering at all: the endpoint, headers and body
// were used verbatim, so the persisted watermark could only leave through a
// query parameter whose name was fixed in `statePersistence`. Any source
// filtering incrementally by another route — an OData `$filter`, a path
// segment, an `If-Modified-Since` header, a POST search body — was simply out of
// reach (Story 25.5).
func (h *HTTPPolling) renderStateField(src string, target template.Target) (string, error) {
	if !template.HasVariables(src) {
		return src, nil
	}
	compiled, err := templateEngine.Compile(src, target, false)
	if err != nil {
		return "", err
	}
	return compiled.Render(h.stateRenderContext())
}

// renderEndpoint resolves templates in the endpoint.
func (h *HTTPPolling) renderEndpoint() (string, error) {
	rendered, err := h.renderStateField(h.endpoint, template.TargetURL)
	if err != nil {
		return "", fmt.Errorf("evaluating endpoint template: %w", err)
	}
	return rendered, nil
}

// renderQueryParams resolves templates in the static query parameter values.
//
// TargetText, not TargetURL: MergeQueryParams encodes them when it serializes
// the query string, and escaping twice would send %2520 for a space. Same rule
// as http_call and httpRequest.
func (h *HTTPPolling) renderQueryParams() (map[string]string, error) {
	if len(h.queryParams) == 0 {
		return nil, nil
	}
	rendered := make(map[string]string, len(h.queryParams))
	for name, value := range h.queryParams {
		out, err := h.renderStateField(value, template.TargetText)
		if err != nil {
			return nil, fmt.Errorf("evaluating queryParams[%q] template: %w", name, err)
		}
		rendered[name] = out
	}
	return rendered, nil
}

// renderHeaders resolves templates in the header values.
func (h *HTTPPolling) renderHeaders() (map[string]string, error) {
	if len(h.headers) == 0 {
		return nil, nil
	}
	rendered := make(map[string]string, len(h.headers))
	for name, value := range h.headers {
		out, err := h.renderStateField(value, template.TargetText)
		if err != nil {
			return nil, fmt.Errorf("evaluating headers[%q] template: %w", name, err)
		}
		rendered[name] = out
	}
	return rendered, nil
}

// renderBody resolves templates in the request body, escaping substituted
// values for the body's own format.
func (h *HTTPPolling) renderBody() (string, error) {
	rendered, err := h.renderStateField(h.bodyTemplate, h.bodyTarget())
	if err != nil {
		return "", fmt.Errorf("evaluating body template: %w", err)
	}
	return rendered, nil
}

// bodyTarget picks the escaping for the request body from its declared
// Content-Type.
//
// Escaping has to match the format actually being sent: JSON escaping inside an
// XML body turns a `"` into `\"` — valid nowhere — and leaves `<` and `&` free
// to break the document. A body with no declared Content-Type stays on JSON,
// which is what a polling POST search sends in practice.
func (h *HTTPPolling) bodyTarget() template.Target {
	contentType := ""
	for name, value := range h.headers {
		if strings.EqualFold(name, "Content-Type") {
			contentType = value
			break
		}
	}
	if contentType == "" {
		return template.TargetJSON
	}
	// mime.ParseMediaType rather than a Contains check: the header carries
	// parameters (`application/json; charset=utf-8`) that are not part of the
	// type.
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return template.TargetJSON
	}
	switch {
	case mediaType == "application/x-www-form-urlencoded":
		return template.TargetURL
	case strings.HasSuffix(mediaType, "/xml"), strings.HasSuffix(mediaType, "+xml"):
		return template.TargetXML
	case strings.HasSuffix(mediaType, "/json"), strings.HasSuffix(mediaType, "+json"):
		return template.TargetJSON
	default:
		// text/plain, text/csv and the like: there is no escaping that means
		// anything, and inventing one corrupts the value.
		return template.TargetText
	}
}
