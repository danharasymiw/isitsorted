package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAsyncAPI(t *testing.T) {
	rl := newRateLimiter(100, time.Minute)
	ctr := &counter{}
	act := newActivityLog(20, "")
	srv := httptest.NewServer(newServer(rl, ctr, act))
	defer srv.Close()

	submit := func(body string) *http.Response {
		resp, err := http.Post(srv.URL+"/async/is-sorted", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	poll := func(id string) *http.Response {
		resp, err := http.Get(srv.URL + "/async/is-sorted/" + id)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	t.Run("submit returns 202 with job ID", func(t *testing.T) {
		resp := submit(`{"list":[1,2,3],"order":"asc"}`)
		if resp.StatusCode != 202 {
			t.Fatalf("want 202, got %d", resp.StatusCode)
		}
		var got map[string]string
		json.NewDecoder(resp.Body).Decode(&got)
		if got["id"] == "" {
			t.Error("want non-empty job ID")
		}
	})

	t.Run("poll returns completed result", func(t *testing.T) {
		resp := submit(`{"list":[1,2,3],"order":"asc"}`)
		var sub map[string]string
		json.NewDecoder(resp.Body).Decode(&sub)

		// Give the goroutine a moment to finish.
		time.Sleep(50 * time.Millisecond)

		resp = poll(sub["id"])
		if resp.StatusCode != 200 {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		var got map[string]any
		json.NewDecoder(resp.Body).Decode(&got)
		if got["status"] != "complete" {
			t.Errorf("want status=complete, got %v", got["status"])
		}
		if got["sorted"] != true {
			t.Errorf("want sorted=true, got %v", got["sorted"])
		}
	})

	t.Run("poll unsorted list", func(t *testing.T) {
		resp := submit(`{"list":[3,1,2],"order":"asc"}`)
		var sub map[string]string
		json.NewDecoder(resp.Body).Decode(&sub)

		time.Sleep(50 * time.Millisecond)

		resp = poll(sub["id"])
		var got map[string]any
		json.NewDecoder(resp.Body).Decode(&got)
		if got["status"] != "complete" {
			t.Errorf("want status=complete, got %v", got["status"])
		}
		if got["sorted"] != false {
			t.Errorf("want sorted=false, got %v", got["sorted"])
		}
	})

	t.Run("poll nonexistent ID returns 404", func(t *testing.T) {
		resp := poll("00000000-0000-0000-0000-000000000000")
		if resp.StatusCode != 404 {
			t.Fatalf("want 404, got %d", resp.StatusCode)
		}
	})

	t.Run("submit invalid input returns 400", func(t *testing.T) {
		resp := submit(`not json`)
		if resp.StatusCode != 400 {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
	})

	t.Run("submit missing list returns 400", func(t *testing.T) {
		resp := submit(`{"order":"asc"}`)
		if resp.StatusCode != 400 {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
	})

	t.Run("submit invalid order returns 400", func(t *testing.T) {
		resp := submit(`{"list":[1,2],"order":"sideways"}`)
		if resp.StatusCode != 400 {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
	})

	t.Run("result increments counter", func(t *testing.T) {
		before := ctr.value()
		resp := submit(`{"list":[1,2,3]}`)
		var sub map[string]string
		json.NewDecoder(resp.Body).Decode(&sub)
		time.Sleep(50 * time.Millisecond)

		after := ctr.value()
		if after != before+1 {
			t.Errorf("want counter %d, got %d", before+1, after)
		}
	})

	t.Run("result appears in activity log", func(t *testing.T) {
		before := len(act.recent())
		resp := submit(`{"list":[5,4,3],"order":"desc"}`)
		var sub map[string]string
		json.NewDecoder(resp.Body).Decode(&sub)
		time.Sleep(50 * time.Millisecond)

		after := len(act.recent())
		if after != before+1 {
			t.Errorf("want %d activity entries, got %d", before+1, after)
		}
	})

	t.Run("rate limit returns 429", func(t *testing.T) {
		strictRL := newRateLimiter(2, time.Minute)
		strictSrv := httptest.NewServer(newServer(strictRL, &counter{}, newActivityLog(20, "")))
		defer strictSrv.Close()
		for i := 0; i < 2; i++ {
			http.Post(strictSrv.URL+"/async/is-sorted", "application/json", strings.NewReader(`{"list":[1,2]}`))
		}
		resp, _ := http.Post(strictSrv.URL+"/async/is-sorted", "application/json", strings.NewReader(`{"list":[1,2]}`))
		if resp.StatusCode != 429 {
			t.Fatalf("want 429, got %d", resp.StatusCode)
		}
	})
}
