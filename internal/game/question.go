package game

import (
	"time"

	"github.com/google/uuid"
)

type Category struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	Description   string    `json:"description"`
	QuestionCount int       `json:"question_count"`
}

type Question struct {
	ID           uuid.UUID
	CategoryID   uuid.UUID
	QuestionText string
	Options      []string
	CorrectIndex int
	Difficulty   int
}

type Match struct {
	ID           uuid.UUID  `json:"id"`
	CategoryID   uuid.UUID  `json:"category_id"`
	Player1ID    uuid.UUID  `json:"player1_id"`
	Player2ID    uuid.UUID  `json:"player2_id"`
	Player1Score int        `json:"player1_score"`
	Player2Score int        `json:"player2_score"`
	WinnerID     *uuid.UUID `json:"winner_id"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at"`
}

type MatchSummary struct {
	Match
	CategoryName    string `json:"category_name"`
	Player1Username string `json:"player1_username"`
	Player2Username string `json:"player2_username"`
}

type MatchDetail struct {
	Match
	Rounds []RoundDetail `json:"rounds"`
}

type RoundDetail struct {
	RoundNumber  int      `json:"round_number"`
	QuestionText string   `json:"question_text"`
	Options      []string `json:"options"`
	CorrectIndex int      `json:"correct_index"`
	P1Choice     *int     `json:"p1_choice"`
	P1Correct    *bool    `json:"p1_correct"`
	P1TimeMs     *int     `json:"p1_time_ms"`
	P1Points     int      `json:"p1_points"`
	P2Choice     *int     `json:"p2_choice"`
	P2Correct    *bool    `json:"p2_correct"`
	P2TimeMs     *int     `json:"p2_time_ms"`
	P2Points     int      `json:"p2_points"`
}

type InsertRoundParams struct {
	MatchID     uuid.UUID
	RoundNumber int
	QuestionID  uuid.UUID
	P1Choice    *int
	P1Correct   *bool
	P1TimeMs    *int
	P1Points    int
	P2Choice    *int
	P2Correct   *bool
	P2TimeMs    *int
	P2Points    int
}

type CompleteMatchParams struct {
	MatchID  uuid.UUID
	P1Score  int
	P2Score  int
	WinnerID *uuid.UUID
}

type CompleteMatchTxParams struct {
	CompleteMatchParams
	P1ID     uuid.UUID
	P1Rating int
	P2ID     uuid.UUID
	P2Rating int
}
