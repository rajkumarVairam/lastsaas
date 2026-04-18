package objectstore

import (
	"context"
	"time"
)

// dbFallback is the Store used when no object store is configured.
// It signals callers to store bytes in MongoDB instead (the legacy path).
// Put always returns an empty URL so the handler knows to write Data to the DB.
// Delete is a no-op — DB documents are removed by the handler directly.
// PresignGet returns ("", nil) — callers must proxy the download from MongoDB directly.
//
// This keeps local dev working with zero config while production uses R2 or S3.
type dbFallback struct{}

func (d *dbFallback) Put(_ context.Context, _ string, _ []byte, _ string) (string, error) {
	return "", nil // empty URL signals "store in DB"
}

func (d *dbFallback) Delete(_ context.Context, _ string) error {
	return nil // handler deletes the DB document itself
}

func (d *dbFallback) PresignGet(_ context.Context, _ string, _ time.Duration, _ string) (string, error) {
	return "", nil // signals "proxy from MongoDB"
}

func (d *dbFallback) Ping(_ context.Context) error { return nil } // no external service

func (d *dbFallback) Provider() string { return "db" }
