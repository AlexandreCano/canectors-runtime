package sqltemplate

import (
	"testing"

	"github.com/cannectors/runtime/internal/template"
)

// A query template mixes Jinja blocks with $N placeholders and a separate list
// of parameter expressions, and the compiler has to keep those two in step for
// three placeholder dialects. Malformed input must surface as an error: a panic
// here would take down the pipeline that loaded the config.

func FuzzCompile(f *testing.F) {
	f.Add("SELECT 1", "")
	f.Add("SELECT * FROM t WHERE id = $1", "record.id")
	f.Add("SELECT $1, $2, $1", "record.a")
	f.Add("{% if record.flag %}SELECT 1{% else %}SELECT 2{% endif %}", "")
	f.Add("SELECT $0", "record.a")
	f.Add("SELECT $99", "record.a")
	f.Add("{% for x in record.items %}$1{% endfor %}", "record.id")
	f.Add("SELECT '$1'", "record.a")
	f.Add("{{ record.raw }}", "")
	f.Add("{% unclosed", "")

	engine := template.NewEngine()

	f.Fuzz(func(t *testing.T, query, param string) {
		var params []string
		if param != "" {
			params = []string{param}
		}
		for _, driver := range []string{"postgres", "mysql", "sqlite"} {
			compiled, err := Compile(engine, query, params, driver)
			if err != nil {
				continue
			}
			if compiled == nil {
				t.Fatalf("Compile(%q, %v, %q) returned nil without an error", query, params, driver)
			}
			// Building must survive an arbitrary record too: the parameter
			// expressions are evaluated against it.
			_, _, _ = compiled.Build(template.RenderContext{
				Record: map[string]any{"id": 1, "a": "x", "flag": true, "items": []any{1, 2}},
			})
		}
	})
}
