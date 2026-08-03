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
