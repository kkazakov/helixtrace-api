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
