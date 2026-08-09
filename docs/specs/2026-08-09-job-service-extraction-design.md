# Job Service Extraction

**Date:** 2026-08-09
**Status:** Approved

## Goal

Extract job lifecycle management from the gateway into a dedicated job service, giving the system proper service boundaries. The gateway becomes a thin HTTP proxy that serves the frontend and forwards all job operations to the job service via a REST API defined by an OpenAPI spec. The worker is unchanged.

## Architecture

Three services, one Redis instance, one S3-compatible bucket:

```
                                    ┌─────────────────┐
                                    │   Job Service    │
                              ┌────▶│  (internal API)  │◀────┐
                              │     │                  │     │
                              │     └────────┬─────────┘     │
                              │              │               │
                         HTTP │         Redis│& S3           │ Redis
                              │              │               │ (BRPOP)
                              │              ▼               │
┌──────────┐    HTTP    ┌─────┴──────┐  ┌─────────┐   ┌─────┴──────┐
│  Client  │───────────▶│  Gateway   │  │  Redis  │   │   Worker   │
│          │◀───────────│            │  │         │   │            │
└──────────┘    SSE     └────────────┘  └─────────┘   └────────────┘
```

**Gateway** — Thin HTTP proxy and static frontend server. No direct Redis or S3 access. All job operations go through the job service. In-memory rate limiting.

**Job Service** — Owns the entire job lifecycle. Exposes an OpenAPI-defined REST API consumed only by the gateway. Directly manages Redis (queue, status, pub/sub) and S3 (list storage, results, presigned URLs). Not exposed to the internet.

**Worker** — Largely unchanged. Pops from Redis queue, reads lists from S3, runs sort checks, publishes results. Backend peer to the job service — both access Redis and S3 directly. Gains a `/health` endpoint for status page monitoring.

## Data Flow

### Job submission (synchronous validation, async processing)

```
Client ──POST /is-sorted──▶ Gateway ──POST /jobs──▶ Job Service
                            (rate limit,            (parse, validate → 400 if bad)
                             format response)       (put list → S3)
                                                    (push job → Redis queue)
                                                    (set status → Redis)
                           ◀── 202 {id} ◀────────── 201 {id}
```

### SSE streaming

```
Client ──GET /is-sorted/{id}/events──▶ Gateway ──GET /jobs/{id}/events──▶ Job Service
                                       (re-emit SSE,                      (subscribe Redis
                                        format HTML                        pub/sub, emit
                                        or JSON)                           JSON SSE)
```

### Worker processing

```
Worker ──BRPOP──▶ Redis queue
       ──GET───▶ S3 (read list)
       ──sort check──
       ──SET───▶ Redis (result, status)
       ──PUT───▶ S3 (result)
       ──PUB───▶ Redis pubsub ──▶ Job Service SSE ──▶ Gateway SSE ──▶ Client
```

## Responsibility Mapping

### Gateway keeps

- HTTP routing and static frontend serving
- In-memory rate limiting (no Redis dependency)
- SSE streaming to clients (proxies JSON SSE from job service, re-emits as HTML or JSON based on client type)
- HTML rendering for HTMX responses (converts job service JSON into HTML fragments)
- Status page (host-routed on `status.*`, adds job service health check)

### Job service owns

- Job submission — validates input, writes list to S3, pushes to Redis queue, sets status
- Job status and result retrieval — reads from Redis
- SSE event stream — subscribes to Redis pub/sub, emits JSON-only SSE
- Presigned upload URL generation — owns the S3 client
- Counter values — reads from Redis
- Activity log — reads from Redis
- Health endpoint — pings Redis and S3

### Worker keeps (largely unchanged)

- Gains a `/health` endpoint (pings Redis and S3) for status page monitoring
- Pops from Redis queue directly
- Reads lists from S3, runs sort check
- Writes results to Redis and S3
- Publishes status events via Redis pub/sub
- Snapshot and cleanup background loops

## Package Structure

```
cmd/
  gateway/
    main.go           # server setup, routing, host router, status page
    handlers.go       # HTTP handlers that proxy to job service
    sse.go            # SSE proxy (consume from job service, re-emit to client)
    html.go           # HTML rendering for HTMX responses
    ratelimit.go      # in-memory rate limiter
    client.go         # HTTP client for calling job service API
    static/           # embedded frontend assets
    Dockerfile
    railway.toml
  jobservice/
    main.go           # server setup, routing
    handlers.go       # REST API handlers
    sse.go            # SSE endpoint (subscribes to Redis pubsub, emits JSON)
    Dockerfile
    railway.toml
  worker/
    main.go           # server setup
    worker.go         # job processing loop (moved from internal/worker/)
    sortcheck.go      # sort comparison logic (moved from internal/worker/)
    Dockerfile
    railway.toml
internal/
  model/              # shared data types (Job, Result, StatusEvent, ActivityEntry)
  queue/              # Redis list-based job queue + status/result storage
  pubsub/             # Redis pub/sub wrapper for status events
  storage/            # S3-compatible bucket client
  counter/            # Redis-backed atomic counters
  activity/           # Redis-backed capped activity log
```

### Deleted

- `internal/gateway/` — all gateway logic moves to `cmd/gateway/`
- `internal/worker/` — moves to `cmd/worker/`

### Shared packages (used by job service + worker)

`model`, `queue`, `pubsub`, `storage`, `counter`, `activity` remain in `internal/` because both the job service and worker import them.

## OpenAPI Spec

The job service exposes the following REST API. All request and response bodies are JSON. The gateway is the sole consumer.

### POST /jobs

Submit a sort-check job. The job service parses and validates the list synchronously — invalid input returns 400 before anything is queued.

**Request:**
```json
{
  "list": ["3", "1", "4", "1", "5"],
  "order": "asc"
}
```

**201 Created:**
```json
{ "id": "a1b2c3d4-..." }
```

**400 Bad Request:**
```json
{ "error": "cannot parse \"foo\": ..." }
```

### GET /jobs/{id}

Get the current status or result of a job.

**200 OK (pending):**
```json
{ "id": "...", "status": "queued" }
```

**200 OK (complete):**
```json
{ "id": "...", "status": "done", "sorted": true }
```

**200 OK (failed):**
```json
{ "id": "...", "status": "error", "error": "..." }
```

**404 Not Found:**
```json
{ "error": "job not found" }
```

### GET /jobs/{id}/events

SSE stream of job status updates. Always emits JSON events.

**Events:**
- `event: status` — `{"status": "queued"}` or `{"status": "processing", "worker_id": 3}`
- `event: result` — `{"status": "done", "sorted": true}` or `{"status": "error", "error": "..."}`
- `event: close` — terminal event

### POST /uploads

Generate a presigned S3 upload URL for direct client uploads.

**200 OK:**
```json
{ "id": "...", "upload_url": "https://..." }
```

### POST /uploads/{id}/check

Trigger a sort-check for a previously uploaded list. The job service reads the list from S3, validates it, and returns 400 if invalid.

**Request:**
```json
{ "order": "asc" }
```

**201 Created:**
```json
{ "id": "..." }
```

**400 Bad Request:**
```json
{ "error": "cannot parse \"foo\": ..." }
```

### GET /stats/count

Get sort-check counters.

**200 OK:**
```json
{ "total": 42, "sorted": 30, "not_sorted": 12 }
```

### GET /stats/activity

Get recent sort checks.

**200 OK:**
```json
{
  "entries": [
    {
      "at": "2026-08-09T14:30:00Z",
      "sorted": true,
      "order": "asc",
      "list": ["1", "2", "3"]
    }
  ]
}
```

### GET /health

Health check for monitoring.

**200 OK:**
```json
{
  "status": "healthy",
  "redis": "ok",
  "storage": "ok"
}
```

## Rate Limiting

The gateway uses an in-memory sliding window rate limiter (e.g. `golang.org/x/time/rate`). This replaces the current Redis-backed rate limiter. Trade-off: rate limit state is lost on gateway restart and is per-instance rather than shared. Both are acceptable for this application.

Applied only to `POST /is-sorted` on the gateway side, before proxying to the job service.

## Status Page

The host-routed status page (`status.*`) adds a health check for the job service via `GET /health`. Components displayed:

- API Gateway — always operational (if you can see the page, it's up)
- Job Service — based on `/health` response
- Worker — based on `/health` response (requires adding a health endpoint to worker)
- Redis — reported by job service health check
- Object Storage — reported by job service health check

The 90-day uptime history grid remains cosmetic (no persisted probe history). Real monitoring is a separate effort.

## Deployment

Each service gets its own `Dockerfile` and `railway.toml` under its `cmd/` directory. Railway watch paths ensure independent builds:

- `cmd/gateway/` watches: `cmd/gateway/**`, `go.mod`, `go.sum`
- `cmd/jobservice/` watches: `cmd/jobservice/**`, `internal/**`, `parser/**`, `go.mod`, `go.sum`
- `cmd/worker/` watches: `cmd/worker/**`, `internal/**`, `parser/**`, `go.mod`, `go.sum`

The gateway no longer rebuilds when `internal/` or `parser/` changes — it has no dependency on them.

The job service needs the same `REDIS_URL`, `S3_*` environment variables as the current gateway. The gateway needs a new `JOB_SERVICE_URL` environment variable pointing to the job service's internal Railway URL.

## What This Does Not Cover

- Real uptime monitoring or incident tracking (separate effort)
- Authentication between services (internal network trust)
- gRPC or other protocol alternatives (decided: HTTP/REST)
- Worker scaling or retry logic
