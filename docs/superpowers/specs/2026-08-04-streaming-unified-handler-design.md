# Streaming Unified Handler Design

## Problem

The backend has three separate POST handlers (`/is-sorted`, `/check`, `/async/is-sorted`) that all do the same thing — parse a list, determine order, check if sorted. They buffer the entire list into memory before processing. Sort-checking only needs the previous element to compare against the current one, making it a natural fit for streaming (O(1) memory instead of O(n)).

## Design

### Unified Endpoint and Routing

Replace the three separate POST handlers with:

- **`POST /is-sorted`** — single sync endpoint. Dispatches on `Content-Type`:
  - `application/json` → JSON adapter parses input, JSON response (`{"sorted": true/false}`)
  - `application/x-www-form-urlencoded` → form adapter parses input, HTML response (result card + OOB swaps for counters and activity feed)
- **`POST /async/is-sorted`** — stays separate (different response semantics: 202 + job polling). Shares the same JSON parsing logic.
- **`GET /async/is-sorted/{id}`** — no change.

`POST /check` is removed. The HTMX form changes `hx-post` to `/is-sorted`.

### Streaming Sort Checker

Replace `check(list []*parser.Value, order string) bool` with a stateful `sortChecker`:

```go
type sortChecker struct {
    asc  bool
    prev *big.Rat
    done bool
}

func newSortChecker(order string) *sortChecker
func (sc *sortChecker) next(v *parser.Value) bool
```

The caller feeds values one at a time. `next()` returns false on the first out-of-order pair, and the caller can stop immediately without parsing the rest of the input.

### Input Adapters

**JSON adapter**: Uses `json.Decoder.Token()` to walk the request body token-by-token. Walks the top-level object keys, buffering only the `"order"` string (tiny). When it hits `"list"`, streams array elements one at a time, decoding each via `parseRaw()` and feeding it to the sort checker. If `"order"` hasn't appeared before `"list"`, we buffer the first element, finish reading the rest of the top-level keys to find `"order"` (or default to `"asc"`), then initialize the checker and resume streaming the array from the second element onward.

**Form adapter**: Calls `r.ParseForm()` (buffers the form body — fine, it's small), gets the `"list"` and `"order"` fields, splits tokens via `SplitBracketAware`, and yields them one at a time through `parser.ParseValue()`.

Both adapters collect raw string representations as they iterate, for the activity log.

### Response Formatting and Activity Log

After the streaming check completes, the handler formats the response based on the original Content-Type:

- **JSON**: `{"sorted": true/false}`
- **Form/HTML**: Result card HTML + OOB swaps for counter displays and activity feed

The activity log gets fed from both content types. The raw strings collected during iteration are passed to `act.add()`.

### Consolidated Counter Endpoint

Replace `GET /count`, `GET /count/sorted`, `GET /count/not-sorted` with a single `GET /count` that returns the total count as the primary content plus the sorted and not-sorted counts as HTMX OOB swaps:

```html
<span>1,234</span>
<div id="sorted-count-display" hx-swap-oob="innerHTML">987</div>
<div id="not-sorted-count-display" hx-swap-oob="innerHTML">247</div>
```

The frontend changes from three separate `hx-get` attributes to a single one on the total count element.

### Async Endpoint

`POST /async/is-sorted` shares the JSON adapter for parsing but buffers the parsed values into a slice before spawning the goroutine — the request body is tied to the HTTP request lifecycle and is gone after the handler returns. The goroutine feeds the buffered values through the same `sortChecker`. Activity log and counter updates remain in the goroutine.

`GET /async/is-sorted/{id}` is unchanged.

## File Changes

| File | Change |
|------|--------|
| `check.go` | Replace `check(list, order)` with `sortChecker` struct + `next()` method |
| `handler.go` | Unified handler dispatching on Content-Type. Remove `isSortedHandler`, `checkFormHandler`. Remove `sortRequest` struct. Add JSON adapter, form adapter. |
| `async.go` | Reuse JSON adapter to parse input, buffer into slice, feed `sortChecker` in goroutine |
| `counter.go` | Remove `sortedCountHandler`, `notSortedCountHandler`. Single `countHandler` returns OOB swaps. |
| `main.go` | Remove `/check`, `/count/sorted`, `/count/not-sorted` routes. |
| `static/index.html` | Form posts to `/is-sorted`. Single `hx-get="/count"` on total counter element. |
| `handler_test.go` | Form tests post to `/is-sorted`, JSON tests stay as-is. |
| `async_test.go` | Minimal changes — same endpoint, same behavior. |
| `check_test.go` | Update to use `sortChecker` interface. |

No changes to `parser/`, `activity.go`, or `ratelimit.go`.
