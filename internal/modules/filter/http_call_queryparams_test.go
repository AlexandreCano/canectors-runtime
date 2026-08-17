package filter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/cannectors/runtime/internal/moduleconfig"
	"github.com/cannectors/runtime/pkg/connector"
)

// captureURL runs one record through an http_call module and returns the URL
// the server actually received, decoded.
func captureURL(t *testing.T, cfg HTTPCallConfig, record map[string]any) *url.URL {
	t.Helper()

	var got *url.URL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := *r.URL
		got = &u
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	cfg.Endpoint = server.URL + cfg.Endpoint
	if cfg.OnError == "" {
		cfg.OnError = "fail"
	}
	if len(cfg.Keys) == 0 {
		// http_call requires a key or a body; a header key keeps the URL clean.
		cfg.Keys = []moduleconfig.KeyConfig{{Field: "id", ParamType: "header", ParamName: "X-Id"}}
	}

	module, err := NewHTTPCallFromConfig(cfg)
	if err != nil {
		t.Fatalf("failed to create module: %v", err)
	}
	if _, err := module.Process(context.Background(), []map[string]any{record}); err != nil {
		t.Fatalf("Process returned an error: %v", err)
	}
	if got == nil {
		t.Fatal("server received no request")
	}
	return got
}

// AC2: queryParams was accepted by the schema on http_call and silently dropped.
func TestHTTPCallQueryParamsAreSent(t *testing.T) {
	got := captureURL(t, HTTPCallConfig{
		HTTPRequestBase: moduleconfig.HTTPRequestBase{
			Endpoint:    "/api/orders",
			QueryParams: map[string]string{"status": "open", "limit": "50"},
		},
		ModuleBase: connector.ModuleBase{OnError: "fail"},
	}, map[string]any{"id": "1"})

	if v := got.Query().Get("status"); v != "open" {
		t.Errorf("status = %q, want open", v)
	}
	if v := got.Query().Get("limit"); v != "50" {
		t.Errorf("limit = %q, want 50", v)
	}
}

// AC3: values accept templates and are encoded.
func TestHTTPCallQueryParamsSupportTemplates(t *testing.T) {
	got := captureURL(t, HTTPCallConfig{
		HTTPRequestBase: moduleconfig.HTTPRequestBase{
			Endpoint:    "/api/orders",
			QueryParams: map[string]string{"customer": "{{ record.name }}"},
		},
		ModuleBase: connector.ModuleBase{OnError: "fail"},
	}, map[string]any{"id": "1", "name": "Dupont & Fils"})

	if v := got.Query().Get("customer"); v != "Dupont & Fils" {
		t.Errorf("customer = %q, want %q", v, "Dupont & Fils")
	}
}

// A templated value must be encoded exactly once. Rendering with TargetURL and
// then letting the query serializer encode again would send %2520 for a space.
func TestHTTPCallQueryParamsAreNotDoubleEncoded(t *testing.T) {
	got := captureURL(t, HTTPCallConfig{
		HTTPRequestBase: moduleconfig.HTTPRequestBase{
			Endpoint:    "/api/orders",
			QueryParams: map[string]string{"q": "{{ record.term }}"},
		},
		ModuleBase: connector.ModuleBase{OnError: "fail"},
	}, map[string]any{"id": "1", "term": "a b&c"})

	if v := got.Query().Get("q"); v != "a b&c" {
		t.Errorf("q = %q, want %q — a double encoding shows up here", v, "a b&c")
	}
}

// AC4: queryParams wins over the same parameter written into the endpoint.
func TestHTTPCallQueryParamsOverrideEndpointLiteral(t *testing.T) {
	got := captureURL(t, HTTPCallConfig{
		HTTPRequestBase: moduleconfig.HTTPRequestBase{
			Endpoint:    "/api/orders?status=closed",
			QueryParams: map[string]string{"status": "open"},
		},
		ModuleBase: connector.ModuleBase{OnError: "fail"},
	}, map[string]any{"id": "1"})

	if v := got.Query()["status"]; len(v) != 1 || v[0] != "open" {
		t.Errorf("status = %v, want exactly [open]", v)
	}
}

// A per-record key must still win over the module-wide default.
func TestHTTPCallKeyWinsOverQueryParams(t *testing.T) {
	got := captureURL(t, HTTPCallConfig{
		HTTPRequestBase: moduleconfig.HTTPRequestBase{
			Endpoint:    "/api/orders",
			QueryParams: map[string]string{"id": "default"},
		},
		ModuleBase: connector.ModuleBase{OnError: "fail"},
		Keys:       []moduleconfig.KeyConfig{{Field: "id", ParamType: "query", ParamName: "id"}},
	}, map[string]any{"id": "42"})

	if v := got.Query().Get("id"); v != "42" {
		t.Errorf("id = %q, want 42 (the record's key, not the default)", v)
	}
}

// AC5: a value substituted into a path segment must reach the server intact.
// This is the case url.QueryEscape got wrong by encoding the space as +.
func TestHTTPCallPathTemplateSurvivesEncoding(t *testing.T) {
	got := captureURL(t, HTTPCallConfig{
		HTTPRequestBase: moduleconfig.HTTPRequestBase{Endpoint: "/api/customers/{{ record.name }}"},
		ModuleBase:      connector.ModuleBase{OnError: "fail"},
	}, map[string]any{"id": "1", "name": "Dupont & Fils"})

	const want = "/api/customers/Dupont & Fils"
	if got.Path != want {
		t.Errorf("path = %q, want %q", got.Path, want)
	}
}

// AC7: an endpoint with neither template nor queryParams is sent unchanged.
func TestHTTPCallEndpointWithoutParamsIsUnchanged(t *testing.T) {
	got := captureURL(t, HTTPCallConfig{
		HTTPRequestBase: moduleconfig.HTTPRequestBase{Endpoint: "/api/orders?b=2&a=1"},
		ModuleBase:      connector.ModuleBase{OnError: "fail"},
	}, map[string]any{"id": "1"})

	if got.RawQuery != "b=2&a=1" {
		t.Errorf("RawQuery = %q, want %q — the query was reordered or re-encoded", got.RawQuery, "b=2&a=1")
	}
}

// A `{param}` path placeholder must survive the queryParams merge: merging
// parses and re-serializes the URL, which percent-encodes the braces. The
// placeholder was then left unsubstituted and the request went out with a
// literal %7Bid%7D in its path.
func TestHTTPCallPathKeySurvivesQueryParams(t *testing.T) {
	got := captureURL(t, HTTPCallConfig{
		HTTPRequestBase: moduleconfig.HTTPRequestBase{
			Endpoint:    "/api/orders/{id}",
			QueryParams: map[string]string{"status": "open"},
		},
		ModuleBase: connector.ModuleBase{OnError: "fail"},
		Keys:       []moduleconfig.KeyConfig{{Field: "id", ParamType: "path", ParamName: "id"}},
	}, map[string]any{"id": "42"})

	if got.Path != "/api/orders/42" {
		t.Errorf("path = %q, want /api/orders/42", got.Path)
	}
	if v := got.Query().Get("status"); v != "open" {
		t.Errorf("status = %q, want open", v)
	}
}

// The default cache key must separate records that end up on different URLs.
// It was built from the raw endpoint, so a queryParams value differing per
// record shared one slot: the second record was enriched with the first
// record's response, and only one request ever left.
func TestHTTPCallQueryParamsTemplateSeparatesCacheEntries(t *testing.T) {
	var urls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urls = append(urls, r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	module, err := NewHTTPCallFromConfig(HTTPCallConfig{
		HTTPRequestBase: moduleconfig.HTTPRequestBase{
			Endpoint:    server.URL + "/api/customers",
			QueryParams: map[string]string{"tenant": "{{ record.tenantId }}"},
		},
		// The same key value for both records: without the URL in the key,
		// nothing else tells the two calls apart.
		Keys:       []moduleconfig.KeyConfig{{Field: "region", ParamType: "header", ParamName: "X-Region"}},
		Cache:      moduleconfig.CacheConfig{Enabled: true},
		ModuleBase: connector.ModuleBase{OnError: "fail"},
	})
	if err != nil {
		t.Fatalf("failed to create module: %v", err)
	}

	if _, err := module.Process(context.Background(), []map[string]any{
		{"region": "eu", "tenantId": "acme"},
		{"region": "eu", "tenantId": "globex"},
	}); err != nil {
		t.Fatalf("Process returned an error: %v", err)
	}

	if len(urls) != 2 {
		t.Fatalf("got %d requests, want 2 (one per tenant): %v", len(urls), urls)
	}
	if urls[0] == urls[1] {
		t.Errorf("both records hit %q; the tenant did not reach the URL", urls[0])
	}
}

// The cache still caches: two records producing the same URL share one call.
func TestHTTPCallCacheStillHitsOnIdenticalURLs(t *testing.T) {
	var count int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	module, err := NewHTTPCallFromConfig(HTTPCallConfig{
		HTTPRequestBase: moduleconfig.HTTPRequestBase{
			Endpoint:    server.URL + "/api/customers",
			QueryParams: map[string]string{"tenant": "{{ record.tenantId }}"},
		},
		Keys:       []moduleconfig.KeyConfig{{Field: "region", ParamType: "header", ParamName: "X-Region"}},
		Cache:      moduleconfig.CacheConfig{Enabled: true},
		ModuleBase: connector.ModuleBase{OnError: "fail"},
	})
	if err != nil {
		t.Fatalf("failed to create module: %v", err)
	}

	if _, err := module.Process(context.Background(), []map[string]any{
		{"region": "eu", "tenantId": "acme"},
		{"region": "eu", "tenantId": "acme"},
	}); err != nil {
		t.Fatalf("Process returned an error: %v", err)
	}

	if count != 1 {
		t.Errorf("got %d requests, want 1 — the second record should have been a cache hit", count)
	}
}

// Path key, query key and queryParams together: each lands where it belongs and
// the per-record key still overrides the module-wide default.
func TestHTTPCallPathAndQueryKeysWithQueryParams(t *testing.T) {
	got := captureURL(t, HTTPCallConfig{
		HTTPRequestBase: moduleconfig.HTTPRequestBase{
			Endpoint:    "/api/orders/{id}/items",
			QueryParams: map[string]string{"status": "open", "page": "default"},
		},
		ModuleBase: connector.ModuleBase{OnError: "fail"},
		Keys: []moduleconfig.KeyConfig{
			{Field: "id", ParamType: "path", ParamName: "id"},
			{Field: "page", ParamType: "query", ParamName: "page"},
		},
	}, map[string]any{"id": "42", "page": "3"})

	if got.Path != "/api/orders/42/items" {
		t.Errorf("path = %q, want /api/orders/42/items", got.Path)
	}
	if v := got.Query().Get("status"); v != "open" {
		t.Errorf("status = %q, want open", v)
	}
	if v := got.Query().Get("page"); v != "3" {
		t.Errorf("page = %q, want 3 (the record's key, not the default)", v)
	}
}
