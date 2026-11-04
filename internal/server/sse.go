package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/alexandrmotologa/keygate/internal/storage"
)

// EventBroker coordinates Server-Sent Events subscriptions and broadcasts.
type EventBroker struct {
	mu      sync.RWMutex
	clients map[chan *storage.UsageEvent]bool
}

// NewEventBroker initializes a real-time event broker.
func NewEventBroker() *EventBroker {
	return &EventBroker{
		clients: make(map[chan *storage.UsageEvent]bool),
	}
}

// Subscribe registers a new listener channel.
func (b *EventBroker) Subscribe() chan *storage.UsageEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan *storage.UsageEvent, 100)
	b.clients[ch] = true
	return ch
}

// Unsubscribe removes a listener channel.
func (b *EventBroker) Unsubscribe(ch chan *storage.UsageEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.clients[ch]; exists {
		delete(b.clients, ch)
		close(ch)
	}
}

// RecordEvent implements proxy.TelemetrySink by broadcasting to all SSE listeners.
func (b *EventBroker) RecordEvent(event *storage.UsageEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for ch := range b.clients {
		select {
		case ch <- event:
		default:
			// Non-blocking drop if client buffer is full
		}
	}
}

// ServeHTTP handles incoming SSE client connections.
func (b *EventBroker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	clientCh := b.Subscribe()
	defer b.Unsubscribe(clientCh)

	// Send initial connection comment
	_, _ = fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			_, err := fmt.Fprintf(w, ": keep-alive\n\n")
			if err != nil {
				return
			}
			flusher.Flush()
		case event, ok := <-clientCh:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			_, err = fmt.Fprintf(w, "data: %s\n\n", data)
			if err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
