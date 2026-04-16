# Local Setup Guide 

This guide details how to run the LastSaaS application locally, especially optimized for Windows, without disturbing the default `README.md` and keeping upstream merges clean. You can add `LOCAL_SETUP.md` to your `.gitignore` if you do not wish to push this file.

## Prerequisites
1. **Go (1.25+)**: Required for running the backend.
2. **Node.js (22+)**: Required for running the React frontend.
3. **MongoDB**: You can either install MongoDB locally or create a free cluster on [MongoDB Atlas](https://www.mongodb.com/atlas).
4. **Git Bash** (Recommended for Windows to run bash commands easily) or **PowerShell**.

## Step 1: Environment Variables Setup

Instead of running the bash setup script (which might fail on Windows without Git Bash), you can manually set up your environment variables:

1. Copy the `.env.example` file to `.env`:
   ```powershell
   Copy-Item .env.example .env
   ```
2. Open `.env` and fill in the required variables:
   - `DATABASE_NAME=lastsaas-dev`
   - `MONGODB_URI=mongodb://localhost:27017` (If running local MongoDB, otherwise paste your Atlas URI)
   - `JWT_ACCESS_SECRET` and `JWT_REFRESH_SECRET`: Put any random long string for local development (e.g., `local-dev-secret-string-do-not-use-in-prod`).

## Step 2: Start the Go Backend

The backend needs the `.env` variables loaded. If you are using PowerShell, you can use a tool or read the file, but most modern Go boilerplates automatically load `.env` if it's in the project root. Alternatively, you can run this via Git Bash.

**Using PowerShell**:
```powershell
# Navigate to backend
cd backend

# Run the Go server. (Assuming the code has godotenv to load the .env automatically)
go run ./cmd/server
```

*(Note: The server will start on `http://localhost:4290`)*

## Step 3: Start the React Frontend

Open a **new** terminal/PowerShell window for the frontend.

```powershell
cd frontend

# Install Node dependencies
npm install

# Start the Vite development server
npm run dev
```

*(Note: The frontend will start on `http://localhost:4280`)*

## Step 4: First-Time Initialization (Admin Account Creation)

To log into your new app, you need to create the root tenant and an initial admin account. There is no hardcoded default User ID and Password; instead, you will configure them interactively through the CLI. 

Open a **third** terminal window:

```powershell
cd backend
go run ./cmd/lastsaas setup
```

The CLI will prompt you for:
- Your Organization name
- Your Name
- **Email Address (This becomes your Admin User ID)**
- **Password (This becomes your Admin Password)**

Follow the interactive prompts to create your owner account. Once finished, visit `http://localhost:4280` in your browser and log in with those credentials!

## Step 5: (Optional) Testing

To run the backend tests locally to ensure everything works:
```powershell
cd backend
$env:LASTSAAS_ENV="test"
go test -count=1 -v ./...
```

## Step 6: Onboarding a New Client

When you need to configure the application for a new client (e.g., "Client A"), you have two approaches depending on your chosen business requirements:

### Approach 1: Multi-Tenant Setup (Recommended)
This approach uses LastSaaS's native multi-tenancy. You only run one instance of the application, and you manage the client as an organization inside your admin dashboard.

1. **Log in as Root Admin:** Access your application and log in with your root owner credentials.
2. **Setup a Plan:** In the **Admin → Plans** menu, create a subscription package and define entitlements for the client.
3. **Create the Tenant:** Go to **Admin → Tenants**. You can manually create a new tenant organization (e.g., "Client A Inc.") from the dashboard. Alternatively, the client can sign up and create their own organization.
4. **Assign the Plan:** Once their tenant is created, assign them the plan you created.
5. **Team Invitations:** The client can now log in, invite their team members, and access their isolated workspace.

### Approach 2: Dedicated Deployment (White-Glove)
If the client requires absolute data isolation or you are providing them a completely separate installation, you must spin up a new instance.

1. **Separate Database:** Open your `.env` file for the new deployment and change the `DATABASE_NAME` (e.g., `DATABASE_NAME=client-a-prod`).
2. **Environment Variables:** Update `.env` variables for the specific environment:
   - `FRONTEND_URL` (Client A's custom domain)
   - `APP_NAME` (e.g., `APP_NAME="Client A Portal"`)
   - Re-generate `JWT_ACCESS_SECRET` and `JWT_REFRESH_SECRET`
3. **Run the Setup CLI:** Run the setup command for this new instance to create the admin account:
   ```powershell
   cd backend
   go run ./cmd/lastsaas setup
   ```
4. **Brand the Application:** Log into the Admin Panel for the new deployment. Go to **Admin → Branding Editor** to upload the client's logo, modify theme colors, and customize their landing page.

## Step 7: Admin UI Capabilities

Yes, you can accomplish almost all configuration and management directly through the built-in Admin UI without touching any code. Here is a comprehensive list of what the Root Admin (Owner) can do directly from the dashboard:

### 1. Subscription & Billing (Stripe Integration)
- **Stripe Plans:** You can completely create and manage Stripe subscription packages (e.g., Starter, Pro, Enterprise) directly from **Admin → Plans**. You define the pricing, limits, entitlements, and trial days here; LastSaaS will automatically sync this to Stripe. No need to touch the Stripe dashboard!
- **Credit Bundles:** Create one-time purchase packs (e.g., "500 Credits for $5") from **Admin → Credit Bundles**.
- **Promotions:** Create and manage Stripe discount codes and coupons from **Admin → Promotions**.
- **Financial Dashboard:** View transaction history across all tenants, and see real-time charts for Revenue, ARR, DAU, and MAU.

### 2. Tenant (Client) Management
- **View & Edit Tenants:** See a list of all client organizations, view their current plan, billing status, and member lists.
- **Plan Assignment:** Manually assign or override a tenant's subscription plan.
- **Status Control:** Deactivate, suspend, or manage customer accounts easily.

### 3. Branding & White-Labeling
- **Branding Editor:** Upload custom logos, change theme colors (which auto-generates shade palettes), and set custom fonts.
- **Pages & CSS:** Inject custom CSS, modify the landing page HTML, and generate custom public pages (like `/p/terms`).

### 4. System Operation & Monitoring
- **System Health:** View real-time CPU, memory, disk, and HTTP latency metrics across your server instances. Look at integration health (e.g., is MongoDB or Stripe down?).
- **Log Viewer:** Search through system logs and filter by severity to troubleshoot bugs without touching the server terminal.
- **Product Analytics:** View the conversion funnel (Visitors → Signups → Paid), SaaS KPIs (MRR, Churn, LTV), and engagement cohorts.
- **Configuration Variables:** Edit runtime string/number/enum variables live without needing to redeploy.

### 5. Utilities
- **User Management:** Suspend users, send them in-app messages, or securely impersonate a user account to troubleshoot what they are seeing.
- **Webhooks:** Create outgoing webhooks for 19 different events (e.g., `payment.received` or `user.registered`) and view delivery history.
- **API Keys:** Issue admin-level or user-level `lsk_` scoped API keys.
