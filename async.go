package main

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type job struct {
	mu      sync.RWMutex
	id      string
	status  string
	sorted  bool
	created time.Time
}

type jobStore struct {
	jobs sync.Map
}

func newJobStore() *jobStore {
	s := &jobStore{}
	go s.cleanup()
	return s
}

func (s *jobStore) create(id string) *job {
	j := &job{id: id, status: "processing", created: time.Now()}
	s.jobs.Store(id, j)
	return j
}

func (s *jobStore) get(id string) *job {
	v, ok := s.jobs.Load(id)
	if !ok {
		return nil
	}
	return v.(*job)
}

func (s *jobStore) cleanup() {
	for {
		time.Sleep(60 * time.Second)
		now := time.Now()
		s.jobs.Range(func(key, value any) bool {
			j := value.(*job)
			j.mu.RLock()
			age := now.Sub(j.created)
			j.mu.RUnlock()
			if age > 5*time.Minute {
				s.jobs.Delete(key)
			}
			return true
		})
	}
}

func newUUID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func asyncSubmitHandler(js *jobStore, ctr *counter, act *activityLog) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		list, rawList, order, err := parseJSONBody(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		j := js.create(newUUID())

		go func() {
			sorted := check(list, order)
			j.mu.Lock()
			j.status = "complete"
			j.sorted = sorted
			j.mu.Unlock()
			ctr.increment(sorted)
			act.add(sorted, order, rawList)
		}()

		writeJSON(w, http.StatusAccepted, map[string]string{"id": j.id})
	}
}

func asyncStatusHandler(js *jobStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		j := js.get(id)
		if j == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
			return
		}

		j.mu.RLock()
		status := j.status
		sorted := j.sorted
		j.mu.RUnlock()

		if status == "complete" {
			writeJSON(w, http.StatusOK, map[string]any{
				"id":     id,
				"status": status,
				"sorted": sorted,
			})
		} else {
			writeJSON(w, http.StatusOK, map[string]any{
				"id":     id,
				"status": status,
			})
		}
	}
}
