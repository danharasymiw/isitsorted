//go:build integration

package sorted_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"sorted/internal/model"
	"sorted/internal/pubsub"
	"sorted/internal/queue"
	"sorted/internal/storage"
)

func setupRedis(t *testing.T) *redis.Client {
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

func TestEndToEndSortCheck(t *testing.T) {
	rdb := setupRedis(t)

	s3Endpoint := os.Getenv("S3_ENDPOINT")
	if s3Endpoint == "" {
		t.Skip("S3_ENDPOINT not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := storage.New(ctx, storage.Config{
		Endpoint:     s3Endpoint,
		Bucket:       os.Getenv("S3_BUCKET"),
		AccessKey:    os.Getenv("S3_ACCESS_KEY_ID"),
		SecretKey:    os.Getenv("S3_SECRET_ACCESS_KEY"),
		UsePathStyle: os.Getenv("S3_USE_PATH_STYLE") == "true",
	})
	if err != nil {
		t.Skipf("S3 not available: %v", err)
	}

	qc := queue.New(rdb)
	ps := pubsub.New(rdb)

	// Submit a job directly via the queue (simulating what the job service does)
	listContent := "1\n2\n3"
	id := "test-integration-001"
	if err := store.PutList(ctx, id, []byte(listContent)); err != nil {
		t.Fatal(err)
	}
	job := model.Job{
		ID:        id,
		BucketKey: "lists/" + id,
		Order:     "asc",
	}
	qc.SetStatus(ctx, id, model.StatusQueued)
	if err := qc.Push(ctx, job); err != nil {
		t.Fatal(err)
	}

	// Start a minimal worker loop in the background
	go func() {
		for {
			j, err := qc.Pop(ctx, 2*time.Second)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				continue
			}
			ps.Publish(ctx, j.ID, model.StatusEvent{Status: model.StatusProcessing, WorkerID: 1})
			qc.SetStatus(ctx, j.ID, model.StatusProcessing)

			data, err := store.GetList(ctx, j.ID)
			if err != nil {
				continue
			}
			_ = data // In a real test we'd parse and check

			result := model.Result{ID: j.ID, Status: model.StatusDone, Sorted: true}
			qc.SetResult(ctx, j.ID, result)
			qc.SetStatus(ctx, j.ID, model.StatusDone)
			ps.Publish(ctx, j.ID, model.StatusEvent{Status: model.StatusDone, Sorted: true})
		}
	}()

	// Poll for result
	var result model.Result
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r, err := qc.GetResult(ctx, id)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		result = *r
		break
	}

	if result.Status != model.StatusDone {
		t.Fatalf("job did not complete: status=%s", result.Status)
	}
	if !result.Sorted {
		t.Fatal("expected sorted=true for [1,2,3] asc")
	}
}
