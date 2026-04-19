package sse

import (
	"sync"
)

// Event is a named payload pushed to subscribed clients.
type Event struct {
	// TenantID scopes delivery. Empty string broadcasts to all subscribers.
	TenantID string
	Name     string
	Data     string // JSON-encoded payload
}

// Hub routes SSE events to subscribed client channels.
// Clients subscribe per-tenant; a publish to a tenant ID delivers only to
// that tenant's subscribers. Each node maintains its own Hub — cross-node
// delivery is handled by the Watcher (MongoDB change stream).
type Hub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan Event]struct{} // tenantID → set of channels
}

// New returns an initialised Hub.
func New() *Hub {
	return &Hub{
		subscribers: make(map[string]map[chan Event]struct{}),
	}
}

// Subscribe registers ch to receive events for tenantID.
// The caller is responsible for calling Unsubscribe when the connection closes.
func (h *Hub) Subscribe(tenantID string, ch chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subscribers[tenantID] == nil {
		h.subscribers[tenantID] = make(map[chan Event]struct{})
	}
	h.subscribers[tenantID][ch] = struct{}{}
}

// Unsubscribe removes ch from tenantID's subscriber set and closes the channel.
func (h *Hub) Unsubscribe(tenantID string, ch chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if subs, ok := h.subscribers[tenantID]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(h.subscribers, tenantID)
		}
	}
	close(ch)
}

// Publish sends evt to all subscribers of evt.TenantID.
// Non-blocking: slow or full channels are skipped (the client will reconnect).
func (h *Hub) Publish(evt Event) {
	h.mu.RLock()
	subs := h.subscribers[evt.TenantID]
	channels := make([]chan Event, 0, len(subs))
	for ch := range subs {
		channels = append(channels, ch)
	}
	h.mu.RUnlock()

	for _, ch := range channels {
		select {
		case ch <- evt:
		default:
		}
	}
}

// SubscriberCount returns the number of active SSE connections across all tenants.
func (h *Hub) SubscriberCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	n := 0
	for _, subs := range h.subscribers {
		n += len(subs)
	}
	return n
}
