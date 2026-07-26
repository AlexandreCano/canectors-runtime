package input

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cannectors/runtime/internal/moduleconfig"
	"github.com/cannectors/runtime/internal/persistence"
	"github.com/cannectors/runtime/internal/sqltemplate"
	"github.com/cannectors/runtime/internal/template"
	"github.com/cannectors/runtime/pkg/connector"

	_ "modernc.org/sqlite"
)

func setupDatabaseInputSQLiteDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("closing sqlite db: %v", closeErr)
		}
	})

	_, err = db.Exec(`
		CREATE TABLE records (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		INSERT INTO records (id, name, updated_at) VALUES
			(1, 'old', '2026-01-01T00:00:00Z'),
			(2, 'middle', '2026-01-02T00:00:00Z'),
			(3, 'new', '2026-01-03T00:00:00Z'),
			(4, 'newer', '2026-01-04T00:00:00Z');
	`)
	if err != nil {
		t.Fatalf("creating database input fixtures: %v", err)
	}

	return db
}

func newDatabaseInputForTest(t *testing.T, db *sql.DB, cfg DatabaseInputConfig, state *persistence.State) *DatabaseInput {
	t.Helper()
	sqlQuery, err := sqltemplate.Compile(template.NewEngine(), cfg.Query, cfg.Parameters, "sqlite")
	if err != nil {
		t.Fatalf("compiling query: %v", err)
	}
	return &DatabaseInput{
		config:    cfg,
		db:        db,
		driver:    "sqlite",
		sqlQuery:  sqlQuery,
		timeout:   200 * time.Millisecond,
		lastState: state,
	}
}

func databaseInputState(ts *time.Time, id string) *persistence.State {
	state := &persistence.State{PipelineID: "test", LastTimestamp: ts}
	if id != "" {
		state.LastID = &id
	}
	return state
}

func assertDatabaseInputNames(t *testing.T, got []map[string]any, want []string) {
	t.Helper()
	names := make([]string, len(got))
	for i, record := range got {
		names[i], _ = record["name"].(string)
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("names = %#v, want %#v", names, want)
	}
}

func TestDatabaseInput_IncrementalQueries(t *testing.T) {
	since := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		cfg   DatabaseInputConfig
		state *persistence.State
		want  []string
	}{
		{
			name: "timestamp incremental",
			cfg: DatabaseInputConfig{
				SQLRequestBase: moduleconfig.SQLRequestBase{
					Query:      "SELECT id, name FROM records WHERE updated_at > $1 ORDER BY id",
					Parameters: []string{"state.lastRunTimestamp"},
				},
				Incremental: &IncrementalConfig{Enabled: true, TimestampField: "updated_at"},
			},
			state: databaseInputState(&since, ""),
			want:  []string{"new", "newer"},
		},
		{
			name: "id incremental",
			cfg: DatabaseInputConfig{
				SQLRequestBase: moduleconfig.SQLRequestBase{
					Query:      "SELECT id, name FROM records WHERE id > $1 ORDER BY id",
					Parameters: []string{"state.lastRunId ?? 0"},
				},
				Incremental: &IncrementalConfig{Enabled: true, IDField: "id"},
			},
			state: databaseInputState(nil, "2"),
			want:  []string{"new", "newer"},
		},
		{
			name: "timestamp and id incremental",
			cfg: DatabaseInputConfig{
				SQLRequestBase: moduleconfig.SQLRequestBase{
					Query:      "SELECT id, name FROM records WHERE updated_at > $1 AND id > $2 ORDER BY id",
					Parameters: []string{"state.lastRunTimestamp", "state.lastRunId ?? 0"},
				},
				Incremental: &IncrementalConfig{Enabled: true, TimestampField: "updated_at", IDField: "id"},
			},
			state: databaseInputState(&since, "2"),
			want:  []string{"new", "newer"},
		},
		{
			name: "first run uses epoch timestamp",
			cfg: DatabaseInputConfig{
				SQLRequestBase: moduleconfig.SQLRequestBase{
					Query:      "SELECT id, name FROM records WHERE updated_at > $1 ORDER BY id",
					Parameters: []string{"state.lastRunTimestamp"},
				},
			},
			want: []string{"old", "middle", "new", "newer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupDatabaseInputSQLiteDB(t)
			input := newDatabaseInputForTest(t, db, tt.cfg, tt.state)

			got, err := input.Fetch(context.Background())
			if err != nil {
				t.Fatalf("Fetch() error = %v", err)
			}
			assertDatabaseInputNames(t, got, tt.want)
		})
	}
}

func TestDatabaseInput_Pagination(t *testing.T) {
	tests := []struct {
		name string
		cfg  DatabaseInputConfig
		want []string
	}{
		{
			// No pagination parameter: the module appends LIMIT/OFFSET literally.
			name: "limit offset literal",
			cfg: DatabaseInputConfig{
				SQLRequestBase: moduleconfig.SQLRequestBase{Query: "SELECT id, name FROM records ORDER BY id"},
				Pagination:     &moduleconfig.DatabasePaginationConfig{Type: "limit-offset", Limit: 2},
			},
			want: []string{"old", "middle", "new", "newer"},
		},
		{
			// Pagination-aware parameters: the author owns LIMIT/OFFSET via $N.
			name: "limit offset with placeholders",
			cfg: DatabaseInputConfig{
				SQLRequestBase: moduleconfig.SQLRequestBase{
					Query:      "SELECT id, name FROM records ORDER BY id LIMIT $2 OFFSET $1",
					Parameters: []string{"pagination.offset", "pagination.limit"},
				},
				Pagination: &moduleconfig.DatabasePaginationConfig{Type: "limit-offset", Limit: 2},
			},
			want: []string{"old", "middle", "new", "newer"},
		},
		{
			// Cursor pagination: pagination.cursor is nil on the first page.
			name: "cursor with placeholders",
			cfg: DatabaseInputConfig{
				SQLRequestBase: moduleconfig.SQLRequestBase{
					Query:      "SELECT id, name FROM records WHERE id > $1 ORDER BY id LIMIT $2",
					Parameters: []string{"pagination.cursor ?? 0", "pagination.limit"},
				},
				Pagination: &moduleconfig.DatabasePaginationConfig{Type: "cursor", Limit: 2, CursorField: "id"},
			},
			want: []string{"old", "middle", "new", "newer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupDatabaseInputSQLiteDB(t)
			input := newDatabaseInputForTest(t, db, tt.cfg, nil)

			got, err := input.Fetch(context.Background())
			if err != nil {
				t.Fatalf("Fetch() error = %v", err)
			}
			assertDatabaseInputNames(t, got, tt.want)
		})
	}
}

func TestDatabaseInput_NewFromConfig_QueryFileAndEnvConnectionString(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "input.db")

	db, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		t.Fatalf("sql.Open sqlite file: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE records (id INTEGER PRIMARY KEY, name TEXT NOT NULL);
		INSERT INTO records (id, name) VALUES (1, 'Alice');
	`)
	if err != nil {
		t.Fatalf("creating sqlite file fixture: %v", err)
	}
	if closeErr := db.Close(); closeErr != nil {
		t.Fatalf("closing sqlite file fixture: %v", closeErr)
	}

	queryPath := filepath.Join(tmpDir, "input.sql")
	if writeErr := os.WriteFile(queryPath, []byte("SELECT id, name FROM records ORDER BY id"), 0600); writeErr != nil {
		t.Fatalf("writing query file: %v", writeErr)
	}
	t.Setenv("DATABASE_INPUT_TEST_DSN", "file:"+dbPath)

	input, err := NewDatabaseInputFromConfig(&connector.ModuleConfig{
		Type: "database",
		Raw: mustJSON(map[string]any{
			"connectionStringRef": "${DATABASE_INPUT_TEST_DSN}",
			"driver":              "sqlite",
			"queryFile":           queryPath,
			"incremental": map[string]any{
				"enabled":        true,
				"timestampField": "updated_at",
			},
		}),
	})
	if err != nil {
		t.Fatalf("NewDatabaseInputFromConfig() error = %v", err)
	}
	defer func() {
		if closeErr := input.Close(); closeErr != nil {
			t.Errorf("closing database input: %v", closeErr)
		}
	}()

	got, err := input.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	assertDatabaseInputNames(t, got, []string{"Alice"})

	if cfg := input.GetPersistenceConfig(); cfg == nil || !cfg.TimestampEnabled() {
		t.Fatalf("persistence config = %#v, want timestamp enabled", cfg)
	}
}

func TestDatabaseInput_BuildDriverSpecificPlaceholders(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	querySrc := "SELECT * FROM records WHERE updated_at > $1 AND id > $2 AND name = $3"
	params := []string{"state.lastRunTimestamp", "state.lastRunId", `"Alice"`}

	build := func(driver string) (string, []any) {
		t.Helper()
		sqlQuery, err := sqltemplate.Compile(template.NewEngine(), querySrc, params, driver)
		if err != nil {
			t.Fatalf("compiling query: %v", err)
		}
		input := &DatabaseInput{driver: driver, sqlQuery: sqlQuery, lastState: databaseInputState(&ts, "42")}
		query, args, buildErr := input.buildForPage(nil)
		if buildErr != nil {
			t.Fatalf("buildForPage: %v", buildErr)
		}
		return query, args
	}

	query, args := build("postgres")
	if query != "SELECT * FROM records WHERE updated_at > $1 AND id > $2 AND name = $3" {
		t.Fatalf("query = %q", query)
	}
	if len(args) != 3 || args[0] != ts.Format(time.RFC3339) || args[1] != "42" || args[2] != "Alice" {
		t.Fatalf("args = %#v", args)
	}

	query, args = build("mysql")
	if strings.Count(query, "?") != 3 {
		t.Fatalf("mysql query = %q, want three ? placeholders", query)
	}
	if len(args) != 3 {
		t.Fatalf("mysql args len = %d, want 3", len(args))
	}
}
