# Skill: Performance Audit (Go Server)

## Purpose

Profile and optimize the latency-critical paths in the Go server. Rapid is a real-time game where milliseconds matter for perceived responsiveness and scoring fairness. This skill focuses on the hot paths that directly affect player experience.

## When to use

- After implementing the game session or making changes to the round loop
- After changing WebSocket message handling
- After changing database queries in the game path
- Before deployment to production
- When players report the game feeling "laggy"

## Critical paths (ranked by impact)

### 1. Question delivery (server → both players)
**Budget: < 1ms server-side**

The time between the server deciding to send a question and both WebSocket writes completing. This includes JSON serialization of the question + options (per-player, shuffled independently), and two WebSocket frame writes.

Check:
- [ ] JSON marshaling is not creating unnecessary allocations. Pre-allocate buffers if serializing frequently.
- [ ] Option shuffling uses `rand.Shuffle` (O(n), n=4, effectively free) not a sort-based shuffle.
- [ ] Both player writes happen back-to-back in the same goroutine. There must be no blocking operation between write to player 1 and write to player 2.
- [ ] No database query happens between deciding to send and actually sending.
- [ ] No logging with string formatting (use structured slog fields, which are lazy).

### 2. Answer processing (receive → score → store)
**Budget: < 2ms server-side**

From receiving an answer WebSocket frame to having the score computed and ready to broadcast.

Check:
- [ ] JSON deserialization of the answer message is tight — known small payload, no reflection-heavy parsing.
- [ ] Shuffle reverse mapping (`reverseMap`) is a simple loop over 4 elements — O(1) effectively.
- [ ] RTT lookup is a read-locked map access (RWMutex.RLock), not a full lock.
- [ ] `FairTimeMs` computation is pure arithmetic — no system calls, no allocations.
- [ ] `computePoints` is pure arithmetic.
- [ ] No database write happens here — results are held in memory until both players have answered.

### 3. Round result broadcast (score → send to both)
**Budget: < 1ms server-side**

After both answers are in and scores computed, serialize and send results to both players.

Check:
- [ ] Result messages are serialized per-player (different `correct_index` and `their_choice` mappings) — verify this serialization doesn't allocate unnecessarily.
- [ ] Both writes are back-to-back, same as question delivery.
- [ ] The suspense pause (800ms) is implemented client-side, not server-side. The server should NOT sleep between computing the result and sending it.

### 4. Pong processing (receive → update RTT)
**Budget: < 0.5ms**

Pong messages must be processed with minimal latency because any delay in processing the pong inflates the RTT measurement, which directly affects scoring fairness.

Check:
- [ ] Pong handling is the FIRST check in the message routing switch — before any other message type.
- [ ] Pong handling does NOT go through the game session's message channel. It's handled directly in the client's read loop.
- [ ] The EWMA computation is pure arithmetic — no allocations, no locks beyond the client's RTT mutex.
- [ ] The pending ping map lookup and deletion is fast (small map, typically 1-2 entries).

### 5. Question loading (pre-game)
**Budget: < 50ms (not in the hot path, but affects game start time)**

Loading 10 random questions from Postgres when a game starts.

Check:
- [ ] The query uses `ORDER BY RANDOM() LIMIT 10` or equivalent. On small tables (200-300 rows) this is fine. On larger tables, use a more efficient random sampling strategy.
- [ ] Questions are loaded ONCE at game start and held in the session's memory — not queried per-round.
- [ ] The question loading happens BEFORE sending `game_start` to players, so there's no delay between acceptance and the first question.

## Profiling tools

### Go built-in profiler (pprof)
Add pprof endpoints to the server for on-demand profiling:

```go
import _ "net/http/pprof"
// In main.go, this registers handlers on DefaultServeMux
// Access at http://localhost:8080/debug/pprof/
```

Useful profiles:
- `goroutine` — verify no goroutine leaks (should be ~stable during gameplay)
- `heap` — check for memory growth during long game sessions
- `cpu` — 30-second CPU profile during a game to find hot spots
- `trace` — execution trace showing goroutine scheduling

### Benchmarks

Write Go benchmarks for hot-path functions:

```go
func BenchmarkComputePoints(b *testing.B) {
    for i := 0; i < b.N; i++ {
        computePoints(2500)
    }
}

func BenchmarkFairTimeMs(b *testing.B) {
    client := &Client{rttAvg: 150 * time.Millisecond}
    elapsed := 3 * time.Second
    for i := 0; i < b.N; i++ {
        client.FairTimeMs(elapsed)
    }
}

func BenchmarkShuffleAndBuildQuestion(b *testing.B) {
    q := Question{
        Text: "What is the capital?",
        Options: []string{"London", "Paris", "Berlin", "Madrid"},
        CorrectIndex: 1,
    }
    for i := 0; i < b.N; i++ {
        shuffle := randomShuffle(4)
        buildQuestionPayload(q, shuffle, 1)
    }
}

func BenchmarkMessageSerialization(b *testing.B) {
    payload := RoundResultMsg{
        Round: 3, YourChoice: 1, YourCorrect: true, YourPoints: 850,
        TheirChoice: 2, TheirCorrect: false, TheirPoints: 0,
        YourTotal: 2400, TheirTotal: 1800, CorrectIndex: 1,
    }
    for i := 0; i < b.N; i++ {
        json.Marshal(payload)
    }
}
```

Run: `go test -bench=. -benchmem ./internal/game/`

The `-benchmem` flag shows allocations per operation — critical for the hot path.

### OpenTelemetry trace analysis

After running a test game, examine traces in Grafana Tempo:
- Find the `game.round` parent span
- Check child span durations: `send_question`, `await_answers`, `score`, `broadcast_result`
- `send_question` and `broadcast_result` should be < 1ms
- `score` should be < 1ms
- `await_answers` will be 1-15 seconds (human think time) — this is expected

### Metrics to watch

In Grafana:
- `rapid_fair_time_ms` histogram — should be a clean bell curve centered around 2-5 seconds. If there's a spike near 0 or outliers above 15s, something's wrong with RTT compensation.
- `rapid_rtt_ms` histogram — should show stable RTT per connection. Jitter (wide distribution) suggests network issues, not server issues.
- `rapid_answer_latency_raw_ms` vs `rapid_fair_time_ms` — the gap between these is the RTT being stripped. If they're too similar, compensation isn't working.
- Go runtime metrics: goroutine count should be stable during gameplay (2 per client + 1 per game + base), heap should not grow unboundedly.

## Red flags

- Any allocation in `computePoints`, `FairTimeMs`, or `reverseMap` — these should be zero-alloc.
- A mutex lock in the game session's round loop (session state is single-goroutine, no locks needed).
- A database query inside the round loop (all data should be pre-loaded).
- JSON serialization using reflection (the standard `encoding/json` uses reflection, which is fine for Rapid's scale, but if benchmarks show it's a bottleneck, consider `json-iterator` or code-generated serialization).
- WebSocket writes that are not sequential (writing to both players concurrently via goroutines introduces scheduling uncertainty — sequential writes in one goroutine are more deterministic).
- Logging with `fmt.Sprintf` in the hot path (use `slog.Info("msg", "key", value)` which is lazy).
