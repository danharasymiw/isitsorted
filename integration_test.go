//go:build integration

package sorted_test

import (
	"context"
	"embed"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"sorted/internal/activity"
	"sorted/internal/counter"
	"sorted/internal/gateway"
	"sorted/internal/model"
	"sorted/internal/pubsub"
	"sorted/internal/queue"
	"sorted/internal/storage"
	"sorted/internal/worker"
)

// testStaticFS is an empty embedded filesystem used in place of the
// gateway's real static assets. The integration test only exercises the
// JSON API endpoints, not static file serving, so no embedded files are
// needed here.
var testStaticFS embed.FS

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
	t.Cleanup(func() { rdb.FlushDB(ctx); rdb.Close() })
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
		Endpoint:  s3Endpoint,
		Bucket:    os.Getenv("S3_BUCKET"),
		AccessKey: os.Getenv("S3_ACCESS_KEY"),
		SecretKey: os.Getenv("S3_SECRET_KEY"),
	})
	if err != nil {
		t.Skipf("S3 not available: %v", err)
	}

	qc := queue.New(rdb)
	ps := pubsub.New(rdb)
	ctr := counter.New(rdb)
	act := activity.New(rdb)
	rl := gateway.NewLimiter(rdb, 100, time.Minute)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	gw := gateway.New(qc, ps, store, ctr, act, rl, testStaticFS)
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()

	w := worker.New(qc, ps, store, ctr, act, logger)
	go w.Run(ctx)

	// Submit a sorted list
	body := `{"list": [1, 2, 3], "order": "asc"}`
	resp, err := http.Post(srv.URL+"/is-sorted", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	var submitResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&submitResp); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	id, exists := submitResp["id"]
	if !exists || id == "" {
		t.Fatal("missing or empty id in submit response")
	}

	// Poll for result
	var result model.Result
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err = http.Get(srv.URL + "/is-sorted/" + id)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if result.Status == model.StatusDone {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if result.Status != model.StatusDone {
		t.Fatalf("job did not complete: status=%s", result.Status)
	}
	if !result.Sorted {
		t.Fatal("expected sorted=true for [1,2,3] asc")
	}
}
