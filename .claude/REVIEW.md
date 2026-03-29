# Skill: Code Review (Go Server)

## Purpose

Review code changes for bugs, regressions, inconsistencies, security issues, and deviation from project conventions. Act as a senior Go developer reviewing a pull request.

## When to use

After implementing a feature or making changes, before considering the work complete.

## Review checklist

### Error handling
- [ ] Every `error` return is checked. No `_` on error values unless explicitly justified.
- [ ] Errors are wrapped with context: `fmt.Errorf("loading questions for category %s: %w", categoryID, err)` — not just `return err`.
- [ ] Error messages are lowercase and don't start with a capital letter (Go convention).
- [ ] HTTP handlers return appropriate status codes: 400 for bad input, 401 for auth failures, 404 for not found, 500 for server errors. Never return 200 with an error body.
- [ ] WebSocket errors trigger appropriate cleanup (client unregistration, game session termination).

### Concurrency safety
- [ ] No shared mutable state accessed from multiple goroutines without synchronization.
- [ ] Game session state is ONLY accessed from the session goroutine. If you need a mutex in game.go, something is wrong.
- [ ] The Hub's client map is protected by a mutex for register/unregister/lookup operations.
- [ ] Redis operations are atomic where needed (use transactions for read-modify-write).
- [ ] Channel operations won't block indefinitely. Every channel read has a `select` with a timeout or context cancellation case.
- [ ] No goroutine leaks. Every `go func()` has a clear termination path. Check: if the parent context is cancelled, does the goroutine exit?

### Database
- [ ] SQL queries use parameterized arguments (`$1`, `$2`), never string concatenation.
- [ ] Transactions are used when multiple writes must be atomic (e.g., saving match result + updating ratings).
- [ ] `sqlc generate` has been run after any changes to `.sql` files or migrations.
- [ ] Migrations are backwards-compatible. A new column should have a DEFAULT or be nullable.
- [ ] Indexes exist on frequently queried columns (foreign keys, username lookups, match history queries).

### WebSocket protocol
- [ ] Message types match the protocol spec in the project README / prompt document.
- [ ] All WebSocket writes use the client's `Send()` method (not raw `conn.Write()`).
- [ ] The game session sends messages to BOTH players where required (round_result, game_end).
- [ ] Player-specific data is correct per-player. Check: is player 1 seeing player 2's score labelled as "yours"? Shuffle maps are per-player?
- [ ] Option shuffle mapping is correct: the `correct_index` and `their_choice` in round_result are mapped to the RECEIVING player's shuffle order.
- [ ] Timeout handling: if a player doesn't answer within 15s, they get 0 points and the round proceeds.
- [ ] Disconnection handling: if a player disconnects mid-game, remaining rounds should complete (opponent gets default wins) and the match should be saved.

### RTT compensation
- [ ] RTT is measured from the server's perspective (send ping, receive pong, compute delta).
- [ ] The EWMA alpha value is 0.3. Changing this affects scoring fairness.
- [ ] `fair_time = server_elapsed - RTT`. If fair_time is negative (shouldn't happen but could with jitter), clamp to 0.
- [ ] Points formula: `max(0, 1000 - (fair_time_ms / 10))`. Only for correct answers.
- [ ] RTT values are logged per round for debugging.

### Observability
- [ ] New code paths have trace spans with meaningful names: `package.operation` format.
- [ ] Spans include relevant attributes (user_id, match_id, category, round number).
- [ ] Errors are recorded on spans: `span.RecordError(err)` + `span.SetStatus(codes.Error, "description")`.
- [ ] Custom metrics are updated: connection counts, game counts, latency histograms.
- [ ] Log messages use structured fields, not string formatting: `slog.Info("msg", "key", value)` not `slog.Info(fmt.Sprintf(...))`.
- [ ] Log levels are appropriate: INFO for business events, WARN for recoverable issues, ERROR for failures.

### Security
- [ ] Firebase JWT validation on all REST endpoints (except health check).
- [ ] WebSocket connections are authenticated before being registered in the hub.
- [ ] Users can only access their own data. A user can't query another user's match history by guessing IDs.
- [ ] Friend operations validate the relationship: you can't challenge someone who isn't your friend.
- [ ] Input validation: username format, category existence, choice index bounds (0-3).

### Style and conventions
- [ ] No global variables. Dependencies are passed through constructors.
- [ ] Interfaces are defined by the consumer package, not the provider.
- [ ] Exported functions and types have doc comments.
- [ ] File naming: lowercase, underscores for multi-word (`game_session.go` not `gameSession.go`).
- [ ] Package names are short, lowercase, single-word where possible.
- [ ] No `init()` functions unless absolutely necessary.
- [ ] `context.Context` is the first parameter of functions that need it.

## Output format

For each issue found:
```
[SEVERITY] file.go:concept — Description
  Suggestion: How to fix it
```

Severity levels:
- **CRITICAL**: Will cause bugs, data loss, security issues, or scoring unfairness
- **WARNING**: Code smell, potential future issue, or deviation from conventions
- **NITPICK**: Style preference or minor improvement
