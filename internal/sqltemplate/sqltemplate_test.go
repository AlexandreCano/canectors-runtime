package sqltemplate

import (
	"reflect"
	"strings"
	"testing"

	"github.com/cannectors/runtime/internal/template"
)

var engine = template.NewEngine()

func ctxWithRecord(record map[string]any) template.RenderContext {
	return template.RenderContext{Record: record}
}

func TestBuild_Postgres_Basic(t *testing.T) {
	q, err := Compile(engine,
		`select * from orders where customer_id = $1 and status = $2`,
		[]string{"record.customer.id", "record.status"}, "postgres")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	sql, args, err := q.Build(ctxWithRecord(map[string]any{
		"customer": map[string]any{"id": 42}, "status": "paid",
	}))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if sql != `select * from orders where customer_id = $1 and status = $2` {
		t.Errorf("sql = %q", sql)
	}
	if !reflect.DeepEqual(args, []any{42, "paid"}) {
		t.Errorf("args = %#v", args)
	}
}

func TestBuild_MySQL_QuestionMarks(t *testing.T) {
	q, err := Compile(engine,
		`select * from t where a = $1 and b = $2 and a2 = $1`,
		[]string{"record.a", "record.b"}, "mysql")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	sql, args, err := q.Build(ctxWithRecord(map[string]any{"a": 1, "b": 2}))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if sql != `select * from t where a = ? and b = ? and a2 = ?` {
		t.Errorf("sql = %q", sql)
	}
	// $1 referenced twice: the value is re-emitted per occurrence.
	if !reflect.DeepEqual(args, []any{1, 2, 1}) {
		t.Errorf("args = %#v", args)
	}
}

func TestBuild_Postgres_ReusesRepeatedPlaceholder(t *testing.T) {
	q, err := Compile(engine,
		`select * from t where a = $2 or b = $1 or c = $2`,
		[]string{"record.one", "record.two"}, "postgres")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	sql, args, err := q.Build(ctxWithRecord(map[string]any{"one": "I", "two": "II"}))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	// Renumbered by first appearance: $2 -> $1, $1 -> $2, second $2 reuses $1.
	if sql != `select * from t where a = $1 or b = $2 or c = $1` {
		t.Errorf("sql = %q", sql)
	}
	if !reflect.DeepEqual(args, []any{"II", "I"}) {
		t.Errorf("args = %#v", args)
	}
}

func TestBuild_ConditionalClause(t *testing.T) {
	src := `select * from orders where customer_id = $1{% if record.status %} and status = $2{% endif %}`
	q, err := Compile(engine, src, []string{"record.id", "record.status"}, "postgres")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	// Branch taken: both parameters bound.
	sql, args, err := q.Build(ctxWithRecord(map[string]any{"id": 7, "status": "paid"}))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.Contains(sql, "status = $2") || !reflect.DeepEqual(args, []any{7, "paid"}) {
		t.Errorf("taken branch: sql=%q args=%#v", sql, args)
	}

	// Branch skipped: $2 disappears, only $1 is bound.
	sql, args, err = q.Build(ctxWithRecord(map[string]any{"id": 7, "status": ""}))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if strings.Contains(sql, "$2") || !reflect.DeepEqual(args, []any{7}) {
		t.Errorf("skipped branch: sql=%q args=%#v", sql, args)
	}
}

func TestBuildDescribed_SourcesFollowBoundArguments(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		params      []string
		driver      string
		record      map[string]any
		wantSQL     string
		wantArgs    []any
		wantSources []string
	}{
		{
			// Postgres renumbers by first appearance: a dropped clause shifts the
			// remaining bindings, so the declared order would mislabel them.
			name:        "postgres dropped clause",
			query:       `update t set a = $1{% if record.withB %}, b = $2{% endif %} where id = $3`,
			params:      []string{"record.a", "record.b", "record.id"},
			driver:      "postgres",
			record:      map[string]any{"a": "A", "b": "B", "id": 7, "withB": false},
			wantSQL:     `update t set a = $1 where id = $2`,
			wantArgs:    []any{"A", 7},
			wantSources: []string{"record.a", "record.id"},
		},
		{
			// Postgres reuses one argument for a repeated reference, and honors
			// the order of first appearance rather than the declared order.
			name:        "postgres reordered and repeated",
			query:       `select * from t where a = $2 or b = $1 or c = $2`,
			params:      []string{"record.a", "record.b"},
			driver:      "postgres",
			record:      map[string]any{"a": 1, "b": 2},
			wantSQL:     `select * from t where a = $1 or b = $2 or c = $1`,
			wantArgs:    []any{2, 1},
			wantSources: []string{"record.b", "record.a"},
		},
		{
			// MySQL binds one argument per occurrence, so a repeated placeholder
			// is named as many times as it binds.
			name:        "mysql repeated placeholder",
			query:       `insert into t (a) values ($1) on duplicate key update a = $1`,
			params:      []string{"record.a"},
			driver:      "mysql",
			record:      map[string]any{"a": 1},
			wantSQL:     `insert into t (a) values (?) on duplicate key update a = ?`,
			wantArgs:    []any{1, 1},
			wantSources: []string{"record.a", "record.a"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q, err := Compile(engine, tc.query, tc.params, tc.driver)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			sql, args, sources, err := q.BuildDescribed(ctxWithRecord(tc.record))
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			if sql != tc.wantSQL {
				t.Errorf("sql = %q, want %q", sql, tc.wantSQL)
			}
			if !reflect.DeepEqual(args, tc.wantArgs) {
				t.Errorf("args = %#v, want %#v", args, tc.wantArgs)
			}
			if !reflect.DeepEqual(sources, tc.wantSources) {
				t.Errorf("sources = %#v, want %#v", sources, tc.wantSources)
			}
			if len(sources) != len(args) {
				t.Errorf("%d sources for %d args", len(sources), len(args))
			}
		})
	}
}

func TestBuild_NilBindsAsNull(t *testing.T) {
	q, err := Compile(engine, `update t set v = $1 where id = $2`,
		[]string{"record.missing", "record.id"}, "postgres")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	_, args, err := q.Build(ctxWithRecord(map[string]any{"id": 1}))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if args[0] != nil {
		t.Errorf("expected nil (NULL) for missing field, got %#v", args[0])
	}
	if args[1] != 1 {
		t.Errorf("args[1] = %#v", args[1])
	}
}

func TestBuild_TypesPreserved(t *testing.T) {
	q, err := Compile(engine, `insert into t values ($1, $2, $3)`,
		[]string{"record.n", "record.f", "record.b"}, "postgres")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	_, args, err := q.Build(ctxWithRecord(map[string]any{"n": 42, "f": 1.5, "b": true}))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !reflect.DeepEqual(args, []any{42, 1.5, true}) {
		t.Errorf("types not preserved: %#v", args)
	}
}

func TestBuild_StateAndPaginationContext(t *testing.T) {
	q, err := Compile(engine, `select * from t where ts > $1 and id > $2 limit $3`,
		[]string{"state.lastTimestamp", "pagination.cursor ?? 0", "pagination.limit"}, "postgres")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if !q.UsesPagination() {
		t.Fatalf("UsesPagination should be true")
	}
	_, args, err := q.Build(template.RenderContext{
		State:      map[string]any{"lastTimestamp": "2026-01-01T00:00:00Z"},
		Pagination: map[string]any{"cursor": nil, "limit": 200},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !reflect.DeepEqual(args, []any{"2026-01-01T00:00:00Z", 0, 200}) {
		t.Errorf("args = %#v", args)
	}
}

func TestCompile_ValidationErrors(t *testing.T) {
	cases := []struct {
		name   string
		query  string
		params []string
	}{
		{"placeholder beyond params", `select $2`, []string{"record.a"}},
		{"unreferenced parameter", `select $1`, []string{"record.a", "record.b"}},
		{"empty parameter expr", `select $1`, []string{"  "}},
		{"invalid expr", `select $1`, []string{"record.."}},
		{"invalid jinja", `select {% if %}`, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Compile(engine, tc.query, tc.params, "postgres"); err == nil {
				t.Fatalf("expected compile error")
			}
		})
	}
}

func TestCompile_RejectsValueInterpolation(t *testing.T) {
	// An output tag in the query would render a raw, unescaped value — SQL
	// injection. It must be rejected at compile time.
	cases := []string{
		`select * from t where name = '{{ record.name }}'`,
		`select * from {{ record.table }} where id = $1`,
		`select {{ record.col }} from t`,
	}
	for _, src := range cases {
		if _, err := Compile(engine, src, nil, "postgres"); err == nil {
			t.Errorf("expected rejection of value interpolation in %q", src)
		}
	}

	// {% if %} control blocks and {# comments #} are still allowed.
	if _, err := Compile(engine,
		`select * from t where id = $1 {# note #}{% if false %}x{% endif %}`,
		[]string{"record.id"}, "postgres"); err != nil {
		t.Errorf("control blocks/comments should be allowed: %v", err)
	}
}

// {% include %} would splice file content — possibly at a record-controlled
// path — straight into the SQL text, which HasOutputTag cannot see. The engine
// rejects the tag, so the no-interpolation guarantee holds.
func TestCompile_RejectsTemplateInclusion(t *testing.T) {
	for _, src := range []string{
		`select * from t {% include "/etc/passwd" %}`,
		`select * from t {% include record.frag %}`,
	} {
		if _, err := Compile(engine, src, nil, "postgres"); err == nil {
			t.Errorf("expected rejection of template inclusion in %q", src)
		}
	}
}

// UsesPagination inspects the expression AST: a record field whose name merely
// contains "pagination" must not be mistaken for pagination usage, which would
// hand the LIMIT/OFFSET clause to the author and make the paging loop repeat the
// same page forever.
func TestUsesPagination_NoSubstringFalsePositive(t *testing.T) {
	q, err := Compile(engine, `select * from t where token = $1`,
		[]string{"record.paginationToken"}, "postgres")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if q.UsesPagination() {
		t.Errorf("record.paginationToken must not count as pagination usage")
	}

	q, err = Compile(engine, `select * from t offset $1`, []string{"pagination.offset"}, "postgres")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if !q.UsesPagination() {
		t.Errorf("pagination.offset must count as pagination usage")
	}
}

func TestBuild_NoCastOrDollarQuoteConfusion(t *testing.T) {
	q, err := Compile(engine, `select id::text from t where id = $1`,
		[]string{"record.id"}, "postgres")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	sql, args, err := q.Build(ctxWithRecord(map[string]any{"id": 5}))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if sql != `select id::text from t where id = $1` || len(args) != 1 {
		t.Errorf("cast handling broken: sql=%q args=%#v", sql, args)
	}
}

// A $N inside a SQL string literal, a dollar-quoted body, or a comment is NOT a
// placeholder: it must not be counted, translated, or bound.
func TestPlaceholders_SkipLiteralsAndComments(t *testing.T) {
	cases := []struct {
		name   string
		query  string
		params []string
		driver string
		want   string
		args   []any
	}{
		{
			name:   "dollar in single-quoted literal",
			query:  `select '$5 fee' as label, x from t where id = $1`,
			params: []string{"record.id"},
			driver: "mysql",
			want:   `select '$5 fee' as label, x from t where id = ?`,
			args:   []any{7},
		},
		{
			name:   "dollar-quoted body with digit",
			query:  `select $$ costs $9 total $$ , x from t where id = $1`,
			params: []string{"record.id"},
			driver: "postgres",
			want:   `select $$ costs $9 total $$ , x from t where id = $1`,
			args:   []any{7},
		},
		{
			name:   "dollar in line comment",
			query:  "select x from t where id = $1 -- was $2 before\n",
			params: []string{"record.id"},
			driver: "postgres",
			// gonja strips the trailing newline; the $2 in the comment is skipped.
			want: "select x from t where id = $1 -- was $2 before",
			args: []any{7},
		},
		{
			name:   "escaped quote inside literal",
			query:  `select 'it''s $3' as s, x from t where id = $1`,
			params: []string{"record.id"},
			driver: "mysql",
			want:   `select 'it''s $3' as s, x from t where id = ?`,
			args:   []any{7},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, err := Compile(engine, tc.query, tc.params, tc.driver)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			sql, args, err := q.Build(ctxWithRecord(map[string]any{"id": 7}))
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			if sql != tc.want {
				t.Errorf("sql = %q, want %q", sql, tc.want)
			}
			if !reflect.DeepEqual(args, tc.args) {
				t.Errorf("args = %#v, want %#v", args, tc.args)
			}
		})
	}
}
