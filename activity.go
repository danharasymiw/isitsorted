package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type activityEntry struct {
	at     time.Time
	sorted bool
	order  string
	list   []string
}

type activityFile struct {
	At     time.Time `json:"at"`
	Sorted bool      `json:"sorted"`
	Order  string    `json:"order"`
	List   []string  `json:"list"`
}

type activityLog struct {
	mu      sync.Mutex
	entries []activityEntry
	max     int
	path    string
}

func newActivityLog(max int, path string) *activityLog {
	a := &activityLog{max: max, path: path}
	if path == "" {
		return a
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("activity: load: %v", err)
		}
		return a
	}
	var entries []activityFile
	if err := json.Unmarshal(data, &entries); err != nil {
		log.Printf("activity: parse: %v", err)
		return a
	}
	for _, e := range entries {
		a.entries = append(a.entries, activityEntry{
			at:     e.At,
			sorted: e.Sorted,
			order:  e.Order,
			list:   e.List,
		})
	}
	return a
}

func (a *activityLog) add(sorted bool, order string, list []string) {
	if a.max == 0 {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	cp := make([]string, len(list))
	copy(cp, list)
	a.entries = append(a.entries, activityEntry{at: time.Now(), sorted: sorted, order: order, list: cp})
	if len(a.entries) > a.max {
		a.entries = a.entries[len(a.entries)-a.max:]
	}
	a.save()
}

func (a *activityLog) save() {
	if a.path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(a.path), 0755); err != nil {
		log.Printf("activity: mkdir: %v", err)
		return
	}
	out := make([]activityFile, len(a.entries))
	for i, e := range a.entries {
		out[i] = activityFile{At: e.at, Sorted: e.sorted, Order: e.order, List: e.list}
	}
	data, _ := json.Marshal(out)
	tmp := a.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		log.Printf("activity: write: %v", err)
		return
	}
	if err := os.Rename(tmp, a.path); err != nil {
		log.Printf("activity: rename: %v", err)
	}
}

func (a *activityLog) recent() []activityEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]activityEntry, len(a.entries))
	copy(out, a.entries)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	if d < 5*time.Second {
		return "just now"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh ago", int(d.Hours()))
}

func formatList(list []string) string {
	return "[" + strings.Join(list, ", ") + "]"
}

func orderLabel(order string) string {
	if order == "desc" {
		return "desc"
	}
	return "asc"
}

func renderActivity(entries []activityEntry) string {
	if len(entries) == 0 {
		return `<p class="activity-empty">No checks yet</p>`
	}
	var b strings.Builder
	for _, e := range entries {
		icon, label, cls := "✗", "not sorted", "no"
		if e.sorted {
			icon, label, cls = "✓", "sorted", "yes"
		}
		fmt.Fprintf(&b, `<div class="activity-entry %s"><span class="activity-icon">%s</span> <span class="activity-label">%s</span><span class="activity-order">%s</span><span class="activity-list">%s</span><span class="activity-time">%s</span></div>`,
			cls, icon, label, orderLabel(e.order), formatList(e.list), timeAgo(e.at))
	}
	return b.String()
}

func activityHandler(act *activityLog) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(renderActivity(act.recent())))
	}
}
