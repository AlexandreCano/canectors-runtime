package filter

import "testing"

// asObject reads a value as a response object, failing the test otherwise. The
// two-value assertion form is what keeps these tests readable while satisfying
// errcheck's check-type-assertions.
func asObject(t *testing.T, value any) map[string]any {
	t.Helper()
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%#v is not an object", value)
	}
	return object
}

// asList reads a value as a response list, failing the test otherwise.
func asList(t *testing.T, value any) []any {
	t.Helper()
	list, ok := value.([]any)
	if !ok {
		t.Fatalf("%#v is not a list", value)
	}
	return list
}

// TestMergeCallMap_ClonesResponse locks the isolation the three call filters
// rely on: the response object may be the one held by the LRU cache, so what
// lands in the record must be a copy all the way down. sql_call reaches this
// helper directly, without going through mergeCallResult.
func TestMergeCallMap_ClonesResponse(t *testing.T) {
	response := map[string]any{
		"customer": map[string]any{"tier": "gold"},
		"tags":     []any{map[string]any{"name": "vip"}},
	}

	first := mergeCallMap(mergeStrategyMerge, "", map[string]any{"id": "1"}, response)
	second := mergeCallMap(mergeStrategyMerge, "", map[string]any{"id": "2"}, response)

	asObject(t, first["customer"])["tier"] = "mutated"
	asObject(t, asList(t, first["tags"])[0])["name"] = "mutated"

	if got := asObject(t, second["customer"])["tier"]; got != "gold" {
		t.Errorf("nested map leaked between records: tier = %v", got)
	}
	if got := asObject(t, asList(t, second["tags"])[0])["name"]; got != "vip" {
		t.Errorf("nested list leaked between records: name = %v", got)
	}
	if got := asObject(t, response["customer"])["tier"]; got != "gold" {
		t.Errorf("the cached response itself was mutated: tier = %v", got)
	}
}
