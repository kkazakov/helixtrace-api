-- 002_tokens.sql
-- Creates the tokens table for session management across multiple workers.
-- Engine: ReplacingMergeTree (deduplicates by token on merge).

CREATE TABLE IF NOT EXISTS tokens
(
    token      String,
    email      String,
    created_at DateTime64(3, 'UTC') DEFAULT now64(),
    expires_at DateTime64(3, 'UTC')
)
ENGINE = ReplacingMergeTree(created_at)
ORDER BY token
TTL expires_at DELETE;
