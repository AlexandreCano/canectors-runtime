package soapclient

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/cannectors/runtime/internal/errhandling"
)

// SOAPFaultError is returned when a SOAP response contains a Fault element.
type SOAPFaultError struct {
	Version    SOAPVersion
	Code       string
	Reason     string
	Actor      string
	Detail     string
	StatusCode int
}

func (e *SOAPFaultError) Error() string {
	if e.Code == "" {
		return "SOAP fault: " + e.Reason
	}
	return fmt.Sprintf("SOAP fault %s: %s", e.Code, e.Reason)
}

// Classify implements errhandling.Classifiable.
//
// A SOAP fault is an application-level answer, so its HTTP status says little:
// SOAP 1.1 mandates 500 for every fault, including one that means "your request
// was malformed". Left to the generic rules, a Client fault would be reported
// as a retryable server error and replayed forever.
//
// The fault code carries the real distinction, and it is the one place the
// SOAP specs agree on: Client (1.1) / Sender (1.2) blames the request,
// Server (1.1) / Receiver (1.2) blames the service. The code is namespace
// qualified ("soap:Client"), hence the match on the local part.
func (e *SOAPFaultError) Classify() *errhandling.ClassifiedError {
	classified := &errhandling.ClassifiedError{
		Category:   errhandling.CategoryUnknown,
		Retryable:  false,
		StatusCode: e.StatusCode,
		Message:    e.Reason,
	}

	switch strings.ToLower(faultCodeLocalPart(e.Code)) {
	case "client", "sender":
		classified.Category = errhandling.CategoryValidation
	case "server", "receiver":
		classified.Category = errhandling.CategoryServer
		classified.Retryable = true
	}

	// An unrecognized fault code stays non-retryable: the service answered, and
	// replaying a request it already rejected is the failure mode this whole
	// classification exists to prevent.
	return classified
}

// faultCodeLocalPart strips the namespace prefix from a fault code
// ("soap:Client" → "Client"), returning the code unchanged when it has none.
func faultCodeLocalPart(code string) string {
	if i := strings.LastIndex(code, ":"); i >= 0 {
		return code[i+1:]
	}
	return code
}

type faultEnvelope struct {
	Body struct {
		Fault *faultPayload `xml:"Fault"`
	} `xml:"Body"`
}

type faultPayload struct {
	XMLName xml.Name

	Code11   string `xml:"faultcode"`
	Reason11 string `xml:"faultstring"`
	Actor11  string `xml:"faultactor"`
	Detail11 rawXML `xml:"detail"`

	Code12 struct {
		Value string `xml:"Value"`
	} `xml:"Code"`
	Reason12 struct {
		Text string `xml:"Text"`
	} `xml:"Reason"`
	Detail12 rawXML `xml:"Detail"`
}

type rawXML struct {
	Inner string `xml:",innerxml"`
}

// ParseSOAPFault extracts SOAP 1.1 or SOAP 1.2 fault details from an envelope.
func ParseSOAPFault(body []byte) (*SOAPFaultError, error) {
	var envelope faultEnvelope
	if err := xml.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	if envelope.Body.Fault == nil {
		return nil, nil
	}
	fault := envelope.Body.Fault
	if fault.Code12.Value != "" || fault.Reason12.Text != "" || fault.XMLName.Space == NamespaceSOAP12 {
		return &SOAPFaultError{
			Version: SOAPVersion12,
			Code:    strings.TrimSpace(fault.Code12.Value),
			Reason:  strings.TrimSpace(fault.Reason12.Text),
			Detail:  strings.TrimSpace(fault.Detail12.Inner),
		}, nil
	}
	return &SOAPFaultError{
		Version: SOAPVersion11,
		Code:    strings.TrimSpace(fault.Code11),
		Reason:  strings.TrimSpace(fault.Reason11),
		Actor:   strings.TrimSpace(fault.Actor11),
		Detail:  strings.TrimSpace(fault.Detail11.Inner),
	}, nil
}
