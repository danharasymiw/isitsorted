// Package queue wraps Redis lists and strings to provide job queuing and
// status/result storage for the "Is It Sorted?" distributed service.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"sorted/internal/model"
)

const (
	queueKey  = "sorted:queue"
	statusKey = "sorted:status:%s"
	resultKey = "sorted:result:%s"
	resultTTL = 5 * time.Minute
)

// Client wraps a Redis client to provide job queue and status/result storage
// operations.
type Client struct {
	rdb *redis.Client
}

// New creates a Client backed by the given Redis client.
func New(rdb *redis.Client) *Client {
	return &Client{rdb: rdb}
}

// Push enqueues a job for processing.
func (c *Client) Push(ctx context.Context, job model.Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	return c.rdb.LPush(ctx, queueKey, data).Err()
}

// Pop blocks up to timeout waiting for a job to become available.
func (c *Client) Pop(ctx context.Context, timeout time.Duration) (*model.Job, error) {
	res, err := c.rdb.BRPop(ctx, timeout, queueKey).Result()
	if err != nil {
		return nil, err
	}
	var job model.Job
	if err := json.Unmarshal([]byte(res[1]), &job); err != nil {
		return nil, fmt.Errorf("unmarshal job: %w", err)
	}
	return &job, nil
}

// SetStatus records the current status of a job.
func (c *Client) SetStatus(ctx context.Context, id, status string) error {
	return c.rdb.Set(ctx, fmt.Sprintf(statusKey, id), status, resultTTL).Err()
}

// GetStatus returns the current status of a job.
func (c *Client) GetStatus(ctx context.Context, id string) (string, error) {
	return c.rdb.Get(ctx, fmt.Sprintf(statusKey, id)).Result()
}

// SetResult stores the result of a completed job.
func (c *Client) SetResult(ctx context.Context, id string, result model.Result) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	return c.rdb.Set(ctx, fmt.Sprintf(resultKey, id), data, resultTTL).Err()
}

// GetResult retrieves the result of a completed job.
func (c *Client) GetResult(ctx context.Context, id string) (*model.Result, error) {
	data, err := c.rdb.Get(ctx, fmt.Sprintf(resultKey, id)).Result()
	if err != nil {
		return nil, err
	}
	var result model.Result
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}
	return &result, nil
}
