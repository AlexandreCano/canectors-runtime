package input

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/cannectors/runtime/internal/logger"
)

// fetchWithPagination dispatches to the correct pagination strategy based on
// the module configuration. Non-paginated endpoints are handled by
// fetchSingle directly, not through this file.
func (h *HTTPPolling) fetchWithPagination(ctx context.Context) ([]map[string]any, error) {
	baseEndpoint, err := h.buildEndpointWithState(h.endpoint)
	if err != nil {
		return nil, fmt.Errorf("building endpoint with state: %w", err)
	}
	switch h.pagination.Type {
	case "page":
		return h.fetchPageBased(ctx, baseEndpoint)
	case "offset":
		return h.fetchOffsetBased(ctx, baseEndpoint)
	case "cursor":
		return h.fetchCursorBased(ctx, baseEndpoint)
	default:
		return nil, fmt.Errorf("unknown pagination type %q (expected 'cursor', 'page' or 'offset')", h.pagination.Type)
	}
}

func (h *HTTPPolling) fetchPageBased(ctx context.Context, baseEndpoint string) ([]map[string]any, error) {
	var allRecords []map[string]any
	page := 1
	pagesFetched := 0

	logger.Debug(logMsgPaginationStarted,
		"module_type", "httpPolling",
		"pagination_type", "page",
		"page_param", h.pagination.Param,
	)

	for page <= maxPaginationPages {
		params := h.paginationLimitParams()
		params[h.pagination.Param] = strconv.Itoa(page)
		pageURL, err := h.buildPaginatedURLMultiFrom(baseEndpoint, params)
		if err != nil {
			return nil, err
		}
		records, totalPages, err := h.fetchPageWithMeta(ctx, pageURL, h.pagination.TotalPagesField)
		if err != nil {
			return nil, err
		}
		pagesFetched++

		logger.Debug(logMsgPaginationPageFetched,
			"module_type", "httpPolling",
			"pagination_type", "page",
			"current_page", page,
			"total_pages", totalPages,
			"records_in_page", len(records),
			"total_records_so_far", len(allRecords)+len(records),
		)

		allRecords = append(allRecords, records...)
		if totalPages > 0 && page >= totalPages {
			break
		}
		if len(records) == 0 {
			break
		}
		page++
	}

	logger.Info(logMsgPaginationCompleted,
		"module_type", "httpPolling",
		"pagination_type", "page",
		"pages_fetched", pagesFetched,
		"total_records", len(allRecords),
	)
	return allRecords, nil
}

func (h *HTTPPolling) fetchOffsetBased(ctx context.Context, baseEndpoint string) ([]map[string]any, error) {
	var allRecords []map[string]any
	offset := 0
	limit := h.pagination.Limit
	if limit == 0 {
		limit = 100
	}
	pageNum := 0

	logger.Debug(logMsgPaginationStarted,
		"module_type", "httpPolling",
		"pagination_type", "offset",
		"offset_param", h.pagination.Param,
		"limit_param", h.pagination.LimitParam,
		"limit", limit,
	)

	for offset < maxPaginationPages*limit {
		pageNum++
		offsetURL, err := h.buildPaginatedURLMultiFrom(baseEndpoint, map[string]string{
			h.pagination.Param:      strconv.Itoa(offset),
			h.pagination.LimitParam: strconv.Itoa(limit),
		})
		if err != nil {
			return nil, err
		}
		records, total, err := h.fetchOffsetWithMeta(ctx, offsetURL, h.pagination.TotalField)
		if err != nil {
			return nil, err
		}

		logger.Debug(logMsgPaginationPageFetched,
			"module_type", "httpPolling",
			"pagination_type", "offset",
			"current_offset", offset,
			"limit", limit,
			"total_available", total,
			"records_in_page", len(records),
			"total_records_so_far", len(allRecords)+len(records),
		)

		allRecords = append(allRecords, records...)
		if total > 0 && len(allRecords) >= total {
			break
		}
		if len(records) == 0 || len(records) < limit {
			break
		}
		offset += limit
	}

	logger.Info(logMsgPaginationCompleted,
		"module_type", "httpPolling",
		"pagination_type", "offset",
		"pages_fetched", pageNum,
		"total_records", len(allRecords),
	)
	return allRecords, nil
}

func (h *HTTPPolling) fetchCursorBased(ctx context.Context, baseEndpoint string) ([]map[string]any, error) {
	var allRecords []map[string]any
	cursor := ""
	iterationsFetched := 0

	logger.Debug(logMsgPaginationStarted,
		"module_type", "httpPolling",
		"pagination_type", "cursor",
		"cursor_param", h.pagination.Param,
	)

	for iterationsFetched < maxPaginationPages {
		params := h.paginationLimitParams()
		if cursor != "" {
			params[h.pagination.Param] = cursor
		}
		fetchURL, err := h.buildPaginatedURLMultiFrom(baseEndpoint, params)
		if err != nil {
			return nil, err
		}
		records, nextCursor, err := h.fetchCursorWithMeta(ctx, fetchURL, h.pagination.NextCursorField)
		if err != nil {
			return nil, err
		}
		iterationsFetched++

		logger.Debug(logMsgPaginationPageFetched,
			"module_type", "httpPolling",
			"pagination_type", "cursor",
			"iteration", iterationsFetched,
			"has_next_cursor", nextCursor != "",
			"records_in_page", len(records),
			"total_records_so_far", len(allRecords)+len(records),
		)

		allRecords = append(allRecords, records...)
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	logger.Info(logMsgPaginationCompleted,
		"module_type", "httpPolling",
		"pagination_type", "cursor",
		"iterations", iterationsFetched,
		"total_records", len(allRecords),
	)
	return allRecords, nil
}

func (h *HTTPPolling) fetchPageWithMeta(ctx context.Context, endpoint, totalPagesField string) ([]map[string]any, int, error) {
	obj, err := h.fetchAndParseObject(ctx, endpoint)
	if err != nil {
		return nil, 0, err
	}
	records, err := h.extractRecordsFromObject(obj)
	if err != nil {
		return nil, 0, err
	}
	return records, extractIntField(obj, totalPagesField), nil
}

func (h *HTTPPolling) fetchOffsetWithMeta(ctx context.Context, endpoint, totalField string) ([]map[string]any, int, error) {
	obj, err := h.fetchAndParseObject(ctx, endpoint)
	if err != nil {
		return nil, 0, err
	}
	records, err := h.extractRecordsFromObject(obj)
	if err != nil {
		return nil, 0, err
	}
	return records, extractIntField(obj, totalField), nil
}

func (h *HTTPPolling) fetchCursorWithMeta(ctx context.Context, endpoint, nextCursorField string) ([]map[string]any, string, error) {
	obj, err := h.fetchAndParseObject(ctx, endpoint)
	if err != nil {
		return nil, "", err
	}
	records, err := h.extractRecordsFromObject(obj)
	if err != nil {
		return nil, "", err
	}
	return records, extractStringField(obj, nextCursorField), nil
}

func (h *HTTPPolling) fetchAndParseObject(ctx context.Context, endpoint string) (map[string]any, error) {
	body, err := h.doRequestWithRetry(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrJSONParse, err)
	}
	return result, nil
}

// resolvePath walks obj following a dot-separated field path and returns the
// value at the leaf. A single segment behaves like a plain map lookup, so flat
// field names keep working; nested paths like "meta.next_cursor" are resolved
// segment by segment. This backs dataField / nextCursorField / totalPagesField /
// totalField so the runtime matches the nested paths shown in the docs.
func resolvePath(obj map[string]any, path string) (any, bool) {
	if path == "" {
		return nil, false
	}
	var current any = obj
	for _, segment := range strings.Split(path, ".") {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func extractIntField(obj map[string]any, field string) int {
	if val, ok := resolvePath(obj, field); ok {
		if num, ok := val.(float64); ok {
			return int(num)
		}
	}
	return 0
}

func extractStringField(obj map[string]any, field string) string {
	val, ok := resolvePath(obj, field)
	if !ok {
		return ""
	}
	// Cursors are opaque tokens; APIs often return them as JSON numbers rather
	// than strings. Coerce scalar values so a numeric next_cursor is followed
	// exactly like a string one (integers up to 2^53 render exactly).
	switch v := val.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return ""
	}
}

func (h *HTTPPolling) extractRecordsFromObject(obj map[string]any) ([]map[string]any, error) {
	if h.dataField != "" {
		return h.extractDataFromField(obj, h.dataField)
	}
	commonFields := []string{"data", "items", "results", "records"}
	for _, field := range commonFields {
		if data, ok := obj[field]; ok {
			if converted, err := h.convertToRecords(data); err == nil {
				return converted, nil
			}
		}
	}
	return nil, fmt.Errorf("%w: pagination requires dataField when response is object (tried common fields: %v)", ErrInvalidDataField, commonFields)
}

// paginationLimitParams returns the page-size query param to add to every
// paginated request when limitParam/limit are configured. Offset already emits
// it inline; page and cursor rely on this so all three strategies behave alike.
func (h *HTTPPolling) paginationLimitParams() map[string]string {
	params := map[string]string{}
	if h.pagination.LimitParam != "" && h.pagination.Limit > 0 {
		params[h.pagination.LimitParam] = strconv.Itoa(h.pagination.Limit)
	}
	return params
}

// buildPaginatedURLMultiFrom adds multiple query parameters to baseURL.
func (h *HTTPPolling) buildPaginatedURLMultiFrom(baseURL string, params map[string]string) (string, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf(errMsgParsingEndpointURL, err)
	}
	q := parsedURL.Query()
	for param, value := range params {
		q.Set(param, value)
	}
	parsedURL.RawQuery = q.Encode()
	return parsedURL.String(), nil
}
