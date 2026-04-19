# Architectural Primitives Roadmap

While the application handles business logic (auth, billing, multitenancy) efficiently, it requires several foundational backend primitives to scale securely and handle data-heavy workloads.

---

## ✅ Priority 1: Object Storage (R2 / S3) — COMPLETED

**Shipped:** `d824272`

A provider-agnostic `Store` interface (`Put`, `Delete`, `PresignGet`, `Ping`) supports Cloudflare R2, AWS S3, and a MongoDB db-fallback for zero-config local dev. Switching providers requires only a config change — no code changes.

- `BrandingAsset` (public): logo, favicon, media library served via CDN URL with 301 redirect. Legacy MongoDB blobs still served for backward compatibility.
- `Document` (private): per-tenant file storage namespaced as `documents/{tenantId}/{docId}`. Download via 15-minute presigned GET (302). Visibility control: `tenant` (all members) or `owner` (uploader only).
- `Store.Ping()` wired into the health integration dashboard — misconfigured credentials surface at startup.
- AWS SDK v2 used for both R2 and S3 (same code, different endpoint).

**Not yet built:** presigned PUT for direct browser-to-bucket uploads (bypasses server for large files). Tracked in #22 (Document UI) when large file support becomes necessary.

---

## ✅ Priority 2: Durable Background Jobs — COMPLETED

**Shipped:** `5a69ad6`

Implemented without Redis — a MongoDB-backed job queue with the same durability and reliability guarantees, with no additional infrastructure dependency.

- Atomic `findOneAndUpdate` claim prevents double-execution across concurrent workers
- N-worker pool (default 5) with per-job 4-minute execution timeout
- Exponential backoff on retry: 30s base → 1h ceiling
- Stale lock reclaim on startup and every 5 minutes (crash recovery)
- `Handler` interface (`Type() string`, `Execute(ctx, job) error`) — product job types registered at startup
- Full REST API: list, enqueue, get, cancel, retry (`/api/tenant/jobs/*`)
- 30-day TTL index auto-cleans completed/cancelled jobs

Ideal for social media scheduling, async AI processing, PDF generation, data exports.

---

## ✅ Priority 3: Distributed Rate Limiting — COMPLETED

**Status:** Already in the codebase via `internal/middleware` — MongoDB-backed distributed rate limiter using atomic counters. Rate limit state is shared across all nodes; a request hitting Node A counts against the same bucket as a request hitting Node B.

**Not yet built:** query-result caching for heavy aggregation pipelines (PM analytics, usage dashboards). These are bounded and indexed (#12, #14 fixed) but not cached. Add when query latency becomes measurable.

---

## ✅ Priority 4: Server-Sent Events (SSE) — COMPLETED

**Shipped:** `3d26f41`

Real-time push channel for the job queue and any custom product events, with no polling.

- `internal/sse/Hub` — in-memory tenant-scoped pub/sub. Non-blocking publish skips slow clients rather than blocking the caller.
- `internal/sse/Watcher` — MongoDB change stream on the jobs collection. Every node watches independently, so SSE clients receive events regardless of which node ran the job. Graceful fallback when change streams are unavailable (standalone local MongoDB).
- `GET /api/tenant/events/stream` — authenticated SSE endpoint. Removes the per-connection write deadline via `http.NewResponseController` so the connection stays open indefinitely. 30-second heartbeat comment keeps connections alive through reverse proxies.
- Events pushed: `job.completed`, `job.failed`, `job.dead`. Each carries `id`, `type`, `status`, and optionally `result` or `error`.
- Frontend uses the browser `EventSource` API — reconnect is automatic on disconnect.

---

## ✅ Priority 5: Recurring Scheduled Tasks (Cron) — COMPLETED

**Shipped:** `3d26f41`

True recurring schedules built on top of the existing job queue — no Redis, no separate infrastructure.

- `CronSchedule` model: 5-field cron expression, timezone, jobType, payload template, maxAttempts, isActive, nextRunAt, lastRunAt, distributed lock fields.
- `internal/cron/Scheduler` — 30-second tick with atomic `findOneAndUpdate` claim. Multiple nodes compete; only one fires per schedule tick. Stale lock reclaim on startup and every 5 minutes for crash recovery.
- `NextRunTime` uses `github.com/robfig/cron/v3` to parse expressions and compute the next fire time in the schedule's timezone.
- Paused schedules recompute `nextRunAt` on resume (no missed-fire backlog).
- Full REST API: list, create, get, update (expression/timezone changes recompute nextRunAt), delete, pause, resume.
- MongoDB indexes, JSON Schema validation, and validation tests included.
