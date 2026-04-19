package configstore

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"saasquickstart/internal/db"
	"saasquickstart/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Store is a thread-safe in-memory cache of configuration variables backed by MongoDB.
type Store struct {
	db    *db.MongoDB
	mu    sync.RWMutex
	cache map[string]models.ConfigVar
}

// New creates a Store. Call Load() to populate from DB.
func New(database *db.MongoDB) *Store {
	return &Store{
		db:    database,
		cache: make(map[string]models.ConfigVar),
	}
}

// Load reads all config vars from DB into the cache.
func (s *Store) Load(ctx context.Context) error {
	cursor, err := s.db.ConfigVars().Find(ctx, bson.M{})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	var vars []models.ConfigVar
	if err := cursor.All(ctx, &vars); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = make(map[string]models.ConfigVar, len(vars))
	for _, v := range vars {
		s.cache[v.Name] = v
	}
	return nil
}

// Get returns the value of a config variable by name.
// Returns "" if not found. This is the lightweight hot path — RLock only, no DB call.
func (s *Store) Get(name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if v, ok := s.cache[name]; ok {
		return v.Value
	}
	return ""
}

// GetVar returns the full ConfigVar struct from the cache.
func (s *Store) GetVar(name string) (models.ConfigVar, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.cache[name]
	return v, ok
}

// GetAll returns all cached config variables sorted by name.
func (s *Store) GetAll() []models.ConfigVar {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]models.ConfigVar, 0, len(s.cache))
	for _, v := range s.cache {
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Set updates a variable's value in DB and reloads it into the cache.
func (s *Store) Set(ctx context.Context, name, value string) error {
	now := time.Now()
	result, err := s.db.ConfigVars().UpdateOne(ctx,
		bson.M{"name": name},
		bson.M{"$set": bson.M{"value": value, "updatedAt": now}},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("config variable %q not found", name)
	}
	return s.Reload(ctx, name)
}

// Reload re-reads a single variable from DB into the cache.
func (s *Store) Reload(ctx context.Context, name string) error {
	var v models.ConfigVar
	err := s.db.ConfigVars().FindOne(ctx, bson.M{"name": name}).Decode(&v)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[v.Name] = v
	return nil
}

// StartAutoReload periodically reloads all config vars from the database.
// Acts as a fallback sync mechanism — WatchChanges provides real-time updates
// on replica sets; this catches any gaps (e.g. standalone MongoDB in local dev).
func (s *Store) StartAutoReload(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.Load(ctx); err != nil {
					slog.Warn("configstore: auto-reload failed", "error", err)
				}
			}
		}
	}()
}

// WatchChanges opens a MongoDB change stream and applies config updates to the
// cache in real time (< 100ms propagation on replica sets / Atlas).
// If change streams are unavailable (standalone MongoDB used in local dev), it
// logs once and returns — StartAutoReload acts as the fallback.
// The goroutine exits when ctx is cancelled.
func (s *Store) WatchChanges(ctx context.Context) {
	go func() {
		backoff := 2 * time.Second
		for {
			if ctx.Err() != nil {
				return
			}
			err := s.watchOnce(ctx)
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				if isChangeStreamUnsupported(err) {
					slog.Info("configstore: change streams unavailable, using polling fallback", "reason", err)
					return
				}
				slog.Warn("configstore: change stream interrupted, reconnecting", "error", err, "backoff", backoff)
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
}

func (s *Store) watchOnce(ctx context.Context) error {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "operationType", Value: bson.D{{Key: "$in", Value: bson.A{"insert", "update", "replace", "delete"}}}},
		}}},
	}
	stream, err := s.db.ConfigVars().Watch(ctx, pipeline,
		options.ChangeStream().SetFullDocument(options.UpdateLookup),
	)
	if err != nil {
		return fmt.Errorf("open change stream: %w", err)
	}
	defer stream.Close(ctx)

	slog.Info("configstore: change stream active — real-time config sync enabled")
	for stream.Next(ctx) {
		var event struct {
			OperationType string           `bson:"operationType"`
			FullDocument  *models.ConfigVar `bson:"fullDocument"`
		}
		if err := stream.Decode(&event); err != nil {
			slog.Warn("configstore: failed to decode change event", "error", err)
			continue
		}
		switch event.OperationType {
		case "insert", "update", "replace":
			if event.FullDocument != nil {
				s.mu.Lock()
				s.cache[event.FullDocument.Name] = *event.FullDocument
				s.mu.Unlock()
				slog.Debug("configstore: live-updated key", "key", event.FullDocument.Name)
			}
		case "delete":
			// Reload all — delete events don't carry the document name.
			if err := s.Load(ctx); err != nil {
				slog.Warn("configstore: reload after delete failed", "error", err)
			}
		}
	}
	return stream.Err()
}

func isChangeStreamUnsupported(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "replica set") ||
		strings.Contains(msg, "changeStream") ||
		strings.Contains(msg, "IllegalOperation") ||
		strings.Contains(msg, "not supported")
}
