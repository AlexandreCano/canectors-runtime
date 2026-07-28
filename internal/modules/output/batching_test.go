package output

import (
	"strings"
	"testing"
)

func recordsOfSize(n int) []map[string]any {
	records := make([]map[string]any, n)
	for i := range records {
		records[i] = map[string]any{"i": i}
	}
	return records
}

func TestChunkRecords(t *testing.T) {
	tests := []struct {
		name       string
		recordLen  int
		size       int
		wantSizes  []int
		wantNilOut bool
	}{
		{name: "exact multiple", recordLen: 100, size: 50, wantSizes: []int{50, 50}},
		{name: "partial last batch", recordLen: 120, size: 50, wantSizes: []int{50, 50, 20}},
		{name: "size larger than input", recordLen: 3, size: 50, wantSizes: []int{3}},
		{name: "size of one", recordLen: 3, size: 1, wantSizes: []int{1, 1, 1}},
		{name: "size zero means single batch", recordLen: 120, size: 0, wantSizes: []int{120}},
		{name: "negative size means single batch", recordLen: 7, size: -1, wantSizes: []int{7}},
		{name: "empty input yields no batch", recordLen: 0, size: 50, wantNilOut: true},
		{name: "empty input without chunking yields no batch", recordLen: 0, size: 0, wantNilOut: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			batches := chunkRecords(recordsOfSize(tc.recordLen), tc.size)

			if tc.wantNilOut {
				if len(batches) != 0 {
					t.Fatalf("expected no batch, got %d", len(batches))
				}
				return
			}

			if len(batches) != len(tc.wantSizes) {
				t.Fatalf("batch count = %d, want %d", len(batches), len(tc.wantSizes))
			}
			seen := 0
			for i, batch := range batches {
				if len(batch) != tc.wantSizes[i] {
					t.Errorf("batch %d size = %d, want %d", i, len(batch), tc.wantSizes[i])
				}
				// Order must be preserved across batches: records carry their
				// original index, so the flattened sequence must be 0..n-1.
				for _, record := range batch {
					if record["i"] != seen {
						t.Fatalf("record out of order: got %v, want %d", record["i"], seen)
					}
					seen++
				}
			}
			if seen != tc.recordLen {
				t.Errorf("records covered = %d, want %d", seen, tc.recordLen)
			}
		})
	}
}

func TestNormalizeBatchSize(t *testing.T) {
	tests := []struct {
		name        string
		batchSize   int
		requestMode string
		want        int
		wantErr     string
	}{
		{name: "absent in batch mode", batchSize: 0, requestMode: "batch", want: 0},
		{name: "absent in single mode", batchSize: 0, requestMode: "single", want: 0},
		{name: "set in batch mode", batchSize: 50, requestMode: "batch", want: 50},
		{name: "set in single mode is rejected", batchSize: 50, requestMode: "single", wantErr: "requestMode 'batch'"},
		{name: "negative is rejected", batchSize: -1, requestMode: "batch", wantErr: "must be greater than 0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeBatchSize(tc.batchSize, tc.requestMode, "soapRequest")

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %q, want it to contain %q", err, tc.wantErr)
				}
				if !strings.Contains(err.Error(), "soapRequest") {
					t.Errorf("error %q should name the module type", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("batchSize = %d, want %d", got, tc.want)
			}
		})
	}
}
