package main

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ipEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// Limiter is an in-memory, per-IP token-bucket rate limiter. It replaces the
// Redis-backed limiter now that the gateway no longer talks to Redis
// directly; each gateway instance enforces its own limit independently.
// A background goroutine evicts IPs idle for more than 5 minutes.
type Limiter struct {
	ips   map[string]*ipEntry
	mu    sync.Mutex
	limit rate.Limit
	burst int
}

func NewLimiter(requestsPerMinute int) *Limiter {
	l := &Limiter{
		ips:   make(map[string]*ipEntry),
		limit: rate.Limit(float64(requestsPerMinute) / 60.0),
		burst: requestsPerMinute,
	}
	go l.evictLoop()
	return l
}

func (l *Limiter) evictLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		l.mu.Lock()
		cutoff := time.Now().Add(-5 * time.Minute)
		for ip, entry := range l.ips {
			if entry.lastSeen.Before(cutoff) {
				delete(l.ips, ip)
			}
		}
		l.mu.Unlock()
	}
}

func (l *Limiter) getLimiter(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, exists := l.ips[ip]
	if !exists {
		entry = &ipEntry{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.ips[ip] = entry
	}
	entry.lastSeen = time.Now()
	return entry.limiter
}

func (l *Limiter) Allow(ip string) bool {
	return l.getLimiter(ip).Allow()
}

func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if ip == "" {
			ip = r.RemoteAddr
		}
		if !l.Allow(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
