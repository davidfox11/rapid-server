package game

import "math/rand/v2"

func ShuffleOptions(options []string) ([]string, map[int]int) {
	n := len(options)
	perm := rand.Perm(n)
	shuffled := make([]string, n)
	shuffleMap := make(map[int]int, n)
	for shuffledIdx, canonicalIdx := range perm {
		shuffled[shuffledIdx] = options[canonicalIdx]
		shuffleMap[shuffledIdx] = canonicalIdx
	}
	return shuffled, shuffleMap
}

func MapChoiceToCanonical(shuffledChoice int, shuffleMap map[int]int) int {
	return shuffleMap[shuffledChoice]
}

func MapCanonicalToShuffled(canonicalIndex int, shuffleMap map[int]int) int {
	for shuffled, canonical := range shuffleMap {
		if canonical == canonicalIndex {
			return shuffled
		}
	}
	return -1
}
