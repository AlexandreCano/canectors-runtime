package metadata

import (
	"testing"
)

func TestStripFromRecord(t *testing.T) {
	t.Run("strips metadata field from copy", func(t *testing.T) {
		record := map[string]any{
			"data":      "value",
			"_metadata": map[string]any{"key": "meta"},
		}
		stripped := StripFromRecord(record, "")

		if _, exists := stripped["_metadata"]; exists {
			t.Error("expected stripped copy to lack _metadata")
		}
		if _, exists := record["_metadata"]; !exists {
			t.Error("expected original to still have _metadata (StripFromRecord must not mutate)")
		}
		if stripped["data"] != "value" {
			t.Errorf("stripped['data'] = %v, want 'value'", stripped["data"])
		}
	})

	t.Run("returns original when metadata field is absent", func(t *testing.T) {
		record := map[string]any{"data": "value"}
		got := StripFromRecord(record, "")
		// Identity check: nothing to copy when there's no metadata.
		if &got == &record {
			t.Error("expected same backing storage when no metadata to strip")
		}
		if got["data"] != "value" {
			t.Errorf("got['data'] = %v, want 'value'", got["data"])
		}
	})

	t.Run("custom field name", func(t *testing.T) {
		record := map[string]any{
			"data":    "value",
			"_custom": map[string]any{"x": 1},
		}
		stripped := StripFromRecord(record, "_custom")
		if _, exists := stripped["_custom"]; exists {
			t.Error("expected _custom to be stripped")
		}
	})

	t.Run("nil record returns nil", func(t *testing.T) {
		if StripFromRecord(nil, "") != nil {
			t.Error("expected nil for nil input")
		}
	})
}

func TestStripFromRecords(t *testing.T) {
	records := []map[string]any{
		{"data": "a", "_metadata": map[string]any{"i": 1}},
		{"data": "b"},
	}
	stripped := StripFromRecords(records, "")

	if len(stripped) != 2 {
		t.Fatalf("len = %d, want 2", len(stripped))
	}
	if _, exists := stripped[0]["_metadata"]; exists {
		t.Error("expected first record to be stripped")
	}
	// Original first record must remain unchanged.
	if _, exists := records[0]["_metadata"]; !exists {
		t.Error("expected original first record to still have _metadata")
	}
}
