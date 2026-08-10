package storage

import (
	"context"
	"os"
	"testing"
	"time"
)

func testStorage(t *testing.T) *Client {
	t.Helper()
	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_ENDPOINT not set, skipping storage tests")
	}
	ctx := context.Background()
	c, err := New(ctx, Config{
		Endpoint:     endpoint,
		Bucket:       os.Getenv("S3_BUCKET"),
		AccessKey:    os.Getenv("S3_ACCESS_KEY"),
		SecretKey:    os.Getenv("S3_SECRET_KEY"),
		UsePathStyle: os.Getenv("S3_USE_PATH_STYLE") == "true",
	})
	if err != nil {
		t.Fatalf("create storage client: %v", err)
	}
	return c
}

func TestPutGetList(t *testing.T) {
	c := testStorage(t)
	ctx := context.Background()

	data := []byte("1\n2\n3\n")
	if err := c.PutList(ctx, "test-1", data); err != nil {
		t.Fatal(err)
	}
	got, err := c.GetList(ctx, "test-1")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("got %q, want %q", got, data)
	}
}

func TestPutGetResult(t *testing.T) {
	c := testStorage(t)
	ctx := context.Background()

	data := []byte(`{"sorted":true}`)
	if err := c.PutResult(ctx, "test-1", data); err != nil {
		t.Fatal(err)
	}
	got, err := c.GetResult(ctx, "test-1")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("got %q, want %q", got, data)
	}
}

func TestPutGetState(t *testing.T) {
	c := testStorage(t)
	ctx := context.Background()

	data := []byte(`{"count":42}`)
	if err := c.PutState(ctx, "counter.json", data); err != nil {
		t.Fatal(err)
	}
	got, err := c.GetState(ctx, "counter.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("got %q, want %q", got, data)
	}
}

func TestListByPrefix(t *testing.T) {
	c := testStorage(t)
	ctx := context.Background()

	if err := c.PutList(ctx, "prefix-a", []byte("a")); err != nil {
		t.Fatal(err)
	}
	if err := c.PutList(ctx, "prefix-b", []byte("b")); err != nil {
		t.Fatal(err)
	}
	if err := c.PutResult(ctx, "other", []byte("x")); err != nil {
		t.Fatal(err)
	}

	objects, err := c.ListByPrefix(ctx, "lists/prefix-")
	if err != nil {
		t.Fatal(err)
	}
	if len(objects) != 2 {
		t.Fatalf("got %d objects, want 2", len(objects))
	}
	keys := map[string]bool{}
	for _, o := range objects {
		keys[o.Key] = true
	}
	if !keys["lists/prefix-a"] || !keys["lists/prefix-b"] {
		t.Fatalf("unexpected keys: %v", keys)
	}
}

func TestDelete(t *testing.T) {
	c := testStorage(t)
	ctx := context.Background()

	if err := c.PutList(ctx, "to-delete", []byte("data")); err != nil {
		t.Fatal(err)
	}

	got, err := c.GetList(ctx, "to-delete")
	if err != nil {
		t.Fatal("expected object to exist before delete")
	}
	if string(got) != "data" {
		t.Fatalf("got %q, want %q", got, "data")
	}

	if err := c.Delete(ctx, "lists/to-delete"); err != nil {
		t.Fatal(err)
	}

	_, err = c.GetList(ctx, "to-delete")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestPresignPut(t *testing.T) {
	c := testStorage(t)
	ctx := context.Background()

	url, err := c.PresignPut(ctx, "test-presign", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if url == "" {
		t.Fatal("expected non-empty presigned URL")
	}
}
