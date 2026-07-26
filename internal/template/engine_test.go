package template

import (
	"encoding/json"
	"testing"
)

func testCtx() RenderContext {
	return RenderContext{
		Record: map[string]any{
			"id":     42,
			"status": "paid",
			"name":   `Tom & "Jerry"`,
			"q":      "a b&c",
			"note":   "line1\nline2 \"x\"",
			"xmlish": `<tag>a&b</tag>`,
			"items": []any{
				map[string]any{"name": "first"},
				map[string]any{"name": "second"},
			},
			"tags": []any{"go", "rust"},
		},
		State: map[string]any{"lastRunTimestamp": "2026-01-01T00:00:00Z"},
	}
}

func render(t *testing.T, e *Engine, src string, target Target) string {
	t.Helper()
	c, err := e.Compile(src, target, false)
	if err != nil {
		t.Fatalf("compile %q: %v", src, err)
	}
	out, err := c.Render(testCtx())
	if err != nil {
		t.Fatalf("render %q: %v", src, err)
	}
	return out
}

func TestEngine_BasicSubstitutionAndNavigation(t *testing.T) {
	e := NewEngine()
	cases := map[string]string{
		`{{ record.id }}`:              "42",
		`{{ record.items[0].name }}`:   "first",
		`{{ record.items[1].name }}`:   "second",
		`{{ state.lastRunTimestamp }}`: "2026-01-01T00:00:00Z",
	}
	for src, want := range cases {
		if got := render(t, e, src, TargetText); got != want {
			t.Errorf("%s = %q, want %q", src, got, want)
		}
	}
}

func TestEngine_Filters(t *testing.T) {
	e := NewEngine()
	cases := map[string]string{
		`{{ record.status | upper }}`:           "PAID",
		`{{ record.status | capitalize }}`:      "Paid",
		`{{ record.missing | default("n/a") }}`: "n/a",
		`{{ record.tags | join(",") }}`:         "go,rust",
		`{{ record.tags | length }}`:            "2",
	}
	for src, want := range cases {
		if got := render(t, e, src, TargetText); got != want {
			t.Errorf("%s = %q, want %q", src, got, want)
		}
	}
}

func TestEngine_ControlBlocks(t *testing.T) {
	e := NewEngine()
	cases := map[string]string{
		`{% if record.status == "paid" %}OK{% else %}NO{% endif %}`: "OK",
		`{% if record.status == "void" %}OK{% else %}NO{% endif %}`: "NO",
		`{% for tag in record.tags %}<t>{{ tag }}</t>{% endfor %}`:  "<t>go</t><t>rust</t>",
	}
	for src, want := range cases {
		if got := render(t, e, src, TargetText); got != want {
			t.Errorf("%s = %q, want %q", src, got, want)
		}
	}
}

func TestEngine_URLTarget(t *testing.T) {
	e := NewEngine()
	got := render(t, e, `https://api/u/{{ record.id }}?q={{ record.q }}`, TargetURL)
	want := `https://api/u/42?q=a+b%26c`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestEngine_XMLTarget(t *testing.T) {
	e := NewEngine()
	got := render(t, e, `<o><c>{{ record.name }}</c><r>{{ record.xmlish }}</r></o>`, TargetXML)
	want := `<o><c>Tom &amp; &#34;Jerry&#34;</c><r>&lt;tag&gt;a&amp;b&lt;/tag&gt;</r></o>`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestEngine_JSONTarget_StaysValid(t *testing.T) {
	e := NewEngine()
	got := render(t, e, `{"name":"{{ record.name }}","note":"{{ record.note }}"}`, TargetJSON)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("produced invalid JSON %q: %v", got, err)
	}
	if parsed["name"] != `Tom & "Jerry"` || parsed["note"] != "line1\nline2 \"x\"" {
		t.Fatalf("json roundtrip wrong: %#v", parsed)
	}
}

func TestEngine_TextTargetNoEscaping(t *testing.T) {
	e := NewEngine()
	// In TargetText the raw value must pass through unescaped (e.g. SQL text).
	got := render(t, e, `name={{ record.name }}`, TargetText)
	if got != `name=Tom & "Jerry"` {
		t.Fatalf("text target escaped value: %q", got)
	}
}

func TestEngine_StrictUndefined(t *testing.T) {
	e := NewEngine()

	// Lenient: missing field renders empty.
	if got := render(t, e, `[{{ record.nope }}]`, TargetText); got != "[]" {
		t.Fatalf("lenient missing = %q, want %q", got, "[]")
	}

	// Strict: missing field is an error.
	c, err := e.Compile(`[{{ record.nope }}]`, TargetText, true)
	if err != nil {
		t.Fatalf("compile strict: %v", err)
	}
	if _, err := c.Render(testCtx()); err == nil {
		t.Fatalf("strict mode: expected error on missing field, got nil")
	}
}

// An escaping filter is auto-injected on every output tag; it must not swallow
// the strict-undefined error (a missing variable resolves to an error Value).
func TestEngine_StrictUndefined_WithEscapeFilter(t *testing.T) {
	e := NewEngine()
	for _, target := range []Target{TargetURL, TargetJSON, TargetXML} {
		c, err := e.Compile(`<a>{{ record.missing }}</a>`, target, true)
		if err != nil {
			t.Fatalf("compile (target %d): %v", target, err)
		}
		if _, err := c.Render(testCtx()); err == nil {
			t.Errorf("target %d: expected strict error on missing field through escape filter, got nil", target)
		}
	}
	// Lenient with an escape filter: missing field renders empty, no error.
	if got := render(t, e, `<a>{{ record.missing }}</a>`, TargetXML); got != "<a></a>" {
		t.Fatalf("lenient XML missing = %q, want %q", got, "<a></a>")
	}
}

func TestEngine_CompileCaches(t *testing.T) {
	e := NewEngine()
	a, err := e.Compile(`{{ record.id }}`, TargetURL, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.Compile(`{{ record.id }}`, TargetURL, false)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("expected cached *Compiled to be reused")
	}
	// Different target must not collide with the cached entry.
	if c, _ := e.Compile(`{{ record.id }}`, TargetText, false); c == a {
		t.Fatalf("cache key ignored target")
	}
}

func TestEngine_InvalidSyntaxFailsCompile(t *testing.T) {
	e := NewEngine()
	if err := e.Validate(`{{ record.id `, TargetText, false); err == nil {
		t.Fatalf("expected compile error for unterminated tag")
	}
	if err := e.Validate(`{% if x %}no end`, TargetText, false); err == nil {
		t.Fatalf("expected compile error for unclosed block")
	}
}

// A comment containing an apostrophe must not disable escaping of a later
// output tag (regression: findTagClose used to treat comments quote-aware,
// swallowing the rest of the template and skipping escaping — an XML/JSON/URL
// injection vector).
func TestEngine_CommentDoesNotBypassEscaping(t *testing.T) {
	e := NewEngine()
	got := render(t, e, `{# don't escape #}<x>{{ record.xmlish }}</x>`, TargetXML)
	want := `<x>&lt;tag&gt;a&amp;b&lt;/tag&gt;</x>`
	if got != want {
		t.Fatalf("comment apostrophe bypassed escaping:\n got=%q\nwant=%q", got, want)
	}
}

// {% raw %} content is emitted verbatim; the {{ }} inside must not be rewritten,
// while an output tag after the block is still escaped.
func TestEngine_RawBlockNotRewritten(t *testing.T) {
	e := NewEngine()
	got := render(t, e, `{% raw %}{{ literal }}{% endraw %}<y>{{ record.xmlish }}</y>`, TargetXML)
	want := `{{ literal }}<y>&lt;tag&gt;a&amp;b&lt;/tag&gt;</y>`
	if got != want {
		t.Fatalf("raw block handling:\n got=%q\nwant=%q", got, want)
	}
}

// Template loading tags are rejected: gonja would parse the loaded template
// itself, so its {{ }} tags would never receive an escape filter — and the path
// can come from record data (arbitrary file read, and SQL injection past the
// HasOutputTag guard, which only sees the outer source).
func TestEngine_TemplateLoadingBlocksRejected(t *testing.T) {
	e := NewEngine()
	for _, src := range []string{
		`{% include "/etc/hostname" %}`,
		`{% include record.path %}`,
		`{%- include "x.j2" -%}`,
		`{% extends "base.j2" %}`,
		`{% import "m.j2" as m %}`,
		`{% from "m.j2" import thing %}`,
		`{% for p in record.items %}{% include p %}{% endfor %}`,
	} {
		for _, target := range []Target{TargetText, TargetURL, TargetJSON, TargetXML} {
			if err := e.Validate(src, target, false); err == nil {
				t.Errorf("target %s: %q compiled, want rejection", target, src)
			}
		}
	}
	// A literal mention inside a comment or a raw block is not a tag.
	if err := e.Validate(`{# include "x" #}{% raw %}{% include "y" %}{% endraw %}`, TargetXML, false); err != nil {
		t.Errorf("comment/raw mention rejected: %v", err)
	}
}

// {% filter %} applies its filter after the injected escaping, corrupting the
// output (`&lt;` -> `&LT;`). It is rejected for escaping targets; inline filters
// are escaped last and stay allowed.
func TestEngine_FilterBlockRejectedForEscapedTargets(t *testing.T) {
	e := NewEngine()
	const src = `{% filter upper %}{{ record.xmlish }}{% endfilter %}`
	for _, target := range []Target{TargetURL, TargetJSON, TargetXML} {
		if err := e.Validate(src, target, false); err == nil {
			t.Errorf("target %s: {%% filter %%} compiled, want rejection", target)
		}
	}
	if err := e.Validate(src, TargetText, false); err != nil {
		t.Errorf("text target must allow {%% filter %%}: %v", err)
	}
	if got := render(t, e, `{{ record.xmlish | upper }}`, TargetXML); got != `&lt;TAG&gt;A&amp;B&lt;/TAG&gt;` {
		t.Errorf("inline filter escaping = %q", got)
	}
}

// Strict templates (SOAP bodies) must not silently emit an empty value for a
// field that is present but null — the pre-Jinja engine failed on it. An
// explicit default still covers it.
func TestEngine_StrictRejectsNullValue(t *testing.T) {
	e := NewEngine()
	ctx := RenderContext{Record: map[string]any{"x": nil}}
	for _, target := range []Target{TargetURL, TargetJSON, TargetXML} {
		c, compileErr := e.Compile(`<a>{{ record.x }}</a>`, target, true)
		if compileErr != nil {
			t.Fatalf("compile (target %s): %v", target, compileErr)
		}
		if _, err := c.Render(ctx); err == nil {
			t.Errorf("target %s: null value rendered, want error", target)
		}

		withDefault, compileErr := e.Compile(`<a>{{ record.x | default('na') }}</a>`, target, true)
		if compileErr != nil {
			t.Fatalf("compile default (target %s): %v", target, compileErr)
		}
		if got, err := withDefault.Render(ctx); err != nil || got != "<a>na</a>" {
			t.Errorf("target %s: default on null = %q, err=%v", target, got, err)
		}
	}
	// Lenient keeps rendering an empty value for a null field.
	c, err := e.Compile(`<a>{{ record.x }}</a>`, TargetXML, false)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := c.Render(ctx); err != nil || got != "<a></a>" {
		t.Errorf("lenient null = %q, err=%v", got, err)
	}
}

// Records come from JSON, where every number decodes to float64. gonja renders
// float64(42) as "42.0" and true as "True" (Python semantics), which would break
// URLs (/orders/42.0) and emit invalid JSON bodies. Values must keep the
// record's own semantics on every target, including TargetText (headers,
// cache keys), and must match what the `keys` mechanism produces via
// ValueToString.
func TestEngine_JSONNumbersAndBoolsKeepRecordSemantics(t *testing.T) {
	var record map[string]any
	if err := json.Unmarshal([]byte(`{"id": 42, "big": 1234567890123, "ratio": 1.5, "active": true}`), &record); err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"{{ record.id }}":     "42",
		"{{ record.big }}":    "1234567890123",
		"{{ record.ratio }}":  "1.5",
		"{{ record.active }}": "true",
	}
	for _, target := range []Target{TargetText, TargetURL, TargetJSON, TargetXML} {
		for src, expected := range want {
			c, err := NewEngine().Compile(src, target, false)
			if err != nil {
				t.Fatalf("compile %s (target %s): %v", src, target, err)
			}
			got, err := c.Render(RenderContext{Record: record})
			if err != nil {
				t.Fatalf("render %s (target %s): %v", src, target, err)
			}
			if got != expected {
				t.Errorf("target %s: %s = %q, want %q", target, src, got, expected)
			}
		}
	}

	// A JSON body substituting a number and a bool must stay valid JSON.
	c, err := NewEngine().Compile(`{"id": {{ record.id }}, "active": {{ record.active }}}`, TargetJSON, false)
	if err != nil {
		t.Fatal(err)
	}
	body, err := c.Render(RenderContext{Record: record})
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("rendered body %q is not valid JSON: %v", body, err)
	}
}

// The tag scanner treats `\` as a string escape while gonja's lexer rejects
// backslashes in string literals. The divergence must stay fail-closed (compile
// error), never "tag ends early and the rest renders unescaped".
func TestEngine_BackslashInStringLiteralFailsClosed(t *testing.T) {
	e := NewEngine()
	for _, src := range []string{`{{ "a\" }}{{ record.xmlish }}`, `{{ 'a\' }}{{ record.xmlish }}`} {
		if err := e.Validate(src, TargetXML, false); err == nil {
			t.Errorf("%q compiled; a backslash-induced tokenizer divergence must fail closed", src)
		}
	}
}

func TestHasOutputTag(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{`select $1`, false},
		{`select {{ record.x }}`, true},
		{`{% if x %}a{% endif %}`, false},       // control block, not an output tag
		{`{# {{ x }} #}select 1`, false},        // output tag inside a comment
		{`{% raw %}{{ x }}{% endraw %}`, false}, // output tag inside raw
		{`{# c #}{{ record.y }}`, true},         // real tag after a comment
	}
	for _, tc := range cases {
		if got := HasOutputTag(tc.in); got != tc.want {
			t.Errorf("HasOutputTag(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestInjectEscape_PreservesStructure(t *testing.T) {
	cases := []struct{ in, filter, want string }{
		{`a {{ x }} b`, filterURLEsc, `a {{ (x) | __cnx_urlesc }} b`},
		{`{{- x -}}`, filterXMLEsc, `{{- (x) | __cnx_xmlesc -}}`},
		{`{% if x %}{{ y }}{% endif %}`, filterJSONEsc, `{% if x %}{{ (y) | __cnx_jsonesc }}{% endif %}`},
		{`{# {{ y }} #}{{ z }}`, filterURLEsc, `{# {{ y }} #}{{ (z) | __cnx_urlesc }}`}, // comment body untouched
		{`{{ "a}}b" }}`, filterURLEsc, `{{ ("a}}b") | __cnx_urlesc }}`},                 // }} inside a string literal
		{`no tags here`, filterURLEsc, `no tags here`},
		{`{{ x }}`, "", `{{ x }}`}, // empty filter => unchanged
	}
	for _, tc := range cases {
		if got := injectEscape(tc.in, tc.filter); got != tc.want {
			t.Errorf("injectEscape(%q, %q) = %q, want %q", tc.in, tc.filter, got, tc.want)
		}
	}
}
