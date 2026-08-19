package database

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestResolveEnvRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ref      string
		envVar   string
		envValue string
		want     string
	}{
		{
			name:     "valid env ref",
			ref:      "${TEST_DB_CONN}",
			envVar:   "TEST_DB_CONN",
			envValue: "postgres://user:pass@host:5432/db",
			want:     "postgres://user:pass@host:5432/db",
		},
		{
			name:   "invalid format - no prefix",
			ref:    "TEST_DB_CONN}",
			envVar: "TEST_DB_CONN",
			want:   "",
		},
		{
			name:   "invalid format - no suffix",
			ref:    "${TEST_DB_CONN",
			envVar: "TEST_DB_CONN",
			want:   "",
		},
		{
			name:   "env var not set",
			ref:    "${NONEXISTENT_VAR_12345}",
			envVar: "",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVar != "" && tt.envValue != "" {
				_ = os.Setenv(tt.envVar, tt.envValue)
				t.Cleanup(func() { _ = os.Unsetenv(tt.envVar) })
			}

			got := ResolveEnvRef(tt.ref)
			if got != tt.want {
				t.Errorf("ResolveEnvRef(%q) = %q, want %q", tt.ref, got, tt.want)
			}
		})
	}
}

func TestDetectDriver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		connString string
		want       string
		wantErr    bool
	}{
		{
			name:       "postgres URL",
			connString: "postgres://user:pass@localhost:5432/mydb",
			want:       DriverPostgres,
		},
		{
			name:       "postgresql URL",
			connString: "postgresql://user:pass@localhost:5432/mydb?sslmode=disable",
			want:       DriverPostgres,
		},
		{
			name:       "postgres URL with options",
			connString: "postgres://user:pass@localhost:5432/mydb?sslmode=require",
			want:       DriverPostgres,
		},
		{
			name:       "mysql tcp DSN",
			connString: "user:password@tcp(localhost:3306)/mydb",
			want:       DriverMySQL,
		},
		{
			name:       "mysql unix DSN",
			connString: "user:password@unix(/var/run/mysql.sock)/mydb",
			want:       DriverMySQL,
		},
		{
			name:       "sqlite file URL",
			connString: "file:./test.db",
			want:       DriverSQLite,
		},
		{
			name:       "sqlite .db extension",
			connString: "./data/mydb.db",
			want:       DriverSQLite,
		},
		{
			name:       "sqlite .sqlite extension",
			connString: "/path/to/database.sqlite",
			want:       DriverSQLite,
		},
		{
			name:       "unknown format",
			connString: "unknown://something",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetectDriver(tt.connString)
			if (err != nil) != tt.wantErr {
				t.Errorf("DetectDriver(%q) error = %v, wantErr %v", tt.connString, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("DetectDriver(%q) = %q, want %q", tt.connString, got, tt.want)
			}
		})
	}
}

func TestIsDriverSupported(t *testing.T) {
	t.Parallel()

	tests := []struct {
		driver string
		want   bool
	}{
		{DriverPostgres, true},
		{DriverMySQL, true},
		{DriverSQLite, true},
		{"oracle", false},
		{"sqlserver", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.driver, func(t *testing.T) {
			if got := IsDriverSupported(tt.driver); got != tt.want {
				t.Errorf("IsDriverSupported(%q) = %v, want %v", tt.driver, got, tt.want)
			}
		})
	}
}

// TestSanitizeConnectionStringSecrets covers the DSN forms whose secret is not
// a URL userinfo — a libpq key=value pair, a query parameter, a sqlite pragma
// key — plus the passwordless URL the masking must not invent a password for.
// These strings reach the user's terminal through the dry-run preview, not just
// the debug log.
func TestSanitizeConnectionStringSecrets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		connString string
		want       string
	}{
		{
			name:       "libpq key=value DSN",
			connString: "host=db.internal port=5432 user=app password=s3cr3t dbname=warehouse",
			want:       "host=db.internal port=5432 user=app password=[REDACTED] dbname=warehouse",
		},
		{
			name:       "libpq DSN with quoted password",
			connString: "host=db.internal user=app password='s3c r3t' dbname=warehouse",
			want:       "host=db.internal user=app password=[REDACTED] dbname=warehouse",
		},
		{
			name:       "password as a query parameter",
			connString: "postgres://app@db.internal:5432/warehouse?password=s3cr3t&sslmode=require",
			want:       "postgres://app@db.internal:5432/warehouse?password=[REDACTED]&sslmode=require",
		},
		{
			name:       "sqlite encryption key",
			connString: "file:./warehouse.db?_pragma_key=s3cr3t&_pragma=busy_timeout(5000)",
			want:       "file:./warehouse.db?_pragma_key=[REDACTED]&_pragma=busy_timeout(5000)",
		},
		{
			name:       "URL with both userinfo and query password",
			connString: "postgres://app:urlpass@db.internal/warehouse?passwd=querypass",
			want:       "postgres://app:[REDACTED]@db.internal/warehouse?passwd=[REDACTED]",
		},
		{
			name:       "URL without a password is left alone",
			connString: "postgres://app@db.internal:5432/warehouse?sslmode=verify-full",
			want:       "postgres://app@db.internal:5432/warehouse?sslmode=verify-full",
		},
		{
			name:       "sqlite path carries no secret",
			connString: "file:./warehouse.db",
			want:       "file:./warehouse.db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := SanitizeConnectionString(tt.connString); got != tt.want {
				t.Errorf("SanitizeConnectionString(%q) = %q, want %q", tt.connString, got, tt.want)
			}
		})
	}
}

// TestOpenResolvedHonoursContext locks the cancellation path: the connect
// timeout bounds the caller's context instead of replacing it, so a canceled
// run stops waiting instead of sitting out DefaultConnectTimeout.
func TestOpenResolvedHonoursContext(t *testing.T) {
	t.Parallel()

	resolved, err := Resolve(Config{ConnectionString: "postgres://app:pass@192.0.2.1:5432/warehouse"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	db, err := OpenResolved(ctx, Config{}, resolved)
	if err == nil {
		_ = db.Close()
		t.Fatal("OpenResolved succeeded with a canceled context")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("OpenResolved waited %s on a canceled context, want an immediate return", elapsed)
	}
}

func TestSanitizeConnectionString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		connString string
		wantPrefix string
	}{
		{
			name:       "postgres URL with password",
			connString: "postgres://user:secretpass@localhost:5432/mydb",
			wantPrefix: "postgres://user:",
		},
		{
			name:       "postgres URL with special chars in password",
			connString: "postgres://user:p%40ssw0rd@localhost:5432/mydb",
			wantPrefix: "postgres://user:",
		},
		{
			name:       "mysql DSN with password",
			connString: "user:password@tcp(localhost:3306)/mydb",
			wantPrefix: "user:[REDACTED]@",
		},
		{
			name:       "sqlite path (no password)",
			connString: "file:./test.db",
			wantPrefix: "file:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeConnectionString(tt.connString)
			// For postgres URLs, verify password is redacted
			if tt.name != "sqlite path (no password)" && tt.name != "mysql DSN with password" {
				if !strings.Contains(got, "REDACTED") {
					t.Errorf("SanitizeConnectionString(%q) = %q, expected REDACTED in output", tt.connString, got)
				}
			}
			if !strings.HasPrefix(got, tt.wantPrefix) {
				t.Errorf("SanitizeConnectionString(%q) = %q, expected prefix %q", tt.connString, got, tt.wantPrefix)
			}
		})
	}
}

func TestGetPlaceholderStyle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		driver string
		want   PlaceholderStyle
	}{
		{DriverPostgres, PlaceholderDollar},
		{DriverMySQL, PlaceholderQuestion},
		{DriverSQLite, PlaceholderQuestion},
	}

	for _, tt := range tests {
		t.Run(tt.driver, func(t *testing.T) {
			if got := GetPlaceholderStyle(tt.driver); got != tt.want {
				t.Errorf("GetPlaceholderStyle(%q) = %v, want %v", tt.driver, got, tt.want)
			}
		})
	}
}

func TestResolveConnectionString(t *testing.T) {
	t.Parallel()

	t.Run("env ref takes precedence", func(t *testing.T) {
		_ = os.Setenv("TEST_CONN_REF", "postgres://from-env")
		t.Cleanup(func() { _ = os.Unsetenv("TEST_CONN_REF") })

		got, err := ResolveConnectionString("postgres://from-string", "${TEST_CONN_REF}")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "postgres://from-env" {
			t.Errorf("got %q, want postgres://from-env", got)
		}
	})

	t.Run("falls back to connection string", func(t *testing.T) {
		got, err := ResolveConnectionString("postgres://from-string", "${NONEXISTENT_VAR_XYZ}")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "postgres://from-string" {
			t.Errorf("got %q, want postgres://from-string", got)
		}
	})

	t.Run("error when no connection string", func(t *testing.T) {
		_, err := ResolveConnectionString("", "")
		if !errors.Is(err, ErrMissingConnectionString) {
			t.Errorf("got error %v, want ErrMissingConnectionString", err)
		}
	})
}

func TestGetDriverName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		driver string
		want   string
	}{
		{DriverPostgres, "pgx"},
		{DriverMySQL, "mysql"},
		{DriverSQLite, "sqlite"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.driver, func(t *testing.T) {
			if got := GetDriverName(tt.driver); got != tt.want {
				t.Errorf("GetDriverName(%q) = %q, want %q", tt.driver, got, tt.want)
			}
		})
	}
}
