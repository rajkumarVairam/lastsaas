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
