package urlsan

import (
	// xurls owns the hard part of this file: finding where a URL starts and ends
	// inside free-form text (trailing punctuation, parentheses, IDN hosts). A
	// hand-rolled `scheme://\S+` regex swallows the sentence's final period and
	// stops at the first parenthesis, which is how a redactor starts leaking the
	// tail of a query string it meant to strip.
	"mvdan.cc/xurls/v2"
)

// urlInText matches URLs that carry an explicit scheme. Scheme-relative and
// bare-host matching (xurls.Relaxed) is deliberately not used: error messages
// are full of dotted identifiers like `record.customer.id` and
// `_errors[0].statusCode`, and Relaxed reads those as hosts.
var urlInText = xurls.Strict()

// RedactText returns text with every embedded URL replaced by its sanitized
// form, so a message can be attached to a record or logged without carrying the
// secrets a URL holds (query string, fragment, user:password).
//
// It exists because Sanitize takes a URL, not a sentence, while the values that
// actually need redacting are error messages that interpolate an endpoint:
// `http_call request failed: Get "https://api.example.com/v1/orders?token=s3cr3t"`.
// Passing that whole string to Sanitize would return "[invalid URL]" and throw
// the diagnostic away; passing it through unchanged would publish the token.
func RedactText(text string) string {
	if text == "" {
		return ""
	}
	return urlInText.ReplaceAllStringFunc(text, Sanitize)
}
