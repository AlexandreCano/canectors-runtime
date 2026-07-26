package template

// RenderContext holds the data exposed to a template at render time.
// Each field becomes a top-level variable inside the template:
//
//	record      -> the current record
//	meta        -> record metadata (the _metadata field)
//	state       -> persistence state (last run timestamp, id, offset)
//	pagination  -> current pagination cursor (page/offset/cursor)
type RenderContext struct {
	Record     map[string]any
	Meta       map[string]any
	State      map[string]any
	Pagination map[string]any
}

// Vars builds the variable map handed to gonja — and, for SQL, to the expr
// programs of the parameters list, so both languages see the same names. All
// four top-level variables are always present (empty when unset) so a reference
// to a top-level name is never itself "undefined" — only a missing field below
// it can be, which is what the strict/lenient setting governs.
func (rc RenderContext) Vars() map[string]any {
	return map[string]any{
		"record":     orEmpty(rc.Record),
		"meta":       orEmpty(rc.Meta),
		"state":      orEmpty(rc.State),
		"pagination": orEmpty(rc.Pagination),
	}
}

func orEmpty(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}
