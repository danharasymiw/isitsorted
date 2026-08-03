# UI Redesign & API Docs — Design Spec

**Date:** 2026-08-03  
**Status:** Approved

## Overview

Redesign `static/index.html` with a clean, modern aesthetic and add an inline API reference section below the form. No external CSS frameworks or fonts — pure inline styles in the HTML file.

## Visual Design

**Aesthetic:** Clean & Modern  
**Audience:** General public — approachable, trustworthy  
**Color palette:**
- Page background: `#f1f5f9` (slate-100) — softer than white
- Surface/card: `#ffffff`
- Primary text: `#0f172a` (slate-900)
- Secondary text: `#64748b` (slate-500)
- Muted text: `#94a3b8` (slate-400)
- Border: `#e2e8f0` (slate-200)
- Accent (button, radio active): `#4f46e5` (indigo-600)
- Result yes: green (`#15803d` text on `#f0fdf4` bg, `#bbf7d0` border)
- Result no: red (`#dc2626` text on `#fef2f2` bg, `#fecaca` border)

**Typography:** System font stack (`-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif`). Code blocks use monospace.

**Layout:** Single column, max-width 520px, centered. Page padding `60px auto`.

## Form Section

Replaces the current plain form with:

- Overline label: `IS IT SORTED?` in small uppercase slate-400
- Heading: `Check your list` — 26px, weight 800, tight letter-spacing
- Subtitle: existing copy, slate-500
- Textarea: white background, `#e2e8f0` border, 10px border-radius, subtle box-shadow, `box-sizing: border-box`, full width
- Radios + button row: flex, space-between. Indigo filled circle for selected radio state.
- Button: indigo background, white text, 8px radius, slight box-shadow, `Check →` label

**Result states** (rendered into `#result` by HTMX, replacing `.yes`/`.no` paragraph classes):

- Sorted: green card (`#f0fdf4` bg, `#bbf7d0` border), ✓ icon, "Yes, it's sorted" in `#15803d`
- Not sorted: red card (`#fef2f2` bg, `#fecaca` border), ✗ icon, "No, it's not sorted" in `#dc2626`
- Error: red card, error message text

The server-side `checkFormHandler` currently returns bare `<p class="result yes">` / `<p class="result no">` HTML fragments. These must be updated to return card HTML:

- **Sorted:** `<div class="result-card yes"><span class="result-icon">✓</span><div><strong>Yes, it's sorted</strong></div></div>`
- **Not sorted:** `<div class="result-card no"><span class="result-icon">✗</span><div><strong>No, it's not sorted</strong></div></div>`
- **Error:** `<div class="result-card error"><span class="result-icon">!</span><div><strong>Invalid input</strong><p>{message}</p></div></div>`

Corresponding CSS classes (`result-card`, `yes`, `no`, `error`, `result-icon`) are defined in `index.html`.

## API Docs Section

Separated from the form by a full-width `#e2e8f0` horizontal rule, scrolls below.

**Contents:**

1. **Overline + heading:** `JSON API` overline, `POST /is-sorted` as h2
2. **Description:** one sentence description + rate limit note (20 req/min per IP)
3. **Request block:** dark code block (`#0f172a` bg) showing method, Content-Type header, and example JSON body with syntax highlighting via inline `<span>` colors
4. **Response — sorted:** `200 OK` + `{"sorted": true}`
5. **Response — error:** `400 Bad Request` + `{"error": "..."}`
6. **curl example:** dark code block with a working curl snippet

Code blocks use monospace font, `#0f172a` background, 8px border-radius.

## Files Changed

| File | Change |
|------|--------|
| `static/index.html` | Full rewrite — new styles + API docs section |
| `handler.go` | Update `checkFormHandler` HTML fragments to match new result card markup |

## Out of Scope

- No JavaScript beyond the existing htmx CDN script
- No dark mode toggle
- No copy-to-clipboard buttons on code blocks
- No separate `/docs` route
