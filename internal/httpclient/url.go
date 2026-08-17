package httpclient

import (
	"fmt"
	"net/url"
)

// MergeQueryParams appends static query parameters to an endpoint URL.
//
// It is shared by httpPolling, http_call and httpRequest so that `queryParams`
// means the same thing in all three. It used to be implemented in the output
// only, which left the field accepted by the schema — it lives in the shared
// httpRequestBase definition — and silently ignored everywhere else: a pipeline
// declaring it on httpPolling validated, ran, and dropped the parameters
// without a word (Story 25.1).
//
// Precedence: a parameter given here replaces one already spelled out in the
// endpoint's query string. The explicit field wins over the URL literal, which
// is what lets a default be written into the endpoint and overridden per module.
//
// An endpoint with nothing to add is returned verbatim rather than parsed and
// re-serialized: a url.Parse/String round trip rewrites parts of the URL the
// author wrote deliberately, and no pipeline should have its endpoint reshaped
// to add nothing.
func MergeQueryParams(endpoint string, params map[string]string) (string, error) {
	if len(params) == 0 {
		return endpoint, nil
	}

	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parsing endpoint URL for query parameters: %w", err)
	}

	q := parsed.Query()
	for name, value := range params {
		q.Set(name, value)
	}
	parsed.RawQuery = q.Encode()

	return parsed.String(), nil
}
