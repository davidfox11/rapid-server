package friend

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/rapidtrivia/rapid-server/internal/auth"
	"github.com/rapidtrivia/rapid-server/internal/user"
)

type Handler struct {
	store    *Store
	users    *user.Store
	presence PresenceChecker
	logger   *slog.Logger
}

func NewHandler(store *Store, users *user.Store, presence PresenceChecker, logger *slog.Logger) *Handler {
	return &Handler{store: store, users: users, presence: presence, logger: logger}
}

func (h *Handler) Request() http.HandlerFunc {
	type request struct {
		AddresseeID string `json:"addressee_id"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		firebaseUID, _ := auth.UserIDFromContext(r.Context())
		currentUser, err := h.users.FindByFirebaseUID(r.Context(), firebaseUID)
		if err != nil || currentUser == nil {
			writeError(w, http.StatusUnauthorized, "user not found")
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		addresseeID, err := uuid.Parse(req.AddresseeID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid addressee_id")
			return
		}

		if addresseeID == currentUser.ID {
			writeError(w, http.StatusBadRequest, "cannot send friend request to yourself")
			return
		}

		addressee, err := h.users.FindByID(r.Context(), addresseeID)
		if err != nil {
			h.logger.Error("finding addressee", "error", err, "addressee_id", addresseeID)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if addressee == nil {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}

		existing, err := h.store.FindBetween(r.Context(), currentUser.ID, addresseeID)
		if err != nil {
			h.logger.Error("checking existing friendship", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if existing != nil {
			writeError(w, http.StatusConflict, "friendship already exists")
			return
		}

		friendship, err := h.store.CreateFriendship(r.Context(), currentUser.ID, addresseeID)
		if err != nil {
			h.logger.Error("creating friendship", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		h.logger.Info("friend request sent", "from", currentUser.ID, "to", addresseeID)
		writeJSON(w, http.StatusCreated, friendship)
	}
}

func (h *Handler) Respond() http.HandlerFunc {
	type request struct {
		FriendshipID string `json:"friendship_id"`
		Accepted     bool   `json:"accepted"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		firebaseUID, _ := auth.UserIDFromContext(r.Context())
		currentUser, err := h.users.FindByFirebaseUID(r.Context(), firebaseUID)
		if err != nil || currentUser == nil {
			writeError(w, http.StatusUnauthorized, "user not found")
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		friendshipID, err := uuid.Parse(req.FriendshipID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid friendship_id")
			return
		}

		friendship, err := h.store.GetFriendship(r.Context(), friendshipID)
		if err != nil {
			h.logger.Error("getting friendship", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if friendship == nil {
			writeError(w, http.StatusNotFound, "friendship not found")
			return
		}

		if friendship.AddresseeID != currentUser.ID {
			writeError(w, http.StatusForbidden, "not the addressee of this request")
			return
		}
		if friendship.Status != "pending" {
			writeError(w, http.StatusConflict, "friendship already responded to")
			return
		}

		if err := h.store.RespondToFriendship(r.Context(), friendshipID, req.Accepted); err != nil {
			h.logger.Error("responding to friendship", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		action := "declined"
		if req.Accepted {
			action = "accepted"
		}
		h.logger.Info("friend request "+action, "friendship_id", friendshipID, "by", currentUser.ID)
		writeJSON(w, http.StatusOK, map[string]string{"status": action})
	}
}

func (h *Handler) List() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		firebaseUID, _ := auth.UserIDFromContext(r.Context())
		currentUser, err := h.users.FindByFirebaseUID(r.Context(), firebaseUID)
		if err != nil || currentUser == nil {
			writeError(w, http.StatusUnauthorized, "user not found")
			return
		}

		rows, err := h.store.ListAcceptedFriends(r.Context(), currentUser.ID)
		if err != nil {
			h.logger.Error("listing friends", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		friends := make([]FriendWithPresence, len(rows))
		for i, row := range rows {
			friendID := uuid.UUID(row.ID.Bytes)
			wins, losses, _ := h.store.GetH2HRecord(r.Context(), currentUser.ID, friendID)

			friends[i] = FriendWithPresence{
				ID:                 friendID,
				Username:           row.Username,
				DisplayName:        row.DisplayName,
				DefaultAvatarIndex: int(row.DefaultAvatarIndex),
				Rating:             int(row.Rating),
				Status:             h.presence.GetPresence(friendID),
				H2HWins:            wins,
				H2HLosses:          losses,
			}
			if row.AvatarUrl.Valid {
				friends[i].AvatarURL = &row.AvatarUrl.String
			}
			if row.LastSeenAt.Valid {
				friends[i].LastSeenAt = &row.LastSeenAt.Time
			}
		}

		writeJSON(w, http.StatusOK, friends)
	}
}

func (h *Handler) Search() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		firebaseUID, _ := auth.UserIDFromContext(r.Context())
		currentUser, err := h.users.FindByFirebaseUID(r.Context(), firebaseUID)
		if err != nil || currentUser == nil {
			writeError(w, http.StatusUnauthorized, "user not found")
			return
		}

		q := r.URL.Query().Get("q")
		if len(q) < 2 {
			writeError(w, http.StatusBadRequest, "search query must be at least 2 characters")
			return
		}

		results, err := h.users.SearchByUsername(r.Context(), q)
		if err != nil {
			h.logger.Error("searching users", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// Exclude current user from results
		filtered := make([]user.User, 0, len(results))
		for _, u := range results {
			if u.ID != currentUser.ID {
				filtered = append(filtered, u)
			}
		}

		writeJSON(w, http.StatusOK, filtered)
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
