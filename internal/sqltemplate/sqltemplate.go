// Package sqltemplate builds parameterized SQL queries from Jinja templates.
//
// The model separates the query *shape* from the query *values*:
//
//   - query: a Jinja template rendered to SQL text (TargetText, strict). It may
//     use {% if %} / {% for %} blocks to build optional clauses. Placeholders
//     $1, $2, ... appear literally in the text and refer to entries of the
//     parameters list (1-based).
//   - parameters: an ordered list of expr expressions evaluated against the
//     render context (record, meta, state, pagination). Each value is
//     bound to its placeholder natively typed (nil binds as NULL) — values are
//     never interpolated into the SQL text.
//
// After rendering, canonical $N placeholders are translated for the driver:
// postgres keeps $N (renumbered by first appearance, reusing one argument for
// repeated references), mysql/sqlite emit one ? and one argument per
// occurrence.
package sqltemplate

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/parser"
	"github.com/expr-lang/expr/vm"

	"github.com/cannectors/runtime/internal/database"
	"github.com/cannectors/runtime/internal/template"
)

// Query is a compiled SQL template with its parameter expressions. Safe for
// concurrent Build calls.
type Query struct {
	tmpl      *template.Compiled
	params    []*vm.Program
	paramSrcs []string
	driver    string
}

// sqlToken matches, left to right, a region where a `$N` is NOT a placeholder —
// a single-quoted string literal (with ” escapes), a line comment, a block
// comment, or a `$$…$$` dollar-quoted string — OR a `$N` placeholder (group 1).
// RE2 tries alternatives leftmost-first, so a `$N` inside a literal or comment
// is consumed by that alternative and never captured as a placeholder.
//
// Not handled: *tagged* dollar-quotes ($tag$…$tag$), a rare PostgreSQL construct
// whose matching tags would need a backreference RE2 lacks — a `$N` inside one
// would be misread. Acceptable: the query is author-written static text and such
// a construct in a pipeline query is exceptional.
var sqlToken = regexp.MustCompile(`(?s)'(?:[^']|'')*'|--[^\n]*|/\*.*?\*/|\$\$.*?\$\$|\$(\d+)`)

// paginationVar is the context variable a parameter expression must read for the
// module to hand it the LIMIT/OFFSET binding.
const paginationVar = "pagination"

// placeholderRef is a `$N` placeholder located in SQL text.
type placeholderRef struct {
	start, end int // byte range of the "$N" token
	n          int // the placeholder number
}

// scanPlaceholders returns the `$N` placeholders in sql, in order, skipping the
// literal and comment regions matched by sqlToken.
func scanPlaceholders(sql string) []placeholderRef {
	var refs []placeholderRef
	for _, m := range sqlToken.FindAllStringSubmatchIndex(sql, -1) {
		// m = [matchStart, matchEnd, group1Start, group1End]; group 1 is set
		// (>= 0) only when the match is a $N placeholder.
		if m[2] < 0 {
			continue
		}
		n, _ := strconv.Atoi(sql[m[2]:m[3]])
		refs = append(refs, placeholderRef{start: m[0], end: m[1], n: n})
	}
	return refs
}

// Compile parses the query template (Jinja, strict undefined) and the
// parameter expressions, and statically validates placeholder consistency:
// every $N in the template source must have a parameters[N-1], and every
// declared parameter must be referenced somewhere in the source.
func Compile(engine *template.Engine, querySrc string, paramSrcs []string, driver string) (*Query, error) {
	// Values must never be interpolated into the SQL text: an output tag would
	// render a raw, unescaped record value and open a SQL injection. Control
	// blocks ({% if %}/{% for %}) are still allowed to shape the query.
	if template.HasOutputTag(querySrc) {
		return nil, fmt.Errorf("SQL query must not interpolate values with {{ ... }}; bind dynamic values through $N placeholders and the parameters list")
	}

	tmpl, err := engine.Compile(querySrc, template.TargetText, true)
	if err != nil {
		return nil, fmt.Errorf("compiling query template: %w", err)
	}

	params := make([]*vm.Program, len(paramSrcs))
	for i, src := range paramSrcs {
		if strings.TrimSpace(src) == "" {
			return nil, fmt.Errorf("parameter %d: expression is empty", i+1)
		}
		prog, exprErr := expr.Compile(src, expr.AllowUndefinedVariables())
		if exprErr != nil {
			return nil, fmt.Errorf("parameter %d (%q): %w", i+1, src, exprErr)
		}
		params[i] = prog
	}

	if err := validatePlaceholders(querySrc, len(paramSrcs)); err != nil {
		return nil, err
	}

	return &Query{tmpl: tmpl, params: params, paramSrcs: paramSrcs, driver: driver}, nil
}

// validatePlaceholders checks $N references in the template source against the
// declared parameter count. Placeholders inside {% %} branches appear in the
// source, so conditional clauses are covered. $N inside SQL string/dollar-quote
// literals and comments are not placeholders and are skipped.
func validatePlaceholders(querySrc string, paramCount int) error {
	referenced := make(map[int]bool)
	for _, ref := range scanPlaceholders(querySrc) {
		if ref.n > paramCount {
			return fmt.Errorf("query references $%d but only %d parameter(s) are declared", ref.n, paramCount)
		}
		referenced[ref.n] = true
	}
	for i := 1; i <= paramCount; i++ {
		if !referenced[i] {
			return fmt.Errorf("parameter %d is declared but $%d never appears in the query", i, i)
		}
	}
	return nil
}

// UsesPagination reports whether any parameter expression references the
// pagination context. Input modules use this to decide between placeholder
// pagination (rebinding parameters per page) and literal LIMIT/OFFSET.
func (q *Query) UsesPagination() bool {
	return ReferencesPagination(q.paramSrcs)
}

// ReferencesPagination reports whether any of the given parameter expressions
// reads the pagination context. Exported so a module can enforce the
// requirement before opening a database connection.
func ReferencesPagination(srcs []string) bool {
	for _, src := range srcs {
		if referencesPagination(src) {
			return true
		}
	}
	return false
}

// Build renders the query for the given context and returns driver-ready SQL
// with its bound arguments. Only parameters whose placeholder survives
// rendering (e.g. inside a taken {% if %} branch) are evaluated and bound.
func (q *Query) Build(rc template.RenderContext) (string, []any, error) {
	sql, args, _, err := q.BuildDescribed(rc)
	return sql, args, err
}

// BuildDescribed renders the query like Build and also returns, for each bound
// argument, the parameter expression it came from.
//
// The sources cannot be read off the declared parameter list: a dropped {% if %}
// branch removes bindings, postgres renumbers by first appearance and reuses one
// argument for repeated references, and mysql/sqlite bind one argument per
// occurrence. Dry-run previews need the per-argument mapping to label the values
// they display, and a positional guess would label them wrong.
func (q *Query) BuildDescribed(rc template.RenderContext) (string, []any, []string, error) {
	rendered, err := q.tmpl.Render(rc)
	if err != nil {
		return "", nil, nil, fmt.Errorf("rendering query template: %w", err)
	}

	// HasOutputTag is enforced at Compile, so no {{ }} can reach here. A residual
	// delimiter would mean a bug in the tokenizer — fail closed rather than emit
	// a possibly-injectable query.
	if strings.Contains(rendered, "{{") || strings.Contains(rendered, "}}") {
		return "", nil, nil, fmt.Errorf("template delimiters remain in rendered query")
	}

	env := rc.Vars()

	// Evaluate lazily: a parameter is only evaluated when its placeholder is
	// present in the rendered text.
	values := make([]any, len(q.params))
	evaluated := make([]bool, len(q.params))
	evalParam := func(n int) (any, error) {
		idx := n - 1
		if !evaluated[idx] {
			v, runErr := vm.Run(q.params[idx], env)
			if runErr != nil {
				return nil, fmt.Errorf("evaluating parameter %d (%q): %w", n, q.paramSrcs[idx], runErr)
			}
			values[idx] = v
			evaluated[idx] = true
		}
		return values[idx], nil
	}

	refs := scanPlaceholders(rendered)
	postgres := database.GetPlaceholderStyle(q.driver) == database.PlaceholderDollar
	newIndex := make(map[int]int) // postgres: original $N -> renumbered index

	var out strings.Builder
	out.Grow(len(rendered))
	var args []any
	var sources []string
	last := 0

	for _, ref := range refs {
		if ref.n < 1 || ref.n > len(q.params) {
			return "", nil, nil, fmt.Errorf("rendered query references $%d but only %d parameter(s) are declared", ref.n, len(q.params))
		}
		out.WriteString(rendered[last:ref.start])
		last = ref.end

		if postgres {
			// Renumber by first appearance; reuse one argument for repeats.
			idx, seen := newIndex[ref.n]
			if !seen {
				v, evalErr := evalParam(ref.n)
				if evalErr != nil {
					return "", nil, nil, evalErr
				}
				args = append(args, v)
				sources = append(sources, q.paramSrcs[ref.n-1])
				idx = len(args)
				newIndex[ref.n] = idx
			}
			out.WriteString("$")
			out.WriteString(strconv.Itoa(idx))
		} else {
			// MySQL/SQLite: one ? and one argument per occurrence.
			v, evalErr := evalParam(ref.n)
			if evalErr != nil {
				return "", nil, nil, evalErr
			}
			args = append(args, v)
			sources = append(sources, q.paramSrcs[ref.n-1])
			out.WriteString("?")
		}
	}
	out.WriteString(rendered[last:])
	translated := out.String()
	return translated, args, sources, nil
}

// referencesPagination reports whether an expr expression reads the `pagination`
// context variable. It inspects the parsed AST rather than the source text, so a
// record field whose name merely contains "pagination" (record.paginationToken)
// is not mistaken for pagination usage — which would wrongly hand the
// LIMIT/OFFSET clause over to the author and make the paging loop repeat the
// same page.
func referencesPagination(src string) bool {
	tree, err := parser.Parse(src)
	if err != nil {
		// Unparseable expressions are rejected by Compile; be conservative here.
		return strings.Contains(src, paginationVar)
	}
	visitor := &identifierVisitor{name: paginationVar}
	ast.Walk(&tree.Node, visitor)
	return visitor.found
}

// identifierVisitor reports whether the walked tree references an identifier.
type identifierVisitor struct {
	name  string
	found bool
}

func (v *identifierVisitor) Visit(node *ast.Node) {
	if id, ok := (*node).(*ast.IdentifierNode); ok && id.Value == v.name {
		v.found = true
	}
}
