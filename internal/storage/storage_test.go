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
		Endpoint:  endpoint,
		Bucket:    os.Getenv("S3_BUCKET"),
		AccessKey: os.Getenv("S3_ACCESS_KEY"),
		SecretKey: os.Getenv("S3_SECRET_KEY"),
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
