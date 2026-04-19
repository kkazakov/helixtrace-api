-- 004_point_categories.sql
-- Creates the point_categories table for categorizing user-defined points.
-- Engine: ReplacingMergeTree (deduplicates by id on merge).

CREATE TABLE IF NOT EXISTS point_categories
(
    id         UInt8,
    name       LowCardinality(String),
    updated_at DateTime64(3, 'UTC') DEFAULT now64()
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY id;

-- Seed categories
INSERT INTO point_categories (id, name) VALUES
    (1, 'poi'),
    (2, 'repeater'),
    (3, 'unknown');
