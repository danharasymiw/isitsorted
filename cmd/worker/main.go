// Command worker runs the "Is It Sorted?" background job processor: it
// pops jobs off the Redis queue, reads list input from the bucket, checks
// whether the list is sorted, and records status/results.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"github.com/redis/go-redis/v9"
	"sorted/internal/activity"
	"sorted/internal/counter"
	"sorted/internal/pubsub"
	"sorted/internal/queue"
	"sorted/internal/storage"
	"sorted/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		logger.Error("REDIS_URL is required")
		os.Exit(1)
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		logger.Error("invalid REDIS_URL", "error", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(opts)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	store, err := storage.New(ctx, storage.Config{
		Endpoint:     os.Getenv("S3_ENDPOINT"),
		Bucket:       os.Getenv("S3_BUCKET"),
		AccessKey:    os.Getenv("S3_ACCESS_KEY_ID"),
		SecretKey:    os.Getenv("S3_SECRET_ACCESS_KEY"),
		UsePathStyle: os.Getenv("S3_USE_PATH_STYLE") == "true",
	})
	if err != nil {
		logger.Error("failed to create storage client", "error", err)
		os.Exit(1)
	}

	w := worker.New(
		queue.New(rdb),
		pubsub.New(rdb),
		store,
		counter.New(rdb),
		activity.New(rdb),
		logger,
	)

	logger.Info("starting worker")
	if err := w.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}
