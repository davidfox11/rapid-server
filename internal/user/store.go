package user

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

var tracer = otel.Tracer("user.store")

type Store struct {
	q *db.Queries
}

func NewStore(q *db.Queries) *Store {
	return &Store{q: q}
}

func (s *Store) FindByFirebaseUID(ctx context.Context, uid string) (*User, error) {
	ctx, span := tracer.Start(ctx, "user.store.FindByFirebaseUID",
		trace.WithAttributes(attribute.String("firebase_uid", uid)))
	defer span.End()

	row, err := s.q.FindUserByFirebaseUID(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("finding user by firebase uid: %w", err)
	}
	return mapUser(row), nil
}

func (s *Store) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
	ctx, span := tracer.Start(ctx, "user.store.FindByID",
		trace.WithAttributes(attribute.String("user.id", id.String())))
	defer span.End()

	row, err := s.q.FindUserByID(ctx, pgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("finding user by id: %w", err)
	}
	return mapUser(row), nil
}

func (s *Store) Create(ctx context.Context, params CreateParams) (*User, error) {
	ctx, span := tracer.Start(ctx, "user.store.Create",
		trace.WithAttributes(attribute.String("username", params.Username)))
	defer span.End()

	row, err := s.q.CreateUser(ctx, db.CreateUserParams{
		FirebaseUid: params.FirebaseUID,
		Username:    params.Username,
		DisplayName: params.DisplayName,
		AvatarUrl:   pgText(params.AvatarURL),
	})
	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}
	return mapUser(row), nil
}

func (s *Store) SearchByUsername(ctx context.Context, prefix string) ([]User, error) {
	ctx, span := tracer.Start(ctx, "user.store.SearchByUsername",
		trace.WithAttributes(attribute.String("prefix", prefix)))
	defer span.End()

	rows, err := s.q.SearchUsersByUsername(ctx, pgtype.Text{String: prefix, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("searching users by username: %w", err)
	}

	users := make([]User, len(rows))
	for i, row := range rows {
		users[i] = *mapUser(row)
	}
	return users, nil
}

func (s *Store) GetBestCategory(ctx context.Context, userID uuid.UUID) (*BestCategory, error) {
	ctx, span := tracer.Start(ctx, "user.store.GetBestCategory",
		trace.WithAttributes(attribute.String("user.id", userID.String())))
	defer span.End()

	row, err := s.q.GetUserBestCategory(ctx, pgUUID(userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting best category: %w", err)
	}

	var pct float64
	if row.AccuracyPct.Valid {
		f, _ := row.AccuracyPct.Float64Value()
		pct = f.Float64
	}

	return &BestCategory{
		Name:        row.Name,
		AccuracyPct: pct,
	}, nil
}

func (s *Store) CountPendingFriendRequests(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := tracer.Start(ctx, "user.store.CountPendingFriendRequests",
		trace.WithAttributes(attribute.String("user.id", userID.String())))
	defer span.End()

	count, err := s.q.CountPendingFriendRequests(ctx, pgUUID(userID))
	if err != nil {
		return 0, fmt.Errorf("counting pending friend requests: %w", err)
	}
	return count, nil
}

func (s *Store) UpdateLastSeen(ctx context.Context, userID uuid.UUID) error {
	ctx, span := tracer.Start(ctx, "user.store.UpdateLastSeen",
		trace.WithAttributes(attribute.String("user.id", userID.String())))
	defer span.End()

	if err := s.q.UpdateUserLastSeen(ctx, pgUUID(userID)); err != nil {
		return fmt.Errorf("updating last seen: %w", err)
	}
	return nil
}

// mapUser converts a sqlc-generated User to the domain User type.
func mapUser(row db.User) *User {
	u := &User{
		ID:                 row.ID.Bytes,
		FirebaseUID:        row.FirebaseUid,
		Username:           row.Username,
		DisplayName:        row.DisplayName,
		DefaultAvatarIndex: int(row.DefaultAvatarIndex),
		Rating:             int(row.Rating),
		RatingWeekStart:    int(row.RatingWeekStart),
		CreatedAt:          row.CreatedAt.Time,
	}
	if row.AvatarUrl.Valid {
		u.AvatarURL = &row.AvatarUrl.String
	}
	if row.LastSeenAt.Valid {
		u.LastSeenAt = &row.LastSeenAt.Time
	}
	return u
}

// pgUUID converts a uuid.UUID to pgtype.UUID.
func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// pgText converts a *string to pgtype.Text.
func pgText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}
