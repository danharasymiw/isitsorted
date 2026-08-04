package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestIsSortedHandler(t *testing.T) {
	rl := newRateLimiter(100, time.Minute)
	srv := httptest.NewServer(newServer(rl, &counter{}, newActivityLog(20, "")))
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

	t.Run("garbage string in list returns 400", func(t *testing.T) {
		resp := post(`{"list":[1,"banana",3],"order":"asc"}`)
		if resp.StatusCode != 400 {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
	})

	// New: word numbers are valid
	t.Run("word numbers accepted", func(t *testing.T) {
		resp := post(`{"list":["one","two","three"],"order":"asc"}`)
		if resp.StatusCode != 200 {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		if !decodeBool(resp, "sorted") {
			t.Error("want sorted=true")
		}
	})

	// New: floats
	t.Run("floats sorted", func(t *testing.T) {
		resp := post(`{"list":[1.1, 2.2, 3.3],"order":"asc"}`)
		if resp.StatusCode != 200 {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		if !decodeBool(resp, "sorted") {
			t.Error("want sorted=true")
		}
	})

	// New: big integers
	t.Run("big ints sorted", func(t *testing.T) {
		resp := post(`{"list":[99999999999999999998, 99999999999999999999, 100000000000000000000],"order":"asc"}`)
		if resp.StatusCode != 200 {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		if !decodeBool(resp, "sorted") {
			t.Error("want sorted=true")
		}
	})

	// New: mixed types
	t.Run("mixed types sorted", func(t *testing.T) {
		resp := post(`{"list":["one", 2, 3.5, "four"],"order":"asc"}`)
		if resp.StatusCode != 200 {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		if !decodeBool(resp, "sorted") {
			t.Error("want sorted=true")
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
		strictSrv := httptest.NewServer(newServer(strictRL, &counter{}, newActivityLog(20, "")))
		defer strictSrv.Close()
		for i := 0; i < 2; i++ {
			http.Post(strictSrv.URL+"/is-sorted", "application/json", strings.NewReader(`{"list":[1,2]}`))
		}
		resp, _ := http.Post(strictSrv.URL+"/is-sorted", "application/json", strings.NewReader(`{"list":[1,2]}`))
		if resp.StatusCode != 429 {
			t.Fatalf("want 429, got %d", resp.StatusCode)
		}
	})

	t.Run("uncertainty string ascending sorted", func(t *testing.T) {
		resp := post(`{"list":["7","10±2","15"],"order":"asc"}`)
		if resp.StatusCode != 200 {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		if !decodeBool(resp, "sorted") {
			t.Error("want sorted=true")
		}
	})

	t.Run("uncertainty string not sorted", func(t *testing.T) {
		resp := post(`{"list":["9","10±2","11"],"order":"asc"}`)
		if decodeBool(resp, "sorted") {
			t.Error("want sorted=false")
		}
	})

	t.Run("uncertainty pick equals neighbor", func(t *testing.T) {
		resp := post(`{"list":["10±1","9"],"order":"asc"}`)
		if resp.StatusCode != 200 {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		if !decodeBool(resp, "sorted") {
			t.Error("want sorted=true (pick 9 from {9,11})")
		}
	})

	t.Run("interval notation sorted", func(t *testing.T) {
		resp := post(`{"list":["1","[5..8]","20"],"order":"asc"}`)
		if resp.StatusCode != 200 {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		if !decodeBool(resp, "sorted") {
			t.Error("want sorted=true")
		}
	})

	t.Run("set notation sorted", func(t *testing.T) {
		resp := post(`{"list":["{1, 3, 5}","10","20"],"order":"asc"}`)
		if resp.StatusCode != 200 {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		if !decodeBool(resp, "sorted") {
			t.Error("want sorted=true")
		}
	})

	t.Run("expression with uncertainty", func(t *testing.T) {
		resp := post(`{"list":["1","(10±1)*2","30"],"order":"asc"}`)
		if resp.StatusCode != 200 {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		if !decodeBool(resp, "sorted") {
			t.Error("want sorted=true for [1, {18..22}, 30]")
		}
	})
}

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
