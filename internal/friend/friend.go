package friend

import (
	"time"

	"github.com/google/uuid"
)

type Friendship struct {
	ID          uuid.UUID `json:"id"`
	RequesterID uuid.UUID `json:"requester_id"`
	AddresseeID uuid.UUID `json:"addressee_id"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type PendingRequest struct {
	Friendship
	Username           string  `json:"username"`
	DisplayName        string  `json:"display_name"`
	AvatarURL          *string `json:"avatar_url"`
	DefaultAvatarIndex int     `json:"default_avatar_index"`
	Rating             int     `json:"rating"`
}

type FriendWithPresence struct {
	ID                 uuid.UUID  `json:"id"`
	Username           string     `json:"username"`
	DisplayName        string     `json:"display_name"`
	AvatarURL          *string    `json:"avatar_url"`
	DefaultAvatarIndex int        `json:"default_avatar_index"`
	Rating             int        `json:"rating"`
	Status             string     `json:"status"`
	LastSeenAt         *time.Time `json:"last_seen_at,omitempty"`
	H2HWins            int64      `json:"h2h_wins"`
	H2HLosses          int64      `json:"h2h_losses"`
}

type PresenceChecker interface {
	GetPresence(userID uuid.UUID) string
}

type OfflinePresenceChecker struct{}

func (o *OfflinePresenceChecker) GetPresence(_ uuid.UUID) string {
	return "offline"
}
