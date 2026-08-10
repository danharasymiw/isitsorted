package main

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReemitHTMLStatus(t *testing.T) {
	tests := []struct {
		name       string
		data       string
		wantSubstr string
	}{
		{
			"queued",
			`{"status":"queued"}`,
			"Queued as abcdefgh",
		},
		{
			"processing",
			`{"status":"processing","worker_id":3}`,
			"Processing on Worker #3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &Gateway{client: NewJobClient("http://unused")}
			w := httptest.NewRecorder()
			g.reemitHTML(w, w, "status", tt.data, "abcdefgh-1234-5678")
			body := w.Body.String()
			if !strings.Contains(body, tt.wantSubstr) {
				t.Errorf("reemitHTML status %q: body = %q, want substring %q", tt.name, body, tt.wantSubstr)
			}
			if !strings.Contains(body, "event: status") {
				t.Error("missing SSE event prefix")
			}
		})
	}
}

func TestReemitHTMLResultDone(t *testing.T) {
	g := &Gateway{client: NewJobClient("http://unused")}
	w := httptest.NewRecorder()
	g.reemitHTML(w, w, "result", `{"status":"done","sorted":true}`, "abc")
	body := w.Body.String()
	if !strings.Contains(body, "Yes, it's sorted!") {
		t.Errorf("expected sorted result HTML, got %s", body)
	}
	if !strings.Contains(body, "event: counters") {
		t.Error("expected counters event for refresh")
	}
	if !strings.Contains(body, "event: close") {
		t.Error("expected close event")
	}
}

func TestReemitHTMLResultError(t *testing.T) {
	g := &Gateway{client: NewJobClient("http://unused")}
	w := httptest.NewRecorder()
	g.reemitHTML(w, w, "result", `{"status":"error","error":"bad input"}`, "abc")
	body := w.Body.String()
	if !strings.Contains(body, "bad input") {
		t.Errorf("expected error message, got %s", body)
	}
	if !strings.Contains(body, "result-card error") {
		t.Error("expected error card class")
	}
}

func TestReemitHTMLClose(t *testing.T) {
	g := &Gateway{client: NewJobClient("http://unused")}
	w := httptest.NewRecorder()
	g.reemitHTML(w, w, "close", "", "abc")
	if !strings.Contains(w.Body.String(), "event: close") {
		t.Error("expected close event")
	}
}

func TestReemitHTMLError(t *testing.T) {
	g := &Gateway{client: NewJobClient("http://unused")}
	w := httptest.NewRecorder()
	g.reemitHTML(w, w, "error", `{"error":"something"}`, "abc")
	body := w.Body.String()
	if !strings.Contains(body, "event: error") {
		t.Error("expected error event")
	}
	if !strings.Contains(body, `{"error":"something"}`) {
		t.Errorf("expected error data passthrough, got %s", body)
	}
}

func TestSSEHandlerHTMLMode(t *testing.T) {
	fake := http.NewServeMux()
	fake.HandleFunc("GET /jobs/{id}/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		f := w.(http.Flusher)
		_, _ = fmt.Fprintf(w, "event: status\ndata: {\"status\":\"queued\"}\n\n")
		f.Flush()
		_, _ = fmt.Fprintf(w, "event: result\ndata: {\"status\":\"done\",\"sorted\":false}\n\n")
		_, _ = fmt.Fprintf(w, "event: close\ndata: \n\n")
		f.Flush()
	})
	srv := httptest.NewServer(fake)
	defer srv.Close()

	g := &Gateway{client: NewJobClient(srv.URL), limiter: NewLimiter(1000)}

	req := httptest.NewRequest("GET", "/is-sorted/abcdefgh-1234/events?format=html", nil)
	req.SetPathValue("id", "abcdefgh-1234")
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		g.sseHandler(w, req)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("sseHandler did not return in time")
	}

	body := w.Body.String()
	if !strings.Contains(body, "Queued as abcdefgh") {
		t.Errorf("expected queued HTML, got %s", body)
	}
	if !strings.Contains(body, "Nope, not sorted.") {
		t.Errorf("expected not-sorted result HTML, got %s", body)
	}
}

func TestSSEHandlerJobServiceUnavailable(t *testing.T) {
	g := &Gateway{client: NewJobClient("http://127.0.0.1:0"), limiter: NewLimiter(1000)}

	req := httptest.NewRequest("GET", "/is-sorted/abc/events", nil)
	req.SetPathValue("id", "abc")
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		g.sseHandler(w, req)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("sseHandler did not return in time")
	}

	body := w.Body.String()
	if !strings.Contains(body, "event: error") {
		t.Errorf("expected error event, got %s", body)
	}
	if !strings.Contains(body, "job service unavailable") {
		t.Errorf("expected unavailable message, got %s", body)
	}
}

func TestSSEHandlerJSONPassthrough(t *testing.T) {
	fake := http.NewServeMux()
	fake.HandleFunc("GET /jobs/{id}/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		f := w.(http.Flusher)
		_, _ = fmt.Fprintf(w, "event: status\ndata: {\"status\":\"queued\"}\n\n")
		f.Flush()
		_, _ = fmt.Fprintf(w, "event: result\ndata: {\"status\":\"done\",\"sorted\":true}\n\n")
		_, _ = fmt.Fprintf(w, "event: close\ndata: \n\n")
		f.Flush()
	})
	srv := httptest.NewServer(fake)
	defer srv.Close()

	g := &Gateway{client: NewJobClient(srv.URL), limiter: NewLimiter(1000)}

	req := httptest.NewRequest("GET", "/is-sorted/abcdefgh-1234/events", nil)
	req.SetPathValue("id", "abcdefgh-1234")
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		g.sseHandler(w, req)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("sseHandler did not return in time")
	}

	body := w.Body.String()
	t.Logf("body:\n%s", body)
	scanner := bufio.NewScanner(strings.NewReader(body))
	var events []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			events = append(events, strings.TrimPrefix(line, "event: "))
		}
	}
	want := []string{"status", "result", "close"}
	if len(events) != len(want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("events[%d] = %q, want %q", i, events[i], want[i])
		}
	}
}
