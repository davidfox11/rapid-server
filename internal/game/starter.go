package game

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/rapidtrivia/rapid-server/internal/user"
	"github.com/rapidtrivia/rapid-server/internal/ws"
)

type Starter struct {
	store    *Store
	presence PresenceNotifier
	logger   *slog.Logger
}

func NewStarter(store *Store, presence PresenceNotifier, logger *slog.Logger) *Starter {
	return &Starter{store: store, presence: presence, logger: logger}
}

func (s *Starter) StartGame(ctx context.Context, p1, p2 *ws.Client, p1User, p2User *user.User, categoryID uuid.UUID) {
	cat, err := s.store.GetCategory(ctx, categoryID)
	if err != nil || cat == nil {
		s.logger.Error("category not found for game start", "category_id", categoryID, "error", err)
		errMsg, _ := ws.NewEnvelope("error", ws.ErrorMsg{Message: "category not found"})
		p1.Send <- errMsg
		p2.Send <- errMsg
		return
	}

	questions, err := s.store.LoadRandomQuestions(ctx, categoryID, totalRounds)
	if err != nil || len(questions) < totalRounds {
		s.logger.Error("not enough questions", "category_id", categoryID, "loaded", len(questions), "error", err)
		errMsg, _ := ws.NewEnvelope("error", ws.ErrorMsg{Message: "not enough questions in this category"})
		p1.Send <- errMsg
		p2.Send <- errMsg
		return
	}

	match, err := s.store.CreateMatch(ctx, categoryID, p1.ID, p2.ID)
	if err != nil {
		s.logger.Error("creating match", "error", err)
		errMsg, _ := ws.NewEnvelope("error", ws.ErrorMsg{Message: "failed to create match"})
		p1.Send <- errMsg
		p2.Send <- errMsg
		return
	}

	session := NewGameSession(NewSessionParams{
		MatchID:     match.ID,
		Category:    *cat,
		Questions:   questions,
		Player1:     p1,
		Player2:     p2,
		P1Rating:    p1User.Rating,
		P2Rating:    p2User.Rating,
		P1Name:      p1User.DisplayName,
		P2Name:      p2User.DisplayName,
		P1Avatar:    p1User.AvatarURL,
		P2Avatar:    p2User.AvatarURL,
		P1AvatarIdx: p1User.DefaultAvatarIndex,
		P2AvatarIdx: p2User.DefaultAvatarIndex,
		Store:       s.store,
		Presence:    s.presence,
		Logger:      s.logger,
	})

	go session.Run(ctx)
}
