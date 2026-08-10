package pubsub

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

func TestPublishSubscribe(t *testing.T) {
	rdb := testRedis(t)
	c := New(rdb)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, unsub := c.Subscribe(ctx, "job-1")
	defer unsub()

	time.Sleep(50 * time.Millisecond)

	err := c.Publish(ctx, "job-1", model.StatusEvent{Status: model.StatusProcessing})
	if err != nil {
		t.Fatal(err)
	}
	err = c.Publish(ctx, "job-1", model.StatusEvent{Status: model.StatusDone, Sorted: true})
	if err != nil {
		t.Fatal(err)
	}

	got := <-ch
	if got.Status != model.StatusProcessing {
		t.Fatalf("expected processing, got %q", got.Status)
	}
	got = <-ch
	if got.Status != model.StatusDone || !got.Sorted {
		t.Fatalf("expected done+sorted, got %+v", got)
	}
}
