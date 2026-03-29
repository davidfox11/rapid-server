package friend

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rapidtrivia/rapid-server/internal/db"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("friend.store")

type Store struct {
	q *db.Queries
}

func NewStore(q *db.Queries) *Store {
	return &Store{q: q}
}

func (s *Store) CreateFriendship(ctx context.Context, requesterID, addresseeID uuid.UUID) (*Friendship, error) {
	ctx, span := tracer.Start(ctx, "friend.store.CreateFriendship",
		trace.WithAttributes(
			attribute.String("requester_id", requesterID.String()),
			attribute.String("addressee_id", addresseeID.String())))
	defer span.End()

	row, err := s.q.CreateFriendship(ctx, db.CreateFriendshipParams{
		RequesterID: pgUUID(requesterID),
		AddresseeID: pgUUID(addresseeID),
	})
	if err != nil {
		return nil, fmt.Errorf("creating friendship: %w", err)
	}
	return mapFriendship(row), nil
}

func (s *Store) FindBetween(ctx context.Context, userA, userB uuid.UUID) (*Friendship, error) {
	ctx, span := tracer.Start(ctx, "friend.store.FindBetween")
	defer span.End()

	row, err := s.q.FindFriendshipBetween(ctx, db.FindFriendshipBetweenParams{
		UserA: pgUUID(userA),
		UserB: pgUUID(userB),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("finding friendship between users: %w", err)
	}
	return mapFriendship(row), nil
}

func (s *Store) GetFriendship(ctx context.Context, id uuid.UUID) (*Friendship, error) {
	ctx, span := tracer.Start(ctx, "friend.store.GetFriendship",
		trace.WithAttributes(attribute.String("friendship_id", id.String())))
	defer span.End()

	row, err := s.q.GetFriendship(ctx, pgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting friendship: %w", err)
	}
	return mapFriendship(row), nil
}

func (s *Store) RespondToFriendship(ctx context.Context, friendshipID uuid.UUID, accepted bool) error {
	ctx, span := tracer.Start(ctx, "friend.store.RespondToFriendship",
		trace.WithAttributes(
			attribute.String("friendship_id", friendshipID.String()),
			attribute.Bool("accepted", accepted)))
	defer span.End()

	status := "declined"
	if accepted {
		status = "accepted"
	}
	if err := s.q.UpdateFriendshipStatus(ctx, db.UpdateFriendshipStatusParams{
		Status: status,
		ID:     pgUUID(friendshipID),
	}); err != nil {
		return fmt.Errorf("updating friendship status: %w", err)
	}
	return nil
}

func (s *Store) ListAcceptedFriends(ctx context.Context, userID uuid.UUID) ([]db.User, error) {
	ctx, span := tracer.Start(ctx, "friend.store.ListAcceptedFriends",
		trace.WithAttributes(attribute.String("user.id", userID.String())))
	defer span.End()

	rows, err := s.q.ListAcceptedFriends(ctx, pgUUID(userID))
	if err != nil {
		return nil, fmt.Errorf("listing accepted friends: %w", err)
	}
	return rows, nil
}

func (s *Store) ListPendingRequests(ctx context.Context, userID uuid.UUID) ([]PendingRequest, error) {
	ctx, span := tracer.Start(ctx, "friend.store.ListPendingRequests",
		trace.WithAttributes(attribute.String("user.id", userID.String())))
	defer span.End()

	rows, err := s.q.ListPendingFriendRequests(ctx, pgUUID(userID))
	if err != nil {
		return nil, fmt.Errorf("listing pending requests: %w", err)
	}

	requests := make([]PendingRequest, len(rows))
	for i, row := range rows {
		requests[i] = PendingRequest{
			Friendship: Friendship{
				ID:          row.ID.Bytes,
				RequesterID: row.RequesterID.Bytes,
				AddresseeID: row.AddresseeID.Bytes,
				Status:      row.Status,
				CreatedAt:   row.CreatedAt.Time,
			},
			Username:           row.Username,
			DisplayName:        row.DisplayName,
			DefaultAvatarIndex: int(row.DefaultAvatarIndex),
			Rating:             int(row.Rating),
		}
		if row.AvatarUrl.Valid {
			requests[i].AvatarURL = &row.AvatarUrl.String
		}
	}
	return requests, nil
}

func (s *Store) GetH2HRecord(ctx context.Context, userID, opponentID uuid.UUID) (wins int64, losses int64, err error) {
	ctx, span := tracer.Start(ctx, "friend.store.GetH2HRecord",
		trace.WithAttributes(
			attribute.String("user.id", userID.String()),
			attribute.String("opponent_id", opponentID.String())))
	defer span.End()

	row, err := s.q.GetH2HRecord(ctx, db.GetH2HRecordParams{
		UserID:     pgUUID(userID),
		OpponentID: pgUUID(opponentID),
	})
	if err != nil {
		return 0, 0, fmt.Errorf("getting h2h record: %w", err)
	}
	return row.Wins, row.Losses, nil
}

func mapFriendship(row db.Friendship) *Friendship {
	return &Friendship{
		ID:          row.ID.Bytes,
		RequesterID: row.RequesterID.Bytes,
		AddresseeID: row.AddresseeID.Bytes,
		Status:      row.Status,
		CreatedAt:   row.CreatedAt.Time,
	}
}

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}
