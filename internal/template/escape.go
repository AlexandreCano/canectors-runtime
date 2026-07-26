package template

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"net/url"
	"sync"

	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/nikolalohinski/gonja/v2/logging"
)

// gonja's built-in autoescape is HTML-only and not replaceable through the
// public API. We instead register dedicated escaping filters and inject the
// right one onto every output tag at compile time (see injectEscape). The
// `__cnx_` prefix keeps these internal filters out of the user-facing namespace.
const (
	filterURLEsc  = "__cnx_urlesc"
	filterXMLEsc  = "__cnx_xmlesc"
	filterJSONEsc = "__cnx_jsonesc"
	// filterTextEsc applies no escaping. It is still injected on every output tag
	// of a TargetText template so value stringification stays consistent with the
	// escaped targets (see renderValue).
	filterTextEsc = "__cnx_text"
	// Strict variants additionally reject a null value: strict templates (SOAP
	// bodies) must not silently emit an empty element for a field that is
	// present but null. `| default('…')` still covers it, since gonja's default
	// filter substitutes on nil.
	filterURLEscStrict  = "__cnx_urlesc_strict"
	filterXMLEscStrict  = "__cnx_xmlesc_strict"
	filterJSONEscStrict = "__cnx_jsonesc_strict"
	filterTextEscStrict = "__cnx_text_strict"
)

var registerFiltersOnce sync.Once

// registerFilters installs the escaping filters on gonja's default environment.
// Idempotent and safe to call from every NewEngine.
func registerFilters() {
	registerFiltersOnce.Do(func() {
		// gonja logs through logrus; silence it so templating never spams stderr.
		logging.SetEnabled(false)
		fs := gonja.DefaultEnvironment.Filters
		_ = fs.Register(filterURLEsc, escapeFilter(urlEscape, false))
		_ = fs.Register(filterXMLEsc, escapeFilter(xmlEscape, false))
		_ = fs.Register(filterJSONEsc, escapeFilter(jsonEscape, false))
		_ = fs.Register(filterTextEsc, escapeFilter(noEscape, false))
		_ = fs.Register(filterURLEscStrict, escapeFilter(urlEscape, true))
		_ = fs.Register(filterXMLEscStrict, escapeFilter(xmlEscape, true))
		_ = fs.Register(filterJSONEscStrict, escapeFilter(jsonEscape, true))
		_ = fs.Register(filterTextEscStrict, escapeFilter(noEscape, true))
	})
}

// escapeFilter builds a gonja filter applying escape to the incoming value.
//
// Every filter returns AsSafeValue so gonja's HTML autoescape (if ever enabled)
// cannot double-escape an already-escaped value. They also propagate an error
// input unchanged: in strict mode a missing variable resolves to an error
// Value, and stringifying it here would silently swallow the strict error.
func escapeFilter(escape func(string) string, rejectNil bool) exec.FilterFunction {
	return func(_ *exec.Evaluator, in *exec.Value, _ *exec.VarArgs) *exec.Value {
		if in.IsError() {
			return in
		}
		if rejectNil && in.IsNil() {
			return exec.AsValue(errors.New("template value is null and has no default (add | default('…') to allow it)"))
		}
		return exec.AsSafeValue(escape(renderValue(in)))
	}
}

// renderValue converts a substituted value to text with the record's own
// (JSON/Go) semantics rather than gonja's Python-flavored ones.
//
// Records decoded from JSON hold every number as a float64, and gonja renders
// float64(42) as "42.0" and true as "True" — which silently breaks URLs
// (/orders/42.0 instead of /orders/42) and produces invalid JSON bodies. These
// are the types where the two engines disagree; every other type (time.Time,
// dicts, lists…) keeps gonja's formatting.
func renderValue(in *exec.Value) string {
	switch v := in.ToGoSimpleType(false).(type) {
	case float64, int, int64, bool:
		return ValueToString(v)
	default:
		return in.String()
	}
}

func noEscape(s string) string { return s }

func urlEscape(s string) string { return url.QueryEscape(s) }

func xmlEscape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// jsonEscape escapes a value for embedding INSIDE a JSON string literal: it
// marshals the string and strips the wrapping quotes json.Marshal adds.
func jsonEscape(s string) string {
	enc, err := json.Marshal(s)
	if err != nil || len(enc) < 2 {
		return ""
	}
	return string(enc[1 : len(enc)-1])
}

// escapeFilterFor returns the filter name to inject for a target. Strict
// templates get the null-rejecting variant. TargetText gets a no-op escaping
// filter rather than none at all, so value stringification (renderValue) is
// identical on every target.
func escapeFilterFor(t Target, strict bool) string {
	switch t {
	case TargetURL:
		if strict {
			return filterURLEscStrict
		}
		return filterURLEsc
	case TargetXML:
		if strict {
			return filterXMLEscStrict
		}
		return filterXMLEsc
	case TargetJSON:
		if strict {
			return filterJSONEscStrict
		}
		return filterJSONEsc
	default:
		if strict {
			return filterTextEscStrict
		}
		return filterTextEsc
	}
}

// escapes reports whether a target actually escapes substituted values. Used to
// gate the checks that only matter when escaping is injected.
func (t Target) escapes() bool { return t != TargetText }
