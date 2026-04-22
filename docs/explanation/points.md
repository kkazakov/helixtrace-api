# Points Handler

**Type:** Explanation

The points handler manages user-defined geographic points with CRUD operations, elevation fetching, category management, and optional integration with external Meshcore repeater data.

## Responsibility

- Create points with automatic elevation fetching from OpenTopoData
- Update points via partial update (ReplacingMergeTree re-insert pattern)
- Soft-delete points by setting `deleted = true`
- Retrieve single point by ID with ownership/public visibility checks
- List user's points with optional public point inclusion
- List available point categories
- Fetch elevation for arbitrary coordinates
- Optionally merge Meshcore Dashboard repeater data into point listings

## Source Files

- `internal/handlers/point.go` — PointHandler, all point operations, meshcore integration

## Public Interface

```go
type PointHandler struct {
    Conn clickhouse.Conn
    Cfg  *config.Config
}
```

### Endpoints

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/point` | Create a new point |
| `GET` | `/api/point/{id}` | Get point by ID |
| `PUT` | `/api/point/{id}` | Update a point |
| `DELETE` | `/api/point/{id}` | Soft-delete a point |
| `GET` | `/api/points` | List points (`?include_public=true`, `?include_meshcore_dashboard=true`) |
| `GET` | `/api/point/info` | Fetch elevation for coordinates (`?lat=&lon=`) |
| `GET` | `/api/point-categories` | List categories |

## Internal Structure

### Create Point (`CreatePoint`)
1. Parse JSON body: lat, lon, public, label, category_id
2. Validate lat in [-90, 90], lon in [-180, 180]
3. Require category_id != 0
4. Get authenticated user email from context
5. Validate category_id exists in `point_categories FINAL`
6. Fetch elevation from OpenTopoData via `fetchElevation()`
7. Generate UUIDv4 for point ID
8. Insert into `points` table with `deleted = false`
9. Return 201 Created with point data

### Update Point (`UpdatePoint`)
1. Extract point ID from path parameter
2. Parse JSON body with pointer fields (allows partial updates)
3. Get authenticated user email from context
4. Fetch existing point from `points FINAL` where `id = ? AND user = ? AND deleted = false`
5. Return 404 if point not found or user doesn't own it
6. Merge provided fields with existing values (pointer nil = keep existing)
7. Validate lat/lon ranges and category_id if provided
8. Re-insert point with same ID, updated `updated_at`, and `deleted = false`
9. Return 200 OK with updated point

### Delete Point (`DeletePoint`)
1. Extract point ID from path parameter
2. Get authenticated user email from context
3. Fetch existing point (same ownership check as Update)
4. Return 404 if not found
5. Re-insert with `deleted = true` and updated `updated_at`
6. Return 200 OK with `{"status": "deleted"}`

### Get Point (`GetPoint`)
1. Extract point ID from path parameter
2. Query `points FINAL` where `id = ? AND deleted = false AND (user = ? OR public = true)`
3. Return 404 if not found
4. Include `user` field in response only if the requester owns the point
5. Return 200 OK with point data wrapped in `{"data": {...}}`

### List Points (`ListPoints`)
1. Get authenticated user email from context
2. Check `include_public` query parameter
3. If `include_public = true`: query user's points + all public points
4. Otherwise: query only user's points
5. Both queries filter `deleted = false` and order by `updated_at DESC`
6. Check `include_meshcore_dashboard` query parameter
7. If enabled and `MESHCORE_DASHBOARD_API` is configured, merge external repeater data
8. Return empty array `[]` if no points (not null)

### Get Point Info (`GetPointInfo`)
1. Parse `lat` and `lon` query parameters
2. Validate ranges
3. Fetch elevation from OpenTopoData
4. Return 200 OK with `{"data": {"lat": ..., "lon": ..., "elevation": ...}}`

### List Categories (`ListCategories`)
1. Query `point_categories FINAL ORDER BY id`
2. Return empty array `[]` if no categories
3. Return 200 OK with category list

### Elevation Fetching (`fetchElevation`)
Single-point elevation fetch from OpenTopoData. Constructs URL `{baseURL}?locations=lat,lon`, parses JSON response, rounds elevation to 2 decimal places.

### Meshcore Repeater Integration

When `include_meshcore_dashboard=true` and `MESHCORE_DASHBOARD_API` is configured:

1. **Cache check** — `meshcoreCache` sync.Map stores fetched repeater data with 1-hour TTL
2. **API fetch** — GET `{MESHCORE_DASHBOARD_API}/api/repeaters/companion`
3. **Filter** — Keep only repeaters heard within the last 14 days
4. **Parse coordinates** — Extract lat/lon from string fields
5. **Batch elevation fetch** — `fetchElevationsBatched()` fetches elevations for all repeaters in batches of `OPENTOPADATA_MAX_LOCATIONS`
6. **Build response** — Create `externalPointResponse` objects with `external: true`, `category_id: 2` (repeater)
7. **Cache result** — Store serialized JSON in meshcoreCache
8. **Merge** — Append external points to user's points in the response

### Batch Elevation Fetching (`fetchElevationsBatched`)
Similar to trace path batch fetching. On error for a batch, fills with elevation 0 for each failed location, allowing partial results.

## Data Model

### points table
| Column | Type | Codec | Purpose |
|---|---|---|---|
| `id` | UUID | ZSTD(1) | UUIDv4 primary key |
| `lat` | Float64 | ZSTD(1) | Latitude |
| `lon` | Float64 | ZSTD(1) | Longitude |
| `elevation` | Float64 | Delta, ZSTD(1) | Elevation in meters |
| `user` | String | ZSTD(1) | Owner email |
| `public` | Bool | — | Visibility flag, default false |
| `label` | String | ZSTD(1) | User-defined label |
| `category_id` | UInt8 | LZ4 | FK to point_categories |
| `deleted` | Bool | — | Soft delete flag |
| `updated_at` | DateTime64 | Delta, ZSTD(1) | Version for ReplacingMergeTree |

Engine: `ReplacingMergeTree(updated_at)`, ORDER BY `(user, id)`

### point_categories table
| Column | Type | Purpose |
|---|---|---|
| `id` | UInt8 | Category ID |
| `name` | LowCardinality(String) | Category name |
| `updated_at` | DateTime64 | Version for ReplacingMergeTree |

Engine: `ReplacingMergeTree(updated_at)`, ORDER BY `id`

Seed data: `poi`(1), `repeater`(2), `unknown`(3)

## Dependencies

### Internal
- [Configuration](configuration.md) — OpenTopoData settings, Meshcore API URL
- [Database](database.md) — ClickHouse connection
- [Authentication](authentication.md) — EmailFromContext for user identification
- [Models](models.md) — Point, PointCategory structs

### External
- OpenTopoData API — Elevation data
- Meshcore Dashboard API — External repeater data (optional)
- `google/uuid` — UUID generation

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `OPENTOPADATA_SERVER` | `https://api.opentopodata.org/v1/` | Elevation API base URL |
| `OPENTOPADATA_MAX_LOCATIONS` | `100` | Max locations per batch |
| `MESHCORE_DASHBOARD_API` | _(empty)_ | Meshcore Dashboard API URL |

## Design Decisions

### Soft deletes via re-insert
Instead of `ALTER TABLE DELETE`, points are "deleted" by re-inserting with `deleted = true` and a newer `updated_at`. The ReplacingMergeTree engine keeps the latest version. Queries filter `WHERE deleted = false`. This avoids expensive ALTER TABLE operations on large datasets.

### Ownership enforcement
Update and Delete operations verify the point belongs to the authenticated user (`WHERE id = ? AND user = ?`). Get allows access to public points from other users.

### Partial updates via pointer fields
The `updatePointRequest` struct uses pointer types (`*float64`, `*bool`, etc.) to distinguish between "field not provided" (nil) and "field set to zero value". This enables partial updates without requiring the client to send all fields.

### Meshcore cache with TTL
External repeater data is cached for 1 hour to reduce API calls. The cache uses a `sync.Map` with a `cacheEntry` struct that tracks fetch time. Stale entries are automatically refreshed on the next request.

## Testing

No Go tests exist. Point endpoints are tested via the Bruno collection in `bruno/Points/`.
