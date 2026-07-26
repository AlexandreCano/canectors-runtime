package input

import "testing"

func TestResolvePath(t *testing.T) {
	obj := map[string]any{
		"flat": "top",
		"meta": map[string]any{
			"next_cursor": "abc",
			"page": map[string]any{
				"total": float64(7),
			},
		},
		"scalar": float64(3),
	}
	tests := []struct {
		name  string
		path  string
		want  any
		found bool
	}{
		{"flat field", "flat", "top", true},
		{"nested one level", "meta.next_cursor", "abc", true},
		{"nested two levels", "meta.page.total", float64(7), true},
		{"missing leaf", "meta.missing", nil, false},
		{"missing root", "nope", nil, false},
		{"descend into scalar", "scalar.child", nil, false},
		{"empty path", "", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, found := resolvePath(obj, tc.path)
			if found != tc.found {
				t.Fatalf("found = %v, want %v", found, tc.found)
			}
			if found && got != tc.want {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestExtractStringField(t *testing.T) {
	obj := map[string]any{
		"str":    "token-1",
		"num":    float64(1002),
		"bignum": float64(128374645),
		"flt":    float64(3.5),
		"boolv":  true,
		"meta":   map[string]any{"next_cursor": "nested-token"},
		"obj":    map[string]any{"k": "v"},
	}
	tests := []struct {
		field string
		want  string
	}{
		{"str", "token-1"},
		{"num", "1002"},                      // numeric cursor coerced, no trailing .0
		{"bignum", "128374645"},              // large int still exact
		{"flt", "3.5"},                       // non-integer numeric preserved
		{"meta.next_cursor", "nested-token"}, // nested path resolved
		{"missing", ""},                      // absent → empty
		{"obj", ""},                          // non-scalar → empty
		{"", ""},                             // empty field → empty
		// A bool must stay empty: coercing `has_more: false` to "false" would
		// look like a valid cursor and keep the loop running to the page cap.
		{"boolv", ""},
	}
	for _, tc := range tests {
		if got := extractStringField(obj, tc.field); got != tc.want {
			t.Errorf("extractStringField(%q) = %q, want %q", tc.field, got, tc.want)
		}
	}
}

func TestExtractIntField(t *testing.T) {
	obj := map[string]any{
		"total": float64(6),
		"meta":  map[string]any{"total_pages": float64(3)},
		"str":   "nope",
	}
	tests := []struct {
		field string
		want  int
	}{
		{"total", 6},
		{"meta.total_pages", 3}, // nested path resolved
		{"missing", 0},
		{"str", 0}, // non-numeric → 0
		{"", 0},
	}
	for _, tc := range tests {
		if got := extractIntField(obj, tc.field); got != tc.want {
			t.Errorf("extractIntField(%q) = %d, want %d", tc.field, got, tc.want)
		}
	}
}
