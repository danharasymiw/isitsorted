# UI Redesign & Parody Landing Page Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign the "Is It Sorted" page as a parody SaaS landing page with a file-backed check counter.

**Architecture:** Three independent changes ship in sequence: (1) a new `counter.go` module with atomic persistence, (2) wiring the counter into the existing handlers and registering `GET /count`, (3) a full rewrite of `static/index.html`. The counter path is configurable via `DATA_DIR` env var — on Railway, point this at a mounted volume.

**Tech Stack:** Go 1.25, net/http, htmx 2.0 (CDN), inline CSS — no new dependencies.

## Global Constraints

- No external CSS frameworks or font imports
- No new Go module dependencies
- All copy is exactly as specified in the spec — do not paraphrase
- Domain in all copy: `isitsorted.ca`
- `DATA_DIR` env var controls counter file location; defaults to `./data`
- On Railway: set `DATA_DIR=/data` and mount a volume at `/data` for persistence

---

## File Map

| File | Status | Responsibility |
|------|--------|---------------|
| `counter.go` | **Create** | Counter struct, file persistence, `GET /count` handler |
| `counter_test.go` | **Create** | Tests for counter load, increment, persist, HTTP handler |
| `handler.go` | **Modify** | Convert handlers to factories accepting `*counter`; update HTML fragments; call `ctr.increment()` |
| `handler_test.go` | **Modify** | Update `newServer` calls to pass a `*counter` |
| `main.go` | **Modify** | Load counter at startup; pass to `newServer`; register `GET /count` |
| `static/index.html` | **Rewrite** | Full parody landing page |
| `.gitignore` | **Modify** | Add `data/` |

---

## Task 1: File-backed counter module

**Files:**
- Create: `counter.go`
- Create: `counter_test.go`

**Interfaces:**
- Produces:
  - `newCounter(path string) *counter` — loads from disk or starts at 0
  - `(*counter).increment()` — increments and persists
  - `(*counter).value() int64` — returns current count
  - `countHandler(ctr *counter) http.HandlerFunc` — `GET /count`, returns formatted number as HTML fragment
  - `formatCount(n int64) string` — formats int with commas, e.g. `1234567` → `"1,234,567"`

- [ ] **Step 1: Write the failing tests**

Create `counter_test.go`:

```go
package main

import (
	"net/http/httptest"
	"os"
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
	c.increment()
	c.increment()
	if c.value() != 2 {
		t.Fatalf("want 2, got %d", c.value())
	}
}

func TestCounter_PersistsAcrossRestarts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "count.json")
	c1 := newCounter(path)
	c1.increment()
	c1.increment()
	c1.increment()

	c2 := newCounter(path)
	if c2.value() != 3 {
		t.Fatalf("want 3 after reload, got %d", c2.value())
	}
}

func TestCountHandler(t *testing.T) {
	c := newCounter(filepath.Join(t.TempDir(), "count.json"))
	c.increment()
	c.increment()

	req := httptest.NewRequest("GET", "/count", nil)
	w := httptest.NewRecorder()
	countHandler(c)(w, req)

	if w.Code != 200 {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if w.Body.String() != "2" {
		t.Fatalf("want body \"2\", got %q", w.Body.String())
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
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./... -run "TestNewCounter|TestCounter|TestCountHandler|TestFormatCount" -v
```

Expected: compile errors — `counter`, `newCounter`, `countHandler`, `formatCount` not defined.

- [ ] **Step 3: Implement `counter.go`**

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

type counter struct {
	mu   sync.Mutex
	n    int64
	path string
}

type counterFile struct {
	Count int64 `json:"count"`
}

func newCounter(path string) *counter {
	c := &counter{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("counter: load: %v", err)
		}
		return c
	}
	var f counterFile
	if err := json.Unmarshal(data, &f); err != nil {
		log.Printf("counter: parse: %v", err)
		return c
	}
	c.n = f.Count
	return c
}

func (c *counter) increment() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
	c.save()
}

func (c *counter) value() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func (c *counter) save() {
	if c.path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0755); err != nil {
		log.Printf("counter: mkdir: %v", err)
		return
	}
	data, _ := json.Marshal(counterFile{Count: c.n})
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		log.Printf("counter: write: %v", err)
		return
	}
	if err := os.Rename(tmp, c.path); err != nil {
		log.Printf("counter: rename: %v", err)
	}
}

func countHandler(ctr *counter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, formatCount(ctr.value()))
	}
}

func formatCount(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	result := make([]byte, 0, len(s)+len(s)/3)
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(ch))
	}
	return string(result)
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./... -run "TestNewCounter|TestCounter|TestCountHandler|TestFormatCount" -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add counter.go counter_test.go
git commit -m "feat: file-backed check counter"
```

---

## Task 2: Wire counter into server + update handlers

**Files:**
- Modify: `main.go`
- Modify: `handler.go`
- Modify: `handler_test.go`
- Modify: `.gitignore`

**Interfaces:**
- Consumes: `newCounter`, `countHandler` from `counter.go`
- Produces:
  - `newServer(rl *rateLimiter, ctr *counter) http.Handler` — updated signature
  - `isSortedHandler(ctr *counter) http.HandlerFunc` — factory
  - `checkFormHandler(ctr *counter) http.HandlerFunc` — factory, updated HTML fragments

- [ ] **Step 1: Add `data/` to .gitignore**

```bash
echo "data/" >> .gitignore
```

- [ ] **Step 2: Update `main.go`**

Replace the full contents of `main.go`:

```go
package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

//go:embed static
var staticFS embed.FS

func newServer(rl *rateLimiter, ctr *counter) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /is-sorted", rl.middleware(isSortedHandler(ctr)))
	mux.Handle("POST /check", rl.middleware(checkFormHandler(ctr)))
	mux.Handle("GET /count", countHandler(ctr))
	sub, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /", http.FileServer(http.FS(sub)))
	return mux
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}
	ctr := newCounter(filepath.Join(dataDir, "count.json"))
	rl := newRateLimiter(20, time.Minute)
	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, newServer(rl, ctr)))
}
```

- [ ] **Step 3: Update `handler.go` — convert to factories + new HTML fragments**

Replace the full contents of `handler.go`:

```go
package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type sortRequest struct {
	List  []int  `json:"list"`
	Order string `json:"order"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func isSortedHandler(ctr *counter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		var req sortRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if req.List == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "list is required"})
			return
		}
		if req.Order == "" {
			req.Order = "asc"
		}
		if req.Order != "asc" && req.Order != "desc" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": `order must be "asc" or "desc"`})
			return
		}
		ctr.increment()
		writeJSON(w, http.StatusOK, map[string]bool{"sorted": check(req.List, req.Order)})
	}
}

func checkFormHandler(ctr *counter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		order := r.FormValue("order")
		if order == "" {
			order = "asc"
		}
		if order != "asc" && order != "desc" {
			http.Error(w, "invalid order", http.StatusBadRequest)
			return
		}

		raw := r.FormValue("list")
		parts := strings.FieldsFunc(raw, func(c rune) bool {
			return c == ',' || c == '\n' || c == '\r'
		})

		var list []int
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			n, err := strconv.Atoi(p)
			if err != nil {
				w.Header().Set("Content-Type", "text/html")
				w.Write([]byte(`<div class="result-card error"><span class="result-icon">!</span><div><strong>Invalid input</strong><p>All values must be integers.</p></div></div>`))
				return
			}
			list = append(list, n)
		}

		ctr.increment()
		w.Header().Set("Content-Type", "text/html")
		if check(list, order) {
			w.Write([]byte(`<div class="result-card yes"><span class="result-icon">✓</span><div><strong>Yes, it&#39;s sorted</strong></div></div>`))
		} else {
			w.Write([]byte(`<div class="result-card no"><span class="result-icon">✗</span><div><strong>No, it&#39;s not sorted</strong></div></div>`))
		}
	}
}
```

- [ ] **Step 4: Update `handler_test.go` — fix `newServer` calls**

Every call to `newServer(rl)` becomes `newServer(rl, &counter{})`. There are two locations — the main test server and the strict rate-limit server.

Replace the full contents of `handler_test.go`:

```go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIsSortedHandler(t *testing.T) {
	rl := newRateLimiter(100, time.Minute)
	srv := httptest.NewServer(newServer(rl, &counter{}))
	defer srv.Close()

	post := func(body string) *http.Response {
		resp, err := http.Post(srv.URL+"/is-sorted", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	decodeBool := func(resp *http.Response, key string) bool {
		var got map[string]bool
		json.NewDecoder(resp.Body).Decode(&got)
		return got[key]
	}

	t.Run("sorted ascending", func(t *testing.T) {
		resp := post(`{"list":[1,2,3],"order":"asc"}`)
		if resp.StatusCode != 200 {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		if !decodeBool(resp, "sorted") {
			t.Error("want sorted=true")
		}
	})

	t.Run("unsorted ascending", func(t *testing.T) {
		resp := post(`{"list":[1,3,2],"order":"asc"}`)
		if decodeBool(resp, "sorted") {
			t.Error("want sorted=false")
		}
	})

	t.Run("sorted descending", func(t *testing.T) {
		resp := post(`{"list":[3,2,1],"order":"desc"}`)
		if !decodeBool(resp, "sorted") {
			t.Error("want sorted=true")
		}
	})

	t.Run("default order is asc", func(t *testing.T) {
		resp := post(`{"list":[1,2,3]}`)
		if !decodeBool(resp, "sorted") {
			t.Error("want sorted=true with default asc")
		}
	})

	t.Run("invalid order returns 400", func(t *testing.T) {
		resp := post(`{"list":[1,2,3],"order":"sideways"}`)
		if resp.StatusCode != 400 {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
	})

	t.Run("missing list returns 400", func(t *testing.T) {
		resp := post(`{"order":"asc"}`)
		if resp.StatusCode != 400 {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
	})

	t.Run("malformed JSON returns 400", func(t *testing.T) {
		resp := post(`not json`)
		if resp.StatusCode != 400 {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
	})

	t.Run("non-integer in list returns 400", func(t *testing.T) {
		resp := post(`{"list":[1,"two",3],"order":"asc"}`)
		if resp.StatusCode != 400 {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
	})

	t.Run("body exceeding 1MB returns 400", func(t *testing.T) {
		big := `{"list":[` + strings.Repeat("1,", 600000) + `0],"order":"asc"}`
		resp, err := http.Post(srv.URL+"/is-sorted", "application/json", strings.NewReader(big))
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 400 {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
	})

	t.Run("rate limit returns 429", func(t *testing.T) {
		strictRL := newRateLimiter(2, time.Minute)
		strictSrv := httptest.NewServer(newServer(strictRL, &counter{}))
		defer strictSrv.Close()
		for i := 0; i < 2; i++ {
			http.Post(strictSrv.URL+"/is-sorted", "application/json", strings.NewReader(`{"list":[1,2]}`))
		}
		resp, _ := http.Post(strictSrv.URL+"/is-sorted", "application/json", strings.NewReader(`{"list":[1,2]}`))
		if resp.StatusCode != 429 {
			t.Fatalf("want 429, got %d", resp.StatusCode)
		}
	})
}
```

- [ ] **Step 5: Run all tests**

```bash
go test ./... -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add main.go handler.go handler_test.go .gitignore
git commit -m "feat: wire counter into handlers and server"
```

---

## Task 3: Rewrite static/index.html

**Files:**
- Modify: `static/index.html`

**Interfaces:**
- Consumes: `GET /count` (HTMX load trigger), `POST /check` (HTMX form)

- [ ] **Step 1: Replace `static/index.html`**

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Is It Sorted — IISaaS</title>
  <script src="https://unpkg.com/htmx.org@2.0.0/dist/htmx.min.js"></script>
  <style>
    *, *::before, *::after { box-sizing: border-box; }
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #f1f5f9; margin: 0; padding: 0; color: #0f172a; }
    .page { max-width: 560px; margin: 0 auto; }
    section { padding: 32px 24px; border-bottom: 1px solid #e2e8f0; }
    .overline { font-size: 10px; color: #94a3b8; text-transform: uppercase; letter-spacing: 1px; font-weight: 600; margin: 0 0 6px; }
    h1 { margin: 0 0 8px; }
    h2 { font-size: 18px; font-weight: 700; letter-spacing: -0.5px; margin: 0 0 4px; }
    .btn-primary { background: #4f46e5; color: #fff; border: none; border-radius: 8px; padding: 10px 22px; font-size: 13px; font-weight: 600; cursor: pointer; }
    .btn-primary:hover { background: #4338ca; }
    .btn-secondary { background: #fff; color: #334155; border: 1.5px solid #e2e8f0; border-radius: 8px; padding: 10px 22px; font-size: 13px; font-weight: 600; cursor: pointer; }

    /* hero */
    .hero { text-align: center; padding: 48px 24px 36px; }
    .hero .brand { font-size: 10px; color: #4f46e5; text-transform: uppercase; letter-spacing: 2px; font-weight: 600; margin: 0 0 8px; }
    .hero h1 { font-size: 40px; font-weight: 900; letter-spacing: -2px; line-height: 1; }
    .hero .tagline { font-size: 13px; font-weight: 600; color: #475569; margin: 0 0 10px; }
    .hero .description { font-size: 14px; color: #64748b; max-width: 340px; margin: 0 auto 6px; line-height: 1.6; }
    .hero .caveat { font-size: 12px; color: #94a3b8; font-style: italic; margin: 0 0 20px; }
    .hero-actions { display: flex; gap: 8px; justify-content: center; flex-wrap: wrap; }

    /* stats */
    .stats { display: flex; justify-content: space-around; background: #fff; flex-wrap: wrap; gap: 12px; }
    .stat { text-align: center; }
    .stat-value { font-size: 20px; font-weight: 800; color: #0f172a; }
    .stat-value.live { color: #4f46e5; }
    .stat-label { font-size: 10px; color: #94a3b8; text-transform: uppercase; letter-spacing: 0.5px; margin-top: 2px; }

    /* features */
    .features { background: #fff; text-align: center; }
    .features ul { list-style: none; margin: 0 auto; padding: 0; display: inline-block; text-align: left; line-height: 2.2; font-size: 13px; color: #334155; }
    .features .check { color: #16a34a; font-weight: 700; }

    /* form */
    textarea { width: 100%; font-size: 1rem; padding: 10px; border: 1.5px solid #e2e8f0; border-radius: 10px; background: #fff; box-shadow: 0 1px 4px rgba(15,23,42,0.06); resize: vertical; font-family: inherit; }
    textarea:focus { outline: none; border-color: #4f46e5; }
    .controls { display: flex; align-items: center; gap: 16px; margin-top: 12px; }
    .radio-label { display: flex; align-items: center; gap: 6px; font-size: 13px; color: #334155; cursor: pointer; }
    .radio-label input[type="radio"] { display: none; }
    .radio-dot { width: 14px; height: 14px; border-radius: 50%; border: 2px solid #cbd5e1; display: inline-block; flex-shrink: 0; }
    .radio-label input[type="radio"]:checked + .radio-dot { background: #4f46e5; border-color: #4f46e5; }

    /* result cards */
    .result-card { display: flex; align-items: center; gap: 12px; border-radius: 10px; padding: 14px 16px; border: 1.5px solid; margin-top: 16px; }
    .result-card.yes { background: #f0fdf4; border-color: #bbf7d0; }
    .result-card.no { background: #fef2f2; border-color: #fecaca; }
    .result-card.error { background: #fef2f2; border-color: #fecaca; }
    .result-icon { font-size: 20px; line-height: 1; }
    .result-card.yes strong { color: #15803d; font-size: 15px; }
    .result-card.no strong { color: #dc2626; font-size: 15px; }
    .result-card.error strong { color: #dc2626; font-size: 15px; }
    .result-card p { margin: 4px 0 0; font-size: 13px; color: #64748b; }

    /* testimonials */
    .testimonials-list { display: flex; flex-direction: column; gap: 12px; margin-top: 16px; }
    .testimonial-card { background: #fff; border: 1.5px solid #e2e8f0; border-radius: 10px; padding: 14px 16px; }
    .testimonial-card p { font-size: 13px; color: #334155; margin: 0 0 10px; line-height: 1.6; }
    .testimonial-author { font-size: 11px; font-weight: 600; color: #0f172a; }
    .testimonial-author span { color: #94a3b8; font-weight: 400; }

    /* faq */
    .faq-list { display: flex; flex-direction: column; gap: 16px; margin-top: 20px; }
    .faq-item dt { font-size: 13px; font-weight: 700; color: #0f172a; margin-bottom: 4px; }
    .faq-item dd { font-size: 13px; color: #64748b; margin: 0; line-height: 1.6; }

    /* api docs */
    .section-label { font-size: 11px; font-weight: 600; color: #475569; text-transform: uppercase; letter-spacing: 0.5px; margin: 0 0 6px; }
    .code-block { background: #0f172a; border-radius: 8px; padding: 12px 14px; font-family: monospace; font-size: 11px; line-height: 1.8; margin-bottom: 16px; overflow-x: auto; }
    .code-block .dim { color: #64748b; }
    .code-block .key { color: #7dd3fc; }
    .code-block .str { color: #86efac; }
    .code-block .num { color: #fbbf24; }
    .code-block .cmd { color: #c084fc; }
    .code-block .base { color: #e2e8f0; }
    code { background: #f1f5f9; border-radius: 3px; padding: 1px 4px; font-size: 11px; font-family: monospace; }

    /* footer */
    .footer { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px; font-size: 11px; }
    .footer .left { color: #94a3b8; }
    .footer .right { color: #cbd5e1; }
  </style>
</head>
<body>
  <div class="page">

    <!-- Hero -->
    <section class="hero">
      <p class="brand">isitsorted.ca</p>
      <h1>Is It Sorted</h1>
      <p class="tagline">Is It Sorted as a Service. (IISaaS)</p>
      <p class="description">An API that tells you if a list of integers is sorted.</p>
      <p class="caveat">It does not sort the list.</p>
      <div class="hero-actions">
        <a href="#demo"><button class="btn-primary">Try it free &rarr;</button></a>
        <a href="#api"><button class="btn-secondary">View API docs</button></a>
      </div>
    </section>

    <!-- Stats -->
    <section class="stats">
      <div class="stat">
        <div class="stat-value">Mostly</div>
        <div class="stat-label">Uptime</div>
      </div>
      <div class="stat">
        <div class="stat-value">&lt; 2ms</div>
        <div class="stat-label">p99 latency</div>
      </div>
      <div class="stat">
        <div class="stat-value live" hx-get="/count" hx-trigger="load">&mdash;</div>
        <div class="stat-label">Lists checked</div>
      </div>
      <div class="stat">
        <div class="stat-value">1</div>
        <div class="stat-label">Developer trusts us</div>
      </div>
      <div class="stat">
        <div class="stat-value">0</div>
        <div class="stat-label">Data breaches</div>
      </div>
    </section>

    <!-- Features -->
    <section class="features">
      <p class="overline" style="text-align:center;">Features</p>
      <ul>
        <li><span class="check">&#10003;</span> &nbsp;Detects ascending order</li>
        <li><span class="check">&#10003;</span> &nbsp;Detects descending order</li>
        <li><span class="check">&#10003;</span> &nbsp;Returns true or false</li>
        <li><span class="check">&#10003;</span> &nbsp;Handles empty lists</li>
      </ul>
    </section>

    <!-- Form -->
    <section id="demo">
      <p class="overline">Live demo</p>
      <h2>Try it yourself</h2>
      <p style="font-size:13px;color:#64748b;margin:0 0 14px;">Paste integers &mdash; one per line or comma-separated.</p>
      <form hx-post="/check" hx-target="#result" hx-swap="innerHTML">
        <textarea name="list" rows="6" placeholder="1&#10;2&#10;3&#10;4"></textarea>
        <div class="controls">
          <label class="radio-label">
            <input type="radio" name="order" value="asc" checked>
            <span class="radio-dot"></span>
            Ascending
          </label>
          <label class="radio-label">
            <input type="radio" name="order" value="desc">
            <span class="radio-dot"></span>
            Descending
          </label>
          <button type="submit" class="btn-primary" style="margin-left:auto;">Check &rarr;</button>
        </div>
      </form>
      <div id="result"></div>
    </section>

    <!-- Testimonials -->
    <section>
      <p class="overline" style="text-align:center;">What our users say</p>
      <div class="testimonials-list">
        <div class="testimonial-card">
          <p>"JavaScript's <code>.sort()</code> compares everything as strings by default. <code>[10, 1, 2].sort()</code> returns <code>[1, 10, 2]</code>. I know this. I still double check now."</p>
          <div class="testimonial-author">@sk_dev <span>&middot; 4 stars</span></div>
        </div>
        <div class="testimonial-card">
          <p>"not sure why this exists but it works"</p>
          <div class="testimonial-author">anonymous <span>&middot; 5 stars</span></div>
        </div>
        <div class="testimonial-card">
          <p>"correctly identified my unsorted list. not happy about it."</p>
          <div class="testimonial-author">@priya_dev <span>&middot; 3 stars</span></div>
        </div>
      </div>
    </section>

    <!-- FAQ -->
    <section>
      <p class="overline" style="text-align:center;">Frequently asked questions</p>
      <dl class="faq-list">
        <div class="faq-item">
          <dt>Does it sort the list?</dt>
          <dd>No.</dd>
        </div>
        <div class="faq-item">
          <dt>Why would I use this?</dt>
          <dd>You've got a list. You want to know if it's sorted. You're already here.</dd>
        </div>
        <div class="faq-item">
          <dt>Is there a paid plan?</dt>
          <dd>No. 20 requests/minute, free, forever.</dd>
        </div>
        <div class="faq-item">
          <dt>What's the SLA?</dt>
          <dd>It's running on a $6/mo VPS. You can work it out.</dd>
        </div>
      </dl>
    </section>

    <!-- API Docs -->
    <section id="api">
      <p class="overline">JSON API</p>
      <h2>POST /is-sorted</h2>
      <p style="font-size:13px;color:#64748b;margin:0 0 16px;">Rate limited to 20 requests/minute per IP.</p>

      <p class="section-label">Request</p>
      <div class="code-block">
        <span class="dim">POST /is-sorted</span><br>
        <span class="dim">Content-Type: application/json</span><br><br>
        <span class="base">{</span><br>
        &nbsp;&nbsp;<span class="key">"list"</span><span class="base">: [</span><span class="num">1, 3, 2, 4</span><span class="base">],</span><br>
        &nbsp;&nbsp;<span class="key">"order"</span><span class="base">: </span><span class="str">"asc"</span><br>
        <span class="base">}</span>
      </div>

      <p class="section-label">Response</p>
      <div class="code-block">
        <span class="dim">200 OK</span><br><br>
        <span class="base">{</span> <span class="key">"sorted"</span><span class="base">:</span> <span class="str">true</span> <span class="base">}</span>
      </div>

      <p class="section-label">curl</p>
      <div class="code-block">
        <span class="cmd">curl</span> <span class="str">-X POST https://isitsorted.ca/is-sorted</span> \<br>
        &nbsp;&nbsp;<span class="str">-H "Content-Type: application/json"</span> \<br>
        &nbsp;&nbsp;<span class="str">-d '{"list":[1,2,3],"order":"asc"}'</span>
      </div>
    </section>

    <!-- Footer -->
    <section class="footer">
      <span class="left">&copy; 2026 isitsorted.ca &mdash; All rights reserved. Patent pending.</span>
      <span class="right">Built on a $6/mo VPS.</span>
    </section>

  </div>
</body>
</html>
```

- [ ] **Step 2: Start the server and verify the page visually**

```bash
go run . &
open http://localhost:8080
```

Check:
- Hero renders: "Is It Sorted" headline, tagline, two buttons
- Stats bar shows `—` for lists checked (HTMX loads it as `0`)
- Features list: green checkmarks, centered
- Form works: submit a list, result card appears
- Testimonials and FAQ render correctly
- API docs: dark code blocks visible
- Footer: two columns

- [ ] **Step 3: Kill the dev server and run all tests**

```bash
kill %1
go test ./...
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add static/index.html
git commit -m "feat: parody landing page redesign"
```
