package game

import (
	"math"
	"time"
)

const eloK = 32

func ComputeFairTime(rawTime, playerRTT time.Duration) time.Duration {
	fair := rawTime - playerRTT
	if fair < 0 {
		return 0
	}
	return fair
}

func ComputePoints(fairTime time.Duration, correct bool) int {
	if !correct {
		return 0
	}
	ms := int(fairTime.Milliseconds())
	points := 1000 - ms/10
	if points < 0 {
		return 0
	}
	return points
}

func ComputeELO(ratingA, ratingB int, actualScoreA float64) (newA, newB, changeA, changeB int) {
	expectedA := 1.0 / (1.0 + math.Pow(10, float64(ratingB-ratingA)/400.0))
	deltaA := eloK * (actualScoreA - expectedA)
	deltaB := eloK * ((1.0 - actualScoreA) - (1.0 - expectedA))

	changeA = int(math.Round(deltaA))
	changeB = int(math.Round(deltaB))
	newA = ratingA + changeA
	newB = ratingB + changeB
	return
}
