package input

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/cannectors/runtime/internal/persistence"
	"github.com/cannectors/runtime/pkg/connector"
)

// capturePollingURL runs one Fetch and returns the URL the server received.
// Optional tweaks run on the module before Fetch, for the state a config alone
// cannot express.
func capturePollingURL(t *testing.T, path string, cfg map[string]any, tweaks ...func(*HTTPPolling)) *url.URL {
	t.Helper()
	return capturePollingURLWithBody(t, path, cfg, `[]`, tweaks...)
}

// capturePollingURLWithBody is capturePollingURL with a chosen response body,
// for the pagination strategies that need an object rather than a bare array.
// Only the first request is captured: pagination issues several.
func capturePollingURLWithBody(t *testing.T, path string, cfg map[string]any, body string, tweaks ...func(*HTTPPolling)) *url.URL {
	t.Helper()

	var got *url.URL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got == nil {
			u := *r.URL
			got = &u
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	cfg["endpoint"] = server.URL + path

	polling, err := NewHTTPPollingFromConfig(&connector.ModuleConfig{
		Type: "http-polling",
		Raw:  mustJSON(cfg),
	})
	if err != nil {
		t.Fatalf("failed to create module: %v", err)
	}
	for _, tweak := range tweaks {
		tweak(polling)
	}
	if _, err := polling.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch returned an error: %v", err)
	}
	if got == nil {
		t.Fatal("server received no request")
	}
	return got
}

// AC1: queryParams was accepted by the schema on httpPolling and silently
// dropped — the pipeline validated, ran, and polled without its filters.
func TestHTTPPollingQueryParamsAreSent(t *testing.T) {
	got := capturePollingURL(t, "/api/orders", map[string]any{
		"queryParams": map[string]any{"status": "open", "limit": "50"},
	})

	if v := got.Query().Get("status"); v != "open" {
		t.Errorf("status = %q, want open", v)
	}
	if v := got.Query().Get("limit"); v != "50" {
		t.Errorf("limit = %q, want 50", v)
	}
}

// AC5: a value carrying URL metacharacters must reach the server intact.
func TestHTTPPollingQueryParamsAreEncoded(t *testing.T) {
	const value = "a&b=c?d#e f"

	got := capturePollingURL(t, "/api/orders", map[string]any{
		"queryParams": map[string]any{"q": value},
	})

	if v := got.Query().Get("q"); v != value {
		t.Errorf("q = %q, want %q", v, value)
	}
	if len(got.Query()) != 1 {
		t.Errorf("value leaked into extra parameters: %v", got.Query())
	}
}

// AC4: queryParams wins over the same parameter written into the endpoint.
func TestHTTPPollingQueryParamsOverrideEndpointLiteral(t *testing.T) {
	got := capturePollingURL(t, "/api/orders?status=closed", map[string]any{
		"queryParams": map[string]any{"status": "open"},
	})

	if v := got.Query()["status"]; len(v) != 1 || v[0] != "open" {
		t.Errorf("status = %v, want exactly [open]", v)
	}
}

// AC7: without queryParams the endpoint is sent unchanged — no reordering, no
// re-encoding from a parse/serialize round trip.
func TestHTTPPollingEndpointWithoutParamsIsUnchanged(t *testing.T) {
	got := capturePollingURL(t, "/api/orders?b=2&a=1", map[string]any{})

	if got.RawQuery != "b=2&a=1" {
		t.Errorf("RawQuery = %q, want %q", got.RawQuery, "b=2&a=1")
	}
}

// On a first run no state exists, so the static default is what remains: the
// merge order must not drop it either.
func TestHTTPPollingQueryParamsSurviveAnEmptyState(t *testing.T) {
	got := capturePollingURL(t, "/api/orders", map[string]any{
		"queryParams": map[string]any{"since": "static-default"},
		"statePersistence": map[string]any{
			"timestamp": map[string]any{"enabled": true, "queryParam": "since"},
		},
	})

	if v := got.Query().Get("since"); v != "static-default" {
		t.Errorf("since = %q, want static-default on a first run", v)
	}
}

// State-derived parameters are computed for the run at hand, so they must win
// over a module-wide default carrying the same name.
func TestHTTPPollingStateParamsWinOverQueryParams(t *testing.T) {
	lastID := "12345"

	got := capturePollingURL(t, "/api/orders", map[string]any{
		"queryParams": map[string]any{"after_id": "static-default"},
		"statePersistence": map[string]any{
			"id": map[string]any{"enabled": true, "queryParam": "after_id", "field": "id"},
		},
	}, func(h *HTTPPolling) {
		// A persisted cursor, as LoadState would have set it.
		h.lastState = &persistence.State{PipelineID: "test", LastID: &lastID}
	})

	if v := got.Query()["after_id"]; len(v) != 1 || v[0] != lastID {
		t.Errorf("after_id = %v, want exactly [%s] — the state cursor, not the default", v, lastID)
	}
}

// queryParams must survive the paginated path too. It rebuilt its own endpoint
// from the raw configuration, so every paginated pipeline polled without its
// parameters — no error, no log, exactly the failure this module was fixed for.
func TestHTTPPollingQueryParamsAreSentWithPagination(t *testing.T) {
	got := capturePollingURLWithBody(t, "/api/orders", map[string]any{
		"queryParams": map[string]any{"status": "open"},
		"dataField":   "data",
		"pagination": map[string]any{
			"type":            "page",
			"param":           "page",
			"totalPagesField": "total_pages",
		},
	}, `{"data":[],"total_pages":1}`)

	if v := got.Query().Get("status"); v != "open" {
		t.Errorf("status = %q, want open (query: %v)", v, got.Query())
	}
	if v := got.Query().Get("page"); v == "" {
		t.Error("pagination parameter missing: the merge dropped it instead")
	}
}

// The state cursor must reach the paginated path as well — it used to be
// rebuilt there, which is what made the two mergers diverge.
func TestHTTPPollingStateParamsAreSentWithPagination(t *testing.T) {
	lastID := "999"

	got := capturePollingURLWithBody(t, "/api/orders", map[string]any{
		"dataField": "data",
		"statePersistence": map[string]any{
			"id": map[string]any{"enabled": true, "queryParam": "after_id", "field": "id"},
		},
		"pagination": map[string]any{
			"type":            "page",
			"param":           "page",
			"totalPagesField": "total_pages",
		},
	}, `{"data":[],"total_pages":1}`, func(h *HTTPPolling) {
		h.lastState = &persistence.State{PipelineID: "test", LastID: &lastID}
	})

	if v := got.Query().Get("after_id"); v != lastID {
		t.Errorf("after_id = %q, want %s (query: %v)", v, lastID, got.Query())
	}
}
