package httpclient

import (
	"net/url"
	"strings"
	"testing"
)

func TestMergeQueryParams(t *testing.T) {
	t.Run("no params returns the endpoint verbatim", func(t *testing.T) {
		// AC7: an endpoint with nothing to add must not be reshaped by a
		// parse/serialize round trip.
		const endpoint = "https://api.example.com/v1/orders?b=2&a=1"

		got, err := MergeQueryParams(endpoint, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != endpoint {
			t.Errorf("got %q, want the endpoint unchanged %q", got, endpoint)
		}
	})

	t.Run("empty map returns the endpoint verbatim", func(t *testing.T) {
		const endpoint = "https://api.example.com/v1/orders"

		got, err := MergeQueryParams(endpoint, map[string]string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != endpoint {
			t.Errorf("got %q, want %q", got, endpoint)
		}
	})

	t.Run("adds a parameter", func(t *testing.T) {
		got, err := MergeQueryParams("https://api.example.com/v1/orders", map[string]string{"status": "open"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertQuery(t, got, "status", "open")
	})

	t.Run("keeps parameters already in the endpoint", func(t *testing.T) {
		got, err := MergeQueryParams("https://api.example.com/v1/orders?page=2", map[string]string{"status": "open"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertQuery(t, got, "page", "2")
		assertQuery(t, got, "status", "open")
	})

	// AC4: the explicit field wins over the endpoint literal, so a default can be
	// written into the URL and overridden per module.
	t.Run("overrides a parameter spelled out in the endpoint", func(t *testing.T) {
		got, err := MergeQueryParams("https://api.example.com/v1/orders?status=closed", map[string]string{"status": "open"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertQuery(t, got, "status", "open")

		parsed, err := url.Parse(got)
		if err != nil {
			t.Fatalf("result does not parse: %v", err)
		}
		if values := parsed.Query()["status"]; len(values) != 1 {
			t.Errorf("status appears %d times, want exactly 1: %q", len(values), got)
		}
	})

	// AC5: values are encoded when the query string is serialized, so a value
	// carrying URL metacharacters cannot break out of its parameter.
	t.Run("encodes values carrying URL metacharacters", func(t *testing.T) {
		const value = "a&b=c?d#e f/g+h"

		got, err := MergeQueryParams("https://api.example.com/v1/orders", map[string]string{"q": value})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertQuery(t, got, "q", value)

		parsed, err := url.Parse(got)
		if err != nil {
			t.Fatalf("result does not parse: %v", err)
		}
		if len(parsed.Query()) != 1 {
			t.Errorf("value leaked into extra parameters: %v (%q)", parsed.Query(), got)
		}
	})

	t.Run("encodes non-ASCII values", func(t *testing.T) {
		const value = "café münchen"

		got, err := MergeQueryParams("https://api.example.com/v1/orders", map[string]string{"q": value})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertQuery(t, got, "q", value)
	})

	// AC7: adding a parameter must not reshape the ones already there. Decoding
	// the query into url.Values and re-encoding it returned `$filter` as
	// `%24filter`, spaces as `+`, and the parameters sorted (Story 25.5).
	t.Run("keeps the endpoint's own parameters verbatim", func(t *testing.T) {
		got, err := MergeQueryParams(
			"https://api.example.com/v1/orders?$filter=LastModifiedDate%20gt%202026-01-01&b=2&a=1",
			map[string]string{"$top": "200"},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		const wantPrefix = "https://api.example.com/v1/orders?$filter=LastModifiedDate%20gt%202026-01-01&b=2&a=1&"
		if !strings.HasPrefix(got, wantPrefix) {
			t.Errorf("got %q, want the original query preserved as %q", got, wantPrefix)
		}
		assertQuery(t, got, "$top", "200")
		assertQuery(t, got, "$filter", "LastModifiedDate gt 2026-01-01")
		// Asserted on the raw query, not through Query(): `%24top` decodes to
		// `$top` too, so only the raw form shows whether an added parameter is
		// spelled like the ones the endpoint already carries.
		if !strings.HasSuffix(got, "&$top=200") {
			t.Errorf("got %q, want the added parameter spelled $top, not %%24top", got)
		}
	})

	// A raw space in the endpoint is the OData case: it must leave encoded
	// whether or not the caller also goes through NormalizeURL.
	t.Run("encodes raw spaces the endpoint carried", func(t *testing.T) {
		got, err := MergeQueryParams(
			"https://api.example.com/v1/orders?$filter=LastModifiedDate gt 2026-01-01",
			map[string]string{"$top": "200"},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(got, " ") {
			t.Errorf("a raw space survived: %q", got)
		}
		assertQuery(t, got, "$filter", "LastModifiedDate gt 2026-01-01")
	})

	// %20, not +: `+` decodes to a space only under form-encoding rules, which
	// the source is not obliged to follow.
	t.Run("added values encode a space as %20", func(t *testing.T) {
		got, err := MergeQueryParams(
			"https://api.example.com/v1/orders",
			map[string]string{"$orderby": "LastModifiedDate asc"},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(got, "LastModifiedDate%20asc") {
			t.Errorf("got %q, want the space as %%20", got)
		}
		assertQuery(t, got, "$orderby", "LastModifiedDate asc")
	})

	t.Run("the same parameters always produce the same URL", func(t *testing.T) {
		params := map[string]string{"c": "3", "a": "1", "b": "2"}

		first, err := MergeQueryParams("https://api.example.com/v1/orders", params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for range 10 {
			got, err := MergeQueryParams("https://api.example.com/v1/orders", params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != first {
				t.Fatalf("got %q then %q — map order leaked into the URL", first, got)
			}
		}
	})

	t.Run("unparsable endpoint is an error", func(t *testing.T) {
		_, err := MergeQueryParams("://not a url", map[string]string{"a": "1"})
		if err == nil {
			t.Fatal("expected an error for an unparsable endpoint")
		}
	})
}

// assertQuery checks that the parameter decodes back to the value that went in —
// the only thing encoding is for.
func assertQuery(t *testing.T, rawURL, param, want string) {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("result does not parse: %v (%q)", err, rawURL)
	}
	if got := parsed.Query().Get(param); got != want {
		t.Errorf("%s = %q, want %q (url: %q)", param, got, want, rawURL)
	}
}

func TestNormalizeURL(t *testing.T) {
	t.Run("no space is returned verbatim", func(t *testing.T) {
		const endpoint = "https://api.example.com/v1/orders?b=2&a=1"

		got, err := NormalizeURL(endpoint)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != endpoint {
			t.Errorf("got %q, want the endpoint unchanged", got)
		}
	})

	// The motivating case: OData filters are written with spaces, and Go hands
	// them to the wire raw, so the server answers 400 on a request line it
	// cannot parse.
	t.Run("spaces in the query are percent-encoded", func(t *testing.T) {
		got, err := NormalizeURL("https://api.example.com/v1/orders?$filter=LastModifiedDate gt 2026-01-01T00:00:00Z")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if strings.Contains(got, " ") {
			t.Errorf("a raw space survived: %q", got)
		}
		parsed, err := url.Parse(got)
		if err != nil {
			t.Fatalf("result does not parse: %v", err)
		}
		const want = "LastModifiedDate gt 2026-01-01T00:00:00Z"
		if v := parsed.Query().Get("$filter"); v != want {
			t.Errorf("$filter = %q, want %q — the value must survive encoding", v, want)
		}
	})

	t.Run("an already-encoded space is not double-encoded", func(t *testing.T) {
		got, err := NormalizeURL("https://api.example.com/v1/orders?q=a%20b c")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		parsed, err := url.Parse(got)
		if err != nil {
			t.Fatalf("result does not parse: %v", err)
		}
		if v := parsed.Query().Get("q"); v != "a b c" {
			t.Errorf("q = %q, want %q", v, "a b c")
		}
	})

	t.Run("the query's own shape is preserved", func(t *testing.T) {
		got, err := NormalizeURL("https://api.example.com/v1?b=2&a=1&c=x y")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(got, "b=2&a=1") {
			t.Errorf("parameters were reordered or re-encoded: %q", got)
		}
	})
}
