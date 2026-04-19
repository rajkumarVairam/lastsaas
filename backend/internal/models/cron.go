package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CronSchedule defines a recurring job that the scheduler re-enqueues automatically.
// The expression field uses standard 5-field cron syntax (minute hour dom month dow).
type CronSchedule struct {
	ID          primitive.ObjectID     `bson:"_id,omitempty"        json:"id"`
	TenantID    primitive.ObjectID     `bson:"tenantId"             json:"tenantId"    validate:"required"`
	CreatedBy   primitive.ObjectID     `bson:"createdBy"            json:"createdBy"   validate:"required"`
	Name        string                 `bson:"name"                 json:"name"        validate:"required,min=1,max=200"`
	Expression  string                 `bson:"expression"           json:"expression"  validate:"required,min=1,max=100"`
	Timezone    string                 `bson:"timezone"             json:"timezone"    validate:"required,min=1,max=100"`
	JobType     string                 `bson:"jobType"              json:"jobType"     validate:"required,min=1,max=100"`
	Payload     map[string]interface{} `bson:"payload,omitempty"    json:"payload,omitempty"`
	MaxAttempts int                    `bson:"maxAttempts"          json:"maxAttempts" validate:"required,gte=1,lte=20"`
	IsActive    bool                   `bson:"isActive"             json:"isActive"`
	NextRunAt   time.Time              `bson:"nextRunAt"            json:"nextRunAt"   validate:"required"`
	LastRunAt   *time.Time             `bson:"lastRunAt,omitempty"  json:"lastRunAt,omitempty"`
	LockedBy    string                 `bson:"lockedBy,omitempty"   json:"lockedBy,omitempty"`
	LockedUntil *time.Time             `bson:"lockedUntil,omitempty" json:"lockedUntil,omitempty"`
	CreatedAt   time.Time              `bson:"createdAt"            json:"createdAt"   validate:"required"`
	UpdatedAt   time.Time              `bson:"updatedAt"            json:"updatedAt"   validate:"required"`
	SeedTag   string             `json:"-" bson:"seedTag,omitempty"`
}
