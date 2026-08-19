package output

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/cannectors/runtime/internal/soapclient"
	"github.com/cannectors/runtime/pkg/connector"
)

func TestSOAPRequest_SendSingle(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `<Name>Alice</Name>`) && !strings.Contains(string(body), `<Name>Bob</Name>`) {
			t.Fatalf("request body missing templated name:\n%s", body)
		}
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(`<Envelope><Body><SubmitResponse><ok>true</ok></SubmitResponse></Body></Envelope>`))
	}))
	defer server.Close()

	raw, err := json.Marshal(map[string]any{
		"endpoint":    server.URL,
		"operation":   "Submit",
		"body":        `<Submit><Name>{{record.name}}</Name></Submit>`,
		"requestMode": "single",
	})
	if err != nil {
		t.Fatal(err)
	}
	module, err := NewSOAPRequestFromConfig(&connector.ModuleConfig{Type: "soapRequest", Raw: raw})
	if err != nil {
		t.Fatalf("NewSOAPRequestFromConfig: %v", err)
	}
	defer func() { _ = module.Close() }()

	sent, err := module.Send(context.Background(), []map[string]any{
		{"name": "Alice"},
		{"name": "Bob"},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if sent != 2 {
		t.Fatalf("sent = %d, want 2", sent)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("calls = %d, want 2", got)
	}
}

func TestSOAPRequest_PreviewBatch(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"endpoint":    "https://example.com/soap",
		"operation":   "SubmitBatch",
		"body":        `<SubmitBatch><First>{{record.records[0].name}}</First></SubmitBatch>`,
		"requestMode": "batch",
	})
	if err != nil {
		t.Fatal(err)
	}
	module, err := NewSOAPRequestFromConfig(&connector.ModuleConfig{Type: "soapRequest", Raw: raw})
	if err != nil {
		t.Fatalf("NewSOAPRequestFromConfig: %v", err)
	}
	defer func() { _ = module.Close() }()

	previews, err := module.PreviewOperations([]map[string]any{{"name": "Alice"}}, PreviewOptions{})
	if err != nil {
		t.Fatalf("PreviewOperations: %v", err)
	}
	if len(previews) != 1 {
		t.Fatalf("expected one preview, got %d", len(previews))
	}
	if previews[0].HTTP.Method != "POST" || previews[0].RecordCount != 1 {
		t.Fatalf("unexpected preview metadata: %#v", previews[0])
	}
	if got := previews[0].HTTP.Headers["Content-Type"]; got != `text/xml; charset=utf-8` {
		t.Fatalf("Content-Type = %q", got)
	}
	if !strings.Contains(previews[0].HTTP.BodyPreview, `<First>Alice</First>`) {
		t.Fatalf("unexpected body preview:\n%s", previews[0].HTTP.BodyPreview)
	}
}

func TestSOAPRequest_SendBatch(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `<First>Alice</First>`) {
			t.Fatalf("request body missing batch data:\n%s", body)
		}
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(`<Envelope><Body><SubmitResponse><ok>true</ok></SubmitResponse></Body></Envelope>`))
	}))
	defer server.Close()

	module := newSOAPRequestTestModule(t, map[string]any{
		"endpoint":    server.URL,
		"operation":   "SubmitBatch",
		"body":        `<SubmitBatch><First>{{record.records[0].name}}</First></SubmitBatch>`,
		"requestMode": "batch",
	})
	defer func() { _ = module.Close() }()

	sent, err := module.Send(context.Background(), []map[string]any{{"name": "Alice"}, {"name": "Bob"}})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if sent != 2 || atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("sent=%d calls=%d", sent, atomic.LoadInt32(&calls))
	}
}

func TestSOAPRequest_OnErrorSkipAndLog(t *testing.T) {
	for _, tt := range []struct {
		name    string
		onError string
	}{
		{name: "skip", onError: "skip"},
		{name: "log", onError: "log"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if atomic.AddInt32(&calls, 1) == 1 {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`broken`))
					return
				}
				w.Header().Set("Content-Type", "text/xml")
				_, _ = w.Write([]byte(`<Envelope><Body><SubmitResponse><ok>true</ok></SubmitResponse></Body></Envelope>`))
			}))
			defer server.Close()

			module := newSOAPRequestTestModule(t, map[string]any{
				"endpoint":    server.URL,
				"operation":   "Submit",
				"body":        `<Submit/>`,
				"requestMode": "single",
				"onError":     tt.onError,
				"retry":       map[string]any{"maxAttempts": 0, "delayMs": 1, "backoffMultiplier": 1, "maxDelayMs": 1},
			})
			defer func() { _ = module.Close() }()
			sent, err := module.Send(context.Background(), []map[string]any{{"id": "1"}, {"id": "2"}})
			if err != nil {
				t.Fatalf("Send: %v", err)
			}
			if sent != 1 {
				t.Fatalf("sent = %d, want 1", sent)
			}
		})
	}
}

func TestSOAPRequest_MTOMBase64Attachment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, attachments, multipartRequest, err := soapclient.ParseMTOMResponse(r.Header.Get("Content-Type"), r.Body)
		if err != nil {
			t.Fatalf("ParseMTOMResponse: %v", err)
		}
		if !multipartRequest {
			t.Fatal("expected multipart request")
		}
		if got := string(attachments["doc-1"].Data); got != "pdf-bytes" {
			t.Fatalf("attachment data = %q", got)
		}
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(`<Envelope><Body><UploadResponse><ok>true</ok></UploadResponse></Body></Envelope>`))
	}))
	defer server.Close()

	module := newSOAPRequestTestModule(t, map[string]any{
		"endpoint":    server.URL,
		"operation":   "Upload",
		"requestMode": "single",
		"body":        `<Upload><File><xop:Include xmlns:xop="http://www.w3.org/2004/08/xop/include" href="cid:doc-1"/></File></Upload>`,
		"mtom": map[string]any{
			"enabled": true,
			"attachments": []any{map[string]any{
				"contentId":   "doc-1",
				"contentType": "application/pdf",
				"sourceField": "document",
				"encoding":    "base64",
			}},
		},
	})
	defer func() { _ = module.Close() }()
	sent, err := module.Send(context.Background(), []map[string]any{{"document": base64.StdEncoding.EncodeToString([]byte("pdf-bytes"))}})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if sent != 1 {
		t.Fatalf("sent = %d", sent)
	}
}

func TestSOAPRequest_WSSecurityAndFault(t *testing.T) {
	t.Run("ws-security header", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			for _, want := range []string{"Security", "alice", "#PasswordText"} {
				if !strings.Contains(string(body), want) {
					t.Fatalf("request missing %q:\n%s", want, body)
				}
			}
			w.Header().Set("Content-Type", "text/xml")
			_, _ = w.Write([]byte(`<Envelope><Body><SubmitResponse><ok>true</ok></SubmitResponse></Body></Envelope>`))
		}))
		defer server.Close()
		module := newSOAPRequestTestModule(t, map[string]any{
			"endpoint":   server.URL,
			"operation":  "Submit",
			"body":       `<Submit/>`,
			"wsSecurity": map[string]any{"username": "alice", "password": "secret", "passwordType": "PasswordText"},
		})
		defer func() { _ = module.Close() }()
		if _, err := module.Send(context.Background(), []map[string]any{{"id": "1"}}); err != nil {
			t.Fatalf("Send: %v", err)
		}
	})

	t.Run("fault", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/xml")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><soap:Fault><faultcode>soap:Server</faultcode><faultstring>Transient</faultstring></soap:Fault></soap:Body></soap:Envelope>`))
		}))
		defer server.Close()
		module := newSOAPRequestTestModule(t, map[string]any{
			"endpoint":  server.URL,
			"operation": "Submit",
			"body":      `<Submit/>`,
		})
		defer func() { _ = module.Close() }()
		_, err := module.Send(context.Background(), []map[string]any{{"id": "1"}})
		var fault *soapclient.SOAPFaultError
		if !errors.As(err, &fault) {
			t.Fatalf("expected SOAPFaultError, got %T: %v", err, err)
		}
	})
}

func TestSOAPRequest_RetryInfoAndPreviewSOAP12Headers(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`temporary`))
			return
		}
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(`<Envelope><Body><SubmitResponse><ok>true</ok></SubmitResponse></Body></Envelope>`))
	}))
	defer server.Close()

	module := newSOAPRequestTestModule(t, map[string]any{
		"endpoint":    server.URL,
		"operation":   "Submit",
		"soapVersion": "1.2",
		"soapAction":  "urn:Submit",
		"body":        `<Submit/>`,
		"retry":       map[string]any{"maxAttempts": 1, "delayMs": 1, "backoffMultiplier": 1, "maxDelayMs": 1, "retryableStatusCodes": []any{500}},
	})
	defer func() { _ = module.Close() }()
	if _, err := module.Send(context.Background(), []map[string]any{{"id": "1"}}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if info := module.GetRetryInfo(); info == nil || info.TotalAttempts != 2 || info.RetryCount != 1 {
		t.Fatalf("retry info = %#v", info)
	}
	previews, err := module.PreviewOperations([]map[string]any{{"id": "1"}}, PreviewOptions{})
	if err != nil {
		t.Fatalf("PreviewOperations: %v", err)
	}
	contentType := previews[0].HTTP.Headers["Content-Type"]
	if !strings.HasPrefix(contentType, "application/soap+xml") || !strings.Contains(contentType, `action="urn:Submit"`) {
		t.Fatalf("SOAP 1.2 Content-Type = %q", contentType)
	}
	if _, ok := previews[0].HTTP.Headers["SOAPAction"]; ok {
		t.Fatalf("SOAPAction should be absent for SOAP 1.2 preview: %#v", previews[0].HTTP.Headers)
	}
}

func newSOAPRequestTestModule(t *testing.T, cfg map[string]any) *SOAPRequestModule {
	t.Helper()
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	module, err := NewSOAPRequestFromConfig(&connector.ModuleConfig{Type: "soapRequest", Raw: raw})
	if err != nil {
		t.Fatalf("NewSOAPRequestFromConfig: %v", err)
	}
	return module
}

// soapBatchSizeServer records the recordCount reported by each request body so
// tests can assert on batch boundaries. failBatch, when >= 0, makes the request
// at that index fail with a 500.
func soapBatchSizeServer(t *testing.T, failBatch int) (*httptest.Server, *[]int) {
	t.Helper()
	counts := make([]int, 0, 4)
	var mu sync.Mutex
	re := regexp.MustCompile(`<Count>(\d+)</Count>`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		match := re.FindSubmatch(body)
		if match == nil {
			t.Errorf("request body missing <Count>:\n%s", body)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		count, err := strconv.Atoi(string(match[1]))
		if err != nil {
			t.Errorf("parsing count: %v", err)
		}
		mu.Lock()
		index := len(counts)
		counts = append(counts, count)
		mu.Unlock()

		if index == failBatch {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`boom`))
			return
		}
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(`<Envelope><Body><SubmitResponse><ok>true</ok></SubmitResponse></Body></Envelope>`))
	}))
	return server, &counts
}

func soapBatchSizeConfig(endpoint string, extra map[string]any) map[string]any {
	cfg := map[string]any{
		"endpoint":    endpoint,
		"operation":   "SubmitBatch",
		"body":        `<SubmitBatch><Count>{{record.recordCount}}</Count></SubmitBatch>`,
		"requestMode": "batch",
		"retry":       map[string]any{"maxAttempts": 0},
	}
	for k, v := range extra {
		cfg[k] = v
	}
	return cfg
}

// TestSOAPRequest_BatchSizeSplitsRequests locks the core contract: 120 records
// with batchSize 50 produce three requests of 50/50/20, and each body sees the
// recordCount of its own batch.
func TestSOAPRequest_BatchSizeSplitsRequests(t *testing.T) {
	server, counts := soapBatchSizeServer(t, -1)
	defer server.Close()

	module := newSOAPRequestTestModule(t, soapBatchSizeConfig(server.URL, map[string]any{"batchSize": 50}))
	defer func() { _ = module.Close() }()

	sent, err := module.Send(context.Background(), recordsOfSize(120))
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if sent != 120 {
		t.Fatalf("sent = %d, want 120", sent)
	}
	if want := []int{50, 50, 20}; !slices.Equal(*counts, want) {
		t.Fatalf("batch record counts = %v, want %v", *counts, want)
	}
}

// TestSOAPRequest_BatchSizeAbsentSendsOneRequest is the non-regression guard:
// without batchSize the whole set still goes out in a single request.
func TestSOAPRequest_BatchSizeAbsentSendsOneRequest(t *testing.T) {
	server, counts := soapBatchSizeServer(t, -1)
	defer server.Close()

	module := newSOAPRequestTestModule(t, soapBatchSizeConfig(server.URL, nil))
	defer func() { _ = module.Close() }()

	sent, err := module.Send(context.Background(), recordsOfSize(120))
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if sent != 120 {
		t.Fatalf("sent = %d, want 120", sent)
	}
	if want := []int{120}; !slices.Equal(*counts, want) {
		t.Fatalf("batch record counts = %v, want %v", *counts, want)
	}
}

// TestSOAPRequest_BatchSizeOnErrorFail locks partial-send accounting: the
// second batch fails, so the first batch's records count as sent and the rest
// are abandoned.
func TestSOAPRequest_BatchSizeOnErrorFail(t *testing.T) {
	server, counts := soapBatchSizeServer(t, 1)
	defer server.Close()

	module := newSOAPRequestTestModule(t, soapBatchSizeConfig(server.URL, map[string]any{
		"batchSize": 50,
		"onError":   "fail",
	}))
	defer func() { _ = module.Close() }()

	sent, err := module.Send(context.Background(), recordsOfSize(120))
	if err == nil {
		t.Fatal("expected an error when a batch fails with onError fail")
	}
	if !strings.Contains(err.Error(), "batch 1") {
		t.Errorf("error should name the failing batch, got %q", err)
	}
	if sent != 50 {
		t.Fatalf("sent = %d, want 50 (first batch only)", sent)
	}
	if len(*counts) != 2 {
		t.Fatalf("requests attempted = %d, want 2 (stop after the failure)", len(*counts))
	}
}

// TestSOAPRequest_BatchSizeOnErrorSkip locks that skip keeps going through the
// remaining batches and excludes the failed batch from the sent count.
func TestSOAPRequest_BatchSizeOnErrorSkip(t *testing.T) {
	server, counts := soapBatchSizeServer(t, 1)
	defer server.Close()

	module := newSOAPRequestTestModule(t, soapBatchSizeConfig(server.URL, map[string]any{
		"batchSize": 50,
		"onError":   "skip",
	}))
	defer func() { _ = module.Close() }()

	sent, err := module.Send(context.Background(), recordsOfSize(120))
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if sent != 70 {
		t.Fatalf("sent = %d, want 70 (120 minus the skipped batch of 50)", sent)
	}
	if len(*counts) != 3 {
		t.Fatalf("requests attempted = %d, want 3", len(*counts))
	}
}

// TestSOAPRequest_BatchSizeContextCancelled locks the per-batch cancellation
// guard: a dead context stops the loop before any request goes out, with a
// clean context.Canceled rather than a transport error.
//
// Canceling from the server handler instead would race with the in-flight
// request the handler is answering, so the partial-send accounting is covered
// by TestSOAPRequest_BatchSizeOnErrorFail, not here.
func TestSOAPRequest_BatchSizeContextCancelled(t *testing.T) {
	server, counts := soapBatchSizeServer(t, -1)
	defer server.Close()

	module := newSOAPRequestTestModule(t, soapBatchSizeConfig(server.URL, map[string]any{"batchSize": 50}))
	defer func() { _ = module.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sent, err := module.Send(ctx, recordsOfSize(120))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if sent != 0 {
		t.Fatalf("sent = %d, want 0", sent)
	}
	if len(*counts) != 0 {
		t.Fatalf("requests attempted = %d, want 0", len(*counts))
	}
}

// TestSOAPRequest_BatchSizeAbsentOnErrorSkip pins the batch-mode behavior of
// onError skip when no batchSize is set: the single batch is the whole set, so
// a failure drops everything, Send reports zero sent and no error, and the
// pipeline carries on.
func TestSOAPRequest_BatchSizeAbsentOnErrorSkip(t *testing.T) {
	server, counts := soapBatchSizeServer(t, 0) // the only request fails
	defer server.Close()

	module := newSOAPRequestTestModule(t, soapBatchSizeConfig(server.URL, map[string]any{"onError": "skip"}))
	defer func() { _ = module.Close() }()

	sent, err := module.Send(context.Background(), recordsOfSize(120))
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if sent != 0 {
		t.Fatalf("sent = %d, want 0", sent)
	}
	if len(*counts) != 1 {
		t.Fatalf("requests attempted = %d, want 1", len(*counts))
	}
}

// TestSOAPRequest_BatchSizeRejectedInSingleMode locks the configuration guard.
func TestSOAPRequest_BatchSizeRejectedInSingleMode(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"endpoint":    "https://example.com/soap",
		"operation":   "Op",
		"body":        "<Op/>",
		"requestMode": "single",
		"batchSize":   50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewSOAPRequestFromConfig(&connector.ModuleConfig{Type: "soapRequest", Raw: raw}); err == nil {
		t.Fatal("batchSize with requestMode single should be rejected at build time")
	}
}

// TestSOAPRequest_BatchSizePreviewMatchesRequests locks that a dry-run shows
// exactly the requests Send would emit.
func TestSOAPRequest_BatchSizePreviewMatchesRequests(t *testing.T) {
	module := newSOAPRequestTestModule(t, soapBatchSizeConfig("https://example.com/soap", map[string]any{"batchSize": 50}))
	defer func() { _ = module.Close() }()

	previews, err := module.PreviewOperations(recordsOfSize(120), PreviewOptions{})
	if err != nil {
		t.Fatalf("PreviewOperations: %v", err)
	}
	if len(previews) != 3 {
		t.Fatalf("previews = %d, want 3", len(previews))
	}
	wantCounts := []int{50, 50, 20}
	for i, preview := range previews {
		if preview.RecordCount != wantCounts[i] {
			t.Errorf("preview %d RecordCount = %d, want %d", i, preview.RecordCount, wantCounts[i])
		}
		want := fmt.Sprintf("<Count>%d</Count>", wantCounts[i])
		if !strings.Contains(preview.HTTP.BodyPreview, want) {
			t.Errorf("preview %d body missing %s:\n%s", i, want, preview.HTTP.BodyPreview)
		}
	}
}
