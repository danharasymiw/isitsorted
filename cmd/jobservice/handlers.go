package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"sorted/internal/activity"
	"sorted/internal/counter"
	"sorted/internal/model"
	"sorted/internal/pubsub"
	"sorted/internal/queue"
	"sorted/internal/storage"
	"sorted/parser"
)

const (
	maxBodySize = 1 << 20 // 1 MB
	sseTimeout  = 30 * time.Second
	presignTTL  = 15 * time.Minute
)

type JobService struct {
	queue    *queue.Client
	pubsub   *pubsub.Client
	storage  *storage.Client
	counter  *counter.Counter
	activity *activity.Log
	rdb      *redis.Client
}

func (s *JobService) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /jobs", s.submitHandler)
	mux.HandleFunc("GET /jobs/{id}", s.statusHandler)
	mux.HandleFunc("GET /jobs/{id}/events", s.sseHandler)
	mux.HandleFunc("POST /uploads", s.uploadHandler)
	mux.HandleFunc("POST /uploads/{id}/check", s.uploadCheckHandler)
	mux.HandleFunc("GET /stats/count", s.countHandler)
	mux.HandleFunc("GET /stats/activity", s.activityHandler)
	mux.HandleFunc("GET /health", s.healthHandler)
	return mux
}

func newUUID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

type submitRequest struct {
	List  []string `json:"list"`
	Order string   `json:"order"`
}

func (s *JobService) submitHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req submitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid JSON: %s", err)})
		return
	}
	if len(req.List) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "list is required"})
		return
	}
	if req.Order == "" {
		req.Order = "asc"
	}
	if req.Order != "asc" && req.Order != "desc" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "order must be asc or desc"})
		return
	}

	// Expand comma-separated values using bracket-aware splitting (handles
	// form submissions where "1, {2,3}, 4" arrives as a single string).
	var expanded []string
	for _, item := range req.List {
		parts := parser.SplitBracketAware(item, ',')
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if _, err := parser.ParseValue(p); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("cannot parse %q: %s", p, err)})
				return
			}
			expanded = append(expanded, p)
		}
	}
	if len(expanded) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "list is required"})
		return
	}

	id := newUUID()
	listContent := strings.Join(expanded, "\n")
	ctx := r.Context()

	if err := s.storage.PutList(ctx, id, []byte(listContent)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "storage error"})
		return
	}

	job := model.Job{
		ID:          id,
		BucketKey:   "lists/" + id,
		Order:       req.Order,
		SubmittedAt: time.Now(),
	}
	s.queue.SetStatus(ctx, id, model.StatusQueued)
	if err := s.queue.Push(ctx, job); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "queue error"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (s *JobService) statusHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()
	result, err := s.queue.GetResult(ctx, id)
	if err != nil {
		status, err2 := s.queue.GetStatus(ctx, id)
		if err2 != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"id": id, "status": status})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *JobService) uploadHandler(w http.ResponseWriter, r *http.Request) {
	id := newUUID()
	url, err := s.storage.PresignPut(r.Context(), id, presignTTL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "presign error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id, "upload_url": url})
}

func (s *JobService) uploadCheckHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Order string `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		body.Order = "asc"
	}
	if body.Order == "" {
		body.Order = "asc"
	}
	if body.Order != "asc" && body.Order != "desc" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "order must be asc or desc"})
		return
	}

	ctx := r.Context()

	data, err := s.storage.GetList(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "upload not found"})
		return
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, err := parser.ParseValue(line); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("cannot parse %q: %s", line, err)})
			return
		}
	}

	job := model.Job{
		ID:          id,
		BucketKey:   "lists/" + id,
		Order:       body.Order,
		SubmittedAt: time.Now(),
	}
	s.queue.SetStatus(ctx, id, model.StatusQueued)
	if err := s.queue.Push(ctx, job); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "queue error"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

type countResponse struct {
	Total     int64 `json:"total"`
	Sorted    int64 `json:"sorted"`
	NotSorted int64 `json:"not_sorted"`
}

type activityResponse struct {
	Entries []model.ActivityEntry `json:"entries"`
}

func (s *JobService) countHandler(w http.ResponseWriter, r *http.Request) {
	total, sorted, notSorted, _ := s.counter.Values(r.Context())
	writeJSON(w, http.StatusOK, countResponse{Total: total, Sorted: sorted, NotSorted: notSorted})
}

func (s *JobService) activityHandler(w http.ResponseWriter, r *http.Request) {
	entries, _ := s.activity.Recent(r.Context())
	if entries == nil {
		entries = []model.ActivityEntry{}
	}
	writeJSON(w, http.StatusOK, activityResponse{Entries: entries})
}

type healthResponse struct {
	Status  string `json:"status"`
	Redis   string `json:"redis"`
	Storage string `json:"storage"`
}

func (s *JobService) healthHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	h := healthResponse{Status: "healthy", Redis: "ok", Storage: "ok"}

	if err := s.rdb.Ping(ctx).Err(); err != nil {
		h.Redis = "error"
		h.Status = "degraded"
	}
	if _, err := s.storage.GetState(ctx, "counter.json"); err != nil {
		h.Storage = "error"
		h.Status = "degraded"
	}

	writeJSON(w, http.StatusOK, h)
}
