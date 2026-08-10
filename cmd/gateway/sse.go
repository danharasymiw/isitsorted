package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

const sseTimeout = 30 * time.Second

func (g *Gateway) sseHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	id := r.PathValue("id")
	html := r.URL.Query().Get("format") == "html"

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	resp, err := g.client.SSEStream(r.Context(), id)
	if err != nil {
		_, _ = fmt.Fprintf(w, "event: error\ndata: {\"error\":\"job service unavailable\"}\n\n")
		_, _ = fmt.Fprintf(w, "event: close\ndata: \n\n")
		flusher.Flush()
		return
	}
	defer func() { _ = resp.Body.Close() }()

	scanner := bufio.NewScanner(resp.Body)
	var eventType string

	timeout := time.NewTimer(sseTimeout)
	defer timeout.Stop()

	lines := make(chan string)
	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()

	for {
		select {
		case line, ok := <-lines:
			if !ok {
				return
			}
			if strings.HasPrefix(line, "event: ") {
				eventType = strings.TrimPrefix(line, "event: ")
				continue
			}
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				timeout.Reset(sseTimeout)

				if html {
					g.reemitHTML(w, flusher, eventType, data, id)
				} else {
					_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, data)
					flusher.Flush()
				}

				if eventType == "close" {
					return
				}
			}
		case <-timeout.C:
			_, _ = fmt.Fprintf(w, "event: error\ndata: {\"error\":\"timeout\"}\n\n")
			_, _ = fmt.Fprintf(w, "event: close\ndata: \n\n")
			flusher.Flush()
			return
		case <-r.Context().Done():
			return
		}
	}
}

func (g *Gateway) reemitHTML(w http.ResponseWriter, flusher http.Flusher, eventType, data, id string) {
	switch eventType {
	case "status":
		var ev struct {
			Status   string `json:"status"`
			WorkerID int    `json:"worker_id"`
		}
		_ = json.Unmarshal([]byte(data), &ev)
		short := id
		if len(short) > 8 {
			short = short[:8]
		}
		label := fmt.Sprintf("Queued as %s...", short)
		if ev.Status == "processing" {
			workerID := ev.WorkerID
			if workerID == 0 {
				workerID = rand.Intn(5) + 1
			}
			label = fmt.Sprintf("Processing on Worker #%d...", workerID)
		}
		_, _ = fmt.Fprintf(w, "event: status\ndata: <div class=\"result-card processing\">%s</div>\n\n", label)
		flusher.Flush()

	case "result":
		var ev struct {
			Status string `json:"status"`
			Sorted bool   `json:"sorted"`
			Error  string `json:"error"`
		}
		_ = json.Unmarshal([]byte(data), &ev)
		if ev.Status == "done" {
			_, _ = fmt.Fprintf(w, "event: result\ndata: %s\n\n", resultHTML(ev.Sorted))
			_, _ = fmt.Fprintf(w, "event: counters\ndata: refresh\n\n")
		} else {
			_, _ = fmt.Fprintf(w, "event: result\ndata: <div class=\"result-card error\"><strong>Error:</strong> %s</div>\n\n", htmlEscape(ev.Error))
		}
		_, _ = fmt.Fprintf(w, "event: close\ndata: \n\n")
		flusher.Flush()

	case "error":
		_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
		flusher.Flush()

	case "close":
		_, _ = fmt.Fprintf(w, "event: close\ndata: \n\n")
		flusher.Flush()
	}
}
