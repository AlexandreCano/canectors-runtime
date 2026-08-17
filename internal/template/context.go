package template

import (
	"slices"

	"github.com/cannectors/runtime/internal/metadata"
)

// Reserved top-level variable names. A loop alias may not take one of these:
// Extra loses on collision, and the loop filter rejects such an itemName when
// the pipeline is built, so the clash is reported instead of silently applied.
const (
	VarRecord     = "record"
	VarMeta       = "meta"
	VarState      = "state"
	VarPagination = "pagination"
)

// reservedVarNames lists the top-level variable names a template always sees.
// Unexported so no caller can grow or reorder the set behind Vars' back; ask
// through IsReservedVarName or ReservedVarNames instead.
var reservedVarNames = []string{VarRecord, VarMeta, VarState, VarPagination}

// IsReservedVarName reports whether name is a top-level variable a template
// always sees. A loop alias may not take one: Extra loses on collision, so the
// loop filter rejects such an itemName when the pipeline is built.
func IsReservedVarName(name string) bool {
	return slices.Contains(reservedVarNames, name)
}

// ReservedVarNames returns the reserved top-level variable names, as a copy so
// the set cannot be grown or reordered behind Vars' back.
//
// The loop filter mirrors this set into the `itemName` rejection list, and the
// JSON schema restates it in an enum so a bad alias is caught at validation
// time too. A test ties the three together: growing this list without touching
// the schema fails it, rather than shipping a schema that accepts an alias the
// runtime refuses.
func ReservedVarNames() []string {
	return slices.Clone(reservedVarNames)
}

// RenderContext holds the data exposed to a template at render time.
// Each field becomes a top-level variable inside the template:
//
//	record      -> the current record
//	meta        -> record metadata (the _metadata field)
//	state       -> persistence state (last run timestamp, id, offset)
//	pagination  -> current pagination cursor (page/offset/cursor)
//	<extra>     -> inside a loop, the scope: one variable per active loop alias,
//	               plus any field a nested filter wrote at scope level
type RenderContext struct {
	Record     map[string]any
	Meta       map[string]any
	State      map[string]any
	Pagination map[string]any

	// Extra holds additional top-level variables. Inside a loop it carries the
	// loop scope itself, so `{{ line.number }}` reads the current item — the
	// same name a field path uses — and so does any field a nested filter wrote
	// at scope level. A `record` key in here is ignored: Vars assigns the
	// reserved names last, so Record always wins.
	Extra map[string]any
}

// Vars builds the variable map handed to gonja — and, for SQL, to the expr
// programs of the parameters list, so both languages see the same names. All
// four top-level variables are always present (empty when unset) so a reference
// to a top-level name is never itself "undefined" — only a missing field below
// it can be, which is what the strict/lenient setting governs.
func (rc RenderContext) Vars() map[string]any {
	vars := make(map[string]any, len(rc.Extra)+len(reservedVarNames))
	for name, value := range rc.Extra {
		vars[name] = value
	}
	// The reserved names are assigned last so Extra can never shadow them.
	vars[VarRecord] = orEmpty(rc.Record)
	vars[VarMeta] = orEmpty(rc.Meta)
	vars[VarState] = orEmpty(rc.State)
	vars[VarPagination] = orEmpty(rc.Pagination)
	return vars
}

// ContextForRecord builds the render context for a record, unwrapping a loop
// scope when it finds one.
//
// It exists so templates address a value by the same name a field path does.
// Inside a loop, nested modules receive a *scope* — a map holding the root
// record under `record`, the current item under its alias, and the loop
// metadata — and that scope was handed to templates as if it were the record
// itself. Authors therefore had to write `{{ record.line.number }}` for the item
// and `{{ record.record.orderId }}` for the root, while `keys[].field` addressed
// the very same values as `line.number` and `record.orderId`. Two conventions in
// one module, and `record.record.number` is guessable by nobody.
//
// Worse than unguessable: `{{ record.orderId }}` — the natural way to reach a
// root field — resolved to nothing at all, silently, because the scope has no
// such key.
//
// Unwrapping the scope makes both mechanisms agree: `line.number` is the item,
// `record.orderId` is the root, in templates and in field paths alike.
func ContextForRecord(record map[string]any) RenderContext {
	meta, _ := record[metadata.DefaultFieldName].(map[string]any)

	root, ok := loopScopeRoot(record, meta)
	if !ok {
		return RenderContext{Record: record, Meta: meta}
	}

	// The scope goes into Extra whole, without being filtered or copied: Vars
	// assigns the reserved names last, so the scope's own `record` key is
	// overwritten by the root record on its way in. One less map per rendered
	// field, and the shadowing rule does the work.
	//
	// meta is already the root record's metadata: the loop filter puts that very
	// map in the scope (it reads it from the root to write the loop bookkeeping),
	// so `meta` and `_metadata` name the same thing here.
	return RenderContext{Record: root, Meta: meta, Extra: record}
}

// loopScopeRoot reports whether record is a loop scope and, if so, returns the
// root record it carries.
//
// The signature is `_metadata.loop` — written by the loop filter, and stripped
// from inbound records by the executor so source data cannot forge it — plus a
// `record` key holding a map. Requiring both keeps a plain record that merely
// happens to have a field named `record` from being mistaken for a scope.
func loopScopeRoot(record, meta map[string]any) (root map[string]any, ok bool) {
	if len(meta) == 0 {
		return nil, false
	}
	loopMeta, isMap := meta[metadata.LoopFieldName].(map[string]any)
	if !isMap || len(loopMeta) == 0 {
		return nil, false
	}
	root, isMap = record[VarRecord].(map[string]any)
	if !isMap {
		return nil, false
	}
	return root, true
}

func orEmpty(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}
