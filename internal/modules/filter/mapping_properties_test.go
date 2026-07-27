package filter

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// The generated matrix pins one input per transform. These targets state
// properties that must hold for *every* input instead, which is what catches
// the cases nobody thought to write down. They are fuzz targets so the corpus
// grows in CI, and they double as ordinary tests on the seeds.

// Trimming twice must equal trimming once, or a transform chain would depend on
// how many times it ran.
func FuzzTransform_TrimIsIdempotent(f *testing.F) {
	for _, seed := range []string{"", " ", "  x  ", "\t\nx\r\n", "x", "  é  ", " x "} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		once, err := applyTrim(s)
		if err != nil {
			t.Fatalf("applyTrim(%q) error = %v", s, err)
		}
		twice, err := applyTrim(once)
		if err != nil {
			t.Fatalf("applyTrim(%q) error = %v", once, err)
		}
		if once != twice {
			t.Errorf("trim is not idempotent for %q: %q then %q", s, once, twice)
		}
	})
}

// Each case transform is idempotent, so a chain does not depend on how many
// times it ran.
//
// A stronger property is tempting — uppercase(lowercase(s)) == uppercase(s) —
// but it is false in Unicode and the first run of this target proved it:
// "İstanbul" lowercases to "i" plus a combining dot, which uppercases back to
// plain "ISTANBUL" rather than "İSTANBUL". The same happens with ß, which
// uppercases to SS and never returns. That is standard Go case folding, not a
// defect here, and it is worth knowing before writing a mapping that folds case
// twice on non-ASCII data.
func FuzzTransform_CaseTransformsAreIdempotent(f *testing.F) {
	for _, seed := range []string{"", "MiXeD", "ÉLAN", "ßstrasse", "ǅ", "123", "İstanbul"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if !utf8.ValidString(s) {
			t.Skip("case folding is only defined for valid UTF-8")
		}
		for name, apply := range map[string]func(any) (any, error){
			"uppercase": applyUppercase,
			"lowercase": applyLowercase,
		} {
			once, err := apply(s)
			if err != nil {
				t.Fatalf("%s(%q) error = %v", name, s, err)
			}
			twice, err := apply(once)
			if err != nil {
				t.Fatalf("%s(second pass) error = %v", name, err)
			}
			if once != twice {
				t.Errorf("%s is not idempotent for %q: %q then %q", name, s, once, twice)
			}
		}
	})
}

// split then join with the same separator returns the input with each part
// trimmed — split trims, so this is the exact round-trip, not plain identity.
func FuzzTransform_SplitJoinRoundTrip(f *testing.F) {
	for _, seed := range []string{"", "a,b,c", " a , b ", ",", "a", "a,,b", "é,ü"} {
		f.Add(seed, ",")
	}
	f.Fuzz(func(t *testing.T, s, sep string) {
		if sep == "" {
			t.Skip("an empty separator falls back to the default, breaking the pairing")
		}
		parts, err := applySplit(s, sep)
		if err != nil {
			t.Fatalf("applySplit(%q, %q) error = %v", s, sep, err)
		}
		joined, err := applyJoin(parts, sep)
		if err != nil {
			t.Fatalf("applyJoin error = %v", err)
		}
		want := make([]string, 0)
		for _, part := range strings.Split(s, sep) {
			want = append(want, strings.TrimSpace(part))
		}
		if joined != strings.Join(want, sep) {
			t.Errorf("split then join of %q with %q = %q, want %q",
				s, sep, joined, strings.Join(want, sep))
		}
	})
}

// Any integer survives a trip through toString and back, which is what a
// mapping chain relies on when it normalises a numeric field.
func FuzzTransform_IntRoundTrip(f *testing.F) {
	for _, seed := range []int64{0, 1, -1, 42, -9007199254740991, 9007199254740991} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, n int64) {
		asString, err := applyToString(int(n))
		if err != nil {
			t.Fatalf("applyToString(%d) error = %v", n, err)
		}
		back, err := applyToInt(asString)
		if err != nil {
			t.Fatalf("applyToInt(%q) error = %v", asString, err)
		}
		if back != int(n) {
			t.Errorf("toInt(toString(%d)) = %v, want %d", n, back, n)
		}
	})
}

// toInt must accept exactly what strconv accepts: no string parses in one and
// fails in the other, which is what makes the failure messages trustworthy.
func FuzzTransform_ToIntMatchesStrconv(f *testing.F) {
	for _, seed := range []string{"0", "42", "-7", " 8 ", "1_000", "0x10", "", "abc", "9999999999999999999999"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got, gotErr := applyToInt(s)
		want, wantErr := strconv.Atoi(strings.TrimSpace(s))
		if (gotErr == nil) != (wantErr == nil) {
			t.Fatalf("applyToInt(%q) error = %v, strconv.Atoi error = %v", s, gotErr, wantErr)
		}
		if gotErr == nil && got != want {
			t.Errorf("applyToInt(%q) = %v, want %v", s, got, want)
		}
	})
}

// toArray wraps a scalar once and then leaves it alone: applying it twice must
// not nest the value deeper, otherwise chains would build ragged structures.
func FuzzTransform_ToArrayWrapsOnlyOnce(f *testing.F) {
	for _, seed := range []string{"", "x", "[]"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		once, err := applyToArray(s)
		if err != nil {
			t.Fatalf("applyToArray(%q) error = %v", s, err)
		}
		twice, err := applyToArray(once)
		if err != nil {
			t.Fatalf("applyToArray(second pass) error = %v", err)
		}
		if fmt.Sprint(once) != fmt.Sprint(twice) {
			t.Errorf("toArray is not idempotent for %q: %v then %v", s, once, twice)
		}
	})
}

// A pattern that matches nothing must leave the value untouched.
func FuzzTransform_ReplaceWithNoMatchIsIdentity(f *testing.F) {
	for _, seed := range []string{"", "abc", "line\nbreak", "é"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		// A pattern that cannot occur in the input, whatever the input is.
		pattern := regexpMustNotMatch(t)
		got, err := applyReplace(s, pattern, "REPLACED")
		if err != nil {
			t.Fatalf("applyReplace(%q) error = %v", s, err)
		}
		if got != s {
			t.Errorf("replace with a non-matching pattern changed %q into %v", s, got)
		}
	})
}

// regexpMustNotMatch builds a pattern that cannot appear in fuzz input: the
// generator produces strings, and this requires a byte sequence no valid string
// can contain in that position.
func regexpMustNotMatch(t *testing.T) *regexp.Regexp {
	t.Helper()
	pattern, err := regexp.Compile(`\x00NEVER-MATCHES-THIS-SENTINEL\x00`)
	if err != nil {
		t.Fatalf("compiling the sentinel pattern: %v", err)
	}
	return pattern
}
