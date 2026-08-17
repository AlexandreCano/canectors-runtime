package input

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/cannectors/runtime/internal/logger"
	"github.com/cannectors/runtime/internal/persistence"
	"github.com/cannectors/runtime/pkg/connector"
)

// capturedRequest is what the source actually received.
type capturedRequest struct {
	URL     *url.URL
	Headers http.Header
	Body    string
}

// pollWithState runs one Fetch with a pre-loaded state and returns the request
// the source saw.
//
// The state is injected directly rather than persisted and reloaded: this
// exercises the rendering, and the load/save round trip has its own tests.
func pollWithState(t *testing.T, cfg map[string]any, state *persistence.State) capturedRequest {
	t.Helper()

	var got capturedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := *r.URL
		body, _ := io.ReadAll(r.Body)
		got = capturedRequest{URL: &u, Headers: r.Header.Clone(), Body: string(body)}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	if endpoint, ok := cfg["endpoint"].(string); ok {
		cfg["endpoint"] = server.URL + endpoint
	} else {
		cfg["endpoint"] = server.URL
	}

	polling, err := NewHTTPPollingFromConfig(&connector.ModuleConfig{
		Type: "http-polling",
		Raw:  mustJSON(cfg),
	})
	if err != nil {
		t.Fatalf("failed to create module: %v", err)
	}
	polling.lastState = state

	if _, err := polling.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch returned an error: %v", err)
	}
	if got.URL == nil {
		t.Fatal("server received no request")
	}
	return got
}

// testTimestamp is the persisted watermark every case in this file renders.
const testTimestamp = "2026-03-15T08:30:00Z"

func stateAt(t *testing.T) *persistence.State {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, testTimestamp)
	if err != nil {
		t.Fatalf("bad test timestamp: %v", err)
	}
	return &persistence.State{LastTimestamp: &ts}
}

// AC1: the watermark reaches an OData-style filter in the endpoint — the shape
// that blocked the most migrations, since it is not a bare query parameter.
func TestHTTPPollingStateInEndpointFilter(t *testing.T) {
	got := pollWithState(t, map[string]any{
		"endpoint": "/api/orders?$filter=LastModifiedDate gt {{ state.lastTimestamp }}",
	}, stateAt(t))

	const want = "LastModifiedDate gt 2026-03-15T08:30:00Z"
	if v := got.URL.Query().Get("$filter"); v != want {
		t.Errorf("$filter = %q, want %q", v, want)
	}
}

// The value must be encoded on the way out — a raw `:` or space in a query
// string is how a filter arrives truncated (Story 25.1).
func TestHTTPPollingStateInEndpointIsEncoded(t *testing.T) {
	got := pollWithState(t, map[string]any{
		"endpoint": "/api/orders?$filter=LastModifiedDate gt {{ state.lastTimestamp }}",
	}, stateAt(t))

	if got.URL.RawQuery == "" {
		t.Fatal("no query string reached the server")
	}
	// Decoding is the real assertion; the raw form must not have leaked spaces.
	if v := got.URL.Query().Get("$filter"); v == "" {
		t.Errorf("filter did not survive encoding, raw query was %q", got.URL.RawQuery)
	}
}

// AC1 in a path segment.
func TestHTTPPollingStateInPathSegment(t *testing.T) {
	got := pollWithState(t, map[string]any{
		"endpoint": "/api/orders/since/{{ state.lastTimestamp }}",
	}, stateAt(t))

	const want = "/api/orders/since/2026-03-15T08:30:00Z"
	if got.URL.Path != want {
		t.Errorf("path = %q, want %q", got.URL.Path, want)
	}
}

// AC2: an If-Modified-Since style header.
func TestHTTPPollingStateInHeader(t *testing.T) {
	got := pollWithState(t, map[string]any{
		"endpoint": "/api/orders",
		"headers":  map[string]any{"X-Since": "{{ state.lastTimestamp }}"},
	}, stateAt(t))

	if v := got.Headers.Get("X-Since"); v != "2026-03-15T08:30:00Z" {
		t.Errorf("X-Since = %q, want the persisted timestamp", v)
	}
}

// AC3: a POST search body — the common shape for paginated search APIs.
func TestHTTPPollingStateInBody(t *testing.T) {
	got := pollWithState(t, map[string]any{
		"endpoint": "/api/orders/search",
		"method":   "POST",
		"body":     `{"since":"{{ state.lastTimestamp }}"}`,
	}, stateAt(t))

	const want = `{"since":"2026-03-15T08:30:00Z"}`
	if got.Body != want {
		t.Errorf("body = %q, want %q", got.Body, want)
	}
}

// AC4: a queryParams value.
func TestHTTPPollingStateInQueryParams(t *testing.T) {
	got := pollWithState(t, map[string]any{
		"endpoint":    "/api/orders",
		"queryParams": map[string]any{"updated_after": "{{ state.lastTimestamp }}"},
	}, stateAt(t))

	if v := got.URL.Query().Get("updated_after"); v != "2026-03-15T08:30:00Z" {
		t.Errorf("updated_after = %q, want the persisted timestamp", v)
	}
}

// AC5: the ID cursor is available in the same places.
func TestHTTPPollingStateIDIsAvailable(t *testing.T) {
	id := "cursor-42"
	got := pollWithState(t, map[string]any{
		"endpoint": "/api/orders?after={{ state.lastId }}",
	}, &persistence.State{LastID: &id})

	if v := got.URL.Query().Get("after"); v != "cursor-42" {
		t.Errorf("after = %q, want cursor-42", v)
	}
}

// AC7: on a first run the timestamp is the epoch, so a naive filter means
// "everything so far" rather than producing a malformed comparison.
func TestHTTPPollingStateFirstRunUsesEpoch(t *testing.T) {
	got := pollWithState(t, map[string]any{
		"endpoint": "/api/orders?$filter=LastModifiedDate gt {{ state.lastTimestamp }}",
	}, nil)

	const want = "LastModifiedDate gt " + persistence.EpochTimestamp
	if v := got.URL.Query().Get("$filter"); v != want {
		t.Errorf("$filter = %q, want %q", v, want)
	}
}

// AC7: the ID has no such neutral value, so it is absent and default() supplies
// one — which is why it must be absent rather than an empty string.
func TestHTTPPollingStateFirstRunIDDefaults(t *testing.T) {
	got := pollWithState(t, map[string]any{
		"endpoint": `/api/orders?after={{ state.lastId | default("0") }}`,
	}, nil)

	if v := got.URL.Query().Get("after"); v != "0" {
		t.Errorf("after = %q, want the default 0", v)
	}
}

// The shape of examples/28: a filter written into the endpoint *and* static
// queryParams. Merging the parameters used to re-encode the whole query, so the
// filter left as `%24filter=…+gt+…` — decodable, but not the request the
// pipeline describes.
func TestHTTPPollingStateInEndpointFilterWithQueryParams(t *testing.T) {
	got := pollWithState(t, map[string]any{
		"endpoint":    "/api/orders?$filter=LastModifiedDate gt {{ state.lastTimestamp }}",
		"queryParams": map[string]any{"$orderby": "LastModifiedDate asc", "$top": "200"},
	}, stateAt(t))

	if !strings.HasPrefix(got.URL.RawQuery, "$filter=") {
		t.Errorf("RawQuery = %q, want the endpoint's own filter first and unencoded", got.URL.RawQuery)
	}
	if strings.Contains(got.URL.RawQuery, "+") {
		t.Errorf("RawQuery = %q, want spaces as %%20 rather than +", got.URL.RawQuery)
	}
	const wantFilter = "LastModifiedDate gt 2026-03-15T08:30:00Z"
	if v := got.URL.Query().Get("$filter"); v != wantFilter {
		t.Errorf("$filter = %q, want %q", v, wantFilter)
	}
	if v := got.URL.Query().Get("$orderby"); v != "LastModifiedDate asc" {
		t.Errorf("$orderby = %q, want it intact", v)
	}
	// Checked on the raw query: Query() decodes `%24top` to `$top`, so it cannot
	// tell the two spellings apart, and one URL should not mix them.
	if !strings.Contains(got.URL.RawQuery, "&$top=200") {
		t.Errorf("RawQuery = %q, want the added parameter spelled $top, not %%24top", got.URL.RawQuery)
	}
}

// The `queryParam` shortcut and a templated endpoint in the same pipeline: the
// watermark is appended, the author's filter is not rewritten around it.
func TestHTTPPollingStateQueryParamKeepsEndpointShape(t *testing.T) {
	got := pollWithState(t, map[string]any{
		"endpoint": "/api/orders?$filter=Status eq 'open'&b=2&a=1",
		"statePersistence": map[string]any{
			"timestamp": map[string]any{"enabled": true, "queryParam": "since"},
		},
	}, stateAt(t))

	if !strings.HasPrefix(got.URL.RawQuery, "$filter=Status%20eq%20'open'&b=2&a=1&") {
		t.Errorf("RawQuery = %q, want the endpoint's query preserved before the watermark", got.URL.RawQuery)
	}
	if v := got.URL.Query().Get("since"); v != testTimestamp {
		t.Errorf("since = %q, want the persisted timestamp", v)
	}
}

// A state variable that does not exist renders as nothing, which turns an
// incremental filter into a malformed one. The name is fixed and known, so it is
// rejected when the module is built rather than by the source's 400.
func TestHTTPPollingUnknownStateVariableIsRejected(t *testing.T) {
	for name, cfg := range map[string]map[string]any{
		"endpoint":    {"endpoint": "https://api.example.com/orders?since={{ state.lastTimestmp }}"},
		"header":      {"endpoint": "https://api.example.com/orders", "headers": map[string]any{"X-Since": "{{ state.lastRunTimestamp }}"}},
		"queryParams": {"endpoint": "https://api.example.com/orders", "queryParams": map[string]any{"since": "{{ state.lastRunId }}"}},
		"body":        {"endpoint": "https://api.example.com/orders", "method": "POST", "body": `{"since":"{{ state.whatever }}"}`},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewHTTPPollingFromConfig(&connector.ModuleConfig{
				Type: "http-polling",
				Raw:  mustJSON(cfg),
			})
			if err == nil {
				t.Fatal("expected the unknown state variable to be rejected")
			}
			if !strings.Contains(err.Error(), "unknown state variable") {
				t.Errorf("error = %v, want it to name the unknown variable", err)
			}
		})
	}
}

// A record field that happens to be called state is not this map, and must not
// be validated against its names.
func TestHTTPPollingRecordStateFieldIsNotAStateReference(t *testing.T) {
	_, err := NewHTTPPollingFromConfig(&connector.ModuleConfig{
		Type: "http-polling",
		Raw:  mustJSON(map[string]any{"endpoint": "https://api.example.com/state.json?x={{ record.state.whatever }}"}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// The body's escaping follows its declared Content-Type: JSON escaping inside an
// XML body is valid nowhere.
func TestHTTPPollingBodyEscapingFollowsContentType(t *testing.T) {
	id := `a"b<c&d`
	for name, tc := range map[string]struct {
		contentType string
		want        string
	}{
		// json.Marshal also escapes < and & — the escaping is the encoder's, the
		// point here is which one gets applied.
		"json":     {"application/json", `{"id":"a\"b\u003cc\u0026d"}`},
		"xml":      {"application/xml", `{"id":"a&#34;b&lt;c&amp;d"}`},
		"none":     {"", `{"id":"a\"b\u003cc\u0026d"}`},
		"json+ext": {"application/vnd.api+json; charset=utf-8", `{"id":"a\"b\u003cc\u0026d"}`},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := map[string]any{
				"endpoint": "/api/orders/search",
				"method":   "POST",
				"body":     `{"id":"{{ state.lastId }}"}`,
			}
			if tc.contentType != "" {
				cfg["headers"] = map[string]any{"Content-Type": tc.contentType}
			}

			got := pollWithState(t, cfg, &persistence.State{LastID: &id})

			if got.Body != tc.want {
				t.Errorf("body = %q, want %q", got.Body, tc.want)
			}
		})
	}
}

// A pipeline with no templates at all must keep working untouched.
func TestHTTPPollingWithoutTemplatesIsUnchanged(t *testing.T) {
	got := pollWithState(t, map[string]any{
		"endpoint": "/api/orders?b=2&a=1",
		"headers":  map[string]any{"X-Static": "value"},
	}, nil)

	if got.URL.RawQuery != "b=2&a=1" {
		t.Errorf("RawQuery = %q, want it untouched", got.URL.RawQuery)
	}
	if v := got.Headers.Get("X-Static"); v != "value" {
		t.Errorf("X-Static = %q, want value", v)
	}
}

// Pagination is applied to an endpoint Fetch has already shaped — state
// rendered, queryParams merged, spaces encoded. Adding `page` used to go
// through url.Values and re-serialize all of it, so the OData filter this story
// exists for came back as `%24filter=…+gt+…` on every page but the first.
func TestHTTPPollingPaginationKeepsEndpointShape(t *testing.T) {
	var got *url.URL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := *r.URL
		got = &u
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"value":[],"totalPages":1}`))
	}))
	defer server.Close()

	polling, err := NewHTTPPollingFromConfig(&connector.ModuleConfig{
		Type: "http-polling",
		Raw: mustJSON(map[string]any{
			"endpoint":  server.URL + "/api/orders?$filter=LastModifiedDate gt {{ state.lastTimestamp }}&b=2&a=1",
			"dataField": "value",
			"pagination": map[string]any{
				"type":            "page",
				"param":           "page",
				"limitParam":      "$top",
				"limit":           50,
				"totalPagesField": "totalPages",
			},
		}),
	})
	if err != nil {
		t.Fatalf("failed to create module: %v", err)
	}
	polling.lastState = stateAt(t)

	if _, err := polling.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch returned an error: %v", err)
	}
	if got == nil {
		t.Fatal("server received no request")
	}

	const wantPrefix = "$filter=LastModifiedDate%20gt%202026-03-15T08%3A30%3A00Z&b=2&a=1&"
	if !strings.HasPrefix(got.RawQuery, wantPrefix) {
		t.Errorf("RawQuery = %q, want it to start with %q", got.RawQuery, wantPrefix)
	}
	if !strings.Contains(got.RawQuery, "&page=1") || !strings.Contains(got.RawQuery, "$top=50") {
		t.Errorf("RawQuery = %q, want the pagination parameters appended", got.RawQuery)
	}
	if strings.Contains(got.RawQuery, "+") {
		t.Errorf("RawQuery = %q, want spaces as %%20 rather than +", got.RawQuery)
	}
}

// timestamp.enabled and id.enabled are separate switches, so "persistence is
// on" is not enough: a pipeline persisting only the ID while filtering on
// state.lastTimestamp renders the epoch on every run, forever.
func TestHTTPPollingWarnsPerUnpersistedStateVariable(t *testing.T) {
	var buf bytes.Buffer
	previous := logger.Logger
	logger.Logger = slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	t.Cleanup(func() { logger.Logger = previous })

	_, err := NewHTTPPollingFromConfig(&connector.ModuleConfig{
		Type: "http-polling",
		Raw: mustJSON(map[string]any{
			"endpoint": "https://api.example.com/orders?since={{ state.lastTimestamp }}&after={{ state.lastId }}",
			"statePersistence": map[string]any{
				"id": map[string]any{"enabled": true, "field": "id"},
			},
		}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logs := buf.String()
	if !strings.Contains(logs, `"variable":"state.lastTimestamp"`) {
		t.Errorf("no warning for the unpersisted timestamp, logs were %q", logs)
	}
	if strings.Contains(logs, `"variable":"state.lastId"`) {
		t.Errorf("warned about state.lastId, which this pipeline does persist: %q", logs)
	}
}
