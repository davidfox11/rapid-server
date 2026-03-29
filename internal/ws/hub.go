package ws

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/rapidtrivia/rapid-server/internal/auth"
	"github.com/rapidtrivia/rapid-server/internal/user"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Hub struct {
	clients    map[uuid.UUID]*Client
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	logger     *slog.Logger
	userStore  *user.Store
}

func NewHub(logger *slog.Logger, userStore *user.Store) *Hub {
	return &Hub{
		clients:    make(map[uuid.UUID]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		logger:     logger,
		userStore:  userStore,
	}
}

func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			h.mu.Unlock()
			connectionsActive.Add(ctx, 1)
			connectionsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "connected")))
			h.logger.Info("client connected", "user_id", client.ID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				close(client.Send)
			}
			h.mu.Unlock()
			connectionsActive.Add(ctx, -1)
			connectionsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "disconnected")))
			h.logger.Info("client disconnected", "user_id", client.ID)

			if h.userStore != nil {
				_ = h.userStore.UpdateLastSeen(ctx, client.ID)
			}

		case <-ctx.Done():
			return
		}
	}
}

func (h *Hub) GetClient(userID uuid.UUID) *Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.clients[userID]
}

// Satisfies friend.PresenceChecker
func (h *Hub) GetPresence(userID uuid.UUID) string {
	h.mu.RLock()
	_, connected := h.clients[userID]
	h.mu.RUnlock()
	if connected {
		return "online"
	}
	return "offline"
}

// handleMessage routes challenge/challenge_resp messages.
// Game logic routing will be added in prompt 09.
func (h *Hub) handleMessage(from *Client, env Envelope) {
	h.logger.Debug("hub received message", "type", env.Type, "from", from.ID)
}

func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	firebaseUID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	u, err := h.userStore.FindByFirebaseUID(r.Context(), firebaseUID)
	if err != nil || u == nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // allow any origin in dev
	})
	if err != nil {
		h.logger.Error("websocket accept failed", "error", err)
		return
	}

	client := newClient(u.ID, conn, h, h.logger)
	h.register <- client

	ctx := r.Context()
	go client.writePump(ctx)
	go client.pingLoop(ctx)
	client.readPump(ctx)
}
