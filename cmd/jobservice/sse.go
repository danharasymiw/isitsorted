package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"sorted/internal/model"
)

func (s *JobService) sseHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	id := r.PathValue("id")
	ctx := r.Context()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	status, _ := s.queue.GetStatus(ctx, id)
	if status == "" {
		_, _ = fmt.Fprintf(w, "event: error\ndata: {\"error\":\"job not found\"}\n\n")
		_, _ = fmt.Fprintf(w, "event: close\ndata: \n\n")
		flusher.Flush()
		return
	}

	if status == model.StatusDone || status == model.StatusError {
		result, err := s.queue.GetResult(ctx, id)
		if err == nil {
			sendSSEResult(w, flusher, result)
			return
		}
	}

	sendSSEStatus(w, flusher, status)

	events, cancel := s.pubsub.Subscribe(ctx, id)
	defer cancel()

	status, _ = s.queue.GetStatus(ctx, id)
	if status == model.StatusDone || status == model.StatusError {
		result, err := s.queue.GetResult(ctx, id)
		if err == nil {
			sendSSEResult(w, flusher, result)
			return
		}
	}

	timeout := time.NewTimer(sseTimeout)
	defer timeout.Stop()

	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			if event.Status == model.StatusDone || event.Status == model.StatusError {
				result := &model.Result{
					ID:     id,
					Status: event.Status,
					Sorted: event.Sorted,
					Error:  event.Error,
				}
				sendSSEResult(w, flusher, result)
				return
			}
			sendSSEStatusEvent(w, flusher, event)
			timeout.Reset(sseTimeout)
		case <-timeout.C:
			_, _ = fmt.Fprintf(w, "event: error\ndata: {\"error\":\"timeout\"}\n\n")
			_, _ = fmt.Fprintf(w, "event: close\ndata: \n\n")
			flusher.Flush()
			return
		case <-ctx.Done():
			return
		}
	}
}

func sendSSEStatus(w http.ResponseWriter, flusher http.Flusher, status string) {
	data, _ := json.Marshal(map[string]string{"status": status})
	_, _ = fmt.Fprintf(w, "event: status\ndata: %s\n\n", data)
	flusher.Flush()
}

func sendSSEStatusEvent(w http.ResponseWriter, flusher http.Flusher, event model.StatusEvent) {
	payload := map[string]any{"status": event.Status}
	if event.WorkerID > 0 {
		payload["worker_id"] = event.WorkerID
	}
	data, _ := json.Marshal(payload)
	_, _ = fmt.Fprintf(w, "event: status\ndata: %s\n\n", data)
	flusher.Flush()
}

func sendSSEResult(w http.ResponseWriter, flusher http.Flusher, result *model.Result) {
	var data []byte
	if result.Status == model.StatusDone {
		data, _ = json.Marshal(map[string]any{"status": "done", "sorted": result.Sorted})
	} else {
		data, _ = json.Marshal(map[string]any{"status": "error", "error": result.Error})
	}
	_, _ = fmt.Fprintf(w, "event: result\ndata: %s\n\n", data)
	_, _ = fmt.Fprintf(w, "event: close\ndata: \n\n")
	flusher.Flush()
}
