package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DocumentVisibility controls who within the tenant can access a document.
type DocumentVisibility string

const (
	DocumentVisibilityTenant DocumentVisibility = "tenant" // any tenant member
	DocumentVisibilityOwner  DocumentVisibility = "owner"  // uploader only
)

// Document stores metadata for a tenant-scoped private file upload.
// The actual bytes live in the object store (or MongoDB Data field for local dev).
// StorageKey is namespaced as "documents/{tenantId}/{docId}" — never exposed to clients.
// Access always goes through the authenticated /download endpoint; no public CDN URL.
type Document struct {
	ID          primitive.ObjectID `json:"id"          bson:"_id,omitempty"`
	TenantID    primitive.ObjectID `json:"tenantId"    bson:"tenantId"    validate:"required"`
	OwnerID     primitive.ObjectID `json:"ownerId"     bson:"ownerId"     validate:"required"`
	StorageKey  string             `json:"storageKey,omitempty" bson:"storageKey,omitempty"`
	Filename    string             `json:"filename"    bson:"filename"    validate:"required,min=1,max=500"`
	ContentType string             `json:"contentType" bson:"contentType" validate:"required,min=1,max=200"`
	Size        int64              `json:"size"        bson:"size"        validate:"required,gte=1"`
	Visibility  DocumentVisibility `json:"visibility"  bson:"visibility"  validate:"required,oneof=tenant owner"`
	// Legacy: only present on docs uploaded when objectstore provider is "db".
	Data      []byte    `json:"-"           bson:"data,omitempty"`
	CreatedAt time.Time `json:"createdAt"   bson:"createdAt"   validate:"required"`
	UpdatedAt time.Time `json:"updatedAt"   bson:"updatedAt"   validate:"required"`
}
