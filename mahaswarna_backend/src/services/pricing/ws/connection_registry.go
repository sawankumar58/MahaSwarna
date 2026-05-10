package ws

import (
	"log/slog"
	"sync"

	"github.com/gorilla/websocket"
)

// ConnectionRegistry maps userID → set of live WebSocket connections.
// A user may have multiple concurrent connections (e.g. phone + tablet).
//
// ARCHITECTURE NOTE: sync.Map is adequate up to ~5,000 concurrent connections.
// At >5,000: replace with a 16-shard map using CRC32(userID) % 16 as the shard selector.
// Upgrade trigger: WS concurrent connections > 5,000 (Grafana alert: ws_active_connections).
type ConnectionRegistry struct {
	m sync.Map // key: string(userID) → []*websocket.Conn
	mu sync.Mutex // guards per-user slice mutations
}

func NewConnectionRegistry() *ConnectionRegistry {
	return &ConnectionRegistry{}
}

// Register associates conn with userID. Safe for concurrent use.
func (r *ConnectionRegistry) Register(userID string, conn *websocket.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, _ := r.m.Load(userID)
	var conns []*websocket.Conn
	if existing != nil {
		conns = existing.([]*websocket.Conn)
	}
	r.m.Store(userID, append(conns, conn))
}

// Remove removes conn from the registry for userID.
func (r *ConnectionRegistry) Remove(userID string, conn *websocket.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.m.Load(userID)
	if !ok {
		return
	}
	conns := existing.([]*websocket.Conn)
	updated := make([]*websocket.Conn, 0, len(conns))
	for _, c := range conns {
		if c != conn {
			updated = append(updated, c)
		}
	}
	if len(updated) == 0 {
		r.m.Delete(userID)
	} else {
		r.m.Store(userID, updated)
	}
}

// Send writes msg to all connections registered for userID.
// Dead connections (write error) are removed.
func (r *ConnectionRegistry) Send(userID string, msg []byte) {
	existing, ok := r.m.Load(userID)
	if !ok {
		return
	}
	conns := existing.([]*websocket.Conn)
	for _, conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			slog.Debug("ws send failed, removing conn", "user", userID, "err", err)
			r.Remove(userID, conn)
		}
	}
}

// CloseAll sends a close frame to every connection and removes them from the registry.
// Called by main.go during graceful shutdown (Phase 2).
// REQUIRED: this method must be exported and defined here — not in main.go.
func (r *ConnectionRegistry) CloseAll(code int, reason string) {
	closeMsg := websocket.FormatCloseMessage(code, reason)
	r.m.Range(func(key, value any) bool {
		conns := value.([]*websocket.Conn)
		for _, conn := range conns {
			if err := conn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
				slog.Debug("ws CloseAll write error", "err", err)
			}
			conn.Close()
		}
		r.m.Delete(key)
		return true
	})
}

// Count returns the total number of active connections across all users.
// Used for Prometheus metric ws_active_connections.
func (r *ConnectionRegistry) Count() int {
	total := 0
	r.m.Range(func(_, value any) bool {
		total += len(value.([]*websocket.Conn))
		return true
	})
	return total
}
