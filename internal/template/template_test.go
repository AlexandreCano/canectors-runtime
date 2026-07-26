package template

import "testing"

func TestHasVariables(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"https://api/{{ record.id }}", true},
		{"{% if record.x %}yes{% endif %}", true}, // block-only templates must render
		{"{{ record.id }}{% for t in record.tags %}{{ t }}{% endfor %}", true},
		{"https://api/static", false},
		{"path/{paramName}/x", false}, // single-brace path keys are not templates
		{"lone {{ opening", false},    // no closing marker, no block
		{"", false},
	}
	for _, tc := range cases {
		if got := HasVariables(tc.in); got != tc.want {
			t.Errorf("HasVariables(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestValueToString(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"integral float", float64(42), "42"},
		{"decimal float", 1.5, "1.5"},
		{"int", 7, "7"},
		{"int64", int64(9), "9"},
		{"bool", true, "true"},
		{"slice fallback", []any{1, 2}, "[1 2]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValueToString(tc.in); got != tc.want {
				t.Errorf("ValueToString(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
