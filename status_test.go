package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStatusHandler(t *testing.T) {
	h := statusHandler()

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("want 200, got %d", w.Code)
	}

	body := w.Body.String()

	if !strings.Contains(body, "All Systems Operational") {
		t.Error("missing hero text")
	}
	if !strings.Contains(body, "Sort Checker") {
		t.Error("missing Sort Checker component")
	}
	if !strings.Contains(body, "Async API") {
		t.Error("missing Async API component")
	}
	if !strings.Contains(body, "Counter") {
		t.Error("missing Counter component")
	}
	if !strings.Contains(body, "Activity Feed") {
		t.Error("missing Activity Feed component")
	}
	if !strings.Contains(body, "Website") {
		t.Error("missing Website component")
	}
	if !strings.Contains(body, "100%") {
		t.Error("missing uptime percentage")
	}
	if !strings.Contains(body, "No incidents reported") {
		t.Error("missing incident history text")
	}

	// Check that today's date appears in the page (for uptime bar tooltips).
	today := time.Now().Format("Jan 2, 2006")
	if !strings.Contains(body, today) {
		t.Errorf("missing today's date %q in uptime bars", today)
	}

	// Check that 90 days ago also appears.
	ago := time.Now().AddDate(0, 0, -89).Format("Jan 2, 2006")
	if !strings.Contains(body, ago) {
		t.Errorf("missing 90-days-ago date %q in uptime bars", ago)
	}
}

func TestHostRouter(t *testing.T) {
	rl := newRateLimiter(100, time.Minute)
	appH := newServer(rl, &counter{}, newActivityLog(20, ""))
	statusH := http.NewServeMux()
	statusH.Handle("GET /", statusHandler())
	router := hostRouter(statusH, appH)

	t.Run("status host serves status page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Host = "status.isitsorted.ca"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("want 200, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "All Systems Operational") {
			t.Error("status host should serve status page")
		}
	})

	t.Run("main host serves app", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Host = "isitsorted.ca"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("want 200, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "Is It Sorted?") {
			t.Error("main host should serve main app")
		}
	})

	t.Run("localhost serves app", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Host = "localhost:8080"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("want 200, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "Is It Sorted?") {
			t.Error("localhost should serve main app")
		}
	})
}
