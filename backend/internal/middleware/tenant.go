package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"saasquickstart/internal/cache"
	"saasquickstart/internal/db"
	"saasquickstart/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	TenantContextKey     contextKey = "tenant"
	MembershipContextKey contextKey = "membership"

	tenantCacheTTL     = 30 * time.Second
	membershipCacheTTL = 30 * time.Second
	planCacheTTL       = 5 * time.Minute
)

// TenantMiddleware resolves the tenant and membership for each request.
// Results are cached in-memory to avoid repeated DB lookups on the hot path:
//   - Tenant:     30s TTL  (billing/plan changes propagate within 30s)
//   - Membership: 30s TTL  (role changes propagate within 30s)
//   - Plan:       5min TTL (entitlement changes propagate within 5min)
type TenantMiddleware struct {
	db              *db.MongoDB
	tenantCache     *cache.TTL[primitive.ObjectID, models.Tenant]
	membershipCache *cache.TTL[string, models.TenantMembership]
	planCache       *cache.TTL[primitive.ObjectID, models.Plan]
}

func NewTenantMiddleware(database *db.MongoDB) *TenantMiddleware {
	return &TenantMiddleware{
		db:              database,
		tenantCache:     cache.New[primitive.ObjectID, models.Tenant](tenantCacheTTL),
		membershipCache: cache.New[string, models.TenantMembership](membershipCacheTTL),
		planCache:       cache.New[primitive.ObjectID, models.Plan](planCacheTTL),
	}
}

// Stop terminates the background cache sweep goroutines.
func (m *TenantMiddleware) Stop() {
	m.tenantCache.Stop()
	m.membershipCache.Stop()
	m.planCache.Stop()
}

func (m *TenantMiddleware) RequireTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API key auth already resolved tenant + membership — pass through.
		if _, ok := GetTenantFromContext(r.Context()); ok {
			if _, ok := GetMembershipFromContext(r.Context()); ok {
				next.ServeHTTP(w, r)
				return
			}
		}

		tenantIDStr := r.Header.Get("X-Tenant-ID")
		if tenantIDStr == "" {
			http.Error(w, `{"error":"X-Tenant-ID header required"}`, http.StatusBadRequest)
			return
		}

		tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
		if err != nil {
			http.Error(w, `{"error":"Invalid tenant ID"}`, http.StatusBadRequest)
			return
		}

		// Tenant lookup — cache miss falls through to MongoDB.
		tenant, ok := m.tenantCache.Get(tenantID)
		if !ok {
			if err := m.db.Tenants().FindOne(r.Context(),
				bson.M{"_id": tenantID, "isActive": true},
			).Decode(&tenant); err != nil {
				http.Error(w, `{"error":"Tenant not found"}`, http.StatusNotFound)
				return
			}
			m.tenantCache.Set(tenantID, tenant)
		}

		user, ok := GetUserFromContext(r.Context())
		if !ok {
			http.Error(w, `{"error":"Not authenticated"}`, http.StatusUnauthorized)
			return
		}

		// Membership lookup — keyed by "userID:tenantID".
		membershipKey := user.ID.Hex() + ":" + tenantID.Hex()
		membership, ok := m.membershipCache.Get(membershipKey)
		if !ok {
			if err := m.db.TenantMemberships().FindOne(r.Context(), bson.M{
				"userId":   user.ID,
				"tenantId": tenantID,
			}).Decode(&membership); err != nil {
				http.Error(w, `{"error":"Not a member of this tenant"}`, http.StatusForbidden)
				return
			}
			m.membershipCache.Set(membershipKey, membership)
		}

		ctx := context.WithValue(r.Context(), TenantContextKey, &tenant)
		ctx = context.WithValue(ctx, MembershipContextKey, &membership)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetTenantFromContext(ctx context.Context) (*models.Tenant, bool) {
	tenant, ok := ctx.Value(TenantContextKey).(*models.Tenant)
	return tenant, ok
}

func GetMembershipFromContext(ctx context.Context) (*models.TenantMembership, bool) {
	membership, ok := ctx.Value(MembershipContextKey).(*models.TenantMembership)
	return membership, ok
}

// RequireActiveBilling blocks requests when the tenant's subscription is not
// active. Root tenants and billing-waived tenants are always allowed through.
func RequireActiveBilling() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenant, ok := GetTenantFromContext(r.Context())
			if !ok {
				http.Error(w, `{"error":"No tenant context"}`, http.StatusBadRequest)
				return
			}

			if tenant.IsRoot || tenant.BillingWaived {
				next.ServeHTTP(w, r)
				return
			}

			if tenant.BillingStatus == models.BillingStatusActive || tenant.BillingStatus == models.BillingStatusNone {
				next.ServeHTTP(w, r)
				return
			}

			http.Error(w, `{"error":"subscription_required","code":"BILLING_INACTIVE"}`, http.StatusPaymentRequired)
		})
	}
}

// RequireEntitlement checks whether the tenant's plan grants a specific boolean
// entitlement. Plan data is served from the TenantMiddleware plan cache (5min TTL).
func (m *TenantMiddleware) RequireEntitlement(feature string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenant, ok := GetTenantFromContext(r.Context())
			if !ok {
				http.Error(w, `{"error":"No tenant context"}`, http.StatusBadRequest)
				return
			}

			if tenant.IsRoot || tenant.BillingWaived {
				next.ServeHTTP(w, r)
				return
			}

			if tenant.PlanID == nil {
				http.Error(w, fmt.Sprintf(`{"error":"Feature '%s' requires an active plan","code":"ENTITLEMENT_REQUIRED"}`, feature), http.StatusForbidden)
				return
			}

			// Plan lookup — cache miss falls through to MongoDB.
			plan, ok := m.planCache.Get(*tenant.PlanID)
			if !ok {
				if err := m.db.Plans().FindOne(r.Context(),
					bson.M{"_id": *tenant.PlanID},
				).Decode(&plan); err != nil {
					http.Error(w, `{"error":"Plan not found"}`, http.StatusInternalServerError)
					return
				}
				m.planCache.Set(*tenant.PlanID, plan)
			}

			ent, exists := plan.Entitlements[feature]
			if !exists || (ent.Type == models.EntitlementTypeBool && !ent.BoolValue) {
				http.Error(w, fmt.Sprintf(`{"error":"Feature '%s' is not included in your plan","code":"ENTITLEMENT_REQUIRED"}`, feature), http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
