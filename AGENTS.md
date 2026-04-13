# AGENTS.md — helixtrace-api

## Project
- Go 1.26.2 module (`github.com/wasp/helixtrace-api`)
- HTTP API using `go-chi/chi/v5` router, ClickHouse as the sole database
- Entry point: `main.go`

## Setup & Run
1. Copy `.env.example` → `.env` (gitignored)
2. Start ClickHouse: `docker compose -f docker/clickhouse/docker-compose.yaml up -d`
3. Run: `./run.sh` (builds then runs) or `go run main.go`
4. Server listens on `0.0.0.0:8000` by default

## Architecture
- `internal/config/` — env loading via `godotenv`, no `.env` file = silent fallback to defaults
- `internal/database/` — ClickHouse connect + **migrations run at startup** from `sql/*.sql` (sorted alphabetically, split on `;`)
- `internal/handlers/` — auth handlers (login, register), JSON helpers
- `internal/middleware/` — Bearer token auth middleware
- `internal/models/` — User/Token structs with `ch:` tags
- `sql/` — numbered migration files; `001_users.sql` seeds admin user

## Auth Flow
- Bearer token auth; tokens are random hex strings stored in ClickHouse `tokens` table (24h TTL)
- `users` and `tokens` tables use `ReplacingMergeTree` — queries must include `FINAL`
- Protected routes wrapped in `r.Group()` with auth middleware

## Testing
- Bruno API collection in `bruno/` (login, register, health)
- No Go tests exist yet; no linter/formatter configured

## Key Env Vars
| Var | Default |
|---|---|
| `CLICKHOUSE_HOST` | `localhost` |
| `CLICKHOUSE_PORT` | `9000` (native) |
| `CLICKHOUSE_DATABASE` | `helixtrace` |
| `CLICKHOUSE_USER` | `admin` |
| `CLICKHOUSE_PASSWORD` | _(empty)_ |
| `API_HOST` | `0.0.0.0` |
| `API_PORT` | `8000` |

## Gotchas
- ClickHouse must be running **before** starting the API; startup fails on connection error
- Migrations are re-applied every startup (`CREATE TABLE IF NOT EXISTS`), so they are idempotent
- Docker compose sets `CLICKHOUSE_PASSWORD=tlZ98k3QR2ycsp` — `.env` must match if using docker
