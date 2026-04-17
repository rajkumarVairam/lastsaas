package version

import (
	"context"
	"log/slog"
	"os"
	"time"

	"lastsaas/internal/db"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Migration describes a one-time data transformation tied to a version boundary.
// Up must be idempotent: if it has already run, running it again must be safe.
// Fail fast: return an error to abort startup — the runner calls os.Exit(1).
type Migration struct {
	Version     string // semver string that introduced this migration, e.g. "1.2.0"
	Description string
	Up          func(ctx context.Context, database *db.MongoDB) error
}

// migrations is the ordered list of all registered migrations.
// Append only — never remove or reorder existing entries.
var migrations []Migration

// migrationRecord is stored in the "migrations" collection to track applied runs.
type migrationRecord struct {
	ID          primitive.ObjectID `bson:"_id"`
	Version     string             `bson:"version"`
	Description string             `bson:"description"`
	AppliedAt   time.Time          `bson:"appliedAt"`
}

// runRegisteredMigrations executes any migration whose version falls in (from, to].
// It records each applied migration in the "migrations" collection and exits on error.
func runRegisteredMigrations(database *db.MongoDB, from, to string) {
	if len(migrations) == 0 {
		slog.Info("Migrations: none registered")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Ensure unique index on version so concurrent nodes don't double-apply.
	coll := database.Migrations()
	coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "version", Value: 1}},
		Options: options.Index().SetUnique(true),
	})

	applied := 0
	for _, m := range migrations {
		if !versionAfter(m.Version, from) || versionAfter(m.Version, to) {
			// Migration version is at or before `from`, or newer than `to` — skip.
			continue
		}

		// Idempotency: skip if already recorded.
		count, _ := coll.CountDocuments(ctx, bson.M{"version": m.Version})
		if count > 0 {
			slog.Info("Migration already applied, skipping", "version", m.Version)
			continue
		}

		slog.Info("Running migration", "version", m.Version, "description", m.Description)
		if err := m.Up(ctx, database); err != nil {
			slog.Error("Migration failed — aborting startup", "version", m.Version, "error", err)
			os.Exit(1)
		}

		rec := migrationRecord{
			ID:          primitive.NewObjectID(),
			Version:     m.Version,
			Description: m.Description,
			AppliedAt:   time.Now(),
		}
		if _, err := coll.InsertOne(ctx, rec); err != nil {
			// Unique index violation means another node beat us — safe to ignore.
			slog.Warn("Migration record insert skipped (concurrent node?)", "version", m.Version, "error", err)
		} else {
			slog.Info("Migration applied", "version", m.Version)
			applied++
		}
	}

	if applied == 0 {
		slog.Info("Migrations: all up to date")
	} else {
		slog.Info("Migrations complete", "applied", applied)
	}
}

// versionAfter reports whether version a is strictly after version b.
// Compares major.minor.patch integers. Non-parseable versions return false.
func versionAfter(a, b string) bool {
	if b == "" || b == "unknown" {
		return true // anything is after an unversioned state
	}
	ma, mna, pa, okA := parseSemver(a)
	mb, mnb, pb, okB := parseSemver(b)
	if !okA || !okB {
		return false
	}
	if ma != mb {
		return ma > mb
	}
	if mna != mnb {
		return mna > mnb
	}
	return pa > pb
}

func parseSemver(v string) (major, minor, patch int, ok bool) {
	// Strip leading 'v'
	if len(v) > 0 && v[0] == 'v' {
		v = v[1:]
	}
	n, err := parseThreeParts(v)
	if err != nil || len(n) != 3 {
		return 0, 0, 0, false
	}
	return n[0], n[1], n[2], true
}

func parseThreeParts(v string) ([]int, error) {
	parts := splitDots(v)
	if len(parts) < 3 {
		// Pad missing parts with zeros
		for len(parts) < 3 {
			parts = append(parts, "0")
		}
	}
	nums := make([]int, 3)
	for i := 0; i < 3; i++ {
		n, err := parseUint(parts[i])
		if err != nil {
			return nil, err
		}
		nums[i] = n
	}
	return nums, nil
}

func splitDots(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func parseUint(s string) (int, error) {
	// Strip pre-release suffix (e.g. "1-alpha" → 1)
	for i, c := range s {
		if c == '-' {
			s = s[:i]
			break
		}
	}
	n := 0
	if len(s) == 0 {
		return 0, nil
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, &parseError{s}
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

type parseError struct{ s string }

func (e *parseError) Error() string { return "not a number: " + e.s }
