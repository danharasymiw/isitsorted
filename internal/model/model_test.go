package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestJobRoundTrip(t *testing.T) {
	j := Job{
		ID:          "abc-123",
		BucketKey:   "lists/abc-123",
		Order:       "asc",
		SubmittedAt: time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC),
	}
	data, err := json.Marshal(j)
	if err != nil {
		t.Fatal(err)
	}
	var got Job
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != j.ID || got.BucketKey != j.BucketKey || got.Order != j.Order {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", got, j)
	}
	if !got.SubmittedAt.Equal(j.SubmittedAt) {
		t.Fatalf("round-trip mismatch: got SubmittedAt %v, want %v", got.SubmittedAt, j.SubmittedAt)
	}
}

func TestResultJSON(t *testing.T) {
	r := Result{ID: "abc", Status: StatusDone, Sorted: true}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var got Result
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusDone || !got.Sorted {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestResultErrorOmitsSorted(t *testing.T) {
	r := Result{ID: "abc", Status: StatusError, Error: "parse failed"}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	m := make(map[string]any)
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["sorted"]; ok {
		t.Fatal("sorted field should be omitted on error")
	}
}

func TestStatusEventRoundTrip(t *testing.T) {
	e := StatusEvent{Status: StatusProcessing}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	m := make(map[string]any)
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["sorted"]; ok {
		t.Fatal("sorted field should be omitted when false")
	}
	if _, ok := m["error"]; ok {
		t.Fatal("error field should be omitted when empty")
	}
}

func TestActivityEntryRoundTrip(t *testing.T) {
	a := ActivityEntry{
		At:     time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC),
		Sorted: true,
		Order:  "asc",
		List:   []string{"1", "2", "3"},
	}
	data, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	var got ActivityEntry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Sorted != a.Sorted || got.Order != a.Order || len(got.List) != len(a.List) {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", got, a)
	}
}
