package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"lastsaas/internal/cron"
	"lastsaas/internal/db"
	"lastsaas/internal/middleware"
	"lastsaas/internal/models"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CronHandler manages recurring schedule CRUD for tenant-scoped cron jobs.
type CronHandler struct {
	db *db.MongoDB
}

// NewCronHandler creates a CronHandler.
func NewCronHandler(database *db.MongoDB) *CronHandler {
	return &CronHandler{db: database}
}

// ListSchedules returns all cron schedules for the current tenant.
// GET /api/tenant/cron-schedules
func (h *CronHandler) ListSchedules(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.GetTenantFromContext(r.Context())

	cursor, err := h.db.CronSchedules().Find(r.Context(),
		bson.M{"tenantId": tenant.ID},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(200),
	)
	if err != nil {
		http.Error(w, "Failed to list schedules", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(r.Context())

	var schedules []models.CronSchedule
	if err := cursor.All(r.Context(), &schedules); err != nil {
		http.Error(w, "Failed to decode schedules", http.StatusInternalServerError)
		return
	}
	if schedules == nil {
		schedules = []models.CronSchedule{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(schedules)
}

type createScheduleRequest struct {
	Name        string                 `json:"name"`
	Expression  string                 `json:"expression"`
	Timezone    string                 `json:"timezone"`
	JobType     string                 `json:"jobType"`
	Payload     map[string]interface{} `json:"payload"`
	MaxAttempts int                    `json:"maxAttempts"`
}

// CreateSchedule creates a new cron schedule for the current tenant.
// POST /api/tenant/cron-schedules
func (h *CronHandler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.GetTenantFromContext(r.Context())
	user, _ := middleware.GetUserFromContext(r.Context())

	var req createScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Expression == "" || req.JobType == "" {
		http.Error(w, "name, expression, and jobType are required", http.StatusBadRequest)
		return
	}
	if req.Timezone == "" {
		req.Timezone = "UTC"
	}
	if req.MaxAttempts == 0 {
		req.MaxAttempts = 3
	}

	nextRun, err := cron.NextRunTime(req.Expression, req.Timezone)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	now := time.Now()
	schedule := models.CronSchedule{
		ID:          primitive.NewObjectID(),
		TenantID:    tenant.ID,
		CreatedBy:   user.ID,
		Name:        req.Name,
		Expression:  req.Expression,
		Timezone:    req.Timezone,
		JobType:     req.JobType,
		Payload:     req.Payload,
		MaxAttempts: req.MaxAttempts,
		IsActive:    true,
		NextRunAt:   nextRun,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if _, err := h.db.CronSchedules().InsertOne(r.Context(), schedule); err != nil {
		http.Error(w, "Failed to create schedule", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(schedule)
}

// GetSchedule returns a single cron schedule by ID.
// GET /api/tenant/cron-schedules/{id}
func (h *CronHandler) GetSchedule(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.GetTenantFromContext(r.Context())
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid schedule ID", http.StatusBadRequest)
		return
	}

	var schedule models.CronSchedule
	if err := h.db.CronSchedules().FindOne(r.Context(),
		bson.M{"_id": id, "tenantId": tenant.ID},
	).Decode(&schedule); err != nil {
		http.Error(w, "Schedule not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(schedule)
}

type updateScheduleRequest struct {
	Name        *string                `json:"name"`
	Expression  *string                `json:"expression"`
	Timezone    *string                `json:"timezone"`
	JobType     *string                `json:"jobType"`
	Payload     map[string]interface{} `json:"payload"`
	MaxAttempts *int                   `json:"maxAttempts"`
}

// UpdateSchedule modifies fields on an existing schedule and recomputes NextRunAt
// if the expression or timezone changed.
// PATCH /api/tenant/cron-schedules/{id}
func (h *CronHandler) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.GetTenantFromContext(r.Context())
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid schedule ID", http.StatusBadRequest)
		return
	}

	var existing models.CronSchedule
	if err := h.db.CronSchedules().FindOne(r.Context(),
		bson.M{"_id": id, "tenantId": tenant.ID},
	).Decode(&existing); err != nil {
		http.Error(w, "Schedule not found", http.StatusNotFound)
		return
	}

	var req updateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	set := bson.M{"updatedAt": time.Now()}
	if req.Name != nil {
		set["name"] = *req.Name
	}
	if req.JobType != nil {
		set["jobType"] = *req.JobType
	}
	if req.Payload != nil {
		set["payload"] = req.Payload
	}
	if req.MaxAttempts != nil {
		set["maxAttempts"] = *req.MaxAttempts
	}

	expr := existing.Expression
	tz := existing.Timezone
	if req.Expression != nil {
		expr = *req.Expression
		set["expression"] = expr
	}
	if req.Timezone != nil {
		tz = *req.Timezone
		set["timezone"] = tz
	}
	if req.Expression != nil || req.Timezone != nil {
		nextRun, err := cron.NextRunTime(expr, tz)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		set["nextRunAt"] = nextRun
	}

	var updated models.CronSchedule
	if err := h.db.CronSchedules().FindOneAndUpdate(r.Context(),
		bson.M{"_id": id, "tenantId": tenant.ID},
		bson.M{"$set": set},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&updated); err != nil {
		http.Error(w, "Failed to update schedule", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

// DeleteSchedule permanently removes a cron schedule.
// DELETE /api/tenant/cron-schedules/{id}
func (h *CronHandler) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.GetTenantFromContext(r.Context())
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid schedule ID", http.StatusBadRequest)
		return
	}

	result, err := h.db.CronSchedules().DeleteOne(r.Context(),
		bson.M{"_id": id, "tenantId": tenant.ID},
	)
	if err != nil {
		http.Error(w, "Failed to delete schedule", http.StatusInternalServerError)
		return
	}
	if result.DeletedCount == 0 {
		http.Error(w, "Schedule not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// PauseSchedule deactivates a schedule without deleting it.
// POST /api/tenant/cron-schedules/{id}/pause
func (h *CronHandler) PauseSchedule(w http.ResponseWriter, r *http.Request) {
	h.setActive(w, r, false)
}

// ResumeSchedule reactivates a paused schedule and recomputes NextRunAt.
// POST /api/tenant/cron-schedules/{id}/resume
func (h *CronHandler) ResumeSchedule(w http.ResponseWriter, r *http.Request) {
	h.setActive(w, r, true)
}

func (h *CronHandler) setActive(w http.ResponseWriter, r *http.Request, active bool) {
	tenant, _ := middleware.GetTenantFromContext(r.Context())
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid schedule ID", http.StatusBadRequest)
		return
	}

	set := bson.M{"isActive": active, "updatedAt": time.Now()}

	if active {
		var existing models.CronSchedule
		if err := h.db.CronSchedules().FindOne(r.Context(),
			bson.M{"_id": id, "tenantId": tenant.ID},
		).Decode(&existing); err != nil {
			http.Error(w, "Schedule not found", http.StatusNotFound)
			return
		}
		nextRun, err := cron.NextRunTime(existing.Expression, existing.Timezone)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		set["nextRunAt"] = nextRun
	}

	var updated models.CronSchedule
	if err := h.db.CronSchedules().FindOneAndUpdate(r.Context(),
		bson.M{"_id": id, "tenantId": tenant.ID},
		bson.M{"$set": set},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&updated); err != nil {
		http.Error(w, "Schedule not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}
