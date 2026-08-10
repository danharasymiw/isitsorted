package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const maxBodySize = 1 << 20 // 1 MB

func (g *Gateway) submitHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	ct := r.Header.Get("Content-Type")
	isForm := strings.HasPrefix(ct, "application/x-www-form-urlencoded")

	var list []string
	var order string

	if isForm {
		if err := r.ParseForm(); err != nil {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<div class="result-card error"><strong>Error:</strong> %s</div>`, htmlEscape(err.Error()))
			return
		}
		listStr := r.FormValue("list")
		order = r.FormValue("order")
		if order == "" {
			order = "asc"
		}
		// Send the raw textarea content as a single string. The job service
		// handles bracket-aware comma splitting and validation via the parser.
		// We split on newlines only — commas within bracket expressions like
		// {1, 2, 3} must be preserved.
		lines := strings.Split(listStr, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				list = append(list, line)
			}
		}
		if len(list) == 0 {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<div class="result-card error"><strong>Error:</strong> list is required</div>`)
			return
		}
	} else {
		var req struct {
			List  []json.RawMessage `json:"list"`
			Order string            `json:"order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid JSON: %s", err)})
			return
		}
		order = req.Order
		list = make([]string, len(req.List))
		for i, raw := range req.List {
			var s string
			if err := json.Unmarshal(raw, &s); err != nil {
				list[i] = strings.TrimSpace(string(raw))
			} else {
				list[i] = s
			}
		}
	}

	status, body, err := g.client.SubmitJob(r.Context(), SubmitRequest{List: list, Order: order})
	if err != nil {
		if isForm {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<div class="result-card error"><strong>Error:</strong> service unavailable</div>`)
		} else {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "job service unavailable"})
		}
		return
	}

	if status >= 400 {
		if isForm {
			var errResp ErrorResponse
			json.Unmarshal(body, &errResp)
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<div class="result-card error"><strong>Error:</strong> %s</div>`, htmlEscape(errResp.Error))
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			w.Write(body)
		}
		return
	}

	var idResp IDResponse
	json.Unmarshal(body, &idResp)

	if isForm {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<div id="result" hx-ext="sse" sse-connect="/is-sorted/%s/events?format=html" sse-swap="status,result" sse-close="close">`+
			`<div class="result-card processing">Queued...</div>`+
			`</div>`, idResp.ID)
	} else {
		writeJSON(w, http.StatusAccepted, map[string]string{"id": idResp.ID})
	}
}

func (g *Gateway) statusHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	status, body, err := g.client.GetStatus(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "job service unavailable"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

func (g *Gateway) uploadHandler(w http.ResponseWriter, r *http.Request) {
	status, body, err := g.client.CreateUpload(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "job service unavailable"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

func (g *Gateway) uploadCheckHandler(w http.ResponseWriter, r *http.Request) {
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

	status, respBody, err := g.client.CheckUpload(r.Context(), id, body.Order)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "job service unavailable"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(respBody)
}

func (g *Gateway) countHandler(w http.ResponseWriter, r *http.Request) {
	_, body, err := g.client.GetCount(r.Context())
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, countHTML(0, 0, 0))
		return
	}
	var count CountResponse
	json.Unmarshal(body, &count)
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, countHTML(count.Total, count.Sorted, count.NotSorted))
}

func (g *Gateway) activityHandler(w http.ResponseWriter, r *http.Request) {
	_, body, err := g.client.GetActivity(r.Context())
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<div class="activity-empty">No checks yet</div>`)
		return
	}
	var activity ActivityResponse
	json.Unmarshal(body, &activity)
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, renderActivity(activity.Entries))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}
