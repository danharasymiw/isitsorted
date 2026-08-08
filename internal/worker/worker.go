// Package worker implements the job-processing loop that pops jobs off the
// queue, reads list input from the bucket, runs the sort check, and
// publishes status events and results.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"sorted/internal/activity"
	"sorted/internal/counter"
	"sorted/internal/model"
	"sorted/internal/pubsub"
	"sorted/internal/queue"
	"sorted/internal/sortcheck"
	"sorted/internal/storage"
	"sorted/parser"
)

// Worker consumes jobs from the queue, checks whether the submitted list is
// sorted, and records the result and status via Redis and bucket storage.
type Worker struct {
	queue    *queue.Client
	pubsub   *pubsub.Client
	storage  *storage.Client
	counter  *counter.Counter
	activity *activity.Log
	logger   *slog.Logger
}

// New creates a Worker from its dependencies.
func New(q *queue.Client, ps *pubsub.Client, s *storage.Client, c *counter.Counter, a *activity.Log, logger *slog.Logger) *Worker {
	return &Worker{
		queue:    q,
		pubsub:   ps,
		storage:  s,
		counter:  c,
		activity: a,
		logger:   logger,
	}
}

// Run blocks, popping jobs off the queue and processing them one at a time,
// until ctx is canceled.
func (w *Worker) Run(ctx context.Context) error {
	w.logger.Info("worker started")
	go w.snapshotLoop(ctx)
	for {
		job, err := w.queue.Pop(ctx, 5*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			continue
		}
		w.logger.Info("job picked up", "id", job.ID)
		if err := w.ProcessJob(ctx, job); err != nil {
			w.logger.Error("job failed", "id", job.ID, "error", err)
		}
	}
}

// snapshotLoop periodically writes counter/activity state to the bucket so
// it survives worker restarts and can be read by other components.
func (w *Worker) snapshotLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.snapshotState(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (w *Worker) snapshotState(ctx context.Context) {
	total, sorted, notSorted, err := w.counter.Values(ctx)
	if err != nil {
		w.logger.Error("snapshot counter failed", "error", err)
		return
	}
	counterJSON, _ := json.Marshal(map[string]int64{
		"count": total, "sorted": sorted, "not_sorted": notSorted,
	})
	w.storage.PutState(ctx, "counter.json", counterJSON)

	entries, err := w.activity.Recent(ctx)
	if err != nil {
		w.logger.Error("snapshot activity failed", "error", err)
		return
	}
	activityJSON, _ := json.Marshal(entries)
	w.storage.PutState(ctx, "activity.json", activityJSON)

	w.logger.Info("state snapshot written to bucket")
}

// ProcessJob reads the job's list from the bucket, parses and checks it,
// and records the status/result via Redis, the bucket, and pub/sub.
func (w *Worker) ProcessJob(ctx context.Context, job *model.Job) error {
	w.pubsub.Publish(ctx, job.ID, model.StatusEvent{Status: model.StatusProcessing})
	w.queue.SetStatus(ctx, job.ID, model.StatusProcessing)

	data, err := w.storage.GetList(ctx, job.ID)
	if err != nil {
		return w.failJob(ctx, job.ID, fmt.Errorf("read list from bucket: %w", err))
	}

	rawList := splitLines(string(data))
	values := make([]*parser.Value, 0, len(rawList))
	for _, raw := range rawList {
		v, err := parser.ParseValue(raw)
		if err != nil {
			return w.failJob(ctx, job.ID, fmt.Errorf("parse %q: %w", raw, err))
		}
		values = append(values, v)
	}

	sorted := sortcheck.Check(values, job.Order)

	result := model.Result{
		ID:     job.ID,
		Status: model.StatusDone,
		Sorted: sorted,
	}
	resultData, _ := json.Marshal(result)
	w.queue.SetResult(ctx, job.ID, result)
	w.queue.SetStatus(ctx, job.ID, model.StatusDone)
	w.storage.PutResult(ctx, job.ID, resultData)

	w.counter.Increment(ctx, sorted)
	w.activity.Add(ctx, model.ActivityEntry{
		At:     time.Now(),
		Sorted: sorted,
		Order:  job.Order,
		List:   rawList,
	})

	w.pubsub.Publish(ctx, job.ID, model.StatusEvent{Status: model.StatusDone, Sorted: sorted})
	w.logger.Info("job completed", "id", job.ID, "sorted", sorted, "items", len(rawList))
	return nil
}

func (w *Worker) failJob(ctx context.Context, id string, err error) error {
	result := model.Result{
		ID:     id,
		Status: model.StatusError,
		Error:  err.Error(),
	}
	w.queue.SetResult(ctx, id, result)
	w.queue.SetStatus(ctx, id, model.StatusError)
	w.pubsub.Publish(ctx, id, model.StatusEvent{Status: model.StatusError, Error: err.Error()})
	return err
}

func splitLines(s string) []string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}
