package game

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rapidtrivia/rapid-server/internal/db"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("game.store")

type Store struct {
	q    *db.Queries
	pool *pgxpool.Pool
}

func NewStore(q *db.Queries, pool *pgxpool.Pool) *Store {
	return &Store{q: q, pool: pool}
}

func (s *Store) ListCategories(ctx context.Context) ([]Category, error) {
	ctx, span := tracer.Start(ctx, "game.store.ListCategories")
	defer span.End()

	rows, err := s.q.ListCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing categories: %w", err)
	}

	cats := make([]Category, len(rows))
	for i, row := range rows {
		cats[i] = Category{
			ID:            row.ID.Bytes,
			Name:          row.Name,
			Slug:          row.Slug,
			Description:   row.Description,
			QuestionCount: int(row.QuestionCount),
		}
	}
	return cats, nil
}

func (s *Store) GetCategory(ctx context.Context, id uuid.UUID) (*Category, error) {
	ctx, span := tracer.Start(ctx, "game.store.GetCategory",
		trace.WithAttributes(attribute.String("category_id", id.String())))
	defer span.End()

	row, err := s.q.GetCategory(ctx, pgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting category: %w", err)
	}
	return &Category{
		ID:            row.ID.Bytes,
		Name:          row.Name,
		Slug:          row.Slug,
		Description:   row.Description,
		QuestionCount: int(row.QuestionCount),
	}, nil
}

func (s *Store) LoadRandomQuestions(ctx context.Context, categoryID uuid.UUID, count int) ([]Question, error) {
	ctx, span := tracer.Start(ctx, "game.store.LoadRandomQuestions",
		trace.WithAttributes(
			attribute.String("category_id", categoryID.String()),
			attribute.Int("count", count)))
	defer span.End()

	rows, err := s.q.LoadRandomQuestions(ctx, db.LoadRandomQuestionsParams{
		CategoryID: pgUUID(categoryID),
		Limit:      int32(count),
	})
	if err != nil {
		return nil, fmt.Errorf("loading random questions: %w", err)
	}

	questions := make([]Question, len(rows))
	for i, row := range rows {
		var opts []string
		if err := json.Unmarshal(row.Options, &opts); err != nil {
			return nil, fmt.Errorf("unmarshaling options for question %s: %w", row.ID.Bytes, err)
		}
		questions[i] = Question{
			ID:           row.ID.Bytes,
			CategoryID:   row.CategoryID.Bytes,
			QuestionText: row.QuestionText,
			Options:      opts,
			CorrectIndex: int(row.CorrectIndex),
			Difficulty:   int(row.Difficulty),
		}
	}
	return questions, nil
}

func (s *Store) CreateMatch(ctx context.Context, categoryID, player1ID, player2ID uuid.UUID) (*Match, error) {
	ctx, span := tracer.Start(ctx, "game.store.CreateMatch")
	defer span.End()

	row, err := s.q.CreateMatch(ctx, db.CreateMatchParams{
		CategoryID: pgUUID(categoryID),
		Player1ID:  pgUUID(player1ID),
		Player2ID:  pgUUID(player2ID),
	})
	if err != nil {
		return nil, fmt.Errorf("creating match: %w", err)
	}
	return mapMatch(row), nil
}

func (s *Store) InsertMatchRound(ctx context.Context, p InsertRoundParams) error {
	ctx, span := tracer.Start(ctx, "game.store.InsertMatchRound",
		trace.WithAttributes(attribute.Int("round", p.RoundNumber)))
	defer span.End()

	return s.q.InsertMatchRound(ctx, db.InsertMatchRoundParams{
		MatchID:     pgUUID(p.MatchID),
		RoundNumber: int32(p.RoundNumber),
		QuestionID:  pgUUID(p.QuestionID),
		P1Choice:    pgInt4(p.P1Choice),
		P1Correct:   pgBool(p.P1Correct),
		P1TimeMs:    pgInt4(p.P1TimeMs),
		P1Points:    int32(p.P1Points),
		P2Choice:    pgInt4(p.P2Choice),
		P2Correct:   pgBool(p.P2Correct),
		P2TimeMs:    pgInt4(p.P2TimeMs),
		P2Points:    int32(p.P2Points),
	})
}

func (s *Store) CompleteMatch(ctx context.Context, p CompleteMatchParams) error {
	ctx, span := tracer.Start(ctx, "game.store.CompleteMatch",
		trace.WithAttributes(attribute.String("match_id", p.MatchID.String())))
	defer span.End()

	return s.q.CompleteMatch(ctx, db.CompleteMatchParams{
		MatchID:  pgUUID(p.MatchID),
		P1Score:  int32(p.P1Score),
		P2Score:  int32(p.P2Score),
		WinnerID: pgUUIDPtr(p.WinnerID),
	})
}

func (s *Store) UpdateBothRatings(ctx context.Context, p1ID uuid.UUID, p1Rating int, p2ID uuid.UUID, p2Rating int) error {
	ctx, span := tracer.Start(ctx, "game.store.UpdateBothRatings")
	defer span.End()

	return s.q.UpdateBothRatings(ctx, db.UpdateBothRatingsParams{
		P1ID:     pgUUID(p1ID),
		P1Rating: int32(p1Rating),
		P2ID:     pgUUID(p2ID),
		P2Rating: int32(p2Rating),
	})
}

// CompleteMatchTx atomically completes a match and updates both players' ratings.
func (s *Store) CompleteMatchTx(ctx context.Context, p CompleteMatchTxParams) error {
	ctx, span := tracer.Start(ctx, "game.store.CompleteMatchTx",
		trace.WithAttributes(attribute.String("match_id", p.MatchID.String())))
	defer span.End()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)

	if err := qtx.CompleteMatch(ctx, db.CompleteMatchParams{
		MatchID:  pgUUID(p.MatchID),
		P1Score:  int32(p.P1Score),
		P2Score:  int32(p.P2Score),
		WinnerID: pgUUIDPtr(p.WinnerID),
	}); err != nil {
		return fmt.Errorf("completing match: %w", err)
	}

	if err := qtx.UpdateBothRatings(ctx, db.UpdateBothRatingsParams{
		P1ID:     pgUUID(p.P1ID),
		P1Rating: int32(p.P1Rating),
		P2ID:     pgUUID(p.P2ID),
		P2Rating: int32(p.P2Rating),
	}); err != nil {
		return fmt.Errorf("updating ratings: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *Store) GetMatchWithRounds(ctx context.Context, matchID uuid.UUID) (*MatchDetail, error) {
	ctx, span := tracer.Start(ctx, "game.store.GetMatchWithRounds",
		trace.WithAttributes(attribute.String("match_id", matchID.String())))
	defer span.End()

	mRow, err := s.q.GetMatch(ctx, pgUUID(matchID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting match: %w", err)
	}

	rows, err := s.q.GetMatchWithRounds(ctx, pgUUID(matchID))
	if err != nil {
		return nil, fmt.Errorf("getting match rounds: %w", err)
	}

	detail := &MatchDetail{Match: *mapMatchRow(mRow)}
	detail.Rounds = make([]RoundDetail, len(rows))
	for i, row := range rows {
		var opts []string
		json.Unmarshal(row.Options, &opts)

		rd := RoundDetail{
			RoundNumber:  int(row.RoundNumber),
			QuestionText: row.QuestionText,
			Options:      opts,
			CorrectIndex: int(row.CorrectIndex),
			P1Points:     int(row.P1Points),
			P2Points:     int(row.P2Points),
		}
		if row.P1Choice.Valid {
			v := int(row.P1Choice.Int32)
			rd.P1Choice = &v
		}
		if row.P1Correct.Valid {
			rd.P1Correct = &row.P1Correct.Bool
		}
		if row.P1TimeMs.Valid {
			v := int(row.P1TimeMs.Int32)
			rd.P1TimeMs = &v
		}
		if row.P2Choice.Valid {
			v := int(row.P2Choice.Int32)
			rd.P2Choice = &v
		}
		if row.P2Correct.Valid {
			rd.P2Correct = &row.P2Correct.Bool
		}
		if row.P2TimeMs.Valid {
			v := int(row.P2TimeMs.Int32)
			rd.P2TimeMs = &v
		}
		detail.Rounds[i] = rd
	}
	return detail, nil
}

func (s *Store) ListUserMatches(ctx context.Context, userID uuid.UUID, limit, offset int) ([]MatchSummary, error) {
	ctx, span := tracer.Start(ctx, "game.store.ListUserMatches",
		trace.WithAttributes(attribute.String("user.id", userID.String())))
	defer span.End()

	rows, err := s.q.ListUserMatches(ctx, db.ListUserMatchesParams{
		UserID:     pgUUID(userID),
		PageLimit:  int32(limit),
		PageOffset: int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("listing user matches: %w", err)
	}

	matches := make([]MatchSummary, len(rows))
	for i, row := range rows {
		m := MatchSummary{
			Match: Match{
				ID:           row.ID.Bytes,
				CategoryID:   row.CategoryID.Bytes,
				Player1ID:    row.Player1ID.Bytes,
				Player2ID:    row.Player2ID.Bytes,
				Player1Score: int(row.Player1Score),
				Player2Score: int(row.Player2Score),
				Status:       row.Status,
				CreatedAt:    row.CreatedAt.Time,
			},
			CategoryName:    row.CategoryName,
			Player1Username: row.Player1Username,
			Player2Username: row.Player2Username,
		}
		if row.WinnerID.Valid {
			id := uuid.UUID(row.WinnerID.Bytes)
			m.WinnerID = &id
		}
		if row.CompletedAt.Valid {
			m.CompletedAt = &row.CompletedAt.Time
		}
		matches[i] = m
	}
	return matches, nil
}

func mapMatchRow(row db.GetMatchRow) *Match {
	m := &Match{
		ID:           row.ID.Bytes,
		CategoryID:   row.CategoryID.Bytes,
		Player1ID:    row.Player1ID.Bytes,
		Player2ID:    row.Player2ID.Bytes,
		Player1Score: int(row.Player1Score),
		Player2Score: int(row.Player2Score),
		Status:       row.Status,
		CreatedAt:    row.CreatedAt.Time,
	}
	if row.WinnerID.Valid {
		id := uuid.UUID(row.WinnerID.Bytes)
		m.WinnerID = &id
	}
	if row.CompletedAt.Valid {
		m.CompletedAt = &row.CompletedAt.Time
	}
	return m
}

func mapMatch(row db.Match) *Match {
	m := &Match{
		ID:           row.ID.Bytes,
		CategoryID:   row.CategoryID.Bytes,
		Player1ID:    row.Player1ID.Bytes,
		Player2ID:    row.Player2ID.Bytes,
		Player1Score: int(row.Player1Score),
		Player2Score: int(row.Player2Score),
		Status:       row.Status,
		CreatedAt:    row.CreatedAt.Time,
	}
	if row.WinnerID.Valid {
		id := uuid.UUID(row.WinnerID.Bytes)
		m.WinnerID = &id
	}
	if row.CompletedAt.Valid {
		m.CompletedAt = &row.CompletedAt.Time
	}
	return m
}

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func pgUUIDPtr(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

func pgInt4(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*v), Valid: true}
}

func pgBool(v *bool) pgtype.Bool {
	if v == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *v, Valid: true}
}
