package objectstore

import (
	"context"
	"fmt"
	"time"

	"saasquickstart/internal/config"
)

// Store is the provider-agnostic interface for object storage.
// All upload and delete operations go through this — handlers never talk to S3/R2 directly.
//
// Adding a new provider (GCS, Azure Blob, etc.):
//  1. Create a new file in this package implementing Store
//  2. Add the provider name to the config and the New switch below
//  3. Nothing else changes
type Store interface {
	// Put uploads data and returns its public URL (empty string for db-fallback provider).
	Put(ctx context.Context, key string, data []byte, contentType string) (url string, err error)
	// Delete removes an object. Implementations must treat "not found" as success.
	Delete(ctx context.Context, key string) error
	// PresignGet returns a time-limited signed URL for private object access.
	// filename is embedded in the signed URL as Content-Disposition so browsers save
	// the file with the correct name. Pass "" to omit it.
	// Returns ("", nil) on the db-fallback provider — callers must proxy from MongoDB in that case.
	PresignGet(ctx context.Context, key string, ttl time.Duration, filename string) (url string, err error)
	// Ping verifies connectivity and credentials without touching user data.
	// Used by the health service to surface misconfiguration early.
	// Always returns nil for the db-fallback provider.
	Ping(ctx context.Context) error
	// Provider returns a human-readable name for logging and health checks.
	Provider() string
}

// New creates a Store based on cfg.ObjectStore.Provider:
//
//	"r2"  — Cloudflare R2 (uses AWS SDK v2 with R2 endpoint)
//	"s3"  — AWS S3      (uses AWS SDK v2 with standard AWS endpoint)
//	"db"  — MongoDB fallback (stores bytes in the database, for local dev)
//	""    — same as "db"
func New(cfg config.ObjectStoreConfig) (Store, error) {
	switch cfg.Provider {
	case "r2":
		if cfg.AccountID == "" {
			return nil, fmt.Errorf("objectstore: r2 provider requires account_id")
		}
		if cfg.AccessKey == "" || cfg.SecretKey == "" {
			return nil, fmt.Errorf("objectstore: r2 provider requires access_key and secret_key")
		}
		if cfg.Bucket == "" {
			return nil, fmt.Errorf("objectstore: r2 provider requires bucket")
		}
		if cfg.PublicURL == "" {
			return nil, fmt.Errorf("objectstore: r2 provider requires public_url")
		}
		endpoint := cfg.Endpoint
		if endpoint == "" {
			endpoint = fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)
		}
		return newS3Compatible(s3CompatConfig{
			accessKey: cfg.AccessKey,
			secretKey: cfg.SecretKey,
			bucket:    cfg.Bucket,
			publicURL: cfg.PublicURL,
			endpoint:  endpoint,
			region:    "auto",
			provider:  "r2",
		})

	case "s3":
		if cfg.AccessKey == "" || cfg.SecretKey == "" {
			return nil, fmt.Errorf("objectstore: s3 provider requires access_key and secret_key")
		}
		if cfg.Bucket == "" {
			return nil, fmt.Errorf("objectstore: s3 provider requires bucket")
		}
		if cfg.Region == "" {
			return nil, fmt.Errorf("objectstore: s3 provider requires region")
		}
		if cfg.PublicURL == "" {
			return nil, fmt.Errorf("objectstore: s3 provider requires public_url")
		}
		return newS3Compatible(s3CompatConfig{
			accessKey: cfg.AccessKey,
			secretKey: cfg.SecretKey,
			bucket:    cfg.Bucket,
			publicURL: cfg.PublicURL,
			endpoint:  cfg.Endpoint, // empty = AWS default
			region:    cfg.Region,
			provider:  "s3",
		})

	case "db", "":
		return &dbFallback{}, nil

	default:
		return nil, fmt.Errorf("objectstore: unknown provider %q (valid: r2, s3, db)", cfg.Provider)
	}
}
