package main

import (
	"embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

//go:embed static
var testStaticFS embed.FS

// newTestGateway builds a Gateway wired to a fake job service so handler
// tests don't need a real job service running.
func newTestGateway(jobService http.Handler) (*Gateway, *httptest.Server) {
	srv := httptest.NewServer(jobService)
	g := &Gateway{
		client:   NewJobClient(srv.URL),
		limiter:  NewLimiter(1000),
		staticFS: testStaticFS,
	}
	return g, srv
}

func TestSubmitHandlerJSON(t *testing.T) {
	fake := http.NewServeMux()
	fake.HandleFunc("POST /jobs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"abc-123"}`))
	})
	g, srv := newTestGateway(fake)
	defer srv.Close()

	body := `{"list": ["1", "2", "3"], "order": "asc"}`
	r := httptest.NewRequest("POST", "/is-sorted", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	g.submitHandler(w, r)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "abc-123") {
		t.Fatalf("expected id in response, got %s", w.Body.String())
	}
}

func TestSubmitHandlerForm(t *testing.T) {
	fake := http.NewServeMux()
	fake.HandleFunc("POST /jobs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"form-id-123"}`))
	})
	g, srv := newTestGateway(fake)
	defer srv.Close()

	body := "list=1%0A2%0A3&order=asc"
	r := httptest.NewRequest("POST", "/is-sorted", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	g.submitHandler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "sse-connect") {
		t.Fatalf("expected SSE scaffold in response, got %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "form-id-123") {
		t.Fatalf("expected job id in response, got %s", w.Body.String())
	}
}

func TestSubmitHandlerFormMissingList(t *testing.T) {
	g, srv := newTestGateway(http.NewServeMux())
	defer srv.Close()

	body := "order=asc"
	r := httptest.NewRequest("POST", "/is-sorted", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	g.submitHandler(w, r)

	if !strings.Contains(w.Body.String(), "list is required") {
		t.Fatalf("expected 'list is required' error, got %s", w.Body.String())
	}
}

func TestSubmitHandlerJobServiceError(t *testing.T) {
	fake := http.NewServeMux()
	fake.HandleFunc("POST /jobs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"list is required"}`))
	})
	g, srv := newTestGateway(fake)
	defer srv.Close()

	body := `{"list": [], "order": "asc"}`
	r := httptest.NewRequest("POST", "/is-sorted", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	g.submitHandler(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestStatusHandlerProxies(t *testing.T) {
	fake := http.NewServeMux()
	fake.HandleFunc("GET /jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"abc","status":"queued"}`))
	})
	g, srv := newTestGateway(fake)
	defer srv.Close()

	r := httptest.NewRequest("GET", "/is-sorted/abc", nil)
	r.SetPathValue("id", "abc")
	w := httptest.NewRecorder()

	g.statusHandler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "queued") {
		t.Fatalf("expected proxied body, got %s", w.Body.String())
	}
}

func TestStatusHandlerJobServiceUnavailable(t *testing.T) {
	g := &Gateway{client: NewJobClient("http://127.0.0.1:0"), limiter: NewLimiter(1000)}

	r := httptest.NewRequest("GET", "/is-sorted/abc", nil)
	r.SetPathValue("id", "abc")
	w := httptest.NewRecorder()

	g.statusHandler(w, r)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadGateway)
	}
}

func TestUploadHandlerProxies(t *testing.T) {
	fake := http.NewServeMux()
	fake.HandleFunc("POST /uploads", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"u1","upload_url":"http://example.com/put"}`))
	})
	g, srv := newTestGateway(fake)
	defer srv.Close()

	r := httptest.NewRequest("GET", "/upload", nil)
	w := httptest.NewRecorder()

	g.uploadHandler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "upload_url") {
		t.Fatalf("expected upload_url in response, got %s", w.Body.String())
	}
}

func TestUploadCheckHandlerDefaultsOrder(t *testing.T) {
	var gotOrder string
	fake := http.NewServeMux()
	fake.HandleFunc("POST /uploads/{id}/check", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Order string `json:"order"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotOrder = body.Order
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"u1"}`))
	})
	g, srv := newTestGateway(fake)
	defer srv.Close()

	r := httptest.NewRequest("POST", "/upload/u1/check", strings.NewReader(`{}`))
	r.SetPathValue("id", "u1")
	w := httptest.NewRecorder()

	g.uploadCheckHandler(w, r)

	if gotOrder != "asc" {
		t.Fatalf("order = %q, want asc", gotOrder)
	}
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestCountHandlerRendersHTML(t *testing.T) {
	fake := http.NewServeMux()
	fake.HandleFunc("GET /stats/count", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"total":1234,"sorted":600,"not_sorted":634}`))
	})
	g, srv := newTestGateway(fake)
	defer srv.Close()

	r := httptest.NewRequest("GET", "/count", nil)
	w := httptest.NewRecorder()

	g.countHandler(w, r)

	if !strings.Contains(w.Body.String(), "1,234") {
		t.Fatalf("expected formatted count, got %s", w.Body.String())
	}
}

func TestCountHandlerFallsBackOnError(t *testing.T) {
	g := &Gateway{client: NewJobClient("http://127.0.0.1:0"), limiter: NewLimiter(1000)}

	r := httptest.NewRequest("GET", "/count", nil)
	w := httptest.NewRecorder()

	g.countHandler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "0") {
		t.Fatalf("expected zeroed counts on failure, got %s", w.Body.String())
	}
}

func TestActivityHandlerEmptyFallback(t *testing.T) {
	g := &Gateway{client: NewJobClient("http://127.0.0.1:0"), limiter: NewLimiter(1000)}

	r := httptest.NewRequest("GET", "/activity", nil)
	w := httptest.NewRecorder()

	g.activityHandler(w, r)

	if !strings.Contains(w.Body.String(), "No checks yet") {
		t.Fatalf("expected empty-state HTML, got %s", w.Body.String())
	}
}

func TestHtmlEscape(t *testing.T) {
	got := htmlEscape(`<script>alert("xss")</script>`)
	if strings.Contains(got, "<script>") {
		t.Fatal("HTML should be escaped")
	}
}
