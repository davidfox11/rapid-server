package user

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID                 uuid.UUID  `json:"id"`
	FirebaseUID        string     `json:"firebase_uid"`
	Username           string     `json:"username"`
	DisplayName        string     `json:"display_name"`
	AvatarURL          *string    `json:"avatar_url"`
	DefaultAvatarIndex int        `json:"default_avatar_index"`
	Rating             int        `json:"rating"`
	RatingWeekStart    int        `json:"rating_week_start"`
	LastSeenAt         *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

type BestCategory struct {
	Name        string  `json:"name"`
	AccuracyPct float64 `json:"accuracy_pct"`
}

type CreateParams struct {
	FirebaseUID string
	Username    string
	DisplayName string
	AvatarURL   *string
}

type ProfileResponse struct {
	User
	BestCategory               *BestCategory `json:"best_category"`
	PendingFriendRequestsCount int64         `json:"pending_friend_requests_count"`
	RatingDeltaWeek            int           `json:"rating_delta_week"`
}
