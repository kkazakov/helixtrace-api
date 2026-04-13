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
- `internal/handlers/` — auth handlers (login, register), trace path handler, JSON helpers
- `internal/middleware/` — Bearer token auth middleware
- `internal/models/` — User/Token structs with `ch:` tags
- `sql/` — numbered migration files; `001_users.sql` seeds admin user

## Routes

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/login` | No | Authenticate user, returns bearer token |
| POST | `/api/register` | No | Create new user account |
| GET | `/api/health` | No | Health check |
| GET | `/api/profile` | Yes | Get authenticated user email |
| GET | `/api/trace-path` | Yes | Get elevation profile between two coordinates |

### Trace Path Endpoint
`GET /api/trace-path?from=lat,lon&to=lat,lon`

- Interpolates points between `from` and `to` at configured distance intervals
- Fetches elevation data from OpenTopoData server (batched in chunks)
- Caches results in ClickHouse `trace_paths` table (SHA256 hash of coordinates + config)
- In-memory mutex prevents duplicate concurrent saves for same hash
- Max 1000 points; if distance/config yields more, spacing is adjusted

## Auth Flow
- Bearer token auth; tokens are random hex strings stored in ClickHouse `tokens` table (24h TTL)
- `users` and `tokens` tables use `ReplacingMergeTree` — queries must include `FINAL`
- Protected routes wrapped in `r.Group()` with auth middleware

## Caching
- `trace_paths` table uses `MergeTree` engine, ordered by `(path_hash, created_at)`
- Cache key is SHA256 of `from_lat,from_lon,to_lat,to_lon,point_distance`
- Lookup uses `ORDER BY created_at DESC LIMIT 1` (no `FINAL` needed)
- Per-hash `sync.Mutex` via `sync.Map` prevents concurrent duplicate inserts
- ClickHouse `count` column is `UInt32` — scan into `*uint32`, not `*int`

## Testing
- Bruno API collection in `bruno/` (login, register, health, trace-path)
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
| `OPENTOPADATA_SERVER` | `https://api.opentopodata.org/v1/` |
| `OPENTOPADATA_MAX_LOCATIONS` | `100` |
| `TRACE_PATH_POINT_DISTANCE` | `50` (meters) |

## Gotchas
- ClickHouse must be running **before** starting the API; startup fails on connection error
- Migrations are re-applied every startup (`CREATE TABLE IF NOT EXISTS`), so they are idempotent
- Docker compose sets `CLICKHOUSE_PASSWORD=tlZ98k3QR2ycsp` — `.env` must match if using docker
- ClickHouse `UInt32` columns must be scanned into `*uint32`, not `*int` — scan errors are silent but cause cache misses
