package urlsan

import (
	"strings"
	"testing"
)

func TestRedactText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty",
			in:   "",
			want: "",
		},
		{
			name: "no URL is left untouched",
			in:   "http_call failed to extract key from record 3: field 'customer.id' not found",
			want: "http_call failed to extract key from record 3: field 'customer.id' not found",
		},
		{
			name: "query string carrying a token is stripped",
			in:   `http_call request failed: Get "https://api.example.com/v1/orders?token=s3cr3t"`,
			want: `http_call request failed: Get "https://api.example.com/v1/orders"`,
		},
		{
			name: "embedded credentials are stripped",
			in:   "dial failed for https://alice:hunter2@api.example.com/v1/orders",
			want: "dial failed for https://api.example.com/v1/orders",
		},
		{
			name: "fragment is stripped",
			in:   "redirected to https://api.example.com/v1/orders#section",
			want: "redirected to https://api.example.com/v1/orders",
		},
		{
			name: "trailing sentence punctuation is not swallowed",
			in:   "called https://api.example.com/v1/orders?key=abc.",
			want: "called https://api.example.com/v1/orders.",
		},
		{
			name: "several URLs are each redacted",
			in:   "https://a.example.com/x?k=1 then https://b.example.com/y?k=2",
			want: "https://a.example.com/x then https://b.example.com/y",
		},
		{
			name: "dotted identifiers are not mistaken for hosts",
			in:   "condition on _errors[0].retryable failed for record.customer.id",
			want: "condition on _errors[0].retryable failed for record.customer.id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RedactText(tt.in); got != tt.want {
				t.Errorf("RedactText(%q)\n got: %q\nwant: %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestRedactTextRemovesSecretsFromAnyPosition is the property that matters for
// Story 26.10 AC6: whatever the surrounding prose, a secret placed in a query
// string or in the authority must not survive redaction.
func TestRedactTextRemovesSecretsFromAnyPosition(t *testing.T) {
	const secret = "s3cr3t-token-value"
	messages := []string{
		"https://api.example.com/v1?token=" + secret,
		"prefix https://api.example.com/v1?token=" + secret,
		"https://api.example.com/v1?token=" + secret + " suffix",
		"wrapped (https://api.example.com/v1?token=" + secret + ") here",
		`quoted "https://api.example.com/v1?a=1&token=` + secret + `" here`,
		"https://user:" + secret + "@api.example.com/v1",
		"https://api.example.com/v1#" + secret,
	}

	for _, msg := range messages {
		got := RedactText(msg)
		if strings.Contains(got, secret) {
			t.Errorf("secret survived redaction\n  in: %q\n out: %q", msg, got)
		}
	}
}
