# rapid-server

Go backend for Rapid — a real-time peer-to-peer trivia app. Two players see the same question simultaneously and race to answer correctly, scored on both accuracy and speed. 10 rounds per game, 15-second timer per question, with RTT compensation for fair scoring.

## Tech Stack

- **Go 1.26** — HTTP server, WebSocket game loop, structured logging
- **PostgreSQL 16** — users, friendships, categories, questions, matches
- **Redis 7** — reserved for future features (matchmaking queue, caching)
- **sqlc** — type-safe SQL query generation
- **OpenTelemetry** — distributed tracing (Tempo) + metrics (Prometheus)
- **Grafana** — dashboards, with Loki for logs and Promtail for collection

## Prerequisites

- Go 1.26+
- Docker & Docker Compose
- sqlc (`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`)

## Quick Start

```bash
# Start all services
docker compose up -d

# Run migrations
go run ./cmd/migrate up

# Seed categories and questions (250 trivia questions across 5 categories)
docker compose exec -T postgres psql -U rapid -d rapid < seeds/001_categories.sql
docker compose exec -T postgres psql -U rapid -d rapid < seeds/002_questions.sql

# Build and restart the server
docker compose up -d --build

# Run smoke test
./scripts/smoke_test.sh
```

## Endpoints

### REST (all except /health and /metrics require `Authorization: Bearer <token>`)

| Method | Path | Description |
|--------|------|-------------|
| GET | /health | Health check |
| GET | /metrics | Prometheus metrics |
| POST | /auth/register | Register a new user |
| GET | /auth/me | Get current user profile |
| POST | /friends/request | Send a friend request |
| POST | /friends/respond | Accept/decline a friend request |
| GET | /friends | List accepted friends with presence and H2H |
| GET | /friends/search?q= | Search users by username prefix |
| GET | /categories | List trivia categories |
| GET | /matches | List completed matches (paginated) |
| GET | /matches/{id} | Get match detail with round breakdown |

### WebSocket

| Path | Description |
|------|-------------|
| GET /ws | WebSocket upgrade (requires auth header) |

Message types: `challenge`, `challenge_resp`, `answer`, `pong` (client → server) and `challenge_recv`, `game_start`, `question`, `opponent_answered`, `round_result`, `game_end`, `friend_presence`, `ping`, `error` (server → client).

## Development

```bash
# Run tests
go test -race ./...

# Regenerate sqlc code after changing .sql files
sqlc generate

# Run migrations
go run ./cmd/migrate up    # apply
go run ./cmd/migrate down  # rollback
```

## Observability

With `docker compose up`, these are available:

| Service | URL |
|---------|-----|
| Grafana | http://localhost:3000 |
| Prometheus | http://localhost:9090 |
| Tempo | http://localhost:3200 |
| Loki | http://localhost:3100 |

The "Rapid — Game Server" dashboard is auto-provisioned in Grafana under Dashboards > Rapid.

## Local Auth

Set `AUTH_MODE=dev` (default in docker-compose) for development. Token format: `dev-<user_id>` extracts `<user_id>` as the Firebase UID. For production, set `AUTH_MODE=firebase` with proper credentials.
