package syslog

import (
	"context"
	"log"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"lastsaas/internal/db"
	"lastsaas/internal/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Patterns that suggest injection attempts in log messages
var injectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)<script[\s>]`),
	regexp.MustCompile(`(?i)javascript:`),
	regexp.MustCompile(`(?i)on(load|error|click|mouseover)\s*=`),
	regexp.MustCompile(`(?i)<iframe[\s>]`),
	regexp.MustCompile(`(?i)<object[\s>]`),
	regexp.MustCompile(`(?i)<embed[\s>]`),
	regexp.MustCompile(`(?i)<svg[\s/].*on\w+\s*=`),
}

const maxMessageLen = 2000

// severityOrder maps each level to a numeric rank for comparison.
// Lower number = higher priority. "none" disables all logging.
var severityOrder = map[models.LogSeverity]int{
	"none":             0,
	models.LogCritical: 1,
	models.LogHigh:     2,
	models.LogMedium:   3,
	models.LogLow:      4,
	models.LogDebug:    5,
}

// Logger writes structured log entries to the database.
type Logger struct {
	db        *db.MongoDB
	getConfig func(string) string
}

// New creates a Logger backed by the given database.
// getConfig is an optional function that returns config variable values (e.g. configstore.Store.Get).
// If nil, all messages are logged regardless of level.
func New(database *db.MongoDB, getConfig func(string) string) *Logger {
	return &Logger{db: database, getConfig: getConfig}
}

// log is the internal implementation shared by Log and LogWithUser.
func (l *Logger) log(ctx context.Context, severity models.LogSeverity, message string, userID *primitive.ObjectID) {
	if l.getConfig != nil {
		minLevel := models.LogSeverity(l.getConfig("log.min_level"))
		if minLevel == "none" {
			return
		}
		minRank, minOK := severityOrder[minLevel]
		sevRank, sevOK := severityOrder[severity]
		if minOK && sevOK && sevRank > minRank {
			return
		}
	}

	message = sanitize(message)

	entry := models.SystemLog{
		ID:        primitive.NewObjectID(),
		Severity:  severity,
		Message:   message,
		UserID:    userID,
		CreatedAt: time.Now(),
	}
	if _, err := l.db.SystemLogs().InsertOne(ctx, entry); err != nil {
		log.Printf("syslog: failed to write log: %v", err)
	}

	if detected := detectInjection(message); detected != "" {
		alert := models.SystemLog{
			ID:        primitive.NewObjectID(),
			Severity:  models.LogCritical,
			Message:   "Injection attempt detected in log entry: " + detected,
			UserID:    userID,
			CreatedAt: time.Now(),
		}
		l.db.SystemLogs().InsertOne(ctx, alert)
	}
}

// Log writes a log entry without user attribution (system context).
func (l *Logger) Log(ctx context.Context, severity models.LogSeverity, message string) {
	l.log(ctx, severity, message, nil)
}

// LogWithUser writes a log entry attributed to a specific user.
func (l *Logger) LogWithUser(ctx context.Context, severity models.LogSeverity, message string, userID primitive.ObjectID) {
	l.log(ctx, severity, message, &userID)
}

// Critical logs a critical-severity message.
func (l *Logger) Critical(ctx context.Context, message string) {
	l.log(ctx, models.LogCritical, message, nil)
}

// High logs a high-severity message.
func (l *Logger) High(ctx context.Context, message string) {
	l.log(ctx, models.LogHigh, message, nil)
}

// Medium logs a medium-severity message.
func (l *Logger) Medium(ctx context.Context, message string) {
	l.log(ctx, models.LogMedium, message, nil)
}

// Low logs a low-severity message.
func (l *Logger) Low(ctx context.Context, message string) {
	l.log(ctx, models.LogLow, message, nil)
}

// Debug logs a debug-severity message.
func (l *Logger) Debug(ctx context.Context, message string) {
	l.log(ctx, models.LogDebug, message, nil)
}

// CriticalWithUser logs a critical-severity message attributed to a user.
func (l *Logger) CriticalWithUser(ctx context.Context, message string, userID primitive.ObjectID) {
	l.log(ctx, models.LogCritical, message, &userID)
}

// HighWithUser logs a high-severity message attributed to a user.
func (l *Logger) HighWithUser(ctx context.Context, message string, userID primitive.ObjectID) {
	l.log(ctx, models.LogHigh, message, &userID)
}

// LogTenantActivity writes a tenant-scoped audit log entry.
func (l *Logger) LogTenantActivity(ctx context.Context, severity models.LogSeverity, message string, userID, tenantID primitive.ObjectID, action string, metadata map[string]interface{}) {
	if l.getConfig != nil {
		minLevel := models.LogSeverity(l.getConfig("log.min_level"))
		if minLevel == "none" {
			return
		}
		minRank, minOK := severityOrder[minLevel]
		sevRank, sevOK := severityOrder[severity]
		if minOK && sevOK && sevRank > minRank {
			return
		}
	}

	message = sanitize(message)

	entry := models.SystemLog{
		ID:        primitive.NewObjectID(),
		Severity:  severity,
		Message:   message,
		UserID:    &userID,
		TenantID:  &tenantID,
		Action:    action,
		Metadata:  metadata,
		CreatedAt: time.Now(),
	}
	if _, err := l.db.SystemLogs().InsertOne(ctx, entry); err != nil {
		log.Printf("syslog: failed to write tenant activity log: %v", err)
	}
}

// sanitize ensures the message is valid UTF-8, strips control characters,
// and enforces the maximum length.
func sanitize(s string) string {
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "")
	}
	// Strip control characters except newline and tab
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, s)
	if len(s) > maxMessageLen {
		s = s[:maxMessageLen]
	}
	return s
}

// detectInjection checks for common injection patterns and returns the
// matched pattern name, or empty string if clean.
func detectInjection(s string) string {
	for _, pat := range injectionPatterns {
		if pat.MatchString(s) {
			return pat.String()
		}
	}
	return ""
}
