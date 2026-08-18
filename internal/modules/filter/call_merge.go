package filter

import (
	"fmt"
	"strings"

	"github.com/cannectors/runtime/internal/errhandling"
)

// The three call filters (http_call, soap_call, sql_call) share one merge
// contract: mergeStrategy says how the response is combined with the record,
// and resultKey names the destination when the response is nested whole.
// Story 25.2 keeps that contract in a single place so the verbs cannot drift
// apart again between modules.
const (
	mergeStrategyMerge   = "merge"
	mergeStrategyReplace = "replace"
	mergeStrategyAppend  = "append"

	// defaultCallMergeStrategy applies when mergeStrategy is omitted.
	defaultCallMergeStrategy = mergeStrategyMerge
)

// resolveCallMergeContract normalizes the merge strategy and resolves the
// destination key used by the append strategy. The append strategy has no
// default destination for any of the three filters: resultKey is mandatory,
// at validation time and at construction time alike. The returned key is
// trimmed and validated against the reserved record paths, since it writes
// straight into the record.
func resolveCallMergeContract(moduleType, strategy, resultKey string) (string, string, error) {
	normalized, err := normalizeCallMergeStrategy(moduleType, strategy)
	if err != nil {
		return "", "", err
	}

	key := strings.TrimSpace(resultKey)
	if normalized == mergeStrategyAppend && key == "" {
		return "", "", fmt.Errorf("%s: resultKey is required when mergeStrategy is append", moduleType)
	}
	// resultKey writes straight into the record, so it can reach the reserved
	// error list just as set and mapping can — and overwriting it would let a
	// pipeline erase the runtime's own trace of what failed.
	if reservedErr := errhandling.ValidateRecordTarget(key); reservedErr != nil {
		return "", "", fmt.Errorf("%s: %w", moduleType, reservedErr)
	}
	return normalized, key, nil
}

// normalizeCallMergeStrategy validates the merge strategy strictly. The schema
// enum already restricts pipeline authors, but this is the runtime safety net
// for configs built in-process (tests, registry callers).
func normalizeCallMergeStrategy(moduleType, strategy string) (string, error) {
	switch strategy {
	case "":
		return defaultCallMergeStrategy, nil
	case mergeStrategyMerge, mergeStrategyReplace, mergeStrategyAppend:
		return strategy, nil
	default:
		return "", fmt.Errorf("%s: invalid mergeStrategy %q (expected one of: merge, replace, append)", moduleType, strategy)
	}
}
