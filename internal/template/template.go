// Package template renders Jinja templates with contextual escaping for
// pipeline configuration fields (see engine.go). This file keeps the small
// shared helpers used across modules.
package template

import (
	"fmt"
	"strings"
)

// Template syntax constants
const (
	// TemplatePrefix is the opening delimiter for template variables
	TemplatePrefix = "{{"
	// TemplateSuffix is the closing delimiter for template variables
	TemplateSuffix = "}}"
	// BlockPrefix is the opening delimiter for control structures
	BlockPrefix = "{%"
)

// HasVariables reports whether a string contains template markers (an output
// tag or a control block). Modules use it as a fast path to skip rendering
// static values.
func HasVariables(s string) bool {
	return (strings.Contains(s, TemplatePrefix) && strings.Contains(s, TemplateSuffix)) ||
		strings.Contains(s, BlockPrefix)
}

// ValueToString converts any value to its string representation.
// Floats holding integral values render without a decimal point, matching how
// JSON numbers round-trip through map[string]any records.
func ValueToString(value any) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case float64:
		// Format integers without decimal point
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%v", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case bool:
		return fmt.Sprintf("%t", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
