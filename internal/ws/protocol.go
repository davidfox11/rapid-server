package ws

import (
	"encoding/json"

	"github.com/google/uuid"
)

type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Client → Server

type ChallengeMsg struct {
	OpponentID uuid.UUID `json:"opponent_id"`
	CategoryID uuid.UUID `json:"category_id"`
}

type ChallengeRespMsg struct {
	MatchID      uuid.UUID `json:"match_id"`
	ChallengerID string    `json:"challenger_id"`
	CategoryID   string    `json:"category_id"`
	Accepted     bool      `json:"accepted"`
}

type AnswerMsg struct {
	Round  int `json:"round"`
	Choice int `json:"choice"`
}

type PongMsg struct {
	PingID string `json:"ping_id"`
}

// Server → Client

type ChallengeRecvMsg struct {
	MatchID                    uuid.UUID `json:"match_id"`
	ChallengerID               uuid.UUID `json:"challenger_id"`
	ChallengerName             string    `json:"challenger_name"`
	ChallengerAvatarURL        *string   `json:"challenger_avatar_url"`
	ChallengerDefaultAvatarIdx int       `json:"challenger_default_avatar_index"`
	CategoryID                 uuid.UUID `json:"category_id"`
	CategoryName               string    `json:"category_name"`
	H2HYou                     int64     `json:"h2h_you"`
	H2HThem                    int64     `json:"h2h_them"`
}

type GameStartMsg struct {
	MatchID                  uuid.UUID `json:"match_id"`
	OpponentName             string    `json:"opponent_name"`
	OpponentAvatarURL        *string   `json:"opponent_avatar_url"`
	OpponentDefaultAvatarIdx int       `json:"opponent_default_avatar_index"`
	CategoryName             string    `json:"category_name"`
	TotalRounds              int       `json:"total_rounds"`
}

type QuestionMsg struct {
	Round     int      `json:"round"`
	Text      string   `json:"text"`
	Options   []string `json:"options"`
	TimeoutMs int      `json:"timeout_ms"`
}

type OpponentAnsweredMsg struct {
	Round int `json:"round"`
}

type RoundResultMsg struct {
	Round        int  `json:"round"`
	YourChoice   int  `json:"your_choice"`
	YourCorrect  bool `json:"your_correct"`
	YourPoints   int  `json:"your_points"`
	TheirChoice  int  `json:"their_choice"`
	TheirCorrect bool `json:"their_correct"`
	TheirPoints  int  `json:"their_points"`
	YourTotal    int  `json:"your_total"`
	TheirTotal   int  `json:"their_total"`
	CorrectIndex int  `json:"correct_index"`
	YourTimeMs   int  `json:"your_time_ms"`
	TheirTimeMs  int  `json:"their_time_ms"`
}

type GameEndMsg struct {
	YourScore    int    `json:"your_score"`
	TheirScore   int    `json:"their_score"`
	Result       string `json:"result"`
	RatingChange int    `json:"rating_change"`
}

type FriendPresenceMsg struct {
	UserID uuid.UUID `json:"user_id"`
	Status string    `json:"status"`
}

type PingMsg struct {
	PingID string `json:"ping_id"`
}

type ErrorMsg struct {
	Message string `json:"message"`
}

func NewEnvelope(msgType string, payload any) ([]byte, error) {
	p, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Envelope{Type: msgType, Payload: p})
}
