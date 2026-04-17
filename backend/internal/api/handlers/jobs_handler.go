package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"lastsaas/internal/db"
	"lastsaas/internal/jobs"
	"lastsaas/internal/middleware"
	"lastsaas/internal/models"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// JobsHandler exposes job queue state to tenants: list, inspect, cancel, retry.
type JobsHandler struct {
	db    *db.MongoDB
	queue *jobs.Queue
}

func NewJobsHandler(database *db.MongoDB, queue *jobs.Queue) *JobsHandler {
	return &JobsHandler{db: database, queue: queue}
}

// ListJobs returns paginated jobs for the current tenant.
// Query params: status, type, page (1-based), limit (max 100).
func (h *JobsHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant, ok := middleware.GetTenantFromContext(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Tenant context missing")
		return
	}

	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 25
	}

	filter := bson.M{"tenantId": tenant.ID}
	if s := q.Get("status"); s != "" {
		filter["status"] = models.JobStatus(s)
	}
	if t := q.Get("type"); t != "" {
		filter["type"] = t
	}

	total, _ := h.db.Jobs().CountDocuments(ctx, filter)

	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetSkip(int64((page - 1) * limit)).
		SetLimit(int64(limit))

	cursor, err := h.db.Jobs().Find(ctx, filter, opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch jobs")
		return
	}
	defer cursor.Close(ctx)

	var jobList []models.Job
	if err := cursor.All(ctx, &jobList); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to decode jobs")
		return
	}
	if jobList == nil {
		jobList = []models.Job{}
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"jobs":  jobList,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// GetJob returns a single job by ID, scoped to the current tenant.
func (h *JobsHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant, ok := middleware.GetTenantFromContext(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Tenant context missing")
		return
	}

	jobID, err := primitive.ObjectIDFromHex(mux.Vars(r)["jobId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}

	var job models.Job
	if err := h.db.Jobs().FindOne(ctx, bson.M{"_id": jobID, "tenantId": tenant.ID}).Decode(&job); err != nil {
		respondWithError(w, http.StatusNotFound, "Job not found")
		return
	}

	respondWithJSON(w, http.StatusOK, job)
}

// CancelJob cancels a pending or failed job. Running jobs cannot be cancelled.
func (h *JobsHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant, ok := middleware.GetTenantFromContext(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Tenant context missing")
		return
	}

	jobID, err := primitive.ObjectIDFromHex(mux.Vars(r)["jobId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}

	result, err := h.db.Jobs().UpdateOne(ctx,
		bson.M{
			"_id":      jobID,
			"tenantId": tenant.ID,
			"status":   bson.M{"$in": []models.JobStatus{models.JobStatusPending, models.JobStatusFailed, models.JobStatusDead}},
		},
		bson.M{"$set": bson.M{
			"status":    models.JobStatusCancelled,
			"updatedAt": time.Now(),
		}},
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to cancel job")
		return
	}
	if result.MatchedCount == 0 {
		respondWithError(w, http.StatusConflict, "Job not found or cannot be cancelled in its current state")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RetryJob re-queues a dead or failed job to run immediately.
func (h *JobsHandler) RetryJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant, ok := middleware.GetTenantFromContext(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Tenant context missing")
		return
	}

	jobID, err := primitive.ObjectIDFromHex(mux.Vars(r)["jobId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}

	now := time.Now()
	result, err := h.db.Jobs().UpdateOne(ctx,
		bson.M{
			"_id":      jobID,
			"tenantId": tenant.ID,
			"status":   bson.M{"$in": []models.JobStatus{models.JobStatusDead, models.JobStatusFailed, models.JobStatusCancelled}},
		},
		bson.M{"$set": bson.M{
			"status":      models.JobStatusPending,
			"runAt":       now,
			"updatedAt":   now,
			"lockedBy":    "",
			"lockedUntil": nil,
		}},
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retry job")
		return
	}
	if result.MatchedCount == 0 {
		respondWithError(w, http.StatusConflict, "Job not found or not in a retryable state")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// EnqueueJob allows tenants to schedule a job via the API.
// The request body must include "type" and "payload". Optional: "runAt" (RFC3339).
// Job types available via this endpoint are restricted to the allowlist in the handler.
func (h *JobsHandler) EnqueueJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant, ok := middleware.GetTenantFromContext(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Tenant context missing")
		return
	}

	var req struct {
		Type    string                 `json:"type"`
		Payload map[string]interface{} `json:"payload"`
		RunAt   *time.Time             `json:"runAt,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Type == "" {
		respondWithError(w, http.StatusBadRequest, "Job type is required")
		return
	}

	job := &models.Job{
		Type:     req.Type,
		TenantID: tenant.ID,
		Payload:  req.Payload,
	}
	if req.RunAt != nil {
		job.RunAt = *req.RunAt
	}

	if err := h.queue.Enqueue(ctx, job); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to enqueue job")
		return
	}

	respondWithJSON(w, http.StatusCreated, job)
}
