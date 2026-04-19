# Deploying to Render

## Prerequisites

- Render account at https://render.com
- MongoDB Atlas cluster (free M0 works for starting out)
- Stripe account in test mode (switch to live when ready)
- Resend account for transactional email

---

## First deployment

### 1. Connect your repo

1. Render Dashboard → **New** → **Web Service**
2. Connect your GitHub repo
3. Render auto-detects the `Dockerfile` — confirm it

### 2. Set environment variables

In the Render dashboard under **Environment**, add these secrets.  
Variables marked **generate** can use Render's "Generate" button for random values.

| Variable | Value | Notes |
|---|---|---|
| `MONGODB_URI` | `mongodb+srv://...` | From Atlas → Connect → Drivers |
| `DATABASE_NAME` | `lastsaas` | Or your chosen DB name |
| `FRONTEND_URL` | `https://yourapp.onrender.com` | Update after first deploy |
| `JWT_ACCESS_SECRET` | *(generate)* | Min 32 chars |
| `JWT_REFRESH_SECRET` | *(generate)* | Min 32 chars, different from access |
| `WEBHOOK_ENCRYPTION_KEY` | *(generate)* | 32+ chars |
| `RESEND_API_KEY` | `re_...` | From Resend dashboard |
| `FROM_EMAIL` | `noreply@yourdomain.com` | Must be verified in Resend |
| `FROM_NAME` | `YourApp` | Sender display name |
| `APP_NAME` | `YourApp` | Shown in UI and emails |
| `STRIPE_SECRET_KEY` | `sk_test_...` | Use test key until go-live |
| `STRIPE_PUBLISHABLE_KEY` | `pk_test_...` | |
| `STRIPE_WEBHOOK_SECRET` | `whsec_...` | From step 4 below |
| `GOOGLE_CLIENT_ID` | *(optional)* | Skip if not using Google OAuth |
| `GOOGLE_CLIENT_SECRET` | *(optional)* | |
| `GOOGLE_REDIRECT_URL` | `https://yourapp.onrender.com/api/auth/google/callback` | |

> Do **not** set `PORT` or `SERVER_PORT` — Render injects `PORT` automatically and the Dockerfile maps it.

### 3. Deploy

Click **Deploy**. Build takes 3–5 minutes (Go + Node compilation).  
Watch logs — a successful start looks like:
```
INFO Server listening addr=0.0.0.0:10000
```

### 4. Configure Stripe webhooks

1. Stripe Dashboard → **Developers** → **Webhooks** → **Add endpoint**
2. URL: `https://yourapp.onrender.com/api/billing/webhook`
3. Select events (or choose "receive all events")
4. Copy the **Signing secret** (`whsec_...`) → paste into `STRIPE_WEBHOOK_SECRET` in Render
5. Redeploy (or Render picks it up on next deploy)

### 5. Run first-time setup

```bash
# Once deployed, bootstrap the root admin account via the web UI
open https://yourapp.onrender.com/setup
```

Or hit the setup endpoint:
```bash
curl -X POST https://yourapp.onrender.com/api/bootstrap/setup \
  -H "Content-Type: application/json" \
  -d '{"name":"Your Name","email":"you@example.com","password":"StrongPassword123!"}'
```

### 6. Update FRONTEND_URL

After your first deploy you'll know the final URL. Update `FRONTEND_URL` in Render env vars to match — this is used for OAuth redirect URLs and email links.

---

## Custom domain

1. Render Dashboard → your service → **Settings** → **Custom Domain**
2. Add `app.yourdomain.com`
3. Add the CNAME record in your DNS provider
4. Render provisions TLS automatically
5. Update `FRONTEND_URL` and `GOOGLE_REDIRECT_URL` to use the custom domain
6. Update Stripe webhook endpoint URL

---

## Going live with Stripe

1. Switch `STRIPE_SECRET_KEY` and `STRIPE_PUBLISHABLE_KEY` to `sk_live_...` / `pk_live_...`
2. Create a new Stripe webhook endpoint pointing to your live URL, copy the new `whsec_...`
3. Update `STRIPE_WEBHOOK_SECRET`
4. In Stripe: create your Products and Prices (or they are created lazily on first checkout — see CLAUDE.md)

---

## Plan / pricing

| Render plan | Cost | Notes |
|---|---|---|
| Free | $0 | Spins down after 15 min inactivity — 30-60s cold start. OK for staging. |
| Starter | $7/mo | Always-on, 512MB RAM. Good for early production. |
| Standard | $25/mo | 2GB RAM. Use when you have real traffic. |

For MongoDB: Atlas free tier (M0, 512MB) is enough to start. Upgrade to M10 ($57/mo) when you need replicas and backups.

---

## Deploying updates

Every push to your connected branch auto-deploys (controlled by `autoDeploy: true` in `render.yaml`).

To deploy manually:
```bash
git push origin main
```

Or trigger manually in the Render dashboard → **Manual Deploy**.

---

## Render vs Fly comparison

| | Render | Fly |
|---|---|---|
| Always-on cheapest | $7/mo | ~$5/mo (shared CPU) |
| Docker support | ✅ | ✅ |
| Custom domains + TLS | ✅ free | ✅ free |
| MongoDB managed | ❌ (use Atlas) | ❌ (use Atlas) |
| Build time | 3–5 min | 2–4 min |
| Dashboard UI | Excellent | Decent |
| CLI required | No | Yes (`flyctl`) |
| Multi-region | Paid | Yes |
