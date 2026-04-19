package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"saasquickstart/internal/db"
	"saasquickstart/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Watcher listens for job status changes via a MongoDB change stream and
// publishes events to the Hub. Running a Watcher on every node ensures that
// SSE clients receive updates regardless of which node processed the job.
type Watcher struct {
	db   *db.MongoDB
	hub  *Hub
}

// NewWatcher creates a Watcher that will publish job events to hub.
func NewWatcher(database *db.MongoDB, hub *Hub) *Watcher {
	return &Watcher{db: database, hub: hub}
}

// Start launches the watcher goroutine. Reconnects automatically on error.
// Returns when ctx is cancelled.
func (w *Watcher) Start(ctx context.Context) {
	go func() {
		backoff := 2 * time.Second
		for {
			if ctx.Err() != nil {
				return
			}
			err := w.watchOnce(ctx)
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				if isChangeStreamUnsupported(err) {
					slog.Info("sse: change streams unavailable, SSE will rely on client polling fallback", "reason", err)
					return
				}
				slog.Warn("sse: change stream interrupted, reconnecting", "error", err, "backoff", backoff)
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					return
				}
				if backoff < 60*time.Second {
					backoff *= 2
				}
			} else {
				backoff = 2 * time.Second
			}
		}
	}()
	slog.Info("sse watcher started")
}

func (w *Watcher) watchOnce(ctx context.Context) error {
	// Watch only update operations that change status to a terminal state.
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "operationType", Value: bson.D{{Key: "$in", Value: bson.A{"update", "replace"}}}},
		}}},
	}
	stream, err := w.db.Jobs().Watch(ctx, pipeline,
		options.ChangeStream().SetFullDocument(options.UpdateLookup),
	)
	if err != nil {
		return fmt.Errorf("open change stream: %w", err)
	}
	defer stream.Close(ctx)

	slog.Info("sse: job change stream active")
	for stream.Next(ctx) {
		var event struct {
			OperationType string      `bson:"operationType"`
			FullDocument  *models.Job `bson:"fullDocument"`
		}
		if err := stream.Decode(&event); err != nil {
			slog.Warn("sse: failed to decode change event", "error", err)
			continue
		}
		if event.FullDocument == nil {
			continue
		}
		job := event.FullDocument
		if job.Status != models.JobStatusCompleted && job.Status != models.JobStatusFailed && job.Status != models.JobStatusDead {
			continue
		}
		w.publishJobEvent(job)
	}
	return stream.Err()
}

func (w *Watcher) publishJobEvent(job *models.Job) {
	eventName := "job." + string(job.Status)

	payload := map[string]interface{}{
		"id":     job.ID.Hex(),
		"type":   job.Type,
		"status": string(job.Status),
	}
	if job.Status == models.JobStatusFailed || job.Status == models.JobStatusDead {
		payload["error"] = job.LastError
	}
	if job.Result != nil {
		payload["result"] = job.Result
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	w.hub.Publish(Event{
		TenantID: job.TenantID.Hex(),
		Name:     eventName,
		Data:     string(data),
	})
}

func isChangeStreamUnsupported(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "replica set") ||
		strings.Contains(msg, "changeStream") ||
		strings.Contains(msg, "IllegalOperation") ||
		strings.Contains(msg, "not supported")
}
