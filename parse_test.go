package main

import (
	"math/big"
	"testing"
)

func rat(s string) *big.Rat {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		panic("bad test rat: " + s)
	}
	return r
}

func TestParseValue(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *big.Rat
		wantErr bool
	}{
		// Plain integers
		{"int", "42", rat("42"), false},
		{"negative int", "-7", rat("-7"), false},
		{"zero", "0", rat("0"), false},

		// Floats
		{"float", "3.14", rat("314/100"), false},
		{"negative float", "-0.5", rat("-1/2"), false},

		// Big integers beyond int64
		{"big int", "99999999999999999999", rat("99999999999999999999"), false},
		{"negative big int", "-12345678901234567890", rat("-12345678901234567890"), false},

		// English words — basic
		{"word zero", "zero", rat("0"), false},
		{"word one", "one", rat("1"), false},
		{"word thirteen", "thirteen", rat("13"), false},
		{"word twenty", "twenty", rat("20"), false},
		{"word twenty-three", "twenty-three", rat("23"), false},
		{"word ninety-nine", "ninety-nine", rat("99"), false},

		// English words — hundreds
		{"word one hundred", "one hundred", rat("100"), false},
		{"word three hundred forty-two", "three hundred forty-two", rat("342"), false},

		// English words — thousands+
		{"word one thousand", "one thousand", rat("1000"), false},
		{"word thousand alone", "thousand", rat("1000"), false},
		{"word twelve thousand", "twelve thousand", rat("12000"), false},
		{"word complex", "three hundred forty-two thousand five hundred sixty-seven", rat("342567"), false},
		{"word million", "one million", rat("1000000"), false},
		{"word large", "two billion one hundred million", rat("2100000000"), false},

		// English words — negative
		{"word negative five", "negative five", rat("-5"), false},
		{"word minus twelve", "minus twelve", rat("-12"), false},

		// English words — with "and"
		{"word with and", "one hundred and one", rat("101"), false},

		// Case insensitivity
		{"mixed case", "Twenty-Three", rat("23"), false},

		// Math constants
		{"pi", "pi", rat("314159265358979323846/100000000000000000000"), false},
		{"pi unicode", "π", rat("314159265358979323846/100000000000000000000"), false},
		{"e", "e", rat("271828182845904523536/100000000000000000000"), false},
		{"tau", "tau", rat("628318530717958647692/100000000000000000000"), false},
		{"phi", "phi", rat("161803398874989484820/100000000000000000000"), false},
		{"zeta(3)", "ζ(3)", rat("120205690315959428540/100000000000000000000"), false},
		{"zeta(3) ascii", "zeta(3)", rat("120205690315959428540/100000000000000000000"), false},
		{"negative pi", "negative pi", rat("-314159265358979323846/100000000000000000000"), false},
		{"minus pi", "-pi", rat("-314159265358979323846/100000000000000000000"), false},
		{"Pi case insensitive", "Pi", rat("314159265358979323846/100000000000000000000"), false},

		// Expressions
		{"unary double neg", "-(-1)", rat("1"), false},
		{"addition", "1+2", rat("3"), false},
		{"subtraction", "10-3", rat("7"), false},
		{"multiplication", "3*4", rat("12"), false},
		{"division", "10/4", rat("5/2"), false},
		{"power", "2^10", rat("1024"), false},
		{"chained power right-assoc", "2^2^2", rat("16"), false},
		{"parens", "(1+2)*3", rat("9"), false},
		{"nested parens", "((2+3))", rat("5"), false},
		{"expr with constant", "pi*2", rat("628318530717958647692/100000000000000000000"), false},
		{"unary plus", "+5", rat("5"), false},
		{"plus-minus nominal", "1±2", rat("1"), false},
		{"plus-minus in expr", "3+1±0.5", rat("4"), false},
		{"division by zero", "1/0", nil, true},
		{"non-integer exponent", "2^1.5", nil, true},

		// Errors
		{"empty", "", nil, true},
		{"garbage", "banana", nil, true},
		{"partial garbage", "twenty banana", nil, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseValue(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseValue(%q) = %v, want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseValue(%q) error: %v", tc.input, err)
			}
			if got.Cmp(tc.want) != 0 {
				t.Errorf("parseValue(%q) = %s, want %s", tc.input, got.RatString(), tc.want.RatString())
			}
		})
	}
}

func TestConstantsAscending(t *testing.T) {
	// ζ(3) ≈ 1.202, φ ≈ 1.618, e ≈ 2.718, π ≈ 3.142, τ ≈ 6.283
	inputs := []string{"ζ(3)", "φ", "e", "π", "τ"}
	var list []*big.Rat
	for _, s := range inputs {
		v, err := parseValue(s)
		if err != nil {
			t.Fatalf("parseValue(%q): %v", s, err)
		}
		list = append(list, v)
	}
	if !check(list, "asc") {
		t.Errorf("expected [ζ(3), φ, e, π, τ] to be sorted ascending")
	}
}

func TestFormatRat(t *testing.T) {
	tests := []struct {
		input *big.Rat
		want  string
	}{
		{rat("42"), "42"},
		{rat("-7"), "-7"},
		{rat("314/100"), "3.14"},
		{rat("1/2"), "0.5"},
		{rat("1/3"), "0.3333333333"},
		{rat("99999999999999999999"), "99999999999999999999"},
	}
	for _, tc := range tests {
		got := formatRat(tc.input)
		if got != tc.want {
			t.Errorf("formatRat(%s) = %q, want %q", tc.input.RatString(), got, tc.want)
		}
	}
}
