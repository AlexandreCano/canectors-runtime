package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateTemplates_Valid(t *testing.T) {
	data := map[string]any{
		"input": map[string]any{
			"type":     "httpPolling",
			"endpoint": "https://api.example.com/orders/{{ record.id }}",
		},
		"filters": []any{
			map[string]any{
				"type":       "sql_call",
				"query":      "SELECT * FROM t WHERE id = $1{% if record.status %} AND status = $2{% endif %}",
				"parameters": []any{"record.id", "record.status"},
				"cache":      map[string]any{"key": "customer:{{record.id}}"},
			},
		},
		"output": map[string]any{
			"type": "httpRequest",
			"body": `{"name":"{{ record.name | default('n/a') }}"}`,
		},
	}
	if errs := ValidateTemplates(data); len(errs) != 0 {
		t.Fatalf("expected no errors, got %+v", errs)
	}
}

func TestValidateTemplates_InvalidJinjaSyntax(t *testing.T) {
	data := map[string]any{
		"output": map[string]any{
			"type":     "httpRequest",
			"endpoint": "https://api.example.com/{{ record.id",
		},
	}
	errs := ValidateTemplates(data)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %+v", errs)
	}
	if errs[0].Path != "/output/endpoint" || errs[0].Type != "template" {
		t.Fatalf("error = %+v, want template error at /output/endpoint", errs[0])
	}
}

func TestValidateTemplates_UnclosedBlock(t *testing.T) {
	data := map[string]any{
		"input": map[string]any{
			"type": "soapPolling",
			"body": "<a>{% if record.x %}yes</a>",
		},
	}
	errs := ValidateTemplates(data)
	if len(errs) != 1 || errs[0].Path != "/input/body" {
		t.Fatalf("expected 1 error at /input/body, got %+v", errs)
	}
}

func TestValidateTemplates_SQLPlaceholderConsistency(t *testing.T) {
	cases := []struct {
		name    string
		module  map[string]any
		wantSub string
	}{
		{
			name: "placeholder beyond declared parameters",
			module: map[string]any{
				"type":       "sql_call",
				"query":      "SELECT * FROM t WHERE a = $1 AND b = $2",
				"parameters": []any{"record.a"},
			},
			wantSub: "$2",
		},
		{
			name: "unreferenced parameter",
			module: map[string]any{
				"type":       "database",
				"query":      "SELECT * FROM t WHERE a = $1",
				"parameters": []any{"record.a", "record.b"},
			},
			wantSub: "never appears",
		},
		{
			name: "invalid parameter expression",
			module: map[string]any{
				"type":       "database",
				"query":      "SELECT * FROM t WHERE a = $1",
				"parameters": []any{"record.a +"},
			},
			wantSub: "parameter 1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := map[string]any{"filters": []any{tc.module}}
			errs := ValidateTemplates(data)
			if len(errs) != 1 {
				t.Fatalf("expected 1 error, got %+v", errs)
			}
			if errs[0].Type != "sql-template" || !strings.Contains(errs[0].Message, tc.wantSub) {
				t.Fatalf("error = %+v, want sql-template mentioning %q", errs[0], tc.wantSub)
			}
		})
	}
}

// An external query file must go through the same $N/parameters check, and an
// unreadable one must be reported instead of silently skipping the check (the
// module resolves the path the same way, so `run` would fail too).
func TestValidateTemplates_QueryFile(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.sql")
	if err := os.WriteFile(good, []byte("SELECT * FROM t WHERE a = $1"), 0o600); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "bad.sql")
	if err := os.WriteFile(bad, []byte("SELECT * FROM t WHERE a = $1 AND b = $2"), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name      string
		module    map[string]any
		wantErrs  int
		wantSub   string
		wantPath  string
		wantTypeQ bool
	}{
		{
			name:     "consistent query file",
			module:   map[string]any{"type": "sql_call", "queryFile": good, "parameters": []any{"record.a"}},
			wantErrs: 0,
		},
		{
			name:     "query file with orphan placeholder",
			module:   map[string]any{"type": "sql_call", "queryFile": bad, "parameters": []any{"record.a"}},
			wantErrs: 1,
			wantSub:  "$2",
		},
		{
			name:     "unreadable query file",
			module:   map[string]any{"type": "database", "queryFile": filepath.Join(dir, "nope.sql")},
			wantErrs: 1,
			wantSub:  "cannot read query file",
			wantPath: "/output/queryFile",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := ValidateTemplates(map[string]any{"output": tc.module})
			if len(errs) != tc.wantErrs {
				t.Fatalf("expected %d error(s), got %+v", tc.wantErrs, errs)
			}
			if tc.wantErrs == 0 {
				return
			}
			if !strings.Contains(errs[0].Message, tc.wantSub) {
				t.Fatalf("error = %+v, want message mentioning %q", errs[0], tc.wantSub)
			}
			if tc.wantPath != "" && errs[0].Path != tc.wantPath {
				t.Fatalf("error path = %q, want %q", errs[0].Path, tc.wantPath)
			}
		})
	}
}

func TestValidateTemplates_IgnoresNonTemplateStrings(t *testing.T) {
	data := map[string]any{
		"input": map[string]any{
			"type": "httpPolling",
			// Path-key placeholders (single braces) are not Jinja templates.
			"endpoint": "https://api.example.com/projects/{projectId}/tasks",
		},
		"output": map[string]any{
			"type":  "database",
			"query": "SELECT 1", // no placeholders, no parameters: fine
		},
	}
	if errs := ValidateTemplates(data); len(errs) != 0 {
		t.Fatalf("expected no errors, got %+v", errs)
	}
}
