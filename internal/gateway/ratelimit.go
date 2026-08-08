package gateway

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "sorted:ratelimit:"

type Limiter struct {
	rdb    *redis.Client
	limit  int64
	window time.Duration
}

func NewLimiter(rdb *redis.Client, limit int, window time.Duration) *Limiter {
	return &Limiter{rdb: rdb, limit: int64(limit), window: window}
}

func (l *Limiter) Allow(ctx context.Context, ip string) (bool, error) {
	key := keyPrefix + ip
	n, err := l.rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if n == 1 {
		l.rdb.Expire(ctx, key, l.window)
	}
	return n <= l.limit, nil
}

func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if ip == "" {
			ip = r.RemoteAddr
		}
		ok, err := l.Allow(r.Context(), ip)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprintf(w, `{"error":"rate limit exceeded"}`)
			return
		}
		next.ServeHTTP(w, r)
	})
}
