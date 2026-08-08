package gateway

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_URL")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	t.Cleanup(func() { rdb.FlushDB(ctx); rdb.Close() })
	return rdb
}

func TestAllowUnderLimit(t *testing.T) {
	rdb := testRedis(t)
	l := NewLimiter(rdb, 3, time.Minute)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		ok, err := l.Allow(ctx, "1.2.3.4")
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
}

func TestBlockOverLimit(t *testing.T) {
	rdb := testRedis(t)
	l := NewLimiter(rdb, 2, time.Minute)
	ctx := context.Background()

	l.Allow(ctx, "1.2.3.4")
	l.Allow(ctx, "1.2.3.4")
	ok, _ := l.Allow(ctx, "1.2.3.4")
	if ok {
		t.Fatal("3rd request should be blocked")
	}
}

func TestDifferentIPsIndependent(t *testing.T) {
	rdb := testRedis(t)
	l := NewLimiter(rdb, 1, time.Minute)
	ctx := context.Background()

	ok1, _ := l.Allow(ctx, "1.1.1.1")
	ok2, _ := l.Allow(ctx, "2.2.2.2")
	if !ok1 || !ok2 {
		t.Fatal("different IPs should be independent")
	}
}
