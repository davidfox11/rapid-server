package ws

import (
	"context"
	"encoding/json"
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

// GameStarter is satisfied by the game package to avoid an import cycle.
// The hub calls this when a challenge is accepted.
type GameStarter interface {
	StartGame(ctx context.Context, p1, p2 *Client, p1User, p2User *user.User, categoryID uuid.UUID)
}

type pendingChallenge struct {
	challengerID uuid.UUID
	categoryID   uuid.UUID
	matchID      uuid.UUID
}

type Hub struct {
	clients           map[uuid.UUID]*Client
	register          chan *Client
	unregister        chan *Client
	mu                sync.RWMutex
	logger            *slog.Logger
	userStore         *user.Store
	pendingChallenges map[uuid.UUID]*pendingChallenge // keyed by matchID
	challengeMu       sync.Mutex
	gameStarter       GameStarter
}

func NewHub(logger *slog.Logger, userStore *user.Store) *Hub {
	return &Hub{
		clients:           make(map[uuid.UUID]*Client),
		register:          make(chan *Client),
		unregister:        make(chan *Client),
		logger:            logger,
		userStore:         userStore,
		pendingChallenges: make(map[uuid.UUID]*pendingChallenge),
	}
}

func (h *Hub) SetGameStarter(gs GameStarter) {
	h.gameStarter = gs
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

func (h *Hub) GetPresence(userID uuid.UUID) string {
	h.mu.RLock()
	_, connected := h.clients[userID]
	h.mu.RUnlock()
	if connected {
		return "online"
	}
	return "offline"
}

func (h *Hub) handleMessage(from *Client, env Envelope) {
	switch env.Type {
	case "challenge":
		h.handleChallenge(from, env)
	case "challenge_resp":
		h.handleChallengeResp(from, env)
	default:
		h.logger.Debug("hub received unknown message", "type", env.Type, "from", from.ID)
	}
}

func (h *Hub) handleChallenge(from *Client, env Envelope) {
	var msg ChallengeMsg
	if err := json.Unmarshal(env.Payload, &msg); err != nil {
		h.sendError(from, "invalid challenge payload")
		return
	}

	opponent := h.GetClient(msg.OpponentID)
	if opponent == nil {
		h.sendError(from, "opponent is not online")
		return
	}

	ctx := context.Background()
	challenger, err := h.userStore.FindByID(ctx, from.ID)
	if err != nil || challenger == nil {
		h.sendError(from, "internal error")
		return
	}

	matchID := uuid.New()

	h.challengeMu.Lock()
	h.pendingChallenges[matchID] = &pendingChallenge{
		challengerID: from.ID,
		categoryID:   msg.CategoryID,
		matchID:      matchID,
	}
	h.challengeMu.Unlock()

	challengeRecv, _ := NewEnvelope("challenge_recv", ChallengeRecvMsg{
		MatchID:                    matchID,
		ChallengerID:               from.ID,
		ChallengerName:             challenger.DisplayName,
		ChallengerAvatarURL:        challenger.AvatarURL,
		ChallengerDefaultAvatarIdx: challenger.DefaultAvatarIndex,
		CategoryID:                 msg.CategoryID,
		CategoryName:               "",
		H2HYou:                     0,
		H2HThem:                    0,
	})

	select {
	case opponent.Send <- challengeRecv:
	default:
		h.sendError(from, "opponent is busy")
	}

	h.logger.Info("challenge sent", "from", from.ID, "to", msg.OpponentID, "match_id", matchID)
}

func (h *Hub) handleChallengeResp(from *Client, env Envelope) {
	var msg ChallengeRespMsg
	if err := json.Unmarshal(env.Payload, &msg); err != nil {
		h.logger.Error("invalid challenge_resp payload", "error", err, "raw", string(env.Payload))
		h.sendError(from, "invalid challenge_resp payload")
		return
	}
	h.logger.Info("challenge_resp received", "match_id", msg.MatchID, "accepted", msg.Accepted, "from", from.ID)

	h.challengeMu.Lock()
	pending, ok := h.pendingChallenges[msg.MatchID]
	if ok {
		delete(h.pendingChallenges, msg.MatchID)
	}
	h.challengeMu.Unlock()

	if !ok {
		h.sendError(from, "challenge not found or expired")
		return
	}

	challenger := h.GetClient(pending.challengerID)

	if !msg.Accepted {
		if challenger != nil {
			declinedMsg, _ := NewEnvelope("error", ErrorMsg{Message: "challenge declined"})
			select {
			case challenger.Send <- declinedMsg:
			default:
			}
		}
		h.logger.Info("challenge declined", "match_id", msg.MatchID, "by", from.ID)
		return
	}

	if challenger == nil {
		h.sendError(from, "challenger is no longer online")
		return
	}

	if h.gameStarter == nil {
		h.sendError(from, "game system not available")
		return
	}

	ctx := context.Background()
	p1User, _ := h.userStore.FindByID(ctx, challenger.ID)
	p2User, _ := h.userStore.FindByID(ctx, from.ID)
	if p1User == nil || p2User == nil {
		h.sendError(from, "user lookup failed")
		return
	}

	h.gameStarter.StartGame(ctx, challenger, from, p1User, p2User, pending.categoryID)
	h.logger.Info("challenge accepted, starting game", "match_id", msg.MatchID)
}

func (h *Hub) sendError(client *Client, message string) {
	data, _ := NewEnvelope("error", ErrorMsg{Message: message})
	select {
	case client.Send <- data:
	default:
	}
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
		InsecureSkipVerify: true,
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
