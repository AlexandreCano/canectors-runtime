package template

import (
	"fmt"
	"strings"
)

// tagKind classifies a segment produced by scanTemplate.
type tagKind int

const (
	kindText    tagKind = iota // literal text between tags
	kindOutput                 // {{ ... }}
	kindBlock                  // {% ... %} (not a raw block)
	kindComment                // {# ... #}
	kindRaw                    // a whole {% raw %} ... {% endraw %} region, verbatim
)

// scanTemplate tokenizes src into text / tag segments and invokes emit for each,
// in order, so the concatenation of every seg reconstructs src exactly.
//
// This is the single tokenizer shared by injectEscape and HasOutputTag — the
// security boundary. It must agree with gonja's lexer about where each tag
// starts and ends, otherwise a mis-parsed comment or raw block could hide an
// output tag from escaping. In particular:
//   - Comments close at the FIRST `#}` (gonja does not honor quotes inside
//     comments, so neither may we — a stray apostrophe in `{# don't #}` must
//     not swallow the rest of the template).
//   - `{% raw %}…{% endraw %}` is emitted as one verbatim region; the `{{ }}`
//     inside it are literal, not rendered.
//   - Inside `{{ }}` / `{% %}`, quoted strings are respected so a `}}` / `%}`
//     within a string literal does not end the tag early.
func scanTemplate(src string, emit func(kind tagKind, seg string)) {
	i, n := 0, len(src)
	textStart := 0

	flushText := func(upto int) {
		if upto > textStart {
			emit(kindText, src[textStart:upto])
		}
	}

	for i < n {
		if i+1 < n && src[i] == '{' && (src[i+1] == '{' || src[i+1] == '%' || src[i+1] == '#') {
			open := src[i+1]

			var end int
			if open == '#' {
				end = indexAfter(src, i+2, "#}")
			} else {
				end = findTagClose(src, i+2, tagCloseFor(open))
			}
			if end < 0 {
				// Unterminated tag: the remainder is literal; gonja will report
				// the precise parse error at compile time.
				break
			}

			if open == '%' && blockKeyword(src[i:end]) == "raw" {
				rawEnd := findRawEnd(src, end)
				flushText(i)
				emit(kindRaw, src[i:rawEnd])
				i, textStart = rawEnd, rawEnd
				continue
			}

			flushText(i)
			switch open {
			case '{':
				emit(kindOutput, src[i:end])
			case '%':
				emit(kindBlock, src[i:end])
			default:
				emit(kindComment, src[i:end])
			}
			i, textStart = end, end
			continue
		}
		i++
	}
	flushText(n)
}

// injectEscape rewrites every output tag `{{ expr }}` into `{{ (expr) | filter }}`
// so the substituted value is escaped for the target, leaving literal text,
// control structures, comments and `{% raw %}` blocks untouched. Whitespace
// markers are preserved: `{{- expr -}}` -> `{{- (expr) | filter -}}`. When
// filter is empty (TargetText) src is returned unchanged.
func injectEscape(src, filter string) string {
	if filter == "" || !strings.Contains(src, "{{") {
		return src
	}
	var b strings.Builder
	b.Grow(len(src) + 16)
	scanTemplate(src, func(kind tagKind, seg string) {
		if kind == kindOutput {
			b.WriteString(rewriteOutputTag(seg, filter))
		} else {
			b.WriteString(seg)
		}
	})
	return b.String()
}

// HasOutputTag reports whether src contains a rendered `{{ ... }}` output tag,
// ignoring `{{` that appear inside comments or `{% raw %}` blocks. SQL query
// templates use it to forbid value interpolation (all dynamic values must go
// through bound parameters).
func HasOutputTag(src string) bool {
	if !strings.Contains(src, "{{") {
		return false
	}
	found := false
	scanTemplate(src, func(kind tagKind, seg string) {
		if kind == kindOutput {
			found = true
		}
	})
	return found
}

// checkBlocks rejects the control structures that would defeat compile-time
// escape injection. It runs before injectEscape so the author gets a precise
// error instead of a silently unescaped (or corrupted) render:
//
//   - include / extends / import / from pull in another template that gonja
//     parses itself, so its {{ }} tags never get an escape filter — and the
//     path may come from record data (arbitrary file read, and for SQL queries
//     an injection vector that HasOutputTag cannot see).
//   - {% filter %} applies its filter *after* the injected escaping, which
//     corrupts the result ({% filter upper %} turns `&lt;` into `&LT;`). Inline
//     filters ({{ value | upper }}) are escaped last and stay correct, so the
//     block form is only rejected for targets that escape.
func checkBlocks(src string, target Target) error {
	if !strings.Contains(src, BlockPrefix) {
		return nil
	}
	var err error
	scanTemplate(src, func(kind tagKind, seg string) {
		if err != nil || kind != kindBlock {
			return
		}
		switch keyword := blockKeyword(seg); keyword {
		case "include", "extends", "import", "from":
			err = fmt.Errorf("{%% %s %%} is not supported: a template must be self-contained, "+
				"because the tags of a loaded template would bypass contextual escaping", keyword)
		case "filter":
			if target.escapes() {
				err = fmt.Errorf("{%% filter %%} is not supported in a %s template: it transforms "+
					"values after they are escaped, which corrupts the output — use an inline "+
					"filter instead ({{ value | upper }})", target)
			}
		}
	})
	return err
}

func tagCloseFor(open byte) string {
	switch open {
	case '{':
		return "}}"
	case '%':
		return "%}"
	default: // '#'
		return "#}"
	}
}

// indexAfter returns the index just past the first occurrence of sub at or after
// from, or -1 if not found.
func indexAfter(src string, from int, sub string) int {
	idx := strings.Index(src[from:], sub)
	if idx < 0 {
		return -1
	}
	return from + idx + len(sub)
}

// findTagClose returns the index just past closeStr, starting the search at from,
// while skipping over quoted string literals (so a `}}`/`%}` inside a string does
// not end the tag). Returns -1 if closeStr is never found. Used for `{{ }}` and
// `{% %}` tags, whose expression/statement lexers are quote-aware in gonja.
func findTagClose(src string, from int, closeStr string) int {
	var quote byte
	for i := from; i < len(src); i++ {
		c := src[i]
		if quote != 0 {
			if c == '\\' { // skip escaped char inside the string literal
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
		default:
			if strings.HasPrefix(src[i:], closeStr) {
				return i + len(closeStr)
			}
		}
	}
	return -1
}

// blockKeyword returns the first keyword of a `{% ... %}` tag (e.g. "if", "for",
// "raw", "endraw"), stripping whitespace-control markers.
func blockKeyword(tag string) string {
	inner := strings.TrimSpace(tag[2 : len(tag)-2])
	inner = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(inner, "-"), "-"))
	if sp := strings.IndexAny(inner, " \t"); sp >= 0 {
		return inner[:sp]
	}
	return inner
}

// findRawEnd returns the index just past the matching `{% endraw %}` starting the
// search at from (just past the `{% raw %}` opener). If none is found, returns
// len(src) so the remainder is emitted verbatim and gonja reports the unclosed
// block.
func findRawEnd(src string, from int) int {
	i, n := from, len(src)
	for i < n {
		if i+1 < n && src[i] == '{' && src[i+1] == '%' {
			end := findTagClose(src, i+2, "%}")
			if end < 0 {
				return n
			}
			if blockKeyword(src[i:end]) == "endraw" {
				return end
			}
			i = end
			continue
		}
		i++
	}
	return n
}

// rewriteOutputTag wraps the inner expression of a `{{ ... }}` tag with the
// escaping filter, preserving whitespace-control markers.
func rewriteOutputTag(tag, filter string) string {
	inner := tag[2 : len(tag)-2] // strip the {{ and }} delimiters

	left, right := "", ""
	if strings.HasPrefix(inner, "-") {
		left, inner = "-", inner[1:]
	}
	if strings.HasSuffix(inner, "-") {
		right, inner = "-", inner[:len(inner)-1]
	}

	expr := strings.TrimSpace(inner)
	if expr == "" {
		// Empty tag `{{ }}` — leave untouched; compilation will reject it.
		return tag
	}
	return "{{" + left + " (" + expr + ") | " + filter + " " + right + "}}"
}
