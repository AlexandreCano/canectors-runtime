package filter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cannectors/runtime/internal/moduleconfig"
	"github.com/cannectors/runtime/pkg/connector"
)

// loopScopeCapture runs a loop whose nested filter is an http_call, and returns
// the URLs the server received — one per item.
//
// http_call is the probe because it exercises both addressing mechanisms at
// once: the endpoint is a template, and keys[].field is a field path. The whole
// point of this story is that the two must agree.
func loopScopeCapture(t *testing.T, endpointPath string, keys []moduleconfig.KeyConfig, record map[string]any) []*url.URL {
	t.Helper()

	var got []*url.URL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := *r.URL
		got = append(got, &u)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	if len(keys) == 0 {
		keys = []moduleconfig.KeyConfig{{Field: "record.orderId", ParamType: "header", ParamName: "X-Order"}}
	}

	inner, err := NewHTTPCallFromConfig(HTTPCallConfig{
		HTTPRequestBase: moduleconfig.HTTPRequestBase{Endpoint: server.URL + endpointPath},
		ModuleBase:      connector.ModuleBase{OnError: "fail"},
		Keys:            keys,
	})
	if err != nil {
		t.Fatalf("failed to create nested http_call: %v", err)
	}

	loop, err := NewLoopFromConfig(LoopConfig{
		Field:    "lines",
		ItemName: "line",
		Filters:  []*NestedModuleConfig{{Type: "http_call"}},
	}, func(*NestedModuleConfig, int) (Module, error) { return inner, nil })
	if err != nil {
		t.Fatalf("failed to create loop: %v", err)
	}

	if _, err := loop.Process(context.Background(), []map[string]any{record}); err != nil {
		t.Fatalf("loop.Process returned an error: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("server received no request")
	}
	return got
}

func orderWithLines() map[string]any {
	return map[string]any{
		"orderId": "ORD-1",
		"lines": []any{
			map[string]any{"number": "L1"},
			map[string]any{"number": "L2"},
		},
	}
}

// AC1 + AC2: inside a loop, the alias reaches the current item and `record`
// reaches the root — the same names field paths use.
func TestLoopTemplateAddressesItemAndRootWithOneConvention(t *testing.T) {
	got := loopScopeCapture(t,
		"/item/{{ line.number }}/root/{{ record.orderId }}",
		nil,
		orderWithLines(),
	)

	want := []string{"/item/L1/root/ORD-1", "/item/L2/root/ORD-1"}
	if len(got) != len(want) {
		t.Fatalf("got %d requests, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Path != w {
			t.Errorf("request %d path = %q, want %q", i, got[i].Path, w)
		}
	}
}

// AC4: keys[].field and templates now resolve the same values.
func TestLoopKeysAndTemplatesAgree(t *testing.T) {
	got := loopScopeCapture(t,
		"/item/{{ line.number }}/root/{{ record.orderId }}",
		[]moduleconfig.KeyConfig{
			{Field: "line.number", ParamType: "query", ParamName: "keyItem"},
			{Field: "record.orderId", ParamType: "query", ParamName: "keyRoot"},
		},
		orderWithLines(),
	)

	first := got[0]
	if v := first.Query().Get("keyItem"); v != "L1" {
		t.Errorf("keyItem = %q, want L1", v)
	}
	if v := first.Query().Get("keyRoot"); v != "ORD-1" {
		t.Errorf("keyRoot = %q, want ORD-1", v)
	}
	// The template rendered the same two values into the path.
	if first.Path != "/item/L1/root/ORD-1" {
		t.Errorf("path = %q, want /item/L1/root/ORD-1", first.Path)
	}
}

// AC6: loop metadata is addressable from a template.
func TestLoopTemplateReadsLoopIndex(t *testing.T) {
	got := loopScopeCapture(t,
		"/idx/{{ _metadata.loop.line.index }}",
		nil,
		orderWithLines(),
	)

	want := []string{"/idx/0", "/idx/1"}
	for i, w := range want {
		if got[i].Path != w {
			t.Errorf("request %d path = %q, want %q", i, got[i].Path, w)
		}
	}
}

// AC3: the old double-prefixed form no longer resolves. It fails loudly rather
// than rendering empty — a silent empty in a URL is how the old convention cost
// so much time.
func TestLoopOldPrefixedFormNoLongerResolves(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	inner, err := NewHTTPCallFromConfig(HTTPCallConfig{
		HTTPRequestBase: moduleconfig.HTTPRequestBase{
			Endpoint: server.URL + "/old/{{ record.line.number }}",
		},
		ModuleBase: connector.ModuleBase{OnError: "fail"},
		Keys: []moduleconfig.KeyConfig{
			{Field: "record.orderId", ParamType: "header", ParamName: "X-Order"},
		},
	})
	if err != nil {
		t.Fatalf("failed to create nested http_call: %v", err)
	}

	loop, err := NewLoopFromConfig(LoopConfig{
		Field:    "lines",
		ItemName: "line",
		Filters:  []*NestedModuleConfig{{Type: "http_call"}},
	}, func(*NestedModuleConfig, int) (Module, error) { return inner, nil })
	if err != nil {
		t.Fatalf("failed to create loop: %v", err)
	}

	_, err = loop.Process(context.Background(), []map[string]any{orderWithLines()})
	if err == nil {
		t.Fatal("record.line.number still resolved; the old convention survived")
	}
}

// An alias may not take the name of a top-level template variable: the alias
// would lose the collision at render time and the loop would quietly not work.
func TestLoopRejectsItemNameShadowingATemplateVariable(t *testing.T) {
	for _, itemName := range reservedItemNames() {
		t.Run(itemName, func(t *testing.T) {
			_, err := NewLoopFromConfig(LoopConfig{
				Field:    "lines",
				ItemName: itemName,
				Filters:  []*NestedModuleConfig{{Type: "drop"}},
			}, func(*NestedModuleConfig, int) (Module, error) { return NewDrop(), nil })

			if err == nil {
				t.Fatalf("itemName %q was accepted", itemName)
			}
			if !strings.Contains(err.Error(), itemName) {
				t.Errorf("error does not name the offending alias: %v", err)
			}
		})
	}
}

// The reserved-alias set lives twice: in Go, where the constructor enforces it,
// and in the JSON schema, which restates it so `validate` catches a bad alias
// before a run starts. Two lists drift, and the drift is one-way nasty — a
// schema that accepts an itemName the runtime refuses turns a validation error
// into a startup failure. This test is the seam between them.
func TestLoopReservedItemNamesMatchTheSchema(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "config", "schema", "filter-schema.json"))
	if err != nil {
		t.Fatalf("reading filter schema: %v", err)
	}

	var schema struct {
		Defs struct {
			Loop struct {
				Properties struct {
					ItemName struct {
						Not struct {
							Enum []string `json:"enum"`
						} `json:"not"`
					} `json:"itemName"`
				} `json:"properties"`
			} `json:"loopFilterConfig"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("parsing filter schema: %v", err)
	}

	fromSchema := schema.Defs.Loop.Properties.ItemName.Not.Enum
	if len(fromSchema) == 0 {
		t.Fatal("the schema no longer forbids any itemName — the enum moved or was dropped")
	}

	fromGo := reservedItemNames()
	slices.Sort(fromSchema)
	slices.Sort(fromGo)
	if !slices.Equal(fromSchema, fromGo) {
		t.Errorf("reserved itemNames disagree:\n  schema: %v\n  Go:     %v", fromSchema, fromGo)
	}
}
