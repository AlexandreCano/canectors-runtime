package output

import (
	"net/url"
	"strings"
	"testing"
)

// AC6: an endpoint that does not survive template evaluation must stop the
// request, not be sent anyway. It used to be logged at WARN and returned, so the
// module issued a request to a URL it had just judged invalid.
func TestResolveEndpointForRecordRejectsInvalidURL(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		record   map[string]any
	}{
		{
			name:     "template renders an endpoint with no scheme",
			endpoint: "{{ record.base }}/api/orders",
			record:   map[string]any{"base": "not-a-url"},
		},
		{
			name:     "template renders an endpoint with no host",
			endpoint: "https://{{ record.host }}/api/orders",
			record:   map[string]any{"host": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &HTTPRequestModule{
				endpoint: tt.endpoint,
				engine:   templateEngine,
			}

			got, err := module.resolveEndpointForRecord(tt.record)

			if err == nil {
				t.Fatalf("expected an error, got endpoint %q", got)
			}
			if got != "" {
				t.Errorf("returned %q alongside the error; nothing must be sent", got)
			}
			if !strings.Contains(err.Error(), "endpoint") {
				t.Errorf("error does not name the endpoint: %v", err)
			}
		})
	}
}

// A valid endpoint still resolves, so the guard did not simply reject
// everything.
func TestResolveEndpointForRecordAcceptsValidURL(t *testing.T) {
	module := &HTTPRequestModule{
		endpoint: "https://api.example.com/orders/{{ record.id }}",
		engine:   templateEngine,
	}

	got, err := module.resolveEndpointForRecord(map[string]any{"id": "42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://api.example.com/orders/42" {
		t.Errorf("got %q, want %q", got, "https://api.example.com/orders/42")
	}
}

// AC3: queryParams values accept templates on httpRequest too. They used to be
// injected raw, so `{{ record.name }}` was percent-encoded and sent verbatim.
func TestResolveEndpointForRecordRendersQueryParamTemplates(t *testing.T) {
	module := &HTTPRequestModule{
		endpoint: "https://api.example.com/orders",
		engine:   templateEngine,
	}
	module.request.QueryParams = map[string]string{
		"customer": "{{ record.name }}",
		"source":   "cannectors",
	}

	got, err := module.resolveEndpointForRecord(map[string]any{"name": "Dupont & Fils"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("result does not parse: %v (%q)", err, got)
	}
	// Decoded exactly once: a double encoding would surface here as the raw
	// %2520-style value rather than the original text.
	if v := parsed.Query().Get("customer"); v != "Dupont & Fils" {
		t.Errorf("customer = %q, want %q (url: %q)", v, "Dupont & Fils", got)
	}
	if v := parsed.Query().Get("source"); v != "cannectors" {
		t.Errorf("source = %q, want cannectors", v)
	}
}

// AC7: with nothing to add to the query string, the endpoint goes out exactly
// as written. It used to be parsed and re-encoded unconditionally, which sorted
// the parameters of an endpoint this module had no contribution to make to.
func TestResolveEndpointForRecordLeavesTheQueryStringAlone(t *testing.T) {
	const endpoint = "https://api.example.com/orders?b=2&a=1"

	module := &HTTPRequestModule{endpoint: endpoint, engine: templateEngine}

	got, err := module.resolveEndpointForRecord(map[string]any{"id": "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != endpoint {
		t.Errorf("got %q, want the endpoint unchanged %q", got, endpoint)
	}
}

// The guard is on "nothing to add", not on "no query string": a queryParams
// entry still merges into an endpoint that already carries parameters.
func TestResolveEndpointForRecordStillMergesIntoAnExistingQuery(t *testing.T) {
	module := &HTTPRequestModule{
		endpoint: "https://api.example.com/orders?page=2",
		engine:   templateEngine,
	}
	module.request.QueryParams = map[string]string{"status": "open"}

	got, err := module.resolveEndpointForRecord(map[string]any{"id": "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("result does not parse: %v (%q)", err, got)
	}
	if v := parsed.Query().Get("page"); v != "2" {
		t.Errorf("page = %q, want 2 — the endpoint's own parameter was dropped", v)
	}
	if v := parsed.Query().Get("status"); v != "open" {
		t.Errorf("status = %q, want open", v)
	}
}

// A template that cannot be evaluated fails the record instead of sending a
// URL carrying the unrendered markers.
func TestResolveEndpointForRecordRejectsBrokenQueryParamTemplate(t *testing.T) {
	module := &HTTPRequestModule{
		endpoint: "https://api.example.com/orders",
		engine:   templateEngine,
	}
	module.request.QueryParams = map[string]string{"q": "{{ record.name | nosuchfilter }}"}

	got, err := module.resolveEndpointForRecord(map[string]any{"name": "x"})
	if err == nil {
		t.Fatalf("expected an error, got endpoint %q", got)
	}
	if got != "" {
		t.Errorf("returned %q alongside the error; nothing must be sent", got)
	}
	if !strings.Contains(err.Error(), "queryParams") {
		t.Errorf("error does not name the offending field: %v", err)
	}
}
