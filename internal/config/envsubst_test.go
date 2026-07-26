package config

import (
	"strings"
	"testing"
)

func TestSubstituteEnvVars_ResolvesStringsEverywhere(t *testing.T) {
	t.Setenv("LAB_TOKEN", "resolved-token")
	t.Setenv("LAB_BASE_URL", "https://api.example.test")

	data := map[string]any{
		"input": map[string]any{
			"endpoint": "${LAB_BASE_URL}/api/orders",
			"authentication": map[string]any{
				"credentials": map[string]any{"token": "${LAB_TOKEN}"},
			},
		},
		"filters": []any{
			map[string]any{"type": "set", "value": "${LAB_TOKEN}"},
		},
	}

	if err := SubstituteEnvVars(data); err != nil {
		t.Fatalf("SubstituteEnvVars() error = %v, want nil", err)
	}

	input, ok := data["input"].(map[string]any)
	if !ok {
		t.Fatalf("input node has unexpected type %T", data["input"])
	}
	if got := input["endpoint"]; got != "https://api.example.test/api/orders" {
		t.Errorf("endpoint = %v, want the reference expanded inside the string", got)
	}
	auth, ok := input["authentication"].(map[string]any)
	if !ok {
		t.Fatalf("authentication node has unexpected type %T", input["authentication"])
	}
	creds, ok := auth["credentials"].(map[string]any)
	if !ok {
		t.Fatalf("credentials node has unexpected type %T", auth["credentials"])
	}
	if got := creds["token"]; got != "resolved-token" {
		t.Errorf("token = %v, want %q", got, "resolved-token")
	}
	filters, ok := data["filters"].([]any)
	if !ok {
		t.Fatalf("filters node has unexpected type %T", data["filters"])
	}
	filter, ok := filters[0].(map[string]any)
	if !ok {
		t.Fatalf("filters[0] has unexpected type %T", filters[0])
	}
	if got := filter["value"]; got != "resolved-token" {
		t.Errorf("filters[0].value = %v, want the reference resolved inside slices too", got)
	}
}

func TestSubstituteEnvVars_MissingOrEmptyFailsFast(t *testing.T) {
	t.Setenv("LAB_EMPTY", "")

	cases := []struct {
		name     string
		data     map[string]any
		wantName string
	}{
		{
			"unset variable",
			map[string]any{"token": "${LAB_DEFINITELY_UNSET}"},
			"LAB_DEFINITELY_UNSET",
		},
		{
			"empty variable is treated as missing",
			map[string]any{"token": "${LAB_EMPTY}"},
			"LAB_EMPTY",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := SubstituteEnvVars(tc.data)
			if err == nil {
				t.Fatal("SubstituteEnvVars() error = nil, want an error")
			}
			if !strings.Contains(err.Error(), tc.wantName) {
				t.Errorf("error %q should name the offending variable %q", err, tc.wantName)
			}
		})
	}
}

func TestSubstituteEnvVars_ReportsEveryMissingVariableOnce(t *testing.T) {
	data := map[string]any{
		"a": "${LAB_MISSING_ONE}",
		"b": "${LAB_MISSING_TWO}",
		"c": "${LAB_MISSING_ONE}",
	}
	err := SubstituteEnvVars(data)
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	for _, name := range []string{"LAB_MISSING_ONE", "LAB_MISSING_TWO"} {
		if !strings.Contains(msg, name) {
			t.Errorf("error %q should mention %q", msg, name)
		}
	}
	if strings.Count(msg, "LAB_MISSING_ONE") != 1 {
		t.Errorf("error %q should list LAB_MISSING_ONE once, not per occurrence", msg)
	}
}

func TestSubstituteEnvVars_LeavesNonEnvSyntaxAlone(t *testing.T) {
	t.Setenv("LAB_TOKEN", "resolved-token")

	data := map[string]any{
		// record templates use a different syntax and must survive untouched
		"body":     "{\"id\": \"{{record.id}}\"}",
		"lower":    "${lowercase_not_matched}",
		"digit":    "${1BAD}",
		"attempts": 3,
		"enabled":  true,
		"nothing":  nil,
		// resolved elsewhere by the database layer, must stay literal
		"connectionStringRef": "${LAB_TOKEN}",
	}
	if err := SubstituteEnvVars(data); err != nil {
		t.Fatalf("SubstituteEnvVars() error = %v, want nil", err)
	}
	if got := data["body"]; got != "{\"id\": \"{{record.id}}\"}" {
		t.Errorf("record template was modified: %v", got)
	}
	if got := data["lower"]; got != "${lowercase_not_matched}" {
		t.Errorf("lowercase reference should not match: %v", got)
	}
	if got := data["digit"]; got != "${1BAD}" {
		t.Errorf("name starting with a digit should not match: %v", got)
	}
	if got := data["attempts"]; got != 3 {
		t.Errorf("number was altered: %v", got)
	}
	if got := data["enabled"]; got != true {
		t.Errorf("boolean was altered: %v", got)
	}
	if got := data["connectionStringRef"]; got != "${LAB_TOKEN}" {
		t.Errorf("connectionStringRef = %v, want the literal reference preserved", got)
	}
}
