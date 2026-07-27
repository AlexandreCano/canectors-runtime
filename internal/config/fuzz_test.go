package config

import (
	"testing"
)

// Configuration files are the runtime's main untrusted input: a pipeline can be
// written by hand, generated, or pasted from somewhere. The parser and the
// template validator must reject anything malformed with an error, never with a
// panic — a crash on a bad config is a denial of service on whatever schedules
// the run.
//
// These targets assert liveness, not correctness: the contract is "returns
// something, does not panic, does not hang".

func FuzzParseConfigString(f *testing.F) {
	f.Add("")
	f.Add("{}")
	f.Add("name: x\ninput: {}\nfilters: []\noutput: {}")
	f.Add(`{"name":"x","input":{"type":"httpPolling"},"filters":[],"output":{}}`)
	// Shapes that historically trip parsers: deep nesting, anchors, tabs,
	// duplicated keys, and a document that is valid YAML but not a mapping.
	f.Add("a: &x\n  b: *x")
	f.Add("- just\n- a\n- list")
	f.Add("\t\tname: tabs")
	f.Add("name: a\nname: b")
	f.Add("name: \"\\uFFFF\\u0000\"")

	f.Fuzz(func(t *testing.T, content string) {
		for _, format := range []string{"", "yaml", "json"} {
			result := ParseConfigString(content, format)
			if result == nil {
				t.Fatalf("ParseConfigString(%q, %q) returned nil", content, format)
			}
			// Parsed data must be safe to hand to the next stage even when the
			// input was garbage, so run the validators over whatever came back.
			if result.Data != nil {
				_ = ValidateConfig(result.Data)
				_ = ValidateTemplates(result.Data)
				_ = SubstituteEnvVars(result.Data)
			}
		}
	})
}

// SubstituteEnvVars walks arbitrary parsed structures. It must not panic on
// unusual shapes: deeply nested maps, slices of slices, nil values, or values
// that merely look like references.
func FuzzSubstituteEnvVars(f *testing.F) {
	f.Add("plain")
	f.Add("${UNSET_VARIABLE_NAME}")
	f.Add("${}")
	f.Add("${nested${inner}}")
	f.Add("{{ record.x }}")
	f.Add("$")
	f.Add("${A}${B}${C}")

	f.Fuzz(func(t *testing.T, value string) {
		data := map[string]any{
			"flat": value,
			"nested": map[string]any{
				"deep":  map[string]any{"leaf": value},
				"slice": []any{value, 1, true, nil, map[string]any{"k": value}},
			},
			"number": 42,
			"null":   nil,
		}
		// Missing variables are an error, not a panic; either outcome is fine
		// here as long as the walk survives.
		_ = SubstituteEnvVars(data)
	})
}
