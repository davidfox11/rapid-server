# Skill: Protocol Sync (Shared — Go + Flutter)

## Purpose

The Go server and Flutter app communicate via a shared protocol (REST endpoints + WebSocket messages). When one side changes, the other MUST be updated. This skill detects protocol drift between the two repos.

## When to use

- After changing any WebSocket message type, payload, or field name in Go
- After changing any REST endpoint, request body, or response shape in Go
- After changing any Dart model class, WebSocket message handler, or API client method in Flutter
- Before any deployment or release
- Periodically as a health check

## Protocol contract locations

### Go server (source of truth for the protocol)
- `internal/ws/protocol.go` — all WebSocket message types and payload structs
- `internal/user/handler.go` — user REST endpoint request/response shapes
- `internal/friend/handler.go` — friend REST endpoint request/response shapes
- `internal/game/store.go` — match/round query return shapes
- `migrations/*.sql` — database schema (affects what fields are available)

### Flutter app (must mirror the server)
- `lib/models/ws_messages.dart` — all WebSocket message types (must match protocol.go)
- `lib/models/user.dart` — User model (must match /auth/me response)
- `lib/models/friend.dart` — Friend model (must match /friends response)
- `lib/models/match.dart` — Match/MatchRound models (must match /matches/:id response)
- `lib/models/category.dart` — Category model (must match /categories response)
- `lib/services/api_client.dart` — endpoint paths and request bodies
- `lib/services/ws_client.dart` — message type routing

## Sync checklist

### WebSocket messages

For EACH message type, verify:

| Field | Go (protocol.go) | Dart (ws_messages.dart) | Match? |
|-------|-------------------|-------------------------|--------|
| Type string | Must be identical | Must be identical | |
| Payload field names | JSON tags on struct | fromJson key names | |
| Payload field types | Go type | Dart type | |
| Optional fields | pointer or omitempty | nullable (?) | |

Specific messages to check:
- [ ] `challenge` — opponent_id, category_id
- [ ] `challenge_resp` — match_id, challenger_id, category_id, accepted
- [ ] `answer` — round, choice
- [ ] `pong` — ping_id
- [ ] `challenge_recv` — match_id, challenger_id, challenger_name, challenger_avatar_url, challenger_default_avatar_index, category_id, category_name, h2h_you, h2h_them
- [ ] `game_start` — match_id, opponent_name, opponent_avatar_url, opponent_default_avatar_index, category_name, total_rounds
- [ ] `question` — round, text, options, timeout_ms
- [ ] `opponent_answered` — round
- [ ] `round_result` — round, your_choice, your_correct, your_points, their_choice, their_correct, their_points, your_total, their_total, correct_index, your_time_ms, their_time_ms
- [ ] `game_end` — your_score, their_score, result, rating_change
- [ ] `friend_presence` — user_id, status
- [ ] `ping` — ping_id
- [ ] `error` — message

### REST endpoints

For EACH endpoint, verify:
- [ ] Path matches between Go router and Dart API client
- [ ] HTTP method matches
- [ ] Request body field names match (JSON keys)
- [ ] Response body field names match (JSON keys)
- [ ] Auth header is included in Dart requests
- [ ] Error response format is handled in Dart

### Data types mapping

| Go type | Dart type | JSON format |
|---------|-----------|-------------|
| `string` | `String` | `"value"` |
| `int` | `int` | `123` |
| `bool` | `bool` | `true` |
| `*string` (nullable) | `String?` | `"value"` or `null` |
| `time.Time` | `DateTime` | ISO 8601 string |
| `uuid.UUID` | `String` | UUID string |
| `[]string` | `List<String>` | `["a", "b"]` |
| `json.RawMessage` | `Map<String, dynamic>` | nested object |

### Common drift patterns to watch for

1. **Field added in Go, missing in Dart**: Go adds a new field to a response. Dart's `fromJson` ignores it silently (no error) but the UI doesn't show it. Symptom: new feature works in API tests but not in the app.

2. **Field renamed in Go, stale in Dart**: JSON tag changed in Go struct but Dart model still uses the old name. Symptom: null values in Dart where data should exist.

3. **Type changed in Go, not in Dart**: e.g., Go changes a field from `int` to `string`. Dart's JSON parsing throws a runtime type error. Symptom: crash on receiving specific messages.

4. **New message type in Go, no handler in Dart**: Server sends a new WebSocket message type, Dart's message router doesn't recognize it and drops it. Symptom: silent failure of new feature.

5. **Enum values out of sync**: Go uses string constants for status fields ("online", "offline", "in_match"). Dart has an enum that doesn't include a new value. Symptom: crash or unhandled state.

## Output format

```
SYNC STATUS: [IN SYNC / DRIFT DETECTED]

Drift items:
1. [message_type] field_name — Go has `field_name string`, Dart missing
2. [endpoint] /auth/me — Go returns `rating_delta_week`, Dart User model missing this field
3. [message_type] game_end — Go added `rating_change` field, Dart ws_messages.dart not updated

Action required:
- Update lib/models/ws_messages.dart: add rating_change to GameEndMessage
- Update lib/models/user.dart: add ratingDeltaWeek field + fromJson mapping
```
