package soaputil

import (
	"testing"

	"github.com/cannectors/runtime/internal/moduleconfig"
)

func TestExtractIntField(t *testing.T) {
	data := map[string]any{
		"meta": map[string]any{
			"totalPages": float64(7),
			"totalInt":   int(42),
			"totalI64":   int64(99),
			"totalStr":   "12",
			"bad":        "not-a-number",
		},
	}
	cases := []struct {
		name  string
		field string
		want  int
	}{
		{"empty field returns zero", "", 0},
		{"missing field returns zero", "meta.unknown", 0},
		{"float64 path", "meta.totalPages", 7},
		{"int path", "meta.totalInt", 42},
		{"int64 path", "meta.totalI64", 99},
		{"numeric string path", "meta.totalStr", 12},
		{"unconvertible string returns zero", "meta.bad", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExtractIntField(data, tc.field); got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestExtractStringField(t *testing.T) {
	data := map[string]any{
		"meta": map[string]any{
			"next":   "cursor-abc",
			"empty":  "",
			"nullV":  nil,
			"number": float64(123),
		},
	}
	cases := []struct {
		name  string
		field string
		want  string
	}{
		{"empty field returns empty", "", ""},
		{"missing field returns empty", "meta.unknown", ""},
		{"nil value returns empty", "meta.nullV", ""},
		{"string path", "meta.next", "cursor-abc"},
		{"empty string path", "meta.empty", ""},
		{"non-string value is stringified", "meta.number", "123"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExtractStringField(data, tc.field); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// A query key must be appended to the endpoint's own query, not merged through
// url.Values: re-encoding it there reordered the parameters and rewrote the
// escaping the pipeline chose (Story 25.1 AC7).
func TestBuildEndpointWithKeysKeepsEndpointQueryShape(t *testing.T) {
	keys := []moduleconfig.KeyConfig{{ParamName: "tenant", ParamType: "query", Field: "tenant"}}

	got, err := BuildEndpointWithKeys(
		"https://soap.example.com/service?$filter=Status%20eq%20'open'&b=2&a=1",
		map[string]any{"tenant": "acme"},
		keys,
		map[string]string{"tenant": "acme"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const want = "https://soap.example.com/service?$filter=Status%20eq%20'open'&b=2&a=1&tenant=acme"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
