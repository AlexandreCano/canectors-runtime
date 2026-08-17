package template

import "testing"

// A plain record keeps the historical shape: it is the `record` variable, and
// nothing else appears at the top level.
func TestContextForRecordPlainRecord(t *testing.T) {
	record := map[string]any{"orderId": "ORD-1"}

	vars := ContextForRecord(record).Vars()

	got, _ := vars[VarRecord].(map[string]any)
	if got["orderId"] != "ORD-1" {
		t.Errorf("record.orderId = %v, want ORD-1", got["orderId"])
	}
	reserved := []string{VarRecord, VarMeta, VarState, VarPagination}
	for _, name := range reserved {
		if _, present := vars[name]; !present {
			t.Errorf("reserved variable %q missing from the context", name)
		}
		if !IsReservedVarName(name) {
			t.Errorf("IsReservedVarName(%q) = false", name)
		}
	}
	if len(vars) != len(reserved) {
		t.Errorf("got %d variables, want only the %d reserved ones: %v", len(vars), len(reserved), vars)
	}
}

// A loop scope is unwrapped: the alias becomes its own top-level variable and
// `record` points at the root record, not at the scope.
func TestContextForRecordUnwrapsLoopScope(t *testing.T) {
	root := map[string]any{"orderId": "ORD-1"}
	scope := map[string]any{
		"record": root,
		"line":   map[string]any{"number": "L1"},
		"_metadata": map[string]any{
			"loop": map[string]any{"line": map[string]any{"index": 0}},
		},
	}

	vars := ContextForRecord(scope).Vars()

	// AC2: the root record is reachable as `record.<field>`, not `record.record`.
	gotRecord, _ := vars[VarRecord].(map[string]any)
	if gotRecord["orderId"] != "ORD-1" {
		t.Errorf("record.orderId = %v, want ORD-1", gotRecord["orderId"])
	}
	if _, nested := gotRecord[VarRecord]; nested {
		t.Error("record still nests another record: record.record survived")
	}

	// AC1: the item is reachable under its alias, the same name a field path uses.
	gotLine, _ := vars["line"].(map[string]any)
	if gotLine["number"] != "L1" {
		t.Errorf("line.number = %v, want L1", gotLine["number"])
	}

	// AC6: loop metadata stays addressable.
	gotMeta, _ := vars["_metadata"].(map[string]any)
	loop, _ := gotMeta["loop"].(map[string]any)
	line, _ := loop["line"].(map[string]any)
	if line["index"] != 0 {
		t.Errorf("_metadata.loop.line.index = %v, want 0", line["index"])
	}
}

// AC5: nested loops expose every active alias at the top level.
func TestContextForRecordUnwrapsNestedLoopScope(t *testing.T) {
	root := map[string]any{"orderId": "ORD-1"}
	scope := map[string]any{
		"record": root,
		"line":   map[string]any{"number": "L1"},
		"cell":   map[string]any{"value": "C1"},
		"_metadata": map[string]any{
			"loop": map[string]any{
				"line": map[string]any{"index": 0},
				"cell": map[string]any{"index": 2},
			},
		},
	}

	vars := ContextForRecord(scope).Vars()

	for alias, wantField := range map[string]string{"line": "number", "cell": "value"} {
		got, ok := vars[alias].(map[string]any)
		if !ok {
			t.Errorf("alias %q missing from the context", alias)
			continue
		}
		if got[wantField] == nil {
			t.Errorf("%s.%s is nil", alias, wantField)
		}
	}
}

// A record that merely has a field named `record` is not a loop scope: without
// the loop metadata marker, unwrapping it would hand templates the wrong root.
func TestContextForRecordDoesNotUnwrapPlainRecordNamedRecord(t *testing.T) {
	record := map[string]any{
		"record": map[string]any{"nested": "value"},
		"id":     "42",
	}

	vars := ContextForRecord(record).Vars()

	got, _ := vars[VarRecord].(map[string]any)
	if got["id"] != "42" {
		t.Errorf("record.id = %v, want 42 — the record was mistaken for a loop scope", got["id"])
	}
}

// Empty loop metadata is not a scope either.
func TestContextForRecordIgnoresEmptyLoopMetadata(t *testing.T) {
	record := map[string]any{
		"record":    map[string]any{"nested": "value"},
		"id":        "42",
		"_metadata": map[string]any{"loop": map[string]any{}},
	}

	vars := ContextForRecord(record).Vars()

	got, _ := vars[VarRecord].(map[string]any)
	if got["id"] != "42" {
		t.Errorf("record.id = %v, want 42", got["id"])
	}
}

// A field a nested filter wrote at scope level — an http_call response merge, a
// sql_call resultKey, a `set` without the `record.` prefix — is a top-level
// variable too, addressed by the same bare name a field path uses. It is
// deliberately *not* under `record`: `record` is the root record, and the scope
// write never reached it.
func TestContextForRecordExposesScopeLevelWritesAtTopLevel(t *testing.T) {
	scope := map[string]any{
		"record": map[string]any{"orderId": "ORD-1"},
		"line":   map[string]any{"number": "L1"},
		// What an http_call in the chain just merged in.
		"catalogPrice": 42,
		"_metadata": map[string]any{
			"loop": map[string]any{"line": map[string]any{"index": 0}},
		},
	}

	vars := ContextForRecord(scope).Vars()

	if vars["catalogPrice"] != 42 {
		t.Errorf("catalogPrice = %v, want 42", vars["catalogPrice"])
	}
	root, _ := vars[VarRecord].(map[string]any)
	if _, onRoot := root["catalogPrice"]; onRoot {
		t.Error("the scope write leaked onto the root record")
	}
}

// Extra must never shadow a reserved variable, whatever ends up in it. This is
// load-bearing, not belt-and-braces: ContextForRecord hands the whole loop
// scope to Extra, `record` key included, and relies on this rule to overwrite
// it with the root record instead of copying the scope minus one key.
func TestVarsReservedNamesWinOverExtra(t *testing.T) {
	rc := RenderContext{
		Record: map[string]any{"real": true},
		Extra:  map[string]any{VarRecord: "hijacked", VarState: "hijacked"},
	}

	vars := rc.Vars()

	got, _ := vars[VarRecord].(map[string]any)
	if got == nil || got["real"] != true {
		t.Errorf("record = %v, want the real record", vars[VarRecord])
	}
	if _, isString := vars[VarState].(string); isString {
		t.Error("state was overwritten by Extra")
	}
}
