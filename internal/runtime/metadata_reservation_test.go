// Package runtime provides the pipeline execution engine.
package runtime

import (
	"testing"

	"github.com/cannectors/runtime/internal/metadata"
	"github.com/cannectors/runtime/internal/template"
	"github.com/cannectors/runtime/pkg/connector"
)

// `_metadata` is runtime bookkeeping, and since the loop addressing fix it is
// also the marker that tells a loop scope apart from a plain record. A source
// payload carrying it — together with a field named `record`, which any API is
// free to return — would otherwise be unwrapped as a scope, and every template
// downstream would resolve `record.*` against the forged root instead of the
// record. Silently: that is the whole failure mode this reservation removes.
func TestExecutor_ReservesMetadataFieldFromInboundRecords(t *testing.T) {
	forged := map[string]any{
		"id": "real",
		"record": map[string]any{
			"id": "forged",
		},
		metadata.DefaultFieldName: map[string]any{
			metadata.LoopFieldName: map[string]any{
				"line": map[string]any{"index": 9},
			},
		},
	}

	mockOutput := NewMockOutputModule(nil)
	executor := NewExecutorWithModules(nil, nil, mockOutput, false)

	result, err := executor.ExecuteWithRecords(&connector.Pipeline{
		ID:      "test-metadata-reservation",
		Name:    "Test Pipeline",
		Version: "1.0.0",
		Enabled: true,
	}, []map[string]any{forged})
	if err != nil {
		t.Fatalf("ExecuteWithRecords() error = %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("status = %s, want success", result.Status)
	}

	if len(mockOutput.sentRecords) != 1 {
		t.Fatalf("got %d records sent, want 1", len(mockOutput.sentRecords))
	}
	sent := mockOutput.sentRecords[0]

	if _, carried := sent[metadata.DefaultFieldName]; carried {
		t.Errorf("%s survived the input stage: %v", metadata.DefaultFieldName, sent)
	}
	// The field a source is legitimately allowed to call `record` is untouched.
	if _, kept := sent["record"]; !kept {
		t.Error("the source's own `record` field was dropped along with the reserved one")
	}

	// The consequence that matters: `record.id` is the record's own field, not
	// the one the payload tried to install as a root.
	rendered := template.ContextForRecord(sent)
	if got, _ := rendered.Record["id"].(string); got != "real" {
		t.Errorf("record.id renders as %q, want \"real\" — the payload forged a loop scope", got)
	}
}
