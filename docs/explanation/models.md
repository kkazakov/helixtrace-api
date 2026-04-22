# Models

**Type:** Explanation

The models package defines Go structs that map to ClickHouse tables. Structs use `ch:` tags for column name mapping.

## Responsibility

- Define data structures for Users, Tokens, Points, and Point Categories
- Provide ClickHouse column mapping via `ch:` struct tags
- Serve as scan targets for query results

## Source Files

- `internal/models/user.go` — User, Token structs
- `internal/models/point.go` — PointCategory, Point structs

## Public Interface

### User
```go
type User struct {
    Email        string `ch:"email"`
    PasswordHash string `ch:"password_hash"`
    Username     string `ch:"username"`
    Active       bool   `ch:"active"`
    AccessRights string `ch:"access_rights"`
}
```

### Token
```go
type Token struct {
    Token     string `ch:"token"`
    Email     string `ch:"email"`
    CreatedAt string `ch:"created_at"`
    ExpiresAt string `ch:"expires_at"`
}
```

### PointCategory
```go
type PointCategory struct {
    ID   uint8  `ch:"id"`
    Name string `ch:"name"`
}
```

### Point
```go
type Point struct {
    ID         string  `ch:"id"`
    Lat        float64 `ch:"lat"`
    Lon        float64 `ch:"lon"`
    Elevation  float64 `ch:"elevation"`
    User       string  `ch:"user"`
    Public     bool    `ch:"public"`
    Label      string  `ch:"label"`
    CategoryID uint8   `ch:"category_id"`
    Deleted    bool    `ch:"deleted"`
    UpdatedAt string  `ch:"updated_at"`
}
```

## Internal Structure

Models are simple data carriers with no methods or business logic. The `ch:` tags map struct fields to ClickHouse column names.

### Type mapping notes

- `DateTime64` columns are scanned into `string` in Go (CreatedAt, ExpiresAt, UpdatedAt). The ClickHouse driver handles the conversion.
- `UUID` columns are scanned into `string` (Point.ID).
- `UInt8` maps to Go `uint8` (CategoryID).
- `Array(Float64)` columns in `trace_paths` are scanned directly into `[]float64` slices in the TracePathHandler (not via a model struct).

## Dependencies

None. Models are a foundational component used by all other components.

## Design Decisions

### String for DateTime64
DateTime64 fields use `string` type rather than `time.Time`. This avoids timezone conversion issues and lets the handler code control parsing format. The trade-off is manual parsing when time comparisons are needed.

### Separate request/response structs in handlers
Request and response types are defined in the handler packages (not in models) because they have different shapes from the database models. This keeps models focused on database mapping.

## Testing

No tests exist for the models package.
