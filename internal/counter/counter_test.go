package counter

import (
	"context"
	"os"
	"testing"

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

func TestIncrement(t *testing.T) {
	rdb := testRedis(t)
	c := New(rdb)
	ctx := context.Background()

	c.Increment(ctx, true)
	c.Increment(ctx, false)
	c.Increment(ctx, true)

	total, sorted, notSorted, err := c.Values(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("total: got %d, want 3", total)
	}
	if sorted != 2 {
		t.Fatalf("sorted: got %d, want 2", sorted)
	}
	if notSorted != 1 {
		t.Fatalf("notSorted: got %d, want 1", notSorted)
	}
}

func TestFormatCount(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{1234567, "1,234,567"},
	}
	for _, tt := range tests {
		got := FormatCount(tt.n)
		if got != tt.want {
			t.Errorf("FormatCount(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}
