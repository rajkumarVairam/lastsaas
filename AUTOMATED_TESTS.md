# Automated Test Coverage

## How to run

```bash
# Full run — seeds DB, mints tokens, runs all 50 tests
cd frontend && npx playwright test

# Re-run without re-seeding (faster, uses existing manifest)
cd frontend && SKIP_SEED=1 npx playwright test

# Run a single spec file
cd frontend && SKIP_SEED=1 npx playwright test e2e/billing.spec.ts

# Run a single test by name
cd frontend && SKIP_SEED=1 npx playwright test -g "active user login"
```

> Seed runs once before the suite via `global-setup.ts`.  
> JWTs are minted server-side and cached in `seed-manifest.json` — no login API calls during tests.

---

## Test files and what they cover

### `e2e/smoke.spec.ts` — 5 tests
Quick sanity checks, the first things to look at if something is broken.

| Test | What it verifies |
|---|---|
| App loads at root URL | `/` returns 200, app boots |
| Login page is accessible | Email + password fields present |
| Signup page is accessible | Registration form present |
| Full login → dashboard → logout flow | Active user: login, land on /dashboard, clear token, redirect to /login |
| Admin login → admin panel flow | Root admin: login, navigate to /admin, admin UI loads |

---

### `e2e/auth.spec.ts` — 9 tests

| Test | What it verifies |
|---|---|
| Login page renders correctly | Email, password fields and submit button visible |
| Invalid credentials shows error | Bad email/password returns error or rate-limit message, stays on /login |
| Protected pages redirect unauthenticated users | /dashboard, /team, /plan, /settings → /login |
| Active user login reaches dashboard | JWT injection lands on /dashboard |
| Root admin login reaches admin panel | rootAdmin JWT, navigate to /admin, URL confirms access |
| Non-root user cannot access admin routes | activeOwner navigates to /admin → redirected away |
| Signup form is accessible | Submit button visible |
| Login page links to signup | Link present |
| Signup page links to login | Link present |

---

### `e2e/admin.spec.ts` — 10 tests

| Test | What it verifies |
|---|---|
| Admin routes redirect unauthenticated users | /admin, /admin/users, /admin/tenants, /admin/logs → /login |
| Root admin can access users list | /admin/users loads for rootAdmin |
| Root admin can access tenants list | /admin/tenants loads |
| Root admin can access plans page | /admin/plans loads, plan content present |
| Root admin can access health page | /admin/health loads |
| Root admin can access logs page | /admin/logs loads |
| Root admin can access financial page | /admin/financial loads |
| Root admin can access config page | /admin/config loads |
| Root admin can access API docs page | /admin/api loads |
| Non-root user redirected away from admin panel | activeOwner → /admin gets blocked |
| Root admin can view specific tenant profile | /admin/tenants/{id} loads with real tenant ID from manifest |

---

### `e2e/billing.spec.ts` — 8 tests

| Test | Account used | What it verifies |
|---|---|---|
| Free plan user sees upgrade CTA | freeOwner | /plan shows upgrade prompt |
| Trial user can access dashboard | trialOwner | Lands on /dashboard |
| Active monthly subscriber | activeOwner | /dashboard and /plan both load |
| Annual subscriber | annualOwner | /dashboard loads |
| Lifetime / billing-waived | lifetimeOwner | /dashboard and /settings load |
| Past-due user | pastDueOwner | /dashboard and /plan load without crash |
| Canceled user sees win-back UI | canceledOwner | /plan shows resubscribe CTA |
| Enterprise user | enterpriseOwner | /dashboard and /plan load |
| Plan page accessible for all users | freeOwner, activeOwner, annualOwner, lifetimeOwner, enterpriseOwner | /plan loads for each |

---

### `e2e/credits.spec.ts` — 4 tests

| Test | Account used | What it verifies |
|---|---|---|
| Full-credits user — no blocks | aiFullOwner (1500 credits) | /dashboard loads, no "out of credits" blocker |
| Low-credits user | aiLowOwner (18 credits) | /dashboard loads |
| Empty-credits user — purchase prompt | aiEmptyOwner (0 credits) | /buy-credits loads |
| Buy-credits page accessible | aiFullOwner | /buy-credits loads |

---

### `e2e/team.spec.ts` — 5 tests

| Test | Account used | What it verifies |
|---|---|---|
| Team owner can access team management | teamOwner | /team loads, "Invite Member" button visible |
| Team admin can view team page | teamAdmin | /team loads |
| Team member can view team page | teamMember | /team loads |
| All team roles can access dashboard | teamOwner, teamAdmin, teamMember | /dashboard loads for each |
| Team owner can access settings | teamOwner | /settings loads |

---

### `e2e/navigation.spec.ts` — 7 tests

| Test | What it verifies |
|---|---|
| Unknown routes redirect | /nonexistent → /login or /dashboard |
| Login → signup navigation | Link click lands on /signup |
| Signup → login navigation | Link click lands on /login |
| Forgot password page loads | /forgot-password returns content |
| Authenticated user navigates app | /dashboard, /team, /plan, /settings, /activity — all load without redirect to /login |
| Activity page audit log | /activity loads for authenticated user |
| Settings page | /settings loads for authenticated user |

---

## Total: 50 tests across 6 files

| Category | Tests | Pass rate |
|---|---|---|
| Smoke | 5 | ✅ 5/5 |
| Authentication | 9 | ✅ 9/9 |
| Admin panel | 10 | ✅ 10/10 |
| Billing states | 8 | ✅ 8/8 |
| AI credits | 4 | ✅ 4/4 |
| Team / RBAC | 5 | ✅ 5/5 |
| Navigation | 7 | ✅ 7/7 |
| **Total** | **50** | **✅ 50/50** |

---

## Adding tests for new features

When you add a feature or change existing behaviour:

1. **Add seed data if needed** — edit `backend/internal/seed/seed.go`
   - Add the scenario in the appropriate `seed*` function
   - Add new account to `Manifest.Accounts` if a new user persona is required
   - Add new getter to `frontend/e2e/fixtures/seed.ts`

2. **Write the E2E test** — add to an existing spec or create a new one in `frontend/e2e/`
   - Use `loginAs(page, seed.accountName.email, seed.accountName.password)` for authenticated tests
   - Always import from `./fixtures/seed` — never hardcode emails or IDs

3. **Update this document** — add a row to the relevant spec file table above

4. **Run the suite** to confirm nothing regressed:
   ```bash
   cd frontend && npx playwright test
   ```

> The seed command is idempotent — `--reset` wipes all tagged documents before re-seeding, so you can add new scenarios without breaking existing ones.
