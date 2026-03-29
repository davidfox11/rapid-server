package user

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/rapidtrivia/rapid-server/internal/auth"
)

var usernameRegex = regexp.MustCompile(`^[a-z0-9_]{3,20}$`)

type Handler struct {
	store  *Store
	logger *slog.Logger
}

func NewHandler(store *Store, logger *slog.Logger) *Handler {
	return &Handler{store: store, logger: logger}
}

func (h *Handler) Register() http.HandlerFunc {
	type request struct {
		Username    string  `json:"username"`
		DisplayName string  `json:"display_name"`
		AvatarURL   *string `json:"avatar_url"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := auth.UserIDFromContext(r.Context())

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		req.Username = strings.TrimSpace(strings.ToLower(req.Username))
		req.DisplayName = strings.TrimSpace(req.DisplayName)

		if !usernameRegex.MatchString(req.Username) {
			writeError(w, http.StatusBadRequest, "username must be 3-20 lowercase alphanumeric characters or underscores")
			return
		}
		if req.DisplayName == "" {
			writeError(w, http.StatusBadRequest, "display_name is required")
			return
		}

		existing, err := h.store.FindByFirebaseUID(r.Context(), uid)
		if err != nil {
			h.logger.Error("checking existing user", "error", err, "firebase_uid", uid)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if existing != nil {
			writeError(w, http.StatusConflict, "user already registered")
			return
		}

		u, err := h.store.Create(r.Context(), CreateParams{
			FirebaseUID: uid,
			Username:    req.Username,
			DisplayName: req.DisplayName,
			AvatarURL:   req.AvatarURL,
		})
		if err != nil {
			if strings.Contains(err.Error(), "users_username_key") {
				writeError(w, http.StatusConflict, "username already taken")
				return
			}
			h.logger.Error("creating user", "error", err, "firebase_uid", uid)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		h.logger.Info("user registered", "user_id", u.ID, "username", u.Username)
		writeJSON(w, http.StatusCreated, u)
	}
}

func (h *Handler) Me() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := auth.UserIDFromContext(r.Context())

		u, err := h.store.FindByFirebaseUID(r.Context(), uid)
		if err != nil {
			h.logger.Error("finding user", "error", err, "firebase_uid", uid)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if u == nil {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}

		bestCat, err := h.store.GetBestCategory(r.Context(), u.ID)
		if err != nil {
			h.logger.Error("getting best category", "error", err, "user_id", u.ID)
		}

		pendingCount, err := h.store.CountPendingFriendRequests(r.Context(), u.ID)
		if err != nil {
			h.logger.Error("counting pending friend requests", "error", err, "user_id", u.ID)
		}

		resp := ProfileResponse{
			User:                       *u,
			BestCategory:               bestCat,
			PendingFriendRequestsCount: pendingCount,
			RatingDeltaWeek:            u.Rating - u.RatingWeekStart,
		}

		writeJSON(w, http.StatusOK, resp)
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
