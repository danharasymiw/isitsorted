# Unified Handler Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Unify the three POST handlers into a single `POST /is-sorted` that dispatches on Content-Type, and consolidate the three counter endpoints into one.

**Architecture:** Single handler checks Content-Type to decide how to parse input (JSON vs form) and how to format the response (JSON vs HTML). The async endpoint shares JSON parsing logic. One counter endpoint returns all three counts via HTMX OOB swaps.

**Tech Stack:** Go, HTMX

## Global Constraints

- Body size limit remains 1MB (`http.MaxBytesReader` with `1<<20`)
- `check()` function and `check_test.go` are unchanged
- `parser/` package, `activity.go`, `ratelimit.go` are unchanged
- All existing test behaviors must be preserved (same inputs → same outputs)

---

### Task 1: Unified handler — replace `isSortedHandler` and `checkFormHandler`

**Files:**
- Modify: `handler.go` (full rewrite of handler functions, keep `writeJSON`, `parseRaw`, `htmlEscape`)
- Test: `handler_test.go`

**Interfaces:**
- Consumes: `check(list []*parser.Value, order string) bool` from `check.go`, `parser.SplitBracketAware` and `parser.ParseValue` from `parser/`, `counter.increment/value/sortedValue/notSortedValue` from `counter.go`, `activityLog.add/recent` from `activity.go`, `renderActivity` from `activity.go`, `formatCount` from `counter.go`
- Produces: `isSortedHandler(ctr *counter, act *activityLog) http.HandlerFunc` — unified handler used by `main.go`

- [ ] **Step 1: Write the failing test — form posts to `/is-sorted`**

Update `handler_test.go` — change `TestCheckFormHandler` to post to `/is-sorted` instead of `/check`:

```go
func TestCheckFormHandler(t *testing.T) {
	rl := newRateLimiter(100, time.Minute)
	srv := httptest.NewServer(newServer(rl, &counter{}, newActivityLog(20, "")))
	defer srv.Close()

	t.Run("uncertainty via form", func(t *testing.T) {
		resp, err := http.PostForm(srv.URL+"/is-sorted", url.Values{
			"list":  {"7\n10±2\n15"},
			"order": {"asc"},
		})
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Yes") {
			t.Errorf("expected sorted, got: %s", body)
		}
	})

	t.Run("finite set via form", func(t *testing.T) {
		resp, err := http.PostForm(srv.URL+"/is-sorted", url.Values{
			"list":  {"{1, 3, 7}, 10, 15"},
			"order": {"asc"},
		})
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Yes") {
			t.Errorf("expected sorted, got: %s", body)
		}
	})

	t.Run("discrete sets with valid pick sorted", func(t *testing.T) {
		resp, err := http.PostForm(srv.URL+"/is-sorted", url.Values{
			"list":  {"10±2, 10±5"},
			"order": {"asc"},
		})
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Yes") {
			t.Errorf("expected sorted (pick 8,15), got: %s", body)
		}
	})

	t.Run("discrete sets no valid pick", func(t *testing.T) {
		resp, err := http.PostForm(srv.URL+"/is-sorted", url.Values{
			"list":  {"10±1, 9, 10±2, 11"},
			"order": {"asc"},
		})
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "No") {
			t.Errorf("expected not sorted, got: %s", body)
		}
	})
}
```

- [ ] **Step 2: Run tests to verify form tests fail**

Run: `cd /Users/danielharasymiw/workspaces/me/sorted && go test -run TestCheckFormHandler -v`
Expected: FAIL — `/is-sorted` does not accept form-encoded requests yet

- [ ] **Step 3: Rewrite `handler.go` with unified handler**

Replace `isSortedHandler` and `checkFormHandler` with a single `isSortedHandler` that dispatches on Content-Type. Keep `writeJSON`, `parseRaw`, `htmlEscape`, and `sortRequest` (still used by JSON path and async):

```go
package main

import (
	"encoding/json"
	"net/http"
	"sorted/parser"
	"strings"
)

type sortRequest struct {
	List  []json.RawMessage `json:"list"`
	Order string            `json:"order"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func parseRaw(raw json.RawMessage) (*parser.Value, error) {
	s := strings.TrimSpace(string(raw))

	if len(s) >= 2 && s[0] == '"' {
		var unquoted string
		if err := json.Unmarshal(raw, &unquoted); err != nil {
			return nil, err
		}
		return parser.ParseValue(unquoted)
	}

	return parser.ParseValue(s)
}

func isSortedHandler(ctr *counter, act *activityLog) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		ct := r.Header.Get("Content-Type")
		if strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
			handleForm(w, r, ctr, act)
		} else {
			handleJSON(w, r, ctr, act)
		}
	}
}

func handleJSON(w http.ResponseWriter, r *http.Request, ctr *counter, act *activityLog) {
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

	list := make([]*parser.Value, 0, len(req.List))
	rawList := make([]string, 0, len(req.List))
	for _, raw := range req.List {
		v, err := parseRaw(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		list = append(list, v)
		rawList = append(rawList, strings.TrimSpace(string(raw)))
	}

	sorted := check(list, req.Order)
	ctr.increment(sorted)
	act.add(sorted, req.Order, rawList)
	writeJSON(w, http.StatusOK, map[string]bool{"sorted": sorted})
}

func handleForm(w http.ResponseWriter, r *http.Request, ctr *counter, act *activityLog) {
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
	lines := strings.FieldsFunc(raw, func(c rune) bool { return c == '\n' || c == '\r' })
	var tokens []string
	for _, line := range lines {
		fields := parser.SplitBracketAware(line, ',')
		if len(fields) > 0 && strings.TrimSpace(fields[len(fields)-1]) == "" {
			fields = fields[:len(fields)-1]
		}
		tokens = append(tokens, fields...)
	}

	var list []*parser.Value
	var rawList []string
	for _, p := range tokens {
		p = strings.TrimSpace(p)
		if p == "" {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<div class="result-card error"><span class="result-icon">!</span><div><strong>Invalid input</strong><p>Empty value — remove consecutive commas.</p></div></div>`))
			return
		}
		v, err := parser.ParseValue(p)
		if err != nil {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<div class="result-card error"><span class="result-icon">!</span><div><strong>Invalid input</strong><p>Could not parse &#34;` + htmlEscape(p) + `&#34; as a number.</p></div></div>`))
			return
		}
		list = append(list, v)
		rawList = append(rawList, p)
	}

	sorted := check(list, order)
	ctr.increment(sorted)
	act.add(sorted, order, rawList)
	oobCount := `<div id="count-display" hx-swap-oob="innerHTML">` + formatCount(ctr.value()) + `</div>`
	oobSorted := `<div id="sorted-count-display" hx-swap-oob="innerHTML">` + formatCount(ctr.sortedValue()) + `</div>`
	oobNotSorted := `<div id="not-sorted-count-display" hx-swap-oob="innerHTML">` + formatCount(ctr.notSortedValue()) + `</div>`
	oobActivity := `<div id="activity-feed" hx-swap-oob="innerHTML">` + renderActivity(act.recent()) + `</div>`
	w.Header().Set("Content-Type", "text/html")
	if sorted {
		w.Write([]byte(`<div class="result-card yes"><span class="result-icon">✓</span><div><strong>Yes, it&#39;s sorted</strong></div></div>` + oobCount + oobSorted + oobNotSorted + oobActivity))
	} else {
		w.Write([]byte(`<div class="result-card no"><span class="result-icon">✗</span><div><strong>No, it&#39;s not sorted</strong></div></div>` + oobCount + oobSorted + oobNotSorted + oobActivity))
	}
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}
```

- [ ] **Step 4: Update routes in `main.go`**

Remove the `/check` route and update `isSortedHandler` to accept `act`:

```go
func newServer(rl *rateLimiter, ctr *counter, act *activityLog) http.Handler {
	js := newJobStore()
	mux := http.NewServeMux()
	mux.Handle("POST /is-sorted", rl.middleware(isSortedHandler(ctr, act)))
	mux.Handle("GET /count", countHandler(ctr))
	mux.Handle("GET /count/sorted", sortedCountHandler(ctr))
	mux.Handle("GET /count/not-sorted", notSortedCountHandler(ctr))
	mux.Handle("GET /activity", activityHandler(act))
	mux.Handle("POST /async/is-sorted", rl.middleware(asyncSubmitHandler(js, ctr, act)))
	mux.Handle("GET /async/is-sorted/{id}", asyncStatusHandler(js))
	sub, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /", http.FileServer(http.FS(sub)))
	return mux
}
```

- [ ] **Step 5: Update frontend form target**

In `static/index.html`, change `hx-post="/check"` to `hx-post="/is-sorted"`:

```html
<form hx-post="/is-sorted" hx-target="#result" hx-swap="innerHTML">
```

- [ ] **Step 6: Run all tests**

Run: `cd /Users/danielharasymiw/workspaces/me/sorted && go test ./... -v`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add handler.go handler_test.go main.go static/index.html
git commit -m "refactor: unify /is-sorted and /check into single handler with Content-Type dispatch"
```

---

### Task 2: Update async handler to share JSON parsing

**Files:**
- Modify: `async.go:70-118` (replace duplicated JSON parsing with call to shared logic)
- Test: `async_test.go` (no changes expected — same endpoint, same behavior)

**Interfaces:**
- Consumes: `handleJSON`-style parsing from `handler.go` (`sortRequest`, `parseRaw`), `check()` from `check.go`
- Produces: `asyncSubmitHandler(js *jobStore, ctr *counter, act *activityLog) http.HandlerFunc` — same signature as before

- [ ] **Step 1: Extract shared JSON parsing into a helper**

Add to `handler.go` a function that parses the JSON body and returns the parsed list, raw strings, and order. Add `"fmt"` to the import block:

```go
func parseJSONBody(r *http.Request) (list []*parser.Value, rawList []string, order string, err error) {
	var req sortRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, nil, "", err
	}
	if req.List == nil {
		return nil, nil, "", fmt.Errorf("list is required")
	}
	order = req.Order
	if order == "" {
		order = "asc"
	}
	if order != "asc" && order != "desc" {
		return nil, nil, "", fmt.Errorf(`order must be "asc" or "desc"`)
	}

	list = make([]*parser.Value, 0, len(req.List))
	rawList = make([]string, 0, len(req.List))
	for _, raw := range req.List {
		v, err := parseRaw(raw)
		if err != nil {
			return nil, nil, "", err
		}
		list = append(list, v)
		rawList = append(rawList, strings.TrimSpace(string(raw)))
	}
	return list, rawList, order, nil
}
```

Then update `handleJSON` to use it:

```go
func handleJSON(w http.ResponseWriter, r *http.Request, ctr *counter, act *activityLog) {
	list, rawList, order, err := parseJSONBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	sorted := check(list, order)
	ctr.increment(sorted)
	act.add(sorted, order, rawList)
	writeJSON(w, http.StatusOK, map[string]bool{"sorted": sorted})
}
```

- [ ] **Step 2: Rewrite `asyncSubmitHandler` to use `parseJSONBody`**

```go
func asyncSubmitHandler(js *jobStore, ctr *counter, act *activityLog) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		list, rawList, order, err := parseJSONBody(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		j := js.create(newUUID())

		go func() {
			sorted := check(list, order)
			j.mu.Lock()
			j.status = "complete"
			j.sorted = sorted
			j.mu.Unlock()
			ctr.increment(sorted)
			act.add(sorted, order, rawList)
		}()

		writeJSON(w, http.StatusAccepted, map[string]string{"id": j.id})
	}
}
```

- [ ] **Step 3: Run all tests**

Run: `cd /Users/danielharasymiw/workspaces/me/sorted && go test ./... -v`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add handler.go async.go
git commit -m "refactor: extract shared JSON parsing, simplify async handler"
```

---

### Task 3: Consolidate counter endpoints

**Files:**
- Modify: `counter.go:96-115` (replace three handlers with one)
- Modify: `main.go:21-23` (remove two routes)
- Modify: `static/index.html:39-69` (single `hx-get` on total count, remove triggers from sorted/not-sorted)
- Test: `handler_test.go` (existing rate-limit and handler tests exercise `/count` indirectly via OOB swaps — add a direct test)

**Interfaces:**
- Consumes: `counter.value()`, `counter.sortedValue()`, `counter.notSortedValue()`, `formatCount()` — all from `counter.go`
- Produces: `countHandler(ctr *counter) http.HandlerFunc` — returns all three counts

- [ ] **Step 1: Write the failing test**

Add to `handler_test.go`:

```go
func TestCountHandler(t *testing.T) {
	rl := newRateLimiter(100, time.Minute)
	ctr := &counter{}
	srv := httptest.NewServer(newServer(rl, ctr, newActivityLog(20, "")))
	defer srv.Close()

	// Submit one sorted and one unsorted to populate counters.
	http.Post(srv.URL+"/is-sorted", "application/json", strings.NewReader(`{"list":[1,2,3]}`))
	http.Post(srv.URL+"/is-sorted", "application/json", strings.NewReader(`{"list":[3,1,2]}`))

	resp, err := http.Get(srv.URL + "/count")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	s := string(body)

	if !strings.Contains(s, "2") {
		t.Errorf("want total count 2 in response, got: %s", s)
	}
	if !strings.Contains(s, `id="sorted-count-display"`) {
		t.Errorf("want sorted OOB swap in response, got: %s", s)
	}
	if !strings.Contains(s, `id="not-sorted-count-display"`) {
		t.Errorf("want not-sorted OOB swap in response, got: %s", s)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/danielharasymiw/workspaces/me/sorted && go test -run TestCountHandler -v`
Expected: FAIL — current `countHandler` returns only the total count, no OOB swaps

- [ ] **Step 3: Update `countHandler` in `counter.go`**

Replace the three handlers with one:

```go
func countHandler(ctr *counter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, formatCount(ctr.value())+
			`<div id="sorted-count-display" hx-swap-oob="innerHTML">`+formatCount(ctr.sortedValue())+`</div>`+
			`<div id="not-sorted-count-display" hx-swap-oob="innerHTML">`+formatCount(ctr.notSortedValue())+`</div>`)
	}
}
```

Delete `sortedCountHandler` and `notSortedCountHandler`.

- [ ] **Step 4: Remove routes from `main.go`**

```go
func newServer(rl *rateLimiter, ctr *counter, act *activityLog) http.Handler {
	js := newJobStore()
	mux := http.NewServeMux()
	mux.Handle("POST /is-sorted", rl.middleware(isSortedHandler(ctr, act)))
	mux.Handle("GET /count", countHandler(ctr))
	mux.Handle("GET /activity", activityHandler(act))
	mux.Handle("POST /async/is-sorted", rl.middleware(asyncSubmitHandler(js, ctr, act)))
	mux.Handle("GET /async/is-sorted/{id}", asyncStatusHandler(js))
	sub, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /", http.FileServer(http.FS(sub)))
	return mux
}
```

- [ ] **Step 5: Update frontend — single `hx-get` on total count**

In `static/index.html`, replace the three stat divs that have `hx-get` triggers:

```html
        <div class="stat">
          <div
            id="count-display"
            class="stat-value live"
            hx-get="/count"
            hx-trigger="load"
          >
            &mdash;
          </div>
          <div class="stat-label">Lists checked</div>
        </div>
        <div class="stat">
          <div
            id="sorted-count-display"
            class="stat-value live"
          >
            &mdash;
          </div>
          <div class="stat-label">Sorted</div>
        </div>
        <div class="stat">
          <div
            id="not-sorted-count-display"
            class="stat-value live"
          >
            &mdash;
          </div>
          <div class="stat-label">Not sorted</div>
        </div>
```

The sorted and not-sorted divs no longer have `hx-get` or `hx-trigger` — they get updated via OOB swaps from the `/count` response.

- [ ] **Step 6: Run all tests**

Run: `cd /Users/danielharasymiw/workspaces/me/sorted && go test ./... -v`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add counter.go main.go static/index.html handler_test.go
git commit -m "refactor: consolidate three counter endpoints into single /count with OOB swaps"
```
