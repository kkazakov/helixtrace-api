# AGENTS.md — helixtrace-api

## Project

Go 1.26.2 HTTP API (`go-chi/chi/v5`, ClickHouse) for elevation profiles and geographic point management. Always refer to this project as "helixtrace-api". Deep documentation lives in `docs/` — read it before making architectural changes.

## Commands

```bash
./run.sh                          # build + run (default :8000)
go run main.go                    # run without rebuilding
go build                          # build only
docker compose -f docker/clickhouse/docker-compose.yaml up -d   # start ClickHouse
docker compose up -d --build      # full deploy (API + ClickHouse)
```

No Go tests or linter exist. API testing uses Bruno collection in `bruno/`.

## Architecture

- `main.go` — bootstrap: config → ClickHouse connect → migrations → chi routes → serve
- `internal/config/` — env loading via godotenv; silent fallback if no `.env`
- `internal/database/` — ClickHouse connection + migration runner (`sql/*.sql`, sorted, split on `;`)
- `internal/handlers/` — auth, trace-path, point CRUD, JSON helpers, context helpers
- `internal/middleware/` — Bearer token auth middleware
- `internal/models/` — structs with `ch:` tags for ClickHouse column mapping
- `sql/` — numbered idempotent migrations (`CREATE TABLE IF NOT EXISTS`)

Routes are defined in `main.go`. See `docs/reference/api.md` for full endpoint reference.

## Conventions

**All ReplacingMergeTree queries MUST use `FINAL`.** The tables `users`, `tokens`, `points`, `point_categories` use `ReplacingMergeTree`. Without `FINAL`, stale rows appear. The `trace_paths` table is plain `MergeTree` — no `FINAL` needed there.

```go
// Correct — FINAL required for ReplacingMergeTree
SELECT * FROM points FINAL WHERE id = ? AND deleted = false

// Correct — no FINAL for plain MergeTree cache
SELECT * FROM trace_paths WHERE path_hash = ? ORDER BY created_at DESC LIMIT 1
```

**Updates are re-inserts, not ALTER.** The ClickHouse pattern is: read existing row, merge changes, INSERT with same ID and newer `updated_at`. Soft deletes re-insert with `deleted = true`.

**Partial updates use pointer types.** `*float64`, `*bool`, `*string` in request structs — nil means "don't change", non-nil means "override".

**Get full documentation before architectural changes.** Read `docs/explanation/` component docs and `docs/explanation/architecture-overview.md` before modifying data flow, adding tables, or changing caching strategy.

## Boundaries

- **NEVER** commit `.env` files or files containing credentials
- **NEVER** run `ALTER TABLE DELETE` on any table — use the soft-delete re-insert pattern
- **NEVER** remove `FINAL` from queries on `users`, `tokens`, `points`, `point_categories`
- **NEVER** scan ClickHouse `UInt32` into `*int` — use `*uint32` (silent failure → cache misses)
- **DO NOT** add new dependencies without explaining the rationale
- **REQUIRE HUMAN CONFIRMATION** before: changing ClickHouse table engines, modifying migration files in `sql/`, adding new database tables

## Gotchas

- ClickHouse must be running before the API starts — startup fails on connection error
- `UInt32` scan type mismatch is silent (no panic) but causes trace path cache misses
- Migrations run every startup and are idempotent — safe to restart, but don't add non-idempotent statements
- The trace-path handler uses a `sync.Map` of per-hash `*sync.Mutex` — don't replace with a single global mutex (serializes all requests)
- Docker compose and `.env.example` use different ClickHouse passwords — match them when switching environments
