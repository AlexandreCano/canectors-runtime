package soapclient

import (
	"bytes"
	"testing"
)

func TestBuildEnvelope_EscapesRecordVariables(t *testing.T) {
	envelope, err := BuildEnvelope(EnvelopeOptions{
		Version: SOAPVersion11,
		Body:    `<m:Get><m:Value>{{record.payload}}</m:Value></m:Get>`,
		Record: map[string]any{
			"payload": `</soap:Body><evil attr="x">&`,
		},
	})
	if err != nil {
		t.Fatalf("BuildEnvelope returned error: %v", err)
	}

	if bytes.Contains(envelope, []byte(`</soap:Body><evil`)) {
		t.Fatalf("template variable was not XML-escaped:\n%s", envelope)
	}
	if !bytes.Contains(envelope, []byte(`&lt;/soap:Body&gt;&lt;evil attr=&#34;x&#34;&gt;&amp;`)) {
		t.Fatalf("escaped value missing from envelope:\n%s", envelope)
	}
}

func TestBuildEnvelope_UsesSOAP12Namespace(t *testing.T) {
	envelope, err := BuildEnvelope(EnvelopeOptions{
		Version: SOAPVersion12,
		Body:    `<m:Ping/>`,
	})
	if err != nil {
		t.Fatalf("BuildEnvelope returned error: %v", err)
	}
	if !bytes.Contains(envelope, []byte(NamespaceSOAP12)) {
		t.Fatalf("SOAP 1.2 namespace missing:\n%s", envelope)
	}
}

func TestEvaluateXMLTemplate_ReplacesRepeatedVariables(t *testing.T) {
	got, err := EvaluateXMLTemplate(`<A>{{record.id}}</A><B>{{record.id}}</B>`, map[string]any{"id": "x&y"})
	if err != nil {
		t.Fatalf("EvaluateXMLTemplate returned error: %v", err)
	}
	want := `<A>x&amp;y</A><B>x&amp;y</B>`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// Inside a loop the record handed to the SOAP body is a scope, and the body
// must use the same addressing root as the endpoint, the headers and
// keys[].field: the alias for the item, `record` for the root record.
func TestEvaluateXMLTemplate_LoopScopeAddressing(t *testing.T) {
	scope := map[string]any{
		"record": map[string]any{"orderId": "ORD-1"},
		"line":   map[string]any{"number": "L1"},
		"_metadata": map[string]any{
			"loop": map[string]any{"line": map[string]any{"index": 0}},
		},
	}

	got, err := EvaluateXMLTemplate(
		`<Order>{{ record.orderId }}</Order><Line>{{ line.number }}</Line><Idx>{{ _metadata.loop.line.index }}</Idx>`,
		scope,
	)
	if err != nil {
		t.Fatalf("EvaluateXMLTemplate returned error: %v", err)
	}
	want := `<Order>ORD-1</Order><Line>L1</Line><Idx>0</Idx>`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	// The old doubly-prefixed form is gone, here as everywhere else.
	if _, err := EvaluateXMLTemplate(`<Line>{{ record.line.number }}</Line>`, scope); err == nil {
		t.Fatal("record.line.number still resolved in a SOAP body")
	}
}

func TestEvaluateXMLTemplate_MissingVariableReturnsError(t *testing.T) {
	_, err := EvaluateXMLTemplate(`<A>{{record.missing}}</A>`, map[string]any{"id": "x"})
	if err == nil {
		t.Fatal("expected missing variable error")
	}
}
