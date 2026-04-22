# Documentation Index

Navigation hub for helixtrace-api documentation.

## Getting Started

- [README.md](../README.md) — Project overview, build/run/test commands, quick reference
- [PURPOSE.md](../PURPOSE.md) — Business context, value, constraints

## Architecture

- [Architecture Overview](explanation/architecture-overview.md) — System context, component map, data flows, cross-cutting concerns

## Component Deep-Dives

- [Entry Point & Routing](explanation/entry-point.md) — Application bootstrap, chi router setup, middleware chain
- [Configuration](explanation/configuration.md) — Environment variable loading, CORS parsing, default values
- [Database Layer](explanation/database.md) — ClickHouse connection, migration runner, idempotent schema initialization
- [Authentication](explanation/authentication.md) — Login/register handlers, token generation, Bearer auth middleware
- [Trace Path Handler](explanation/trace-path.md) — Haversine interpolation, OpenTopoData integration, ClickHouse caching with mutex dedup
- [Points Handler](explanation/points.md) — Point CRUD, meshcore repeater integration, batch elevation fetching
- [Models](explanation/models.md) — User, Token, Point, PointCategory data structures

## API Reference

- [REST API](reference/api.md) — Endpoint descriptions, request/response schemas

## Architecture Decision Records

- [ADR-001: ClickHouse as Sole Database](architecture/adr/ADR-001-clickhouse-as-sole-database.md) — Why ClickHouse over PostgreSQL/MySQL
- [ADR-002: ReplacingMergeTree for All User Data](architecture/adr/ADR-002-replacing-merge-tree.md) — Why ReplacingMergeTree with FINAL queries
