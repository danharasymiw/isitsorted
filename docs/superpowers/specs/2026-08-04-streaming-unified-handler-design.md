# Unified Handler Design

## Problem

The backend has three separate POST handlers (`/is-sorted`, `/check`, `/async/is-sorted`) that all do the same thing — parse a list, determine order, check if sorted. The duplication is unnecessary. Similarly, three counter endpoints (`/count`, `/count/sorted`, `/count/not-sorted`) each return a single number when one endpoint could return all three.

## Design

### Unified Endpoint and Routing

Replace the separate POST handlers with:

- **`POST /is-sorted`** — single sync endpoint. Dispatches on `Content-Type`:
  - `application/json` → JSON parsing, JSON response (`{"sorted": true/false}`)
  - `application/x-www-form-urlencoded` → form parsing, HTML response (result card + OOB swaps for counters and activity feed)
- **`POST /async/is-sorted`** — stays separate (different response semantics: 202 + job polling). Shares the same JSON parsing logic.
- **`GET /async/is-sorted/{id}`** — no change.

`POST /check` is removed. The HTMX form changes `hx-post` to `/is-sorted`.

### Input Parsing

**JSON path**: Decodes the body into `sortRequest` (same as today), parses each element via `parseRaw()`, passes the slice to `check()`.

**Form path**: Calls `r.ParseForm()`, reads `"list"` and `"order"` fields, splits tokens via `SplitBracketAware`, parses each via `parser.ParseValue()`, passes the slice to `check()`.

Both paths collect raw string representations for the activity log. `check()` remains unchanged — takes a `[]*parser.Value` slice and an order string.

### Response Formatting and Activity Log

After the check, the handler formats the response based on the original Content-Type:

- **JSON**: `{"sorted": true/false}`
- **Form/HTML**: Result card HTML + OOB swaps for counter displays and activity feed

The activity log gets fed from both content types.

### Consolidated Counter Endpoint

Replace `GET /count`, `GET /count/sorted`, `GET /count/not-sorted` with a single `GET /count` that returns the total count as the primary content plus the sorted and not-sorted counts as HTMX OOB swaps:

```html
<span>1,234</span>
<div id="sorted-count-display" hx-swap-oob="innerHTML">987</div>
<div id="not-sorted-count-display" hx-swap-oob="innerHTML">247</div>
```

The frontend changes from three separate `hx-get` attributes to a single one on the total count element.

### Async Endpoint

`POST /async/is-sorted` shares the same JSON parsing as the sync path but spawns a goroutine for the check. Activity log and counter updates remain in the goroutine.

`GET /async/is-sorted/{id}` is unchanged.

## File Changes

| File | Change |
|------|--------|
| `handler.go` | Unified handler dispatching on Content-Type. Remove `isSortedHandler`, `checkFormHandler`. Single `isSortedHandler` that handles both. |
| `async.go` | Reuse shared JSON parsing logic. Remove duplicated `sortRequest` decode. |
| `counter.go` | Remove `sortedCountHandler`, `notSortedCountHandler`. Single `countHandler` returns OOB swaps. |
| `main.go` | Remove `/check`, `/count/sorted`, `/count/not-sorted` routes. |
| `static/index.html` | Form posts to `/is-sorted`. Single `hx-get="/count"` on total counter element. |
| `handler_test.go` | Form tests post to `/is-sorted`, JSON tests stay as-is. |
| `async_test.go` | Minimal changes — same endpoint, same behavior. |

No changes to `check.go`, `check_test.go`, `parser/`, `activity.go`, or `ratelimit.go`.
