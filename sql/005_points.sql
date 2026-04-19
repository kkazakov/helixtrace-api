-- 005_points.sql
-- Creates the points table for user-defined geographic points.
-- Engine: ReplacingMergeTree (deduplicates by (user, id) on merge).
-- Soft deletes via deleted flag; never use ALTER TABLE DELETE.

CREATE TABLE IF NOT EXISTS points
(
    id          UUID                     CODEC(ZSTD(1)),
    lat         Float64                  CODEC(ZSTD(1)),
    lon         Float64                  CODEC(ZSTD(1)),
    elevation   Float64                  CODEC(Delta, ZSTD(1)),
    user        String                   CODEC(ZSTD(1)),
    public      Bool                     DEFAULT false,
    label       String                   CODEC(ZSTD(1)),
    category_id UInt8                    CODEC(LZ4),
    deleted     Bool                     DEFAULT false,
    updated_at  DateTime64(3, 'UTC')     CODEC(Delta, ZSTD(1))
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (user, id);
