package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

type counter struct {
	mu   sync.Mutex
	n    int64
	path string
}

type counterFile struct {
	Count int64 `json:"count"`
}

func newCounter(path string) *counter {
	c := &counter{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("counter: load: %v", err)
		}
		return c
	}
	var f counterFile
	if err := json.Unmarshal(data, &f); err != nil {
		log.Printf("counter: parse: %v", err)
		return c
	}
	c.n = f.Count
	return c
}

func (c *counter) increment() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
	c.save()
}

func (c *counter) value() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func (c *counter) save() {
	if c.path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0755); err != nil {
		log.Printf("counter: mkdir: %v", err)
		return
	}
	data, _ := json.Marshal(counterFile{Count: c.n})
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		log.Printf("counter: write: %v", err)
		return
	}
	if err := os.Rename(tmp, c.path); err != nil {
		log.Printf("counter: rename: %v", err)
	}
}

func countHandler(ctr *counter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, formatCount(ctr.value()))
	}
}

func formatCount(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	result := make([]byte, 0, len(s)+len(s)/3)
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(ch))
	}
	return string(result)
}
