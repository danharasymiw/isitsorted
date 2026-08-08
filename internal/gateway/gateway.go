// Package gateway implements the HTTP gateway for the "Is It Sorted?"
// distributed service: job submission, SSE status streaming, presigned
// uploads, counter/activity endpoints, and the embedded static HTMX frontend.
package gateway

import (
	"embed"
	"io/fs"
	"net/http"
	"time"

	"sorted/internal/activity"
	"sorted/internal/counter"
	"sorted/internal/pubsub"
	"sorted/internal/queue"
	"sorted/internal/storage"
)

// Gateway holds the dependencies needed to serve the public HTTP API.
type Gateway struct {
	queue    *queue.Client
	pubsub   *pubsub.Client
	storage  *storage.Client
	counter  *counter.Counter
	activity *activity.Log
	limiter  *Limiter
	staticFS embed.FS
}

// New creates a Gateway wired to the given backing services. staticFS must
// embed a top-level "static" directory containing the frontend assets.
func New(q *queue.Client, ps *pubsub.Client, s *storage.Client, c *counter.Counter, a *activity.Log, rl *Limiter, staticFS embed.FS) *Gateway {
	return &Gateway{
		queue:    q,
		pubsub:   ps,
		storage:  s,
		counter:  c,
		activity: a,
		limiter:  rl,
		staticFS: staticFS,
	}
}

// Handler builds the top-level HTTP handler for the gateway.
func (g *Gateway) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("POST /is-sorted", g.limiter.Middleware(http.HandlerFunc(g.submitHandler)))
	mux.HandleFunc("GET /is-sorted/{id}", g.statusHandler)
	mux.HandleFunc("GET /is-sorted/{id}/events", g.sseHandler)
	mux.HandleFunc("GET /upload", g.uploadHandler)
	mux.HandleFunc("POST /upload/{id}/check", g.uploadCheckHandler)
	mux.HandleFunc("GET /count", g.countHandler)
	mux.HandleFunc("GET /activity", g.activityHandler)
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs.html", http.StatusMovedPermanently)
	})

	staticSub, err := fs.Sub(g.staticFS, "static")
	if err != nil {
		staticSub = g.staticFS
	}
	mux.Handle("GET /", http.FileServerFS(staticSub))

	return mux
}

const (
	maxBodySize = 1 << 20 // 1 MB
	sseTimeout  = 30 * time.Second
	presignTTL  = 15 * time.Minute
)
