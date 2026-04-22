# Architecture Overview

**Type:** Explanation

System-level architecture map for helixtrace-api using C4 model concepts.

## System Context

Helixtrace API is an HTTP service that provides elevation profile data between geographic coordinates and manages user-defined geographic points. The system sits between client applications (web/frontend) and external elevation data services.

```
[Client App] --> [Helixtrace API] --> [ClickHouse]
                              --> [OpenTopoData API]
                              --> [Meshcore Dashboard API] (optional)
```

### Users
- End users authenticate via email/password to create, manage, and query geographic points
- Anonymous access is not supported; all data operations require authentication

### External Systems
- **ClickHouse** — Primary data store for users, tokens, points, and trace path cache
- **OpenTopoData API** — Third-party elevation data service (configurable endpoint)
- **Meshcore Dashboard API** — Optional external source for repeater point data

## Containers

### Helixtrace API (Go binary)
- Single-process HTTP server using chi router
- Listens on configurable host:port (default `0.0.0.0:8000`)
- Deploys as a Docker container or standalone binary
- Stateful only in memory (mutex map, meshcore cache); all persistent data in ClickHouse

### ClickHouse Server
- Columnar database running in Docker or standalone
- Stores all application data: users, tokens, points, categories, trace path cache
- Migrations run at API startup via SQL files in `sql/` directory

## Components

| Component | Description | Deep-Dive |
|---|---|---|
| Entry Point & Routing | Bootstrap, chi router, middleware chain | [entry-point.md](entry-point.md) |
| Configuration | Environment variable loading with defaults | [configuration.md](configuration.md) |
| Database Layer | ClickHouse connection, migration runner | [database.md](database.md) |
| Authentication | Login/register, token management, Bearer middleware | [authentication.md](authentication.md) |
| Trace Path Handler | Elevation profile computation with caching | [trace-path.md](trace-path.md) |
| Points Handler | Point CRUD, meshcore integration | [points.md](points.md) |
| Models | Data structures for database mapping | [models.md](models.md) |

## Data Flow

### Trace Path Request
```
Client --> GET /api/trace-path?from=lat,lon&to=lat,lon
  --> AuthMiddleware validates Bearer token (ClickHouse query)
  --> Email injected into context
  --> TracePathHandler computes path hash (SHA256)
  --> Cache lookup in trace_paths table
  --> CACHE HIT: return cached elevation data
  --> CACHE MISS: acquire per-hash mutex
  --> Double-check cache inside lock
  --> Interpolate points between coordinates (Haversine)
  --> Batch fetch elevations from OpenTopoData API
  --> Save results to trace_paths table
  --> Return elevation profile to client
```

### Point Creation
```
Client --> POST /api/point { lat, lon, label, category_id, public }
  --> AuthMiddleware validates Bearer token
  --> PointHandler validates coordinates and category
  --> Fetch elevation from OpenTopoData API
  --> Generate UUID, insert into points table
  --> Return created point with elevation
```

### Point Listing with Meshcore
```
Client --> GET /api/points?include_public=true&include_meshcore_dashboard=true
  --> AuthMiddleware validates Bearer token
  --> Query user's points + public points from ClickHouse
  --> Check meshcore cache (1-hour TTL)
  --> CACHE MISS: fetch repeaters from Meshcore Dashboard API
  --> Filter repeaters heard within 14 days
  --> Batch fetch elevations from OpenTopoData API
  --> Cache result, merge with user points
  --> Return combined point list
```

## Cross-Cutting Concerns

### Authentication
- Bearer token auth via middleware on protected route group
- Tokens stored in ClickHouse with 24-hour expiry and TTL-based cleanup
- Email injected into request context for downstream handlers

### Error Handling
- Consistent JSON error responses via `WriteError(w, status, message)`
- Errors logged with `log.Printf("ERROR %d: %s", status, message)`
- No structured error types; plain string messages

### Logging
- Standard `log` package with `middleware.Logger` for request logging
- Component-specific log prefixes: `[trace-path]`, `[meshcore]`
- `DEBUG` environment variable exists but is not actively used in code

### CORS
- Conditional CORS middleware based on `CORS_ALLOWED_ORIGINS`
- When empty, no CORS headers are added
- Supports credentials and standard headers

### Caching
- **Trace path cache** — ClickHouse-backed with per-hash mutex dedup
- **Meshcore repeater cache** — In-memory sync.Map with 1-hour TTL

## Quality Attributes

### Performance
- ClickHouse columnar storage provides efficient array storage for trace path data
- Batch elevation fetching (100 locations per request) reduces API round trips
- Per-hash mutex allows concurrent processing of different paths

### Reliability
- Fail-fast startup ensures server only runs when fully initialized
- ClickHouse connection verified before accepting requests
- Idempotent migrations allow safe restarts

### Scalability
- Stateless API server supports horizontal scaling behind a load balancer
- ClickHouse handles concurrent reads efficiently
- Per-hash mutexes minimize lock contention

## External Dependencies

| Dependency | Role | Criticality | Fallback |
|---|---|---|---|
| ClickHouse | Primary data store | Critical | None — API fails to start without it |
| OpenTopoData API | Elevation data | High | Point creation and trace paths fail |
| Meshcore Dashboard API | External repeater data | Low | Skipped when `MESHCORE_DASHBOARD_API` is empty |
