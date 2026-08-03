package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type activityEntry struct {
	at     time.Time
	sorted bool
	list   []int
}

type activityLog struct {
	mu      sync.Mutex
	entries []activityEntry
	max     int
}

func newActivityLog(max int) *activityLog {
	return &activityLog{max: max}
}

func (a *activityLog) add(sorted bool, list []int) {
	if a.max == 0 {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	cp := make([]int, len(list))
	copy(cp, list)
	a.entries = append(a.entries, activityEntry{at: time.Now(), sorted: sorted, list: cp})
	if len(a.entries) > a.max {
		a.entries = a.entries[len(a.entries)-a.max:]
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

func formatList(list []int) string {
	parts := make([]string, len(list))
	for i, n := range list {
		parts[i] = strconv.Itoa(n)
	}
	return "[" + strings.Join(parts, ", ") + "]"
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
		fmt.Fprintf(&b, `<div class="activity-entry %s"><span class="activity-icon">%s</span> <span class="activity-label">%s</span><span class="activity-list">%s</span><span class="activity-time">%s</span></div>`,
			cls, icon, label, formatList(e.list), timeAgo(e.at))
	}
	return b.String()
}

func activityHandler(act *activityLog) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(renderActivity(act.recent())))
	}
}
