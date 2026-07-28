// Package output provides implementations for output modules.
// Output modules are responsible for sending data to destination systems.
package output

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/expr-lang/expr/vm"

	"github.com/cannectors/runtime/internal/auth"
	"github.com/cannectors/runtime/internal/errhandling"
	"github.com/cannectors/runtime/internal/httpclient"
	"github.com/cannectors/runtime/internal/logger"
	"github.com/cannectors/runtime/internal/metadata"
	"github.com/cannectors/runtime/internal/moduleconfig"
	"github.com/cannectors/runtime/internal/template"
	"github.com/cannectors/runtime/pkg/connector"
)

// Default configuration values for HTTP Request module
const (
	defaultHTTPTimeout = 30 * time.Second
	defaultUserAgent   = "Cannectors-Runtime/1.0"
	defaultContentType = "application/json"
)

// HTTP header names
const (
	headerUserAgent   = "User-Agent"
	headerContentType = "Content-Type"
)

// Authentication type constants
const (
	authTypeAPIKey      = "api-key"
	defaultAPIKeyHeader = "X-API-Key"
)

// Error types for HTTP request output module
var (
	ErrNilConfig       = errors.New("module configuration is nil")
	ErrMissingEndpoint = errors.New("endpoint is required in module configuration")
	ErrJSONMarshal     = errors.New("failed to marshal records to JSON")
)

// keyEntry holds parsed key config for request building (internal use).
type keyEntry struct {
	field     string
	paramType string
	paramName string
}

// RequestConfig holds request-specific configuration
type RequestConfig struct {
	RequestMode      string            // "batch" or "single"
	BatchSize        int               // Max records per request in batch mode (0 = one request for all)
	Keys             []keyEntry        // Dynamic params from record (path, query, header)
	QueryParams      map[string]string // Static query parameters
	BodyTemplateFile string            // Path to external template file for request body
	bodyTemplateRaw  string            // Loaded template content (internal use)
}

// HTTPRequestModule implements HTTP-based data sending.
// It sends transformed records to a target REST API via HTTP requests.
type HTTPRequestModule struct {
	endpoint             string
	method               string
	headers              map[string]string
	timeout              time.Duration
	request              RequestConfig
	retry                connector.RetryConfig
	authHandler          auth.Handler
	client               *httpclient.Client
	onError              errhandling.OnErrorStrategy // "fail", "skip", "log"
	successCodes         []int                       // HTTP status codes considered success (nil = expression-only)
	successProgram       *vm.Program                 // Compiled success.expression (nil = no expression)
	successExpression    string                      // Raw expression for diagnostics
	lastRetryInfo        *connector.RetryInfo
	retryHintProgram     *vm.Program      // Compiled expr program for retryHintFromBody
	engine               *template.Engine // Jinja templating engine (shared compile cache)
	bodyTarget           template.Target  // escaping target for body templates (JSON vs raw text)
	contentTypeTemplated bool             // True when the Content-Type header is itself a template
	defaultBodyDisabled  bool             // True when method has no default JSON body and no body is configured
}

// HTTPRequestOutputConfig holds typed configuration for the HTTP request output module.
type HTTPRequestOutputConfig struct {
	connector.ModuleBase
	moduleconfig.HTTPRequestBase
	RequestMode string                   `json:"requestMode,omitempty"`
	BatchSize   int                      `json:"batchSize,omitempty"`
	Keys        []moduleconfig.KeyConfig `json:"keys,omitempty"`
	Success     *SuccessConditionConfig  `json:"success,omitempty"`
	Retry       *connector.RetryConfig   `json:"retry,omitempty"`
}

// SuccessConditionConfig holds the success condition for an HTTP response.
// When both Expression and StatusCodes are provided, both must be true.
// When only Expression is provided, it alone determines success.
// When only StatusCodes is provided, status code matching alone determines success.
// When neither is provided, defaultSuccessCodes apply.
type SuccessConditionConfig struct {
	Expression  string `json:"expression,omitempty"`
	StatusCodes []int  `json:"statusCodes,omitempty"`
}

// Default HTTP method when none is configured (Story 24.12 AC1).
const defaultHTTPMethod = "POST"

// Default success status codes (Story 24.12 AC13).
var defaultSuccessCodes = []int{200, 201, 202, 203, 204}

// Methods that, in the absence of an explicit body, default to sending the
// record as JSON (Story 24.12 AC4). Other methods (GET, DELETE, HEAD, ...)
// send no default body (AC5).
var methodsWithDefaultJSONBody = map[string]struct{}{
	"POST":  {},
	"PUT":   {},
	"PATCH": {},
}

func methodHasDefaultJSONBody(method string) bool {
	_, ok := methodsWithDefaultJSONBody[method]
	return ok
}

// NewHTTPRequestFromConfig creates a new HTTP request output module from configuration.
// This is the primary constructor that parses ModuleConfig and creates a ready-to-use module.
//
// Required config fields:
//   - endpoint: The target HTTP endpoint URL
//   - method: HTTP method (POST, PUT, PATCH)
//
// Optional config fields:
//   - headers: Custom HTTP headers (map[string]string)
//   - timeoutMs: Request timeout in milliseconds (default 30000)
//   - requestMode, keys, queryParams, bodyTemplateFile
//   - onError: Error handling mode ("fail", "skip", "log")
func NewHTTPRequestFromConfig(config *connector.ModuleConfig) (*HTTPRequestModule, error) {
	if config == nil {
		return nil, ErrNilConfig
	}

	cfg, err := moduleconfig.ParseModuleConfig[HTTPRequestOutputConfig](*config)
	if err != nil {
		return nil, err
	}
	if cfg.Endpoint == "" {
		return nil, ErrMissingEndpoint
	}
	methodInput := cfg.Method
	if methodInput == "" {
		methodInput = defaultHTTPMethod
	}
	method, err := httpclient.NormalizeAndValidateMethod(methodInput)
	if err != nil {
		return nil, err
	}

	timeout := connector.GetTimeoutDuration(cfg.TimeoutMs, defaultHTTPTimeout)
	headers := cfg.Headers
	requestMode, err := normalizeRequestMode(cfg.RequestMode, "httpRequest")
	if err != nil {
		return nil, err
	}
	batchSize, err := normalizeBatchSize(cfg.BatchSize, requestMode, "httpRequest")
	if err != nil {
		return nil, err
	}
	if keyErr := validateRequestKeyEntries(cfg.Keys); keyErr != nil {
		return nil, keyErr
	}
	reqConfig := RequestConfig{
		RequestMode:      requestMode,
		BatchSize:        batchSize,
		QueryParams:      cfg.QueryParams,
		BodyTemplateFile: cfg.BodyTemplateFile,
		bodyTemplateRaw:  cfg.Body,
	}
	reqConfig.Keys = make([]keyEntry, len(cfg.Keys))
	for i, k := range cfg.Keys {
		reqConfig.Keys[i] = keyEntry{field: k.Field, paramType: k.ParamType, paramName: k.ParamName}
	}

	onError, err := errhandling.ParseOnErrorStrategy(cfg.OnError)
	if err != nil {
		return nil, err
	}

	successCodes, successProgram, successExpression, err := buildSuccessCondition(cfg.Success)
	if err != nil {
		return nil, err
	}

	retryConfig := moduleconfig.ToRetryConfig(cfg.Retry)
	if vErr := retryConfig.Validate(); vErr != nil {
		return nil, fmt.Errorf("httpRequest retry config invalid: %w", vErr)
	}

	client := httpclient.NewClient(timeout)

	// Create authentication handler if configured
	authHandler, err := auth.NewHandler(cfg.Authentication, client.Client)
	if err != nil {
		return nil, fmt.Errorf("creating auth handler: %w", err)
	}

	retryHintProgram, err := httpclient.CompileRetryHint(retryConfig.RetryHintFromBody)
	if err != nil {
		return nil, err
	}

	engine := templateEngine

	// Body templates escape substituted values for JSON when the Content-Type is
	// JSON, keeping the payload valid; otherwise values pass through as raw text.
	// A templated Content-Type is only known per record, so it is resolved at
	// send time (bodyTargetFor) and JSON escaping is assumed here.
	contentTypeTemplated := template.HasVariables(headers[headerContentType])
	bodyTarget := template.TargetText
	if contentTypeTemplated || jsonContentTypeFromHeaders(headers) {
		bodyTarget = template.TargetJSON
	}

	// Validate (= compile) endpoint and header templates for their targets.
	if err := validateTemplateConfig(engine, cfg.Endpoint, headers); err != nil {
		return nil, fmt.Errorf("validating template configuration: %w", err)
	}

	if reqConfig.bodyTemplateRaw != "" {
		if err := engine.Validate(reqConfig.bodyTemplateRaw, bodyTarget, false); err != nil {
			return nil, fmt.Errorf("invalid template syntax in body: %w", err)
		}
	} else if reqConfig.BodyTemplateFile != "" {
		templateContent, err := os.ReadFile(reqConfig.BodyTemplateFile)
		if err != nil {
			return nil, fmt.Errorf("loading body template file %q: %w", reqConfig.BodyTemplateFile, err)
		}
		reqConfig.bodyTemplateRaw = string(templateContent)

		if err := engine.Validate(reqConfig.bodyTemplateRaw, bodyTarget, false); err != nil {
			return nil, fmt.Errorf("invalid template syntax in %q: %w", reqConfig.BodyTemplateFile, err)
		}

		logger.Debug("loaded body template file",
			slog.String("file", reqConfig.BodyTemplateFile),
			slog.Int("size", len(templateContent)),
		)
	}

	module := &HTTPRequestModule{
		endpoint:             cfg.Endpoint,
		method:               method,
		headers:              headers,
		timeout:              timeout,
		request:              reqConfig,
		retry:                retryConfig,
		authHandler:          authHandler,
		client:               client,
		onError:              onError,
		successCodes:         successCodes,
		successProgram:       successProgram,
		successExpression:    successExpression,
		retryHintProgram:     retryHintProgram,
		engine:               engine,
		bodyTarget:           bodyTarget,
		contentTypeTemplated: contentTypeTemplated,
		defaultBodyDisabled:  reqConfig.bodyTemplateRaw == "" && !methodHasDefaultJSONBody(method),
	}

	// Check if endpoint/headers use templating
	hasTemplating := template.HasVariables(cfg.Endpoint)
	for _, v := range headers {
		if template.HasVariables(v) {
			hasTemplating = true
			break
		}
	}

	logger.Debug("http request output module created",
		slog.String("endpoint", httpclient.SanitizeURL(cfg.Endpoint)),
		slog.String("method", method),
		slog.String("timeout", timeout.String()),
		slog.Bool("has_auth", authHandler != nil),
		slog.String("request_mode", reqConfig.RequestMode),
		slog.Bool("has_templating", hasTemplating),
		slog.String("body_template_file", reqConfig.BodyTemplateFile),
	)

	return module, nil
}

// validateTemplateConfig compiles the endpoint and header templates for their
// respective targets, surfacing any syntax error at construction time.
func validateTemplateConfig(engine *template.Engine, endpoint string, headers map[string]string) error {
	if err := engine.Validate(endpoint, template.TargetURL, false); err != nil {
		return fmt.Errorf("invalid endpoint template: %w", err)
	}
	for name, value := range headers {
		if err := engine.Validate(value, template.TargetText, false); err != nil {
			return fmt.Errorf("invalid template in header %q: %w", name, err)
		}
	}
	return nil
}

// Send transmits records to the destination via HTTP.
// Returns the number of records successfully sent and any error encountered.
//
// The context can be used to cancel long-running operations.
//
// Behavior depends on requestMode configuration:
//   - "batch" (default): Sends all records in a single request as JSON array
//   - "single": Sends one request per record, body is single JSON object
//
// Empty or nil records return success with 0 sent.
func (h *HTTPRequestModule) Send(ctx context.Context, records []map[string]any) (int, error) {
	startTime := time.Now()

	// Handle empty/nil records gracefully
	if len(records) == 0 {
		logger.Debug("no records to send, returning success",
			slog.String("module_type", "httpRequest"),
			slog.String("endpoint", httpclient.SanitizeURL(h.endpoint)),
		)
		return 0, nil
	}

	logger.Info("output send started",
		slog.String("module_type", "httpRequest"),
		slog.String("endpoint", httpclient.SanitizeURL(h.endpoint)),
		slog.String("method", h.method),
		slog.Int("record_count", len(records)),
		slog.String("request_mode", h.request.RequestMode),
		slog.Int("batch_size", h.request.BatchSize),
		slog.Bool("has_auth", h.authHandler != nil),
	)

	var sent int
	var err error

	// Choose send mode based on configuration
	if h.request.RequestMode == requestModeSingle {
		sent, err = h.sendSingleRecordMode(ctx, records)
	} else {
		sent, err = h.sendBatchMode(ctx, records)
	}

	duration := time.Since(startTime)

	if err != nil {
		logger.Error("output send failed",
			slog.String("module_type", "httpRequest"),
			slog.String("endpoint", httpclient.SanitizeURL(h.endpoint)),
			slog.String("method", h.method),
			slog.Int("records_sent", sent),
			slog.Int("records_failed", len(records)-sent),
			slog.Duration("duration", duration),
			slog.String("error", err.Error()),
		)
		return sent, err
	}

	logger.Info("output send completed",
		slog.String("module_type", "httpRequest"),
		slog.String("endpoint", httpclient.SanitizeURL(h.endpoint)),
		slog.String("method", h.method),
		slog.Int("records_sent", sent),
		slog.Int("records_failed", len(records)-sent),
		slog.Duration("duration", duration),
	)

	return sent, nil
}

// sendBatchMode groups records into requests. Without batchSize that is one
// request holding every record (the historical behavior); with it, one request
// per batch of at most batchSize records. Body, endpoint and headers are all
// resolved per batch, so a templated endpoint or header resolves against the
// first record of its own batch.
func (h *HTTPRequestModule) sendBatchMode(ctx context.Context, records []map[string]any) (int, error) {
	batches := chunkRecords(records, h.request.BatchSize)
	sent := 0
	for i, batch := range batches {
		select {
		case <-ctx.Done():
			return sent, ctx.Err()
		default:
		}
		err := h.sendOneBatch(ctx, batch, i, len(batches))
		if err != nil {
			switch h.onError {
			case errhandling.OnErrorSkip, errhandling.OnErrorLog:
				logger.Error("httpRequest batch failed",
					slog.String("module_type", "httpRequest"),
					slog.Int("batch_index", i),
					slog.Int("batch_count", len(batches)),
					slog.Int("batch_records", len(batch)),
					slog.String("error", err.Error()),
					slog.String("on_error", string(h.onError)),
				)
				continue
			// OnErrorFail, and any strategy added later: stop on the failing
			// batch rather than counting its records as sent.
			default:
				return sent, fmt.Errorf("httpRequest batch %d: %w", i, err)
			}
		}
		sent += len(batch)
	}
	return sent, nil
}

// sendOneBatch sends a single HTTP request carrying every record of the batch.
// batchIndex and batchCount are logged so a chunked send is observable.
func (h *HTTPRequestModule) sendOneBatch(ctx context.Context, records []map[string]any, batchIndex, batchCount int) error {
	requestStart := time.Now()

	logger.Debug("sending records in batch mode",
		slog.String("module_type", "httpRequest"),
		slog.String("endpoint", httpclient.SanitizeURL(h.endpoint)),
		slog.String("method", h.method),
		slog.Int("record_count", len(records)),
		slog.Int("batch_index", batchIndex),
		slog.Int("batch_count", batchCount),
	)

	var body []byte
	var err error

	if h.request.bodyTemplateRaw != "" {
		// Use template file: evaluate template with first record (batch context)
		// For batch mode with template, we can only use first record's data
		// The template should be designed accordingly.
		// Known limitation: unlike soapRequest, no batchRecord is built here, so
		// a body template cannot loop over the batch. With batchSize set, this
		// resolves against the first record of each batch.
		if len(records) > 0 {
			target := h.bodyTargetFor(records[0])
			bodyStr, renderErr := h.renderField(h.request.bodyTemplateRaw, target, records[0])
			if renderErr != nil {
				return fmt.Errorf("evaluating body template: %w", renderErr)
			}
			body = []byte(bodyStr)

			// Validate resulting JSON is well-formed (AC #4)
			// Only validate if Content-Type indicates JSON
			if target == template.TargetJSON {
				if validationErr := validateJSON(body); validationErr != nil {
					logger.Warn("body template produced invalid JSON, continuing anyway",
						slog.String("error", validationErr.Error()),
						slog.String("body_preview", truncateString(string(body), 100)),
					)
					// Continue execution - let HTTP client handle the error if needed
				}
			}
		} else {
			body = []byte(h.request.bodyTemplateRaw)
		}
		logger.Debug("body generated from template file",
			slog.Int("body_size", len(body)),
		)
	} else if h.defaultBodyDisabled {
		// AC5: GET/DELETE (and any method outside POST/PUT/PATCH) with no
		// configured body sends no payload by default.
		body = nil
	} else {
		// Default: marshal records to JSON array (with metadata stripped)
		recordsForBody := metadata.StripFromRecords(records, metadata.DefaultFieldName)
		body, err = json.Marshal(recordsForBody)
		if err != nil {
			logger.Error("failed to marshal records to JSON",
				slog.String("module_type", "httpRequest"),
				slog.String("endpoint", httpclient.SanitizeURL(h.endpoint)),
				slog.Int("record_count", len(records)),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("%w: %w", ErrJSONMarshal, err)
		}
	}

	logger.Debug("request body prepared",
		slog.String("module_type", "httpRequest"),
		slog.Int("body_size", len(body)),
	)

	endpoint, err := h.resolveEndpointForBatch(h.endpoint, records)
	if err != nil {
		return err
	}

	// Evaluate templated headers using first record in batch mode
	var batchHeaders map[string]string
	if len(records) > 0 {
		batchHeaders, err = h.extractHeadersFromRecord(records[0])
		if err != nil {
			return err
		}
	}

	// Execute request with batch-resolved headers
	err = h.doRequestWithHeaders(ctx, endpoint, body, batchHeaders)
	requestDuration := time.Since(requestStart)

	if err != nil {
		logger.Debug("batch request failed",
			slog.String("module_type", "httpRequest"),
			slog.String("endpoint", httpclient.SanitizeURL(endpoint)),
			slog.Duration("duration", requestDuration),
			slog.String("error", err.Error()),
		)
		return err
	}

	logger.Debug("batch request completed",
		slog.String("module_type", "httpRequest"),
		slog.String("endpoint", httpclient.SanitizeURL(endpoint)),
		slog.Int("records_sent", len(records)),
		slog.Int("batch_index", batchIndex),
		slog.Int("batch_count", batchCount),
		slog.Duration("duration", requestDuration),
	)

	return nil
}

// sendSingleRecordMode sends one HTTP request per record
func (h *HTTPRequestModule) sendSingleRecordMode(ctx context.Context, records []map[string]any) (int, error) {
	logger.Debug("sending records in single record mode",
		slog.String("module_type", "httpRequest"),
		slog.String("endpoint", httpclient.SanitizeURL(h.endpoint)),
		slog.String("method", h.method),
		slog.Int("record_count", len(records)),
		slog.String("on_error", string(h.onError)),
	)

	sent := 0
	failed := 0
	for i, record := range records {
		requestStart := time.Now()

		body, err := h.buildBodyForRecord(record, i)
		if err != nil {
			failed++
			if h.onError == errhandling.OnErrorFail {
				return sent, fmt.Errorf("%w at record %d: %w", ErrJSONMarshal, i, err)
			}
			continue
		}

		endpoint, epErr := h.resolveEndpointForRecord(record)
		if epErr != nil {
			failed++
			if h.onError == errhandling.OnErrorFail {
				return sent, fmt.Errorf("at record %d: %w", i, epErr)
			}
			continue
		}
		recordHeaders, hdrErr := h.extractHeadersFromRecord(record)
		if hdrErr != nil {
			failed++
			if h.onError == errhandling.OnErrorFail {
				return sent, fmt.Errorf("at record %d: %w", i, hdrErr)
			}
			continue
		}

		ok, err := h.executeRequestAndLog(ctx, endpoint, body, recordHeaders, i, requestStart)
		if !ok {
			failed++
			if h.onError == errhandling.OnErrorFail {
				return sent, err
			}
			continue
		}
		sent++
	}

	logger.Debug("single record mode completed",
		slog.String("module_type", "httpRequest"),
		slog.Int("total_records", len(records)),
		slog.Int("sent", sent),
		slog.Int("failed", failed),
	)

	return sent, nil
}

// GetRetryInfo returns retry information from the last Send request (RetryInfoProvider).
func (h *HTTPRequestModule) GetRetryInfo() *connector.RetryInfo {
	return h.lastRetryInfo
}

// Close releases any resources held by the HTTP request module.
func (h *HTTPRequestModule) Close() error {
	h.client.CloseIdleConnections()
	logger.Debug("http request output module closed",
		slog.String("endpoint", httpclient.SanitizeURL(h.endpoint)),
	)
	return nil
}
