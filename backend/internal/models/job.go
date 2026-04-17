package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusDead      JobStatus = "dead"      // exhausted all retries
	JobStatusCancelled JobStatus = "cancelled"
)

// Job is a durable, tenant-scoped unit of work stored in MongoDB.
// Handlers must treat Execute as idempotent: check Result before calling external APIs
// on retry (Attempts > 0) to prevent duplicate side-effects.
type Job struct {
	ID          primitive.ObjectID     `bson:"_id"                    json:"id"`
	Type        string                 `bson:"type"                   json:"type"        validate:"required,min=1,max=100"`
	TenantID    primitive.ObjectID     `bson:"tenantId"               json:"tenantId"    validate:"required"`
	Payload     map[string]interface{} `bson:"payload"                json:"payload"`
	Status      JobStatus              `bson:"status"                 json:"status"      validate:"required,oneof=pending running completed failed dead cancelled"`
	RunAt       time.Time              `bson:"runAt"                  json:"runAt"       validate:"required"`
	LockedBy    string                 `bson:"lockedBy,omitempty"     json:"lockedBy,omitempty"`
	LockedUntil *time.Time             `bson:"lockedUntil,omitempty"  json:"lockedUntil,omitempty"`
	Attempts    int                    `bson:"attempts"               json:"attempts"`
	MaxAttempts int                    `bson:"maxAttempts"            json:"maxAttempts" validate:"required,gte=1,lte=20"`
	LastError   string                 `bson:"lastError,omitempty"    json:"lastError,omitempty"`
	// Result is written by the handler on success. On retry, handlers check this
	// before calling external APIs to detect already-completed work.
	Result      map[string]interface{} `bson:"result,omitempty"       json:"result,omitempty"`
	CompletedAt *time.Time             `bson:"completedAt,omitempty"  json:"completedAt,omitempty"`
	CreatedAt   time.Time              `bson:"createdAt"              json:"createdAt"   validate:"required"`
	UpdatedAt   time.Time              `bson:"updatedAt"              json:"updatedAt"   validate:"required"`
}
