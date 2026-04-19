# Manual Test Cases

Everything in this document requires a human to verify — either because it involves external systems (Stripe, email, OAuth), real-time UI interactions, or visual/UX quality that automated tests do not check.

---

## Seed Credentials

Run the seed command to populate these accounts before manual testing:

```bash
cd backend && go run ./cmd/saasquickstart seed --reset
```

All accounts use password: **`Seed123!`**  
App URL: **http://localhost:4280**  
Backend URL: **http://localhost:4290**

| Account key | Email | Billing state | Notes |
|---|---|---|---|
| rootAdmin | root-admin@seed.local | — | Root tenant admin, full /admin access |
| freeOwner | free@seed.local | None (free plan) | Hits entitlement gates |
| trialOwner | trial@seed.local | Active (trial used) | trialUsedAt is set |
| activeOwner | active@seed.local | Active monthly | Full Pro access |
| annualOwner | annual@seed.local | Active annual | Annual Pro |
| lifetimeOwner | lifetime@seed.local | Waived | billingWaived=true, no Stripe |
| pastDueOwner | pastdue@seed.local | Past due | RequireActiveBilling returns 402 |
| canceledOwner | canceled@seed.local | Canceled | Win-back / resubscribe state |
| enterpriseOwner | enterprise@seed.local | Waived | Enterprise plan, all entitlements |
| teamOwner | team-owner@seed.local | Active | Owner role in shared team tenant |
| teamAdmin | team-admin@seed.local | Active | Admin role in shared team tenant |
| teamMember | team-member@seed.local | Active | User role in shared team tenant |
| aiFullOwner | ai-full@seed.local | Active | 1500 credits remaining |
| aiLowOwner | ai-low@seed.local | Active | 18 credits remaining |
| aiEmptyOwner | ai-empty@seed.local | Active | 0 credits — blocked state |
| apiKeyOwner | apikey-owner@seed.local | Active | Has `lsk_seed_…` API key in manifest |

---

## MT-01 — Stripe Checkout

**Accounts to test with:** freeOwner, canceledOwner, trialOwner  
**Prerequisite:** Stripe CLI running (`npm run dev:stripe`), Stripe test mode configured

| Step | Expected result |
|---|---|
| Log in as freeOwner, go to /plan, click Upgrade | Redirected to Stripe Checkout page |
| Enter test card `4242 4242 4242 4242`, any future date, any CVC | Payment succeeds |
| Redirected back to /billing/success | Success page loads |
| Go to /plan | Account now shows active Pro subscription |
| Log in as canceledOwner, go to /plan | Resubscribe option visible |
| Complete resubscribe flow | Account reactivated |

**Test cards:** https://stripe.com/docs/testing#cards  
`4000 0000 0000 9995` → declined (use to test failed payment)  
`4000 0027 6000 3184` → 3D Secure required

---

## MT-02 — Stripe Billing Portal

**Accounts to test with:** activeOwner, annualOwner

| Step | Expected result |
|---|---|
| Log in, go to /plan, click "Manage Billing" or "Customer Portal" | Redirected to Stripe Billing Portal |
| Cancel subscription in portal | Redirected back, subscription marked for cancellation |
| Log in as activeOwner the next day (or wait for webhook) | Account shows canceled state |
| Update payment method in portal | Payment method updated |
| Download invoice PDF | PDF downloads successfully |

---

## MT-03 — Email Delivery

**Prerequisite:** Resend API key configured, or Resend dev/sandbox mode

| Trigger | Expected email |
|---|---|
| Sign up with a new email | "Verify your email" arrives within 60 seconds |
| Click verify link in email | Account marked verified, redirected to app |
| Forgot password flow | "Reset your password" email arrives |
| Click reset link | Password reset form loads, new password accepted |
| Magic link login | "Your magic link" email arrives within 60 seconds |
| Click magic link | Logged in directly, no password needed |
| Invite team member | Invited member receives invite email |
| Click invite link | Invited user lands on signup/join flow |
| Unsubscribe link in any marketing email | `GET /api/auth/unsubscribe?token=…` → shows confirmation page |
| Check user preferences after unsubscribe | `emailPreferences.marketing = false` |

---

## MT-04 — Google / GitHub / Microsoft OAuth

**Prerequisite:** OAuth app credentials in dev.yaml

| Step | Expected result |
|---|---|
| Click "Continue with Google" on /login | Redirected to Google consent screen |
| Approve Google OAuth | Redirected back, logged in, account created |
| Repeat login with same Google account | Same account, no duplicate user created |
| Click "Continue with GitHub" | GitHub OAuth flow, same behaviour |
| Click "Continue with Microsoft" | Microsoft OAuth flow, same behaviour |
| First OAuth login creates account | User appears in admin /admin/users |

---

## MT-05 — MFA (TOTP)

**Account to test:** Any non-MFA account (e.g. activeOwner after setup)

| Step | Expected result |
|---|---|
| Go to /settings → Security, enable MFA | QR code shown |
| Scan QR with authenticator app (Google Authenticator, Authy) | 6-digit code generated |
| Enter code to confirm setup | MFA enabled |
| Log out, log back in | After password, MFA challenge screen appears |
| Enter correct 6-digit code | Logged in |
| Enter wrong code 3 times | Locked out (rate limited) |
| Disable MFA in /settings | Login no longer requires code |

---

## MT-06 — Team Invitations

**Accounts to test with:** teamOwner (inviter), a fresh email address (invitee)

| Step | Expected result |
|---|---|
| Log in as teamOwner, go to /team | Team management page loads, members listed |
| Click "Invite Member", enter a new email | Invite sent confirmation |
| Open invite email as invitee | Invite email received |
| Click invite link | Lands on signup (if new) or join (if existing account) |
| Complete signup | Account added to team, role = user |
| Log in as teamOwner, check /team | New member appears in list |
| Log in as teamAdmin, try to change teamOwner's role | Should be blocked (can't demote owner) |
| Log in as teamOwner, transfer ownership to teamAdmin | Ownership transferred |

---

## MT-07 — Ownership Transfer

**Accounts to test with:** teamOwner, teamAdmin

| Step | Expected result |
|---|---|
| Log in as teamOwner, go to /settings or /team | Transfer ownership option visible (owners only) |
| Transfer to teamAdmin | teamAdmin now shown as owner |
| Log out, log in as original teamOwner | Now shows as admin, no ownership controls |
| Log in as new owner (teamAdmin) | Transfer ownership option now visible |

---

## MT-08 — API Key Authentication

**Account to test with:** apiKeyOwner  
**API key:** check `seed-manifest.json` → accounts.apiKeyOwner.apiKey

| Step | Expected result |
|---|---|
| Make API request with `Authorization: Bearer lsk_seed_…` | Request succeeds, authenticated as apiKeyOwner |
| Make request without header | 401 Unauthorized |
| Make request with invalid key | 401 Unauthorized |
| Log in as apiKeyOwner, go to /settings → API Keys | Seeded key shown with preview |
| Revoke the key in UI | Key no longer works for API requests |
| Create a new API key | New key generated, shown once, works for API requests |

---

## MT-09 — Admin Panel Operations (Destructive)

**Account:** rootAdmin (root-admin@seed.local)

| Step | Expected result |
|---|---|
| Go to /admin/users, find a seeded user | User record shown with full detail |
| Click user → view profile page | /admin/users/{id} loads, shows memberships |
| Go to /admin/tenants, find a seeded tenant | Tenant record shown |
| Click tenant → view profile | /admin/tenants/{id} loads |
| Go to /admin/plans, create a new plan | Plan form submits, plan appears in list |
| Edit existing plan entitlements | Changes saved, reflected in user's /plan page |
| Go to /admin/promotions, create a promo code | Code created |
| Apply promo code at checkout | Discount applied |
| Go to /admin/announcements, create announcement | Announcement appears in /messages for users |
| Go to /admin/branding, upload logo | Logo appears in app header |
| Go to /admin/config, change a config value | Value persisted, reflected in app behavior |

---

## MT-10 — Rate Limiting

| Scenario | Expected result |
|---|---|
| Login with wrong password 10 times in 15 min | "Rate limit exceeded" with retryAfter shown |
| Wait for retryAfter seconds, try again | Login attempt accepted again |
| Signup 5 times from same IP in 1 hour | Rate limited on account creation |
| Request password reset 5 times in 1 hour | Rate limited |

---

## MT-11 — White-label / Branding

**Account:** rootAdmin, then verify as any customer account

| Step | Expected result |
|---|---|
| Go to /admin/branding | Branding config page loads |
| Set custom primary colour | App header and buttons show new colour |
| Upload logo | Logo appears in app header for all tenants |
| Set custom app name | Page title and nav show custom name |
| Set custom support email | Emails sent to users show custom sender |
| Verify as a customer account | Customer sees branded UI, not default |

---

## MT-12 — Outgoing Webhooks

**Account:** activeOwner  
**Prerequisite:** A local webhook receiver (e.g. `nc -l 8888` or requestbin)

| Step | Expected result |
|---|---|
| Go to /settings → Webhooks (if UI exists) | Webhook management page |
| Add endpoint URL | Endpoint saved |
| Trigger an event (e.g. update profile) | Webhook delivered to endpoint within 5s |
| Check webhook payload | JSON with event type, tenantId, timestamp |
| Set endpoint to unreachable URL | Delivery fails, retried (check /admin/logs) |
| Check event log in admin panel | Failed delivery shown with retry count |

---

## MT-13 — Billing States — Visual Verification

Automated tests verify pages load. Manual tests verify the UI is correct.

| Account | Go to | Verify manually |
|---|---|---|
| freeOwner | /plan | "Upgrade" buttons on paid tiers, free tier highlighted |
| trialOwner | /dashboard | Trial indicator / banner visible if implemented |
| pastDueOwner | /dashboard or /plan | Payment failure banner, "Update payment" CTA |
| canceledOwner | /plan | "Resubscribe" / "Your subscription ended" message |
| aiLowOwner | /dashboard | Low credit warning (e.g. "18 credits remaining") |
| aiEmptyOwner | /dashboard | Credits blocked CTA — "Purchase more credits" |

---

## MT-14 — Responsive / Mobile Layout

**Tested at:** 375px (iPhone SE), 768px (iPad), 1280px (desktop)

| Page | Check |
|---|---|
| /login | Form readable, no overflow |
| /dashboard | Sidebar collapses or becomes drawer on mobile |
| /team | Table scrolls horizontally or stacks on mobile |
| /plan | Plan cards stack vertically on mobile |
| /admin/users | Admin table scrolls horizontally |

---

## Process — Keeping Tests Updated

### When you add a new feature

**Backend changes:**
1. Add a seed scenario in `backend/internal/seed/seed.go` if a new billing state, role, or user type is needed
2. Re-run `go run ./cmd/saasquickstart seed --reset` to verify it works
3. Add the new account to the credentials table in this document

**Frontend changes:**
1. Add an E2E test in the appropriate `frontend/e2e/*.spec.ts` file
2. If the test needs a new account type, add it to fixtures/seed.ts
3. Add a row to `AUTOMATED_TESTS.md`
4. If human verification is needed (Stripe, email, OAuth, visual), add a step to this document under the relevant section (or create a new MT-XX section)

**Run both:**
```bash
cd frontend && npx playwright test          # Automated
# Then manually walk through the relevant MT-XX checklist above
```

### When you change existing behaviour

- If a UI element's text changes, update the selector in the affected spec
- If a route changes, update navigation tests
- If billing logic changes, update billing.spec.ts and MT-01/MT-02/MT-13
- If a new entitlement is added, add it to the plan seed data and billing tests
