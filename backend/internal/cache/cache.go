package cache

import (
	"sync"
	"time"
)

type entry[V any] struct {
	value     V
	expiresAt time.Time
}

// TTL is a thread-safe in-memory cache with per-entry expiry.
// Expired entries are evicted lazily on Get and periodically by a background sweep.
// Safe for concurrent use. Zero value is not usable — use New.
type TTL[K comparable, V any] struct {
	mu      sync.RWMutex
	entries map[K]entry[V]
	ttl     time.Duration
	stopCh  chan struct{}
}

// New creates a TTL cache and starts a background sweep goroutine that evicts
// expired entries every minute. Call Stop when the cache is no longer needed.
func New[K comparable, V any](ttl time.Duration) *TTL[K, V] {
	c := &TTL[K, V]{
		entries: make(map[K]entry[V]),
		ttl:     ttl,
		stopCh:  make(chan struct{}),
	}
	go c.sweepLoop()
	return c
}

// Get returns the cached value for key. Returns zero value and false if the key
// is absent or its TTL has expired.
func (c *TTL[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		var zero V
		return zero, false
	}
	return e.value, true
}

// Set stores value under key, replacing any existing entry. The entry expires
// after the TTL configured on the cache.
func (c *TTL[K, V]) Set(key K, value V) {
	c.mu.Lock()
	c.entries[key] = entry[V]{value: value, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

// Delete removes key immediately, regardless of TTL.
func (c *TTL[K, V]) Delete(key K) {
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}

// Stop terminates the background sweep goroutine. The cache remains readable
// after Stop but expired entries will no longer be evicted automatically.
func (c *TTL[K, V]) Stop() {
	close(c.stopCh)
}

func (c *TTL[K, V]) sweepLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.sweep()
		case <-c.stopCh:
			return
		}
	}
}

func (c *TTL[K, V]) sweep() {
	now := time.Now()
	c.mu.Lock()
	for k, e := range c.entries {
		if now.After(e.expiresAt) {
			delete(c.entries, k)
		}
	}
	c.mu.Unlock()
}
