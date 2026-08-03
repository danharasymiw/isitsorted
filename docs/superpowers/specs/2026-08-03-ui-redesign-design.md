# UI Redesign & Parody Landing Page — Design Spec

**Date:** 2026-08-03  
**Status:** Approved  
**Domain:** isitsorted.ca

## Overview

Redesign `static/index.html` as a parody SaaS landing page. Looks like a polished, professional product site; the humor is entirely in the copy. No external CSS frameworks or fonts — pure inline styles.

## Visual Design

**Aesthetic:** Clean & Modern  
**Color palette:**
- Page background: `#f1f5f9` (slate-100)
- Surface/card: `#ffffff`
- Primary text: `#0f172a` (slate-900)
- Secondary text: `#64748b` (slate-500)
- Muted/label text: `#94a3b8` (slate-400)
- Border: `#e2e8f0` (slate-200)
- Accent: `#4f46e5` (indigo-600)
- Result yes: `#15803d` text, `#f0fdf4` bg, `#bbf7d0` border
- Result no: `#dc2626` text, `#fef2f2` bg, `#fecaca` border

**Typography:** System font stack (`-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif`). Code blocks use monospace.

**Layout:** Single column, max-width 520px, centered, `60px auto` padding.

---

## Page Sections (top to bottom)

### 1. Hero

```
isitsorted.ca                     ← indigo overline, uppercase, tight tracking
Is It Sorted                      ← h1, 40px, weight 900, -2px letter-spacing
Is It Sorted as a Service. (IISaaS)  ← tagline, 13px, weight 600, slate-600
An API that tells you if a list of integers is sorted.  ← description, slate-500
It does not sort the list.         ← italic, slate-400
[Try it free →]  [View API docs]   ← indigo primary + white secondary buttons
```

### 2. Stats Bar

Five columns, white background, separated by a bottom border. Four are joke stats; one is the real live check count (displayed in indigo `#4f46e5` to subtly distinguish it):

| Stat | Label | Notes |
|------|-------|-------|
| Mostly | Uptime | joke |
| < 2ms | p99 latency | joke |
| {count} | Lists checked | **real** — loaded via `hx-get="/count" hx-trigger="load"`, indigo color |
| 1 | Developer trusts us | joke |
| 0 | Data breaches | joke |

### 3. Features

Centered section, white background. Section label: "Features" (overline style).  
Checkmarks are green (`#16a34a`), list is `display:inline-block; text-align:left` centered within the section:

- ✓ Detects ascending order
- ✓ Detects descending order
- ✓ Returns true or false
- ✓ Handles empty lists

### 4. Live Demo (Form)

Section label: "Live demo" (overline). Heading: "Try it yourself".

- Textarea: white, slate border, 10px radius, subtle shadow
- Radios: indigo filled circle for selected state
- Button: `Check →`, indigo, 8px radius

**Result states** (HTMX swap into `#result`):

- Sorted: `<div class="result-card yes"><span class="result-icon">✓</span><div><strong>Yes, it's sorted</strong></div></div>`
- Not sorted: `<div class="result-card no"><span class="result-icon">✗</span><div><strong>No, it's not sorted</strong></div></div>`
- Error: `<div class="result-card error"><span class="result-icon">!</span><div><strong>Invalid input</strong><p>{message}</p></div></div>`

CSS classes `result-card`, `yes`, `no`, `error`, `result-icon` defined in `index.html`.

### 5. Testimonials

Section label: "What our users say". Three cards, white, slate border, 10px radius.

1. > "JavaScript's `.sort()` compares everything as strings by default. `[10, 1, 2].sort()` returns `[1, 10, 2]`. I know this. I still double check now."  
   **@sk_dev** · 4 stars

2. > "not sure why this exists but it works"  
   **anonymous** · 5 stars

3. > "correctly identified my unsorted list. not happy about it."  
   **@priya_dev** · 3 stars

### 6. FAQ

Section label: "Frequently asked questions". Q&A pairs, no borders — just question (bold, slate-900) and answer (slate-500) stacked.

| Question | Answer |
|----------|--------|
| Does it sort the list? | No. |
| Why would I use this? | You've got a list. You want to know if it's sorted. You're already here. |
| Is there a paid plan? | No. 20 requests/minute, free, forever. |
| What's the SLA? | It's running on a $6/mo VPS. You can work it out. |

### 7. API Docs

Section labels: "JSON API" overline, `POST /is-sorted` as h2.  
Description: "Rate limited to 20 requests/minute per IP."

Two dark code blocks (`#0f172a` bg, monospace, 8px radius):

1. Request — method + Content-Type header + example JSON body
2. curl example — `curl -X POST https://isitsorted.ca/is-sorted ...`

### 8. Footer

Two-column flex, full width:
- Left: `© 2026 isitsorted.ca — All rights reserved. Patent pending.`
- Right: `Built on a $6/mo VPS.`

---

## Counter Feature

A persistent check counter increments on every successful request to `/check` or `/is-sorted`.

**Backend (`counter.go` — new file):**
- Atomic `int64` counter in memory, backed by `data/count.json` on disk
- Load count from file at startup; write back after every increment
- File format: `{"count": 42}`
- New handler: `GET /count` returns an HTML fragment — just the formatted number (e.g. `3,847`) — for HTMX to swap in
- Counter increments in `checkFormHandler` and `isSortedHandler` on success

**Frontend (`index.html`):**
- Stats bar placeholder: `<span hx-get="/count" hx-trigger="load">—</span>`
- No polling — count is fetched once on page load

**Data directory:** `data/` — add to `.gitignore`.

---

## Files Changed

| File | Change |
|------|--------|
| `static/index.html` | Full rewrite — new styles, all landing page sections |
| `handler.go` | Update result card HTML fragments; call counter increment |
| `counter.go` | New — file-backed atomic counter + `GET /count` handler |
| `main.go` | Register `GET /count` route; load counter at startup |

## Out of Scope

- No JavaScript beyond the existing htmx CDN script
- No dark mode toggle
- No copy-to-clipboard on code blocks
- No separate `/docs` route
