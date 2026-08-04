package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sorted/parser"
	"strings"
)

type sortRequest struct {
	List  []json.RawMessage `json:"list"`
	Order string            `json:"order"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// parseRaw converts a single JSON value (number or string) to *parser.Value.
func parseRaw(raw json.RawMessage) (*parser.Value, error) {
	s := strings.TrimSpace(string(raw))

	// JSON string → unquote, then parse the inner text.
	if len(s) >= 2 && s[0] == '"' {
		var unquoted string
		if err := json.Unmarshal(raw, &unquoted); err != nil {
			return nil, err
		}
		return parser.ParseValue(unquoted)
	}

	// JSON number → parse the literal digits directly so we keep full
	// precision for integers larger than int64.
	return parser.ParseValue(s)
}

func isSortedHandler(ctr *counter, act *activityLog) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		ct := r.Header.Get("Content-Type")
		if strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
			handleForm(w, r, ctr, act)
		} else {
			handleJSON(w, r, ctr, act)
		}
	}
}

// parseJSONBody decodes a sortRequest from the request body and validates
// it, returning the parsed values, their raw string forms, and the sort
// order to use.
func parseJSONBody(r *http.Request) (list []*parser.Value, rawList []string, order string, err error) {
	var req sortRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, nil, "", err
	}
	if req.List == nil {
		return nil, nil, "", fmt.Errorf("list is required")
	}
	order = req.Order
	if order == "" {
		order = "asc"
	}
	if order != "asc" && order != "desc" {
		return nil, nil, "", fmt.Errorf(`order must be "asc" or "desc"`)
	}

	list = make([]*parser.Value, 0, len(req.List))
	rawList = make([]string, 0, len(req.List))
	for _, raw := range req.List {
		v, err := parseRaw(raw)
		if err != nil {
			return nil, nil, "", err
		}
		list = append(list, v)
		rawList = append(rawList, strings.TrimSpace(string(raw)))
	}
	return list, rawList, order, nil
}

func handleJSON(w http.ResponseWriter, r *http.Request, ctr *counter, act *activityLog) {
	list, rawList, order, err := parseJSONBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	sorted := check(list, order)
	ctr.increment(sorted)
	act.add(sorted, order, rawList)
	writeJSON(w, http.StatusOK, map[string]bool{"sorted": sorted})
}

func handleForm(w http.ResponseWriter, r *http.Request, ctr *counter, act *activityLog) {
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
	// Split on newlines first (blank lines are skipped), then on individual
	// commas within each line. Consecutive commas produce an empty field
	// which is an error; a trailing comma is silently ignored.
	lines := strings.FieldsFunc(raw, func(c rune) bool { return c == '\n' || c == '\r' })
	var tokens []string
	for _, line := range lines {
		fields := parser.SplitBracketAware(line, ',')
		// Drop trailing empty field from a trailing comma.
		if len(fields) > 0 && strings.TrimSpace(fields[len(fields)-1]) == "" {
			fields = fields[:len(fields)-1]
		}
		tokens = append(tokens, fields...)
	}

	var list []*parser.Value
	var rawList []string
	for _, p := range tokens {
		p = strings.TrimSpace(p)
		if p == "" {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<div class="result-card error"><span class="result-icon">!</span><div><strong>Invalid input</strong><p>Empty value — remove consecutive commas.</p></div></div>`))
			return
		}
		v, err := parser.ParseValue(p)
		if err != nil {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<div class="result-card error"><span class="result-icon">!</span><div><strong>Invalid input</strong><p>Could not parse &#34;` + htmlEscape(p) + `&#34; as a number.</p></div></div>`))
			return
		}
		list = append(list, v)
		rawList = append(rawList, p)
	}

	sorted := check(list, order)
	ctr.increment(sorted)
	act.add(sorted, order, rawList)
	oobCount := `<div id="count-display" hx-swap-oob="innerHTML">` + formatCount(ctr.value()) + `</div>`
	oobSorted := `<div id="sorted-count-display" hx-swap-oob="innerHTML">` + formatCount(ctr.sortedValue()) + `</div>`
	oobNotSorted := `<div id="not-sorted-count-display" hx-swap-oob="innerHTML">` + formatCount(ctr.notSortedValue()) + `</div>`
	oobActivity := `<div id="activity-feed" hx-swap-oob="innerHTML">` + renderActivity(act.recent()) + `</div>`
	w.Header().Set("Content-Type", "text/html")
	if sorted {
		w.Write([]byte(`<div class="result-card yes"><span class="result-icon">✓</span><div><strong>Yes, it&#39;s sorted</strong></div></div>` + oobCount + oobSorted + oobNotSorted + oobActivity))
	} else {
		w.Write([]byte(`<div class="result-card no"><span class="result-icon">✗</span><div><strong>No, it&#39;s not sorted</strong></div></div>` + oobCount + oobSorted + oobNotSorted + oobActivity))
	}
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}
