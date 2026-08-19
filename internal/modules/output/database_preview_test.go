package output

import (
	"context"
	"strings"
	"testing"

	"github.com/cannectors/runtime/pkg/connector"
)

// unreachableDSN points at a host that cannot resolve, so any test using it
// proves the code path never dialed.
const unreachableDSN = "postgres://lab_user:lab_password@nonexistent.invalid:5432/warehouse"

// newPreviewOutput builds a database output against an unreachable DSN. The
// constructor must succeed: connecting is Connect's job, not its own.
func newPreviewOutput(t *testing.T, raw map[string]any) *DatabaseOutput {
	t.Helper()
	if _, ok := raw["connectionString"]; !ok {
		raw["connectionString"] = unreachableDSN
	}
	module, err := NewDatabaseOutputFromConfig(&connector.ModuleConfig{
		Type: "database",
		Raw:  mustJSON(raw),
	})
	if err != nil {
		t.Fatalf("NewDatabaseOutputFromConfig: %v", err)
	}
	return module
}

// TestDatabaseOutput_ConstructorDoesNotConnect locks Story 25.6 AC2: building
// the module against an unreachable host succeeds, because nothing is opened
// until Connect.
func TestDatabaseOutput_ConstructorDoesNotConnect(t *testing.T) {
	module := newPreviewOutput(t, map[string]any{
		"query":      "INSERT INTO events (id) VALUES ($1)",
		"parameters": []string{"record.id"},
	})
	defer func() { _ = module.Close() }()

	if module.db != nil {
		t.Fatal("constructor opened a connection")
	}
	if module.resolved.Driver != "postgres" {
		t.Errorf("driver = %q, want postgres", module.resolved.Driver)
	}
}

// TestDatabaseOutput_PreviewDoesNotConnect locks Story 25.6 AC1/AC2: the preview
// describes the statement and leaves the connection unopened.
func TestDatabaseOutput_PreviewDoesNotConnect(t *testing.T) {
	module := newPreviewOutput(t, map[string]any{
		"query":      "INSERT INTO events (id, seq) VALUES ($1, $2)",
		"parameters": []string{"record.id", "record.seq"},
	})
	defer func() { _ = module.Close() }()

	records := []map[string]any{
		{"id": "EVT-1", "seq": 1},
		{"id": "EVT-2", "seq": 2},
	}
	previews, err := module.PreviewOperations(records, PreviewOptions{})
	if err != nil {
		t.Fatalf("PreviewOperations: %v", err)
	}
	if module.db != nil {
		t.Fatal("preview opened a connection")
	}
	if len(previews) != 1 {
		t.Fatalf("previews = %d, want 1", len(previews))
	}

	preview := previews[0]
	if preview.Kind != connector.PreviewKindSQL {
		t.Errorf("Kind = %q, want %q", preview.Kind, connector.PreviewKindSQL)
	}
	if preview.HTTP != nil {
		t.Error("a SQL preview must not carry an HTTP block")
	}
	if preview.RecordCount != 2 {
		t.Errorf("RecordCount = %d, want 2", preview.RecordCount)
	}

	sqlPreview := preview.SQL
	if sqlPreview == nil {
		t.Fatal("SQL block is nil")
	}
	if sqlPreview.Operation != "INSERT INTO" || sqlPreview.Table != "events" {
		t.Errorf("operation/table = %q/%q, want %q/%q",
			sqlPreview.Operation, sqlPreview.Table, "INSERT INTO", "events")
	}
	if sqlPreview.Statement != "INSERT INTO events (id, seq) VALUES ($1, $2)" {
		t.Errorf("unexpected statement: %q", sqlPreview.Statement)
	}
	if got := strings.Join(sqlPreview.Parameters, ","); got != "record.id,record.seq" {
		t.Errorf("parameters = %q", got)
	}
	if len(sqlPreview.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(sqlPreview.Rows))
	}
	if got := strings.Join(sqlPreview.Rows[0], ", "); got != `"EVT-1", 1` {
		t.Errorf("row 0 = %q", got)
	}
}

// TestDatabaseOutput_PreviewMasksCredentials locks Story 25.6 AC4: the password
// in the connection string never reaches the preview unless asked for.
func TestDatabaseOutput_PreviewMasksCredentials(t *testing.T) {
	module := newPreviewOutput(t, map[string]any{
		"query":      "INSERT INTO events (id) VALUES ($1)",
		"parameters": []string{"record.id"},
	})
	defer func() { _ = module.Close() }()
	records := []map[string]any{{"id": "EVT-1"}}

	masked, err := module.PreviewOperations(records, PreviewOptions{})
	if err != nil {
		t.Fatalf("PreviewOperations: %v", err)
	}
	if strings.Contains(masked[0].SQL.Target, "lab_password") {
		t.Errorf("preview leaks the password: %s", masked[0].SQL.Target)
	}
	if !strings.Contains(masked[0].SQL.Target, "[REDACTED]") {
		t.Errorf("preview does not mark the redaction: %s", masked[0].SQL.Target)
	}
	if !strings.Contains(masked[0].SQL.Target, "nonexistent.invalid") {
		t.Errorf("preview hides the host, which the author needs: %s", masked[0].SQL.Target)
	}

	shown, err := module.PreviewOperations(records, PreviewOptions{ShowCredentials: true})
	if err != nil {
		t.Fatalf("PreviewOperations: %v", err)
	}
	if !strings.Contains(shown[0].SQL.Target, "lab_password") {
		t.Errorf("ShowCredentials did not reveal the password: %s", shown[0].SQL.Target)
	}
}

// TestDatabaseOutput_PreviewAggregatesBatch locks Story 25.6 AC5: a large batch
// yields one preview with capped rows and a count of the rest, not one preview
// per record.
func TestDatabaseOutput_PreviewAggregatesBatch(t *testing.T) {
	module := newPreviewOutput(t, map[string]any{
		"query":       "INSERT INTO events (id) VALUES ($1)",
		"parameters":  []string{"record.id"},
		"transaction": true,
	})
	defer func() { _ = module.Close() }()

	records := make([]map[string]any, 250)
	for i := range records {
		records[i] = map[string]any{"id": i}
	}
	previews, err := module.PreviewOperations(records, PreviewOptions{})
	if err != nil {
		t.Fatalf("PreviewOperations: %v", err)
	}
	if len(previews) != 1 {
		t.Fatalf("previews = %d, want 1 aggregated preview", len(previews))
	}
	sqlPreview := previews[0].SQL
	if previews[0].RecordCount != 250 {
		t.Errorf("RecordCount = %d, want 250", previews[0].RecordCount)
	}
	if len(sqlPreview.Rows) != maxPreviewRows {
		t.Errorf("rows = %d, want the %d-row cap", len(sqlPreview.Rows), maxPreviewRows)
	}
	if sqlPreview.RowsOmitted != 250-maxPreviewRows {
		t.Errorf("RowsOmitted = %d, want %d", sqlPreview.RowsOmitted, 250-maxPreviewRows)
	}
	if !sqlPreview.Transaction {
		t.Error("Transaction should be reported")
	}
}

// TestDatabaseOutput_PreviewSplitsPerRenderedStatement locks that a templated
// query rendering differently per record is previewed per variant: reporting one
// statement for all of them would show SQL half the records never run.
func TestDatabaseOutput_PreviewSplitsPerRenderedStatement(t *testing.T) {
	module := newPreviewOutput(t, map[string]any{
		"query":      "INSERT INTO {% if record.archived %}events_archive{% else %}events{% endif %} (id) VALUES ($1)",
		"parameters": []string{"record.id"},
	})
	defer func() { _ = module.Close() }()

	records := []map[string]any{
		{"id": "EVT-1", "archived": false},
		{"id": "EVT-2", "archived": true},
		{"id": "EVT-3", "archived": true},
	}
	previews, err := module.PreviewOperations(records, PreviewOptions{})
	if err != nil {
		t.Fatalf("PreviewOperations: %v", err)
	}
	if len(previews) != 2 {
		t.Fatalf("previews = %d, want one per rendered statement", len(previews))
	}
	tables := map[string]int{}
	for _, preview := range previews {
		tables[preview.SQL.Table] = preview.RecordCount
	}
	if tables["events"] != 1 || tables["events_archive"] != 2 {
		t.Errorf("unexpected split: %v", tables)
	}
}

// TestDatabaseOutput_PreviewLabelsSurvivingParameters locks that the parameter
// labels describe the rendered statement: a dropped conditional clause binds
// fewer values, and the declared list would label them off by one.
func TestDatabaseOutput_PreviewLabelsSurvivingParameters(t *testing.T) {
	module := newPreviewOutput(t, map[string]any{
		"query":      "UPDATE t SET a=$1{% if record.withB %}, b=$2{% endif %} WHERE id=$3",
		"parameters": []string{"record.a", "record.b", "record.id"},
	})
	defer func() { _ = module.Close() }()

	previews, err := module.PreviewOperations([]map[string]any{
		{"a": "A", "b": "B", "id": "ID-1", "withB": false},
		{"a": "A", "b": "B", "id": "ID-2", "withB": true},
	}, PreviewOptions{})
	if err != nil {
		t.Fatalf("PreviewOperations: %v", err)
	}
	if len(previews) != 2 {
		t.Fatalf("previews = %d, want one per rendered statement", len(previews))
	}

	byParams := map[string][]string{}
	for _, preview := range previews {
		sqlPreview := preview.SQL
		if len(sqlPreview.Parameters) != len(sqlPreview.Rows[0]) {
			t.Errorf("%q: %d labels for %d values (%v / %v)", sqlPreview.Statement,
				len(sqlPreview.Parameters), len(sqlPreview.Rows[0]),
				sqlPreview.Parameters, sqlPreview.Rows[0])
		}
		byParams[strings.Join(sqlPreview.Parameters, ",")] = sqlPreview.Rows[0]
	}

	// Without the b clause, $2 is the id — labeled record.id, not record.b.
	if got := byParams["record.a,record.id"]; strings.Join(got, ",") != `"A","ID-1"` {
		t.Errorf("short statement bound %v, want the id in second position", got)
	}
	if got := byParams["record.a,record.b,record.id"]; strings.Join(got, ",") != `"A","B","ID-2"` {
		t.Errorf("full statement bound %v", got)
	}
}

// TestDatabaseOutput_PreviewLabelsRepeatedPlaceholder locks the mysql/sqlite
// side: one argument is bound per occurrence, so a repeated placeholder is
// labeled twice rather than leaving a value unnamed.
func TestDatabaseOutput_PreviewLabelsRepeatedPlaceholder(t *testing.T) {
	module := newPreviewOutput(t, map[string]any{
		"connectionString": "mysql://lab_user:lab_password@nonexistent.invalid:3306/warehouse",
		"driver":           "mysql",
		"query":            "INSERT INTO events (id) VALUES ($1) ON DUPLICATE KEY UPDATE id=$1",
		"parameters":       []string{"record.id"},
	})
	defer func() { _ = module.Close() }()

	previews, err := module.PreviewOperations([]map[string]any{{"id": "EVT-1"}}, PreviewOptions{})
	if err != nil {
		t.Fatalf("PreviewOperations: %v", err)
	}
	sqlPreview := previews[0].SQL
	if got := strings.Join(sqlPreview.Parameters, ","); got != "record.id,record.id" {
		t.Errorf("parameters = %q, want one label per bound occurrence", got)
	}
	if got := strings.Join(sqlPreview.Rows[0], ","); got != `"EVT-1","EVT-1"` {
		t.Errorf("rows = %q", got)
	}
}

// TestDatabaseOutput_PreviewEmptyRecords keeps the empty case quiet rather than
// reporting a statement nothing would run.
func TestDatabaseOutput_PreviewEmptyRecords(t *testing.T) {
	module := newPreviewOutput(t, map[string]any{
		"query":      "INSERT INTO events (id) VALUES ($1)",
		"parameters": []string{"record.id"},
	})
	defer func() { _ = module.Close() }()

	previews, err := module.PreviewOperations(nil, PreviewOptions{})
	if err != nil {
		t.Fatalf("PreviewOperations: %v", err)
	}
	if len(previews) != 0 {
		t.Errorf("previews = %d, want none", len(previews))
	}
}

// TestDatabaseOutput_ConnectFailsOnUnreachableHost locks the other half of the
// contract: a real run still refuses to start against an unreachable database.
func TestDatabaseOutput_ConnectFailsOnUnreachableHost(t *testing.T) {
	module := newPreviewOutput(t, map[string]any{
		"query":      "INSERT INTO events (id) VALUES ($1)",
		"parameters": []string{"record.id"},
		"timeoutMs":  2000,
	})
	defer func() { _ = module.Close() }()

	if err := module.Connect(context.Background()); err == nil {
		t.Fatal("Connect succeeded against an unreachable host")
	}
}

// TestDescribeStatement covers the statement header, including the shapes it
// deliberately refuses to guess.
func TestDescribeStatement(t *testing.T) {
	cases := []struct {
		statement     string
		wantOperation string
		wantTable     string
	}{
		{"INSERT INTO users (a) VALUES ($1)", "INSERT INTO", "users"},
		{"insert into public.users (a) VALUES ($1)", "INSERT INTO", "public.users"},
		{"\n  UPDATE inventory SET qty = $1", "UPDATE", "inventory"},
		{"DELETE FROM sessions WHERE id = $1", "DELETE FROM", "sessions"},
		{`INSERT INTO "public"."users" (a) VALUES ($1)`, "INSERT INTO", `"public"."users"`},
		{"INSERT INTO `orders` (a) VALUES (?)", "INSERT INTO", "`orders`"},
		{"SELECT set_config('x', $1, false)", "SELECT", ""},
		{"CALL refresh_totals($1)", "STATEMENT", ""},
	}
	for _, tc := range cases {
		t.Run(tc.statement, func(t *testing.T) {
			operation, table := describeStatement(tc.statement)
			if operation != tc.wantOperation || table != tc.wantTable {
				t.Errorf("describeStatement(%q) = %q/%q, want %q/%q",
					tc.statement, operation, table, tc.wantOperation, tc.wantTable)
			}
		})
	}
}
