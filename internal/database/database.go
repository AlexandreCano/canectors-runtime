// Package database provides database utilities for Cannectors runtime.
// It provides helpers for driver detection, placeholder conversion, and connection string handling.
// Connection pooling is handled by the standard database/sql package.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/cannectors/runtime/internal/logger"
)

// Supported database driver types
const (
	DriverPostgres = "postgres"
	DriverMySQL    = "mysql"
	DriverSQLite   = "sqlite"
)

// Default connection pool settings
const (
	DefaultMaxOpenConns    = 10
	DefaultMaxIdleConns    = 5
	DefaultConnMaxLifetime = 30 * time.Minute
	DefaultConnMaxIdleTime = 5 * time.Minute
	DefaultConnectTimeout  = 10 * time.Second
)

// Error types for database operations
var (
	ErrMissingConnectionString = errors.New("connection string is required")
	ErrUnsupportedDriver       = errors.New("unsupported database driver")
	ErrConnectionFailed        = errors.New("database connection failed")
)

// Config holds database connection configuration.
type Config struct {
	// ConnectionString is the database connection string (DSN).
	// Formats:
	//   - PostgreSQL: postgres://user:pass@host:port/db?sslmode=require
	//   - MySQL: user:pass@tcp(host:port)/db?tls=true
	//   - SQLite: file:path/to/database.db
	ConnectionString string

	// ConnectionStringRef is an environment variable reference for the connection string.
	// Format: ${ENV_VAR_NAME}
	// Takes precedence over ConnectionString if both are set.
	ConnectionStringRef string

	// Driver specifies the database driver type (postgres, mysql, sqlite).
	// If empty, it will be auto-detected from the connection string.
	Driver string

	// Pool configuration
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration

	// ConnectTimeout is the timeout for establishing a connection.
	ConnectTimeout time.Duration
}

// Resolved holds everything needed to open a connection, obtained without any
// I/O. Splitting it out lets a caller validate its configuration — and compile
// driver-specific SQL — without opening a connection, which is what dry-run
// needs (Story 25.6).
type Resolved struct {
	// ConnString is the connection string after ${VAR} resolution.
	ConnString string

	// Driver is the resolved, supported driver name (postgres, mysql, sqlite).
	Driver string
}

// Resolve resolves the connection string and the driver without contacting the
// database. It reports the same configuration errors Open would report before
// dialing: a missing connection string, an undetectable or unsupported driver.
func Resolve(cfg Config) (Resolved, error) {
	connString, err := ResolveConnectionString(cfg.ConnectionString, cfg.ConnectionStringRef)
	if err != nil {
		return Resolved{}, err
	}

	driver := cfg.Driver
	if driver == "" {
		driver, err = DetectDriver(connString)
		if err != nil {
			return Resolved{}, err
		}
	}

	if !IsDriverSupported(driver) {
		return Resolved{}, fmt.Errorf("%w: %s", ErrUnsupportedDriver, driver)
	}

	return Resolved{ConnString: connString, Driver: driver}, nil
}

// Open creates a new database connection pool with the given configuration.
// Uses the standard database/sql package for connection pooling.
//
// It is for callers that open in a constructor and have no context to offer; a
// caller holding one should use Resolve and OpenResolved so a cancellation
// interrupts the connection attempt.
func Open(cfg Config) (*sql.DB, string, error) {
	resolved, err := Resolve(cfg)
	if err != nil {
		return nil, "", err
	}
	db, err := OpenResolved(context.Background(), cfg, resolved)
	if err != nil {
		return nil, "", err
	}
	return db, resolved.Driver, nil
}

// OpenResolved opens and validates the connection pool for an already-resolved
// configuration. It is the only step that touches the network.
//
// The connect timeout bounds ctx rather than replacing it: a canceled run must
// stop waiting on an unreachable host instead of sitting out the full timeout.
func OpenResolved(ctx context.Context, cfg Config, resolved Resolved) (*sql.DB, error) {
	// Get the actual driver name for sql.Open
	driverName := GetDriverName(resolved.Driver)

	// Open database connection
	db, err := sql.Open(driverName, resolved.ConnString)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConnectionFailed, err)
	}

	// Configure connection pool
	applyPoolConfig(db, cfg)

	// Test connection with timeout
	timeout := cfg.ConnectTimeout
	if timeout <= 0 {
		timeout = DefaultConnectTimeout
	}
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("%w: ping failed: %w", ErrConnectionFailed, err)
	}

	logger.Debug("database connection established",
		"driver", resolved.Driver,
		"max_open_conns", cfg.MaxOpenConns,
		"max_idle_conns", cfg.MaxIdleConns,
	)

	// Log connection pool stats for monitoring
	stats := db.Stats()
	logger.Debug("database connection pool stats",
		"driver", resolved.Driver,
		"open_connections", stats.OpenConnections,
		"in_use", stats.InUse,
		"idle", stats.Idle,
		"wait_count", stats.WaitCount,
		"wait_duration", stats.WaitDuration,
	)

	return db, nil
}

// applyPoolConfig configures the connection pool with defaults.
func applyPoolConfig(db *sql.DB, cfg Config) {
	maxOpen := cfg.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = DefaultMaxOpenConns
	}
	maxIdle := cfg.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = DefaultMaxIdleConns
	}
	maxLifetime := cfg.ConnMaxLifetime
	if maxLifetime <= 0 {
		maxLifetime = DefaultConnMaxLifetime
	}
	maxIdleTime := cfg.ConnMaxIdleTime
	if maxIdleTime <= 0 {
		maxIdleTime = DefaultConnMaxIdleTime
	}

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(maxLifetime)
	db.SetConnMaxIdleTime(maxIdleTime)
}

// ResolveConnectionString resolves the connection string from config or environment.
func ResolveConnectionString(connString, connStringRef string) (string, error) {
	// Try environment variable reference first
	if connStringRef != "" {
		resolved := ResolveEnvRef(connStringRef)
		if resolved != "" {
			return resolved, nil
		}
	}

	// Use direct connection string
	if connString != "" {
		return connString, nil
	}

	return "", ErrMissingConnectionString
}

// ResolveEnvRef extracts the value from an environment variable reference.
// Format: ${ENV_VAR_NAME}
func ResolveEnvRef(ref string) string {
	if !strings.HasPrefix(ref, "${") || !strings.HasSuffix(ref, "}") {
		return ""
	}
	envVar := ref[2 : len(ref)-1]
	return os.Getenv(envVar)
}

// DetectDriver auto-detects the database driver from the connection string.
func DetectDriver(connString string) (string, error) {
	connStringLower := strings.ToLower(connString)

	// Check URL scheme
	if strings.HasPrefix(connStringLower, "postgres://") || strings.HasPrefix(connStringLower, "postgresql://") {
		return DriverPostgres, nil
	}
	if strings.HasPrefix(connStringLower, "file:") || strings.HasSuffix(connStringLower, ".db") || strings.HasSuffix(connStringLower, ".sqlite") {
		return DriverSQLite, nil
	}
	// MySQL DSN format: user:pass@tcp(host:port)/db or user:pass@unix(/path)/db
	if strings.Contains(connStringLower, "@tcp(") || strings.Contains(connStringLower, "@unix(") {
		return DriverMySQL, nil
	}
	// MySQL DSN without explicit protocol
	if matched, _ := regexp.MatchString(`^[^:]+:[^@]+@[^/]+/`, connString); matched {
		return DriverMySQL, nil
	}

	return "", fmt.Errorf("%w: cannot detect driver from connection string", ErrUnsupportedDriver)
}

// IsDriverSupported checks if the driver is supported.
func IsDriverSupported(driver string) bool {
	switch driver {
	case DriverPostgres, DriverMySQL, DriverSQLite:
		return true
	default:
		return false
	}
}

// GetDriverName returns the driver name for sql.Open.
// This maps our canonical driver names to the actual driver package names.
func GetDriverName(driver string) string {
	switch driver {
	case DriverPostgres:
		return "pgx" // github.com/jackc/pgx/v5/stdlib
	case DriverMySQL:
		return "mysql" // github.com/go-sql-driver/mysql
	case DriverSQLite:
		return "sqlite" // modernc.org/sqlite
	default:
		return driver
	}
}

// redactedPlaceholder replaces a password wherever a connection string is shown.
const redactedPlaceholder = "[REDACTED]"

// mysqlDSNPassword matches the password of a MySQL DSN, the one supported form
// url.Parse cannot read a userinfo out of.
// Format: user:pass@tcp(host:port)/db
var mysqlDSNPassword = regexp.MustCompile(`^([^:]+):([^@]+)@`)

// dsnSecretKeyValue matches a secret carried as a key=value pair, whether in a
// libpq DSN (`password=s3cret dbname=db`), a query string (`?password=s3cret`),
// or a sqlite URI (`?_pragma_key=…`). libpq allows quoted values, so those are
// matched whole rather than cut at the first space.
//
// A regexp rather than a DSN parser: each driver has its own (pgconn, mysql,
// modernc/sqlite), none of them masks, and running the right one per driver
// would mean re-detecting and re-parsing a string we only want to display.
var dsnSecretKeyValue = regexp.MustCompile(`(?i)\b(password|passwd|pwd|_pragma_key|_auth_pass|_auth_crypt)=('[^']*'|"[^"]*"|[^\s&;]*)`)

// SanitizeConnectionString removes sensitive information from the connection
// string, for logs and for the dry-run preview.
func SanitizeConnectionString(connString string) string {
	masked := connString

	// Try parsing as URL
	if u, err := url.Parse(connString); err == nil && u.User != nil {
		if _, hasPassword := u.User.Password(); hasPassword {
			// Mask password in URL
			u.User = url.UserPassword(u.User.Username(), redactedPlaceholder)
			// url.String percent-encodes the userinfo, turning the placeholder into
			// %5BREDACTED%5D. Undo it on the placeholder only — this preview is read
			// by a human, and the encoded form reads like part of the password.
			masked = strings.Replace(u.String(), url.PathEscape(redactedPlaceholder), redactedPlaceholder, 1)
		}
		// A DSN url.Parse read a userinfo out of never needs the MySQL fallback:
		// its password, if it has one, is masked above. Applying the fallback
		// anyway would turn a passwordless `postgres://user@host/db` into
		// `postgres:[REDACTED]@...` — a password the DSN does not even carry.
		return dsnSecretKeyValue.ReplaceAllString(masked, "$1="+redactedPlaceholder)
	}

	masked = mysqlDSNPassword.ReplaceAllString(masked, "$1:"+redactedPlaceholder+"@")
	return dsnSecretKeyValue.ReplaceAllString(masked, "$1="+redactedPlaceholder)
}

// PlaceholderStyle represents the SQL parameter placeholder style.
type PlaceholderStyle int

const (
	// PlaceholderQuestion uses ? placeholders (MySQL, SQLite)
	PlaceholderQuestion PlaceholderStyle = iota
	// PlaceholderDollar uses $1, $2 placeholders (PostgreSQL)
	PlaceholderDollar
)

// GetPlaceholderStyle returns the parameter placeholder style for the driver.
func GetPlaceholderStyle(driver string) PlaceholderStyle {
	switch driver {
	case DriverPostgres:
		return PlaceholderDollar
	default:
		return PlaceholderQuestion
	}
}
