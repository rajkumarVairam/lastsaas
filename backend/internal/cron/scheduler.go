package cron

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"saasquickstart/internal/db"
	"saasquickstart/internal/jobs"
	"saasquickstart/internal/models"

	robfigcron "github.com/robfig/cron/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	schedulerTickInterval = 30 * time.Second
	schedulerLockDuration = 2 * time.Minute
	staleReclaimInterval  = 5 * time.Minute
)

// Scheduler polls for due CronSchedules, claims them atomically, enqueues the
// corresponding Job, then advances NextRunAt. Multiple nodes run concurrently —
// the atomic findOneAndUpdate claim ensures each schedule fires exactly once per tick.
type Scheduler struct {
	db        *db.MongoDB
	queue     *jobs.Queue
	machineID string
	stopCh    chan struct{}
}

// New returns a Scheduler. Call Start to begin the tick loop.
func New(database *db.MongoDB, queue *jobs.Queue, machineID string) *Scheduler {
	return &Scheduler{
		db:        database,
		queue:     queue,
		machineID: machineID,
		stopCh:    make(chan struct{}),
	}
}

// Start launches the scheduler goroutine. It returns immediately.
func (s *Scheduler) Start(ctx context.Context) {
	go func() {
		s.reclaimStale(ctx)
		s.tick(ctx)

		ticker := time.NewTicker(schedulerTickInterval)
		staleTicker := time.NewTicker(staleReclaimInterval)
		defer ticker.Stop()
		defer staleTicker.Stop()

		for {
			select {
			case <-ticker.C:
				s.tick(ctx)
			case <-staleTicker.C:
				s.reclaimStale(ctx)
			case <-s.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	slog.Info("cron scheduler started", "node", s.machineID, "interval", schedulerTickInterval)
}

// Stop signals the scheduler goroutine to exit.
func (s *Scheduler) Stop() {
	close(s.stopCh)
}

// NextRunTime returns the next time a cron expression should fire in a given timezone.
func NextRunTime(expression, timezone string) (time.Time, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timezone %q: %w", timezone, err)
	}
	parser := robfigcron.NewParser(robfigcron.Minute | robfigcron.Hour | robfigcron.Dom | robfigcron.Month | robfigcron.Dow)
	sched, err := parser.Parse(expression)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid cron expression %q: %w", expression, err)
	}
	return sched.Next(time.Now().In(loc)), nil
}

func (s *Scheduler) tick(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("cron: tick recovered from panic", "panic", r)
		}
	}()

	now := time.Now()
	lockExpiry := now.Add(schedulerLockDuration)

	for {
		var schedule models.CronSchedule
		filter := bson.M{
			"isActive":  true,
			"nextRunAt": bson.M{"$lte": now},
			"$or": bson.A{
				bson.M{"lockedUntil": bson.M{"$exists": false}},
				bson.M{"lockedUntil": nil},
				bson.M{"lockedUntil": bson.M{"$lt": now}},
			},
		}
		update := bson.M{
			"$set": bson.M{
				"lockedBy":    s.machineID,
				"lockedUntil": lockExpiry,
			},
		}
		err := s.db.CronSchedules().FindOneAndUpdate(
			ctx, filter, update,
			options.FindOneAndUpdate().SetReturnDocument(options.After),
		).Decode(&schedule)
		if err != nil {
			break // no more due schedules
		}

		s.fire(ctx, &schedule)
	}
}

func (s *Scheduler) fire(ctx context.Context, schedule *models.CronSchedule) {
	now := time.Now()

	job := &models.Job{
		Type:        schedule.JobType,
		TenantID:    schedule.TenantID,
		Payload:     schedule.Payload,
		MaxAttempts: schedule.MaxAttempts,
		RunAt:       now,
	}

	enqueueErr := s.queue.Enqueue(ctx, job)
	if enqueueErr != nil {
		slog.Error("cron: failed to enqueue job", "schedule", schedule.ID.Hex(), "jobType", schedule.JobType, "error", enqueueErr)
	}

	nextRun, err := NextRunTime(schedule.Expression, schedule.Timezone)
	if err != nil {
		slog.Error("cron: failed to compute next run time", "schedule", schedule.ID.Hex(), "error", err)
		// Deactivate the schedule to prevent a tight loop on bad expressions.
		_, _ = s.db.CronSchedules().UpdateOne(ctx,
			bson.M{"_id": schedule.ID},
			bson.M{"$set": bson.M{"isActive": false, "lockedBy": "", "lockedUntil": nil, "updatedAt": now}},
		)
		return
	}

	update := bson.M{
		"$set": bson.M{
			"nextRunAt":   nextRun,
			"lastRunAt":   now,
			"lockedBy":    "",
			"lockedUntil": nil,
			"updatedAt":   now,
		},
	}
	if _, err := s.db.CronSchedules().UpdateOne(ctx, bson.M{"_id": schedule.ID}, update); err != nil {
		slog.Warn("cron: failed to advance schedule", "schedule", schedule.ID.Hex(), "error", err)
	}

	if enqueueErr == nil {
		slog.Info("cron: fired", "schedule", schedule.ID.Hex(), "jobType", schedule.JobType, "nextRunAt", nextRun)
	}
}

// reclaimStale unlocks schedules whose lock has expired (node crashed mid-execution).
func (s *Scheduler) reclaimStale(ctx context.Context) {
	now := time.Now()
	result, err := s.db.CronSchedules().UpdateMany(ctx,
		bson.M{
			"lockedUntil": bson.M{"$lt": now},
			"lockedBy":    bson.M{"$ne": ""},
		},
		bson.M{"$set": bson.M{"lockedBy": "", "lockedUntil": primitive.Null{}, "updatedAt": now}},
	)
	if err != nil {
		slog.Warn("cron: stale lock reclaim failed", "error", err)
		return
	}
	if result.ModifiedCount > 0 {
		slog.Info("cron: reclaimed stale locks", "count", result.ModifiedCount)
	}
}
