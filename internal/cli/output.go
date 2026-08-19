// Package cli provides CLI output formatting and display functions.
package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/cannectors/runtime/pkg/connector"
)

// OutputOptions configures CLI output behavior.
type OutputOptions struct {
	Verbose bool
	Quiet   bool
	DryRun  bool
}

// PrintExecutionResult displays the pipeline execution result.
func PrintExecutionResult(result *connector.ExecutionResult, err error, opts OutputOptions) {
	if result == nil {
		fmt.Fprintln(os.Stderr, "✗ No execution result available")
		return
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ Pipeline execution failed")
		if result.Error != nil {
			fmt.Fprintf(os.Stderr, "  Module: %s\n", result.Error.Module)
			fmt.Fprintf(os.Stderr, "  Error: %s\n", result.Error.Message)
		}
		return
	}

	if !opts.Quiet {
		fmt.Println("✓ Pipeline executed successfully")
		fmt.Printf("  Status: %s\n", result.Status)
		fmt.Printf("  Records processed: %d\n", result.RecordsProcessed)
		if result.RecordsFailed > 0 {
			fmt.Printf("  Records failed: %d\n", result.RecordsFailed)
		}
		printErrorCounts(result.ErrorCounts)
		if opts.Verbose {
			fmt.Printf("  Duration: %v\n", result.CompletedAt.Sub(result.StartedAt))
		}

		if opts.DryRun {
			switch {
			case len(result.DryRunPreview) > 0:
				PrintDryRunPreview(result.DryRunPreview, opts.Verbose)
			case result.DryRunPreviewUnsupported != "":
				PrintDryRunPreviewUnsupported(result.DryRunPreviewUnsupported)
			case result.DryRunPreviewError != "":
				PrintDryRunPreviewFailed(result.DryRunPreviewError)
			}
		}
	}
}

// printErrorCounts reports the failures that modules in onError: log mode let
// through, broken down by category.
//
// Without it a run where every record failed its enrichment still prints
// "✓ Pipeline executed successfully" and nothing else — technically true, and
// the most misleading thing the CLI could say.
//
// The total counts markers, not records: one record that failed an enrichment
// call and then a persistence call contributes two. Hence "Error markers" —
// calling them records would overstate how many records are affected.
//
// Categories are sorted so the summary is stable between runs: map iteration
// order would otherwise reshuffle the lines and make two runs look different
// when they are not.
func printErrorCounts(counts map[string]int) {
	if len(counts) == 0 {
		return
	}

	categories := make([]string, 0, len(counts))
	total := 0
	for category, n := range counts {
		categories = append(categories, category)
		total += n
	}
	sort.Strings(categories)

	fmt.Printf("  Error markers: %d\n", total)
	for _, category := range categories {
		fmt.Printf("    %s: %d\n", category, counts[category])
	}
}

// maxOperationsCompact caps how many operations are printed without --verbose.
const maxOperationsCompact = 10

// PrintDryRunPreview displays the operations a dry-run would have performed.
func PrintDryRunPreview(previews []connector.OperationPreview, verbose bool) {
	fmt.Println()
	fmt.Println("📋 Dry-Run Preview (what would have been done):")
	fmt.Println()

	// A pipeline in single-record mode — or one whose statement renders
	// differently per record — produces one operation per record. Show a
	// scannable head by default and the whole list under --verbose, the same
	// deal as the body and the bound values.
	shown := len(previews)
	if !verbose && shown > maxOperationsCompact {
		shown = maxOperationsCompact
	}

	for i := 0; i < shown; i++ {
		preview := previews[i]
		if len(previews) > 1 {
			fmt.Printf("─── Operation %d of %d ───\n", i+1, len(previews))
		}

		switch {
		case preview.SQL != nil:
			printSQLPreview(preview, verbose)
		case preview.HTTP != nil:
			printHTTPPreview(preview, verbose)
		default:
			fmt.Printf("  (module reported a %q preview with no detail)\n", preview.Kind)
		}

		if i < shown-1 {
			fmt.Println()
		}
	}

	if hidden := len(previews) - shown; hidden > 0 {
		fmt.Printf("\n... (%d more operations, use --verbose for all)\n", hidden)
	}

	fmt.Println()
	fmt.Println("ℹ️  Nothing was sent or written to the target system (dry-run mode)")
}

// PrintDryRunPreviewUnsupported tells the user the output module has no preview,
// so an empty dry-run is not mistaken for "nothing would happen".
func PrintDryRunPreviewUnsupported(outputType string) {
	fmt.Println()
	fmt.Printf("📋 Dry-Run Preview: the %q output module cannot describe what it would do.\n", outputType)
	fmt.Println("   The records above went through input and filters; the output was skipped.")
}

// PrintDryRunPreviewFailed reports that the output module could not describe
// what it would do, and why. The dry-run itself still succeeded — nothing was
// written — but the author gets the reason rather than a blank preview.
func PrintDryRunPreviewFailed(reason string) {
	fmt.Println()
	fmt.Println("📋 Dry-Run Preview: the output module failed to describe what it would do.")
	fmt.Printf("   Reason: %s\n", reason)
	fmt.Println("   Nothing was sent or written to the target system (dry-run mode).")
}

// printHTTPPreview renders the request an HTTP-based output would send.
func printHTTPPreview(preview connector.OperationPreview, verbose bool) {
	http := preview.HTTP
	fmt.Printf("  Endpoint: %s %s\n", http.Method, http.Endpoint)
	fmt.Printf("  Records: %d\n", preview.RecordCount)

	if len(http.Headers) > 0 {
		printHeaders(http.Headers)
	}

	if http.BodyPreview != "" {
		printBodyPreview(http.BodyPreview, verbose)
	}
}

// printSQLPreview renders the statement a database output would execute.
//
// The statement is printed once, then the bound values row by row: a batch of
// 500 records executes one statement 500 times, and printing the SQL 500 times
// would bury the one thing the reader is checking.
func printSQLPreview(preview connector.OperationPreview, verbose bool) {
	sqlPreview := preview.SQL
	target := sqlPreview.Operation
	if sqlPreview.Table != "" {
		target += " " + sqlPreview.Table
	}
	fmt.Printf("  Operation: %s (%s)\n", target, sqlPreview.Driver)
	fmt.Printf("  Target: %s\n", sqlPreview.Target)
	fmt.Printf("  Records: %d\n", preview.RecordCount)
	if sqlPreview.Transaction {
		fmt.Println("  Transaction: all statements in one transaction")
	}

	fmt.Println("  Statement:")
	printIndentedBody(sqlPreview.Statement, "    ")

	if len(sqlPreview.Parameters) > 0 {
		fmt.Printf("  Parameters: %s\n", strings.Join(sqlPreview.Parameters, ", "))
	}
	printSQLRows(sqlPreview, verbose)
}

// printSQLRows lists the bound values, one line per execution. Like the HTTP
// body, the module hands over everything it collected and the display decides
// how much to show: compact by default, everything under --verbose.
func printSQLRows(sqlPreview *connector.SQLPreview, verbose bool) {
	if len(sqlPreview.Rows) == 0 {
		return
	}
	const maxRowsCompact = 5
	const maxValueCompact = 60

	shown := len(sqlPreview.Rows)
	if !verbose && shown > maxRowsCompact {
		shown = maxRowsCompact
	}

	if verbose || shown == len(sqlPreview.Rows) {
		fmt.Println("  Values:")
	} else {
		fmt.Println("  Values (truncated, use --verbose for all):")
	}

	for i := 0; i < shown; i++ {
		values := sqlPreview.Rows[i]
		if !verbose {
			values = truncateValues(values, maxValueCompact)
		}
		fmt.Printf("    %d: %s\n", i+1, strings.Join(values, ", "))
	}

	hidden := len(sqlPreview.Rows) - shown + sqlPreview.RowsOmitted
	if hidden > 0 {
		fmt.Printf("    ... (%d more executions)\n", hidden)
	}
}

// truncateValues shortens long bound values so one wide column cannot push the
// rest of the row off screen. Counted in runes: cutting record data mid-rune
// would print a replacement character for any accented value.
func truncateValues(values []string, max int) []string {
	out := make([]string, len(values))
	for i, v := range values {
		if utf8.RuneCountInString(v) > max {
			v = string([]rune(v)[:max]) + "…"
		}
		out[i] = v
	}
	return out
}

// printHeaders prints sorted headers.
func printHeaders(headers map[string]string) {
	fmt.Println("  Headers:")
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Printf("    %s: %s\n", name, headers[name])
	}
}

// printBodyPreview displays the formatted JSON body preview.
func printBodyPreview(bodyPreview string, verbose bool) {
	const maxLinesCompact = 10
	lineCount := countLines(bodyPreview)

	if verbose || lineCount <= maxLinesCompact {
		fmt.Println("  Body:")
		printIndentedBody(bodyPreview, "    ")
		return
	}

	fmt.Println("  Body (truncated, use --verbose for full):")
	printTruncatedBody(bodyPreview, "    ", maxLinesCompact)
}

// printIndentedBody prints the body with indentation.
func printIndentedBody(body string, indent string) {
	lines := splitLines(body)
	for _, line := range lines {
		fmt.Printf("%s%s\n", indent, line)
	}
}

// printTruncatedBody prints the first N lines of the body.
func printTruncatedBody(body string, indent string, maxLines int) {
	lines := splitLines(body)
	for i := 0; i < maxLines && i < len(lines); i++ {
		fmt.Printf("%s%s\n", indent, lines[i])
	}
	if len(lines) > maxLines {
		fmt.Printf("%s... (%d more lines)\n", indent, len(lines)-maxLines)
	}
}

// countLines counts the number of lines in a string.
func countLines(s string) int {
	return strings.Count(s, "\n") + 1
}

// splitLines splits a string into lines.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// PrintConfigSummary prints connector name and version if available.
func PrintConfigSummary(data map[string]any) {
	if data == nil {
		return
	}

	if name, ok := data["name"].(string); ok {
		fmt.Printf("  Connector: %s\n", name)
	}
	if version, ok := data["version"].(string); ok {
		fmt.Printf("  Version: %s\n", version)
	}
}
