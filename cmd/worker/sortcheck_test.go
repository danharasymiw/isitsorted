package main

import (
	"math/big"
	"sorted/parser"
	"testing"
)

func ints(ns ...int64) []*parser.Value {
	out := make([]*parser.Value, len(ns))
	for i, n := range ns {
		out[i] = parser.PointValue(new(big.Rat).SetInt64(n))
	}
	return out
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name  string
		list  []*parser.Value
		order string
		want  bool
	}{
		{"empty", ints(), "asc", true},
		{"single element", ints(5), "asc", true},
		{"asc sorted", ints(1, 2, 3, 4), "asc", true},
		{"asc unsorted", ints(1, 3, 2, 4), "asc", false},
		{"asc equal adjacent", ints(1, 1, 2), "asc", true},
		{"asc unsorted at end", ints(1, 2, 3, 1), "asc", false},
		{"desc sorted", ints(4, 3, 2, 1), "desc", true},
		{"desc unsorted", ints(4, 2, 3, 1), "desc", false},
		{"desc equal adjacent", ints(3, 3, 1), "desc", true},
		{"desc unsorted at end", ints(4, 3, 2, 5), "desc", false},
	}

	// Float cases.
	half := parser.PointValue(new(big.Rat).SetFloat64(0.5))
	onePointFive := parser.PointValue(new(big.Rat).SetFloat64(1.5))
	twoPointFive := parser.PointValue(new(big.Rat).SetFloat64(2.5))
	tests = append(tests,
		struct {
			name  string
			list  []*parser.Value
			order string
			want  bool
		}{"floats sorted", []*parser.Value{half, onePointFive, twoPointFive}, "asc", true},
		struct {
			name  string
			list  []*parser.Value
			order string
			want  bool
		}{"floats unsorted", []*parser.Value{onePointFive, half, twoPointFive}, "asc", false},
	)

	// Big integer case.
	a, _ := new(big.Rat).SetString("99999999999999999998")
	b, _ := new(big.Rat).SetString("99999999999999999999")
	c, _ := new(big.Rat).SetString("100000000000000000000")
	tests = append(tests,
		struct {
			name  string
			list  []*parser.Value
			order string
			want  bool
		}{"big ints sorted", []*parser.Value{parser.PointValue(a), parser.PointValue(b), parser.PointValue(c)}, "asc", true},
	)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Check(tc.list, tc.order)
			if got != tc.want {
				t.Errorf("Check(%v, %q) = %v, want %v", tc.list, tc.order, got, tc.want)
			}
		})
	}
}

func TestCheckRanges(t *testing.T) {
	interval := func(min, max int64) *parser.Value {
		return &parser.Value{
			Min: new(big.Rat).SetInt64(min),
			Max: new(big.Rat).SetInt64(max),
		}
	}
	discrete := func(vals ...int64) *parser.Value {
		pts := make([]*big.Rat, len(vals))
		for i, v := range vals {
			pts[i] = new(big.Rat).SetInt64(v)
		}
		return parser.DiscreteValue(pts)
	}
	pt := func(n int64) *parser.Value {
		return parser.PointValue(new(big.Rat).SetInt64(n))
	}

	tests := []struct {
		name  string
		list  []*parser.Value
		order string
		want  bool
	}{
		// Continuous ranges use forall semantics
		{"interval between points asc", []*parser.Value{pt(7), interval(8, 12), pt(15)}, "asc", true},
		{"interval overlaps prev asc", []*parser.Value{pt(9), interval(8, 12), pt(15)}, "asc", false},
		{"interval overlaps next asc", []*parser.Value{pt(7), interval(8, 12), pt(11)}, "asc", false},
		{"overlapping intervals asc", []*parser.Value{interval(8, 12), interval(5, 15)}, "asc", false},
		{"interval between points desc", []*parser.Value{pt(15), interval(8, 12), pt(7)}, "desc", true},
		{"interval overlaps next desc", []*parser.Value{pt(15), interval(8, 12), pt(9)}, "desc", false},

		// Discrete sets use exists semantics (pick best value)
		{"discrete can pick lower asc", []*parser.Value{discrete(9, 11), pt(9)}, "asc", true},
		{"discrete pick fits between asc", []*parser.Value{pt(7), discrete(8, 12), pt(15)}, "asc", true},
		{"discrete no valid pick asc", []*parser.Value{pt(9), discrete(8, 12), pt(11)}, "asc", false},
		{"discrete overlapping sets asc", []*parser.Value{discrete(8, 12), discrete(5, 15)}, "asc", true},
		{"discrete all below prev asc", []*parser.Value{pt(15), discrete(8, 12)}, "asc", false},
		{"discrete can pick higher desc", []*parser.Value{discrete(9, 11), pt(11)}, "desc", true},
		{"discrete between points desc", []*parser.Value{pt(15), discrete(8, 12), pt(7)}, "desc", true},
		{"discrete no valid pick desc", []*parser.Value{pt(11), discrete(8, 12), pt(9)}, "desc", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Check(tc.list, tc.order)
			if got != tc.want {
				t.Errorf("Check(..., %q) = %v, want %v", tc.order, got, tc.want)
			}
		})
	}
}

func TestCheckInfinity(t *testing.T) {
	pt := func(n int64) *parser.Value {
		return parser.PointValue(new(big.Rat).SetInt64(n))
	}
	posInf := parser.InfValue(1)
	negInf := parser.InfValue(-1)

	tests := []struct {
		name  string
		list  []*parser.Value
		order string
		want  bool
	}{
		{"neg inf to pos inf asc", []*parser.Value{negInf, pt(0), posInf}, "asc", true},
		{"neg inf start asc", []*parser.Value{negInf, pt(-100), pt(0)}, "asc", true},
		{"pos inf end asc", []*parser.Value{pt(0), pt(100), posInf}, "asc", true},
		{"pos inf wrong position asc", []*parser.Value{posInf, pt(0)}, "asc", false},
		{"neg inf wrong position asc", []*parser.Value{pt(0), negInf}, "asc", false},
		{"pos inf to neg inf desc", []*parser.Value{posInf, pt(0), negInf}, "desc", true},
		{"pos inf start desc", []*parser.Value{posInf, pt(100), pt(0)}, "desc", true},
		{"neg inf end desc", []*parser.Value{pt(0), pt(-100), negInf}, "desc", true},
		{"neg inf wrong position desc", []*parser.Value{negInf, pt(0)}, "desc", false},
		{"two pos inf asc", []*parser.Value{posInf, posInf}, "asc", true},
		{"two neg inf asc", []*parser.Value{negInf, negInf}, "asc", true},
		{"only infinities asc", []*parser.Value{negInf, posInf}, "asc", true},
		{"only infinities desc", []*parser.Value{posInf, negInf}, "desc", true},
		{"only infinities wrong asc", []*parser.Value{posInf, negInf}, "asc", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Check(tc.list, tc.order)
			if got != tc.want {
				t.Errorf("Check(..., %q) = %v, want %v", tc.order, got, tc.want)
			}
		})
	}
}
