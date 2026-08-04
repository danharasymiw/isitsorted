package parser

import (
	"math/big"
	"testing"
)

func TestParseMultiLang(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		// German
		{"german eins", "eins", 1, false},
		{"german drei hundert zwanzig", "drei hundert zwanzig", 320, false},
		{"german einundzwanzig", "einundzwanzig", 21, false},
		{"german dreiundvierzig", "dreiundvierzig", 43, false},
		{"german eine Million zweihundert", "eine Million zweihundert", 1000200, false},
		{"german minus drei", "minus drei", -3, false},

		// French
		{"french trois", "trois", 3, false},
		{"french trois cent vingt", "trois cent vingt", 320, false},
		{"french soixante-dix", "soixante-dix", 70, false},
		{"french quatre-vingts", "quatre-vingts", 80, false},
		{"french quatre-vingt-dix", "quatre-vingt-dix", 90, false},
		{"french quatre-vingt-onze", "quatre-vingt-onze", 91, false},
		{"french moins trois", "moins trois", -3, false},

		// Spanish
		{"spanish dos", "dos", 2, false},
		{"spanish veinticinco", "veinticinco", 25, false},
		{"spanish cien", "cien", 100, false},

		// Portuguese
		{"portuguese sete", "sete", 7, false},
		{"portuguese vinte", "vinte", 20, false},

		// Italian
		{"italian cinque", "cinque", 5, false},
		{"italian dieci", "dieci", 10, false},

		// Dutch
		{"dutch drie", "drie", 3, false},
		{"dutch twintig", "twintig", 20, false},

		// Swedish
		{"swedish fyra", "fyra", 4, false},
		{"swedish tjugo", "tjugo", 20, false},

		// Russian
		{"russian один", "один", 1, false},
		{"russian двадцать", "двадцать", 20, false},

		// Arabic
		{"arabic واحد", "واحد", 1, false},

		// Hindi
		{"hindi एक", "एक", 1, false},
		{"hindi दस", "दस", 10, false},

		// CJK - Chinese
		{"chinese 三百二十", "三百二十", 320, false},
		{"chinese 一", "一", 1, false},
		{"chinese 十", "十", 10, false},
		{"chinese 十五", "十五", 15, false},

		// CJK - Japanese (same characters as Chinese for basic numbers)
		{"japanese 五万", "五万", 50000, false},

		// Korean Sino
		{"korean sino 삼", "삼", 3, false},
		{"korean sino 삼백이십", "삼백이십", 320, false},
		{"korean sino 일", "일", 1, false},

		// Korean Native
		{"korean native 하나", "하나", 1, false},

		// Rejection
		{"garbage rejected", "drei banana", 0, true},
		{"empty rejected", "", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseMultiLang(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseMultiLang(%q) = %v, want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseMultiLang(%q) error: %v", tc.input, err)
			}
			want := big.NewInt(tc.want)
			if got.Cmp(want) != 0 {
				t.Errorf("parseMultiLang(%q) = %s, want %s", tc.input, got.String(), want.String())
			}
		})
	}
}

func TestParseValueMultiLang(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  *big.Rat
	}{
		{"german via ParseValue", "eins", rat("1")},
		{"spanish via ParseValue", "dos", rat("2")},
		{"korean via ParseValue", "삼", rat("3")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseValue(tc.input)
			if err != nil {
				t.Fatalf("ParseValue(%q) error: %v", tc.input, err)
			}
			if got.Cmp(tc.want) != 0 {
				t.Errorf("ParseValue(%q) = %s, want %s", tc.input, got.RatString(), tc.want.RatString())
			}
		})
	}
}
