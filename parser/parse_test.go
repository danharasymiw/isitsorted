package parser

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
		{"division by zero", "1/0", nil, true},
		{"non-integer exponent", "2^1.5", nil, true},
		{"negative power with zero in set", "(5±5)^-1", nil, true},

		// Factorials
		{"factorial 0", "0!", rat("1"), false},
		{"factorial 1", "1!", rat("1"), false},
		{"factorial 5", "5!", rat("120"), false},
		{"factorial 10", "10!", rat("3628800"), false},
		{"factorial in expr", "3!+1", rat("7"), false},
		{"factorial with pow", "3!^2", rat("36"), false},
		{"neg factorial", "-3!", rat("-6"), false},
		{"factorial negative input", "(-3)!", nil, true},

		// Emoji digits
		{"emoji single", "3️⃣", rat("3"), false},
		{"emoji multi", "1️⃣2️⃣3️⃣", rat("123"), false},
		{"emoji keycap ten", "🔟", rat("10"), false},
		{"emoji 🔢", "🔢", rat("1234"), false},
		{"emoji 🔢5️⃣", "🔢5️⃣", rat("12345"), false},
		{"emoji in expr", "2️⃣^1️⃣0️⃣", rat("1024"), false},
		{"emoji mixed", "1️⃣0️⃣+5️⃣", rat("15"), false},

		// Roman numerals
		{"roman I", "I", rat("1"), false},
		{"roman XIV", "XIV", rat("14"), false},
		{"roman MCMXCIX", "MCMXCIX", rat("1999"), false},
		{"roman lowercase", "xlii", rat("42"), false},
		{"roman nulla", "nulla", rat("0"), false},
		{"roman vinculum", "X̅", rat("10000"), false},

		// Braille numbers
		{"braille 1", "⠼⠁", rat("1"), false},
		{"braille 42", "⠼⠙⠃", rat("42"), false},
		{"braille bare", "⠁⠃", rat("12"), false},
		{"braille negative", "⠤⠼⠑", rat("-5"), false},
		{"braille decimal", "⠼⠉⠲⠁⠙", rat("314/100"), false},

		// Errors
		{"empty", "", nil, true},
		{"garbage", "banana", nil, true},
		{"partial garbage", "twenty banana", nil, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseValue(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("ParseValue(%q) = %v, want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseValue(%q) error: %v", tc.input, err)
			}
			if got.Min.Cmp(tc.want) != 0 || got.Max.Cmp(tc.want) != 0 {
				t.Errorf("ParseValue(%q) = [%s..%s], want %s",
					tc.input, got.Min.RatString(), got.Max.RatString(), tc.want.RatString())
			}
		})
	}
}

func TestParseInfinity(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantInf int8
	}{
		// Symbols
		{"inf", "inf", 1},
		{"infinity", "infinity", 1},
		{"unicode ∞", "∞", 1},
		{"neg inf", "-inf", -1},
		{"neg infinity", "-infinity", -1},
		{"neg unicode", "-∞", -1},
		{"negative infinity word", "negative infinity", -1},
		{"minus infinity word", "minus infinity", -1},
		{"case insensitive", "Infinity", 1},
		{"case insensitive INF", "INF", 1},

		// German
		{"german unendlich", "unendlich", 1},
		{"german unendlichkeit", "unendlichkeit", 1},

		// French
		{"french infini", "infini", 1},

		// Spanish/Portuguese/Italian
		{"spanish infinito", "infinito", 1},

		// Dutch
		{"dutch oneindig", "oneindig", 1},

		// Swedish
		{"swedish oändlig", "oändlig", 1},

		// Russian
		{"russian бесконечность", "бесконечность", 1},

		// Arabic
		{"arabic لانهاية", "لانهاية", 1},

		// Hindi
		{"hindi अनंत", "अनंत", 1},

		// Japanese/Chinese traditional
		{"japanese 無限", "無限", 1},

		// Chinese simplified
		{"chinese 无限", "无限", 1},

		// Korean
		{"korean 무한", "무한", 1},

		// Negative multilingual
		{"negative german", "-unendlich", -1},
		{"negative french", "minus infini", -1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseValue(tc.input)
			if err != nil {
				t.Fatalf("ParseValue(%q) error: %v", tc.input, err)
			}
			if got.Inf != tc.wantInf {
				t.Errorf("ParseValue(%q).Inf = %d, want %d", tc.input, got.Inf, tc.wantInf)
			}
		})
	}
}

func TestFormatInfinity(t *testing.T) {
	pos := InfValue(1)
	neg := InfValue(-1)
	if got := FormatValue(pos); got != "∞" {
		t.Errorf("FormatValue(+∞) = %q, want %q", got, "∞")
	}
	if got := FormatValue(neg); got != "-∞" {
		t.Errorf("FormatValue(-∞) = %q, want %q", got, "-∞")
	}
}

func TestConstantsAscending(t *testing.T) {
	// ζ(3) ≈ 1.202, φ ≈ 1.618, e ≈ 2.718, π ≈ 3.142, τ ≈ 6.283
	inputs := []string{"ζ(3)", "φ", "e", "π", "τ"}
	var list []*big.Rat
	for _, s := range inputs {
		v, err := ParseValue(s)
		if err != nil {
			t.Fatalf("ParseValue(%q): %v", s, err)
		}
		list = append(list, v.Min)
	}
	for i := 1; i < len(list); i++ {
		if list[i].Cmp(list[i-1]) < 0 {
			t.Errorf("expected [ζ(3), φ, e, π, τ] to be sorted ascending")
			break
		}
	}
}

func TestParseValueRanges(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMin *big.Rat
		wantMax *big.Rat
		wantErr bool
	}{
		// Basic uncertainty
		{"10±1", "10±1", rat("9"), rat("11"), false},
		{"10±2", "10±2", rat("8"), rat("12"), false},
		{"0±5", "0±5", rat("-5"), rat("5"), false},

		// +/- alias
		{"10+/-1", "10+/-1", rat("9"), rat("11"), false},

		// Uncertainty in expressions
		{"(10±1)*2", "(10±1)*2", rat("18"), rat("22"), false},
		{"(10±2)+3", "(10±2)+3", rat("11"), rat("15"), false},
		{"5+10±2", "5+10±2", rat("13"), rat("17"), false},

		// Arithmetic with ranges
		{"neg uncertainty", "-(10±1)", rat("-11"), rat("-9"), false},
		{"uncertainty squared", "(10±1)^2", rat("81"), rat("121"), false},

		// Discrete {-2, 2} squared: both give 4
		{"span zero squared", "(0±2)^2", rat("4"), rat("4"), false},

		// Interval notation
		{"closed interval dots", "[9..11]", rat("9"), rat("11"), false},
		{"open interval dots", "(9..11)", rat("9"), rat("11"), false},
		{"mixed interval [)", "[9..11)", rat("9"), rat("11"), false},
		{"mixed interval (]", "(9..11]", rat("9"), rat("11"), false},
		{"closed interval comma", "[9, 11]", rat("9"), rat("11"), false},
		{"interval with floats", "[1.5..3.5]", rat("3/2"), rat("7/2"), false},
		{"interval with negatives", "[-5..5]", rat("-5"), rat("5"), false},

		// Finite sets
		{"set of three", "{1, 3, 7}", rat("1"), rat("7"), false},
		{"set of two", "{5, 10}", rat("5"), rat("10"), false},
		{"set single", "{42}", rat("42"), rat("42"), false},
		{"set unordered", "{7, 2, 9, 1}", rat("1"), rat("9"), false},

		// Set-builder notation
		{"set-builder closed", "{x | x ∈ [9..11]}", rat("9"), rat("11"), false},
		{"set-builder open", "{x | x ∈ (0..100)}", rat("0"), rat("100"), false},
		{"set-builder with in", "{n | n in [1..5]}", rat("1"), rat("5"), false},
		{"set-builder colon", "{x : x ∈ [9..11]}", rat("9"), rat("11"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseValue(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("ParseValue(%q) = [%s..%s], want error", tc.input,
						got.Min.RatString(), got.Max.RatString())
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseValue(%q) error: %v", tc.input, err)
			}
			if got.Min.Cmp(tc.wantMin) != 0 || got.Max.Cmp(tc.wantMax) != 0 {
				t.Errorf("ParseValue(%q) = [%s..%s], want [%s..%s]",
					tc.input,
					got.Min.RatString(), got.Max.RatString(),
					tc.wantMin.RatString(), tc.wantMax.RatString())
			}
		})
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
		got := FormatRat(tc.input)
		if got != tc.want {
			t.Errorf("FormatRat(%s) = %q, want %q", tc.input.RatString(), got, tc.want)
		}
	}
}
