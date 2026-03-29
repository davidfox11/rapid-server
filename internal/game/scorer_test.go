package game

import (
	"testing"
	"time"
)

func TestComputeFairTime(t *testing.T) {
	tests := []struct {
		name    string
		raw     time.Duration
		rtt     time.Duration
		want    time.Duration
	}{
		{"normal", 500 * time.Millisecond, 100 * time.Millisecond, 400 * time.Millisecond},
		{"zero rtt", 300 * time.Millisecond, 0, 300 * time.Millisecond},
		{"rtt exceeds raw, clamps to zero", 50 * time.Millisecond, 100 * time.Millisecond, 0},
		{"large rtt", 1000 * time.Millisecond, 800 * time.Millisecond, 200 * time.Millisecond},
		{"both zero", 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeFairTime(tt.raw, tt.rtt)
			if got != tt.want {
				t.Errorf("ComputeFairTime(%v, %v) = %v, want %v", tt.raw, tt.rtt, got, tt.want)
			}
		})
	}
}

func TestComputePoints(t *testing.T) {
	tests := []struct {
		name    string
		fair    time.Duration
		correct bool
		want    int
	}{
		{"instant correct", 0, true, 1000},
		{"1s correct", 1 * time.Second, true, 900},
		{"5s correct", 5 * time.Second, true, 500},
		{"9s correct", 9 * time.Second, true, 100},
		{"10s correct, zero points", 10 * time.Second, true, 0},
		{"over limit", 15 * time.Second, true, 0},
		{"incorrect fast", 100 * time.Millisecond, false, 0},
		{"incorrect slow", 5 * time.Second, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputePoints(tt.fair, tt.correct)
			if got != tt.want {
				t.Errorf("ComputePoints(%v, %v) = %d, want %d", tt.fair, tt.correct, got, tt.want)
			}
		})
	}
}

func TestComputeELO(t *testing.T) {
	tests := []struct {
		name      string
		ratingA   int
		ratingB   int
		scoreA    float64
		wantNewA  int
		wantNewB  int
	}{
		{"equal ratings, A wins", 1200, 1200, 1.0, 1216, 1184},
		{"equal ratings, draw", 1200, 1200, 0.5, 1200, 1200},
		{"equal ratings, B wins", 1200, 1200, 0.0, 1184, 1216},
		{"higher rated wins (small change)", 1400, 1200, 1.0, 1408, 1192},
		{"lower rated wins (upset bonus)", 1200, 1400, 1.0, 1224, 1376},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newA, newB, _, _ := ComputeELO(tt.ratingA, tt.ratingB, tt.scoreA)
			if newA != tt.wantNewA {
				t.Errorf("newA = %d, want %d", newA, tt.wantNewA)
			}
			if newB != tt.wantNewB {
				t.Errorf("newB = %d, want %d", newB, tt.wantNewB)
			}
		})
	}
}

func TestComputeELO_symmetric(t *testing.T) {
	_, _, changeA, changeB := ComputeELO(1200, 1200, 1.0)
	if changeA != -changeB {
		t.Errorf("changes not symmetric: changeA=%d, changeB=%d", changeA, changeB)
	}
}

func BenchmarkComputeFairTime(b *testing.B) {
	b.ReportAllocs()
	raw := 500 * time.Millisecond
	rtt := 100 * time.Millisecond
	for i := 0; i < b.N; i++ {
		ComputeFairTime(raw, rtt)
	}
}

func BenchmarkComputePoints(b *testing.B) {
	b.ReportAllocs()
	fair := 2500 * time.Millisecond
	for i := 0; i < b.N; i++ {
		ComputePoints(fair, true)
	}
}

func BenchmarkComputeELO(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ComputeELO(1200, 1300, 1.0)
	}
}
