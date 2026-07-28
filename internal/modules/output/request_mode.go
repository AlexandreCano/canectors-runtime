package output

import "fmt"

// requestMode values and their default, shared by httpRequest and soapRequest.
const (
	requestModeBatch   = "batch"
	requestModeSingle  = "single"
	defaultRequestMode = requestModeBatch
)

// normalizeRequestMode validates and normalizes the requestMode field
// (Story 24.12 AC15). Empty defaults to defaultRequestMode. moduleType names
// the calling module in the error, since both outputs share this helper.
func normalizeRequestMode(mode, moduleType string) (string, error) {
	switch mode {
	case "":
		return defaultRequestMode, nil
	case requestModeBatch, requestModeSingle:
		return mode, nil
	default:
		return "", fmt.Errorf("%s invalid requestMode %q: must be 'batch' or 'single'", moduleType, mode)
	}
}
