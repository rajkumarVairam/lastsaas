# Observability & Low-Maintenance Roadmap

While the application has a built-in health dashboard and system log viewer, moving to a true "low-maintenance" and scalable production environment requires externalizing your observability. If your server goes down, your internal dashboard goes down with it.

Implement these features to ensure the application catches its own bugs and notifies you before your customers do.

---

## Priority 1 (Critical): External Uptime & Incident Routing

**Feature:** External API Uptime Checks and Paging
**Description:**
Your internal health dashboard cannot tell you if the server itself is completely offline. You need an external watcher to ping your APIs globally and physically wake you up (SMS/Call) if critical infrastructure fails.
**Acceptance Criteria:**
- [ ] Connect production endpoints to an external uptime monitor (e.g., BetterStack, UptimeRobot).
- [ ] Set up an escalation policy (e.g., PagerDuty or Opsgenie) to trigger SMS/Calls immediately if the main database connection drops or the health-check API returns `500`.

---

## Priority 2 (High): Centralized Error Tracking

**Feature:** Frontend & Backend Exception Tracking via Sentry
**Description:**
Instead of hunting through terminal logs, integrate an error-tracking service like Sentry or Bugsnag. This will automatically capture unhandled exceptions, group similar crashes, and capture the exact stack trace and user browser context.
**Acceptance Criteria:**
- [ ] Integrate Sentry SDK into the React frontend.
- [ ] Integrate Sentry SDK into the Go backend panic recovery middleware.
- [ ] Configure Sentry to automatically create tickets or Slack/Discord alerts for high-frequency errors.

---

## Priority 3 (High): Automated Database Backups

**Feature:** Point-in-Time Recovery (PITR) for MongoDB
**Description:**
If a database collection is accidentally dropped or a bad deployment corrupts production data, you need a 1-click "undo" button.
**Acceptance Criteria:**
- [ ] Enable continuous cloud backups in MongoDB Atlas.
- [ ] Ensure Point-in-Time Recovery (PITR) is active, enabling restores accurate to the second.
- [ ] Document the database restore drill procedure.

---

## Priority 4 (Medium): Scalable Log Forwarding

**Feature:** External Log Storage & Search
**Description:**
As traffic scales, storing system logs in your primary MongoDB cluster will cause severe performance degradation and bloat costs. Logs should be output to `stdout` and shipped externally.
**Acceptance Criteria:**
- [ ] Update backend logging configuration to output structured JSON to stdout.
- [ ] Set up a log shipper (e.g., Vector, FluentBit) in the deployment pipeline.
- [ ] Connect logs to an external, cheap storage sink (e.g., Axiom, Datadog Logs, AWS CloudWatch) with indexing capabilities.

---

## Priority 5 (Medium): Application Performance Monitoring (APM)

**Feature:** Distributed Tracing for Latency Bottlenecks
**Description:**
If users report that the application is slow, you need to know exactly which function is causing it without adding manual `fmt.Println(time.Since(start))` calls everywhere.
**Acceptance Criteria:**
- [ ] Implement OpenTelemetry tracing across the Go backend.
- [ ] Instrument HTTP clients and MongoDB database calls.
- [ ] Connect traces to a visualization backend (Datadog APM, Jaeger) to view waterfall charts of request execution times.
