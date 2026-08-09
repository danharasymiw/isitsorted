package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/redis/go-redis/v9"
	"sorted/internal/activity"
	"sorted/internal/counter"
	"sorted/internal/pubsub"
	"sorted/internal/queue"
	"sorted/internal/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

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

	ctx := context.Background()
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

	svc := &JobService{
		queue:    queue.New(rdb),
		pubsub:   pubsub.New(rdb),
		storage:  store,
		counter:  counter.New(rdb),
		activity: activity.New(rdb),
		rdb:      rdb,
	}

	logger.Info("job service starting", "port", port)
	if err := http.ListenAndServe(":"+port, svc.Handler()); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
