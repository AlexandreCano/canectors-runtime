package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/cannectors/runtime/pkg/connector"
)

func TestCountLines(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 1},
		{"single line", "hello", 1},
		{"two lines", "a\nb", 2},
		{"trailing newline", "a\nb\n", 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := countLines(tc.in); got != tc.want {
				t.Errorf("countLines(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty returns nil", "", nil},
		{"single line", "hello", []string{"hello"}},
		{"two lines", "a\nb", []string{"a", "b"}},
		{"trailing newline stripped", "a\nb\n", []string{"a", "b"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitLines(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("splitLines(%q) = %v (len %d), want %v (len %d)", tc.in, got, len(got), tc.want, len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("splitLines(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
				}
			}
		})
	}
}

// captureStdout runs fn with os.Stdout redirected and returns what it printed.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = original }()

	fn()
	if closeErr := w.Close(); closeErr != nil {
		t.Fatalf("closing pipe: %v", closeErr)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading pipe: %v", err)
	}
	return string(out)
}

func sqlPreviewFixture(rows [][]string, omitted int) []connector.OperationPreview {
	return []connector.OperationPreview{
		{
			Kind:        connector.PreviewKindSQL,
			RecordCount: len(rows) + omitted,
			SQL: &connector.SQLPreview{
				Driver:      "postgres",
				Target:      "postgres://lab_user:[REDACTED]@warehouse.internal:5432/analytics",
				Operation:   "INSERT INTO",
				Table:       "events",
				Statement:   "INSERT INTO events (id, seq) VALUES ($1, $2)",
				Parameters:  []string{"record.id", "record.seq"},
				Rows:        rows,
				RowsOmitted: omitted,
				Transaction: true,
			},
		},
	}
}

// TestPrintDryRunPreview_SQL locks Story 25.6 AC1: the statement, its
// parameters and the bound values all reach the user.
func TestPrintDryRunPreview_SQL(t *testing.T) {
	previews := sqlPreviewFixture([][]string{{`"EVT-1"`, "1"}}, 0)

	out := captureStdout(t, func() { PrintDryRunPreview(previews, false) })

	for _, want := range []string{
		"INSERT INTO events (postgres)",
		"warehouse.internal",
		"[REDACTED]",
		"INSERT INTO events (id, seq) VALUES ($1, $2)",
		"record.id, record.seq",
		`"EVT-1", 1`,
		"Transaction: all statements in one transaction",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("preview output missing %q:\n%s", want, out)
		}
	}
}

// TestPrintDryRunPreview_SQLTruncatesRows locks Story 25.6 AC3/AC5: a batch is
// readable by default and complete under --verbose.
func TestPrintDryRunPreview_SQLTruncatesRows(t *testing.T) {
	rows := make([][]string, 12)
	for i := range rows {
		rows[i] = []string{fmt.Sprintf("%q", fmt.Sprintf("EVT-%d", i+1)), strconv.Itoa(i + 1)}
	}
	previews := sqlPreviewFixture(rows, 88)

	compact := captureStdout(t, func() { PrintDryRunPreview(previews, false) })
	if !strings.Contains(compact, "Values (truncated, use --verbose for all):") {
		t.Errorf("compact output does not announce the truncation:\n%s", compact)
	}
	if strings.Contains(compact, `"EVT-6"`) {
		t.Errorf("compact output printed more than the first rows:\n%s", compact)
	}
	// 7 rows not shown here + 88 the module could not carry.
	if !strings.Contains(compact, "(95 more executions)") {
		t.Errorf("compact output miscounts the hidden executions:\n%s", compact)
	}

	verbose := captureStdout(t, func() { PrintDryRunPreview(previews, true) })
	if !strings.Contains(verbose, `"EVT-12"`) {
		t.Errorf("verbose output should list every carried row:\n%s", verbose)
	}
	if !strings.Contains(verbose, "(88 more executions)") {
		t.Errorf("verbose output should still report the module-level cap:\n%s", verbose)
	}
}

// TestPrintDryRunPreview_SQLTruncatesWideValues keeps one wide column from
// pushing the rest of the row off screen.
func TestPrintDryRunPreview_SQLTruncatesWideValues(t *testing.T) {
	wide := strings.Repeat("x", 200)
	previews := sqlPreviewFixture([][]string{{wide, "1"}}, 0)

	compact := captureStdout(t, func() { PrintDryRunPreview(previews, false) })
	if !strings.Contains(compact, "…") {
		t.Errorf("wide value was not truncated:\n%s", compact)
	}
	verbose := captureStdout(t, func() { PrintDryRunPreview(previews, true) })
	if !strings.Contains(verbose, wide) {
		t.Error("verbose output should print the full value")
	}
}

// TestPrintDryRunPreview_SQLTruncatesWideAccentedValues keeps the truncation on
// rune boundaries: cutting record data mid-rune prints a replacement character.
func TestPrintDryRunPreview_SQLTruncatesWideAccentedValues(t *testing.T) {
	wide := strings.Repeat("é", 200)
	previews := sqlPreviewFixture([][]string{{wide, "1"}}, 0)

	compact := captureStdout(t, func() { PrintDryRunPreview(previews, false) })
	if strings.ContainsRune(compact, utf8.RuneError) {
		t.Errorf("truncation cut a rune in half:\n%s", compact)
	}
	if !strings.Contains(compact, strings.Repeat("é", 60)+"…") {
		t.Errorf("value was not truncated at 60 runes:\n%s", compact)
	}
}

// TestPrintDryRunPreview_TruncatesOperations keeps a single-record-mode pipeline
// from printing one operation per record (Story 25.6 AC5).
func TestPrintDryRunPreview_TruncatesOperations(t *testing.T) {
	previews := make([]connector.OperationPreview, 0, 25)
	for i := 0; i < 25; i++ {
		previews = append(previews, sqlPreviewFixture([][]string{{fmt.Sprintf("%q", fmt.Sprintf("EVT-%d", i+1)), strconv.Itoa(i + 1)}}, 0)[0])
	}

	compact := captureStdout(t, func() { PrintDryRunPreview(previews, false) })
	if !strings.Contains(compact, "Operation 10 of 25") {
		t.Errorf("compact output should print the first %d operations:\n%s", maxOperationsCompact, compact)
	}
	if strings.Contains(compact, "Operation 11 of 25") {
		t.Errorf("compact output printed past the cap:\n%s", compact)
	}
	if !strings.Contains(compact, "(15 more operations, use --verbose for all)") {
		t.Errorf("compact output does not report the hidden operations:\n%s", compact)
	}

	verbose := captureStdout(t, func() { PrintDryRunPreview(previews, true) })
	if !strings.Contains(verbose, "Operation 25 of 25") {
		t.Errorf("verbose output should print every operation:\n%s", verbose)
	}
	if strings.Contains(verbose, "more operations") {
		t.Errorf("verbose output should hide nothing:\n%s", verbose)
	}
}

// TestPrintDryRunPreviewFailed reports why the preview is missing instead of
// printing a request the module never built.
func TestPrintDryRunPreviewFailed(t *testing.T) {
	out := captureStdout(t, func() { PrintDryRunPreviewFailed("evaluating parameter 1 (\"record.id\"): unknown name record") })
	if !strings.Contains(out, "failed to describe what it would do") {
		t.Errorf("output does not say the preview failed:\n%s", out)
	}
	if !strings.Contains(out, "unknown name record") {
		t.Errorf("output does not carry the reason:\n%s", out)
	}
	if strings.Contains(out, "Endpoint") {
		t.Errorf("a failed preview must not look like an HTTP request:\n%s", out)
	}
}

// TestPrintDryRunPreviewUnsupported locks Story 25.6 AC6: the module is named,
// so an empty preview is not read as "nothing would happen".
func TestPrintDryRunPreviewUnsupported(t *testing.T) {
	out := captureStdout(t, func() { PrintDryRunPreviewUnsupported("customSink") })
	if !strings.Contains(out, `"customSink"`) {
		t.Errorf("output does not name the module:\n%s", out)
	}
	if !strings.Contains(out, "cannot describe what it would do") {
		t.Errorf("output does not say why nothing is shown:\n%s", out)
	}
}
