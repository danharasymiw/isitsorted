package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type sortRequest struct {
	List  []int  `json:"list"`
	Order string `json:"order"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func isSortedHandler(ctr *counter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		var req sortRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if req.List == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "list is required"})
			return
		}
		if req.Order == "" {
			req.Order = "asc"
		}
		if req.Order != "asc" && req.Order != "desc" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": `order must be "asc" or "desc"`})
			return
		}
		ctr.increment()
		writeJSON(w, http.StatusOK, map[string]bool{"sorted": check(req.List, req.Order)})
	}
}

func checkFormHandler(ctr *counter, act *activityLog) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		order := r.FormValue("order")
		if order == "" {
			order = "asc"
		}
		if order != "asc" && order != "desc" {
			http.Error(w, "invalid order", http.StatusBadRequest)
			return
		}

		raw := r.FormValue("list")
		parts := strings.FieldsFunc(raw, func(c rune) bool {
			return c == ',' || c == '\n' || c == '\r'
		})

		var list []int
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			n, err := strconv.Atoi(p)
			if err != nil {
				w.Header().Set("Content-Type", "text/html")
				w.Write([]byte(`<div class="result-card error"><span class="result-icon">!</span><div><strong>Invalid input</strong><p>All values must be integers.</p></div></div>`))
				return
			}
			list = append(list, n)
		}

		sorted := check(list, order)
		ctr.increment()
		act.add(sorted, list)
		n := ctr.value()
		oobCount := `<div id="count-display" hx-swap-oob="innerHTML">` + formatCount(n) + `</div>`
		oobActivity := `<div id="activity-feed" hx-swap-oob="innerHTML">` + renderActivity(act.recent()) + `</div>`
		w.Header().Set("Content-Type", "text/html")
		if sorted {
			w.Write([]byte(`<div class="result-card yes"><span class="result-icon">✓</span><div><strong>Yes, it&#39;s sorted</strong></div></div>` + oobCount + oobActivity))
		} else {
			w.Write([]byte(`<div class="result-card no"><span class="result-icon">✗</span><div><strong>No, it&#39;s not sorted</strong></div></div>` + oobCount + oobActivity))
		}
	}
}
