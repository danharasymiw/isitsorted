package main

import (
	"math/big"
	"testing"
)

func ints(ns ...int64) []*big.Rat {
	out := make([]*big.Rat, len(ns))
	for i, n := range ns {
		out[i] = new(big.Rat).SetInt64(n)
	}
	return out
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name  string
		list  []*big.Rat
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
	half := new(big.Rat).SetFloat64(0.5)
	onePointFive := new(big.Rat).SetFloat64(1.5)
	twoPointFive := new(big.Rat).SetFloat64(2.5)
	tests = append(tests,
		struct {
			name  string
			list  []*big.Rat
			order string
			want  bool
		}{"floats sorted", []*big.Rat{half, onePointFive, twoPointFive}, "asc", true},
		struct {
			name  string
			list  []*big.Rat
			order string
			want  bool
		}{"floats unsorted", []*big.Rat{onePointFive, half, twoPointFive}, "asc", false},
	)

	// Big integer case.
	a, _ := new(big.Rat).SetString("99999999999999999998")
	b, _ := new(big.Rat).SetString("99999999999999999999")
	c, _ := new(big.Rat).SetString("100000000000000000000")
	tests = append(tests,
		struct {
			name  string
			list  []*big.Rat
			order string
			want  bool
		}{"big ints sorted", []*big.Rat{a, b, c}, "asc", true},
	)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := check(tc.list, tc.order)
			if got != tc.want {
				t.Errorf("check(%v, %q) = %v, want %v", tc.list, tc.order, got, tc.want)
			}
		})
	}
}
