// Package model defines the shared data types passed between the gateway
// and worker components of the "Is It Sorted?" service.
package model

import "time"

const (
	StatusQueued     = "queued"
	StatusProcessing = "processing"
	StatusDone       = "done"
	StatusError      = "error"
)

// Job describes a unit of work submitted for sort-checking.
type Job struct {
	ID          string    `json:"id"`
	BucketKey   string    `json:"bucket_key"`
	Order       string    `json:"order"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// Result is the outcome of processing a Job.
type Result struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Sorted bool   `json:"sorted,omitempty"`
	Error  string `json:"error,omitempty"`
}

// StatusEvent is a status update pushed to clients (e.g. over SSE).
type StatusEvent struct {
	Status string `json:"status"`
	Sorted bool   `json:"sorted,omitempty"`
	Error  string `json:"error,omitempty"`
}

// ActivityEntry is a single record in the recent-activity feed.
type ActivityEntry struct {
	At     time.Time `json:"at"`
	Sorted bool      `json:"sorted"`
	Order  string    `json:"order"`
	List   []string  `json:"list"`
}
