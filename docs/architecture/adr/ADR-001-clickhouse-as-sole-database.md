# ADR-001: ClickHouse as Sole Database

**Status:** Accepted
**Date:** 2024-01-15

## Context

The application needs to store:
- User accounts and authentication tokens
- User-defined geographic points with metadata
- Cached elevation trace paths (arrays of lat/lon/elevation values)

Traditional row-based databases (PostgreSQL, MySQL) handle relational data well but are less efficient for storing and querying large arrays of numeric data.

## Decision

Use ClickHouse as the sole database for all data storage, including user accounts, tokens, points, and trace path cache.

## Consequences

### Positive
- Efficient storage of trace path arrays using `Array(Float64)` columns
- Columnar compression (ZSTD, Delta) reduces storage for elevation data
- Fast aggregate queries for point listings
- Single database dependency simplifies deployment
- `ReplacingMergeTree` engine provides natural upsert semantics

### Negative
- `ReplacingMergeTree` requires `FINAL` keyword for consistent reads, adding query complexity
- No native foreign key constraints; category validation is done in application code
- ClickHouse is optimized for analytics workloads; point-level CRUD is less conventional
- Operators familiar with PostgreSQL need to learn ClickHouse-specific patterns
- TTL-based token cleanup depends on ClickHouse background merges (not immediate)

### Mitigations
- All queries on ReplacingMergeTree tables include `FINAL`
- Application enforces referential integrity (category validation on create/update)
- Token expiry uses both application-level check (`expires_at > now64()`) and TTL cleanup
