# Trace Path Handler

**Type:** Explanation

The trace path handler computes elevation profiles between two geographic coordinates by interpolating points along a great-circle path, fetching elevation data from OpenTopoData, and caching results in ClickHouse.

## Responsibility

- Parse `from` and `to` coordinate query parameters
- Compute great-circle distance using the Haversine formula
- Interpolate evenly-spaced points between coordinates
- Fetch elevation data from OpenTopoData API in batched requests
- Cache computed paths in ClickHouse to avoid redundant API calls
- Deduplicate concurrent requests for the same path using per-hash mutexes

## Source Files

- `internal/handlers/tracepath.go` — TracePathHandler, haversine, makePathHash, TracePath

## Public Interface

```go
type TracePathHandler struct {
    Conn clickhouse.Conn
    Cfg  *config.Config
    mu   sync.Map
}

func (h *TracePathHandler) TracePath(w http.ResponseWriter, r *http.Request)
```

### Request
```
GET /api/trace-path?from=lat,lon&to=lat,lon
```

### Response
```json
{
  "points": [{"lat": 42.0, "lng": 23.0, "elv": 500.0}, ...],
  "count": 150,
  "distance_between_points": 50.0,
  "status": "ok"
}
```

## Internal Structure

### Haversine Distance (`haversine`)
Computes great-circle distance between two lat/lon points using the Haversine formula with Earth radius 6,371,000 m. Result is in meters.

### Path Hash (`makePathHash`)
Creates a SHA256 hash of the path parameters: `fromLat,fromLon,toLat,toLon,pointDistance` formatted to fixed precision. The hash serves as the cache key.

### Point Interpolation
Given `from` and `to` coordinates and a target distance between points:
1. Compute total distance via Haversine
2. Calculate number of points: `distance / TRACE_PATH_POINT_DISTANCE`
3. Clamp to range [2, 1000]
4. Linearly interpolate: `point[i] = from + (to - from) * (i / (numPoints - 1))`

### OpenTopoData Batch Fetching
Points are fetched in batches of `OPENTOPADATA_MAX_LOCATIONS` (default 100):
1. Split interpolated points into batches
2. For each batch, construct URL: `{baseURL}?locations=lat1,lon1|lat2,lon2|...`
3. HTTP GET the URL, parse JSON response
4. Accumulate results across batches

### Caching Strategy

The caching uses a two-phase approach with per-hash mutexes:

**Phase 1 — Unlocked cache lookup:**
- Compute path hash
- Query `trace_paths` table: `SELECT lats, lngs, elvs, count FROM trace_paths WHERE path_hash = ? ORDER BY created_at DESC LIMIT 1`
- On cache hit, return immediately

**Phase 2 — Locked computation (on cache miss):**
- Acquire per-hash mutex from `sync.Map` via `LoadOrStore`
- Re-check cache inside lock (double-check pattern)
- If still missed, compute path and save to cache
- Release mutex

The `sync.Map` stores `*sync.Mutex` values keyed by path hash. Each unique path gets its own mutex, preventing lock contention between different paths while serializing concurrent requests for the same path.

### Cache Storage (`saveCache`)
Inserts into `trace_paths` table: path_hash, from/to coordinates, point_distance, count, lats array, lngs array, elvs array, created_at.

The `trace_paths` table uses `MergeTree` engine (not ReplacingMergeTree), ordered by `(path_hash, created_at)`. Cache lookup uses `ORDER BY created_at DESC LIMIT 1` to get the most recent entry.

## Data Model

### trace_paths table
| Column | Type | Purpose |
|---|---|---|
| `path_hash` | String | SHA256 cache key |
| `from_lat` | Float64 | Start latitude |
| `from_lon` | Float64 | Start longitude |
| `to_lat` | Float64 | End latitude |
| `to_lon` | Float64 | End longitude |
| `point_distance` | Float64 | Configured spacing in meters |
| `count` | UInt32 | Number of points |
| `lats` | Array(Float64) | Latitude values |
| `lngs` | Array(Float64) | Longitude values |
| `elvs` | Array(Float64) | Elevation values |
| `created_at` | DateTime64 | Cache entry timestamp |

Engine: `MergeTree()`, ORDER BY `(path_hash, created_at)`

## Dependencies

### Internal
- [Configuration](configuration.md) — OpenTopoData server URL, max locations, point distance
- [Database](database.md) — ClickHouse connection for cache operations

### External
- OpenTopoData API — Elevation data source
- `crypto/sha256` — Cache key generation

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `OPENTOPADATA_SERVER` | `https://api.opentopodata.org/v1/` | Elevation API base URL |
| `OPENTOPADATA_MAX_LOCATIONS` | `100` | Max locations per batch request |
| `TRACE_PATH_POINT_DISTANCE` | `50` | Distance between points in meters |

## Design Decisions

### Linear interpolation over geodesic
Points are interpolated linearly in lat/lon space rather than following a true geodesic. For short distances (< ~100 km), the difference is negligible. This simplifies the math and avoids dependency on a geodesic library.

### Per-hash mutex via sync.Map
A single global mutex would serialize all trace path requests. A per-hash mutex allows different paths to be computed concurrently while preventing duplicate work for the same path. The `sync.Map` provides lock-free reads for the common case (mutex lookup).

### MergeTree (not ReplacingMergeTree) for cache
The trace_paths table uses plain MergeTree because cache entries are append-only — new entries are added with newer `created_at`, and queries use `ORDER BY created_at DESC LIMIT 1`. There is no need for deduplication.

### UInt32 for count column
The `count` column is `UInt32` in ClickHouse. Go code scans into `*uint32`, not `*int`. Scanning into the wrong type causes silent failures and cache misses.

## Testing

No Go tests exist. The trace-path endpoint is tested via the Bruno collection in `bruno/Trace-Path.bru`.
