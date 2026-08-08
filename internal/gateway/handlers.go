package gateway

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"sorted/internal/model"
	"sorted/parser"
)

func newUUID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

type sortRequest struct {
	List  []json.RawMessage `json:"list"`
	Order string            `json:"order"`
}

func (g *Gateway) submitHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	ct := r.Header.Get("Content-Type")

	var rawList []string
	var order string
	var err error

	if strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
		rawList, order, err = parseForm(r)
	} else {
		rawList, order, err = parseJSON(r)
	}
	if err != nil {
		if strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<div class="result-card error"><strong>Error:</strong> %s</div>`, htmlEscape(err.Error()))
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return
	}

	id := newUUID()
	listContent := strings.Join(rawList, "\n")

	ctx := r.Context()
	if err := g.storage.PutList(ctx, id, []byte(listContent)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "storage error"})
		return
	}

	job := model.Job{
		ID:          id,
		BucketKey:   "lists/" + id,
		Order:       order,
		SubmittedAt: time.Now(),
	}
	g.queue.SetStatus(ctx, id, model.StatusQueued)
	if err := g.queue.Push(ctx, job); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "queue error"})
		return
	}

	if strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<div id="result" hx-ext="sse" sse-connect="/is-sorted/%s/events" sse-swap="result" sse-close="close">`+
			`<div class="result-card processing">Queued...</div>`+
			`</div>`, id)
	} else {
		writeJSON(w, http.StatusAccepted, map[string]string{"id": id})
	}
}

func (g *Gateway) statusHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := g.queue.GetResult(r.Context(), id)
	if err != nil {
		status, err2 := g.queue.GetStatus(r.Context(), id)
		if err2 != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"id": id, "status": status})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (g *Gateway) uploadHandler(w http.ResponseWriter, r *http.Request) {
	id := newUUID()
	url, err := g.storage.PresignPut(r.Context(), id, presignTTL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "presign error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id, "upload_url": url})
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
	if body.Order != "asc" && body.Order != "desc" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "order must be asc or desc"})
		return
	}

	ctx := r.Context()
	job := model.Job{
		ID:          id,
		BucketKey:   "lists/" + id,
		Order:       body.Order,
		SubmittedAt: time.Now(),
	}
	g.queue.SetStatus(ctx, id, model.StatusQueued)
	if err := g.queue.Push(ctx, job); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "queue error"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"id": id})
}

func (g *Gateway) countHandler(w http.ResponseWriter, r *http.Request) {
	total, sorted, notSorted, _ := g.counter.Values(r.Context())
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, countHTML(total, sorted, notSorted))
}

func (g *Gateway) activityHandler(w http.ResponseWriter, r *http.Request) {
	entries, _ := g.activity.Recent(r.Context())
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, renderActivity(entries))
}

func parseJSON(r *http.Request) ([]string, string, error) {
	var req sortRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, "", fmt.Errorf("invalid JSON: %w", err)
	}
	if len(req.List) == 0 {
		return nil, "", fmt.Errorf("list is required")
	}
	order := req.Order
	if order == "" {
		order = "asc"
	}
	if order != "asc" && order != "desc" {
		return nil, "", fmt.Errorf("order must be asc or desc")
	}
	rawList := make([]string, len(req.List))
	for i, raw := range req.List {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			rawList[i] = strings.TrimSpace(string(raw))
		} else {
			rawList[i] = s
		}
		if _, err := parser.ParseValue(rawList[i]); err != nil {
			return nil, "", fmt.Errorf("cannot parse %q: %w", rawList[i], err)
		}
	}
	return rawList, order, nil
}

func parseForm(r *http.Request) ([]string, string, error) {
	if err := r.ParseForm(); err != nil {
		return nil, "", err
	}
	listStr := r.FormValue("list")
	order := r.FormValue("order")
	if order == "" {
		order = "asc"
	}
	if order != "asc" && order != "desc" {
		return nil, "", fmt.Errorf("order must be asc or desc")
	}
	lines := strings.Split(listStr, "\n")
	var rawList []string
	for _, line := range lines {
		parts := parser.SplitBracketAware(line, ',')
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if _, err := parser.ParseValue(p); err != nil {
				return nil, "", fmt.Errorf("cannot parse %q: %w", p, err)
			}
			rawList = append(rawList, p)
		}
	}
	if len(rawList) == 0 {
		return nil, "", fmt.Errorf("list is required")
	}
	return rawList, order, nil
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}
