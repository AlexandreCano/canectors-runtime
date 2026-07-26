// Package input provides implementations for input modules.
// DatabaseInput module fetches data from databases using SQL queries.
package input

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/cannectors/runtime/internal/database"
	"github.com/cannectors/runtime/internal/errhandling"
	"github.com/cannectors/runtime/internal/logger"
	"github.com/cannectors/runtime/internal/moduleconfig"
	"github.com/cannectors/runtime/internal/pathutil"
	"github.com/cannectors/runtime/internal/persistence"
	"github.com/cannectors/runtime/internal/sqltemplate"
	"github.com/cannectors/runtime/internal/template"
	"github.com/cannectors/runtime/pkg/connector"
)

// Default configuration values for database input
const (
	defaultDatabaseTimeout = 30 * time.Second
	defaultQueryLimit      = 1000
)

// Error types for database input module
var (
	ErrDatabaseNilConfig = errors.New("database input configuration is nil")
)

// DatabaseInputConfig holds configuration for the database input module.
// The query is a Jinja template with $N placeholders bound to the parameters
// list (inherited from SQLRequestBase). Parameter expressions may reference
// `state` (state.lastRunTimestamp, state.lastRunId) and, when pagination is
// configured, `pagination` (pagination.offset / pagination.cursor /
// pagination.limit) — pagination-aware parameters are re-evaluated per page.
type DatabaseInputConfig struct {
	connector.ModuleBase
	moduleconfig.SQLRequestBase

	// Pagination configuration
	Pagination *moduleconfig.DatabasePaginationConfig `json:"pagination"`

	// Incremental query configuration
	Incremental *IncrementalConfig `json:"incremental"`
}

// IncrementalConfig defines incremental query configuration. The fields name
// which record columns feed the persisted state; the query consumes that state
// through parameter expressions (state.lastRunTimestamp, state.lastRunId).
type IncrementalConfig struct {
	// Enabled: whether to enable incremental queries
	Enabled bool `json:"enabled"`
	// TimestampField: field name for timestamp-based incremental queries
	TimestampField string `json:"timestampField"`
	// IDField: field name for ID-based incremental queries
	IDField string `json:"idField"`
}

// DatabaseInput implements a database input module.
// It executes SQL queries to fetch data from databases.
type DatabaseInput struct {
	config     DatabaseInputConfig
	db         *sql.DB
	driver     string
	sqlQuery   *sqltemplate.Query
	timeout    time.Duration
	pipelineID string
	stateStore *persistence.StateStore
	lastState  *persistence.State
}

// NewDatabaseInputFromConfig creates a new database input module from configuration.
func NewDatabaseInputFromConfig(cfg *connector.ModuleConfig) (*DatabaseInput, error) {
	if cfg == nil {
		return nil, ErrDatabaseNilConfig
	}

	config, err := moduleconfig.ParseModuleConfig[DatabaseInputConfig](*cfg)
	if err != nil {
		return nil, err
	}

	if validateErr := config.Validate(); validateErr != nil {
		return nil, fmt.Errorf("database input: %w", validateErr)
	}

	if validateErr := config.Pagination.Validate(); validateErr != nil {
		return nil, fmt.Errorf("database input: %w", validateErr)
	}

	// Cursor pagination has no module-side fallback: the cursor can only reach the
	// query through a parameter expression. Without one, every page would re-run
	// the same unbounded query and the paging loop would never advance.
	if config.Pagination != nil && config.Pagination.Type == "cursor" &&
		!sqltemplate.ReferencesPagination(config.Parameters) {
		return nil, fmt.Errorf("database input: cursor pagination requires a parameter expression referencing the pagination " +
			"context (e.g. parameters: [\"pagination.cursor ?? 0\"] bound to a $N placeholder in the query)")
	}

	// Load query from file if queryFile is specified
	if config.QueryFile != "" {
		if validateErr := pathutil.ValidateFilePath(config.QueryFile); validateErr != nil {
			return nil, fmt.Errorf("query file path: %w", validateErr)
		}
		queryBytes, readErr := os.ReadFile(config.QueryFile)
		if readErr != nil {
			return nil, fmt.Errorf("reading query file %s: %w", config.QueryFile, readErr)
		}
		config.Query = string(queryBytes)
		if config.Query == "" {
			return nil, fmt.Errorf("query file %s is empty", config.QueryFile)
		}
	}

	if _, parseErr := errhandling.ParseOnErrorStrategy(config.OnError); parseErr != nil {
		return nil, parseErr
	}

	// Set timeout
	timeout := connector.GetTimeoutDuration(config.TimeoutMs, defaultDatabaseTimeout)

	// Create database config
	dbConfig := database.Config{
		ConnectionString:    config.ConnectionString,
		ConnectionStringRef: config.ConnectionStringRef,
		Driver:              config.Driver,
		MaxOpenConns:        config.MaxOpenConns,
		MaxIdleConns:        config.MaxIdleConns,
		ConnMaxLifetime:     time.Duration(config.ConnMaxLifetimeSeconds) * time.Second,
		ConnMaxIdleTime:     time.Duration(config.ConnMaxIdleTimeSeconds) * time.Second,
		ConnectTimeout:      timeout,
	}

	// Open database connection
	db, driver, err := database.Open(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("creating database connection: %w", err)
	}

	sqlQuery, err := sqltemplate.Compile(templateEngine, config.Query, config.Parameters, driver)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("database input query: %w", err)
	}

	module := &DatabaseInput{
		config:   config,
		db:       db,
		driver:   driver,
		sqlQuery: sqlQuery,
		timeout:  timeout,
	}

	// Initialize state store if incremental is enabled
	if config.Incremental != nil && config.Incremental.Enabled {
		module.stateStore = persistence.NewStateStore(persistence.DefaultStatePath)
	}

	logger.Debug("database input module created",
		"driver", driver,
		"timeout", timeout.String(),
		"has_pagination", config.Pagination != nil,
		"has_incremental", config.Incremental != nil && config.Incremental.Enabled,
	)

	return module, nil
}

// Fetch retrieves data from the database.
func (d *DatabaseInput) Fetch(ctx context.Context) ([]map[string]any, error) {
	startTime := time.Now()

	// Load state if state store is initialized (for incremental queries)
	if d.stateStore != nil && d.pipelineID != "" {
		state, err := d.LoadState()
		if err != nil {
			logger.Warn("failed to load state for database input, continuing without incremental support",
				"pipeline_id", d.pipelineID,
				"error", err.Error(),
			)
		} else if state != nil {
			d.lastState = state
		}
	}

	logger.Info("database input fetch started",
		"module_type", "database",
		"driver", d.driver,
		"has_pagination", d.config.Pagination != nil,
		"has_incremental", d.config.Incremental != nil && d.config.Incremental.Enabled,
	)

	// Execute query based on pagination configuration
	var records []map[string]any
	var err error

	if d.config.Pagination != nil && d.config.Pagination.Type != "" {
		records, err = d.fetchWithPagination(ctx)
	} else {
		records, err = d.fetchSingle(ctx)
	}

	duration := time.Since(startTime)

	if err != nil {
		logger.Error("database input fetch failed",
			"module_type", "database",
			"duration", duration,
			"error", err.Error(),
		)
		return nil, err
	}

	logger.Info("database input fetch completed",
		"module_type", "database",
		"record_count", len(records),
		"duration", duration,
	)

	return records, nil
}

// stateContext exposes the persisted incremental state to query templates and
// parameter expressions as `state.*`. On the first run lastRunTimestamp is the
// epoch, so incremental queries fetch all records.
func (d *DatabaseInput) stateContext() map[string]any {
	timestamp := time.Unix(0, 0).UTC()
	if d.lastState != nil && d.lastState.LastTimestamp != nil {
		timestamp = *d.lastState.LastTimestamp
	}
	var lastID any
	if d.lastState != nil && d.lastState.LastID != nil {
		lastID = *d.lastState.LastID
	}
	return map[string]any{
		"lastRunTimestamp": timestamp.Format(time.RFC3339),
		"lastRunId":        lastID,
	}
}

// buildForPage renders the query for one page of results. Pagination-aware
// parameter expressions (pagination.offset / pagination.cursor /
// pagination.limit) are re-evaluated with the page context.
func (d *DatabaseInput) buildForPage(pagination map[string]any) (string, []any, error) {
	return d.sqlQuery.Build(template.RenderContext{
		State:      d.stateContext(),
		Pagination: pagination,
	})
}

// fetchSingle executes a single query without pagination.
func (d *DatabaseInput) fetchSingle(ctx context.Context) ([]map[string]any, error) {
	query, args, err := d.buildForPage(nil)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		dbErr := database.ClassifyDatabaseError(err, d.driver, "select", query, len(args))
		return nil, dbErr
	}
	defer func() {
		_ = rows.Close()
	}()

	return d.rowsToRecords(rows)
}

// fetchWithPagination executes queries with pagination.
func (d *DatabaseInput) fetchWithPagination(ctx context.Context) ([]map[string]any, error) {
	switch d.config.Pagination.Type {
	case "limit-offset":
		return d.fetchLimitOffset(ctx)
	case "cursor":
		return d.fetchCursor(ctx)
	default:
		return nil, fmt.Errorf("database input: unknown pagination type %q (expected 'limit-offset' or 'cursor')", d.config.Pagination.Type)
	}
}

// fetchLimitOffset implements LIMIT/OFFSET pagination.
//
// When a parameter expression references the pagination context, the author
// owns the LIMIT/OFFSET clause (e.g. `limit $2 offset $1` with parameters
// pagination.limit / pagination.offset). Otherwise the module appends
// `LIMIT n OFFSET m` literally, as before.
func (d *DatabaseInput) fetchLimitOffset(ctx context.Context) ([]map[string]any, error) {
	var allRecords []map[string]any
	offset := 0
	limit := d.config.Pagination.Limit
	if limit <= 0 {
		limit = defaultQueryLimit
	}
	usesPagination := d.sqlQuery.UsesPagination()

	for {
		query, args, err := d.buildForPage(map[string]any{"offset": offset, "limit": limit})
		if err != nil {
			return nil, err
		}
		if !usesPagination {
			query = fmt.Sprintf("%s LIMIT %d OFFSET %d", query, limit, offset)
		}

		queryCtx, cancel := context.WithTimeout(ctx, d.timeout)
		rows, err := d.db.QueryContext(queryCtx, query, args...)
		if err != nil {
			cancel()
			dbErr := database.ClassifyDatabaseError(err, d.driver, "select", query, len(args))
			return nil, dbErr
		}

		records, err := d.rowsToRecords(rows)
		_ = rows.Close()
		cancel()

		if err != nil {
			return nil, err
		}

		allRecords = append(allRecords, records...)

		if len(records) < limit {
			break
		}

		offset += limit
	}

	return allRecords, nil
}

// fetchCursor implements cursor-based pagination.
//
// The cursor is exposed to parameter expressions as pagination.cursor (nil on
// the first page — use an expression like `pagination.cursor ?? 0` to provide
// a starting bound). A query that binds no pagination parameter is rejected at
// construction, so the cursor always reaches the query.
func (d *DatabaseInput) fetchCursor(ctx context.Context) ([]map[string]any, error) {
	var allRecords []map[string]any
	var cursor any
	limit := d.config.Pagination.Limit
	if limit <= 0 {
		limit = defaultQueryLimit
	}

	cursorField := d.config.Pagination.CursorField

	for {
		query, args, err := d.buildForPage(map[string]any{"cursor": cursor, "limit": limit})
		if err != nil {
			return nil, err
		}

		queryCtx, cancel := context.WithTimeout(ctx, d.timeout)
		rows, err := d.db.QueryContext(queryCtx, query, args...)
		if err != nil {
			cancel()
			dbErr := database.ClassifyDatabaseError(err, d.driver, "select", query, len(args))
			return nil, dbErr
		}

		records, err := d.rowsToRecords(rows)
		_ = rows.Close()
		cancel()

		if err != nil {
			return nil, err
		}

		allRecords = append(allRecords, records...)

		if len(records) < limit {
			break
		}

		if cursorField != "" && len(records) > 0 {
			lastRecord := records[len(records)-1]
			cursor = lastRecord[cursorField]
		} else {
			break
		}
	}

	return allRecords, nil
}

// rowsToRecords converts sql.Rows to a slice of map records.
func (d *DatabaseInput) rowsToRecords(rows *sql.Rows) ([]map[string]any, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("getting column names: %w", err)
	}

	var records []map[string]any

	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		record := make(map[string]any, len(columns))
		for i, col := range columns {
			val := values[i]
			record[col] = convertDatabaseValue(val)
		}

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return records, nil
}

// convertDatabaseValue converts database values to appropriate Go types.
func convertDatabaseValue(val any) any {
	if val == nil {
		return nil
	}

	if b, ok := val.([]byte); ok {
		return string(b)
	}

	if t, ok := val.(time.Time); ok {
		return t.Format(time.RFC3339)
	}

	return val
}

// Close releases resources held by the database input module.
func (d *DatabaseInput) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// SetPipelineID sets the pipeline ID for state persistence.
func (d *DatabaseInput) SetPipelineID(pipelineID string) {
	d.pipelineID = pipelineID
}

// LoadState loads the last persisted state for this pipeline.
func (d *DatabaseInput) LoadState() (*persistence.State, error) {
	if d.stateStore == nil || d.pipelineID == "" {
		return nil, nil
	}

	state, err := d.stateStore.Load(d.pipelineID)
	if err != nil {
		logger.Warn("failed to load state for database input",
			"pipeline_id", d.pipelineID,
			"error", err.Error(),
		)
		return nil, err
	}

	d.lastState = state
	return state, nil
}

// GetPersistenceConfig returns the state persistence configuration.
func (d *DatabaseInput) GetPersistenceConfig() *persistence.StatePersistenceConfig {
	if d.config.Incremental == nil || !d.config.Incremental.Enabled {
		return nil
	}

	return &persistence.StatePersistenceConfig{
		Timestamp: &persistence.TimestampConfig{
			Enabled: d.config.Incremental.TimestampField != "",
		},
		ID: &persistence.IDConfig{
			Enabled: d.config.Incremental.IDField != "",
			Field:   d.config.Incremental.IDField,
		},
	}
}

// GetLastState returns the last loaded state.
func (d *DatabaseInput) GetLastState() *persistence.State {
	return d.lastState
}
