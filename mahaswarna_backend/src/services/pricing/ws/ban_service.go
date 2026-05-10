package ws

import (
	"log/slog"

	"github.com/gorilla/websocket"
)

// BanService disconnects all WebSocket connections for a banned user.
// Called by events/listeners.go when a user_banned pg NOTIFY arrives.
type BanService struct {
	registry *ConnectionRegistry
}

func NewBanService(registry *ConnectionRegistry) *BanService {
	return &BanService{registry: registry}
}

// Disconnect sends a policy-violation close frame and removes the user from the registry.
func (b *BanService) Disconnect(userID string) {
	slog.Info("ws ban: disconnecting user", "user_id", userID)
	closeMsg := websocket.FormatCloseMessage(
		websocket.ClosePolicyViolation, "account suspended",
	)

	existing, ok := b.registry.m.Load(userID)
	if !ok {
		return
	}
	for _, conn := range existing.([]*websocket.Conn) {
		_ = conn.WriteMessage(websocket.CloseMessage, closeMsg)
		conn.Close()
	}
	b.registry.m.Delete(userID)
}
