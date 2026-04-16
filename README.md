# Helixtrace API

HTTP API for fetching elevation profiles between coordinates, backed by ClickHouse.

## Quick Start

```bash
cp .env.example .env
docker compose -f docker/clickhouse/docker-compose.yaml up -d
./run.sh
```

Server listens on `0.0.0.0:8000`.

## API

### Authentication
```
POST /api/login    → { "email": "...", "password": "..." }
POST /api/register → { "email": "...", "password": "...", "username": "..." }
```

Returns a bearer token used in `Authorization: Bearer <token>` for protected routes.

### Trace Path
```
GET /api/trace-path?from=42.4233664,23.0068696&to=42.4817369,23.0368781
```

Returns elevation data for interpolated points between two coordinates. Results are cached in ClickHouse.

### Health
```
GET /api/health
```

### Points

Manage user-defined geographic points with elevation, labels, and categories.

```
POST /api/point        → Create a new point
PUT /api/point/{id}    → Update a point by ID
DELETE /api/point/{id} → Soft-delete a point
GET /api/points        → List user's points (?include_public=true for public points too)
GET /api/point-categories → List available point categories
```

#### Create Point
```json
POST /api/point
{
  "lat": 42.6977,
  "lon": 23.3219,
  "elevation": 550.5,
  "public": false,
  "label": "Sofia office",
  "category_id": 1
}
```

#### Update Point
```json
PUT /api/point/{id}
{
  "label": "Updated label",
  "public": true
}
```
Only provided fields are updated; omitted fields retain their current values.

#### List Points
```
GET /api/points?include_public=true
```
Without `include_public`, returns only the authenticated user's points. With `include_public=true`, also includes other users' public points.

#### Categories
Seed categories: `poi` (1), `repeater` (2), `unknown` (3).

## Configuration

| Variable | Default | Description |
|---|---|---|
| `CLICKHOUSE_HOST` | `localhost` | ClickHouse host |
| `CLICKHOUSE_PORT` | `9000` | ClickHouse native port |
| `CLICKHOUSE_DATABASE` | `helixtrace` | Database name |
| `CLICKHOUSE_USER` | `admin` | Database user |
| `CLICKHOUSE_PASSWORD` | _(empty)_ | Database password |
| `API_HOST` | `0.0.0.0` | Bind address |
| `API_PORT` | `8000` | Bind port |
| `OPENTOPADATA_SERVER` | `https://api.opentopodata.org/v1/` | Elevation API base URL |
| `OPENTOPADATA_MAX_LOCATIONS` | `100` | Max locations per batch request |
| `TRACE_PATH_POINT_DISTANCE` | `50` | Distance between points in meters |
