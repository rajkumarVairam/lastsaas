# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Backend (Go)

```bash
# Run the server (auto-loads .env from project root)
cd backend && go run ./cmd/server

# Run the CLI / MCP server
cd backend && go run ./cmd/lastsaas <command>

# Build
cd backend && go build ./...

# Type-check frontend
cd frontend && npx tsc --noEmit

# All backend tests (unit + integration)
cd backend && LASTSAAS_ENV=test go test -count=1 -v ./...

# Unit tests only (no DB required)
cd backend && LASTSAAS_ENV=test go test -short -count=1 ./...

# Single test
cd backend && LASTSAAS_ENV=test go test -count=1 -v -run TestFunctionName ./internal/package/...

# Integration tests only
cd backend && LASTSAAS_ENV=test go test -count=1 -v -run Integration ./...

# Validation tests (after model changes)
cd backend && go test ./internal/validation/...
```

### Frontend (Node/React)

```bash
cd frontend && npm run dev          # Dev server on :4280
cd frontend && npm run build        # Production build
cd frontend && npm run lint         # ESLint
cd frontend && npm test             # Vitest unit tests
cd frontend && npx playwright test  # E2E tests
```

### First-time setup

```bash
# Backend starts on :4290, frontend on :4280
cd backend && go run ./cmd/lastsaas setup   # Creates root tenant + admin account
```

For Stripe local webhook testing: `stripe listen --forward-to localhost:4290/api/billing/webhook`

## Architecture

### Stack

- **Backend**: Go 1.25, gorilla/mux, MongoDB (Atlas or local)
- **Frontend**: React 19, TypeScript, Vite 7, Tailwind CSS 4, React Query, React Hook Form, Zod
- **Auth**: JWT (30min access + 7d refresh with rotation), bcrypt, Google/GitHub/Microsoft OAuth, TOTP MFA, Magic Links
- **Billing**: Stripe (Checkout, Billing Portal, webhooks, Lazy Provisioning — Stripe Products/Prices are created on first checkout, not when plans are created in admin)
- **Email**: Resend
- **Deployment**: Docker multi-stage → single ~14MB Alpine binary that serves the SPA directly

### Multi-Tenancy Model

- **Root tenant**: The system admin organization. Members are the "admin team." `tenant.IsRoot == true`.
- **Customer tenants**: Created exclusively via the public `/signup` flow (no admin "create tenant" button by design).
- **Memberships**: Users belong to tenants via `TenantMembership` documents. Roles: `owner > admin > user`.
- Every authenticated request carries the user + tenant in context. Use `middleware.GetUserFromContext()` and `middleware.GetTenantFromContext()`.

### Backend Structure

```
backend/
  cmd/server/main.go        # Entry point: wires all services, registers all routes
  cmd/lastsaas/main.go      # CLI + MCP server
  internal/
    api/handlers/           # HTTP handlers — one file per domain
    middleware/             # Auth, tenant resolution, RBAC, rate limiting, billing enforcement, metrics
    models/                 # All MongoDB document structs with validate tags
    db/                     # MongoDB connection, collections, indexes, JSON Schema (schema.go)
    auth/                   # JWT, bcrypt, OAuth providers, TOTP
    events/                 # Internal event emitter → drives outgoing webhook delivery
    webhooks/               # Outgoing webhook delivery engine
    stripe/                 # Stripe service (Checkout, Portal, Customers, Prices, Subscriptions)
    email/                  # Resend templates
    syslog/                 # System log service (use this, not raw slog, for significant events)
    telemetry/              # Event collection, Go SDK, PM analytics queries
    configstore/            # Runtime config variables (DB-backed, cached)
    health/                 # System health monitoring (CPU/mem/disk/HTTP metrics)
    version/                # Auto-migration on startup
    validation/             # Hybrid validation tests
```

### Middleware Chain

Typical route registration in `cmd/server/main.go`:

```go
r.Handle("/api/some/endpoint",
    authMiddleware.RequireAuth(
        tenantMiddleware.ResolveTenant(
            middleware.RequireRole(models.RoleUser)(
                middleware.RequireActiveBilling(db)(
                    http.HandlerFunc(handler.Method)))))))
```

- `RequireAuth` — validates JWT or `lsk_` API key; sets `UserContextKey` in context
- `ResolveTenant` — resolves tenant from `X-Tenant-ID` header or API key; sets `TenantContextKey`
- `RequireRole(minRole)` — checks membership role from context
- `RequireRootTenant()` — restricts to root/admin routes
- `RequireActiveBilling(db)` — blocks expired subscriptions from paid features
- `RequireEntitlement(db, "key")` — gates on plan entitlement

### Adding a New Feature (Pattern)

1. **Model**: Add struct to `internal/models/`, add `validate` tags, update `internal/db/schema.go`, add test in `internal/validation/validate_test.go`
2. **Handler**: Create or extend a file in `internal/api/handlers/`. Handler structs receive dependencies via constructor.
3. **Route**: Register in `cmd/server/main.go` with appropriate middleware chain
4. **Events**: Call `emitter.Emit(events.Event{Type: events.EventXxx, ...})` — the webhook engine picks these up automatically
5. **Telemetry**: Call `telemetry.Track()` for analytics events

### Frontend Structure

```
frontend/src/
  api/client.ts             # Axios instance with silent JWT refresh on 401, X-Tenant-ID header management
  contexts/                 # AuthContext (user + tokens), TenantContext (current tenant), BrandingContext (theme)
  pages/
    admin/                  # Root-tenant admin UI
    app/                    # Customer-facing pages
    auth/                   # Login, signup, MFA, magic link, verification
    public/                 # Landing page, custom pages
  types/index.ts            # All TypeScript types (single source of truth)
```

- Data fetching uses **React Query** (`useQuery`/`useMutation`)
- Forms use **React Hook Form** + **Zod** validation
- API calls go through `api/client.ts` — never call `fetch` directly
- The `BrandingContext` injects white-label theme CSS automatically; components don't need to read branding directly

### Configuration

Config files: `backend/config/dev.yaml` and `prod.yaml` (gitignored; copy from `dev.example.yaml`).

`LASTSAAS_ENV=dev|prod|test` selects which config to load. Secrets are referenced as `${ENV_VAR}` in YAML.

The `.env` file at the project root is auto-loaded by the backend. Key required vars: `DATABASE_NAME`, `MONGODB_URI`, `JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET`, `FRONTEND_URL`.

## Validation

LastSaaS uses hybrid validation: Go-side (`validate` struct tags via go-playground/validator) and MongoDB JSON Schema (`internal/db/schema.go`).

**When modifying model structs in `internal/models/`:**
1. Update `validate` struct tags on the model
2. Update the corresponding MongoDB JSON Schema in `internal/db/schema.go`
3. Keep both in sync — the Go tags and MongoDB schema must enforce the same constraints
4. Run `cd backend && go test ./internal/validation/...` to verify

**When adding a new collection that accepts user/API writes:**
1. Add `validate` tags to the model struct
2. Add a schema function to `internal/db/schema.go` and include it in `AllSchemas()`
3. Add tests in `internal/validation/validate_test.go`

## System Logging

Use `syslog.Logger` for all significant system events. Severity levels: critical, high, medium, low, debug.

## Build Verification

Always verify after changes:
```bash
cd backend && go build ./...
cd frontend && npx tsc --noEmit
```

## Dependent Project Deployment (CRITICAL)

Any project built on the LastSaaS boilerplate — whether using it as a Git submodule, fork, or copy — **MUST** deploy using the SaaS Dockerfile (`Dockerfile.saas`) and the corresponding Fly config (`fly.saas.toml`). Never use bare `fly deploy` on a project that depends on LastSaaS.

**Why this matters:** The SaaS Dockerfile runs both the product backend AND the LastSaaS backend behind Caddy (via supervisord). The LastSaaS backend serves all auth endpoints (`/api/auth/*`), bootstrap status (`/api/bootstrap/status`), OAuth providers (Google, etc.), billing, and admin APIs. Without it, login breaks silently — the product backend has no auth routes, so API calls return HTML from the SPA catch-all, causing mysterious redirects to `/setup` or broken login forms with missing OAuth buttons.

**Correct deploy command:**
```bash
fly deploy -c fly.saas.toml
```

**Propagation rule:** When setting up or working on any dependent project, ensure:
1. The project has a `deploy.md` at its root with full deployment instructions and the "why" behind the multi-process architecture
2. The project's Claude Code memory (MEMORY.md or CLAUDE.md) contains a cross-reference: "See `deploy.md` — never bare `fly deploy`"
3. If the project doesn't have these yet, create them before the first deployment
