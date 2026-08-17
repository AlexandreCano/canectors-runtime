package soapclient

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"

	recordtemplate "github.com/cannectors/runtime/internal/template"
)

// SOAPHeaderTemplate is a raw XML SOAP header fragment with optional record templating.
type SOAPHeaderTemplate struct {
	XML string
}

// EnvelopeOptions configures SOAP envelope construction.
type EnvelopeOptions struct {
	Version    SOAPVersion
	Body       string
	Record     map[string]any
	Headers    []SOAPHeaderTemplate
	WSSecurity *WSSecurityConfig
}

// BuildEnvelope wraps raw body XML in a SOAP envelope and escapes record substitutions.
// Client.Call delegates envelope emission to hooklift; this helper remains for
// low-level tests and callers that need to inspect an envelope without sending it.
func BuildEnvelope(opts EnvelopeOptions) ([]byte, error) {
	namespace, err := EnvelopeNamespace(opts.Version)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(opts.Body) == "" {
		return nil, fmt.Errorf("SOAP body is required")
	}

	body, err := EvaluateXMLTemplate(opts.Body, opts.Record)
	if err != nil {
		return nil, err
	}

	var headers []string
	for i, h := range opts.Headers {
		if strings.TrimSpace(h.XML) == "" {
			continue
		}
		evaluated, evalErr := EvaluateXMLTemplate(h.XML, opts.Record)
		if evalErr != nil {
			return nil, fmt.Errorf("evaluating SOAP header %d: %w", i, evalErr)
		}
		headers = append(headers, evaluated)
	}
	if opts.WSSecurity != nil {
		header, secErr := BuildWSSecurityHeader(*opts.WSSecurity)
		if secErr != nil {
			return nil, secErr
		}
		headers = append(headers, string(header))
	}

	var b strings.Builder
	b.WriteString(`<soap:Envelope xmlns:soap="`)
	b.WriteString(namespace)
	b.WriteString(`">`)
	if len(headers) > 0 {
		b.WriteString(`<soap:Header>`)
		for _, header := range headers {
			b.WriteString(header)
		}
		b.WriteString(`</soap:Header>`)
	}
	b.WriteString(`<soap:Body>`)
	b.WriteString(body)
	b.WriteString(`</soap:Body></soap:Envelope>`)
	return []byte(b.String()), nil
}

// xmlTemplateEngine renders SOAP XML fragments. Substituted values are
// XML-escaped (TargetXML) and missing variables without a default are an error
// (strict): SOAP bodies must not silently drop required fields.
var xmlTemplateEngine = recordtemplate.NewEngine()

// EvaluateXMLTemplate evaluates record templates in an XML fragment, XML-escaping
// substituted values. A missing variable without a default is an error.
func EvaluateXMLTemplate(raw string, record map[string]any) (string, error) {
	if !recordtemplate.HasVariables(raw) {
		return raw, nil
	}
	compiled, err := xmlTemplateEngine.Compile(raw, recordtemplate.TargetXML, true)
	if err != nil {
		return "", err
	}
	// ContextForRecord, not a bare RenderContext: inside a loop the record is a
	// scope, and the body must address the item and the root record by the same
	// names the endpoint, the headers and keys[].field use.
	return compiled.Render(recordtemplate.ContextForRecord(record))
}

func escapeXMLText(value string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(value))
	return b.String()
}
