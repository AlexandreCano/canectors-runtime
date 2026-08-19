package output

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cannectors/runtime/internal/database"
	"github.com/cannectors/runtime/internal/template"
	"github.com/cannectors/runtime/pkg/connector"
)

// maxPreviewRows caps how many executions a preview carries. A 10 000-record
// batch does not need 10 000 rows in memory to tell the author what the write
// looks like; the count of the rest is reported instead.
const maxPreviewRows = 100

// sqlOperation matches the leading keyword of a statement, and the table it
// writes to for the three shapes that name one plainly.
//
// This is deliberately not a SQL parser: the full statement is shown right
// underneath, so the operation and table only have to make the preview scannable.
// A statement it cannot read yields "STATEMENT" and no table, never a wrong one.
var sqlOperation = regexp.MustCompile(`(?is)^\s*(INSERT\s+INTO|UPDATE|DELETE\s+FROM|REPLACE\s+INTO|MERGE\s+INTO|SELECT|WITH)\s+([a-zA-Z_"` + "`" + `\[][\w.$"` + "`" + `\[\]]*)?`)

// PreviewOperations describes the statements the module would execute, without
// opening a connection.
//
// One preview per distinct rendered statement: a templated query can render
// differently per record ({% if %} branches), and reporting one statement for
// all of them would show the author SQL that is not what half their records
// would run. The bound values are listed per execution underneath.
func (d *DatabaseOutput) PreviewOperations(records []map[string]any, opts PreviewOptions) ([]connector.OperationPreview, error) {
	if len(records) == 0 {
		return []connector.OperationPreview{}, nil
	}

	target := d.resolved.ConnString
	if !opts.ShowCredentials {
		target = database.SanitizeConnectionString(target)
	}

	// Group by rendered statement, preserving the order records introduced them.
	order := make([]string, 0, 1)
	rows := make(map[string][][]string)
	counts := make(map[string]int)
	// Parameter labels are per rendered statement, not per declared parameter:
	// which placeholders survive rendering — and how many arguments each one
	// binds — depends on the statement, so the declared list would mislabel the
	// values below it.
	sources := make(map[string][]string)

	for i, record := range records {
		statement, args, argSources, err := d.sqlQuery.BuildDescribed(template.ContextForRecord(record))
		if err != nil {
			return nil, fmt.Errorf("previewing record %d: %w", i, err)
		}
		if _, seen := counts[statement]; !seen {
			order = append(order, statement)
			sources[statement] = argSources
		}
		counts[statement]++
		if len(rows[statement]) < maxPreviewRows {
			rows[statement] = append(rows[statement], formatSQLArgs(args))
		}
	}

	previews := make([]connector.OperationPreview, 0, len(order))
	for _, statement := range order {
		operation, table := describeStatement(statement)
		previews = append(previews, connector.OperationPreview{
			Kind:        connector.PreviewKindSQL,
			RecordCount: counts[statement],
			SQL: &connector.SQLPreview{
				Driver:      d.resolved.Driver,
				Target:      target,
				Operation:   operation,
				Table:       table,
				Statement:   strings.TrimSpace(statement),
				Parameters:  sources[statement],
				Rows:        rows[statement],
				RowsOmitted: counts[statement] - len(rows[statement]),
				Transaction: d.config.Transaction,
			},
		})
	}
	return previews, nil
}

// describeStatement reads the operation and target table off a statement for the
// preview header. Returns ("STATEMENT", "") when it cannot tell.
func describeStatement(statement string) (operation, table string) {
	match := sqlOperation.FindStringSubmatch(statement)
	if match == nil {
		return "STATEMENT", ""
	}
	operation = strings.ToUpper(strings.Join(strings.Fields(match[1]), " "))
	table = match[2]
	// SELECT and WITH name something other than a write target; leave the table
	// out rather than pass a column list off as one.
	if operation == "SELECT" || operation == "WITH" {
		table = ""
	}
	return operation, table
}

// formatSQLArgs renders bound values for display: quoted strings, NULL for nil,
// and %v for everything else. These are record values, not credentials — the
// connection string is the sensitive part of a SQL preview and is masked
// separately.
func formatSQLArgs(args []any) []string {
	out := make([]string, len(args))
	for i, arg := range args {
		switch value := arg.(type) {
		case nil:
			out[i] = "NULL"
		case string:
			out[i] = fmt.Sprintf("%q", value)
		default:
			out[i] = fmt.Sprintf("%v", value)
		}
	}
	return out
}

// Verify DatabaseOutput implements Module, PreviewableModule and
// ConnectableModule.
var (
	_ Module            = (*DatabaseOutput)(nil)
	_ PreviewableModule = (*DatabaseOutput)(nil)
	_ ConnectableModule = (*DatabaseOutput)(nil)
)
