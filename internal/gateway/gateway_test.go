package gateway

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseJSONValid(t *testing.T) {
	body := `{"list": [1, 2, 3], "order": "asc"}`
	r := httptest.NewRequest("POST", "/is-sorted", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	rawList, order, err := parseJSON(r)
	if err != nil {
		t.Fatal(err)
	}
	if order != "asc" {
		t.Fatalf("order = %q, want asc", order)
	}
	if len(rawList) != 3 {
		t.Fatalf("len = %d, want 3", len(rawList))
	}
}

func TestParseJSONDefaultOrder(t *testing.T) {
	body := `{"list": [1, 2]}`
	r := httptest.NewRequest("POST", "/is-sorted", strings.NewReader(body))
	_, order, err := parseJSON(r)
	if err != nil {
		t.Fatal(err)
	}
	if order != "asc" {
		t.Fatalf("order = %q, want asc", order)
	}
}

func TestParseJSONInvalidOrder(t *testing.T) {
	body := `{"list": [1], "order": "sideways"}`
	r := httptest.NewRequest("POST", "/is-sorted", strings.NewReader(body))
	_, _, err := parseJSON(r)
	if err == nil {
		t.Fatal("expected error for invalid order")
	}
}

func TestParseJSONMissingList(t *testing.T) {
	body := `{"order": "asc"}`
	r := httptest.NewRequest("POST", "/is-sorted", strings.NewReader(body))
	_, _, err := parseJSON(r)
	if err == nil {
		t.Fatal("expected error for missing list")
	}
}

func TestParseFormValid(t *testing.T) {
	body := "list=1%0A2%0A3&order=asc"
	r := httptest.NewRequest("POST", "/is-sorted", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rawList, order, err := parseForm(r)
	if err != nil {
		t.Fatal(err)
	}
	if order != "asc" {
		t.Fatalf("order = %q, want asc", order)
	}
	if len(rawList) != 3 {
		t.Fatalf("len = %d, want 3", len(rawList))
	}
}

func TestNewUUID(t *testing.T) {
	id := newUUID()
	if len(id) != 36 {
		t.Fatalf("UUID length = %d, want 36", len(id))
	}
	id2 := newUUID()
	if id == id2 {
		t.Fatal("UUIDs should be unique")
	}
}

func TestResultHTMLSorted(t *testing.T) {
	html := resultHTML(true)
	if !strings.Contains(html, "yes") {
		t.Fatal("sorted result should contain 'yes' class")
	}
	if !strings.Contains(html, "sorted") {
		t.Fatal("sorted result should contain 'sorted'")
	}
}

func TestResultHTMLNotSorted(t *testing.T) {
	html := resultHTML(false)
	if !strings.Contains(html, "no") {
		t.Fatal("not sorted result should contain 'no' class")
	}
}

func TestHtmlEscape(t *testing.T) {
	got := htmlEscape(`<script>alert("xss")</script>`)
	if strings.Contains(got, "<script>") {
		t.Fatal("HTML should be escaped")
	}
}

func TestSSEHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	sendSSEStatus(w, w, "queued")
	if w.Header().Get("Content-Type") != "" {
		// Headers are set by the sseHandler, not sendSSEStatus
	}
	body := w.Body.String()
	if !strings.Contains(body, "event: status") {
		t.Fatal("expected SSE status event")
	}
	if !strings.Contains(body, "queued") {
		t.Fatal("expected queued status")
	}
}
