// Package pubsub wraps Redis pub/sub to broadcast job status events in
// real time for the "Is It Sorted?" distributed service.
package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"sorted/internal/model"
)

const channelPrefix = "sorted:events:"

// Client wraps a Redis client to publish and subscribe to job status events.
type Client struct {
	rdb *redis.Client
}

// New creates a Client backed by the given Redis client.
func New(rdb *redis.Client) *Client {
	return &Client{rdb: rdb}
}

// Publish broadcasts a status event for the given job.
func (c *Client) Publish(ctx context.Context, jobID string, event model.StatusEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	return c.rdb.Publish(ctx, channelPrefix+jobID, data).Err()
}

// Subscribe listens for status events for the given job. It returns a
// channel of events and a cancel function that must be called to release
// the subscription.
func (c *Client) Subscribe(ctx context.Context, jobID string) (<-chan model.StatusEvent, func()) {
	sub := c.rdb.Subscribe(ctx, channelPrefix+jobID)
	ch := make(chan model.StatusEvent, 8)

	go func() {
		defer close(ch)
		for msg := range sub.Channel() {
			var event model.StatusEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				continue
			}
			select {
			case ch <- event:
			case <-ctx.Done():
				return
			}
		}
	}()

	cancel := func() {
		_ = sub.Close()
	}
	return ch, cancel
}
