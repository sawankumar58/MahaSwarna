package ws

import (
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// pingInterval is how often the server pings each connection.
	pingInterval = 30 * time.Second

	// pongDeadline is how long after a ping the server waits for a pong before closing.
	// Max stale entry window = pingInterval + pongDeadline = 40s.
	pongDeadline = 10 * time.Second

	// readDeadline is set on every successful pong. Must be > pingInterval + pongDeadline
	// to avoid false positives. Set to pingInterval + pongDeadline for a tight bound.
	readDeadline = pingInterval + pongDeadline // 40s
)

// RunHeartbeat starts a ping loop for conn. It returns when the connection
// is dead (ping failure or context cancellation).
//
// SMELL-3 FIX: RunHeartbeat no longer accepts an onClose callback and no longer
// removes the connection from the registry or calls conn.Close().
// Connection lifecycle is owned exclusively by ws_server.go's readLoop defer:
//
//	defer func() {
//	    registry.Remove(userID, conn)
//	    conn.Close()
//	}()
//
// How the hand-off works: when RunHeartbeat fails to send a ping (conn is dead),
// it returns without closing. The read deadline it previously set on the conn
// (via SetReadDeadline) means the next ReadMessage in readLoop returns a deadline
// error within ≤40s, triggering the single authoritative cleanup path.
// This eliminates the race where both goroutines called Remove+Close on the same conn.
func RunHeartbeat(conn *websocket.Conn) {
	// Set initial read deadline and pong handler BEFORE the first ping tick.
	conn.SetReadDeadline(time.Now().Add(readDeadline))
	conn.SetPongHandler(func(string) error {
		// Reset read deadline on every pong received.
		conn.SetReadDeadline(time.Now().Add(readDeadline))
		return nil
	})

	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for range ticker.C {
		if err := conn.WriteControl(
			websocket.PingMessage,
			nil,
			time.Now().Add(pongDeadline),
		); err != nil {
			// Failed to send ping — connection is dead.
			// readLoop will detect this via the read deadline and run cleanup.
			slog.Debug("ws heartbeat ping failed", "err", err)
			return
		}
	}
}
