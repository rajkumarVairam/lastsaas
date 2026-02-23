package middleware

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

type RateLimiter struct {
	mu       sync.RWMutex
	requests map[string]*rateLimitEntry
	cleanup  *time.Ticker
	done     chan bool
}

type rateLimitEntry struct {
	count     int
	windowEnd time.Time
}

type RateLimitConfig struct {
	MaxRequests int
	Window      time.Duration
}

var (
	AccountCreationLimit    = RateLimitConfig{MaxRequests: 5, Window: time.Hour}
	LoginAttemptLimit       = RateLimitConfig{MaxRequests: 10, Window: 15 * time.Minute}
	PasswordResetLimit      = RateLimitConfig{MaxRequests: 5, Window: time.Hour}
	ResendVerificationLimit = RateLimitConfig{MaxRequests: 3, Window: 60 * time.Second}
	OAuthInitLimit          = RateLimitConfig{MaxRequests: 10, Window: time.Minute}
	EmailVerificationLimit  = RateLimitConfig{MaxRequests: 10, Window: time.Hour}
	TokenRefreshLimit       = RateLimitConfig{MaxRequests: 30, Window: time.Minute}
	InvitationLimit         = RateLimitConfig{MaxRequests: 20, Window: time.Hour}
	MFAChallengeLimit       = RateLimitConfig{MaxRequests: 5, Window: 5 * time.Minute}
	MagicLinkLimit          = RateLimitConfig{MaxRequests: 5, Window: 15 * time.Minute}
)

func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*rateLimitEntry),
		cleanup:  time.NewTicker(5 * time.Minute),
		done:     make(chan bool),
	}
	go func() {
		for {
			select {
			case <-rl.cleanup.C:
				rl.cleanupExpired()
			case <-rl.done:
				return
			}
		}
	}()
	return rl
}

func (rl *RateLimiter) Stop() {
	rl.cleanup.Stop()
	close(rl.done)
}

func (rl *RateLimiter) cleanupExpired() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	for key, entry := range rl.requests {
		if now.After(entry.windowEnd) {
			delete(rl.requests, key)
		}
	}
}

func (rl *RateLimiter) Allow(key string, config RateLimitConfig) (bool, int, time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.requests[key]

	if !exists || now.After(entry.windowEnd) {
		rl.requests[key] = &rateLimitEntry{
			count:     1,
			windowEnd: now.Add(config.Window),
		}
		return true, config.MaxRequests - 1, now.Add(config.Window)
	}

	if entry.count >= config.MaxRequests {
		return false, 0, entry.windowEnd
	}

	entry.count++
	return true, config.MaxRequests - entry.count, entry.windowEnd
}

func GetClientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		if ip, _, err := net.SplitHostPort(xff); err == nil {
			return ip
		}
		if net.ParseIP(xff) != nil {
			return xff
		}
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				firstIP := xff[:i]
				if net.ParseIP(firstIP) != nil {
					return firstIP
				}
				break
			}
		}
	}

	xri := r.Header.Get("X-Real-IP")
	if xri != "" && net.ParseIP(xri) != nil {
		return xri
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func (rl *RateLimiter) RateLimitHandler(config RateLimitConfig, keyFunc func(*http.Request) string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := keyFunc(r)
		allowed, remaining, resetTime := rl.Allow(key, config)

		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", config.MaxRequests))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		w.Header().Set("X-RateLimit-Reset", resetTime.Format(time.RFC3339))

		if !allowed {
			retryAfter := int(time.Until(resetTime).Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":      "Rate limit exceeded",
				"retryAfter": retryAfter,
			})
			return
		}

		handler(w, r)
	}
}
