package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHostRouterStatus(t *testing.T) {
	statusCalled := false
	appCalled := false

	statusH := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		statusCalled = true
		w.WriteHeader(http.StatusOK)
	})
	appH := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := hostRouter(statusH, appH)

	r := httptest.NewRequest("GET", "/", nil)
	r.Host = "status.isitsorted.ca"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if !statusCalled {
		t.Fatal("status handler should have been called for status. host")
	}
	if appCalled {
		t.Fatal("app handler should not have been called for status. host")
	}
}

func TestHostRouterApp(t *testing.T) {
	statusCalled := false
	appCalled := false

	statusH := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		statusCalled = true
	})
	appH := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appCalled = true
	})

	handler := hostRouter(statusH, appH)

	r := httptest.NewRequest("GET", "/", nil)
	r.Host = "isitsorted.ca"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if statusCalled {
		t.Fatal("status handler should not have been called for app host")
	}
	if !appCalled {
		t.Fatal("app handler should have been called for app host")
	}
}

func TestHostRouterShortHost(t *testing.T) {
	appCalled := false

	statusH := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("status handler should not be called for short host")
	})
	appH := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appCalled = true
	})

	handler := hostRouter(statusH, appH)

	r := httptest.NewRequest("GET", "/", nil)
	r.Host = "abc"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if !appCalled {
		t.Fatal("app handler should have been called for short host")
	}
}
