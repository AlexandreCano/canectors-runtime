package filter

import (
	"errors"
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

// Sentinel errors of the shared merge contract. Both http_call and soap_call
// wrap them, so a caller can route on the cause without knowing which filter
// produced the failure — http_call additionally carries its own error code.
var (
	// ErrCallResultType reports a response that can never enrich a record: a
	// scalar or null where an object or a list was expected.
	ErrCallResultType = errors.New("call result is neither an object nor a list")

	// ErrCallMergeStrategyMismatch reports a response whose shape has no
	// destination under the configured mergeStrategy.
	ErrCallMergeStrategyMismatch = errors.New("call result does not fit the configured mergeStrategy")
)

// callContractError builds a merge-contract failure. The response shape is
// deterministic — replaying the call returns the same body — so the failure is
// a validation error the pipeline routes through onError, never something to
// requeue. The sentinel stays reachable through the chain for errors.Is.
func callContractError(format string, args ...any) error {
	wrapped := fmt.Errorf(format, args...)
	return errhandling.NewValidationError(0, wrapped.Error(), wrapped)
}

// mergeCallResult folds the value returned by a call filter into the record,
// according to the resolved merge contract.
//
// A response is not always an object: a dataField pointing at a list is a very
// common shape (`{"results": [...]}`), and Story 25.3 keeps the list a list
// instead of dropping it. Only the append strategy has a destination able to
// hold one, so merge and replace report the mismatch rather than silently
// enriching the record with nothing.
func mergeCallResult(moduleType, strategy, resultKey string, record map[string]any, data any) (map[string]any, error) {
	switch value := data.(type) {
	case nil:
		return nil, callContractError(
			"%s: %w: the response holds null, which cannot enrich a record — check dataField",
			moduleType, ErrCallResultType)
	case map[string]any:
		return mergeCallMap(strategy, resultKey, record, value), nil
	case []any:
		return appendCallList(moduleType, strategy, resultKey, record, value)
	default:
		return nil, callContractError(
			"%s: %w: the response holds %T — check dataField",
			moduleType, ErrCallResultType, data)
	}
}

// mergeCallMap applies the three strategies to an object response. The response
// is cloned first: with caching enabled it is the very object the LRU holds, so
// handing it to the record would let a downstream filter mutating the record in
// place rewrite the cache entry and every other record sharing it.
func mergeCallMap(strategy, resultKey string, record, data map[string]any) map[string]any {
	data = cloneCallMap(data)
	switch strategy {
	case mergeStrategyReplace:
		result := make(map[string]any, len(record)+len(data))
		for k, v := range record {
			result[k] = v
		}
		for k, v := range data {
			result[k] = v
		}
		return result
	case mergeStrategyAppend:
		return appendUnder(resultKey, record, data)
	default:
		return deepMerge(record, data)
	}
}

// appendCallList nests a list response under resultKey. merge and replace have
// nowhere to put a non-empty list: folding it into a record's top-level keys is
// not defined, and the pre-25.3 behavior — an empty map, no error — is exactly
// the silent data loss this story removes.
//
// An empty list is the exception: "no result" is a functional answer, and
// merging nothing into a record is the record itself. It leaves the record
// untouched under merge and replace rather than failing on a shape mismatch the
// pipeline author cannot avoid — the API decides how many items it returns.
func appendCallList(moduleType, strategy, resultKey string, record map[string]any, items []any) (map[string]any, error) {
	if strategy != mergeStrategyAppend {
		if len(items) == 0 {
			return record, nil
		}
		return nil, callContractError(
			"%s: %w: the response holds a list of %d item(s), which mergeStrategy %q cannot fold into the record — use mergeStrategy: append with a resultKey, then iterate with the loop filter",
			moduleType, ErrCallMergeStrategyMismatch, len(items), strategy)
	}
	return appendUnder(resultKey, record, cloneCallList(items)), nil
}

// appendUnder copies the record and nests data under resultKey.
func appendUnder(resultKey string, record map[string]any, data any) map[string]any {
	result := make(map[string]any, len(record)+1)
	for k, v := range record {
		result[k] = v
	}
	result[resultKey] = data
	return result
}

// deepMerge performs a recursive merge of two maps. Values from b override
// values from a at each level, except for nested maps which are merged.
func deepMerge(a, b map[string]any) map[string]any {
	result := make(map[string]any, len(a)+len(b))
	for k, v := range a {
		result[k] = v
	}
	for k, vb := range b {
		if va, exists := result[k]; exists {
			if mapA, okA := va.(map[string]any); okA {
				if mapB, okB := vb.(map[string]any); okB {
					result[k] = deepMerge(mapA, mapB)
					continue
				}
			}
		}
		result[k] = vb
	}
	return result
}

// cloneCallData deep-copies the containers of a call response. Scalars are
// shared as-is: they are immutable in Go, so only maps and slices can be
// rewritten under another record's feet.
func cloneCallData(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneCallMap(typed)
	case []any:
		return cloneCallList(typed)
	default:
		return value
	}
}

// cloneCallMap deep-copies a response object.
func cloneCallMap(data map[string]any) map[string]any {
	clone := make(map[string]any, len(data))
	for k, v := range data {
		clone[k] = cloneCallData(v)
	}
	return clone
}

// cloneCallList deep-copies a response list.
func cloneCallList(items []any) []any {
	clone := make([]any, len(items))
	for i, item := range items {
		clone[i] = cloneCallData(item)
	}
	return clone
}
