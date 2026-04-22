# ADR-002: ReplacingMergeTree for User Data Tables

**Status:** Accepted
**Date:** 2024-01-15

## Context

The users, tokens, points, and point_categories tables need to support updates. ClickHouse does not support `UPDATE` or `DELETE` in the traditional sense — modifications are done by inserting new rows.

Two approaches were considered:
1. **MergeTree** with application-level version tracking
2. **ReplacingMergeTree** with automatic deduplication by version column

## Decision

Use `ReplacingMergeTree` for users, tokens, points, and point_categories tables. Each table uses a version column (`updated_at` or `created_at`) that determines which row is the "latest."

| Table | Version Column | Order By |
|---|---|---|
| users | `updated_at` | `email` |
| tokens | `created_at` | `token` |
| points | `updated_at` | `(user, id)` |
| point_categories | `updated_at` | `id` |

## Consequences

### Positive
- Updates are simple inserts with a newer version timestamp
- ClickHouse automatically deduplicates during background merges
- Soft deletes work by re-inserting with `deleted = true`
- No need for separate update/delete logic

### Negative
- Queries MUST include `FINAL` to see the latest version
- `FINAL` adds overhead as it forces a merge before returning results
- Background merges are eventual; without `FINAL`, stale rows may appear
- Developers must remember `FINAL` on every query — omitting it causes silent data inconsistency

### Mitigations
- All SELECT queries in the codebase include `FINAL`
- Documentation and code reviews emphasize the `FINAL` requirement
- The `trace_paths` table uses plain `MergeTree` (append-only cache) to avoid `FINAL` overhead on cache lookups
