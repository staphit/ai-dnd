package jsutil

import (
	"math"
	"testing"
)

func TestTruthy(t *testing.T) {
	truthyCases := []any{true, 1.0, -1.0, "x", []any{}, map[string]any{}}
	for _, v := range truthyCases {
		if !Truthy(v) {
			t.Errorf("Truthy(%v) = false, want true", v)
		}
	}
	falsyCases := []any{nil, false, 0.0, "", math.NaN()}
	for _, v := range falsyCases {
		if Truthy(v) {
			t.Errorf("Truthy(%v) = true, want false", v)
		}
	}
}

func TestToNum(t *testing.T) {
	cases := []struct {
		in   any
		want float64
	}{
		{nil, 0}, {true, 1}, {false, 0}, {3.5, 3.5}, {"42", 42}, {"", 0}, {"  7 ", 7},
	}
	for _, c := range cases {
		if got := ToNum(c.in); got != c.want {
			t.Errorf("ToNum(%v) = %v, want %v", c.in, got, c.want)
		}
	}
	if !math.IsNaN(ToNum("abc")) {
		t.Error("ToNum(\"abc\") should be NaN")
	}
}

func TestNumToStrAndNumStr(t *testing.T) {
	if NumToStr(3) != "3" || NumToStr(3.5) != "3.5" || NumToStr(-2) != "-2" || NumToStr(0) != "0" {
		t.Error("NumToStr formatting wrong")
	}
	// NumStr uses `|| default`: 0 is falsy so the default wins.
	if NumStr(nil, 10) != "10" || NumStr(0.0, 3) != "3" || NumStr(5.0, 0) != "5" {
		t.Error("NumStr default handling wrong")
	}
	// NumPlain has no default: 0 stays 0.
	if NumPlain(0.0) != "0" || NumPlain(5.0) != "5" {
		t.Error("NumPlain wrong")
	}
}

func TestStrOfAndStrOr(t *testing.T) {
	if StrOf(nil) != "" || StrOf("x") != "x" || StrOf(true) != "true" || StrOf(3.0) != "3" {
		t.Error("StrOf wrong")
	}
	if StrOr(nil, "d") != "d" || StrOr("", "d") != "d" || StrOr("x", "d") != "x" {
		t.Error("StrOr wrong")
	}
}

func TestJSSlice(t *testing.T) {
	cases := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 3, "hel"},
		{"hello", 10, "hello"},
		{"hello", 0, ""},
		{"hello", -1, ""},
		{"中文字符", 2, "中文"},
		{"", 5, ""},
	}
	for _, c := range cases {
		if got := JSSlice(c.s, c.n); got != c.want {
			t.Errorf("JSSlice(%q, %d) = %q, want %q", c.s, c.n, got, c.want)
		}
	}
}

func TestJSOr(t *testing.T) {
	if JSOr(nil, "d") != "d" || JSOr("x", "d") != "x" || JSOr(0.0, 5.0) != 5.0 {
		t.Error("JSOr wrong")
	}
}

func TestGetAndContainers(t *testing.T) {
	m := map[string]any{"a": map[string]any{"b": "deep"}, "arr": []any{1.0, 2.0}}
	if Get(m, "a", "b") != "deep" {
		t.Error("Get nested miss")
	}
	if Get(m, "a", "missing") != nil || Get(m, "x", "y") != nil {
		t.Error("Get should return nil on miss")
	}
	if s, ok := AsSlice(m["arr"]); !ok || len(s) != 2 {
		t.Error("AsSlice wrong")
	}
	if _, ok := AsSlice("not a slice"); ok {
		t.Error("AsSlice should reject non-slice")
	}
	if AsMap(m["a"]) == nil || AsMap("x") != nil {
		t.Error("AsMap wrong")
	}
}

func TestClampNum(t *testing.T) {
	if ClampNum(15, 5, 30) != 15 || ClampNum(2, 5, 30) != 5 || ClampNum(99, 5, 30) != 30 {
		t.Error("ClampNum wrong")
	}
}
