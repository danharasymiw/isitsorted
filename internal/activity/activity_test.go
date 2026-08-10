package activity

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"sorted/internal/model"
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
	t.Cleanup(func() { rdb.FlushDB(ctx); _ = rdb.Close() })
	return rdb
}

func TestAddAndRecent(t *testing.T) {
	rdb := testRedis(t)
	l := New(rdb)
	ctx := context.Background()

	if err := l.Add(ctx, model.ActivityEntry{At: time.Now(), Sorted: true, Order: "asc", List: []string{"1", "2"}}); err != nil {
		t.Fatal(err)
	}
	if err := l.Add(ctx, model.ActivityEntry{At: time.Now(), Sorted: false, Order: "desc", List: []string{"3", "1"}}); err != nil {
		t.Fatal(err)
	}

	entries, err := l.Recent(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Order != "desc" {
		t.Fatal("most recent should be first (LPUSH order)")
	}
}

func TestRecentCappedAt20(t *testing.T) {
	rdb := testRedis(t)
	l := New(rdb)
	ctx := context.Background()

	for i := 0; i < 25; i++ {
		if err := l.Add(ctx, model.ActivityEntry{At: time.Now(), Sorted: true, Order: "asc", List: []string{"1"}}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := l.Recent(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 20 {
		t.Fatalf("got %d entries, want 20", len(entries))
	}
}

func TestFormatList(t *testing.T) {
	tests := []struct {
		list []string
		want string
	}{
		{[]string{"1", "2", "3"}, "[1, 2, 3]"},
		{[]string{"a"}, "[a]"},
		{nil, "[]"},
	}
	for _, tt := range tests {
		got := FormatList(tt.list)
		if got != tt.want {
			t.Errorf("FormatList(%v) = %q, want %q", tt.list, got, tt.want)
		}
	}
}

func TestOrderLabel(t *testing.T) {
	tests := []struct {
		order string
		want  string
	}{
		{"asc", "ascending"},
		{"desc", "descending"},
		{"", "ascending"},
	}
	for _, tt := range tests {
		got := OrderLabel(tt.order)
		if got != tt.want {
			t.Errorf("OrderLabel(%q) = %q, want %q", tt.order, got, tt.want)
		}
	}
}

func TestTimeAgo(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "just now"},
		{30 * time.Second, "30s ago"},
		{5 * time.Minute, "5m ago"},
		{2 * time.Hour, "2h ago"},
		{48 * time.Hour, "2d ago"},
	}
	for _, tt := range tests {
		got := TimeAgo(time.Now().Add(-tt.d))
		if got != tt.want {
			t.Errorf("TimeAgo(-%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}
