package output

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

// httpBatchSizeConfig builds a batch-mode httpRequest config with retry
// disabled so request counts map one-to-one onto batches.
func httpBatchSizeConfig(endpoint string, extra map[string]any) map[string]any {
	cfg := map[string]any{
		"endpoint":    endpoint,
		"method":      "POST",
		"requestMode": "batch",
		"retry":       map[string]any{"maxAttempts": 0},
	}
	for k, v := range extra {
		cfg[k] = v
	}
	return cfg
}

// bodyRecordCounts decodes each captured request body as a JSON array and
// returns the number of records it carried.
func bodyRecordCounts(t *testing.T, requests []*capturedRequest) []int {
	t.Helper()
	counts := make([]int, 0, len(requests))
	for i, req := range requests {
		var records []map[string]any
		if err := json.Unmarshal(req.Body, &records); err != nil {
			t.Fatalf("request %d body is not a JSON array: %v\n%s", i, err, req.Body)
		}
		counts = append(counts, len(records))
	}
	return counts
}

func assertCounts(t *testing.T, got, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("request count = %d %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("batch record counts = %v, want %v", got, want)
		}
	}
}

// TestHTTPRequest_BatchSizeSplitsRequests locks the core contract: 120 records
// with batchSize 50 produce three requests carrying 50/50/20 records.
func TestHTTPRequest_BatchSizeSplitsRequests(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	module, err := NewHTTPRequestFromConfig(newModuleConfig(httpBatchSizeConfig(ts.URL, map[string]any{"batchSize": 50})))
	if err != nil {
		t.Fatalf("NewHTTPRequestFromConfig: %v", err)
	}

	sent, err := module.Send(context.Background(), recordsOfSize(120))
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if sent != 120 {
		t.Fatalf("sent = %d, want 120", sent)
	}
	assertCounts(t, bodyRecordCounts(t, ts.getRequests()), []int{50, 50, 20})
}

// TestHTTPRequest_BatchSizeAbsentSendsOneRequest is the non-regression guard.
func TestHTTPRequest_BatchSizeAbsentSendsOneRequest(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	module, err := NewHTTPRequestFromConfig(newModuleConfig(httpBatchSizeConfig(ts.URL, nil)))
	if err != nil {
		t.Fatalf("NewHTTPRequestFromConfig: %v", err)
	}

	sent, err := module.Send(context.Background(), recordsOfSize(120))
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if sent != 120 {
		t.Fatalf("sent = %d, want 120", sent)
	}
	assertCounts(t, bodyRecordCounts(t, ts.getRequests()), []int{120})
}

// TestHTTPRequest_BatchSizeAbsentOnErrorSkip pins the batch-mode behavior of
// onError skip when no batchSize is set: the single batch is the whole set, so
// a failure drops everything, Send reports zero sent and no error, and the
// pipeline carries on.
func TestHTTPRequest_BatchSizeAbsentOnErrorSkip(t *testing.T) {
	ts := newTestServer()
	ts.responseCode = http.StatusInternalServerError // the only request fails
	defer ts.Close()

	module, err := NewHTTPRequestFromConfig(newModuleConfig(httpBatchSizeConfig(ts.URL, map[string]any{
		"onError": "skip",
	})))
	if err != nil {
		t.Fatalf("NewHTTPRequestFromConfig: %v", err)
	}

	sent, err := module.Send(context.Background(), recordsOfSize(120))
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if sent != 0 {
		t.Fatalf("sent = %d, want 0", sent)
	}
	if got := len(ts.getRequests()); got != 1 {
		t.Fatalf("requests attempted = %d, want 1", got)
	}
}

// TestHTTPRequest_BatchSizeOnErrorFail locks partial-send accounting: the
// server fails from the second request on, so only the first batch counts and
// the third is never attempted.
func TestHTTPRequest_BatchSizeOnErrorFail(t *testing.T) {
	ts := newTestServer()
	ts.failAfter = 1
	defer ts.Close()

	module, err := NewHTTPRequestFromConfig(newModuleConfig(httpBatchSizeConfig(ts.URL, map[string]any{
		"batchSize": 50,
		"onError":   "fail",
	})))
	if err != nil {
		t.Fatalf("NewHTTPRequestFromConfig: %v", err)
	}

	sent, err := module.Send(context.Background(), recordsOfSize(120))
	if err == nil {
		t.Fatal("expected an error when a batch fails with onError fail")
	}
	if sent != 50 {
		t.Fatalf("sent = %d, want 50 (first batch only)", sent)
	}
	if got := len(ts.getRequests()); got != 2 {
		t.Fatalf("requests attempted = %d, want 2 (stop after the failure)", got)
	}
}

// TestHTTPRequest_BatchSizeOnErrorSkip locks that skip keeps going through the
// remaining batches and excludes failed ones from the sent count.
func TestHTTPRequest_BatchSizeOnErrorSkip(t *testing.T) {
	ts := newTestServer()
	ts.failAfter = 1
	defer ts.Close()

	module, err := NewHTTPRequestFromConfig(newModuleConfig(httpBatchSizeConfig(ts.URL, map[string]any{
		"batchSize": 50,
		"onError":   "skip",
	})))
	if err != nil {
		t.Fatalf("NewHTTPRequestFromConfig: %v", err)
	}

	sent, err := module.Send(context.Background(), recordsOfSize(120))
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if sent != 50 {
		t.Fatalf("sent = %d, want 50 (batches 2 and 3 failed)", sent)
	}
	if got := len(ts.getRequests()); got != 3 {
		t.Fatalf("requests attempted = %d, want 3 (skip continues)", got)
	}
}

// TestHTTPRequest_BatchSizeContextCancelled locks the per-batch cancellation
// guard: a dead context stops the loop before any request goes out, with a
// clean context.Canceled rather than a transport error.
func TestHTTPRequest_BatchSizeContextCancelled(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	module, err := NewHTTPRequestFromConfig(newModuleConfig(httpBatchSizeConfig(ts.URL, map[string]any{"batchSize": 50})))
	if err != nil {
		t.Fatalf("NewHTTPRequestFromConfig: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sent, err := module.Send(ctx, recordsOfSize(120))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if sent != 0 {
		t.Fatalf("sent = %d, want 0", sent)
	}
	if got := len(ts.getRequests()); got != 0 {
		t.Fatalf("requests attempted = %d, want 0", got)
	}
}

// TestHTTPRequest_BatchSizeRejectedInSingleMode locks the configuration guard.
func TestHTTPRequest_BatchSizeRejectedInSingleMode(t *testing.T) {
	cfg := newModuleConfig(map[string]any{
		"endpoint":    "https://example.com/x",
		"method":      "POST",
		"requestMode": "single",
		"batchSize":   50,
	})
	if _, err := NewHTTPRequestFromConfig(cfg); err == nil {
		t.Fatal("batchSize with requestMode single should be rejected at build time")
	}
}

// TestHTTPRequest_BatchSizePreviewMatchesRequests locks that a dry-run shows
// exactly the requests Send would emit.
func TestHTTPRequest_BatchSizePreviewMatchesRequests(t *testing.T) {
	module, err := NewHTTPRequestFromConfig(newModuleConfig(httpBatchSizeConfig("https://example.com/x", map[string]any{"batchSize": 50})))
	if err != nil {
		t.Fatalf("NewHTTPRequestFromConfig: %v", err)
	}

	previews, err := module.PreviewRequest(recordsOfSize(120), PreviewOptions{})
	if err != nil {
		t.Fatalf("PreviewRequest: %v", err)
	}
	want := []int{50, 50, 20}
	if len(previews) != len(want) {
		t.Fatalf("previews = %d, want %d", len(previews), len(want))
	}
	for i, preview := range previews {
		if preview.RecordCount != want[i] {
			t.Errorf("preview %d RecordCount = %d, want %d", i, preview.RecordCount, want[i])
		}
	}
}

// TestHTTPRequest_BatchSizeResolvesEndpointPerBatch pins the consequence of
// resolveEndpointForBatch using records[0]: with chunking, each batch resolves
// its templated endpoint against its own first record. This is a deliberate
// behavior, not an accident — a change here should break this test.
func TestHTTPRequest_BatchSizeResolvesEndpointPerBatch(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	module, err := NewHTTPRequestFromConfig(newModuleConfig(httpBatchSizeConfig(
		ts.URL+"/ingest/{{record.i}}",
		map[string]any{"batchSize": 2},
	)))
	if err != nil {
		t.Fatalf("NewHTTPRequestFromConfig: %v", err)
	}

	// Records carry their index as "i", so batches starting at 0, 2 and 4 must
	// produce three distinct paths.
	if _, err := module.Send(context.Background(), recordsOfSize(6)); err != nil {
		t.Fatalf("Send: %v", err)
	}

	requests := ts.getRequests()
	if len(requests) != 3 {
		t.Fatalf("requests = %d, want 3", len(requests))
	}
	wantPaths := []string{"/ingest/0?", "/ingest/2?", "/ingest/4?"}
	for i, req := range requests {
		if req.Path != wantPaths[i] {
			t.Errorf("request %d path = %q, want %q", i, req.Path, wantPaths[i])
		}
	}
}
