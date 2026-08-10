package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLimiterAllowsUnderLimit(t *testing.T) {
	l := NewLimiter(3)
	for i := 0; i < 3; i++ {
		if !l.Allow("1.2.3.4") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
}

func TestLimiterBlocksOverLimit(t *testing.T) {
	l := NewLimiter(2)
	l.Allow("1.2.3.4")
	l.Allow("1.2.3.4")
	if l.Allow("1.2.3.4") {
		t.Fatal("3rd request should be blocked")
	}
}

func TestLimiterDifferentIPsIndependent(t *testing.T) {
	l := NewLimiter(1)
	ok1 := l.Allow("1.1.1.1")
	ok2 := l.Allow("2.2.2.2")
	if !ok1 || !ok2 {
		t.Fatal("different IPs should be independent")
	}
}

func TestLimiterMiddlewareAllows(t *testing.T) {
	l := NewLimiter(10)
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := l.Middleware(inner)
	r := httptest.NewRequest("POST", "/is-sorted", nil)
	r.RemoteAddr = "1.2.3.4:12345"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if !called {
		t.Fatal("inner handler should have been called")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestLimiterMiddlewareBlocks(t *testing.T) {
	l := NewLimiter(1)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := l.Middleware(inner)

	// First request — allowed
	r1 := httptest.NewRequest("POST", "/is-sorted", nil)
	r1.RemoteAddr = "1.2.3.4:12345"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, r1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first request: status = %d, want 200", w1.Code)
	}

	// Second request — blocked
	r2 := httptest.NewRequest("POST", "/is-sorted", nil)
	r2.RemoteAddr = "1.2.3.4:12345"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, r2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: status = %d, want 429", w2.Code)
	}
	if !strings.Contains(w2.Body.String(), "rate limit exceeded") {
		t.Fatalf("expected rate limit error body, got %s", w2.Body.String())
	}
}

func TestLimiterMiddlewareFallsBackOnMissingPort(t *testing.T) {
	l := NewLimiter(10)
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := l.Middleware(inner)
	r := httptest.NewRequest("POST", "/is-sorted", nil)
	r.RemoteAddr = "1.2.3.4" // no port
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)
	if !called {
		t.Fatal("inner handler should have been called for addr without port")
	}
}
