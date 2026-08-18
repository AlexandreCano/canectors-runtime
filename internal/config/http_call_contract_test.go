package config_test

import (
	"testing"
)

// TestSchemaHTTPCall_AcceptsAnyValidMethod locks Story 24.11 AC1.
func TestSchemaHTTPCall_AcceptsAnyValidMethod(t *testing.T) {
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		t.Run(method, func(t *testing.T) {
			f := `{"type":"http_call","endpoint":"https://e/x","method":"` + method + `","keys":[{"field":"id","paramType":"query","paramName":"id"}]}`
			if !validateFilter(t, f) {
				t.Fatalf("method %s should be accepted", method)
			}
		})
	}
}

// TestSchemaHTTPCall_AcceptsBodyForAnyMethod locks Story 24.11 AC2.
func TestSchemaHTTPCall_AcceptsBodyForAnyMethod(t *testing.T) {
	for _, method := range []string{"GET", "POST", "DELETE"} {
		t.Run(method, func(t *testing.T) {
			f := `{"type":"http_call","endpoint":"https://e/x","method":"` + method + `","body":"{\"a\":1}","keys":[{"field":"id","paramType":"query","paramName":"id"}]}`
			if !validateFilter(t, f) {
				t.Fatalf("body with method %s should be accepted", method)
			}
		})
	}
}

// TestSchemaHTTPCall_AcceptsRetry locks Story 24.11 AC3.
func TestSchemaHTTPCall_AcceptsRetry(t *testing.T) {
	f := `{"type":"http_call","endpoint":"https://e/x","keys":[{"field":"id","paramType":"query","paramName":"id"}],"retry":{"maxAttempts":3,"delayMs":100,"maxDelayMs":1000}}`
	if !validateFilter(t, f) {
		t.Fatal("retry config should be accepted")
	}
}

// TestSchemaHTTPCall_RejectsLegacyDefaultTTL locks Story 24.11 AC6.
func TestSchemaHTTPCall_RejectsLegacyDefaultTTL(t *testing.T) {
	cases := []struct {
		name      string
		filter    string
		wantValid bool
	}{
		{"ttlSeconds ok", `{"type":"http_call","endpoint":"https://e/x","keys":[{"field":"id","paramType":"query","paramName":"id"}],"cache":{"enabled":true,"ttlSeconds":60}}`, true},
		{"defaultTTL rejected", `{"type":"http_call","endpoint":"https://e/x","keys":[{"field":"id","paramType":"query","paramName":"id"}],"cache":{"enabled":true,"defaultTTL":60}}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateFilter(t, tc.filter); got != tc.wantValid {
				t.Fatalf("validateFilter(%s) = %v, want %v", tc.filter, got, tc.wantValid)
			}
		})
	}
}

// TestSchemaHTTPCall_RejectsCacheZeroes locks Story 24.11 AC13.
func TestSchemaHTTPCall_RejectsCacheZeroes(t *testing.T) {
	cases := []struct {
		name      string
		filter    string
		wantValid bool
	}{
		{"maxSize: 0 rejected", `{"type":"http_call","endpoint":"https://e/x","keys":[{"field":"id","paramType":"query","paramName":"id"}],"cache":{"enabled":true,"maxSize":0}}`, false},
		{"ttlSeconds: 0 rejected", `{"type":"http_call","endpoint":"https://e/x","keys":[{"field":"id","paramType":"query","paramName":"id"}],"cache":{"enabled":true,"ttlSeconds":0}}`, false},
		{"maxSize: 1 ok", `{"type":"http_call","endpoint":"https://e/x","keys":[{"field":"id","paramType":"query","paramName":"id"}],"cache":{"enabled":true,"maxSize":1}}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateFilter(t, tc.filter); got != tc.wantValid {
				t.Fatalf("validateFilter(%s) = %v, want %v", tc.filter, got, tc.wantValid)
			}
		})
	}
}

// TestSchemaHTTPCall_KeyParamTypeValidated locks Story 24.11 AC11.
func TestSchemaHTTPCall_KeyParamTypeValidated(t *testing.T) {
	cases := []struct {
		name      string
		filter    string
		wantValid bool
	}{
		{"valid query", `{"type":"http_call","endpoint":"https://e/x","keys":[{"field":"id","paramType":"query","paramName":"id"}]}`, true},
		{"valid path", `{"type":"http_call","endpoint":"https://e/{id}","keys":[{"field":"id","paramType":"path","paramName":"id"}]}`, true},
		{"valid header", `{"type":"http_call","endpoint":"https://e/x","keys":[{"field":"id","paramType":"header","paramName":"X-Id"}]}`, true},
		{"invalid paramType", `{"type":"http_call","endpoint":"https://e/x","keys":[{"field":"id","paramType":"body","paramName":"id"}]}`, false},
		{"empty field", `{"type":"http_call","endpoint":"https://e/x","keys":[{"field":"","paramType":"query","paramName":"id"}]}`, false},
		{"empty paramName", `{"type":"http_call","endpoint":"https://e/x","keys":[{"field":"id","paramType":"query","paramName":""}]}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateFilter(t, tc.filter); got != tc.wantValid {
				t.Fatalf("validateFilter(%s) = %v, want %v", tc.filter, got, tc.wantValid)
			}
		})
	}
}

// TestSchemaHTTPCall_KeysRequiredWithoutBody locks the schema to the runtime rule
// in NewHTTPCallFromConfig: without a body or bodyTemplateFile the keys are the
// only way to parameterize the call per record, so at least one key is required.
// Before this, such a config validated fine and then failed on every scheduler tick.
func TestSchemaHTTPCall_KeysRequiredWithoutBody(t *testing.T) {
	const key = `"keys":[{"field":"id","paramType":"query","paramName":"id"}]`
	cases := []struct {
		name      string
		filter    string
		wantValid bool
	}{
		{
			"no keys and no body is rejected",
			`{"type":"http_call","endpoint":"https://e/x"}`,
			false,
		},
		{
			"empty keys and no body is rejected",
			`{"type":"http_call","endpoint":"https://e/x","keys":[]}`,
			false,
		},
		{
			"keys without body is accepted",
			`{"type":"http_call","endpoint":"https://e/x",` + key + `}`,
			true,
		},
		{
			"body without keys is accepted",
			`{"type":"http_call","endpoint":"https://e/x","body":"{\"a\":1}"}`,
			true,
		},
		{
			"bodyTemplateFile without keys is accepted",
			`{"type":"http_call","endpoint":"https://e/x","bodyTemplateFile":"tpl.json"}`,
			true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateFilter(t, tc.filter); got != tc.wantValid {
				t.Fatalf("validateFilter(%s) = %v, want %v", tc.filter, got, tc.wantValid)
			}
		})
	}
}

// TestSchemaHTTPCall_AppendRequiresResultKey locks Story 25.2 AC1/AC3: http_call
// accepts resultKey like soap_call and sql_call, and append without a
// destination is a validation error rather than a silent fallback.
func TestSchemaHTTPCall_AppendRequiresResultKey(t *testing.T) {
	const keys = `"keys":[{"field":"id","paramType":"query","paramName":"id"}]`
	cases := []struct {
		name      string
		filter    string
		wantValid bool
	}{
		{"append without resultKey", `{"type":"http_call","endpoint":"https://e/x",` + keys + `,"mergeStrategy":"append"}`, false},
		{"append with resultKey", `{"type":"http_call","endpoint":"https://e/x",` + keys + `,"mergeStrategy":"append","resultKey":"enrichment"}`, true},
		{"merge with resultKey", `{"type":"http_call","endpoint":"https://e/x",` + keys + `,"mergeStrategy":"merge","resultKey":"enrichment"}`, true},
		{"merge without resultKey", `{"type":"http_call","endpoint":"https://e/x",` + keys + `,"mergeStrategy":"merge"}`, true},
		{"empty resultKey rejected", `{"type":"http_call","endpoint":"https://e/x",` + keys + `,"mergeStrategy":"append","resultKey":""}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateFilter(t, tc.filter); got != tc.wantValid {
				t.Fatalf("validateFilter(%s) = %v, want %v", tc.filter, got, tc.wantValid)
			}
		})
	}
}
