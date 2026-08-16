package soapclient

import (
	"testing"

	"github.com/cannectors/runtime/internal/errhandling"
)

// SOAP 1.1 mandates HTTP 500 for every fault, so the status code says nothing
// about who is at fault. The fault code does, and it is what the classification
// must read — otherwise a malformed request is replayed forever as a "server
// error".
func TestSOAPFaultErrorClassify(t *testing.T) {
	tests := []struct {
		name          string
		fault         *SOAPFaultError
		wantCategory  errhandling.ErrorCategory
		wantRetryable bool
	}{
		{
			name:          "SOAP 1.1 Client fault blames the request",
			fault:         &SOAPFaultError{Version: SOAPVersion11, Code: "soap:Client", Reason: "invalid order id", StatusCode: 500},
			wantCategory:  errhandling.CategoryValidation,
			wantRetryable: false,
		},
		{
			name:          "SOAP 1.1 Server fault is transient",
			fault:         &SOAPFaultError{Version: SOAPVersion11, Code: "soap:Server", Reason: "backend unavailable", StatusCode: 500},
			wantCategory:  errhandling.CategoryServer,
			wantRetryable: true,
		},
		{
			name:          "SOAP 1.2 Sender fault blames the request",
			fault:         &SOAPFaultError{Version: SOAPVersion12, Code: "env:Sender", Reason: "bad argument", StatusCode: 400},
			wantCategory:  errhandling.CategoryValidation,
			wantRetryable: false,
		},
		{
			name:          "SOAP 1.2 Receiver fault is transient",
			fault:         &SOAPFaultError{Version: SOAPVersion12, Code: "env:Receiver", Reason: "temporary failure", StatusCode: 500},
			wantCategory:  errhandling.CategoryServer,
			wantRetryable: true,
		},
		{
			name:          "an unqualified code is still read",
			fault:         &SOAPFaultError{Version: SOAPVersion11, Code: "Client", Reason: "invalid", StatusCode: 500},
			wantCategory:  errhandling.CategoryValidation,
			wantRetryable: false,
		},
		{
			name:          "an unrecognized code stays non-retryable",
			fault:         &SOAPFaultError{Version: SOAPVersion11, Code: "soap:VersionMismatch", Reason: "unsupported envelope", StatusCode: 500},
			wantCategory:  errhandling.CategoryUnknown,
			wantRetryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := errhandling.ClassifyError(tt.fault)
			if cl.Category != tt.wantCategory {
				t.Errorf("category = %q, want %q", cl.Category, tt.wantCategory)
			}
			if cl.Retryable != tt.wantRetryable {
				t.Errorf("retryable = %v, want %v", cl.Retryable, tt.wantRetryable)
			}
			if cl.StatusCode != tt.fault.StatusCode {
				t.Errorf("statusCode = %d, want %d", cl.StatusCode, tt.fault.StatusCode)
			}
		})
	}
}
