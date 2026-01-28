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

// Open creates a new database connection pool with the given configuration.
// Uses the standard database/sql package for connection pooling.
func Open(cfg Config) (*sql.DB, string, error) {
	// Resolve connection string from environment variable if specified
	connString, err := ResolveConnectionString(cfg.ConnectionString, cfg.ConnectionStringRef)
	if err != nil {
		return nil, "", err
	}

	// Detect driver if not specified
	driver := cfg.Driver
	if driver == "" {
		driver, err = DetectDriver(connString)
		if err != nil {
			return nil, "", err
		}
	}

	// Validate driver is supported
	if !IsDriverSupported(driver) {
		return nil, "", fmt.Errorf("%w: %s", ErrUnsupportedDriver, driver)
	}

	// Get the actual driver name for sql.Open
	driverName := GetDriverName(driver)

	// Open database connection
	db, err := sql.Open(driverName, connString)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrConnectionFailed, err)
	}

	// Configure connection pool
	applyPoolConfig(db, cfg)

	// Test connection with timeout
	timeout := cfg.ConnectTimeout
	if timeout <= 0 {
		timeout = DefaultConnectTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, "", fmt.Errorf("%w: ping failed: %w", ErrConnectionFailed, err)
	}

	logger.Debug("database connection established",
		"driver", driver,
		"max_open_conns", cfg.MaxOpenConns,
		"max_idle_conns", cfg.MaxIdleConns,
	)

	// Log connection pool stats for monitoring
	stats := db.Stats()
	logger.Debug("database connection pool stats",
		"driver", driver,
		"open_connections", stats.OpenConnections,
		"in_use", stats.InUse,
		"idle", stats.Idle,
		"wait_count", stats.WaitCount,
		"wait_duration", stats.WaitDuration,
	)

	return db, driver, nil
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

// SanitizeConnectionString removes sensitive information from the connection string for logging.
func SanitizeConnectionString(connString string) string {
	// Try parsing as URL
	u, err := url.Parse(connString)
	if err == nil && u.User != nil {
		// Mask password in URL
		u.User = url.UserPassword(u.User.Username(), "[REDACTED]")
		return u.String()
	}

	// For MySQL DSN format, mask password
	// Format: user:pass@tcp(host:port)/db
	re := regexp.MustCompile(`^([^:]+):([^@]+)@`)
	return re.ReplaceAllString(connString, "$1:[REDACTED]@")
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

// FormatPlaceholder formats a parameter placeholder for the given index and driver.
// Index is 1-based for consistency across drivers.
func FormatPlaceholder(driver string, index int) string {
	switch GetPlaceholderStyle(driver) {
	case PlaceholderDollar:
		return fmt.Sprintf("$%d", index)
	default:
		return "?"
	}
}

// ConvertPlaceholders converts a query with ? placeholders to the driver's style.
// This allows writing queries with ? placeholders and converting them as needed.
func ConvertPlaceholders(query string, driver string) string {
	if GetPlaceholderStyle(driver) == PlaceholderQuestion {
		return query
	}

	// Convert ? to $1, $2, etc.
	result := query
	paramIndex := 1
	for strings.Contains(result, "?") {
		result = strings.Replace(result, "?", fmt.Sprintf("$%d", paramIndex), 1)
		paramIndex++
	}
	return result
}
