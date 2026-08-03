package main

import (
	"math/big"
	"testing"
)

func TestParseRoman(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
		ok    bool
	}{
		// Basic values
		{"I", "I", 1, true},
		{"V", "V", 5, true},
		{"X", "X", 10, true},
		{"L", "L", 50, true},
		{"C", "C", 100, true},
		{"D", "D", 500, true},
		{"M", "M", 1000, true},

		// Subtractive
		{"IV", "IV", 4, true},
		{"IX", "IX", 9, true},
		{"XL", "XL", 40, true},
		{"XC", "XC", 90, true},
		{"CD", "CD", 400, true},
		{"CM", "CM", 900, true},

		// Additive (lenient)
		{"IIII", "IIII", 4, true},
		{"VIIII", "VIIII", 9, true},

		// Compound
		{"MCMXCIX", "MCMXCIX", 1999, true},
		{"MMXXVI", "MMXXVI", 2026, true},
		{"MMMCMXCIX", "MMMCMXCIX", 3999, true},
		{"XLII", "XLII", 42, true},

		// Lowercase
		{"lowercase xiv", "xiv", 14, true},
		{"lowercase mcmxcix", "mcmxcix", 1999, true},

		// Mixed case
		{"mixed Xiv", "Xiv", 14, true},

		// Nulla (zero)
		{"nulla", "nulla", 0, true},
		{"NULLA", "NULLA", 0, true},

		// Rejection
		{"empty", "", 0, false},
		{"garbage", "banana", 0, false},
		{"mixed garbage", "XIbanana", 0, false},
		{"just spaces", "   ", 0, false},

		// Vinculum (combining overline U+0305)
		{"V̅ = 5000", "V̅", 5000, true},
		{"X̅ = 10000", "X̅", 10000, true},
		{"L̅ = 50000", "L̅", 50000, true},
		{"C̅ = 100000", "C̅", 100000, true},
		{"D̅ = 500000", "D̅", 500000, true},
		{"M̅ = 1000000", "M̅", 1000000, true},

		// Mixed vinculum + standard
		{"X̅MCMXCIX = 11999", "X̅MCMXCIX", 11999, true},
		{"X̅IV = 10004", "X̅IV", 10004, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseRoman(tc.input)
			if ok != tc.ok {
				t.Fatalf("parseRoman(%q) ok = %v, want %v", tc.input, ok, tc.ok)
			}
			if !ok {
				return
			}
			want := new(big.Rat).SetInt64(tc.want)
			if got.Cmp(want) != 0 {
				t.Errorf("parseRoman(%q) = %s, want %d", tc.input, got.RatString(), tc.want)
			}
		})
	}
}
