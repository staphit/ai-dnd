package dm

import "dndduet/internal/jsutil"

// These lowercase wrappers keep the ported request/schema code reading like the
// original JS while delegating to the shared jsutil implementation.

func truthy(v any) bool                  { return jsutil.Truthy(v) }
func toNum(v any) float64                { return jsutil.ToNum(v) }
func numOr(v any, def float64) float64   { return jsutil.NumOr(v, def) }
func numToStr(f float64) string          { return jsutil.NumToStr(f) }
func numStr(v any, def float64) string   { return jsutil.NumStr(v, def) }
func numPlain(v any) string              { return jsutil.NumPlain(v) }
func strOf(v any) string                 { return jsutil.StrOf(v) }
func strOr(v, def any) string            { return jsutil.StrOr(v, def) }
func jsSlice(s string, n int) string     { return jsutil.JSSlice(s, n) }
func asSlice(v any) ([]any, bool)        { return jsutil.AsSlice(v) }
func asMap(v any) map[string]any         { return jsutil.AsMap(v) }
func get(v any, keys ...string) any      { return jsutil.Get(v, keys...) }
func clampNum(n, lo, hi float64) float64 { return jsutil.ClampNum(n, lo, hi) }
