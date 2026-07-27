// Package urlsan strips the parts of a URL that carry secrets before it is
// logged or embedded in an error message.
//
// It is a leaf package on purpose. Both httpclient and errhandling need this,
// and httpclient imports errhandling, so neither could own the helper without
// creating an import cycle — which is how the two packages ended up with the
// same function twice, free to drift apart.
package urlsan

import "net/url"

// Sanitize returns the URL without the parts that carry secrets: the query
// string (api keys, access tokens), the fragment, and any user:password
// credentials embedded in the authority.
//
// A URL that cannot be parsed returns a placeholder rather than the raw value:
// failing to parse is no reason to log something potentially sensitive.
func Sanitize(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "[invalid URL]"
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.User = nil
	return parsed.String()
}
