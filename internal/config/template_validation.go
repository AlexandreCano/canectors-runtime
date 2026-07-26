package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/cannectors/runtime/internal/sqltemplate"
	"github.com/cannectors/runtime/internal/template"
)

// templateValidationEngine compiles templates during config validation. Shared
// package-level instance so repeated validations reuse the compile cache.
var templateValidationEngine = template.NewEngine()

// ValidateTemplates statically checks every template in a parsed pipeline
// configuration without instantiating modules (no connections, no env
// resolution). Two passes:
//
//  1. Generic Jinja syntax: every string value containing {{ or {% is
//     compiled. Contextual escaping and strictness are runtime concerns; this
//     pass only proves the template parses.
//  2. SQL modules (database input/output, sql_call): the query template and
//     its parameters list are compiled together, enforcing $N/parameters
//     consistency and expr syntax.
//
// Returned errors use the same ValidationError shape as schema validation so
// the CLI prints them uniformly.
func ValidateTemplates(data map[string]any) []ValidationError {
	var errs []ValidationError

	walkTemplateStrings(data, "", &errs)

	if input, ok := data["input"].(map[string]any); ok {
		validateSQLModuleTemplates(input, "/input", &errs)
	}
	if filters, ok := data["filters"].([]any); ok {
		for i, f := range filters {
			if m, ok := f.(map[string]any); ok {
				validateSQLModuleTemplates(m, fmt.Sprintf("/filters/%d", i), &errs)
			}
		}
	}
	if output, ok := data["output"].(map[string]any); ok {
		validateSQLModuleTemplates(output, "/output", &errs)
	}

	return errs
}

// walkTemplateStrings recursively compiles every string that looks like a
// Jinja template ({{ variable }} or {% block %} markers).
func walkTemplateStrings(node any, path string, errs *[]ValidationError) {
	switch v := node.(type) {
	case string:
		if !strings.Contains(v, "{{") && !strings.Contains(v, "{%") {
			return
		}
		if err := templateValidationEngine.Validate(v, template.TargetText, false); err != nil {
			*errs = append(*errs, ValidationError{
				Path:    path,
				Type:    "template",
				Message: fmt.Sprintf("invalid template: %v", err),
			})
		}
	case map[string]any:
		for key, val := range v {
			walkTemplateStrings(val, path+"/"+key, errs)
		}
	case []any:
		for i, item := range v {
			walkTemplateStrings(item, fmt.Sprintf("%s/%d", path, i), errs)
		}
	}
}

// validateSQLModuleTemplates compiles the query + parameters pair of SQL-backed
// modules, enforcing $N/parameters consistency. The driver only affects
// placeholder rendering, not validation, so a fixed one is used.
func validateSQLModuleTemplates(module map[string]any, path string, errs *[]ValidationError) {
	moduleType, _ := module["type"].(string)
	if moduleType != "database" && moduleType != "sql_call" {
		return
	}

	query, _ := module["query"].(string)
	if query == "" {
		// queryFile: read it so the $N/parameters check covers external queries
		// too. Modules resolve the path against the process working directory
		// (see loadQueryFromFile), so validating the same way reports exactly
		// what `run` would hit.
		queryFile, _ := module["queryFile"].(string)
		if queryFile == "" {
			return
		}
		content, err := os.ReadFile(queryFile)
		if err != nil {
			*errs = append(*errs, ValidationError{
				Path:    path + "/queryFile",
				Type:    "sql-template",
				Message: fmt.Sprintf("cannot read query file (paths are resolved from the working directory): %v", err),
			})
			return
		}
		query = string(content)
	}

	var params []string
	if raw, ok := module["parameters"].([]any); ok {
		for _, p := range raw {
			if s, ok := p.(string); ok {
				params = append(params, s)
			}
		}
	}

	if _, err := sqltemplate.Compile(templateValidationEngine, query, params, "postgres"); err != nil {
		*errs = append(*errs, ValidationError{
			Path:    path,
			Type:    "sql-template",
			Message: fmt.Sprintf("invalid SQL query template: %v", err),
		})
	}
}
