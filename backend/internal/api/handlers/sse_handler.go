package handlers

import (
	"fmt"
	"net/http"
	"time"

	"saasquickstart/internal/middleware"
	"saasquickstart/internal/sse"
)

const (
	sseHeartbeatInterval = 30 * time.Second
	sseChannelBuffer     = 32
)

// SSEHandler streams real-time events to authenticated tenant clients.
type SSEHandler struct {
	hub *sse.Hub
}

// NewSSEHandler creates an SSEHandler backed by hub.
func NewSSEHandler(hub *sse.Hub) *SSEHandler {
	return &SSEHandler{hub: hub}
}

// Stream opens a Server-Sent Events connection for the authenticated tenant.
// GET /api/tenant/events/stream
//
// The connection stays open until the client disconnects or the server stops.
// A heartbeat comment is sent every 30 seconds to keep proxies from closing idle connections.
// Clients should reconnect on disconnect using the EventSource API's built-in retry.
func (h *SSEHandler) Stream(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.GetTenantFromContext(r.Context())

	// Require the response writer to support flushing.
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Remove the server-level write timeout so the connection stays open indefinitely.
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := make(chan sse.Event, sseChannelBuffer)
	tenantID := tenant.ID.Hex()
	h.hub.Subscribe(tenantID, ch)
	defer h.hub.Unsubscribe(tenantID, ch)

	heartbeat := time.NewTicker(sseHeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: %s\n", evt.Name)
			fmt.Fprintf(w, "data: %s\n\n", evt.Data)
			flusher.Flush()

		case <-heartbeat.C:
			// SSE comment — keeps the connection alive through proxies.
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()

		case <-r.Context().Done():
			return
		}
	}
}
