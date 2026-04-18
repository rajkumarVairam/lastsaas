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

## Priority 4: Server-Sent Events (SSE) — PENDING

**GitHub:** [#20](https://github.com/rajkumarVairam/lastsaas/issues/20)

**Problem:**
The job queue (Priority 2) runs tasks durably, but the frontend has no push channel to know when a long-running job finishes. Polling `/api/tenant/jobs/{id}` every few seconds works but wastes DB reads and adds latency.

**Solution:**
- Implement an SSE dispatcher in the Go backend (`GET /api/tenant/events/stream`)
- Push `job.completed`, `job.failed`, and custom product events over the stream
- React hooks subscribe to the stream and update UI state without polling

This is the natural complement to the job queue for real-time UX (progress bars, live status updates, notification toasts).

---

## Priority 5: Recurring Scheduled Tasks (Cron) — PARTIAL

**Status:** The job queue supports `runAt` for deferred single-shot execution. True recurring schedules (e.g. "run every midnight", "run every Monday at 9am") require a separate mechanism to re-enqueue jobs on a schedule.

**What's needed:**
- A `CronSchedule` model storing the recurrence rule (cron expression or interval), job type, and payload template
- A leader-elected goroutine that evaluates due schedules and calls `queue.Enqueue()` — the existing `LeaderLocks` collection already provides the leader election primitive
- Admin API to create/pause/delete schedules

This avoids Redis entirely by reusing the existing job queue and MongoDB leader lock — consistent with the approach taken for Priority 2.
