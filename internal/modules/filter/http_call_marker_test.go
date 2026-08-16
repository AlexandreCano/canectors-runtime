package filter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cannectors/runtime/internal/errhandling"
	"github.com/cannectors/runtime/internal/moduleconfig"
	"github.com/cannectors/runtime/pkg/connector"
)

// newMarkerTestModule builds an http_call module pointed at endpoint with the
// given onError strategy and retry policy.
func newMarkerTestModule(t *testing.T, endpoint, onError string, retry *connector.RetryConfig) *HTTPCallModule {
	t.Helper()

	config := HTTPCallConfig{
		HTTPRequestBase: moduleconfig.HTTPRequestBase{Endpoint: endpoint},
		Keys: []moduleconfig.KeyConfig{{
			Field:     "id",
			ParamType: "query",
			ParamName: "id",
		}},
		ModuleBase: connector.ModuleBase{OnError: onError},
		Retry:      retry,
	}

	module, err := NewHTTPCallFromConfig(config)
	if err != nil {
		t.Fatalf("failed to create module: %v", err)
	}
	return module
}

// firstMarker returns the single marker expected on a record, failing the test
// when the record carries none or more than one.
func firstMarker(t *testing.T, record map[string]any) map[string]any {
	t.Helper()

	markers := errhandling.RecordMarkers(record)
	if len(markers) != 1 {
		t.Fatalf("expected exactly 1 error marker, got %d (record: %v)", len(markers), record)
	}
	return markers[0]
}

// A 200 whose body is not JSON is a failure of the response, not of the
// request. The marker must not advertise the success status it came with — a
// `statusCode: 200` on a failed record slips past every `statusCode >= 400`
// condition — and must not be retryable: the same request returns the same
// unreadable body.
func TestHTTPCallMarkerOnUnparseableSuccessResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("<html>not json</html>")); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	module := newMarkerTestModule(t, server.URL, "log", nil)

	result, err := module.Process(context.Background(), []map[string]any{{"id": "1"}})
	if err != nil {
		t.Fatalf("Process returned an error: %v", err)
	}

	marker := firstMarker(t, result[0])
	if _, present := marker[errhandling.MarkerKeyStatusCode]; present {
		t.Errorf("statusCode = %v, want absent: 200 did not cause the failure", marker[errhandling.MarkerKeyStatusCode])
	}
	if got := marker[errhandling.MarkerKeyRetryable]; got != false {
		t.Errorf("retryable = %v, want false", got)
	}
	if got := marker[errhandling.MarkerKeyCategory]; got != string(errhandling.CategoryValidation) {
		t.Errorf("category = %v, want %v", got, errhandling.CategoryValidation)
	}
}

// AC1: a 400 is a functional failure — validation category, not retryable, with
// the status code reported. The record survives and keeps its own fields.
func TestHTTPCallMarkerOn400(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	module := newMarkerTestModule(t, server.URL, "log", nil)

	result, err := module.Process(context.Background(), []map[string]any{
		{"id": "123", "original": "data"},
	})
	if err != nil {
		t.Fatalf("expected no error with onError=log, got %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 record, got %d", len(result))
	}
	if result[0]["original"] != "data" {
		t.Error("record lost its own fields")
	}

	marker := firstMarker(t, result[0])

	if got := marker[errhandling.MarkerKeyCategory]; got != string(errhandling.CategoryValidation) {
		t.Errorf("category = %v, want %v", got, errhandling.CategoryValidation)
	}
	if got := marker[errhandling.MarkerKeyRetryable]; got != false {
		t.Errorf("retryable = %v, want false: a 400 must never be replayed", got)
	}
	if got := marker[errhandling.MarkerKeyStatusCode]; got != 400 {
		t.Errorf("statusCode = %v, want 400", got)
	}
	if got := marker[errhandling.MarkerKeyModule]; got != "http_call" {
		t.Errorf("module = %v, want http_call", got)
	}
	if got := marker[errhandling.MarkerKeyRecordIndex]; got != 0 {
		t.Errorf("recordIndex = %v, want 0", got)
	}
}

// AC2: a 503 is a technical failure — server category, retryable, and the
// attempt count is reported once the retry policy is exhausted.
func TestHTTPCallMarkerOn503ReportsAttempts(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	module := newMarkerTestModule(t, server.URL, "log", &connector.RetryConfig{
		MaxAttempts: 2,
		DelayMs:     1,
	})

	result, err := module.Process(context.Background(), []map[string]any{{"id": "123"}})
	if err != nil {
		t.Fatalf("expected no error with onError=log, got %v", err)
	}

	marker := firstMarker(t, result[0])

	if got := marker[errhandling.MarkerKeyCategory]; got != string(errhandling.CategoryServer) {
		t.Errorf("category = %v, want %v", got, errhandling.CategoryServer)
	}
	if got := marker[errhandling.MarkerKeyRetryable]; got != true {
		t.Errorf("retryable = %v, want true for a 503", got)
	}
	if got := marker[errhandling.MarkerKeyStatusCode]; got != 503 {
		t.Errorf("statusCode = %v, want 503", got)
	}

	attempts, ok := marker[errhandling.MarkerKeyAttempts].(int)
	if !ok {
		t.Fatalf("attempts missing from marker: %v", marker)
	}
	if attempts != calls {
		t.Errorf("attempts = %d, want %d (the number of requests the server saw)", attempts, calls)
	}
	if attempts < 2 {
		t.Errorf("attempts = %d, want at least 2 with MaxAttempts=2", attempts)
	}
}

// AC3: a connection failure is a network error and carries no status code —
// there is no HTTP response to take one from.
func TestHTTPCallMarkerOnConnectionFailureHasNoStatusCode(t *testing.T) {
	// A server closed immediately gives an address nothing listens on, which is
	// a connection error rather than an HTTP status.
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	endpoint := server.URL
	server.Close()

	module := newMarkerTestModule(t, endpoint, "log", nil)

	result, err := module.Process(context.Background(), []map[string]any{{"id": "123"}})
	if err != nil {
		t.Fatalf("expected no error with onError=log, got %v", err)
	}

	marker := firstMarker(t, result[0])

	if got := marker[errhandling.MarkerKeyCategory]; got != string(errhandling.CategoryNetwork) {
		t.Errorf("category = %v, want %v", got, errhandling.CategoryNetwork)
	}
	if got := marker[errhandling.MarkerKeyRetryable]; got != true {
		t.Errorf("retryable = %v, want true for a connection failure", got)
	}
	if _, present := marker[errhandling.MarkerKeyStatusCode]; present {
		t.Errorf("statusCode present (%v), want omitted: a connection failure has no status", marker[errhandling.MarkerKeyStatusCode])
	}
}

// AC6: the marker's message must not carry the secrets an endpoint holds in its
// query string.
func TestHTTPCallMarkerRedactsEndpointSecrets(t *testing.T) {
	const secret = "s3cr3t-api-key"

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	endpoint := server.URL + "/v1/orders?apikey=" + secret
	server.Close() // force a connection error so the endpoint lands in the message

	module := newMarkerTestModule(t, endpoint, "log", nil)

	result, err := module.Process(context.Background(), []map[string]any{{"id": "123"}})
	if err != nil {
		t.Fatalf("expected no error with onError=log, got %v", err)
	}

	marker := firstMarker(t, result[0])
	message, _ := marker[errhandling.MarkerKeyMessage].(string)

	if strings.Contains(message, secret) {
		t.Errorf("secret leaked into marker message: %q", message)
	}
}

// AC8: skip still drops the record with no trace, and fail still aborts. The
// marker must not have quietly changed either mode.
func TestHTTPCallMarkerOnlyAppliesToLogMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	t.Run("skip drops the record and leaves no marker", func(t *testing.T) {
		module := newMarkerTestModule(t, server.URL, "skip", nil)

		result, err := module.Process(context.Background(), []map[string]any{{"id": "123"}})
		if err != nil {
			t.Fatalf("expected no error with onError=skip, got %v", err)
		}
		if len(result) != 0 {
			t.Fatalf("expected the record to be dropped, got %d records: %v", len(result), result)
		}
	})

	t.Run("fail aborts without annotating", func(t *testing.T) {
		module := newMarkerTestModule(t, server.URL, "fail", nil)

		records := []map[string]any{{"id": "123"}}
		_, err := module.Process(context.Background(), records)
		if err == nil {
			t.Fatal("expected an error with onError=fail")
		}
		if errhandling.HasMarkers(records[0]) {
			t.Error("fail mode annotated the record; markers belong to log mode only")
		}
	})
}

// AC5: a record failing in two successive http_call filters carries both
// markers, in the order they happened.
func TestHTTPCallMarkersAccumulateAcrossFilters(t *testing.T) {
	badRequest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer badRequest.Close()

	unavailable := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer unavailable.Close()

	first := newMarkerTestModule(t, badRequest.URL, "log", nil)
	second := newMarkerTestModule(t, unavailable.URL, "log", nil)

	ctx := context.Background()
	afterFirst, err := first.Process(ctx, []map[string]any{{"id": "123"}})
	if err != nil {
		t.Fatalf("first filter returned an error: %v", err)
	}
	afterSecond, err := second.Process(ctx, afterFirst)
	if err != nil {
		t.Fatalf("second filter returned an error: %v", err)
	}

	markers := errhandling.RecordMarkers(afterSecond[0])
	if len(markers) != 2 {
		t.Fatalf("expected 2 markers, got %d: %v", len(markers), markers)
	}
	if got := markers[0][errhandling.MarkerKeyStatusCode]; got != 400 {
		t.Errorf("first marker statusCode = %v, want 400", got)
	}
	if got := markers[1][errhandling.MarkerKeyStatusCode]; got != 503 {
		t.Errorf("second marker statusCode = %v, want 503", got)
	}
}

// AC10: an API that answers 404 for "no match" makes that an ordinary
// functional outcome, and the pipeline must be able to say so.
func TestHTTPCallMarkerHonorsClassificationOverride(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	build := func(t *testing.T, override *moduleconfig.ErrorClassificationConfig) *HTTPCallModule {
		t.Helper()
		module, err := NewHTTPCallFromConfig(HTTPCallConfig{
			HTTPRequestBase: moduleconfig.HTTPRequestBase{Endpoint: server.URL},
			Keys: []moduleconfig.KeyConfig{{
				Field: "id", ParamType: "query", ParamName: "id",
			}},
			ModuleBase:          connector.ModuleBase{OnError: "log"},
			ErrorClassification: override,
		})
		if err != nil {
			t.Fatalf("failed to create module: %v", err)
		}
		return module
	}

	t.Run("without override a 404 keeps the default category", func(t *testing.T) {
		result, err := build(t, nil).Process(context.Background(), []map[string]any{{"id": "1"}})
		if err != nil {
			t.Fatalf("Process returned an error: %v", err)
		}
		marker := firstMarker(t, result[0])
		if got := marker[errhandling.MarkerKeyCategory]; got != string(errhandling.CategoryNotFound) {
			t.Errorf("category = %v, want %v", got, errhandling.CategoryNotFound)
		}
	})

	// The override decides retryability; it does not flatten a category that
	// already agrees with it. A 404 declared functional stays a not_found, which
	// is the detail the operator reads in the counters.
	t.Run("declared functional, a 404 stays not_found and non-retryable", func(t *testing.T) {
		override := &moduleconfig.ErrorClassificationConfig{Functional: []int{404}}
		result, err := build(t, override).Process(context.Background(), []map[string]any{{"id": "1"}})
		if err != nil {
			t.Fatalf("Process returned an error: %v", err)
		}
		marker := firstMarker(t, result[0])
		if got := marker[errhandling.MarkerKeyCategory]; got != string(errhandling.CategoryNotFound) {
			t.Errorf("category = %v, want %v", got, errhandling.CategoryNotFound)
		}
		if got := marker[errhandling.MarkerKeyRetryable]; got != false {
			t.Errorf("retryable = %v, want false", got)
		}
	})

	// A 503 declared functional is the opposite case: "server" contradicts a
	// non-retryable verdict, so the category has to move.
	t.Run("declared functional, a technical category is rewritten", func(t *testing.T) {
		serverErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer serverErr.Close()

		module, err := NewHTTPCallFromConfig(HTTPCallConfig{
			HTTPRequestBase:     moduleconfig.HTTPRequestBase{Endpoint: serverErr.URL},
			Keys:                []moduleconfig.KeyConfig{{Field: "id", ParamType: "query", ParamName: "id"}},
			ModuleBase:          connector.ModuleBase{OnError: "log"},
			ErrorClassification: &moduleconfig.ErrorClassificationConfig{Functional: []int{503}},
		})
		if err != nil {
			t.Fatalf("failed to create module: %v", err)
		}

		result, err := module.Process(context.Background(), []map[string]any{{"id": "1"}})
		if err != nil {
			t.Fatalf("Process returned an error: %v", err)
		}
		marker := firstMarker(t, result[0])
		if got := marker[errhandling.MarkerKeyCategory]; got != string(errhandling.CategoryValidation) {
			t.Errorf("category = %v, want %v", got, errhandling.CategoryValidation)
		}
		if got := marker[errhandling.MarkerKeyRetryable]; got != false {
			t.Errorf("retryable = %v, want false", got)
		}
	})

	t.Run("declared technical, a 404 becomes retryable", func(t *testing.T) {
		override := &moduleconfig.ErrorClassificationConfig{Technical: []int{404}}
		result, err := build(t, override).Process(context.Background(), []map[string]any{{"id": "1"}})
		if err != nil {
			t.Fatalf("Process returned an error: %v", err)
		}
		marker := firstMarker(t, result[0])
		if got := marker[errhandling.MarkerKeyRetryable]; got != true {
			t.Errorf("retryable = %v, want true", got)
		}
		if got := marker[errhandling.MarkerKeyCategory]; got != string(errhandling.CategoryServer) {
			t.Errorf("category = %v, want %v", got, errhandling.CategoryServer)
		}
	})
}
