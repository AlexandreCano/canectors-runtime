package template

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sync"

	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/config"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/nikolalohinski/gonja/v2/loaders"
)

// Target selects the contextual escaping applied to values substituted into a
// template. The escaping is injected automatically at compile time, so authors
// never spell it out: an endpoint template escapes for URLs, a SOAP body for
// XML, a JSON body for JSON, and so on.
type Target int

const (
	// TargetText applies no escaping. Use for cache keys, log lines, and SQL
	// query text (where safety comes from bound parameters, not escaping).
	TargetText Target = iota
	// TargetURL escapes substituted values with url.QueryEscape.
	TargetURL
	// TargetJSON escapes substituted values for embedding inside JSON strings.
	TargetJSON
	// TargetXML escapes substituted values with xml.EscapeText (SOAP bodies).
	TargetXML
)

// String returns the target name used in error messages.
func (t Target) String() string {
	switch t {
	case TargetURL:
		return "URL"
	case TargetJSON:
		return "JSON"
	case TargetXML:
		return "XML"
	default:
		return "text"
	}
}

// Compiled is a parsed, ready-to-render template. It is safe for concurrent
// renders: gonja builds a fresh renderer per execution.
type Compiled struct {
	tmpl   *exec.Template
	source string
	target Target
}

// Source returns the original (pre-rewrite) template text.
func (c *Compiled) Source() string { return c.source }

// Target returns the escaping target the template was compiled for.
func (c *Compiled) Target() Target { return c.target }

// Engine compiles and caches Jinja templates with contextual escaping. It is
// safe for concurrent use. Create one with NewEngine and share it; compiled
// templates are cached so repeated Compile calls for the same input are cheap.
type Engine struct {
	cache sync.Map // string key -> compileResult
}

// NewEngine returns a ready-to-use Engine. It also registers the internal
// escaping filters on first use (idempotent).
func NewEngine() *Engine {
	registerFilters()
	return &Engine{}
}

type compileResult struct {
	compiled *Compiled
	err      error
}

// Compile parses source for the given target. When strict is true, referencing
// a missing variable or attribute at render time is an error; when false it
// renders as an empty string. Results (including compile errors) are cached by
// (source, target, strict).
func (e *Engine) Compile(source string, target Target, strict bool) (*Compiled, error) {
	key := cacheKey(source, target, strict)
	if v, ok := e.cache.Load(key); ok {
		r, _ := v.(compileResult)
		return r.compiled, r.err
	}
	c, err := compile(source, target, strict)
	e.cache.Store(key, compileResult{compiled: c, err: err})
	return c, err
}

// Validate reports whether source compiles for the given target/strictness.
// It is a pure syntax/parse check — no record data is required.
func (e *Engine) Validate(source string, target Target, strict bool) error {
	_, err := e.Compile(source, target, strict)
	return err
}

// Render evaluates a compiled template against ctx and returns the result.
func (c *Compiled) Render(ctx RenderContext) (string, error) {
	out, err := c.tmpl.ExecuteToString(exec.NewContext(ctx.Vars()))
	if err != nil {
		return "", fmt.Errorf("rendering template: %w", err)
	}
	return out, nil
}

func compile(source string, target Target, strict bool) (*Compiled, error) {
	if err := checkBlocks(source, target); err != nil {
		return nil, err
	}

	rewritten := injectEscape(source, escapeFilterFor(target, strict))

	// Fresh config per compile so the {% autoescape %} control structure (which
	// mutates Config in place) can never leak across templates.
	cfg := config.New()
	cfg.StrictUndefined = strict

	id := "cnx-" + hashSource(rewritten)
	loader, err := loaders.NewShiftedLoader(id, bytes.NewReader([]byte(rewritten)), denyLoader{})
	if err != nil {
		return nil, fmt.Errorf("template loader: %w", err)
	}

	tmpl, err := exec.NewTemplate(id, cfg, loader, gonja.DefaultEnvironment)
	if err != nil {
		return nil, fmt.Errorf("compiling template: %w", err)
	}
	return &Compiled{tmpl: tmpl, source: source, target: target}, nil
}

// denyLoader refuses every template lookup. A template pulled in by gonja
// ({% include %}, {% extends %}, {% import %}) would be parsed straight from
// disk, so its own {{ }} tags would never go through the compile-time escape
// rewrite — and its path can be an expression fed by record data. Templates are
// always handed to the engine as one self-contained source string; the
// forbidden-block check in checkBlocks reports these tags with a clear message
// and this loader is the fail-closed backstop behind it.
type denyLoader struct{}

func (denyLoader) Read(path string) (io.Reader, error) {
	return nil, fmt.Errorf("loading template %q is not supported", path)
}

func (denyLoader) Resolve(path string) (string, error) {
	return "", fmt.Errorf("resolving template %q is not supported", path)
}

func (d denyLoader) Inherit(string) (loaders.Loader, error) { return d, nil }

func cacheKey(source string, target Target, strict bool) string {
	return fmt.Sprintf("%d:%t:%s", target, strict, source)
}

func hashSource(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}
