package main

import (
	"fmt"
	"strings"
	"time"
)

func resultHTML(sorted bool) string {
	if sorted {
		return `<div class="result-card yes"><span class="result-icon">` +
			`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>` +
			`</span><div><strong>Yes, it's sorted!</strong></div></div>`
	}
	return `<div class="result-card no"><span class="result-icon">` +
		`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>` +
		`</span><div><strong>Nope, not sorted.</strong></div></div>`
}

func renderActivity(entries []ActivityEntry) string {
	if len(entries) == 0 {
		return `<div class="activity-empty">No checks yet</div>`
	}
	var b strings.Builder
	for _, e := range entries {
		class := "activity-entry"
		icon := "✗"
		if e.Sorted {
			class += " sorted"
			icon = "✓"
		}
		_, _ = fmt.Fprintf(&b, `<div class="%s">`, class)
		_, _ = fmt.Fprintf(&b, `<span class="activity-list">[%s]</span>`, strings.Join(e.List, ", "))
		_, _ = fmt.Fprintf(&b, `<span class="activity-meta"><span class="activity-icon">%s</span> %s &middot; %s</span>`,
			icon, orderLabel(e.Order), timeAgo(e.At))
		b.WriteString(`</div>`)
	}
	return b.String()
}

func countHTML(total, sorted, notSorted int64) string {
	return fmt.Sprintf(`%s<div id="sorted-count-display" hx-swap-oob="innerHTML">%s</div><div id="not-sorted-count-display" hx-swap-oob="innerHTML">%s</div>`,
		formatCount(total),
		formatCount(sorted),
		formatCount(notSorted))
}

func formatCount(n int64) string {
	s := fmt.Sprintf("%d", n)
	if n < 0 {
		return s
	}
	var b strings.Builder
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(ch)
	}
	return b.String()
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Second:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func orderLabel(order string) string {
	if order == "desc" {
		return "descending"
	}
	return "ascending"
}
