# Skill: Explain & Learn (Go)

## Purpose

After implementing a change, explain it in a way that helps David learn Go idioms, patterns, and best practices. David is an experienced TypeScript/Node developer learning Go — so draw connections to familiar concepts, highlight what's different, and provide references for deeper learning.

## When to use

After completing an implementation task. Can also be invoked explicitly: "explain this change" or "teach me about this pattern."

## Explanation structure

### 1. What changed (brief)
A 2-3 sentence summary of the functional change.

### 2. Go concepts introduced or used
For each significant Go concept in the change, explain:

**The concept**: What is it, in one paragraph.

**TypeScript equivalent** (or lack thereof): Map it to something David already knows. Examples:
- Go interfaces → TypeScript interfaces, BUT satisfied implicitly
- Go goroutines → somewhat like `Promise.all()` but with real concurrency
- Go channels → no direct equivalent; closest is an EventEmitter that blocks until consumed
- Go `defer` → like `finally` blocks but attached to individual statements
- Go pointer receivers → like methods on a class, but the `this` binding is explicit
- Go error handling → like if every function returned `{ result, error }` and you checked error every time
- Go `select` → like `Promise.race()` but for channels
- Go `context.Context` → like AbortController / AbortSignal in fetch

**Why Go does it this way**: The design philosophy behind the choice. Go is opinionated — explain the opinion.

**Common mistake to avoid**: What a TypeScript developer would instinctively do that's wrong in Go.

### 3. Patterns worth studying
Point to specific patterns used in the code that are worth understanding deeply:
- The "accept interfaces, return structs" pattern
- The "functional options" pattern for configuration
- The "error wrapping" pattern with `%w`
- The "goroutine ownership" pattern (whoever creates it, cleans it up)
- The "fan-in / fan-out" channel pattern
- The "context cancellation" pattern for graceful shutdown
- Table-driven tests

### 4. References
Provide specific resources for deeper learning. Prefer:
- **Effective Go** (https://go.dev/doc/effective_go) — the canonical style guide
- **Go Blog posts** (https://go.dev/blog/) — official deep dives. Particularly relevant:
  - "Go Concurrency Patterns" for goroutines and channels
  - "Context" for understanding context.Context
  - "Error handling and Go" for error philosophy
  - "Share Memory By Communicating" for channel-based design
- **"Learning Go" by Jon Bodner** — reference specific chapters when relevant (David is reading this)
- **"Concurrency in Go" by Katherine Cox-Buday** — reference when goroutine/channel patterns come up
- **Go standard library source code** — Go's stdlib is famously readable. Link to specific files when a pattern comes from there
- **Go Proverbs** (https://go-proverbs.github.io/) — when a proverb applies, cite it

### 5. "If you have 10 minutes" challenge
A small exercise David can try to reinforce the concept:
- "Try rewriting this handler without the interface — notice how testing becomes harder"
- "Add a second consumer to this channel and observe what happens"
- "Remove the context timeout and see how the goroutine behaves on shutdown"

## Tone

Direct and peer-to-peer. David is a senior engineer learning a new language, not a beginner learning to program. Don't explain what a function is — explain why Go's approach to functions is different from what he's used to. Assume competence, highlight surprises.

## Example

After implementing the RTT ping loop:

> **What changed**: Added a background goroutine per client that sends ping messages every 5 seconds and computes an EWMA of round-trip times.
>
> **Go concept: goroutines and `select`**
> The ping loop uses `select` to wait on either a ticker firing or the context being cancelled. In TypeScript, you'd use `setInterval` and clear it on cleanup. In Go, `time.NewTicker` returns a channel that receives a value every N duration, and `select` lets you wait on multiple channels simultaneously — whichever fires first wins.
>
> The key insight: `select` is Go's fundamental multiplexing primitive. The game session's round loop uses the exact same pattern — waiting on player 1's answer, player 2's answer, or a timeout. Once you understand `select`, you understand the core of Go concurrency.
>
> **Common mistake**: Forgetting to call `ticker.Stop()`. Unlike JavaScript's `setInterval` which gets garbage collected, a Go ticker leaks a goroutine if not stopped. Always `defer ticker.Stop()`.
>
> **Reference**: Go Blog — "Go Concurrency Patterns" (https://go.dev/blog/pipelines). Also Chapter 10 of "Learning Go" covers this pattern.
>
> **10-minute challenge**: Add a log line that prints the current EWMA RTT every time it's updated. Connect two clients and observe how the RTT stabilizes over the first few pings.
