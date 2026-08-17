// Package metadata provides utilities for accessing and manipulating per-record
// metadata stored as a nested map within records, separate from record data.
// By convention, metadata field names start with an underscore (e.g. "_metadata").
//
// This package is leaf-level, so any module package — runtime, output, filter —
// can import it without creating a cycle.
package metadata

// DefaultFieldName is the default field name for storing metadata in records.
const DefaultFieldName = "_metadata"

// LoopFieldName is the metadata sub-map under which the loop filter records its
// per-alias bookkeeping (`_metadata.loop.<itemName>.index`).
//
// It lives here rather than in the loop filter because it is also the marker
// that tells a loop scope apart from a plain record, and the template package
// needs to recognize one without importing a module package.
const LoopFieldName = "loop"

// StripFromRecord returns a shallow copy of the record without the metadata field.
// fieldName falls back to DefaultFieldName when empty. Returns the original
// record (not a copy) when the metadata field is absent.
func StripFromRecord(record map[string]any, fieldName string) map[string]any {
	if record == nil {
		return nil
	}
	if fieldName == "" {
		fieldName = DefaultFieldName
	}
	if _, exists := record[fieldName]; !exists {
		return record
	}
	result := make(map[string]any, len(record)-1)
	for k, v := range record {
		if k != fieldName {
			result[k] = v
		}
	}
	return result
}

// StripFromRecords applies StripFromRecord to a slice of records.
func StripFromRecords(records []map[string]any, fieldName string) []map[string]any {
	if records == nil {
		return nil
	}
	result := make([]map[string]any, len(records))
	for i, r := range records {
		result[i] = StripFromRecord(r, fieldName)
	}
	return result
}
