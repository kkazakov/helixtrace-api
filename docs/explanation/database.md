# Database Layer

**Type:** Explanation

The database component manages ClickHouse connectivity and runs idempotent schema migrations at startup.

## Responsibility

- Open ClickHouse connection with configured credentials
- Verify connectivity via ping
- Execute all SQL migration files from the `sql/` directory in alphabetical order
- Parse SQL files by stripping comments and splitting on `;`

If ClickHouse is unreachable or any migration fails, the application exits with `log.Fatalf`.

## Source Files

- `internal/database/clickhouse.go` — Connect and InitDB functions

## Public Interface

```go
func Connect(cfg *config.Config) (clickhouse.Conn, error)
func InitDB(conn clickhouse.Conn, sqlDir string) error
```

## Internal Structure

### `Connect(cfg)`
Creates a `clickhouse.Options` struct with the address and auth from config. Opens connection via `clickhouse.Open()`, then pings to verify the server is reachable. Returns the connection object or an error.

### `InitDB(conn, sqlDir)`
1. Globs `sqlDir/*.sql` to find all migration files
2. Sorts files alphabetically (ensures `001_*.sql` runs before `002_*.sql`)
3. For each file:
   - Reads file content
   - Strips lines starting with `--` (SQL comments)
   - Joins remaining lines into a single string
   - Splits on `;` to extract individual statements
   - Executes each non-empty statement via `conn.Exec()`

## Migration Files

| File | Purpose |
|---|---|
| `sql/001_users.sql` | Users table + admin seed |
| `sql/002_tokens.sql` | Tokens table with TTL |
| `sql/003_trace_paths.sql` | Trace path cache table |
| `sql/004_point_categories.sql` | Point categories + seeds |
| `sql/005_points.sql` | Points table |

All migrations use `CREATE TABLE IF NOT EXISTS`, making them idempotent. Migrations are re-applied on every startup.

## Dependencies

### Internal
- [Configuration](configuration.md) — ClickHouse connection parameters

### External
- `ClickHouse/clickhouse-go/v2` — ClickHouse Go driver

## Design Decisions

### Idempotent migrations via IF NOT EXISTS
Every `CREATE TABLE` uses `IF NOT EXISTS`, so migrations are safe to re-run on every startup. This eliminates the need for a migration tracking table.

### Comment stripping
The migration runner strips `--` comments before executing. This allows human-readable comments in migration files without affecting execution.

### Simple statement splitting
SQL is split on `;` after joining all non-comment lines. This works for the current simple migrations but would break on statements containing `;` in string literals.

### Single connection pattern
One connection is created at startup and shared across all handlers. ClickHouse's Go driver maintains an internal connection pool, so a single `Conn` object handles concurrent requests.

## Testing

No unit tests exist. Migrations are tested implicitly during application startup.
