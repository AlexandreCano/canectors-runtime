package output

import (
	"strings"
	"testing"
)

func TestNormalizeRequestMode(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		moduleType string
		want       string
		wantErr    bool
	}{
		{name: "empty defaults to batch", mode: "", moduleType: "httpRequest", want: defaultRequestMode},
		{name: "batch", mode: "batch", moduleType: "httpRequest", want: "batch"},
		{name: "single", mode: "single", moduleType: "httpRequest", want: "single"},
		{name: "unknown rejected", mode: "bulk", moduleType: "httpRequest", wantErr: true},
		// The helper is shared, so the error must name the caller rather than
		// always blaming httpRequest.
		{name: "unknown rejected names soapRequest", mode: "bulk", moduleType: "soapRequest", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeRequestMode(tc.mode, tc.moduleType)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error for mode %q", tc.mode)
				}
				if !strings.HasPrefix(err.Error(), tc.moduleType+" ") {
					t.Errorf("error %q should be prefixed with %q", err, tc.moduleType)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("mode = %q, want %q", got, tc.want)
			}
		})
	}
}
