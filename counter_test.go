package main

import (
	"path/filepath"
	"testing"
)

func TestNewCounter_StartsAtZero(t *testing.T) {
	c := newCounter(filepath.Join(t.TempDir(), "count.json"))
	if c.value() != 0 {
		t.Fatalf("want 0, got %d", c.value())
	}
}

func TestCounter_Increment(t *testing.T) {
	c := newCounter(filepath.Join(t.TempDir(), "count.json"))
	c.increment(true)
	c.increment(true)
	if c.value() != 2 {
		t.Fatalf("want 2, got %d", c.value())
	}
}

func TestCounter_PersistsAcrossRestarts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "count.json")
	c1 := newCounter(path)
	c1.increment(true)
	c1.increment(true)
	c1.increment(true)

	c2 := newCounter(path)
	if c2.value() != 3 {
		t.Fatalf("want 3 after reload, got %d", c2.value())
	}
}

func TestFormatCount(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{1234567, "1,234,567"},
	}
	for _, tc := range cases {
		got := formatCount(tc.n)
		if got != tc.want {
			t.Errorf("formatCount(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}
