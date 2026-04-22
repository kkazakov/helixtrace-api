# Helixtrace API

HTTP API for fetching elevation profiles between coordinates and managing user-defined geographic points, backed by ClickHouse.

See [PURPOSE.md](PURPOSE.md) for business context and constraints.

## Build, Run, Test

### Prerequisites
- Go 1.26.2
- ClickHouse server running (see Docker setup below)

### Local Development

```bash
# Copy environment configuration
cp .env.example .env

# Start ClickHouse
docker compose -f docker/clickhouse/docker-compose.yaml up -d

# Build and run
./run.sh
# Or: go run main.go
```

Server listens on `0.0.0.0:8000` by default.

### Docker Deployment

```bash
# Build and start API + ClickHouse
./deploy.sh
# Or: docker compose up -d --build
```

### API Testing

Bruno collection in `bruno/` covers all endpoints: Auth, Points, Trace-Path.

## Architecture at a Glance

```
Client --> [Helixtrace API] --> [ClickHouse]
                      --> [OpenTopoData]
                      --> [Meshcore Dashboard] (optional)
```

### Components

| Component | Description |
|---|---|
| [Entry Point & Routing](docs/explanation/entry-point.md) | Bootstrap, chi router, middleware chain |
| [Configuration](docs/explanation/configuration.md) | Environment variable loading |
| [Database Layer](docs/explanation/database.md) | ClickHouse connection, migrations |
| [Authentication](docs/explanation/authentication.md) | Login/register, Bearer token auth |
| [Trace Path Handler](docs/explanation/trace-path.md) | Elevation profiles with caching |
| [Points Handler](docs/explanation/points.md) | Point CRUD, meshcore integration |
| [Models](docs/explanation/models.md) | Data structures |

## Repository Map

| Path | Purpose |
|---|---|
| `main.go` | Application entry point, route definitions |
| `internal/config/` | Environment variable loading |
| `internal/database/` | ClickHouse connection, migration runner |
| `internal/handlers/` | Auth, trace path, point handlers |
| `internal/middleware/` | Bearer token auth middleware |
| `internal/models/` | User, Token, Point, Category structs |
| `sql/` | Numbered migration files (run at startup) |
| `bruno/` | Bruno API collection for testing |
| `docker/` | Docker Compose for ClickHouse dependency |
| `docs/` | Documentation |

## Tech Stack

- **Language:** Go 1.26.2
- **Router:** go-chi/chi/v5
- **Database:** ClickHouse (clickhouse-go/v2 driver)
- **Auth:** Bearer tokens with bcrypt password hashing
- **Elevation Data:** OpenTopoData API (configurable endpoint)
- **Container:** Docker (multi-stage build, debian:bookworm-slim runtime)

## Documentation

- [Documentation Index](docs/index.md) — Navigation hub
- [Architecture Overview](docs/explanation/architecture-overview.md) — System context, data flows, cross-cutting concerns
- [API Reference](docs/reference/api.md) — Endpoint descriptions, request/response schemas
- [Architecture Decision Records](docs/architecture/adr/) — Key architectural choices
