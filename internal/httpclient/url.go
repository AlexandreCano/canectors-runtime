package httpclient

import (
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"
)

// NormalizeURL percent-encodes the raw spaces a pipeline author leaves in an
// endpoint's query string.
//
// A space is never valid in a request URI, and Go does not fix it: url.Parse
// accepts it, keeps it verbatim in RawQuery, and RequestURI hands it to the
// wire. The server then splits the request line on that space and answers 400 —
// with nothing in the pipeline having objected, so the failure reads as a
// problem with the filter's syntax rather than with its encoding.
//
// The case that makes this worth fixing is the one incremental polling is full
// of: `?$filter=LastModifiedDate gt {{ state.lastTimestamp }}`. The spaces
// belong to the OData grammar and an author writing them is not wrong —
// %20-ing them by hand would be.
//
// Only spaces are touched, and only in the query: the path is re-encoded by
// EscapedPath on its own, and rewriting more would undo the deliberate shape of
// the URL (Story 25.1 AC7). An already-encoded %20 is left alone, since it holds
// no raw space to replace.
func NormalizeURL(raw string) (string, error) {
	if !strings.Contains(raw, " ") {
		return raw, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parsing endpoint URL for normalization: %w", err)
	}
	parsed.RawQuery = strings.ReplaceAll(parsed.RawQuery, " ", "%20")
	return parsed.String(), nil
}

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
// The parameters the endpoint already carries are kept **verbatim** and the new
// ones appended after them. Decoding the query into url.Values and re-encoding
// it would round-trip the parts the author wrote deliberately: `$filter` came
// back as `%24filter`, spaces as `+`, and the parameters in alphabetical order.
// A conformant server still reads that correctly, but it is not the request the
// pipeline describes, and `+` only means a space under form-encoding rules the
// source is not obliged to follow (Story 25.1 AC7, Story 25.5).
//
// An endpoint with nothing to add is returned verbatim, without even a
// url.Parse/String round trip: no pipeline should have its endpoint reshaped to
// add nothing.
func MergeQueryParams(endpoint string, params map[string]string) (string, error) {
	if len(params) == 0 {
		return endpoint, nil
	}

	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parsing endpoint URL for query parameters: %w", err)
	}

	pairs := make([]string, 0, len(params))
	for _, pair := range strings.Split(parsed.RawQuery, "&") {
		if pair == "" {
			continue
		}
		if _, overridden := params[queryPairName(pair)]; overridden {
			continue
		}
		pairs = append(pairs, pair)
	}

	// Sorted so the same configuration always produces the same URL — a map
	// iterates in a different order on every run, and a cache key or a recorded
	// request should not move with it.
	for _, name := range slices.Sorted(maps.Keys(params)) {
		pairs = append(pairs, EscapeQueryComponent(name)+"="+EscapeQueryComponent(params[name]))
	}

	// The kept pairs are verbatim, so they may still carry the raw spaces an
	// author wrote in an OData filter. Encoding them here means every caller
	// gets a sendable URL out of this function, whether or not it also calls
	// NormalizeURL.
	parsed.RawQuery = strings.ReplaceAll(strings.Join(pairs, "&"), " ", "%20")

	return parsed.String(), nil
}

// queryPairName returns the decoded parameter name of a raw `name=value` pair.
// A name that does not decode is compared as written — it cannot match a
// configured parameter anyway, and dropping it would lose a parameter the
// author put in the URL.
func queryPairName(pair string) string {
	name, _, _ := strings.Cut(pair, "=")
	decoded, err := url.QueryUnescape(name)
	if err != nil {
		return name
	}
	return decoded
}

// queryComponentEscaper rewrites the two escapes url.QueryEscape produces that
// this package does not want.
//
// `+` means a space only under form-encoding rules the source is not obliged to
// follow, so spaces go out as %20 — the same escaping the template engine
// applies to substituted values (see template.urlEscape).
//
// `$` is a sub-delimiter RFC 3986 allows verbatim in a query, and the query
// names pipelines actually write start with it: `$filter`, `$top`, `$orderby`.
// Escaping it split a single OData URL into two spellings — `?$filter=…` kept
// from the endpoint next to `&%24top=…` added from queryParams — for no gain.
var queryComponentEscaper = strings.NewReplacer("+", "%20", "%24", "$")

// EscapeQueryComponent percent-encodes a query parameter name or value.
func EscapeQueryComponent(s string) string {
	return queryComponentEscaper.Replace(url.QueryEscape(s))
}
