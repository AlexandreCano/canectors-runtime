package filter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cannectors/runtime/internal/errhandling"
	"github.com/cannectors/runtime/internal/moduleconfig"
	"github.com/cannectors/runtime/pkg/connector"
)

// dataFieldServer replies with the given payload for every request.
func dataFieldServer(t *testing.T, payload map[string]any) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			t.Errorf("encoding response: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// dataFieldModule builds an http_call module reading dataField, appending the
// result under resultKey.
func dataFieldModule(t *testing.T, endpoint, dataField, strategy, resultKey, onError string) *HTTPCallModule {
	t.Helper()
	module, err := NewHTTPCallFromConfig(HTTPCallConfig{
		ModuleBase:      connector.ModuleBase{OnError: onError},
		HTTPRequestBase: moduleconfig.HTTPRequestBase{Endpoint: endpoint},
		Keys: []moduleconfig.KeyConfig{{
			Field: "id", ParamType: "query", ParamName: "id",
		}},
		DataField:     dataField,
		MergeStrategy: strategy,
		ResultKey:     resultKey,
	})
	if err != nil {
		t.Fatalf("NewHTTPCallFromConfig: %v", err)
	}
	return module
}

// TestHTTPCall_DataFieldKeepsMultiElementList locks Story 25.3 AC1: a dataField
// resolving to a list of three objects reaches the record with its three
// elements, where it used to be replaced by an empty map.
func TestHTTPCall_DataFieldKeepsMultiElementList(t *testing.T) {
	server := dataFieldServer(t, map[string]any{
		"results": []any{
			map[string]any{"sku": "a"},
			map[string]any{"sku": "b"},
			map[string]any{"sku": "c"},
		},
	})
	module := dataFieldModule(t, server.URL, "results", "append", "matches", "")

	out, err := module.Process(context.Background(), []map[string]any{{"id": "1"}})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	matches, ok := out[0]["matches"].([]any)
	if !ok {
		t.Fatalf("matches = %T, want []any (record: %v)", out[0]["matches"], out[0])
	}
	if len(matches) != 3 {
		t.Fatalf("expected 3 items, got %d: %v", len(matches), matches)
	}
	for i, want := range []string{"a", "b", "c"} {
		item, _ := matches[i].(map[string]any)
		if item["sku"] != want {
			t.Errorf("matches[%d].sku = %v, want %v", i, item["sku"], want)
		}
	}
}

// TestHTTPCall_DataFieldSingleElementListStaysList locks that the shape no
// longer depends on the payload: a one-element list is a list, like any other.
func TestHTTPCall_DataFieldSingleElementListStaysList(t *testing.T) {
	server := dataFieldServer(t, map[string]any{
		"results": []any{map[string]any{"sku": "only"}},
	})
	module := dataFieldModule(t, server.URL, "results", "append", "matches", "")

	out, err := module.Process(context.Background(), []map[string]any{{"id": "1"}})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	matches, ok := out[0]["matches"].([]any)
	if !ok || len(matches) != 1 {
		t.Fatalf("matches = %#v, want a one-element list", out[0]["matches"])
	}
}

// TestHTTPCall_DataFieldNestedPath locks that dataField resolves a path, the way
// soap_call already did (Story 25.3 AC6).
func TestHTTPCall_DataFieldNestedPath(t *testing.T) {
	server := dataFieldServer(t, map[string]any{
		"payload": map[string]any{"customer": map[string]any{"tier": "gold"}},
	})
	module := dataFieldModule(t, server.URL, "payload.customer", "merge", "", "")

	out, err := module.Process(context.Background(), []map[string]any{{"id": "1"}})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if out[0]["tier"] != "gold" {
		t.Fatalf("tier = %v, want gold (record: %v)", out[0]["tier"], out[0])
	}
}

// TestHTTPCall_DataFieldEmptyList locks Story 25.3 AC3: an empty list is a
// functional outcome — the record continues, carrying an empty list.
func TestHTTPCall_DataFieldEmptyList(t *testing.T) {
	server := dataFieldServer(t, map[string]any{"results": []any{}})
	module := dataFieldModule(t, server.URL, "results", "append", "matches", "")

	out, err := module.Process(context.Background(), []map[string]any{{"id": "1"}})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected the record to continue, got %d records", len(out))
	}
	matches, ok := out[0]["matches"].([]any)
	if !ok || len(matches) != 0 {
		t.Fatalf("matches = %#v, want an empty list", out[0]["matches"])
	}
}

// TestHTTPCall_DataFieldListRejectedByMergeAndReplace locks that a list is not
// silently dropped when the strategy has nowhere to put it, and that the error
// names the way out.
func TestHTTPCall_DataFieldListRejectedByMergeAndReplace(t *testing.T) {
	server := dataFieldServer(t, map[string]any{
		"results": []any{map[string]any{"sku": "a"}, map[string]any{"sku": "b"}},
	})
	for _, strategy := range []string{"merge", "replace"} {
		t.Run(strategy, func(t *testing.T) {
			module := dataFieldModule(t, server.URL, "results", strategy, "", "")
			_, err := module.Process(context.Background(), []map[string]any{{"id": "1"}})
			if err == nil {
				t.Fatal("expected an error, got none")
			}
			if !strings.Contains(err.Error(), "mergeStrategy: append") {
				t.Errorf("error does not point at the fix: %v", err)
			}
		})
	}
}

// TestHTTPCall_DataFieldErrorsDistinguishCauses locks Story 25.3 AC4/AC5: a
// missing field and an unusable type are two distinct errors, neither of which
// is the pre-25.3 silent empty map.
func TestHTTPCall_DataFieldErrorsDistinguishCauses(t *testing.T) {
	cases := []struct {
		name     string
		payload  map[string]any
		wantCode string
	}{
		{"missing field", map[string]any{"other": 1}, ErrCodeHTTPCallDataFieldMissing},
		{"scalar field", map[string]any{"results": "not-an-object"}, ErrCodeHTTPCallDataFieldType},
		{"null field", map[string]any{"results": nil}, ErrCodeHTTPCallDataFieldType},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := dataFieldServer(t, tc.payload)
			module := dataFieldModule(t, server.URL, "results", "append", "matches", "")

			_, err := module.Process(context.Background(), []map[string]any{{"id": "1"}})
			if err == nil {
				t.Fatal("expected an error, got none")
			}
			var callErr *HTTPCallError
			if !errors.As(err, &callErr) {
				t.Fatalf("error is %T, want *HTTPCallError: %v", err, err)
			}
			if callErr.Code != tc.wantCode {
				t.Errorf("code = %q, want %q (message: %s)", callErr.Code, tc.wantCode, callErr.Message)
			}
		})
	}
}

// TestHTTPCall_DataFieldErrorHonorsOnError locks Story 25.3 AC4: the dataField
// failure travels through onError like any other http_call failure.
func TestHTTPCall_DataFieldErrorHonorsOnError(t *testing.T) {
	server := dataFieldServer(t, map[string]any{"results": "not-an-object"})

	t.Run("skip drops the record", func(t *testing.T) {
		module := dataFieldModule(t, server.URL, "results", "append", "matches", "skip")
		out, err := module.Process(context.Background(), []map[string]any{{"id": "1"}})
		if err != nil {
			t.Fatalf("Process: %v", err)
		}
		if len(out) != 0 {
			t.Fatalf("expected the record to be dropped, got %v", out)
		}
	})

	t.Run("log keeps the record annotated", func(t *testing.T) {
		module := dataFieldModule(t, server.URL, "results", "append", "matches", "log")
		out, err := module.Process(context.Background(), []map[string]any{{"id": "1"}})
		if err != nil {
			t.Fatalf("Process: %v", err)
		}
		if len(out) != 1 {
			t.Fatalf("expected 1 record, got %d", len(out))
		}
		if _, ok := out[0][errhandling.RecordErrorsField]; !ok {
			t.Fatalf("record carries no error marker: %v", out[0])
		}
		if _, ok := out[0]["matches"]; ok {
			t.Errorf("record should not carry an enrichment: %v", out[0])
		}
	})
}

// TestHTTPCall_DataFieldListFeedsLoop locks Story 25.3 AC2: the list http_call
// leaves on the record is directly iterable by the loop filter.
func TestHTTPCall_DataFieldListFeedsLoop(t *testing.T) {
	server := dataFieldServer(t, map[string]any{
		"results": []any{
			map[string]any{"label": "alpha"},
			map[string]any{"label": "beta"},
		},
	})
	call := dataFieldModule(t, server.URL, "results", "append", "matches", "")

	enriched, err := call.Process(context.Background(), []map[string]any{{"id": "1"}})
	if err != nil {
		t.Fatalf("http_call Process: %v", err)
	}

	source := "match.label"
	target := "match.name"
	loop, err := NewLoopFromConfig(LoopConfig{
		Field:    "matches",
		ItemName: "match",
		Filters: []*NestedModuleConfig{
			{Type: "mapping", Mappings: []FieldMapping{{Source: &source, Target: target}}},
		},
	}, nil)
	if err != nil {
		t.Fatalf("NewLoopFromConfig: %v", err)
	}

	out, err := loop.Process(context.Background(), enriched)
	if err != nil {
		t.Fatalf("loop Process: %v", err)
	}
	matches, ok := out[0]["matches"].([]any)
	if !ok || len(matches) != 2 {
		t.Fatalf("matches = %#v, want 2 items", out[0]["matches"])
	}
	for i, want := range []string{"alpha", "beta"} {
		item, _ := matches[i].(map[string]any)
		if item["name"] != want {
			t.Errorf("matches[%d].name = %v, want %v", i, item["name"], want)
		}
	}
}

// dataFieldCachedModule builds the same module sharing one cache slot across
// every record, which is what makes response aliasing observable.
func dataFieldCachedModule(t *testing.T, endpoint, dataField, resultKey string) *HTTPCallModule {
	t.Helper()
	module, err := NewHTTPCallFromConfig(HTTPCallConfig{
		HTTPRequestBase: moduleconfig.HTTPRequestBase{Endpoint: endpoint},
		Keys: []moduleconfig.KeyConfig{{
			Field: "id", ParamType: "query", ParamName: "id",
		}},
		DataField:     dataField,
		MergeStrategy: "append",
		ResultKey:     resultKey,
		Cache:         moduleconfig.CacheConfig{Enabled: true, Key: "shared"},
	})
	if err != nil {
		t.Fatalf("NewHTTPCallFromConfig: %v", err)
	}
	return module
}

// TestHTTPCall_CachedListIsNotSharedBetweenRecords locks that the list handed to
// a record is the record's own: with caching on, the response is the very object
// the LRU holds, and the pattern this story documents (append + loop) mutates
// the items in place. Sharing it let one record's transform rewrite another
// record's data — and poison the cache entry for the rest of the run.
func TestHTTPCall_CachedListIsNotSharedBetweenRecords(t *testing.T) {
	server := dataFieldServer(t, map[string]any{
		"results": []any{map[string]any{"label": "alpha"}},
	})
	module := dataFieldCachedModule(t, server.URL, "results", "matches")

	out, err := module.Process(context.Background(), []map[string]any{{"id": "1"}, {"id": "2"}})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	first := asObject(t, asList(t, out[0]["matches"])[0])
	second := asObject(t, asList(t, out[1]["matches"])[0])

	first["label"] = "mutated"
	if second["label"] != "alpha" {
		t.Errorf("record 2 sees record 1's mutation: label = %v", second["label"])
	}

	// The cache entry must be pristine too, otherwise every later record in the
	// run inherits the mutation.
	later, err := module.Process(context.Background(), []map[string]any{{"id": "3"}})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	item := asObject(t, asList(t, later[0]["matches"])[0])
	if item["label"] != "alpha" {
		t.Errorf("cache entry was poisoned: label = %v", item["label"])
	}
}

// TestHTTPCall_CachedObjectIsNotSharedBetweenRecords locks the same isolation
// for an object response, where merge only ever copied the top level.
func TestHTTPCall_CachedObjectIsNotSharedBetweenRecords(t *testing.T) {
	server := dataFieldServer(t, map[string]any{
		"payload": map[string]any{"customer": map[string]any{"tier": "gold"}},
	})
	module, err := NewHTTPCallFromConfig(HTTPCallConfig{
		HTTPRequestBase: moduleconfig.HTTPRequestBase{Endpoint: server.URL},
		Keys: []moduleconfig.KeyConfig{{
			Field: "id", ParamType: "query", ParamName: "id",
		}},
		DataField:     "payload",
		MergeStrategy: "merge",
		Cache:         moduleconfig.CacheConfig{Enabled: true, Key: "shared"},
	})
	if err != nil {
		t.Fatalf("NewHTTPCallFromConfig: %v", err)
	}

	out, err := module.Process(context.Background(), []map[string]any{{"id": "1"}, {"id": "2"}})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	asObject(t, out[0]["customer"])["tier"] = "mutated"
	if got := asObject(t, out[1]["customer"])["tier"]; got != "gold" {
		t.Errorf("record 2 sees record 1's mutation: tier = %v", got)
	}
}

// TestHTTPCall_DataFieldEmptyListUnderMergeAndReplace locks Story 25.3 AC3 for
// the strategies that have no destination for a list: "no result" is a
// functional answer, so the record continues untouched instead of failing on a
// shape the pipeline author does not control.
func TestHTTPCall_DataFieldEmptyListUnderMergeAndReplace(t *testing.T) {
	server := dataFieldServer(t, map[string]any{"results": []any{}})
	for _, strategy := range []string{"merge", "replace"} {
		t.Run(strategy, func(t *testing.T) {
			module := dataFieldModule(t, server.URL, "results", strategy, "", "")
			out, err := module.Process(context.Background(), []map[string]any{{"id": "1"}})
			if err != nil {
				t.Fatalf("Process: %v", err)
			}
			if len(out) != 1 {
				t.Fatalf("expected the record to continue, got %d records", len(out))
			}
			if len(out[0]) != 1 || out[0]["id"] != "1" {
				t.Errorf("record should be untouched, got %v", out[0])
			}
		})
	}
}

// TestHTTPCall_ContractErrorsCarrySentinels locks that the shared merge contract
// stays routable: the sentinel travels in the cause chain next to the http_call
// code, and the failure is a non-retryable validation error — replaying the call
// returns the same unusable shape.
func TestHTTPCall_ContractErrorsCarrySentinels(t *testing.T) {
	cases := []struct {
		name     string
		payload  map[string]any
		strategy string
		sentinel error
	}{
		{
			name:     "list under merge",
			payload:  map[string]any{"results": []any{map[string]any{"sku": "a"}}},
			strategy: "merge",
			sentinel: ErrCallMergeStrategyMismatch,
		},
		{
			name:     "scalar",
			payload:  map[string]any{"results": "not-an-object"},
			strategy: "append",
			sentinel: ErrCallResultType,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := dataFieldServer(t, tc.payload)
			resultKey := ""
			if tc.strategy == "append" {
				resultKey = "matches"
			}
			module := dataFieldModule(t, server.URL, "results", tc.strategy, resultKey, "")

			_, err := module.Process(context.Background(), []map[string]any{{"id": "1"}})
			if err == nil {
				t.Fatal("expected an error, got none")
			}
			if !errors.Is(err, tc.sentinel) {
				t.Errorf("error does not carry the shared sentinel: %v", err)
			}
			if errhandling.IsRetryable(err) {
				t.Error("a deterministic shape failure must not be retryable")
			}
			classified := errhandling.ClassifyError(err)
			if classified.Category != errhandling.CategoryValidation {
				t.Errorf("category = %q, want %q", classified.Category, errhandling.CategoryValidation)
			}
		})
	}
}
