package output

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/cannectors/runtime/internal/httpclient"
	"github.com/cannectors/runtime/internal/logger"
	"github.com/cannectors/runtime/internal/template"
)

// resolveEndpointWithStaticQuery appends the module's static query params to an
// endpoint URL that has no record to render against.
//
// Only the dry-run preview reaches it, and only with an empty batch: Send
// returns early on zero records. The endpoint therefore still carries its
// unrendered `{{ }}` markers, and merging static params round-trips the URL,
// which percent-encodes those markers — acceptable in a preview of a batch that
// has nothing to send, wrong anywhere a request is actually issued. Keep this
// off the sending paths; they go through resolveEndpointForRecord.
//
// A parse failure is warned about rather than raised for the same reason: there
// is no request to stop.
func (h *HTTPRequestModule) resolveEndpointWithStaticQuery(endpoint string) string {
	merged, err := httpclient.MergeQueryParams(endpoint, h.request.QueryParams)
	if err != nil {
		logger.Warn("failed to parse endpoint URL for query params",
			slog.String("endpoint", httpclient.SanitizeURL(endpoint)),
			slog.String("error", err.Error()),
		)
		return endpoint
	}
	return merged
}

// resolveEndpointForBatch resolves template variables and keys in the
// endpoint for batch mode. Uses the first record for template evaluation
// and key extraction.
func (h *HTTPRequestModule) resolveEndpointForBatch(endpoint string, records []map[string]any) (string, error) {
	if len(records) == 0 {
		return h.resolveEndpointWithStaticQuery(endpoint), nil
	}
	return h.resolveEndpointForRecord(records[0])
}

// resolveEndpointForRecord resolves path parameters, template variables, and
// query params for a single record. Templates ({{record.field}}) are
// evaluated first, then path parameters ({param}) are substituted.
// Returns an error when a configured key references a missing/null/empty
// record field (Story 24.12 AC16).
func (h *HTTPRequestModule) resolveEndpointForRecord(record map[string]any) (string, error) {
	endpoint := h.endpoint
	if template.HasVariables(endpoint) {
		rendered, err := h.renderField(endpoint, template.TargetURL, record)
		if err != nil {
			return "", fmt.Errorf("evaluating endpoint template: %w", err)
		}
		endpoint = rendered
	}

	for _, k := range h.request.Keys {
		if k.paramType == "path" {
			value, err := requireRecordFieldString(record, k.field)
			if err != nil {
				return "", fmt.Errorf("path key %q: %w", k.paramName, err)
			}
			placeholder := "{" + k.paramName + "}"
			endpoint = strings.ReplaceAll(endpoint, placeholder, url.PathEscape(value))
		}
	}

	// With nothing to add to the query string, the endpoint is left alone: a
	// url.Parse/Encode round trip sorts the parameters and re-encodes them, so
	// an endpoint the author wrote as `?b=2&a=1` went out as `?a=1&b=2` even
	// when this module had nothing to contribute (Story 25.1 AC7). httpPolling
	// and http_call already left it untouched; the output was the odd one out.
	finalURL := endpoint
	if len(h.request.QueryParams) > 0 || h.hasQueryKey() {
		// Same reasoning as the validation below: an endpoint that will not parse
		// is an endpoint the module cannot honor, and returning it unchanged
		// skipped the query parameters and the keys it was about to add.
		parsedURL, err := url.Parse(endpoint)
		if err != nil {
			return "", fmt.Errorf("parsing endpoint URL after template evaluation: %w", err)
		}

		q := parsedURL.Query()
		for param, value := range h.request.QueryParams {
			rendered, err := h.renderQueryParam(param, value, record)
			if err != nil {
				return "", err
			}
			q.Set(param, rendered)
		}
		for _, k := range h.request.Keys {
			if k.paramType == "query" {
				value, err := requireRecordFieldString(record, k.field)
				if err != nil {
					return "", fmt.Errorf("query key %q: %w", k.paramName, err)
				}
				q.Set(k.paramName, value)
			}
		}
		parsedURL.RawQuery = q.Encode()
		finalURL = parsedURL.String()
	}

	// An endpoint that does not survive template evaluation is an error, not a
	// warning: sending it anyway means issuing a request nobody described, to a
	// host nobody chose. Returning it left the caller to send a URL this
	// function had just judged invalid (Story 25.1 AC6).
	if err := validateURL(finalURL); err != nil {
		return "", fmt.Errorf("endpoint is invalid after template evaluation: %w", err)
	}
	return finalURL, nil
}

// hasQueryKey reports whether any configured key targets the query string.
func (h *HTTPRequestModule) hasQueryKey() bool {
	for _, k := range h.request.Keys {
		if k.paramType == "query" {
			return true
		}
	}
	return false
}

// renderQueryParam evaluates a `queryParams` value against the record.
//
// Rendered with TargetText, not TargetURL: the query serializer encodes the
// value itself, and escaping it twice would send %2520 for a space. Same
// treatment as http_call, so that `queryParams` means the same thing on both.
func (h *HTTPRequestModule) renderQueryParam(param, value string, record map[string]any) (string, error) {
	if !template.HasVariables(value) {
		return value, nil
	}
	rendered, err := h.renderField(value, template.TargetText, record)
	if err != nil {
		return "", fmt.Errorf("evaluating queryParams[%q] template: %w", param, err)
	}
	return rendered, nil
}

// validateURL validates that a URL string is well-formed.
func validateURL(urlStr string) error {
	if urlStr == "" {
		return fmt.Errorf("empty URL")
	}
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}
	if parsed.Scheme == "" {
		return fmt.Errorf("URL missing scheme")
	}
	if parsed.Scheme == "http" || parsed.Scheme == "https" {
		if parsed.Host == "" {
			return fmt.Errorf("URL missing host")
		}
	}
	return nil
}
