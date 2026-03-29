package game

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/rapidtrivia/rapid-server/internal/auth"
	"github.com/rapidtrivia/rapid-server/internal/user"
)

type Handler struct {
	store  *Store
	users  *user.Store
	logger *slog.Logger
}

func NewHandler(store *Store, users *user.Store, logger *slog.Logger) *Handler {
	return &Handler{store: store, users: users, logger: logger}
}

func (h *Handler) ListCategories() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cats, err := h.store.ListCategories(r.Context())
		if err != nil {
			h.logger.Error("listing categories", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, cats)
	}
}

func (h *Handler) ListMatches() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		firebaseUID, _ := auth.UserIDFromContext(r.Context())
		u, err := h.users.FindByFirebaseUID(r.Context(), firebaseUID)
		if err != nil || u == nil {
			writeError(w, http.StatusUnauthorized, "user not found")
			return
		}

		limit := 20
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 50 {
				limit = n
			}
		}
		offset := 0
		if v := r.URL.Query().Get("offset"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				offset = n
			}
		}

		matches, err := h.store.ListUserMatches(r.Context(), u.ID, limit, offset)
		if err != nil {
			h.logger.Error("listing matches", "error", err, "user_id", u.ID)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, matches)
	}
}

func (h *Handler) GetMatch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		firebaseUID, _ := auth.UserIDFromContext(r.Context())
		u, err := h.users.FindByFirebaseUID(r.Context(), firebaseUID)
		if err != nil || u == nil {
			writeError(w, http.StatusUnauthorized, "user not found")
			return
		}

		matchID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid match id")
			return
		}

		detail, err := h.store.GetMatchWithRounds(r.Context(), matchID)
		if err != nil {
			h.logger.Error("getting match", "error", err, "match_id", matchID)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if detail == nil {
			writeError(w, http.StatusNotFound, "match not found")
			return
		}

		if detail.Player1ID != u.ID && detail.Player2ID != u.ID {
			writeError(w, http.StatusForbidden, "not a participant in this match")
			return
		}

		writeJSON(w, http.StatusOK, detail)
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
