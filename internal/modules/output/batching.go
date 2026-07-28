package output

import (
	"fmt"
	"slices"
)

// normalizeBatchSize validates batchSize against requestMode and returns the
// effective chunk size. Zero means "no chunking": every record goes into a
// single request, which is the historical behavior of requestMode "batch".
//
// batchSize only has meaning in batch mode — in single mode there is already
// one request per record, so combining the two is a configuration mistake and
// is rejected rather than silently ignored.
func normalizeBatchSize(batchSize int, requestMode, moduleType string) (int, error) {
	if batchSize == 0 {
		return 0, nil
	}
	if batchSize < 0 {
		return 0, fmt.Errorf("%s invalid batchSize %d: must be greater than 0", moduleType, batchSize)
	}
	if requestMode == requestModeSingle {
		return 0, fmt.Errorf("%s batchSize is only valid with requestMode 'batch', got 'single'", moduleType)
	}
	return batchSize, nil
}

// chunkRecords splits records into batches of at most size records, preserving
// order. A size of zero or less yields one batch holding every record, so
// callers never need to branch on "chunking enabled": the un-chunked path is
// simply the chunked path with a single batch. That is what keeps Send and
// PreviewRequest from drifting apart.
//
// An empty input yields no batch at all, so callers loop zero times.
func chunkRecords(records []map[string]any, size int) [][]map[string]any {
	if len(records) == 0 {
		return nil
	}
	if size <= 0 {
		return [][]map[string]any{records}
	}
	// slices.Chunk panics below 1, hence the guard above; it returns an
	// iterator, so materialize it — callers count batches and pre-size slices.
	return slices.Collect(slices.Chunk(records, size))
}
