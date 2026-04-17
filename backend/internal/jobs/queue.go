package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"lastsaas/internal/db"
	"lastsaas/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	defaultConcurrency  = 5
	defaultPollInterval = 2 * time.Second
	lockDuration        = 5 * time.Minute
	staleCheckInterval  = 30 * time.Second
	baseRetryBackoff    = 30 * time.Second
	maxRetryBackoff     = 1 * time.Hour
	jobExecTimeout      = 4 * time.Minute // must be < lockDuration to avoid reclaim races
)

// Queue is a durable, distributed job queue backed by MongoDB.
//
// Multiple nodes can run Queue concurrently. Each job is claimed with a
// findOneAndUpdate — only one node executes any given job. If a node crashes
// mid-execution the lock expires after lockDuration and another node reclaims it.
type Queue struct {
	db           *db.MongoDB
	machineID    string
	handlers     map[string]Handler
	mu           sync.RWMutex
	concurrency  int
	pollInterval time.Duration

	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// Option configures a Queue.
type Option func(*Queue)

// WithConcurrency sets the number of parallel worker goroutines per node (default 5).
func WithConcurrency(n int) Option {
	return func(q *Queue) {
		if n > 0 {
			q.concurrency = n
		}
	}
}

// WithPollInterval sets how often workers check for new jobs (default 2s).
func WithPollInterval(d time.Duration) Option {
	return func(q *Queue) {
		if d > 0 {
			q.pollInterval = d
		}
	}
}

// New creates a Queue. Call Register for each job type, then Start to begin processing.
func New(database *db.MongoDB, machineID string, opts ...Option) *Queue {
	q := &Queue{
		db:           database,
		machineID:    machineID,
		handlers:     make(map[string]Handler),
		concurrency:  defaultConcurrency,
		pollInterval: defaultPollInterval,
	}
	for _, o := range opts {
		o(q)
	}
	return q
}

// Register adds a handler for a job type. Must be called before Start.
func (q *Queue) Register(h Handler) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.handlers[h.Type()] = h
}

// Enqueue inserts a job to run at or after job.RunAt.
// Defaults: RunAt=now, MaxAttempts=5, Status=pending.
func (q *Queue) Enqueue(ctx context.Context, job *models.Job) error {
	now := time.Now()
	if job.ID.IsZero() {
		job.ID = primitive.NewObjectID()
	}
	if job.RunAt.IsZero() {
		job.RunAt = now
	}
	if job.MaxAttempts == 0 {
		job.MaxAttempts = 5
	}
	job.Status = models.JobStatusPending
	job.Attempts = 0
	job.CreatedAt = now
	job.UpdatedAt = now

	_, err := q.db.Jobs().InsertOne(ctx, job)
	if err != nil {
		return fmt.Errorf("jobs: enqueue: %w", err)
	}
	return nil
}

// Start launches the worker pool and stale-lock reclaimer.
// The queue runs until ctx is cancelled or Stop is called.
func (q *Queue) Start(ctx context.Context) {
	qCtx, cancel := context.WithCancel(ctx)
	q.cancel = cancel

	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		q.reclaimLoop(qCtx)
	}()

	for i := 0; i < q.concurrency; i++ {
		q.wg.Add(1)
		go func(workerID int) {
			defer q.wg.Done()
			q.workerLoop(qCtx, workerID)
		}(i)
	}

	slog.Info("Job queue started", "workers", q.concurrency, "node", q.machineID)
}

// Stop signals all workers to finish and waits for in-flight jobs to complete.
func (q *Queue) Stop() {
	if q.cancel != nil {
		q.cancel()
	}
	q.wg.Wait()
	slog.Info("Job queue stopped")
}

// workerLoop polls for pending jobs and executes them until ctx is cancelled.
// When a job is found it immediately tries for another — drains the queue before
// sleeping, so bursts are handled efficiently.
func (q *Queue) workerLoop(ctx context.Context, id int) {
	ticker := time.NewTicker(q.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for {
				if ctx.Err() != nil {
					return
				}
				job, err := q.claimNext(ctx)
				if err != nil {
					slog.Warn("Job queue claim error", "worker", id, "error", err)
					break
				}
				if job == nil {
					break // no work available, go back to sleeping
				}
				q.execute(ctx, job)
			}
		}
	}
}

// claimNext atomically finds and locks the next runnable job for this node.
// Returns nil, nil when no job is available.
func (q *Queue) claimNext(ctx context.Context) (*models.Job, error) {
	now := time.Now()
	lockedUntil := now.Add(lockDuration)

	filter := bson.M{
		"status": models.JobStatusPending,
		"runAt":  bson.M{"$lte": now},
	}
	update := bson.M{
		"$set": bson.M{
			"status":      models.JobStatusRunning,
			"lockedBy":    q.machineID,
			"lockedUntil": lockedUntil,
			"updatedAt":   now,
		},
	}
	opts := options.FindOneAndUpdate().
		SetSort(bson.D{{Key: "runAt", Value: 1}}).
		SetReturnDocument(options.After)

	var job models.Job
	err := q.db.Jobs().FindOneAndUpdate(ctx, filter, update, opts).Decode(&job)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claimNext: %w", err)
	}
	return &job, nil
}

func (q *Queue) execute(ctx context.Context, job *models.Job) {
	q.mu.RLock()
	h, ok := q.handlers[job.Type]
	q.mu.RUnlock()

	if !ok {
		slog.Warn("No handler registered for job type — marking dead", "type", job.Type, "jobId", job.ID.Hex())
		q.markDead(ctx, job, "no handler registered for type: "+job.Type)
		return
	}

	slog.Info("Executing job", "type", job.Type, "jobId", job.ID.Hex(), "attempt", job.Attempts+1, "maxAttempts", job.MaxAttempts)

	execCtx, cancel := context.WithTimeout(ctx, jobExecTimeout)
	defer cancel()

	err := h.Execute(execCtx, job)
	job.Attempts++

	if err == nil {
		q.markCompleted(ctx, job)
		return
	}

	slog.Warn("Job execution failed", "type", job.Type, "jobId", job.ID.Hex(), "attempt", job.Attempts, "error", err)

	if job.Attempts >= job.MaxAttempts {
		q.markDead(ctx, job, err.Error())
		return
	}

	// Exponential backoff: 30s, 60s, 120s, 240s... capped at 1h
	backoff := baseRetryBackoff * (1 << uint(job.Attempts-1))
	if backoff > maxRetryBackoff {
		backoff = maxRetryBackoff
	}
	q.markRetry(ctx, job, err.Error(), time.Now().Add(backoff))
}

func (q *Queue) markCompleted(ctx context.Context, job *models.Job) {
	now := time.Now()
	_, err := q.db.Jobs().UpdateOne(ctx,
		bson.M{"_id": job.ID},
		bson.M{"$set": bson.M{
			"status":      models.JobStatusCompleted,
			"attempts":    job.Attempts,
			"result":      job.Result,
			"completedAt": now,
			"updatedAt":   now,
			"lockedBy":    "",
			"lockedUntil": nil,
		}},
	)
	if err != nil {
		slog.Error("Failed to mark job completed", "jobId", job.ID.Hex(), "error", err)
		return
	}
	slog.Info("Job completed", "type", job.Type, "jobId", job.ID.Hex(), "attempts", job.Attempts)
}

func (q *Queue) markRetry(ctx context.Context, job *models.Job, errMsg string, runAt time.Time) {
	now := time.Now()
	_, err := q.db.Jobs().UpdateOne(ctx,
		bson.M{"_id": job.ID},
		bson.M{"$set": bson.M{
			"status":      models.JobStatusPending,
			"attempts":    job.Attempts,
			"lastError":   errMsg,
			"runAt":       runAt,
			"updatedAt":   now,
			"lockedBy":    "",
			"lockedUntil": nil,
		}},
	)
	if err != nil {
		slog.Error("Failed to schedule job retry", "jobId", job.ID.Hex(), "error", err)
	}
}

func (q *Queue) markDead(ctx context.Context, job *models.Job, errMsg string) {
	now := time.Now()
	_, err := q.db.Jobs().UpdateOne(ctx,
		bson.M{"_id": job.ID},
		bson.M{"$set": bson.M{
			"status":      models.JobStatusDead,
			"attempts":    job.Attempts,
			"lastError":   errMsg,
			"updatedAt":   now,
			"lockedBy":    "",
			"lockedUntil": nil,
		}},
	)
	if err != nil {
		slog.Error("Failed to mark job dead", "jobId", job.ID.Hex(), "error", err)
		return
	}
	slog.Warn("Job dead — exhausted retries", "type", job.Type, "jobId", job.ID.Hex(), "lastError", errMsg)
}

// reclaimLoop periodically resets jobs stuck in "running" with an expired lock.
// This is the crash-recovery path: if a node dies mid-execution, its jobs become
// available again after lockDuration (5 minutes).
func (q *Queue) reclaimLoop(ctx context.Context) {
	ticker := time.NewTicker(staleCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			q.reclaimStaleLocks(ctx)
		}
	}
}

func (q *Queue) reclaimStaleLocks(ctx context.Context) {
	now := time.Now()
	result, err := q.db.Jobs().UpdateMany(ctx,
		bson.M{
			"status":      models.JobStatusRunning,
			"lockedUntil": bson.M{"$lt": now},
		},
		bson.M{"$set": bson.M{
			"status":      models.JobStatusPending,
			"lockedBy":    "",
			"lockedUntil": nil,
			"updatedAt":   now,
		}},
	)
	if err != nil {
		slog.Warn("Failed to reclaim stale job locks", "error", err)
		return
	}
	if result.ModifiedCount > 0 {
		slog.Info("Reclaimed stale job locks", "count", result.ModifiedCount)
	}
}
