package queue

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
	t.Cleanup(func() { rdb.FlushDB(ctx); rdb.Close() })
	return rdb
}

func TestPushPop(t *testing.T) {
	rdb := testRedis(t)
	c := New(rdb)
	ctx := context.Background()

	job := model.Job{
		ID:          "test-1",
		BucketKey:   "lists/test-1",
		Order:       "asc",
		SubmittedAt: time.Now(),
	}
	if err := c.Push(ctx, job); err != nil {
		t.Fatal(err)
	}

	got, err := c.Pop(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "test-1" || got.BucketKey != "lists/test-1" || got.Order != "asc" {
		t.Fatalf("unexpected job: %+v", got)
	}
}

func TestPopTimeout(t *testing.T) {
	rdb := testRedis(t)
	c := New(rdb)
	ctx := context.Background()

	_, err := c.Pop(ctx, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestStatusRoundTrip(t *testing.T) {
	rdb := testRedis(t)
	c := New(rdb)
	ctx := context.Background()

	if err := c.SetStatus(ctx, "job-1", model.StatusQueued); err != nil {
		t.Fatal(err)
	}
	got, err := c.GetStatus(ctx, "job-1")
	if err != nil {
		t.Fatal(err)
	}
	if got != model.StatusQueued {
		t.Fatalf("got %q, want %q", got, model.StatusQueued)
	}
}

func TestResultRoundTrip(t *testing.T) {
	rdb := testRedis(t)
	c := New(rdb)
	ctx := context.Background()

	r := model.Result{ID: "job-1", Status: model.StatusDone, Sorted: true}
	if err := c.SetResult(ctx, "job-1", r); err != nil {
		t.Fatal(err)
	}
	got, err := c.GetResult(ctx, "job-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.StatusDone || !got.Sorted {
		t.Fatalf("unexpected result: %+v", got)
	}
}
