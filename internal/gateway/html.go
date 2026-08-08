package gateway

import (
	"fmt"
	"strings"

	"sorted/internal/activity"
	"sorted/internal/counter"
	"sorted/internal/model"
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

func renderActivity(entries []model.ActivityEntry) string {
	if len(entries) == 0 {
		return `<div class="activity-empty">No checks yet</div>`
	}
	var b strings.Builder
	for _, e := range entries {
		class := "activity-entry"
		icon := "x"
		if e.Sorted {
			class += " sorted"
			icon = "check"
		}
		b.WriteString(fmt.Sprintf(`<div class="%s"><span class="activity-icon %s"></span>`, class, icon))
		b.WriteString(fmt.Sprintf(`<span class="activity-list">%s</span>`, activity.FormatList(e.List)))
		b.WriteString(fmt.Sprintf(`<span class="activity-meta">%s &middot; %s</span>`,
			activity.OrderLabel(e.Order), activity.TimeAgo(e.At)))
		b.WriteString(`</div>`)
	}
	return b.String()
}

func countHTML(total, sorted, notSorted int64) string {
	return fmt.Sprintf(`%s<div id="sorted-count-display" hx-swap-oob="innerHTML">%s</div><div id="not-sorted-count-display" hx-swap-oob="innerHTML">%s</div>`,
		counter.FormatCount(total),
		counter.FormatCount(sorted),
		counter.FormatCount(notSorted))
}
