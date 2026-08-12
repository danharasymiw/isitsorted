# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

"Is It Sorted?" (isitsorted.ca) — a web service that checks whether a user-submitted list of values is sorted. Deployed on Railway.

## Build & Run

```bash
# Build all three binaries
go build -o gateway ./cmd/gateway
go build -o jobservice ./cmd/jobservice
go build -o worker ./cmd/worker

# Run (requires Redis + S3-compatible storage; see env vars below)
./jobservice   # port 8081
./gateway      # port 8080, requires JOB_SERVICE_URL=http://localhost:8081
./worker       # port 8082 (health only)
```

## Testing

```bash
# Unit tests (need Redis + MinIO/S3 running)
go test ./...

# Single package
go test ./parser/
go test ./cmd/worker/

# Integration tests (build tag: integration)
go test -tags integration ./...

# Both unit and integration run with -p 1 in CI
go test -p 1 ./... -count=1
```

Tests require live Redis and S3 — no mocks. Set these env vars:
`REDIS_URL`, `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_USE_PATH_STYLE=true`

For local MinIO: endpoint `http://localhost:9000`, creds `minioadmin/minioadmin`.

## Lint

```bash
go vet ./...
golangci-lint run    # uses .golangci.yml (gofmt formatter only)
```

## Architecture

Three-service architecture communicating via Redis and S3:

**Gateway** (`cmd/gateway`) — Public HTTP server. Renders HTML/HTMX pages from embedded static files (`cmd/gateway/static/`). Proxies all job operations to the job service via `JobClient`. Has a host-based router: `status.*` subdomains serve a status page, everything else serves the app. Holds no direct Redis/S3 dependency.

**Job Service** (`cmd/jobservice`) — Internal API. Accepts job submissions (`POST /jobs`), validates/parses input using the `parser` package, stores list data in S3, enqueues jobs in Redis, and exposes status polling + SSE streams. Also handles presigned upload URLs (`POST /uploads`) and serves stats (counter + activity feed).

**Worker** (`cmd/worker`) — Background processor. Pops jobs from the Redis queue, reads list content from S3, parses values with the `parser` package, runs the sort check (`Check()` in `sortcheck.go`), and publishes results via Redis pub/sub. Periodically snapshots counter/activity state to S3 and cleans up old bucket objects.

**Data flow:** Client → Gateway → Job Service → Redis queue → Worker → Redis (result/pub/sub) + S3 (result). Gateway SSE proxies Job Service SSE which subscribes to Redis pub/sub.

### Key Packages

- `parser/` — Value parsing: integers, rationals, floats, math expressions, roman numerals, braille numbers, mathematical constants (π, e, φ), intervals, sets (`{1,3,7}`), infinity (multilingual), emoji numbers, and multi-language number words. `ParseValue()` is the entry point. Values use `*big.Rat` for arbitrary-precision comparison. Values can be points, ranges (min/max), discrete sets, or ±∞.
- `internal/queue/` — Redis-backed job queue (LPUSH/BRPOP) + status/result key-value storage with TTL.
- `internal/pubsub/` — Redis pub/sub wrapper for real-time SSE event broadcasting.
- `internal/storage/` — S3 client for lists, results, and state snapshots. Key prefixes: `lists/`, `results/`, `state/`.
- `internal/counter/` — Redis-backed sorted/not-sorted counters.
- `internal/activity/` — Redis-backed recent activity feed.
- `internal/model/` — Shared types: `Job`, `Result`, `StatusEvent`, `ActivityEntry`.

### Sort Check Semantics

In `cmd/worker/sortcheck.go`: continuous ranges use forall semantics (every point must satisfy ordering), discrete sets use exists semantics (at least one point must fit, and that point becomes the new reference). Infinities compare as -∞ < finite < +∞.
