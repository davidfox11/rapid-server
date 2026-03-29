# Skill: Test & Self-Iterate (Go Server)

## Purpose

After implementing a feature, validate it works correctly by writing tests, seeding data, making real API calls, inspecting logs and traces, and iterating until the feature is solid. The goal is to catch bugs before David ever sees them.

## When to use

After implementing any feature. The cycle is: implement → test → observe → fix → repeat.

## Testing layers

### 1. Unit tests (always)

Write table-driven tests for all business logic. Go convention is `_test.go` files in the same package.

```go
func TestComputePoints(t *testing.T) {
    tests := []struct {
        name       string
        fairTimeMs int
        want       int
    }{
        {"instant answer", 0, 1000},
        {"one second", 1000, 900},
        {"five seconds", 5000, 500},
        {"ten seconds", 10000, 0},
        {"over ten seconds", 15000, 0},
        {"negative clamps to max", -100, 1000},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := computePoints(tt.fairTimeMs)
            if got != tt.want {
                t.Errorf("computePoints(%d) = %d, want %d", tt.fairTimeMs, got, tt.want)
            }
        })
    }
}
```

Key areas to unit test:
- **Scoring**: `computePoints()`, ELO rating calculation, RTT compensation (`FairTimeMs()`)
- **Shuffle mapping**: `randomShuffle()`, `reverseMap()`, `buildQuestionPayload()` — verify that shuffled options contain the same elements and correct_index maps correctly
- **Protocol serialization**: Envelope marshaling/unmarshaling, message type routing
- **Input validation**: Username format validation, choice bounds checking (0-3)

Run tests with: `go test ./... -v -race`
- The `-race` flag enables the race detector — critical for catching concurrency bugs in the hub and game session.

### 2. Integration tests (for database operations)

Use a test database (Docker Compose includes a test Postgres instance or use the same one with a test schema).

```go
func TestUserStore_Create(t *testing.T) {
    // Setup: connect to test database, run migrations
    pool := setupTestDB(t)
    store := &PostgresStore{db: pool}

    // Test
    user, err := store.Create(context.Background(), CreateParams{
        ID:          uuid.New().String(),
        FirebaseUID: "test-firebase-uid",
        Username:    "testuser",
        DisplayName: "Test User",
    })

    // Assert
    if err != nil { t.Fatalf("unexpected error: %v", err) }
    if user.Username != "testuser" { t.Errorf("got username %q, want %q", user.Username, "testuser") }

    // Verify: read it back
    found, err := store.FindByUsername(context.Background(), "testuser")
    if err != nil { t.Fatalf("find failed: %v", err) }
    if found.ID != user.ID { t.Errorf("found different user") }
}
```

Test database operations:
- User CRUD and uniqueness constraints
- Friendship lifecycle (request → accept → query friends)
- Match creation and completion with round data
- Question loading by category with randomization
- Rating updates (verify atomicity — both players' ratings change in same transaction)

### 3. WebSocket integration tests (for the game loop)

Create a test harness that connects two WebSocket clients and plays a full game:

```go
func TestFullGameFlow(t *testing.T) {
    // Start test server
    srv := setupTestServer(t)
    defer srv.Close()

    // Connect two clients
    c1 := connectWSClient(t, srv.URL, "player1", "Player One")
    c2 := connectWSClient(t, srv.URL, "player2", "Player Two")
    defer c1.Close()
    defer c2.Close()

    // Player 1 challenges Player 2
    c1.Send("challenge", ChallengeMsg{OpponentID: "player2", CategoryID: "general-knowledge"})

    // Player 2 receives and accepts
    msg := c2.Receive()
    assert(t, msg.Type == "challenge_recv", "expected challenge_recv, got %s", msg.Type)
    c2.Send("challenge_resp", ChallengeRespMsg{MatchID: msg.MatchID, Accepted: true})

    // Both receive game_start
    gs1 := c1.Receive()
    gs2 := c2.Receive()
    assert(t, gs1.Type == "game_start", "p1 expected game_start")
    assert(t, gs2.Type == "game_start", "p2 expected game_start")

    // Play 10 rounds
    for round := 1; round <= 10; round++ {
        q1 := c1.Receive() // question
        q2 := c2.Receive() // question
        assert(t, q1.Type == "question", "expected question round %d", round)

        // Both answer (choice 0 for simplicity)
        c1.Send("answer", AnswerMsg{Round: round, Choice: 0})
        c2.Send("answer", AnswerMsg{Round: round, Choice: 1})

        // Both receive round_result
        r1 := c1.Receive()
        r2 := c2.Receive()
        assert(t, r1.Type == "round_result", "expected round_result")

        // Verify per-player perspective is correct
        // r1.your_choice should be 0, r1.their_choice should be mapped to p1's shuffle
    }

    // Both receive game_end
    e1 := c1.Receive()
    e2 := c2.Receive()
    assert(t, e1.Type == "game_end", "expected game_end")

    // Verify: if p1 won, p2 should have lost
    if e1.Result == "win" { assert(t, e2.Result == "loss", "p2 should have lost") }
}
```

### 4. API endpoint tests (HTTP)

Use `net/http/httptest` to test REST handlers:

```go
func TestRegisterHandler(t *testing.T) {
    handler := setupTestHandler(t)
    body := `{"username": "testuser", "display_name": "Test"}`
    req := httptest.NewRequest("POST", "/auth/register", strings.NewReader(body))
    req.Header.Set("Authorization", "Bearer test-token")
    w := httptest.NewRecorder()

    handler.ServeHTTP(w, req)

    if w.Code != http.StatusCreated {
        t.Errorf("got status %d, want %d. Body: %s", w.Code, http.StatusCreated, w.Body.String())
    }
}
```

### 5. Manual smoke testing (live server)

After automated tests pass, verify against the running Docker Compose environment:

**REST endpoint testing with curl:**
```bash
# Health check
curl http://localhost:8080/health

# Register a user (with a mock/dev Firebase token)
curl -X POST http://localhost:8080/auth/register \
  -H "Authorization: Bearer dev-token-player1" \
  -H "Content-Type: application/json" \
  -d '{"username": "player1", "display_name": "Player One"}'

# Get profile with stats
curl http://localhost:8080/auth/me -H "Authorization: Bearer dev-token-player1"

# List categories
curl http://localhost:8080/categories -H "Authorization: Bearer dev-token-player1"

# Search friends
curl "http://localhost:8080/friends/search?q=player" -H "Authorization: Bearer dev-token-player1"
```

**WebSocket testing with websocat:**
```bash
# Connect as player 1
websocat ws://localhost:8080/ws?user_id=player1&username=Player1

# In another terminal, connect as player 2
websocat ws://localhost:8080/ws?user_id=player2&username=Player2

# Send a challenge (type this in player 1's terminal)
{"type":"challenge","payload":{"opponent_id":"player2","category_id":"general-knowledge"}}

# Accept in player 2's terminal
{"type":"challenge_resp","payload":{"match_id":"<from challenge_recv>","challenger_id":"player1","category_id":"general-knowledge","accepted":true}}
```

**Observability verification:**
After running a test game:
- Open Grafana at `http://localhost:3000`
- Check Prometheus metrics: verify `rapid_games_total` incremented, `rapid_rtt_ms` has data
- Check Tempo traces: search for a trace covering the full game session, verify child spans for each round
- Check Loki logs: filter by `match_id` and verify the complete game lifecycle is logged

### 6. Self-iteration loop

When a test fails:
1. Read the error message carefully
2. Check if it's a test bug or an implementation bug
3. If implementation: fix the code, re-run the specific failing test
4. If test: fix the test, re-run
5. After fixing, run the full test suite (`go test ./... -race`) to check for regressions
6. Check the logs (`docker compose logs server`) for any warnings or errors
7. Repeat until green

## Test data seeding

For local development testing, ensure the seed data includes:
- At least 2 test users with known credentials/tokens
- A friendship between them (already accepted)
- At least 15 questions per category (enough for one full game + variation)
- Known-answer questions (so you can predict correct/incorrect in tests)

## Key invariants to verify

These must ALWAYS hold true:
- Both players in a match receive the same question text each round
- Option shuffles are independent per player (same question, different option order)
- `fair_time = server_elapsed - RTT` is never negative (clamp to 0)
- Points are only awarded for correct answers
- Final scores in `game_end` match the sum of round points
- The winner in `game_end` matches who has more total points
- Both players' data is persisted in `match_rounds` after game completion
- ELO rating changes are symmetric (winner gains roughly what loser loses)
- `their_choice` in `round_result` is correctly mapped to the receiving player's shuffle
