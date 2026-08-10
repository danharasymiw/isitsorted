package main

import (
	"net"
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

// Limiter is an in-memory, per-IP token-bucket rate limiter. It replaces the
// Redis-backed limiter now that the gateway no longer talks to Redis
// directly; each gateway instance enforces its own limit independently.
type Limiter struct {
	ips   map[string]*rate.Limiter
	mu    sync.Mutex
	limit rate.Limit
	burst int
}

func NewLimiter(requestsPerMinute int) *Limiter {
	return &Limiter{
		ips:   make(map[string]*rate.Limiter),
		limit: rate.Limit(float64(requestsPerMinute) / 60.0),
		burst: requestsPerMinute,
	}
}

func (l *Limiter) getLimiter(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	limiter, exists := l.ips[ip]
	if !exists {
		limiter = rate.NewLimiter(l.limit, l.burst)
		l.ips[ip] = limiter
	}
	return limiter
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
			w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
