package gateway

import (
	"fmt"
	"net/http"
	"time"

	"sorted/internal/model"
)

func (g *Gateway) sseHandler(w http.ResponseWriter, r *http.Request) {
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

	status, _ := g.queue.GetStatus(ctx, id)
	if status == "" {
		fmt.Fprintf(w, "event: error\ndata: {\"error\":\"job not found\"}\n\n")
		flusher.Flush()
		return
	}

	if status == model.StatusDone || status == model.StatusError {
		result, err := g.queue.GetResult(ctx, id)
		if err == nil {
			sendSSEResult(w, flusher, result)
			return
		}
	}

	sendSSEStatus(w, flusher, status)

	events, cancel := g.pubsub.Subscribe(ctx, id)
	defer cancel()

	// Re-check status after subscribing: the job may have finished between
	// the initial GetStatus call and the Subscribe call above, in which case
	// the publish already happened and would otherwise be missed, forcing
	// the client to wait out the full timeout.
	status, _ = g.queue.GetStatus(ctx, id)
	if status == model.StatusDone || status == model.StatusError {
		result, err := g.queue.GetResult(ctx, id)
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
			fmt.Fprintf(w, "event: error\ndata: {\"error\":\"timeout\"}\n\n")
			fmt.Fprintf(w, "event: close\ndata: \n\n")
			flusher.Flush()
			return
		case <-ctx.Done():
			return
		}
	}
}

func sendSSEStatus(w http.ResponseWriter, flusher http.Flusher, status string) {
	fmt.Fprintf(w, "event: status\ndata: {\"status\":%q}\n\n", status)
	flusher.Flush()
}

func sendSSEStatusEvent(w http.ResponseWriter, flusher http.Flusher, event model.StatusEvent) {
	if event.WorkerID > 0 {
		fmt.Fprintf(w, "event: status\ndata: <div class=\"result-card processing\">Processing on Worker #%d...</div>\n\n", event.WorkerID)
	} else {
		fmt.Fprintf(w, "event: status\ndata: <div class=\"result-card processing\">Processing...</div>\n\n")
	}
	flusher.Flush()
}

func sendSSEResult(w http.ResponseWriter, flusher http.Flusher, result *model.Result) {
	if result.Status == model.StatusDone {
		fmt.Fprintf(w, "event: result\ndata: %s\n\n", resultHTML(result.Sorted))
		fmt.Fprintf(w, "event: counters\ndata: refresh\n\n")
	} else {
		fmt.Fprintf(w, "event: result\ndata: <div class=\"result-card error\"><strong>Error:</strong> %s</div>\n\n", htmlEscape(result.Error))
	}
	fmt.Fprintf(w, "event: close\ndata: \n\n")
	flusher.Flush()
}
