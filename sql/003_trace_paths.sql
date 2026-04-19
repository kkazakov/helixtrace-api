CREATE TABLE IF NOT EXISTS trace_paths (
    path_hash String,
    from_lat Float64,
    from_lon Float64,
    to_lat Float64,
    to_lon Float64,
    point_distance Float64,
    count UInt32,
    lats Array(Float64),
    lngs Array(Float64),
    elvs Array(Float64),
    created_at DateTime64(3) DEFAULT now64(3)
) ENGINE = MergeTree()
ORDER BY (path_hash, created_at);
