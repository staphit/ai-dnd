// Package jsutil reproduces the JavaScript coercion semantics the original Node
// backend relied on (truthiness of `x || default`, `Number(...)`, `String(...)`
// and `String.prototype.slice(0, n)`), so the ported strings are byte-for-byte
// identical. It is shared by the DM prompt builder and the HTTP handlers.
package jsutil

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Truthy mirrors JavaScript truthiness for JSON-decoded values.
func Truthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case float64:
		return t != 0 && !math.IsNaN(t)
	case string:
		return t != ""
	default:
		return true
	}
}

// JSOr returns v when it is JS-truthy, otherwise def (like `v || def`).
func JSOr(v, def any) any {
	if Truthy(v) {
		return v
	}
	return def
}

// ToNum mirrors JavaScript Number(v) for JSON-decoded values.
func ToNum(v any) float64 {
	switch t := v.(type) {
	case nil:
		return 0
	case bool:
		if t {
			return 1
		}
		return 0
	case float64:
		return t
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0
		}
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return math.NaN()
		}
		return n
	default:
		return math.NaN()
	}
}

// NumOr returns Number(v || def).
func NumOr(v any, def float64) float64 { return ToNum(JSOr(v, def)) }

// NumToStr formats a number the way JS template interpolation would.
func NumToStr(f float64) string {
	if math.IsNaN(f) {
		return "NaN"
	}
	if math.IsInf(f, 1) {
		return "Infinity"
	}
	if math.IsInf(f, -1) {
		return "-Infinity"
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e21 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// NumStr = `${Number(v || def)}`.
func NumStr(v any, def float64) string { return NumToStr(NumOr(v, def)) }

// NumPlain = `${Number(v)}` (no `|| default`; undefined becomes NaN).
func NumPlain(v any) string { return NumToStr(ToNum(v)) }

// StrOf mirrors JavaScript String(v) for JSON-decoded values.
func StrOf(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case float64:
		return NumToStr(t)
	default:
		return ""
	}
}

// StrOr = String(v || def).
func StrOr(v, def any) string { return StrOf(JSOr(v, def)) }

// JSSlice reproduces s.slice(0, n): the first n UTF-16 code units.
func JSSlice(s string, n int) string {
	if n <= 0 {
		return ""
	}
	units := utf16.Encode([]rune(s))
	if n >= len(units) {
		return s
	}
	return string(utf16.Decode(units[:n]))
}

// AsSlice returns v as a []any when it is a JSON array, else nil (Array.isArray).
func AsSlice(v any) ([]any, bool) {
	s, ok := v.([]any)
	return s, ok
}

// AsMap returns v as a JSON object, else nil.
func AsMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

// Get navigates nested maps with `?.` semantics, returning nil on any miss.
func Get(v any, keys ...string) any {
	cur := v
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[k]
	}
	return cur
}

// ClampNum = Math.max(lo, Math.min(hi, n)).
func ClampNum(n, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, n))
}
