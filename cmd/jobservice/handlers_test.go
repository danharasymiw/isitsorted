package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"
	"sorted/internal/activity"
	"sorted/internal/counter"
	"sorted/internal/pubsub"
	"sorted/internal/queue"
	"sorted/internal/storage"
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

func testStorage(t *testing.T) *storage.Client {
	t.Helper()
	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_ENDPOINT not set")
	}
	ctx := context.Background()
	store, err := storage.New(ctx, storage.Config{
		Endpoint:     endpoint,
		Bucket:       os.Getenv("S3_BUCKET"),
		AccessKey:    os.Getenv("S3_ACCESS_KEY_ID"),
		SecretKey:    os.Getenv("S3_SECRET_ACCESS_KEY"),
		UsePathStyle: os.Getenv("S3_USE_PATH_STYLE") == "true",
	})
	if err != nil {
		t.Skipf("S3 not available: %v", err)
	}
	return store
}

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	rdb := testRedis(t)
	store := testStorage(t)
	svc := &JobService{
		queue:    queue.New(rdb),
		pubsub:   pubsub.New(rdb),
		storage:  store,
		counter:  counter.New(rdb),
		activity: activity.New(rdb),
		rdb:      rdb,
	}
	return httptest.NewServer(svc.Handler())
}

func TestSubmitJobValid(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()

	body := `{"list": ["1", "2", "3"], "order": "asc"}`
	resp, err := http.Post(srv.URL+"/jobs", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result["id"] == "" {
		t.Fatal("expected non-empty id")
	}
}

func TestSubmitJobInvalidValue(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()

	body := `{"list": ["hello world not a number"], "order": "asc"}`
	resp, err := http.Post(srv.URL+"/jobs", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSubmitJobInvalidOrder(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()

	body := `{"list": ["1"], "order": "sideways"}`
	resp, err := http.Post(srv.URL+"/jobs", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSubmitJobMissingList(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()

	body := `{"order": "asc"}`
	resp, err := http.Post(srv.URL+"/jobs", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}
