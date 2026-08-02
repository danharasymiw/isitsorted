package main

import "testing"

func TestCheck(t *testing.T) {
	tests := []struct {
		name  string
		list  []int
		order string
		want  bool
	}{
		{"empty", []int{}, "asc", true},
		{"single element", []int{5}, "asc", true},
		{"asc sorted", []int{1, 2, 3, 4}, "asc", true},
		{"asc unsorted", []int{1, 3, 2, 4}, "asc", false},
		{"asc equal adjacent", []int{1, 1, 2}, "asc", true},
		{"asc unsorted at end", []int{1, 2, 3, 1}, "asc", false},
		{"desc sorted", []int{4, 3, 2, 1}, "desc", true},
		{"desc unsorted", []int{4, 2, 3, 1}, "desc", false},
		{"desc equal adjacent", []int{3, 3, 1}, "desc", true},
		{"desc unsorted at end", []int{4, 3, 2, 5}, "desc", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := check(tc.list, tc.order)
			if got != tc.want {
				t.Errorf("check(%v, %q) = %v, want %v", tc.list, tc.order, got, tc.want)
			}
		})
	}
}
