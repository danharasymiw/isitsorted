package main

import (
	"strings"
	"testing"
	"time"
)

func TestFormatCount(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{1, "1"},
		{999, "999"},
		{1000, "1,000"},
		{12345, "12,345"},
		{1234567, "1,234,567"},
		{-1, "-1"},
		{-1234, "-1234"},
	}
	for _, tt := range tests {
		got := formatCount(tt.n)
		if got != tt.want {
			t.Errorf("formatCount(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestTimeAgo(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "just now"},
		{500 * time.Millisecond, "just now"},
		{30 * time.Second, "30s ago"},
		{5 * time.Minute, "5m ago"},
		{90 * time.Minute, "1h ago"},
		{2 * time.Hour, "2h ago"},
		{48 * time.Hour, "2d ago"},
		{7 * 24 * time.Hour, "7d ago"},
	}
	for _, tt := range tests {
		got := timeAgo(time.Now().Add(-tt.d))
		if got != tt.want {
			t.Errorf("timeAgo(-%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestOrderLabel(t *testing.T) {
	tests := []struct {
		order string
		want  string
	}{
		{"asc", "ascending"},
		{"desc", "descending"},
		{"", "ascending"},
		{"anything", "ascending"},
	}
	for _, tt := range tests {
		got := orderLabel(tt.order)
		if got != tt.want {
			t.Errorf("orderLabel(%q) = %q, want %q", tt.order, got, tt.want)
		}
	}
}

func TestResultHTML(t *testing.T) {
	sorted := resultHTML(true)
	if !strings.Contains(sorted, "Yes, it's sorted!") {
		t.Errorf("resultHTML(true) missing 'Yes' text: %s", sorted)
	}
	if !strings.Contains(sorted, "result-card yes") {
		t.Errorf("resultHTML(true) missing 'yes' class: %s", sorted)
	}

	notSorted := resultHTML(false)
	if !strings.Contains(notSorted, "Nope, not sorted.") {
		t.Errorf("resultHTML(false) missing 'Nope' text: %s", notSorted)
	}
	if !strings.Contains(notSorted, "result-card no") {
		t.Errorf("resultHTML(false) missing 'no' class: %s", notSorted)
	}
}

func TestCountHTML(t *testing.T) {
	got := countHTML(1234, 600, 634)
	if !strings.Contains(got, "1,234") {
		t.Errorf("countHTML missing formatted total: %s", got)
	}
	if !strings.Contains(got, "600") {
		t.Errorf("countHTML missing sorted count: %s", got)
	}
	if !strings.Contains(got, "634") {
		t.Errorf("countHTML missing not-sorted count: %s", got)
	}
	if !strings.Contains(got, "sorted-count-display") {
		t.Errorf("countHTML missing OOB swap target: %s", got)
	}
}

func TestRenderActivity(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		got := renderActivity(nil)
		if !strings.Contains(got, "No checks yet") {
			t.Errorf("renderActivity(nil) = %q, want empty state", got)
		}
	})

	t.Run("with entries", func(t *testing.T) {
		entries := []ActivityEntry{
			{At: time.Now(), Sorted: true, Order: "asc", List: []string{"1", "2", "3"}},
			{At: time.Now(), Sorted: false, Order: "desc", List: []string{"5", "1"}},
		}
		got := renderActivity(entries)
		if !strings.Contains(got, "activity-entry sorted") {
			t.Error("missing sorted entry class")
		}
		if !strings.Contains(got, "1, 2, 3") {
			t.Error("missing list content")
		}
		if !strings.Contains(got, "ascending") {
			t.Error("missing order label")
		}
		if !strings.Contains(got, "descending") {
			t.Error("missing descending label")
		}
	})
}
