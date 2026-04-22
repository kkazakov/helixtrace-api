# Configuration

**Type:** Explanation

The configuration component loads environment variables with fallback defaults and parses structured values like CORS origins.

## Responsibility

- Load all environment variables required by the application
- Provide fallback defaults when variables are unset
- Parse comma-separated CORS origins into a slice
- Parse numeric values (port, distance, max locations)

If no `.env` file exists, `godotenv.Load()` returns silently and all values fall back to defaults or `os.Getenv`.

## Source Files

- `internal/config/config.go` — Config struct and Load function

## Public Interface

```go
type Config struct {
    OpenTopoDataServer       string
    OpenTopoDataMaxLocations int
    TracePathPointDistance   float64
    ClickHouseHost           string
    ClickHousePort           int
    ClickHouseDatabase       string
    ClickHouseUser           string
    ClickHousePassword       string
    APIHost                  string
    APIPort                  int
    CorsAllowedOrigins       []string
    Debug                    bool
    MeshcoreDashboardAPI     string
}

func Load() (*Config, error)
```

## Internal Structure

### `Load()`
Calls `godotenv.Load()` to load `.env` file, then reads each variable via `getEnv(key, fallback)`. Numeric values are parsed with `strconv.Atoi` / `strconv.ParseFloat`. CORS origins are parsed by `parseCorsOrigins()`.

### `getEnv(key, fallback)`
Reads `os.Getenv(key)`. Returns the environment value if non-empty, otherwise returns the fallback.

### `parseCorsOrigins(value)`
Splits a comma-separated string by `,`, trims whitespace from each element, and filters out empty strings. Returns `nil` if the input is empty.

## Configuration Variables

| Variable | Default | Type | Purpose |
|---|---|---|---|
| `OPENTOPADATA_SERVER` | `https://api.opentopodata.org/v1/` | string | Base URL for elevation data API |
| `OPENTOPADATA_MAX_LOCATIONS` | `100` | int | Maximum locations per batch request |
| `TRACE_PATH_POINT_DISTANCE` | `50` | float64 | Distance between interpolated points in meters |
| `CLICKHOUSE_HOST` | `localhost` | string | ClickHouse server host |
| `CLICKHOUSE_PORT` | `9000` | int | ClickHouse native protocol port |
| `CLICKHOUSE_DATABASE` | `helixtrace` | string | Database name |
| `CLICKHOUSE_USER` | `admin` | string | Database username |
| `CLICKHOUSE_PASSWORD` | _(empty)_ | string | Database password |
| `API_HOST` | `0.0.0.0` | string | HTTP server bind address |
| `API_PORT` | `8000` | int | HTTP server bind port |
| `CORS_ALLOWED_ORIGINS` | _(empty)_ | string | Comma-separated CORS allowed origins |
| `DEBUG` | `false` | bool | Debug logging flag |
| `MESHCORE_DASHBOARD_API` | _(empty)_ | string | Meshcore Dashboard API base URL |

## Dependencies

### Internal
None. This is a foundational component.

### External
- `joho/godotenv` — `.env` file loading

## Design Decisions

### Silent fallback on missing .env
`godotenv.Load()` is called with `_` to discard its error. This allows the application to run without a `.env` file when environment variables are set externally (e.g., in Docker, Kubernetes, or CI/CD).

### No validation
The config component does not validate values beyond parsing. Invalid values (e.g., negative port) are caught at the point of use.

## Testing

No unit tests exist for the configuration component.
