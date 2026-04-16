# Architectural Primitives Roadmap

While the application handles business logic (auth, billing, multitenancy) efficiently, it requires several foundational backend primitives to scale securely and handle data-heavy workloads.

---

## Priority 1: Object Storage (S3 / R2)

**Problem:** 
The application currently lacks a unified, scalable way to store and retrieve large files (e.g., user uploads, CSV imports, generated PDFs, avatars). Storing binary data in MongoDB or on local disk in ephemeral containers is an anti-pattern.

**Solution:**
- Implement a generic Blob Storage interface in Go.
- Integrate the AWS SDK for Go.
- Standardize on Cloudflare R2 (for zero egress fees) or AWS S3.
- Build secure presigned-URL flows so the React frontend can securely upload files directly to the storage bucket without routing heavy byte-streams through the Go backend.

---

## Priority 2: Durable Background Jobs & Queues (Redis)

**Problem:**
Right now, asynchronous tasks (sending emails, webhook dispatching) likely execute in simple Goroutines (`go sendEmail()`). If the server reboots during execution, tasks are permanently lost. Additionally, offering intensive features (e.g., data crunching, AI processing) will cause synchronous HTTP requests to time out.

**Solution:**
- Introduce **Redis** to the infrastructure diagram.
- Implement a durable task queue worker using the `Asynq` package in Go.
- Decouple heavy processing from the web HTTP handlers to ensure the frontend remains snappy and fail-safe retries (exponential backoff) are guaranteed for API limits.

---

## Priority 3: Distributed Caching & Rate Limiting

**Problem:**
As we scale to multiple server nodes spread across geographic regions, in-memory caching (using standard Go maps) becomes disjointed. An API rate limit applied to Server A is unaware of traffic hitting Server B.

**Solution:**
- Centralize temporary state configuration, session lookup caching, and rate limit counters in **Redis**.
- This enables rapid responses for heavy database aggregate queries and prevents abuse across the entire server cluster simultaneously.

---

## Priority 4: WebSockets or Server-Sent Events (SSE)

**Problem:**
Once Background Queues (Priority 2) are implemented, the frontend will need to know when long-running tasks are finished. Polling the database every 2 seconds via standard HTTP is inefficient and drains database resources.

**Solution:**
- Implement a Server-Sent Events (SSE) dispatcher in the Go backend.
- Hook standard React state models to SSE listeners to push notifications or progress bar updates natively.

---

## Priority 5: Distributed Cron / Scheduled Tasks

**Problem:**
There is no durable mechanism for running tasks at exact times globally across a multi-node deployment without race conditions (e.g., "Clean up inactive trial accounts every midnight"). 

**Solution:**
- Utilize the clustered Redis queue architecture to maintain a global schedule capable of handling timezone-specific execution guarantees.
