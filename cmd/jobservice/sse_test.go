package main

import (
	"bufio"
	"net/http"
	"strings"
	"testing"
	"time"

	"sorted/internal/model"
)

type sseEvent struct {
	Event string
	Data  string
}

func collectSSEEvents(t *testing.T, resp *http.Response, timeout time.Duration) []sseEvent {
	t.Helper()
	var events []sseEvent

	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(resp.Body)
		var event string
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "event: ") {
				event = strings.TrimPrefix(line, "event: ")
			} else if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				events = append(events, sseEvent{event, data})
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		resp.Body.Close()
		t.Fatal("SSE stream did not close in time")
	}
	return events
}

func TestSSEJobNotFound(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/jobs/nonexistent/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	events := collectSSEEvents(t, resp, 2*time.Second)
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events (error + close), got %d", len(events))
	}
	if events[0].Event != "error" {
		t.Fatalf("first event = %q, want error", events[0].Event)
	}
	if !strings.Contains(events[0].Data, "job not found") {
		t.Fatalf("error data = %q, want 'job not found'", events[0].Data)
	}
	if events[1].Event != "close" {
		t.Fatalf("second event = %q, want close", events[1].Event)
	}
}

func TestSSEAlreadyDone(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()

	body := `{"list": ["1", "2", "3"], "order": "asc"}`
	submitResp, err := http.Post(srv.URL+"/jobs", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	submitResp.Body.Close()

	// Wait for the job ID, then manually mark it done via the test's queue
	rdb := testRedis(t)
	qc := testQueue(t, rdb)

	// Get the job from the queue and process it inline
	job, err := qc.Pop(t.Context(), 2*time.Second)
	if err != nil {
		t.Fatalf("no job on queue: %v", err)
	}

	result := model.Result{ID: job.ID, Status: model.StatusDone, Sorted: true}
	qc.SetResult(t.Context(), job.ID, result)
	qc.SetStatus(t.Context(), job.ID, model.StatusDone)

	resp, err := http.Get(srv.URL + "/jobs/" + job.ID + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	events := collectSSEEvents(t, resp, 2*time.Second)
	var gotResult, gotClose bool
	for _, ev := range events {
		if ev.Event == "result" && strings.Contains(ev.Data, `"done"`) {
			gotResult = true
		}
		if ev.Event == "close" {
			gotClose = true
		}
	}
	if !gotResult {
		t.Fatal("expected result event with done status")
	}
	if !gotClose {
		t.Fatal("expected close event")
	}
}

func TestSSELivePubSub(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()

	body := `{"list": ["1", "2", "3"], "order": "asc"}`
	submitResp, err := http.Post(srv.URL+"/jobs", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	submitResp.Body.Close()

	rdb := testRedis(t)
	qc := testQueue(t, rdb)
	ps := testPubSub(t, rdb)

	job, err := qc.Pop(t.Context(), 2*time.Second)
	if err != nil {
		t.Fatalf("no job on queue: %v", err)
	}

	// Start SSE stream before publishing events
	resp, err := http.Get(srv.URL + "/jobs/" + job.ID + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Give the SSE handler time to subscribe
	time.Sleep(100 * time.Millisecond)

	// Simulate worker: processing then done
	qc.SetStatus(t.Context(), job.ID, model.StatusProcessing)
	ps.Publish(t.Context(), job.ID, model.StatusEvent{Status: model.StatusProcessing, WorkerID: 1})

	time.Sleep(50 * time.Millisecond)

	qc.SetStatus(t.Context(), job.ID, model.StatusDone)
	qc.SetResult(t.Context(), job.ID, model.Result{ID: job.ID, Status: model.StatusDone, Sorted: true})
	ps.Publish(t.Context(), job.ID, model.StatusEvent{Status: model.StatusDone, Sorted: true})

	events := collectSSEEvents(t, resp, 5*time.Second)
	var gotStatus, gotResult, gotClose bool
	for _, ev := range events {
		if ev.Event == "status" {
			gotStatus = true
		}
		if ev.Event == "result" && strings.Contains(ev.Data, `"done"`) {
			gotResult = true
		}
		if ev.Event == "close" {
			gotClose = true
		}
	}
	if !gotStatus {
		t.Fatal("expected at least one status event")
	}
	if !gotResult {
		t.Fatal("expected result event")
	}
	if !gotClose {
		t.Fatal("expected close event")
	}
}
