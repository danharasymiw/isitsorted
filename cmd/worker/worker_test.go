package main

import (
	"testing"
)

func TestSplitLines(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"1\n2\n3", []string{"1", "2", "3"}},
		{"1\n2\n3\n", []string{"1", "2", "3"}},
		{"  1  \n  2  \n  3  \n", []string{"1", "2", "3"}},
		{"", nil},
		{"\n\n\n", nil},
		{"single", []string{"single"}},
	}
	for _, tt := range tests {
		got := splitLines(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitLines(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitLines(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}
