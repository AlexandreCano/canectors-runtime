package config

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
)

// envVarPattern matches a ${VAR_NAME} reference. The name must match the
// documented [A-Z_][A-Z0-9_]* shape, which keeps `{{ record.x }}` record
// templates and lowercase shell-style expansions untouched.
var envVarPattern = regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)\}`)

// envSubstSkippedKeys lists configuration keys whose ${VAR} reference is
// resolved elsewhere and must therefore stay literal. connectionStringRef is
// resolved by the database layer (database.ResolveConnectionString) and its
// schema pattern requires the literal ${ENV_VAR} form.
var envSubstSkippedKeys = map[string]struct{}{
	"connectionStringRef": {},
}

// SubstituteEnvVars replaces ${VAR} references in every string value of a
// parsed configuration with the matching environment variable. It runs once at
// startup, after schema validation and before the pipeline is built, so the
// runtime only ever sees resolved values.
//
// Only string values are substituted; numbers, booleans and structural nodes
// are left as-is. A reference that is unset or empty is an error: secrets must
// fail fast rather than silently reaching a remote system as a literal
// placeholder. The returned error names the offending variables and never
// includes any resolved value.
func SubstituteEnvVars(data map[string]any) error {
	var missing []string
	substituteMapEnv(data, &missing)
	if len(missing) == 0 {
		return nil
	}
	slices.Sort(missing)
	missing = slices.Compact(missing)
	return fmt.Errorf(
		"unresolved environment variable(s): %s — set them before running, or remove the ${...} reference",
		strings.Join(missing, ", "),
	)
}

func substituteMapEnv(m map[string]any, missing *[]string) {
	for key, value := range m {
		if _, skip := envSubstSkippedKeys[key]; skip {
			continue
		}
		m[key] = substituteValueEnv(value, missing)
	}
}

func substituteValueEnv(value any, missing *[]string) any {
	switch typed := value.(type) {
	case string:
		return substituteStringEnv(typed, missing)
	case map[string]any:
		substituteMapEnv(typed, missing)
		return typed
	case []any:
		for i, item := range typed {
			typed[i] = substituteValueEnv(item, missing)
		}
		return typed
	default:
		return value
	}
}

func substituteStringEnv(value string, missing *[]string) string {
	if !strings.Contains(value, "${") {
		return value
	}
	return envVarPattern.ReplaceAllStringFunc(value, func(match string) string {
		name := envVarPattern.FindStringSubmatch(match)[1]
		resolved := os.Getenv(name)
		if resolved == "" {
			*missing = append(*missing, name)
			// Keep the reference so nothing half-resolved leaks downstream; the
			// caller aborts on the returned error anyway.
			return match
		}
		return resolved
	})
}
