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

func TestSSEHandlerJSONPassthrough(t *testing.T) {
	fake := http.NewServeMux()
	fake.HandleFunc("GET /jobs/{id}/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		f := w.(http.Flusher)
		fmt.Fprintf(w, "event: status\ndata: {\"status\":\"queued\"}\n\n")
		f.Flush()
		fmt.Fprintf(w, "event: result\ndata: {\"status\":\"done\",\"sorted\":true}\n\n")
		fmt.Fprintf(w, "event: close\ndata: \n\n")
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
