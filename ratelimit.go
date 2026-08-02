package main

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type ipEntry struct {
	count int
	reset time.Time
}

type rateLimiter struct {
	mu     sync.Mutex
	ips    map[string]*ipEntry
	limit  int
	window time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		ips:    make(map[string]*ipEntry),
		limit:  limit,
		window: window,
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	e, ok := rl.ips[ip]
	if !ok || now.After(e.reset) {
		delete(rl.ips, ip)
		rl.ips[ip] = &ipEntry{count: 1, reset: now.Add(rl.window)}
		return true
	}
	if e.count >= rl.limit {
		return false
	}
	e.count++
	return true
}

func (rl *rateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if !rl.allow(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
