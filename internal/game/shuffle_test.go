package game

import (
	"testing"
)

func TestShuffleOptions(t *testing.T) {
	options := []string{"A", "B", "C", "D"}

	shuffled, shuffleMap := ShuffleOptions(options)

	if len(shuffled) != 4 {
		t.Fatalf("shuffled length = %d, want 4", len(shuffled))
	}
	if len(shuffleMap) != 4 {
		t.Fatalf("shuffleMap length = %d, want 4", len(shuffleMap))
	}

	// All original options should be present
	seen := map[string]bool{}
	for _, o := range shuffled {
		seen[o] = true
	}
	for _, o := range options {
		if !seen[o] {
			t.Errorf("option %q missing from shuffled result", o)
		}
	}

	// shuffleMap should correctly map shuffled→canonical
	for shuffledIdx, canonicalIdx := range shuffleMap {
		if shuffled[shuffledIdx] != options[canonicalIdx] {
			t.Errorf("shuffleMap[%d]=%d: shuffled[%d]=%q != options[%d]=%q",
				shuffledIdx, canonicalIdx, shuffledIdx, shuffled[shuffledIdx], canonicalIdx, options[canonicalIdx])
		}
	}
}

func TestMapChoiceRoundTrip(t *testing.T) {
	options := []string{"A", "B", "C", "D"}
	_, shuffleMap := ShuffleOptions(options)

	for shuffledIdx := range 4 {
		canonical := MapChoiceToCanonical(shuffledIdx, shuffleMap)
		backToShuffled := MapCanonicalToShuffled(canonical, shuffleMap)
		if backToShuffled != shuffledIdx {
			t.Errorf("round trip failed: shuffled %d → canonical %d → shuffled %d", shuffledIdx, canonical, backToShuffled)
		}
	}
}

func TestShufflesAreIndependent(t *testing.T) {
	options := []string{"A", "B", "C", "D"}
	allSame := true
	for range 100 {
		s1, _ := ShuffleOptions(options)
		s2, _ := ShuffleOptions(options)
		for i := range s1 {
			if s1[i] != s2[i] {
				allSame = false
				break
			}
		}
		if !allSame {
			break
		}
	}
	if allSame {
		t.Error("100 shuffle pairs were all identical — shuffles are not independent")
	}
}
