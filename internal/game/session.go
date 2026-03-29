package game

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/rapidtrivia/rapid-server/internal/ws"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	totalRounds    = 10
	roundTimeoutMs = 15000
	roundTimeout   = 15 * time.Second
)

var (
	sessionTracer = otel.Tracer("game.session")
	sessionMeter  = otel.Meter("game.session")

	gamesActive          metric.Int64UpDownCounter
	gamesTotal           metric.Int64Counter
	roundsTotal          metric.Int64Counter
	fairTimeHist         metric.Float64Histogram
	rawLatencyHist       metric.Float64Histogram
	scoreDistHist        metric.Float64Histogram
	timeoutRoundsTotal   metric.Int64Counter
)

func init() {
	var err error
	gamesActive, err = sessionMeter.Int64UpDownCounter("rapid_games_active")
	if err != nil {
		panic(err)
	}
	gamesTotal, err = sessionMeter.Int64Counter("rapid_games_total")
	if err != nil {
		panic(err)
	}
	roundsTotal, err = sessionMeter.Int64Counter("rapid_rounds_total")
	if err != nil {
		panic(err)
	}
	fairTimeHist, err = sessionMeter.Float64Histogram("rapid_fair_time_ms")
	if err != nil {
		panic(err)
	}
	rawLatencyHist, err = sessionMeter.Float64Histogram("rapid_answer_latency_raw_ms")
	if err != nil {
		panic(err)
	}
	scoreDistHist, err = sessionMeter.Float64Histogram("rapid_score_distribution")
	if err != nil {
		panic(err)
	}
	timeoutRoundsTotal, err = sessionMeter.Int64Counter("rapid_timeout_rounds_total")
	if err != nil {
		panic(err)
	}
}

type playerAnswer struct {
	choice    int
	arriveAt  time.Time
	timedOut  bool
}

type PresenceNotifier interface {
	SetInMatch(userID uuid.UUID, inMatch bool)
	BroadcastPresence(ctx context.Context, userID uuid.UUID, status string)
}

type GameSession struct {
	matchID    uuid.UUID
	category   Category
	questions  []Question
	player1    *ws.Client
	player2    *ws.Client
	p1Rating   int
	p2Rating   int
	p1Name     string
	p2Name     string
	p1Avatar   *string
	p2Avatar   *string
	p1AvatarIdx int
	p2AvatarIdx int
	store      *Store
	presence   PresenceNotifier
	logger     *slog.Logger
}

type NewSessionParams struct {
	MatchID     uuid.UUID
	Category    Category
	Questions   []Question
	Player1     *ws.Client
	Player2     *ws.Client
	P1Rating    int
	P2Rating    int
	P1Name      string
	P2Name      string
	P1Avatar    *string
	P2Avatar    *string
	P1AvatarIdx int
	P2AvatarIdx int
	Store       *Store
	Presence    PresenceNotifier
	Logger      *slog.Logger
}

func NewGameSession(p NewSessionParams) *GameSession {
	return &GameSession{
		matchID:     p.MatchID,
		category:    p.Category,
		questions:   p.Questions,
		player1:     p.Player1,
		player2:     p.Player2,
		p1Rating:    p.P1Rating,
		p2Rating:    p.P2Rating,
		p1Name:      p.P1Name,
		p2Name:      p.P2Name,
		p1Avatar:    p.P1Avatar,
		p2Avatar:    p.P2Avatar,
		p1AvatarIdx: p.P1AvatarIdx,
		p2AvatarIdx: p.P2AvatarIdx,
		store:       p.Store,
		presence:    p.Presence,
		logger:      p.Logger.With("match_id", p.MatchID),
	}
}

func (gs *GameSession) Run(ctx context.Context) {
	ctx, span := sessionTracer.Start(ctx, "game.session",
		trace.WithAttributes(
			attribute.String("match_id", gs.matchID.String()),
			attribute.String("category", gs.category.Name)))
	defer span.End()

	gamesActive.Add(ctx, 1)
	defer func() {
		gamesActive.Add(ctx, -1)
		if gs.presence != nil {
			gs.presence.SetInMatch(gs.player1.ID, false)
			gs.presence.SetInMatch(gs.player2.ID, false)
			gs.presence.BroadcastPresence(ctx, gs.player1.ID, "online")
			gs.presence.BroadcastPresence(ctx, gs.player2.ID, "online")
		}
	}()

	if gs.presence != nil {
		gs.presence.SetInMatch(gs.player1.ID, true)
		gs.presence.SetInMatch(gs.player2.ID, true)
		gs.presence.BroadcastPresence(ctx, gs.player1.ID, "in_match")
		gs.presence.BroadcastPresence(ctx, gs.player2.ID, "in_match")
	}

	gs.logger.Info("game session started",
		"p1", gs.player1.ID, "p2", gs.player2.ID,
		"category", gs.category.Name)

	gs.sendGameStart(ctx)

	p1Total, p2Total := 0, 0
	abandoned := false

	for round := 1; round <= totalRounds; round++ {
		q := gs.questions[round-1]
		p1Points, p2Points, done := gs.playRound(ctx, round, q, &p1Total, &p2Total)
		if done {
			abandoned = true
			break
		}
		p1Total += p1Points
		p2Total += p2Points
	}

	if abandoned {
		gamesTotal.Add(ctx, 1, metric.WithAttributes(
			attribute.String("category", gs.category.Slug),
			attribute.String("result", "abandoned")))
		gs.logger.Info("game abandoned", "p1_score", p1Total, "p2_score", p2Total)
		return
	}

	gs.completeGame(ctx, p1Total, p2Total)
}

func (gs *GameSession) sendGameStart(ctx context.Context) {
	p1Msg, _ := ws.NewEnvelope("game_start", ws.GameStartMsg{
		MatchID:                  gs.matchID,
		OpponentName:             gs.p2Name,
		OpponentAvatarURL:        gs.p2Avatar,
		OpponentDefaultAvatarIdx: gs.p2AvatarIdx,
		CategoryName:             gs.category.Name,
		TotalRounds:              totalRounds,
	})
	p2Msg, _ := ws.NewEnvelope("game_start", ws.GameStartMsg{
		MatchID:                  gs.matchID,
		OpponentName:             gs.p1Name,
		OpponentAvatarURL:        gs.p1Avatar,
		OpponentDefaultAvatarIdx: gs.p1AvatarIdx,
		CategoryName:             gs.category.Name,
		TotalRounds:              totalRounds,
	})

	gs.send(gs.player1, p1Msg)
	gs.send(gs.player2, p2Msg)
}

func (gs *GameSession) playRound(ctx context.Context, round int, q Question, p1Total, p2Total *int) (p1Points, p2Points int, done bool) {
	ctx, roundSpan := sessionTracer.Start(ctx, "game.round",
		trace.WithAttributes(
			attribute.String("match_id", gs.matchID.String()),
			attribute.Int("round", round)))
	defer roundSpan.End()

	p1Shuffled, p1Map := ShuffleOptions(q.Options)
	p2Shuffled, p2Map := ShuffleOptions(q.Options)

	_, sendSpan := sessionTracer.Start(ctx, "game.round.send_question")
	p1QMsg, _ := ws.NewEnvelope("question", ws.QuestionMsg{
		Round:     round,
		Text:      q.QuestionText,
		Options:   p1Shuffled,
		TimeoutMs: roundTimeoutMs,
	})
	p2QMsg, _ := ws.NewEnvelope("question", ws.QuestionMsg{
		Round:     round,
		Text:      q.QuestionText,
		Options:   p2Shuffled,
		TimeoutMs: roundTimeoutMs,
	})
	gs.send(gs.player1, p1QMsg)
	gs.send(gs.player2, p2QMsg)
	questionSendTime := time.Now()
	sendSpan.End()

	_, awaitSpan := sessionTracer.Start(ctx, "game.round.await_answers")
	p1Ans, p2Ans := gs.awaitAnswers(ctx, round)
	awaitSpan.End()

	if p1Ans == nil && p2Ans == nil {
		// Both disconnected or context cancelled
		return 0, 0, true
	}

	_, scoreSpan := sessionTracer.Start(ctx, "game.round.score")

	type scored struct {
		choice    *int
		correct   *bool
		fairTimeMs *int
		points    int
	}

	scorePlayer := func(ans *playerAnswer, shuffleMap map[int]int, rtt time.Duration) scored {
		if ans == nil || ans.timedOut {
			timeoutRoundsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("category", gs.category.Slug)))
			f := false
			return scored{correct: &f}
		}

		rawTime := ans.arriveAt.Sub(questionSendTime)
		fairTime := ComputeFairTime(rawTime, rtt)
		canonicalChoice := MapChoiceToCanonical(ans.choice, shuffleMap)
		correct := canonicalChoice == q.CorrectIndex
		points := ComputePoints(fairTime, correct)

		rawMs := float64(rawTime.Milliseconds())
		fairMs := float64(fairTime.Milliseconds())
		rawLatencyHist.Record(ctx, rawMs)
		fairTimeHist.Record(ctx, fairMs)
		if correct && points > 0 {
			scoreDistHist.Record(ctx, float64(points))
		}
		roundsTotal.Add(ctx, 1, metric.WithAttributes(
			attribute.String("category", gs.category.Slug),
			attribute.Bool("correct", correct)))

		fairMsInt := int(fairTime.Milliseconds())
		return scored{choice: &ans.choice, correct: &correct, fairTimeMs: &fairMsInt, points: points}
	}

	p1Scored := scorePlayer(p1Ans, p1Map, gs.player1.GetRTT())
	p2Scored := scorePlayer(p2Ans, p2Map, gs.player2.GetRTT())
	scoreSpan.End()

	p1Points = p1Scored.points
	p2Points = p2Scored.points
	newP1Total := *p1Total + p1Points
	newP2Total := *p2Total + p2Points

	// Map opponent's canonical choice into the receiving player's shuffle order
	p1TheirChoice := -1
	p2TheirChoice := -1
	if p2Scored.choice != nil {
		p2Canonical := MapChoiceToCanonical(*p2Scored.choice, p2Map)
		p1TheirChoice = MapCanonicalToShuffled(p2Canonical, p1Map)
	}
	if p1Scored.choice != nil {
		p1Canonical := MapChoiceToCanonical(*p1Scored.choice, p1Map)
		p2TheirChoice = MapCanonicalToShuffled(p1Canonical, p2Map)
	}

	p1CorrectIdx := MapCanonicalToShuffled(q.CorrectIndex, p1Map)
	p2CorrectIdx := MapCanonicalToShuffled(q.CorrectIndex, p2Map)

	p1FairMs := 0
	if p1Scored.fairTimeMs != nil {
		p1FairMs = *p1Scored.fairTimeMs
	}
	p2FairMs := 0
	if p2Scored.fairTimeMs != nil {
		p2FairMs = *p2Scored.fairTimeMs
	}
	p1Correct := p1Scored.correct != nil && *p1Scored.correct
	p2Correct := p2Scored.correct != nil && *p2Scored.correct

	_, broadcastSpan := sessionTracer.Start(ctx, "game.round.broadcast_result")
	p1Result, _ := ws.NewEnvelope("round_result", ws.RoundResultMsg{
		Round:        round,
		YourChoice:   derefOr(p1Scored.choice, -1),
		YourCorrect:  p1Correct,
		YourPoints:   p1Points,
		TheirChoice:  p1TheirChoice,
		TheirCorrect: p2Correct,
		TheirPoints:  p2Points,
		YourTotal:    newP1Total,
		TheirTotal:   newP2Total,
		CorrectIndex: p1CorrectIdx,
		YourTimeMs:   p1FairMs,
		TheirTimeMs:  p2FairMs,
	})
	p2Result, _ := ws.NewEnvelope("round_result", ws.RoundResultMsg{
		Round:        round,
		YourChoice:   derefOr(p2Scored.choice, -1),
		YourCorrect:  p2Correct,
		YourPoints:   p2Points,
		TheirChoice:  p2TheirChoice,
		TheirCorrect: p1Correct,
		TheirPoints:  p1Points,
		YourTotal:    newP2Total,
		TheirTotal:   newP1Total,
		CorrectIndex: p2CorrectIdx,
		YourTimeMs:   p2FairMs,
		TheirTimeMs:  p1FairMs,
	})
	gs.send(gs.player1, p1Result)
	gs.send(gs.player2, p2Result)
	broadcastSpan.End()

	_, persistSpan := sessionTracer.Start(ctx, "game.round.persist")
	if err := gs.store.InsertMatchRound(ctx, InsertRoundParams{
		MatchID:     gs.matchID,
		RoundNumber: round,
		QuestionID:  q.ID,
		P1Choice:    p1Scored.choice,
		P1Correct:   p1Scored.correct,
		P1TimeMs:    p1Scored.fairTimeMs,
		P1Points:    p1Points,
		P2Choice:    p2Scored.choice,
		P2Correct:   p2Scored.correct,
		P2TimeMs:    p2Scored.fairTimeMs,
		P2Points:    p2Points,
	}); err != nil {
		gs.logger.Error("persisting round", "error", err, "round", round)
	}
	persistSpan.End()

	return p1Points, p2Points, false
}

func (gs *GameSession) awaitAnswers(ctx context.Context, round int) (*playerAnswer, *playerAnswer) {
	timer := time.NewTimer(roundTimeout)
	defer timer.Stop()

	var p1Ans, p2Ans *playerAnswer

	for p1Ans == nil || p2Ans == nil {
		select {
		case env, ok := <-gs.player1.GameCh:
			if !ok {
				return p1Ans, p2Ans
			}
			if p1Ans != nil {
				continue
			}
			var msg ws.AnswerMsg
			if err := json.Unmarshal(env.Payload, &msg); err != nil || msg.Round != round {
				continue
			}
			p1Ans = &playerAnswer{choice: msg.Choice, arriveAt: time.Now()}
			notifyMsg, _ := ws.NewEnvelope("opponent_answered", ws.OpponentAnsweredMsg{Round: round})
			gs.send(gs.player2, notifyMsg)

		case env, ok := <-gs.player2.GameCh:
			if !ok {
				return p1Ans, p2Ans
			}
			if p2Ans != nil {
				continue
			}
			var msg ws.AnswerMsg
			if err := json.Unmarshal(env.Payload, &msg); err != nil || msg.Round != round {
				continue
			}
			p2Ans = &playerAnswer{choice: msg.Choice, arriveAt: time.Now()}
			notifyMsg, _ := ws.NewEnvelope("opponent_answered", ws.OpponentAnsweredMsg{Round: round})
			gs.send(gs.player1, notifyMsg)

		case <-timer.C:
			if p1Ans == nil {
				p1Ans = &playerAnswer{timedOut: true}
			}
			if p2Ans == nil {
				p2Ans = &playerAnswer{timedOut: true}
			}
			return p1Ans, p2Ans

		case <-ctx.Done():
			return nil, nil
		}
	}
	return p1Ans, p2Ans
}

func (gs *GameSession) completeGame(ctx context.Context, p1Total, p2Total int) {
	var winnerID *uuid.UUID
	var actualScoreP1 float64
	switch {
	case p1Total > p2Total:
		id := gs.player1.ID
		winnerID = &id
		actualScoreP1 = 1.0
	case p2Total > p1Total:
		id := gs.player2.ID
		winnerID = &id
		actualScoreP1 = 0.0
	default:
		actualScoreP1 = 0.5
	}

	_, _, changeP1, changeP2 := ComputeELO(gs.p1Rating, gs.p2Rating, actualScoreP1)
	newP1Rating := gs.p1Rating + changeP1
	newP2Rating := gs.p2Rating + changeP2

	if err := gs.store.CompleteMatchTx(ctx, CompleteMatchTxParams{
		CompleteMatchParams: CompleteMatchParams{
			MatchID:  gs.matchID,
			P1Score:  p1Total,
			P2Score:  p2Total,
			WinnerID: winnerID,
		},
		P1ID:     gs.player1.ID,
		P1Rating: newP1Rating,
		P2ID:     gs.player2.ID,
		P2Rating: newP2Rating,
	}); err != nil {
		gs.logger.Error("completing match", "error", err)
	}

	p1Result := "draw"
	p2Result := "draw"
	if winnerID != nil {
		if *winnerID == gs.player1.ID {
			p1Result = "win"
			p2Result = "loss"
		} else {
			p1Result = "loss"
			p2Result = "win"
		}
	}

	p1End, _ := ws.NewEnvelope("game_end", ws.GameEndMsg{
		YourScore:    p1Total,
		TheirScore:   p2Total,
		Result:       p1Result,
		RatingChange: changeP1,
	})
	p2End, _ := ws.NewEnvelope("game_end", ws.GameEndMsg{
		YourScore:    p2Total,
		TheirScore:   p1Total,
		Result:       p2Result,
		RatingChange: changeP2,
	})
	gs.send(gs.player1, p1End)
	gs.send(gs.player2, p2End)

	gamesTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String("category", gs.category.Slug),
		attribute.String("result", "completed")))

	gs.logger.Info("game completed",
		"p1_score", p1Total, "p2_score", p2Total,
		"winner", winnerID,
		"p1_rating_change", changeP1, "p2_rating_change", changeP2)
}

func (gs *GameSession) send(client *ws.Client, data []byte) {
	select {
	case client.Send <- data:
	default:
		gs.logger.Warn("send channel full", "user_id", client.ID)
	}
}

func derefOr(p *int, def int) int {
	if p == nil {
		return def
	}
	return *p
}
