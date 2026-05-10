package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

const (
	// wsHandshakeRateLimit: max new WS upgrade attempts per IP per minute.
	// ADR-002: gateway is bypassed on :4002; this service self-enforces.
	wsHandshakeRateLimit = 20

	// maxUpgradeBodySize limits the initial HTTP upgrade request body.
	maxUpgradeBodySize = 64 * 1024 // 64 KB

	// maxMessageSize limits inbound WS message size.
	maxMessageSize = 4 * 1024 // 4 KB
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	// CheckOrigin: reject connections without a valid origin in production.
	// For now, accept all origins (Android clients don't send Origin headers).
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Server handles WebSocket upgrades and connection lifecycle.
type Server struct {
	registry *ConnectionRegistry
	fanout   *BufferedFanout
	rdb      *redis.Client
	jwtPub   interface{} // *rsa.PublicKey — loaded from JWT_PUBLIC_KEY env var
}

func NewServer(registry *ConnectionRegistry, fanout *BufferedFanout, rdb *redis.Client) (*Server, error) {
	pubKeyPEM := os.Getenv("JWT_PUBLIC_KEY")
	if pubKeyPEM == "" {
		return nil, fmt.Errorf("JWT_PUBLIC_KEY not set")
	}
	pubKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(pubKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("parse JWT_PUBLIC_KEY: %w", err)
	}
	return &Server{
		registry: registry,
		fanout:   fanout,
		rdb:      rdb,
		jwtPub:   pubKey,
	}, nil
}

// ServeHTTP handles the WebSocket upgrade at GET /ws.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// ADR-002 compensating control: TLS enforcement in production.
	if os.Getenv("APP_ENV") == "production" && r.TLS == nil {
		http.Error(w, "WSS required", http.StatusForbidden)
		return
	}

	// Limit upgrade request body size before any parsing.
	r.Body = http.MaxBytesReader(w, r.Body, maxUpgradeBodySize)

	// Handshake rate limit: 20 new WS upgrades per IP per minute.
	ip := realIP(r)
	if !s.checkHandshakeRateLimit(r.Context(), ip) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}

	// JWT auth from query param: wss://host:4002/ws?token=<jwt>
	// Query param is used because WebSocket upgrade requests cannot carry custom headers
	// from Android's OkHttp WebSocket API.
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	userID, err := s.validateJWT(tokenStr)
	if err != nil {
		slog.Debug("ws: invalid JWT", "err", err, "ip", ip)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Debug("ws: upgrade failed", "err", err)
		return
	}

	conn.SetReadLimit(maxMessageSize)
	s.registry.Register(userID, conn)

	slog.Debug("ws: connection opened", "user_id", userID)

	// SMELL-3 FIX: connection lifecycle is owned exclusively by readLoop's defer.
	// RunHeartbeat signals liveness only — it returns on ping failure but does NOT
	// close the connection or remove from registry. The read deadline it sets on
	// the conn causes readLoop's next ReadMessage to fail, which triggers the
	// single authoritative cleanup path in readLoop's defer.
	go RunHeartbeat(conn)

	s.readLoop(conn, userID)
}

// readLoop processes inbound messages (channel subscribe/unsubscribe) until the
// connection closes. It is the single authoritative cleanup path for the connection.
func (s *Server) readLoop(conn *websocket.Conn, userID string) {
	defer func() {
		// Single authoritative cleanup — heartbeat goroutine only sets deadlines.
		s.registry.Remove(userID, conn)
		conn.Close()
		slog.Debug("ws: connection closed", "user_id", userID)
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}

		env, err := ParseEnvelope(data)
		if err != nil {
			slog.Debug("ws: bad envelope", "user_id", userID, "err", err)
			continue
		}

		switch env.Channel {
		case ChannelRates:
			var payload RatesSubscribePayload
			if err := json.Unmarshal(env.Payload, &payload); err != nil {
				continue
			}
			if payload.CityID != "" {
				s.fanout.Subscribe(payload.CityID, userID)
			}
		default:
			slog.Debug("ws: unknown channel", "channel", env.Channel, "user_id", userID)
		}
	}
}

// checkHandshakeRateLimit enforces 20 new WS upgrades per IP per minute using Redis.
// Key: ws_hs:{ip} — INCR with 60s EXPIRE.
func (s *Server) checkHandshakeRateLimit(ctx context.Context, ip string) bool {
	key := "ws_hs:" + ip
	n, err := s.rdb.Incr(ctx, key).Result()
	if err != nil {
		// Redis error: fail open (allow) to avoid blocking all connections on Redis hiccup.
		slog.Warn("ws: handshake rate limit redis error", "err", err)
		return true
	}
	if n == 1 {
		// First request — set expiry.
		s.rdb.Expire(ctx, key, 60*time.Second)
	}
	return n <= wsHandshakeRateLimit
}

// validateJWT parses and verifies the RS256 JWT, returning the userID claim.
func (s *Server) validateJWT(tokenStr string) (string, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtPub, nil
	})
	if err != nil || !token.Valid {
		return "", fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid claims type")
	}

	userID, ok := claims["sub"].(string)
	if !ok || userID == "" {
		return "", fmt.Errorf("missing sub claim")
	}
	return userID, nil
}

// realIP extracts the client IP, respecting X-Forwarded-For if present.
func realIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	return r.RemoteAddr
}
