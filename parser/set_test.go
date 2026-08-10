package parser

import (
	"testing"
)

func TestSplitBracketAware(t *testing.T) {
	tests := []struct {
		name  string
		input string
		sep   rune
		want  []string
	}{
		{"simple", "a,b,c", ',', []string{"a", "b", "c"}},
		{"single", "a", ',', []string{"a"}},
		{"nested braces", "{1,2},3", ',', []string{"{1,2}", "3"}},
		{"nested brackets", "[1,2],3", ',', []string{"[1,2]", "3"}},
		{"nested parens", "(1,2),3", ',', []string{"(1,2)", "3"}},
		{"deep nesting", "{[1,2],(3,4)},5", ',', []string{"{[1,2],(3,4)}", "5"}},
		{"empty", "", ',', []string{""}},
		{"spaces", " a , b , c ", ',', []string{" a ", " b ", " c "}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitBracketAware(tt.input, tt.sep)
			if len(got) != len(tt.want) {
				t.Fatalf("SplitBracketAware(%q, %q) = %v (len %d), want %v (len %d)",
					tt.input, string(tt.sep), got, len(got), tt.want, len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("part[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"integer", "42", "42"},
		{"float", "3.14", "3.14"},
		{"uncertainty", "10 +/- 2", "10±2"},
		{"interval", "[1..5]", "3±2"},
		{"set discrete", "{1, 3, 7}", "4±3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := ParseValue(tt.input)
			if err != nil {
				t.Fatalf("ParseValue(%q) error: %v", tt.input, err)
			}
			got := FormatValue(v)
			if got != tt.want {
				t.Errorf("FormatValue(ParseValue(%q)) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
