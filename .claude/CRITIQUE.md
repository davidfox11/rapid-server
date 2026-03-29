# Skill: Conceptual Critique (Go Server)

## Purpose

Before implementing a new feature or making a significant change, step back and evaluate it at a higher level. Think about how it fits into the existing architecture, whether the approach is idiomatic Go, whether it introduces unnecessary complexity, and whether there's a simpler way.

## When to use

Invoke this skill BEFORE writing implementation code when:
- Adding a new package or significant new functionality
- Changing the WebSocket protocol or game loop
- Adding a new database table or modifying the schema
- Introducing a new dependency
- Refactoring existing code

## Process

### 1. Understand the intent
- What problem is this solving?
- Who benefits (the player, the developer, the system)?
- Is this an MVP requirement or scope creep?

### 2. Evaluate architectural fit
- Does this belong in the package where it's being added, or does it cross boundaries?
- Does it follow the "package by feature" convention? Types, handlers, and stores for a feature should live together.
- Does it introduce circular dependencies? Go forbids these — check import paths.
- Does it respect the goroutine-per-game-session model? Game state should be local to the session goroutine, not shared.
- Is the data ephemeral (Redis/in-memory) or persistent (Postgres)? Don't put ephemeral game state in Postgres or persistent user data in Redis.

### 3. Evaluate Go idioms
- Are errors handled explicitly? Every error should be checked, not discarded.
- Are interfaces small and defined by the consumer? A 10-method interface is a red flag.
- Is there unnecessary abstraction? In Go, a little copying is better than a little dependency. Don't create a `utils` package.
- Are goroutines cleaned up? Every `go func()` needs a clear exit path, usually via context cancellation.
- Is `context.Context` threaded through from the entry point? It should never be created mid-call-chain except in main.go or tests.

### 4. Evaluate simplicity
- Could this be done with fewer moving parts?
- Is there a standard library solution before reaching for a third-party package?
- Would a future developer (or David in 3 months) understand this without extensive comments?
- Are there fewer than 3 levels of indirection between the HTTP handler and the database query?

### 5. Evaluate observability impact
- Will this new code path be visible in traces? Add spans for any operation that takes measurable time.
- Are the right metrics being updated? New game states, new error conditions, new latency-sensitive paths all need instrumentation.
- Do log messages include enough context (match_id, user_id, round) to debug issues in production?

### 6. Check for Rapid-specific concerns
- Does this affect the real-time game loop latency? The hot path (question → answer → score → broadcast) must stay sub-millisecond on the server side.
- Does this affect RTT compensation accuracy? Anything that delays pong processing or changes timing assumptions is critical.
- Does this change the WebSocket protocol? If so, the Flutter app MUST be updated in lockstep. Flag this explicitly.
- Does this affect fairness between players? Both players must always have an equivalent experience regardless of their network latency.

## Output format

Provide your critique as:
1. **Assessment**: 1-2 sentences on whether the approach is sound
2. **Concerns**: Specific issues ranked by severity
3. **Suggestions**: Alternative approaches if the current one has problems
4. **Protocol impact**: Whether the Flutter app needs changes (YES/NO + details)
5. **Verdict**: PROCEED / REVISE / RETHINK
