package parser

import (
	"math/big"
	"testing"
)

func TestParseBraille(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNum string
		ok      bool
	}{
		// Basic digits with number indicator
		{"indicator 1", "⠼⠁", "1", true},
		{"indicator 5", "⠼⠑", "5", true},
		{"indicator 0", "⠼⠚", "0", true},

		// Multi-digit with indicator
		{"indicator 12", "⠼⠁⠃", "12", true},
		{"indicator 42", "⠼⠙⠃", "42", true},
		{"indicator 100", "⠼⠁⠚⠚", "100", true},
		{"indicator 12345", "⠼⠁⠃⠉⠙⠑", "12345", true},

		// Without indicator (lenient)
		{"bare 12", "⠁⠃", "12", true},
		{"bare 42", "⠙⠃", "42", true},
		{"bare single 7", "⠛", "7", true},

		// Negative with Braille hyphen
		{"negative 5", "⠤⠼⠑", "-5", true},
		{"negative 42", "⠤⠼⠙⠃", "-42", true},
		{"negative bare", "⠤⠙⠃", "-42", true},

		// Decimal
		{"decimal 3.14", "⠼⠉⠲⠁⠙", "314/100", true},
		{"decimal 0.5", "⠼⠚⠲⠑", "1/2", true},
		{"negative decimal", "⠤⠼⠉⠲⠁⠙", "-314/100", true},

		// Rejection
		{"empty", "", "", false},
		{"non-braille", "hello", "", false},
		{"mixed braille ascii", "⠁abc", "", false},
		{"just indicator", "⠼", "", false},
		{"just hyphen", "⠤", "", false},
		{"indicator then hyphen", "⠼⠤", "", false},
		{"double decimal", "⠼⠁⠲⠃⠲⠉", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseBraille(tc.input)
			if ok != tc.ok {
				t.Fatalf("parseBraille(%q) ok = %v, want %v", tc.input, ok, tc.ok)
			}
			if !ok {
				return
			}
			want, _ := new(big.Rat).SetString(tc.wantNum)
			if got.Cmp(want) != 0 {
				t.Errorf("parseBraille(%q) = %s, want %s", tc.input, got.RatString(), want.RatString())
			}
		})
	}
}
