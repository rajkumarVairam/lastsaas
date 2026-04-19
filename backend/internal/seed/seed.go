// Package seed populates a database with realistic scenarios covering every
// billing state, team role, job status, and usage pattern. Seeded documents
// share seedTag="lastsaas_seed" so Reset() can wipe them without touching
// real data. Designed for dev databases and E2E test suites.
package seed

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"lastsaas/internal/auth"
	"lastsaas/internal/db"
	"lastsaas/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	SeedTag      = "lastsaas_seed"
	SeedPassword = "Seed123!"
)

// Manifest is written to seed-manifest.json after a successful run.
// Playwright reads this — no hardcoded IDs in tests.
type Manifest struct {
	SeededAt string             `json:"seededAt"`
	Password string             `json:"password"`
	Accounts map[string]Account `json:"accounts"`
	Plans    map[string]string  `json:"plans"`
	Jobs     map[string]string  `json:"jobs"`
	Tenants  map[string]string  `json:"tenants"`
}

type Account struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	UserID       string `json:"userId"`
	TenantID     string `json:"tenantId"`
	ApiKey       string `json:"apiKey,omitempty"`
	AccessToken  string `json:"accessToken,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
}

type runner struct {
	db       *db.MongoDB
	jwtSvc   *auth.JWTService
	pwHash   string
	manifest Manifest
	now      time.Time
}

// Run seeds all scenarios and writes the manifest to outputPath.
// jwtSvc is optional — when provided, access/refresh tokens are minted for each
// account and written to the manifest so E2E tests can inject them directly
// without calling the login API (avoiding rate-limiter exhaustion).
func Run(ctx context.Context, database *db.MongoDB, outputPath string, jwtSvc *auth.JWTService) error {
	pwSvc := auth.NewPasswordService()
	hash, err := pwSvc.HashPassword(SeedPassword)
	if err != nil {
		return fmt.Errorf("hash seed password: %w", err)
	}

	r := &runner{
		db:     database,
		jwtSvc: jwtSvc,
		pwHash: hash,
		now:    time.Now(),
		manifest: Manifest{
			SeededAt: time.Now().UTC().Format(time.RFC3339),
			Password: SeedPassword,
			Accounts: make(map[string]Account),
			Plans:    make(map[string]string),
			Jobs:     make(map[string]string),
			Tenants:  make(map[string]string),
		},
	}

	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"plans", r.seedPlans},
		{"root_admin", r.seedRootAdmin},
		{"free_tenant", r.seedFreeTenant},
		{"trial_tenant", r.seedTrialTenant},
		{"active_monthly", r.seedActiveMonthly},
		{"active_annual", r.seedActiveAnnual},
		{"lifetime_tenant", r.seedLifetimeTenant},
		{"past_due_tenant", r.seedPastDueTenant},
		{"canceled_tenant", r.seedCanceledTenant},
		{"enterprise_tenant", r.seedEnterpriseTenant},
		{"team_tenant", r.seedTeamTenant},
		{"credits_full", r.seedCreditsFullTenant},
		{"credits_low", r.seedCreditsLowTenant},
		{"credits_empty", r.seedCreditsEmptyTenant},
		{"api_key_tenant", r.seedAPIKeyTenant},
		{"jobs", r.seedJobs},
		{"cron_schedules", r.seedCronSchedules},
	}

	for _, step := range steps {
		fmt.Printf("  seeding %-22s ", step.name+"...")
		if err := step.fn(ctx); err != nil {
			fmt.Println("FAILED")
			return fmt.Errorf("%s: %w", step.name, err)
		}
		fmt.Println("ok")
	}

	// Mint JWT tokens for all seeded accounts so E2E tests can inject them
	// directly into localStorage without calling the login API.
	if r.jwtSvc != nil {
		fmt.Printf("  seeding %-22s ", "tokens...")
		if err := r.mintTokens(); err != nil {
			fmt.Println("FAILED")
			return fmt.Errorf("mint tokens: %w", err)
		}
		fmt.Println("ok")
	}

	return writeManifest(outputPath, r.manifest)
}

// Reset removes every document tagged with SeedTag.
func Reset(ctx context.Context, database *db.MongoDB) error {
	filter := bson.M{"seedTag": SeedTag}
	database.Users().DeleteMany(ctx, filter)
	database.Tenants().DeleteMany(ctx, filter)
	database.TenantMemberships().DeleteMany(ctx, filter)
	database.Plans().DeleteMany(ctx, filter)
	database.Jobs().DeleteMany(ctx, filter)
	database.CronSchedules().DeleteMany(ctx, filter)
	database.APIKeys().DeleteMany(ctx, filter)
	database.Invitations().DeleteMany(ctx, filter)
	fmt.Println("  seed data removed.")
	return nil
}

// ── Plans ─────────────────────────────────────────────────────────────────────

func (r *runner) seedPlans(ctx context.Context) error {
	plans := []models.Plan{
		{
			ID: primitive.NewObjectID(), Name: "Seed Free",
			PricingModel: models.PricingModelFlat, MonthlyPriceCents: 0,
			CreditResetPolicy: models.CreditResetPolicyReset,
			IsArchived: false, CreatedAt: r.now, UpdatedAt: r.now, SeedTag: SeedTag,
			Entitlements: map[string]models.EntitlementValue{
				"custom_branding": {Type: models.EntitlementTypeBool, BoolValue: false},
				"api_access":      {Type: models.EntitlementTypeBool, BoolValue: false},
				"max_members":     {Type: models.EntitlementTypeNumeric, NumericValue: 1},
			},
		},
		{
			ID: primitive.NewObjectID(), Name: "Seed Pro",
			PricingModel: models.PricingModelFlat, MonthlyPriceCents: 9900,
			CreditResetPolicy: models.CreditResetPolicyReset, UsageCreditsPerMonth: 500,
			IsArchived: false, CreatedAt: r.now, UpdatedAt: r.now, SeedTag: SeedTag,
			Entitlements: map[string]models.EntitlementValue{
				"custom_branding": {Type: models.EntitlementTypeBool, BoolValue: true},
				"api_access":      {Type: models.EntitlementTypeBool, BoolValue: true},
				"max_members":     {Type: models.EntitlementTypeNumeric, NumericValue: 25},
			},
		},
		{
			ID: primitive.NewObjectID(), Name: "Seed Pro Annual",
			PricingModel: models.PricingModelFlat, MonthlyPriceCents: 9900,
			AnnualDiscountPct: 20, CreditResetPolicy: models.CreditResetPolicyReset,
			UsageCreditsPerMonth: 500,
			IsArchived: false, CreatedAt: r.now, UpdatedAt: r.now, SeedTag: SeedTag,
			Entitlements: map[string]models.EntitlementValue{
				"custom_branding": {Type: models.EntitlementTypeBool, BoolValue: true},
				"api_access":      {Type: models.EntitlementTypeBool, BoolValue: true},
				"max_members":     {Type: models.EntitlementTypeNumeric, NumericValue: 25},
			},
		},
		{
			ID: primitive.NewObjectID(), Name: "Seed Enterprise",
			PricingModel: models.PricingModelPerSeat, MonthlyPriceCents: 0,
			CreditResetPolicy: models.CreditResetPolicyAccrue,
			IsArchived: false, CreatedAt: r.now, UpdatedAt: r.now, SeedTag: SeedTag,
			Entitlements: map[string]models.EntitlementValue{
				"custom_branding": {Type: models.EntitlementTypeBool, BoolValue: true},
				"api_access":      {Type: models.EntitlementTypeBool, BoolValue: true},
				"sso":             {Type: models.EntitlementTypeBool, BoolValue: true},
				"max_members":     {Type: models.EntitlementTypeNumeric, NumericValue: 9999},
			},
		},
		{
			ID: primitive.NewObjectID(), Name: "Seed AI Credits",
			PricingModel: models.PricingModelFlat, MonthlyPriceCents: 2900,
			CreditResetPolicy: models.CreditResetPolicyReset, UsageCreditsPerMonth: 1000,
			IsArchived: false, CreatedAt: r.now, UpdatedAt: r.now, SeedTag: SeedTag,
			Entitlements: map[string]models.EntitlementValue{
				"api_access":    {Type: models.EntitlementTypeBool, BoolValue: true},
				"ai_generation": {Type: models.EntitlementTypeBool, BoolValue: true},
			},
		},
	}

	names := []string{"free", "pro", "pro_annual", "enterprise", "ai_credits"}
	for i, p := range plans {
		if _, err := r.db.Plans().InsertOne(ctx, p); err != nil {
			return err
		}
		r.manifest.Plans[names[i]] = p.ID.Hex()
	}
	return nil
}

// ── Root admin ────────────────────────────────────────────────────────────────

func (r *runner) seedRootAdmin(ctx context.Context) error {
	var rootTenant models.Tenant
	if err := r.db.Tenants().FindOne(ctx, bson.M{"isRoot": true}).Decode(&rootTenant); err != nil {
		return fmt.Errorf("root tenant not found — run `lastsaas setup` first: %w", err)
	}

	user, err := r.createUser(ctx, "Seed Root Admin", "root-admin@seed.local")
	if err != nil {
		return err
	}
	if err := r.createMembership(ctx, user.ID, rootTenant.ID, models.RoleAdmin); err != nil {
		return err
	}
	r.manifest.Accounts["rootAdmin"] = Account{
		Email: user.Email, Password: SeedPassword,
		UserID: user.ID.Hex(), TenantID: rootTenant.ID.Hex(),
	}
	r.manifest.Tenants["root"] = rootTenant.ID.Hex()
	return nil
}

// ── Tenant scenario helpers ───────────────────────────────────────────────────

type tenantSpec struct {
	slug          string
	name          string
	billing       models.BillingStatus
	billingWaived bool
	planKey       string
	subCredits    int64
	purchCredits  int64
	ownerEmail    string
	ownerName     string
	manifestKey   string
}

func (r *runner) seedTenant(ctx context.Context, spec tenantSpec) error {
	var planID *primitive.ObjectID
	if spec.planKey != "" {
		id, err := primitive.ObjectIDFromHex(r.manifest.Plans[spec.planKey])
		if err != nil {
			return fmt.Errorf("plan %q missing from manifest", spec.planKey)
		}
		planID = &id
	}

	tenant := models.Tenant{
		ID: primitive.NewObjectID(), Name: spec.name, Slug: spec.slug,
		IsActive: true, BillingStatus: spec.billing, BillingWaived: spec.billingWaived,
		PlanID: planID, SubscriptionCredits: spec.subCredits, PurchasedCredits: spec.purchCredits,
		CreatedAt: r.now, UpdatedAt: r.now, SeedTag: SeedTag,
	}
	if _, err := r.db.Tenants().InsertOne(ctx, tenant); err != nil {
		return err
	}

	user, err := r.createUser(ctx, spec.ownerName, spec.ownerEmail)
	if err != nil {
		return err
	}
	if err := r.createMembership(ctx, user.ID, tenant.ID, models.RoleOwner); err != nil {
		return err
	}

	r.manifest.Accounts[spec.manifestKey] = Account{
		Email: user.Email, Password: SeedPassword,
		UserID: user.ID.Hex(), TenantID: tenant.ID.Hex(),
	}
	r.manifest.Tenants[spec.slug] = tenant.ID.Hex()
	return nil
}

// ── Billing scenarios ─────────────────────────────────────────────────────────

func (r *runner) seedFreeTenant(ctx context.Context) error {
	return r.seedTenant(ctx, tenantSpec{
		slug: "seed-free", name: "Pixel Free Co",
		billing: models.BillingStatusNone, planKey: "free",
		ownerEmail: "free@seed.local", ownerName: "Free Owner", manifestKey: "freeOwner",
	})
}

func (r *runner) seedTrialTenant(ctx context.Context) error {
	if err := r.seedTenant(ctx, tenantSpec{
		slug: "seed-trial", name: "Trialing SaaS Inc",
		billing: models.BillingStatusActive, planKey: "pro",
		subCredits: 100,
		ownerEmail: "trial@seed.local", ownerName: "Trial Owner", manifestKey: "trialOwner",
	}); err != nil {
		return err
	}
	// Mark trial as used 10 days ago — tests trial-end upgrade prompts
	uid, _ := primitive.ObjectIDFromHex(r.manifest.Accounts["trialOwner"].UserID)
	trialAt := r.now.AddDate(0, 0, -10)
	r.db.Users().UpdateOne(ctx, bson.M{"_id": uid}, bson.M{"$set": bson.M{"trialUsedAt": trialAt, "updatedAt": r.now}})
	return nil
}

func (r *runner) seedActiveMonthly(ctx context.Context) error {
	return r.seedTenant(ctx, tenantSpec{
		slug: "seed-active", name: "Active Monthly LLC",
		billing: models.BillingStatusActive, planKey: "pro", subCredits: 500,
		ownerEmail: "active@seed.local", ownerName: "Active Owner", manifestKey: "activeOwner",
	})
}

func (r *runner) seedActiveAnnual(ctx context.Context) error {
	return r.seedTenant(ctx, tenantSpec{
		slug: "seed-annual", name: "Annual Plan Restaurant",
		billing: models.BillingStatusActive, planKey: "pro_annual", subCredits: 1000,
		ownerEmail: "annual@seed.local", ownerName: "Annual Owner", manifestKey: "annualOwner",
	})
}

func (r *runner) seedLifetimeTenant(ctx context.Context) error {
	// BillingWaived=true — permanently bypasses RequireActiveBilling + RequireEntitlement
	return r.seedTenant(ctx, tenantSpec{
		slug: "seed-lifetime", name: "Lifetime Access Marketplace",
		billing: models.BillingStatusNone, billingWaived: true, planKey: "pro",
		ownerEmail: "lifetime@seed.local", ownerName: "Lifetime Owner", manifestKey: "lifetimeOwner",
	})
}

func (r *runner) seedPastDueTenant(ctx context.Context) error {
	// RequireActiveBilling returns 402 — tests payment wall + recovery UI
	return r.seedTenant(ctx, tenantSpec{
		slug: "seed-pastdue", name: "Past Due Ecommerce Store",
		billing: models.BillingStatusPastDue, planKey: "pro",
		ownerEmail: "pastdue@seed.local", ownerName: "PastDue Owner", manifestKey: "pastDueOwner",
	})
}

func (r *runner) seedCanceledTenant(ctx context.Context) error {
	if err := r.seedTenant(ctx, tenantSpec{
		slug: "seed-canceled", name: "Churned Agency Co",
		billing: models.BillingStatusCanceled, planKey: "pro",
		ownerEmail: "canceled@seed.local", ownerName: "Canceled Owner", manifestKey: "canceledOwner",
	}); err != nil {
		return err
	}
	canceledAt := r.now.AddDate(0, 0, -7)
	tenantID, _ := primitive.ObjectIDFromHex(r.manifest.Tenants["seed-canceled"])
	r.db.Tenants().UpdateOne(ctx, bson.M{"_id": tenantID}, bson.M{"$set": bson.M{"canceledAt": canceledAt}})
	return nil
}

func (r *runner) seedEnterpriseTenant(ctx context.Context) error {
	// Enterprise: billing waived, all entitlements pass — custom contract model
	return r.seedTenant(ctx, tenantSpec{
		slug: "seed-enterprise", name: "Enterprise Corp",
		billing: models.BillingStatusNone, billingWaived: true, planKey: "enterprise",
		ownerEmail: "enterprise@seed.local", ownerName: "Enterprise Owner", manifestKey: "enterpriseOwner",
	})
}

// ── Team tenant (multi-role members + pending invite) ─────────────────────────

func (r *runner) seedTeamTenant(ctx context.Context) error {
	if err := r.seedTenant(ctx, tenantSpec{
		slug: "seed-team", name: "Team SaaS Inc",
		billing: models.BillingStatusActive, planKey: "pro", subCredits: 500,
		ownerEmail: "team-owner@seed.local", ownerName: "Team Owner", manifestKey: "teamOwner",
	}); err != nil {
		return err
	}

	tenantID, _ := primitive.ObjectIDFromHex(r.manifest.Tenants["seed-team"])
	ownerID, _ := primitive.ObjectIDFromHex(r.manifest.Accounts["teamOwner"].UserID)

	admin, err := r.createUser(ctx, "Team Admin", "team-admin@seed.local")
	if err != nil {
		return err
	}
	r.createMembership(ctx, admin.ID, tenantID, models.RoleAdmin)
	r.manifest.Accounts["teamAdmin"] = Account{
		Email: admin.Email, Password: SeedPassword,
		UserID: admin.ID.Hex(), TenantID: tenantID.Hex(),
	}

	member, err := r.createUser(ctx, "Team Member", "team-member@seed.local")
	if err != nil {
		return err
	}
	r.createMembership(ctx, member.ID, tenantID, models.RoleUser)
	r.manifest.Accounts["teamMember"] = Account{
		Email: member.Email, Password: SeedPassword,
		UserID: member.ID.Hex(), TenantID: tenantID.Hex(),
	}

	// Pending invitation — tests invite accept flow
	invite := models.Invitation{
		ID: primitive.NewObjectID(), TenantID: tenantID,
		Email: "pending-invite@seed.local", Role: models.RoleUser,
		Token: randHex(32), Status: models.InvitationPending,
		InvitedBy: ownerID,
		ExpiresAt: r.now.Add(7 * 24 * time.Hour), CreatedAt: r.now,
		SeedTag: SeedTag,
	}
	r.db.Invitations().InsertOne(ctx, invite)
	return nil
}

// ── AI / Credit tenants ───────────────────────────────────────────────────────

func (r *runner) seedCreditsFullTenant(ctx context.Context) error {
	return r.seedTenant(ctx, tenantSpec{
		slug: "seed-ai-full", name: "AI Full Credits Co",
		billing: models.BillingStatusActive, planKey: "ai_credits",
		subCredits: 1000, purchCredits: 500,
		ownerEmail: "ai-full@seed.local", ownerName: "AI Full Owner", manifestKey: "aiFullOwner",
	})
}

func (r *runner) seedCreditsLowTenant(ctx context.Context) error {
	// 18 credits — triggers low-credit warning UI
	return r.seedTenant(ctx, tenantSpec{
		slug: "seed-ai-low", name: "AI Low Credits Co",
		billing: models.BillingStatusActive, planKey: "ai_credits",
		subCredits: 18, purchCredits: 0,
		ownerEmail: "ai-low@seed.local", ownerName: "AI Low Owner", manifestKey: "aiLowOwner",
	})
}

func (r *runner) seedCreditsEmptyTenant(ctx context.Context) error {
	// 0 credits — all AI features blocked, triggers purchase prompt
	return r.seedTenant(ctx, tenantSpec{
		slug: "seed-ai-empty", name: "AI Empty Credits Co",
		billing: models.BillingStatusActive, planKey: "ai_credits",
		subCredits: 0, purchCredits: 0,
		ownerEmail: "ai-empty@seed.local", ownerName: "AI Empty Owner", manifestKey: "aiEmptyOwner",
	})
}

// ── API key tenant ────────────────────────────────────────────────────────────

func (r *runner) seedAPIKeyTenant(ctx context.Context) error {
	if err := r.seedTenant(ctx, tenantSpec{
		slug: "seed-apikey", name: "API Key Tenant",
		billing: models.BillingStatusActive, planKey: "pro", subCredits: 200,
		ownerEmail: "apikey-owner@seed.local", ownerName: "APIKey Owner", manifestKey: "apiKeyOwner",
	}); err != nil {
		return err
	}

	ownerID, _ := primitive.ObjectIDFromHex(r.manifest.Accounts["apiKeyOwner"].UserID)
	rawKey := "lsk_seed_" + randHex(32)
	h := sha256hex(rawKey)

	apiKey := models.APIKey{
		ID:         primitive.NewObjectID(),
		Name:       "Seed API Key",
		KeyHash:    h,
		KeyPreview: rawKey[:12] + "...",
		Authority:  models.APIKeyAuthorityAdmin,
		CreatedBy:  ownerID,
		IsActive:   true,
		CreatedAt:  r.now,
		SeedTag:    SeedTag,
	}
	if _, err := r.db.APIKeys().InsertOne(ctx, apiKey); err != nil {
		return err
	}

	acct := r.manifest.Accounts["apiKeyOwner"]
	acct.ApiKey = rawKey
	r.manifest.Accounts["apiKeyOwner"] = acct
	return nil
}

// ── Jobs (every status) ───────────────────────────────────────────────────────

func (r *runner) seedJobs(ctx context.Context) error {
	tenantID, _ := primitive.ObjectIDFromHex(r.manifest.Tenants["seed-active"])

	type jobSpec struct {
		status   models.JobStatus
		key      string
		attempts int
		lastErr  string
		result   map[string]interface{}
	}
	specs := []jobSpec{
		{models.JobStatusPending, "pending", 0, "", nil},
		{models.JobStatusRunning, "running", 0, "", nil},
		{models.JobStatusCompleted, "completed", 1, "", map[string]interface{}{"rows": 42}},
		{models.JobStatusFailed, "failed", 1, "connection timeout", nil},
		{models.JobStatusDead, "dead", 3, "max retries exceeded", nil},
		{models.JobStatusCancelled, "cancelled", 0, "", nil},
	}

	for _, s := range specs {
		job := models.Job{
			ID: primitive.NewObjectID(), TenantID: tenantID,
			Type: "seed.report", Payload: map[string]interface{}{"scenario": s.key},
			Status: s.status, RunAt: r.now,
			Attempts: s.attempts, MaxAttempts: 3,
			LastError: s.lastErr, Result: s.result,
			CreatedAt: r.now, UpdatedAt: r.now, SeedTag: SeedTag,
		}
		if s.status == models.JobStatusRunning {
			locked := r.now.Add(5 * time.Minute)
			job.LockedUntil = &locked
		}
		if s.status == models.JobStatusCompleted {
			ct := r.now
			job.CompletedAt = &ct
		}
		if _, err := r.db.Jobs().InsertOne(ctx, job); err != nil {
			return err
		}
		r.manifest.Jobs[s.key] = job.ID.Hex()
	}
	return nil
}

// ── Cron schedules ────────────────────────────────────────────────────────────

func (r *runner) seedCronSchedules(ctx context.Context) error {
	tenantID, _ := primitive.ObjectIDFromHex(r.manifest.Tenants["seed-active"])
	ownerID, _ := primitive.ObjectIDFromHex(r.manifest.Accounts["activeOwner"].UserID)

	schedules := []models.CronSchedule{
		{
			ID: primitive.NewObjectID(), TenantID: tenantID, CreatedBy: ownerID,
			Name: "Seed: Daily Report", Expression: "0 9 * * *", Timezone: "UTC",
			JobType: "seed.report", MaxAttempts: 3, IsActive: true,
			NextRunAt: r.now.Add(24 * time.Hour), CreatedAt: r.now, UpdatedAt: r.now,
			SeedTag: SeedTag,
		},
		{
			ID: primitive.NewObjectID(), TenantID: tenantID, CreatedBy: ownerID,
			Name: "Seed: Paused Sync (inactive)", Expression: "*/30 * * * *", Timezone: "UTC",
			JobType: "seed.sync", MaxAttempts: 2, IsActive: false,
			NextRunAt: r.now.Add(30 * time.Minute), CreatedAt: r.now, UpdatedAt: r.now,
			SeedTag: SeedTag,
		},
	}
	for _, s := range schedules {
		if _, err := r.db.CronSchedules().InsertOne(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

// ── Low-level helpers ─────────────────────────────────────────────────────────

func (r *runner) createUser(ctx context.Context, displayName, email string) (models.User, error) {
	user := models.User{
		ID: primitive.NewObjectID(), Email: email, DisplayName: displayName,
		PasswordHash: r.pwHash, AuthMethods: []models.AuthMethod{models.AuthMethodPassword},
		EmailVerified: true, IsActive: true,
		EmailPreferences: models.EmailPreferences{Marketing: true},
		UnsubscribeToken: randHex(24),
		CreatedAt: r.now, UpdatedAt: r.now, SeedTag: SeedTag,
	}
	if _, err := r.db.Users().InsertOne(ctx, user); err != nil {
		return models.User{}, fmt.Errorf("insert user %s: %w", email, err)
	}
	return user, nil
}

func (r *runner) createMembership(ctx context.Context, userID, tenantID primitive.ObjectID, role models.MemberRole) error {
	m := models.TenantMembership{
		ID: primitive.NewObjectID(), UserID: userID, TenantID: tenantID,
		Role: role, JoinedAt: r.now, UpdatedAt: r.now, SeedTag: SeedTag,
	}
	_, err := r.db.TenantMemberships().InsertOne(ctx, m)
	return err
}

// mintTokens generates a long-lived access + refresh token for every account in
// the manifest and stores them back. Tokens use a 24h TTL so a single seed run
// covers a full day of test runs without needing re-authentication.
func (r *runner) mintTokens() error {
	for key, acct := range r.manifest.Accounts {
		userID := acct.UserID
		// Fetch display name from DB so the JWT claims are realistic.
		oid, err := primitive.ObjectIDFromHex(userID)
		if err != nil {
			return fmt.Errorf("invalid userId for %s: %w", key, err)
		}
		var user models.User
		if err := r.db.Users().FindOne(context.Background(), bson.M{"_id": oid}).Decode(&user); err != nil {
			return fmt.Errorf("fetch user %s: %w", key, err)
		}
		access, err := r.jwtSvc.GenerateAccessTokenWithTTL(userID, user.Email, user.DisplayName, 24*time.Hour)
		if err != nil {
			return fmt.Errorf("access token for %s: %w", key, err)
		}
		refresh, err := r.jwtSvc.GenerateRefreshTokenWithTTL(userID, 7*24*time.Hour)
		if err != nil {
			return fmt.Errorf("refresh token for %s: %w", key, err)
		}
		updated := acct
		updated.AccessToken = access
		updated.RefreshToken = refresh
		r.manifest.Accounts[key] = updated
	}
	return nil
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func sha256hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func writeManifest(path string, m Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
