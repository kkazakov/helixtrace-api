# Entry Point & Routing

**Type:** Explanation

The entry point bootstraps the application, configures the chi router, sets up middleware, and starts the HTTP server.

## Responsibility

- Load configuration from environment
- Establish ClickHouse connection
- Run database migrations
- Configure and wire HTTP routes
- Start the HTTP server

The application fails fast on startup if ClickHouse is unreachable or migrations fail.

## Source Files

- `main.go` — Application entry point, route definitions, server startup

## Public Interface

The application exposes a single `main()` function. It has no library API.

## Internal Structure

### Startup Sequence

1. `config.Load()` reads environment variables (with `.env` file support via godotenv)
2. `database.Connect(cfg)` opens ClickHouse connection and pings to verify connectivity
3. `database.InitDB(conn, sqlDir)` runs all SQL migrations from the `sql/` directory
4. Chi router is created with Logger and Recoverer middleware
5. CORS middleware is added if `CORS_ALLOWED_ORIGINS` is configured
6. Handlers are instantiated with the ClickHouse connection and config
7. Routes are registered (public and protected groups)
8. `http.ListenAndServe()` starts the server

### Route Registration

Public routes (no authentication):
- `POST /api/login` — AuthHandler.Login
- `POST /api/register` — AuthHandler.Register
- `GET /api/health` — Inline handler returning `{"status": "ok"}`

Protected routes (Bearer token auth via middleware group):
- `GET /api/profile` — Inline handler returning authenticated user email
- `GET /api/trace-path` — TracePathHandler.TracePath
- `GET /api/point/info` — PointHandler.GetPointInfo
- `POST /api/point` — PointHandler.CreatePoint
- `GET /api/point/{id}` — PointHandler.GetPoint
- `PUT /api/point/{id}` — PointHandler.UpdatePoint
- `DELETE /api/point/{id}` — PointHandler.DeletePoint
- `GET /api/points` — PointHandler.ListPoints
- `GET /api/point-categories` — PointHandler.ListCategories

### Middleware Chain

The middleware stack applies in this order:
1. `middleware.Logger` — Logs request method, path, and duration
2. `middleware.Recoverer` — Recovers from panics, returns 500
3. `cors.Handler` — CORS preflight handling (conditional)
4. `authmiddleware.AuthMiddleware` — Bearer token validation (protected routes only)

## Dependencies

### Internal
- [Configuration](configuration.md) — Environment variable loading
- [Database](database.md) — ClickHouse connection and migrations
- [Authentication](authentication.md) — Auth middleware for protected routes
- [Trace Path Handler](trace-path.md) — Elevation profile endpoint
- [Points Handler](points.md) — Point CRUD endpoints

### External
- `go-chi/chi/v5` — HTTP router
- `go-chi/cors` — CORS middleware
- `ClickHouse/clickhouse-go/v2` — ClickHouse driver

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `API_HOST` | `0.0.0.0` | Bind address |
| `API_PORT` | `8000` | Bind port |
| `CORS_ALLOWED_ORIGINS` | _(empty)_ | Comma-separated allowed origins |

## Design Decisions

### Fail-fast startup
The application calls `log.Fatalf` on any startup error. This ensures the server only runs when fully initialized. ClickHouse must be available before the API starts.

### Single connection object
One `clickhouse.Conn` is shared across all handlers. ClickHouse connections are connection-pooled internally, so a single Conn object is sufficient for concurrent requests.

## Testing

No Go tests exist. API testing is done via the Bruno collection in `bruno/`.
