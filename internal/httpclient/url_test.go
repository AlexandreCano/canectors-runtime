package httpclient

import (
	"net/url"
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
